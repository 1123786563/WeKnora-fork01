// fakejira.go 提供 Jira 形替身背后的 fake Jira REST v3 服务（T13；仅测试
// 使用，绝不进入生产装配）。它是「按凭据分账本」的受控远端：
//
//   - GET /rest/api/3/myself：Basic(email:APIToken) 验证凭据——替身授权页
//     发码前真实验证成员 Jira 凭据（与 T04 示例服务授权页同语义）；
//   - POST /rest/api/3/search/jql：固定 JQL 语义——忽略请求携带的任何 jql，
//     返回该账本全部未解决（Resolved=false）且 due 落在本周
//     （[本周一, 下周一)，startOfWeek 语义与 T04 jqlMyWeek 一致）的事项；
//   - 故障注入：DenySearch（该账本 search 返回 403/401）、
//     InvalidateAccount（凭据失效 → 401）、SetSearchDelay（慢响应 → 配合
//     替身可配短超时制造超时态）。
//
// 成员凭据由测试侧随机生成（"tok-"+随机十六进制）后经 AddAccount 登记账
// 本——源码不留任何可用凭据字面量（全局约束）。
package plugintest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/utils"
)

// JiraIssue 是 fake 账本里的单条事项（字段白名单与 T04 jiraSearchFields
// 一致：summary/status/duedate）。
type JiraIssue struct {
	Key      string
	Summary  string
	Status   string
	Due      string // YYYY-MM-DD；缺 due 或 due 不在本周内的事项被固定 JQL 排除
	Resolved bool   // true = 已解决（resolution != Unresolved），被固定 JQL 排除
}

// jiraAccount 是一名成员的凭据与账本。
type jiraAccount struct {
	Email       string
	APIToken    string
	Issues      []JiraIssue
	SearchDeny  int           // 0=正常；非 0 = /search/jql 对该账本返回此状态码（403/401 注入）
	MyselfDelay time.Duration // /myself 对该账本的响应延迟（慢凭据验证注入，T13-OCR1-F4 回归）
}

// FakeJira 是按凭据分账本的 fake Jira REST v3 服务。
type FakeJira struct {
	mu       sync.Mutex
	accounts map[string]*jiraAccount // key = Authorization 头原值（"Basic <b64(email:token)>"）
	delay    time.Duration           // /search/jql 响应延迟（超时注入）
	calls    atomic.Int64
	srv      *httptest.Server
	baseURL  string
}

// NewFakeJira 启动 fake Jira（httptest 监听 127.0.0.1）并登记 cleanup。
func NewFakeJira(t testing.TB) *FakeJira {
	t.Helper()
	// 替身经 SSRF-safe client 出站访问本 fake（127.0.0.1 需放行；快照恢复
	// 式，与 plugintest.Server.Start 同惯例，-count>=2 安全）。
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	j := &FakeJira{accounts: map[string]*jiraAccount{}}
	j.srv = httptest.NewServer(http.HandlerFunc(j.handle))
	t.Cleanup(j.srv.Close)
	j.baseURL = j.srv.URL
	return j
}

// BaseURL 返回 fake Jira 的 base URL（浏览链接 = BaseURL()/browse/<KEY>）。
func (j *FakeJira) BaseURL() string { return j.baseURL }

// Calls 返回 fake 收到的请求总数（myself + search；供「拒绝调用不触达
// Jira」类断言取基线）。
func (j *FakeJira) Calls() int64 { return j.calls.Load() }

// AddAccount 登记一名成员的凭据与账本事项（同 email 重复登记 Fatal）。
func (j *FakeJira) AddAccount(t testing.TB, email, apiToken string, issues ...JiraIssue) {
	t.Helper()
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, account := range j.accounts {
		if account.Email == email {
			t.Fatalf("fakejira: duplicate account for %s", email)
		}
	}
	j.accounts[jiraBasicAuth(email, apiToken)] = &jiraAccount{
		Email:    email,
		APIToken: apiToken,
		Issues:   append([]JiraIssue(nil), issues...),
	}
}

// DenySearch 注入该账本 /search/jql 的拒绝状态码（403 无权限 / 401 失效）。
func (j *FakeJira) DenySearch(email string, statusCode int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, account := range j.accounts {
		if account.Email == email {
			account.SearchDeny = statusCode
			return
		}
	}
}

// InvalidateAccount 使该账本凭据失效：此后 /myself 与 /search 均对其 401
// （授权完成后 token 失效场景）。
func (j *FakeJira) InvalidateAccount(email string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for key, account := range j.accounts {
		if account.Email == email {
			delete(j.accounts, key)
			return
		}
	}
}

// SetSearchDelay 注入 /search/jql 响应延迟（配合替身可配短超时制造超时态）。
func (j *FakeJira) SetSearchDelay(d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.delay = d
}

// SetMyselfDelay 注入该账本 /myself 的响应延迟——替身授权页的凭据验证
// （credentialCheck 出站）变慢，供「慢验证不得串行化替身其他链路」的回归
// 测试使用（T13-OCR1-F4）。按账本注入，避免波及其他成员的正常验证。
func (j *FakeJira) SetMyselfDelay(email string, d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, account := range j.accounts {
		if account.Email == email {
			account.MyselfDelay = d
			return
		}
	}
}

// verifyCredentialClient 给 /myself 验证出站一个上界（T13-OCR1-F4 第二层
// 防御）：credentialCheck 是替身 OAuth 端点的同步路径，无界出站会挂起整条
// 授权链；本地 fake 正常响应在毫秒级，10s 足够宽。
var verifyCredentialClient = &http.Client{Timeout: 10 * time.Second}

// VerifyCredentials 以 Basic(email:APIToken) 调 /myself 验证凭据：200 视为
// 有效；401 返回凭据错误（替身授权页发码前的真实验证，T04 同语义）。
func (j *FakeJira) VerifyCredentials(email, apiToken string) error {
	req, err := http.NewRequest(http.MethodGet, j.baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return fmt.Errorf("build myself request: %w", err)
	}
	req.Header.Set("Authorization", jiraBasicAuth(email, apiToken))
	resp, err := verifyCredentialClient.Do(req)
	if err != nil {
		return fmt.Errorf("verify jira credentials: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("invalid jira credentials (HTTP %d)", resp.StatusCode)
}

func (j *FakeJira) handle(w http.ResponseWriter, r *http.Request) {
	j.calls.Add(1)
	j.mu.Lock()
	account := j.accounts[r.Header.Get("Authorization")]
	var myselfDelay time.Duration
	if account != nil {
		myselfDelay = account.MyselfDelay
	}
	j.mu.Unlock()
	if account == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"errorMessages": []string{"unauthorized"}})
		return
	}
	switch r.URL.Path {
	case "/rest/api/3/myself":
		if myselfDelay > 0 {
			time.Sleep(myselfDelay)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"emailAddress": account.Email,
			"displayName":  account.Email,
		})
	case "/rest/api/3/search/jql":
		j.mu.Lock()
		deny, delay := account.SearchDeny, j.delay
		issues := append([]JiraIssue(nil), account.Issues...)
		j.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		if deny != 0 {
			w.WriteHeader(deny)
			_ = json.NewEncoder(w).Encode(map[string]any{"errorMessages": []string{"denied"}})
			return
		}
		payload := make([]map[string]any, 0, len(issues))
		for _, issue := range issues {
			// 固定 JQL 语义：未解决 且 due 在本周内（忽略请求 jql）。
			if issue.Resolved || !jiraDueInThisWeek(issue.Due) {
				continue
			}
			payload = append(payload, map[string]any{
				"key": issue.Key,
				"fields": map[string]any{
					"summary": issue.Summary,
					"status":  map[string]any{"name": issue.Status},
					"duedate": issue.Due,
				},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issues": payload})
	default:
		http.NotFound(w, r)
	}
}

// jiraDueInThisWeek 判定 due（YYYY-MM-DD）是否落在 [本周一, 下周一) 内
// （本地时区；startOfWeek 语义与 T04 jqlMyWeek 一致）。
func jiraDueInThisWeek(due string) bool {
	day, err := time.ParseInLocation("2006-01-02", due, time.Local)
	if err != nil {
		return false
	}
	monday := jiraThisMonday(time.Now())
	next := monday.AddDate(0, 0, 7)
	return !day.Before(monday) && day.Before(next)
}

// jiraThisMonday 归一 now 所在周的周一 00:00（本地时区；Go 的 Sunday=0
// 归一为 7，使周一恒为偏移 0）。
func jiraThisMonday(now time.Time) time.Time {
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(wd - 1))
}

// jiraBasicAuth 构造 Basic 认证头值（凭据只在请求头内存中组装，不落盘）。
func jiraBasicAuth(email, apiToken string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+apiToken))
}
