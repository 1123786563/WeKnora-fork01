package appconnector

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// FS-04 controlled real execution (interface-verification spec): the user
// designates BOTH the test chat and the exact message content; the REAL
// FeishuSendAdapter sends that reviewed text through the official
// open.feishu.cn contract, the real message id is captured, and the message
// is NOT recalled afterwards (cleanup follows external authority only).
//
// Credentials alone NEVER arm this test: FS_APP_ID/FS_APP_SECRET without an
// explicit FS_TEST_CHAT_ID + FS_TEST_MESSAGE designation stays a skip — the
// bot's first visible chat is never auto-picked as a send target.
// A skip is never a pass.
func TestFeishuRealControlledSend(t *testing.T) {
	appID := os.Getenv("FS_APP_ID")
	appSecret := os.Getenv("FS_APP_SECRET")
	// FS-04: the user must explicitly designate the target chat and the
	// content; nothing is inferred from the bot's visible chats.
	chatID := strings.TrimSpace(os.Getenv("FS_TEST_CHAT_ID"))
	message := os.Getenv("FS_TEST_MESSAGE")
	if appID == "" || appSecret == "" || strings.HasPrefix(appID, "cli_xxx") ||
		chatID == "" || message == "" {
		t.Skip("feishu real execution not designated: set FS_APP_ID/FS_APP_SECRET plus FS_TEST_CHAT_ID (the designated test group) and FS_TEST_MESSAGE (the reviewed content) in artifacts/connector-real/feishu.env; skip is not a pass — FS-04 stays blocked-env")
	}
	client := &http.Client{Timeout: 15 * time.Second}

	// Token wiring (production container would route this via the same
	// outbound policy; evidence wiring keeps it explicit here).
	tokenReq, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	resp, err := client.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json", strings.NewReader(string(tokenReq)))
	if err != nil {
		t.Fatalf("tenant token fetch: %v", err)
	}
	defer resp.Body.Close()
	var tokResp struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg"`
		Token string `json:"tenant_access_token"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err := json.Unmarshal(body, &tokResp); err != nil {
		t.Fatalf("token response: %v", err)
	}
	if tokResp.Code != 0 || tokResp.Token == "" {
		t.Fatalf("token fetch failed: code=%d msg=%s (check app credentials + im:message permission + published version)", tokResp.Code, tokResp.Msg)
	}
	t.Logf("tenant_access_token acquired (code=0)")

	// Target cross-check (FS-04 目标核对): the DESIGNATED chat must be
	// among the bot's visible chats. The listing only validates the
	// designation — it never chooses a target.
	req, _ := http.NewRequest(http.MethodGet, "https://open.feishu.cn/open-apis/im/v1/chats?page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+tokResp.Token)
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
	var chats struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []struct {
				ChatID string `json:"chat_id"`
				Name   string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body2, &chats); err != nil {
		t.Fatalf("chats response: %v", err)
	}
	if chats.Code != 0 {
		t.Fatalf("list chats failed: code=%d msg=%s", chats.Code, chats.Msg)
	}
	designated := ""
	for _, item := range chats.Data.Items {
		if item.ChatID == chatID {
			designated = item.Name
			break
		}
	}
	if designated == "" {
		t.Fatalf("designated FS_TEST_CHAT_ID %s is not visible to the bot — add the bot to the designated test group first", chatID)
	}
	t.Logf("designated chat verified: %s (%s)", chatID, designated)

	ad := &FeishuSendAdapter{
		Policy: HTTPPolicy{
			Scheme:     "https",
			Host:       FeishuAPIHost,
			Methods:    []string{http.MethodPost, http.MethodGet},
			PathPrefix: "/open-apis/",
		},
		Token:          func(ctx context.Context) (string, error) { return tokResp.Token, nil },
		KeyGuaranteed:  true,
		DedupWindow:    FeishuDedupWindow,
		FirstAttemptAt: func(a Action) time.Time { return time.Now() },
		Now:            func() time.Time { return time.Now() },
	}
	args := map[string]any{
		"chat_id":  chatID,
		"msg_type": "text",
		"content":  map[string]string{"text": message},
	}
	rawArgs, _ := json.Marshal(args)
	action := Action{
		ID: "act_fs04_real_1", TenantID: 7, ActorID: "user_real",
		ConnectionID: "conn_fs04", Version: "v1",
		Target: chatID, Risk: RiskSend,
		Args: json.RawMessage(rawArgs),
	}
	out, err := ad.Execute(context.Background(), action)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real send: state=%s err=%v", out.State, err)
	}
	t.Logf("REAL message sent: state=%s output=%s", out.State, string(out.Output))
	if !strings.Contains(string(out.Output), "om_") {
		t.Fatalf("real message id (om_...) missing from output: %s", string(out.Output))
	}
	// FS-04: the message is deliberately NOT recalled; evidence above.
	t.Logf("cleanup policy: message left in place per FS-04 (external authority only)")
}
