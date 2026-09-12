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
// designates a test chat and provides one-off app credentials; the REAL
// FeishuSendAdapter sends one reviewed text message through the official
// open.feishu.cn contract, the real message id is captured, and the message
// is NOT recalled afterwards (cleanup follows external authority only).
// Gated on FS_APP_ID/FS_APP_SECRET; a skip is never a pass.
func TestFeishuRealControlledSend(t *testing.T) {
	appID := os.Getenv("FS_APP_ID")
	appSecret := os.Getenv("FS_APP_SECRET")
	if appID == "" || appSecret == "" || strings.HasPrefix(appID, "cli_xxx") {
		t.Skip("feishu real credentials not configured (FS_APP_ID/FS_APP_SECRET in artifacts/connector-real/feishu.env); skip is not a pass — FS-04 stays blocked-env")
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

	// Discover the bot's chat: the user adds the bot to one test group.
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
	if chats.Code != 0 || len(chats.Data.Items) == 0 {
		t.Fatalf("no chats visible: code=%d msg=%s — add the bot to a test group first", chats.Code, chats.Msg)
	}
	chat := chats.Data.Items[0]
	t.Logf("designated chat: %s (%s)", chat.ChatID, chat.Name)

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
	stamp := time.Now().Format("15:04:05")
	args := map[string]any{
		"chat_id":  chat.ChatID,
		"msg_type": "text",
		"content":  map[string]string{"text": "WeKnora FS-04 controlled send " + stamp},
	}
	rawArgs, _ := json.Marshal(args)
	action := Action{
		ID: "act_fs04_real_1", TenantID: 7, ActorID: "user_real",
		ConnectionID: "conn_fs04", Version: "v1",
		Target: chat.ChatID, Risk: RiskSend,
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
