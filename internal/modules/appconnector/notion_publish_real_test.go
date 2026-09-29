package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Real-provider controlled evidence for the publish loop (NO-04
// discipline): the user designates a test parent page (shared with a
// one-off integration) and its token. The REAL adapters run the full
// closed loop against api.notion.com: create (baseline version read) →
// receipt → update against the SAME page (version read + conflict-free
// write) → a stale-version update must be REFUSED with zero writes.
// Gated on NOTION_TOKEN/NOTION_PARENT_PAGE_ID; a skip is never a pass.
func TestNotionRealPublishLoop(t *testing.T) {
	token := os.Getenv("NOTION_TOKEN")
	parent := os.Getenv("NOTION_PARENT_PAGE_ID")
	if token == "" || parent == "" || strings.HasPrefix(parent, "xxxx") {
		t.Skip("notion real credentials not configured (NOTION_TOKEN/NOTION_PARENT_PAGE_ID in artifacts/connector-real/notion.env); skip is not a pass — T18 real-provider evidence stays blocked-env")
	}
	progress := map[string]NotionPageProgress{}
	caps := func(ctx context.Context, a Action) ([]string, error) { return []string{NotionCapabilityInsert}, nil }
	pol := HTTPPolicy{
		Scheme:     "https",
		Host:       NotionAPIHost,
		Methods:    []string{"GET", "POST", "PATCH"},
		PathPrefix: "/v1/",
		Timeout:    30 * time.Second,
	}
	create := &NotionCreateAdapter{
		Policy:                 pol,
		Token:                  func(ctx context.Context) (string, error) { return token, nil },
		ApprovedParents:        []string{parent},
		ConnectionCapabilities: caps,
		LoadProgress:           func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:           func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
	stamp := time.Now().Format("15:04:05")
	block := map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "t18 real publish " + stamp},
		}}},
	}
	createArgs, _ := json.Marshal(map[string]any{"parent": parent, "title": "T18 real " + stamp, "blocks": []any{block}})
	createAction := Action{ID: "act_t18_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: parent, Risk: RiskWrite, Args: createArgs}

	out, err := create.Execute(context.Background(), createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	pageID := out.ExternalID
	rcpt, rerr := ParseNotionPageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != pageID || rcpt.ExternalVersion == "" {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL page created: id=%s version=%s", pageID, rcpt.ExternalVersion)

	// Update against the SAME page: read its current version, publish a
	// new block under it.
	update := &NotionUpdateAdapter{
		Policy:                 pol,
		Token:                  func(ctx context.Context) (string, error) { return token, nil },
		ConnectionCapabilities: caps,
		LoadProgress:           func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:           func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
	current := rcpt.ExternalVersion
	// The create's own append bumped the version; re-read the live value
	// instead of assuming.
	reader := &NotionUpdateAdapter{Policy: pol, Token: func(ctx context.Context) (string, error) { return token, nil }}
	live, lerr := ReadNotionPageVersion(context.Background(), reader.Policy, reader.Token, pageID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	current = live
	updateBlock := map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "t18 real update " + stamp},
		}}},
	}
	updateArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": current,
		"title": "T18 real " + stamp, "blocks": []any{updateBlock},
	})
	updateAction := Action{ID: "act_t18_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: pageID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := update.Execute(context.Background(), updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseNotionPageReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalID != pageID || urcpt.ExternalVersion == current {
		t.Fatalf("real update receipt must carry a NEW version: %+v (had %s)", urcpt, current)
	}
	t.Logf("REAL page updated: id=%s version %s -> %s", pageID, current, urcpt.ExternalVersion)

	// Stale-version update: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": current, // superseded by the update above
		"title": "T18 stale " + stamp, "blocks": []any{updateBlock},
	})
	staleAction := Action{ID: "act_t18_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: pageID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := update.Execute(context.Background(), staleAction)
	if !errors.Is(serr, ErrNotionVersionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale update must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale update refused with zero writes (conflict on %s)", current)
}
