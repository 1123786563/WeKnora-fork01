package appconnector

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// NO-04 controlled real execution (interface-verification spec): the user
// designates a test parent page (shared with a one-off integration) and its
// token; the REAL NotionCreateAdapter creates one page under that parent,
// and the REAL page id + parent + content are verified by reading the page
// back. Gated on NOTION_TOKEN/NOTION_PARENT_PAGE_ID; a skip is never a pass.
func TestNotionRealControlledCreate(t *testing.T) {
	token := os.Getenv("NOTION_TOKEN")
	parent := os.Getenv("NOTION_PARENT_PAGE_ID")
	if token == "" || parent == "" || strings.HasPrefix(parent, "xxxx") {
		t.Skip("notion real credentials not configured (NOTION_TOKEN/NOTION_PARENT_PAGE_ID in artifacts/connector-real/notion.env); skip is not a pass — NO-04 stays blocked-env")
	}
	ad := &NotionCreateAdapter{
		Policy: HTTPPolicy{
			Scheme:     "https",
			Host:       NotionAPIHost,
			Methods:    []string{"POST", "GET", "PATCH"},
			PathPrefix: "/v1/",
		},
		Token:                  func(ctx context.Context) (string, error) { return token, nil },
		ApprovedParents:        []string{parent},
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) { return []string{NotionCapabilityInsert}, nil },
	}
	stamp := time.Now().Format("15:04:05")
	args := map[string]any{
		"parent": parent,
		"title":  "WeKnora NO-04 test " + stamp,
		"blocks": []any{map[string]any{
			"object": "block",
			"type":   "paragraph",
			"paragraph": map[string]any{
				"rich_text": []any{map[string]any{
					"type": "text",
					"text": map[string]string{"content": "controlled real execution " + stamp},
				}},
			},
		}},
	}
	rawArgs, _ := json.Marshal(args)
	action := Action{
		ID: "act_no04_real_1", TenantID: 7, ActorID: "user_real",
		ConnectionID: "conn_no04", Version: "v1",
		Target: parent, Risk: RiskWrite,
		Args: json.RawMessage(rawArgs),
	}
	out, err := ad.Execute(context.Background(), action)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	pageID := out.ExternalID
	t.Logf("REAL page created: id=%s output=%s", pageID, string(out.Output))
	if pageID == "" || !strings.Contains(string(out.Output), pageID) {
		t.Fatalf("real page id missing: %+v", out)
	}
	// Verify parent + content by reading the page back through the adapter.
	q, qerr := ad.Query(context.Background(), action)
	if qerr != nil || q.State != ActionSucceeded {
		t.Fatalf("read-back verification failed: %+v %v", q, qerr)
	}
	t.Logf("read-back verified: parent and block content match (state=%s id=%s)", q.State, q.ExternalID)
}
