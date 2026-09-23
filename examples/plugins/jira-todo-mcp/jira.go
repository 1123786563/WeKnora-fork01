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

	"github.com/Tencent/WeKnora/internal/utils"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// jqlMyWeek 是服务端固定的 JQL 模板：本人 + 未解决 + 本周截止。绝不接受
// 任何调用方输入拼接——工具 schema 无参数，从根上排除 JQL 注入。
const jqlMyWeek = "assignee = currentUser() AND resolution = Unresolved AND due >= startOfWeek() ORDER BY due ASC"

// jiraSearchFields 是搜索时请求的字段白名单（最小化披露）。
var jiraSearchFields = []string{"summary", "status", "duedate"}

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

// jiraMyselfResponse / jiraSearchResponse 是 REST 响应的最小映射。
type jiraMyselfResponse struct {
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

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
}

// do 执行一次带 Basic 认证的 Jira REST 请求并解析 JSON。HTTP 状态 ≥400
// 返回带状态码的错误——绝不把上游失败伪装成空结果。
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
	resp, err := newJiraHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("jira request %s %s failed: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("jira %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
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

// SearchMyWeek 执行固定 JQL 并返回规范化事项列表。空结果是合法成功（返回
// 空切片、nil 错误）。
func (c *JiraClient) SearchMyWeek(ctx context.Context) ([]JiraIssue, error) {
	var result jiraSearchResponse
	payload := map[string]any{
		"jql":    jqlMyWeek,
		"fields": jiraSearchFields,
	}
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", payload, &result); err != nil {
		return nil, err
	}
	issues := make([]JiraIssue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, JiraIssue{
			Key:     issue.Key,
			Summary: issue.Fields.Summary,
			Status:  issue.Fields.Status.Name,
			Due:     issue.Fields.DueDate,
			URL:     strings.TrimRight(c.BaseURL, "/") + "/browse/" + issue.Key,
		})
	}
	return issues, nil
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

// handleSearchMyWeek 是工具执行入口：从 context 取已授权会话（由
// WithHTTPContextFunc 注入），以会话凭据执行固定 JQL 查询。
func handleSearchMyWeek(ctx context.Context, jiraBaseURL string) (*mcp.CallToolResult, error) {
	session := sessionFromContext(ctx)
	if session == nil {
		// 正常情况下 gateCallToolAuth 已在 HTTP 层拦截；此处防御性拒绝，
		// 绝不带默认凭据执行。
		return nil, fmt.Errorf("unauthorized: no personal OAuth session bound to this tool call")
	}
	client := &JiraClient{BaseURL: jiraBaseURL, Email: session.Email, APIToken: session.APIToken}
	issues, err := client.SearchMyWeek(ctx)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(formatIssues(issues))},
	}, nil
}
