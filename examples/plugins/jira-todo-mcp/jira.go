package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/utils"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// jqlMyWeek 是服务端固定的 JQL 模板（T05/T13 复刻消费的权威契约，OCR
// T04-R1-8 修复后含上下界）：本人 + 未解决 + 截止日期在本周内
// （due >= startOfWeek() 且 due < startOfWeek("+1w")，不含下周及以后）。
// 绝不接受任何调用方输入拼接——工具 schema 无参数，从根上排除 JQL 注入。
const jqlMyWeek = "assignee = currentUser() AND resolution = Unresolved AND due >= startOfWeek() AND due < startOfWeek(\"+1w\") ORDER BY due ASC"

// jiraSearchFields 是搜索时请求的字段白名单（最小化披露）。
var jiraSearchFields = []string{"summary", "status", "duedate"}

const (
	// jiraSearchMaxResults 是单页请求上限（服务端可能下调）。
	jiraSearchMaxResults = 100
	// maxSearchPages 是分页跟进 nextPageToken 的页数上限：超限即停止并
	// 标注截断，防失控翻页（OCR T04-R1-6）。
	maxSearchPages = 10
	// maxToolOutputLines / maxToolOutputBytes 封顶工具输出文本（跨任务转交
	// T01-R4-F2）：分页上限名义上约束 10×100=1000 条事项，Jira summary 单条
	// 可达 255 字符——拼接后单次 CallToolResult 可达数百 KB 整体进入调用方
	//（LLM 上下文/MCP 消息）。超限截断并追加与页数截断同风格的显式标注
	//（非静默截断）。
	maxToolOutputLines = 200
	maxToolOutputBytes = 64 << 10
)

// maxToolCallTimeout 是单次工具调用的整体时限（OCR 一轮 F5）：分页循环
// 最多 10 页、每页由 jiraHTTPClient 的 30s Timeout 独立约束——上游持续
// 慢响应时单次调用最坏 ≈5 分钟占用 /mcp 连接与 goroutine（WeKnora 默认
// 30s 客户端会提前切断，但 AdvancedConfig.Timeout 可调大）。var 供测试改写。
var maxToolCallTimeout = 60 * time.Second

// jiraMaxResponseBytes 封顶单次 Jira 响应体的解码读取量（整分支 OCR 二轮
// F5）：分页/条数封顶只约束请求参数，响应体积由对端决定——被攻陷或异常的
// Jira 可在 30s 超时窗口内推送超大 JSON 流，Decoder 随解码逐步分配造成内存
// 放大。截断导致 unexpected EOF 天然 fail-closed（显式报错，不静默当空结果）。
// var 供测试改写（同 maxPendingAuths 先例）。
var jiraMaxResponseBytes int64 = 16 << 20

// jiraHTTPError 是 Jira REST 返回 HTTP ≥400 时的类型化错误：携带状态码，
// 供调用方区分「凭据性失败」（401/403）与「上游故障」（5xx/网络错误）——
// OCR T04-R1-10：上游故障不得伪装成凭据错误。
type jiraHTTPError struct {
	Method     string
	Path       string
	StatusCode int
}

func (e *jiraHTTPError) Error() string {
	return fmt.Sprintf("jira %s %s returned HTTP %d", e.Method, e.Path, e.StatusCode)
}

// jiraHTTPClient 是进程级共享的 SSRF-safe HTTP client（http.Client 并发
// 安全；共享以复用 keep-alive 连接池，OCR T04-R1-5）。
var jiraHTTPClient = newJiraHTTPClient()

// JiraClient 是 Jira REST v3 的只读客户端。凭据（Email + APIToken）仅存于
// 内存中的授权会话，绝不落盘、不写日志。出站请求统一走 SSRF-safe HTTP
// client（仅 http/https，拒绝环回/私有/保留地址；测试经 SSRF_WHITELIST
// 放行 127.0.0.1 fake）。
type JiraClient struct {
	BaseURL  string
	Email    string
	APIToken string
}

// newJiraHTTPClient 返回 SSRF-safe HTTP client（30s 超时，与仓库 MCP 客户端
// 缺省一致）。
func newJiraHTTPClient() *http.Client {
	cfg := utils.DefaultSSRFSafeHTTPClientConfig()
	cfg.Timeout = 30 * time.Second
	return utils.NewSSRFSafeHTTPClient(cfg)
}

// JiraIssue 是搜索结果的规范化视图（只保留工具输出需要的字段）。
type JiraIssue struct {
	Key     string
	Summary string
	Status  string
	Due     string // YYYY-MM-DD；缺失为空
	URL     string
}

// jiraSearchResponse 是搜索响应的最小映射（含分页游标 nextPageToken）。
// 字段值键名按 Atlassian 官方 API 参考核实为 `fields`（无下划线）：
// developer.atlassian.com/cloud/jira/platform/rest/v3/
// api-group-issue-search/#api-rest-api-3-search-jql-post——OCR r3-005 主张的
// `_fields` 不成立（整分支终评 R4 轮核实，裁决见 rulings.md R3）。
type jiraSearchResponse struct {
	Issues []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name string `json:"name"`
			} `json:"status"`
			DueDate string `json:"duedate"`
		} `json:"fields"`
	} `json:"issues"`
	NextPageToken string `json:"nextPageToken"`
}

// jiraMyselfResponse 是 /myself 的最小映射。
type jiraMyselfResponse struct {
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// do 执行一次带 Basic 认证的 Jira REST 请求并解析 JSON。HTTP 状态 ≥400
// 返回携带状态码的 *jiraHTTPError——绝不把上游失败伪装成空结果。
func (c *JiraClient) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, body)
	if err != nil {
		return fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Basic 认证只进请求头，不进任何日志。
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
		[]byte(c.Email+":"+c.APIToken)))
	resp, err := jiraHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira request %s %s failed: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return &jiraHTTPError{Method: method, Path: path, StatusCode: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	// 解码前封顶读取量（整分支 OCR 二轮 F5）：超限截断 → Decoder 报
	// unexpected EOF → 调用方按上游故障处理，绝不静默吞成空结果。
	if err := json.NewDecoder(io.LimitReader(resp.Body, jiraMaxResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("decode jira %s response: %w", path, err)
	}
	return nil
}

// Myself 用 Email+APIToken 验证凭据并返回成员标识（授权页发码前调用）。
func (c *JiraClient) Myself(ctx context.Context) (jiraMyselfResponse, error) {
	var myself jiraMyselfResponse
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/myself", nil, &myself); err != nil {
		return jiraMyselfResponse{}, err
	}
	return myself, nil
}

// SearchMyWeek 执行固定 JQL 并返回规范化事项列表；跟进 nextPageToken 分页
// 取全（页数上限 maxSearchPages，超限置 truncated=true 由调用方标注——
// OCR T04-R1-6：不得静默截断）。空结果是合法成功（返回空切片、nil 错误）。
func (c *JiraClient) SearchMyWeek(ctx context.Context) (issues []JiraIssue, truncated bool, err error) {
	issues = make([]JiraIssue, 0)
	pageToken := ""
	for page := 1; ; page++ {
		payload := map[string]any{
			"jql":        jqlMyWeek,
			"fields":     jiraSearchFields,
			"maxResults": jiraSearchMaxResults,
		}
		if pageToken != "" {
			payload["pageToken"] = pageToken
		}
		var result jiraSearchResponse
		if err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", payload, &result); err != nil {
			return nil, false, err
		}
		for _, issue := range result.Issues {
			issues = append(issues, JiraIssue{
				Key:     issue.Key,
				Summary: issue.Fields.Summary,
				Status:  issue.Fields.Status.Name,
				Due:     issue.Fields.DueDate,
				URL:     strings.TrimRight(c.BaseURL, "/") + "/browse/" + issue.Key,
			})
		}
		if result.NextPageToken == "" {
			return issues, false, nil
		}
		if page >= maxSearchPages {
			return issues, true, nil
		}
		pageToken = result.NextPageToken
	}
}

// formatIssues 把事项列表格式化为工具输出文本：每行
// [{key}] {summary} · 状态 {status} · 截止 {due} · {url}，缺失字段省略
// 对应段；空列表输出空字符串（空数组语义，而非错误）。
func formatIssues(issues []JiraIssue) string {
	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		var b strings.Builder
		fmt.Fprintf(&b, "[%s]", issue.Key)
		if issue.Summary != "" {
			fmt.Fprintf(&b, " %s", issue.Summary)
		}
		if issue.Status != "" {
			fmt.Fprintf(&b, " · 状态 %s", issue.Status)
		}
		if issue.Due != "" {
			fmt.Fprintf(&b, " · 截止 %s", issue.Due)
		}
		if issue.URL != "" {
			fmt.Fprintf(&b, " · %s", issue.URL)
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

// renderIssuesOutput 渲染工具输出文本：formatIssues 结果施加行数/字节双界
// 封顶（跨任务转交 T01-R4-F2），超限截断并追加显式标注；页数截断标注保持
// 原样，两道截断各自独立声明（非静默截断）。字节截断落在 UTF-8 边界上。
func renderIssuesOutput(issues []JiraIssue, pagesTruncated bool) string {
	text := formatIssues(issues)
	var notes []string
	if pagesTruncated {
		notes = append(notes, fmt.Sprintf("⚠ 结果超过 %d 页被截断，请到 Jira 查看完整列表", maxSearchPages))
	}
	if text != "" {
		lines := strings.Split(text, "\n")
		if len(lines) > maxToolOutputLines {
			text = strings.Join(lines[:maxToolOutputLines], "\n")
			notes = append(notes, fmt.Sprintf("⚠ 输出超过 %d 行被截断，请到 Jira 查看完整列表", maxToolOutputLines))
		}
	}
	if len(text) > maxToolOutputBytes {
		cut := maxToolOutputBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		text = text[:cut]
		notes = append(notes, fmt.Sprintf("⚠ 输出超过 %d 字节被截断，请到 Jira 查看完整列表", maxToolOutputBytes))
	}
	for _, note := range notes {
		if text != "" {
			text += "\n"
		}
		text += note
	}
	return text
}

// handleSearchMyWeek 是工具执行入口：从 context 取已授权会话（由
// WithHTTPContextFunc 注入），以会话凭据执行固定 JQL 查询。
func handleSearchMyWeek(ctx context.Context, jiraBaseURL string) (*mcp.CallToolResult, error) {
	session := sessionFromContext(ctx)
	if session == nil {
		// 正常情况下 gateCallToolAuth 已在 HTTP 层拦截；此处防御性拒绝，
		// 绝不带默认凭据执行。
		return nil, fmt.Errorf("unauthorized: no personal OAuth session bound to this tool call")
	}
	// OCR 一轮 F5：分页循环的每页 30s 超时只约束单页，最多 10 页叠加可拖
	// 满 ≈5 分钟——入口施加整体 deadline，慢上游在此被确定性切断。
	ctx, cancel := context.WithTimeout(ctx, maxToolCallTimeout)
	defer cancel()
	client := &JiraClient{BaseURL: jiraBaseURL, Email: session.Email, APIToken: session.APIToken}
	issues, truncated, err := client.SearchMyWeek(ctx)
	if err != nil {
		return nil, err
	}
	text := renderIssuesOutput(issues, truncated)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(text)},
	}, nil
}
