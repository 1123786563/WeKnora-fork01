package appconnector

// Real-provider controlled evidence for the Feishu publish loop (#49):
// the user designates a test folder and a tenant-level app (app id +
// app secret from the deployment env). The REAL adapter runs the full
// closed loop against open.feishu.cn: create (baseline revision read) →
// receipt → append to the SAME document (revision read + conflict-free
// write) → a stale-revision append must be REFUSED with zero writes.
// Gated on FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN; a
// skip is never a pass — T19 real-provider evidence stays blocked-env.
//
// Token acquisition mirrors the datasource feishu connector's internal
// auth (client.go:92 tenant_access_token/internal); the docx API accepts
// tenant tokens (SDK SupportedAccessTokenTypes: Tenant + User).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestFeishuRealPublishLoop(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	folder := os.Getenv("FEISHU_TEST_FOLDER_TOKEN")
	if appID == "" || appSecret == "" || folder == "" {
		t.Skip("feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env")
	}
	ctx := context.Background()
	token, terr := feishuTenantAccessToken(ctx, appID, appSecret)
	if terr != nil {
		t.Fatalf("real tenant token: %v", terr)
	}
	pol := HTTPPolicy{
		Scheme: "https", Host: FeishuAPIHost,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		Timeout: 30 * time.Second,
	}
	tok := func(ctx context.Context) (string, error) { return token, nil }
	progress := map[string]FeishuDocProgress{}
	ad := &FeishuDocxAdapter{
		Policy: pol, Token: tok,
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) {
			return []string{FeishuCapabilityWriteDocx}, nil
		},
		LoadProgress: func(a Action) FeishuDocProgress { return progress[a.ID] },
		SaveProgress: func(a Action, p FeishuDocProgress) error { progress[a.ID] = p; return nil },
	}
	stamp := time.Now().Format("150405")
	blocks, _ := FeishuTextBlocks("t19 real publish " + stamp)
	createArgs, _ := json.Marshal(map[string]any{"parent_folder": folder, "title": "T19 real " + stamp, "blocks": blocks})
	createAction := Action{ID: "act_t19_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: folder, Risk: RiskWrite, Args: createArgs}

	out, err := ad.Execute(ctx, createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	docID := out.ExternalID
	rcpt, rerr := ParseFeishuDocReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != docID {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL document created: id=%s (title is empty by contract — the create API has no title field)", docID)

	// Append to the SAME document: read its current revision, then write.
	live, lerr := ReadFeishuDocumentVersion(ctx, pol, tok, docID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	updateBlocks, _ := FeishuTextBlocks("t19 real update " + stamp)
	updateArgs, _ := json.Marshal(map[string]any{"document_id": docID, "expected_revision": live.RevisionID,
		"title": "T19 real " + stamp, "blocks": updateBlocks})
	updateAction := Action{ID: "act_t19_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: docID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := ad.Execute(ctx, updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseFeishuDocReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalVersion == live.RevisionID {
		t.Fatalf("real update receipt must carry a NEW revision: %+v (had %s)", urcpt, live.RevisionID)
	}
	t.Logf("REAL document updated: id=%s revision %s -> %s", docID, live.RevisionID, urcpt.ExternalVersion)

	// Stale-revision append: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{"document_id": docID, "expected_revision": live.RevisionID,
		"title": "T19 stale " + stamp, "blocks": updateBlocks})
	staleAction := Action{ID: "act_t19_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t19",
		Version: "feishu/v1", Target: docID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := ad.Execute(ctx, staleAction)
	if !errors.Is(serr, ErrFeishuPublishRevisionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale append must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale append refused with zero writes (conflict on %s)", live.RevisionID)
}

// feishuTenantAccessToken exchanges app credentials for a
// tenant_access_token (the internal-auth shape the datasource feishu
// connector also uses).
func feishuTenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+FeishuAPIHost+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.TenantAccessToken == "" {
		return "", fmt.Errorf("token refused: code=%d msg=%s", out.Code, out.Msg)
	}
	return out.TenantAccessToken, nil
}
