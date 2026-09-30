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

// Real-provider controlled evidence for the Confluence publish loop
// (NO-04 discipline): the REAL adapters run the full closed loop against
// the configured Confluence site: create under the designated test parent
// page (baseline version read) → receipt → update against the SAME page
// (version read + conflict-free write) → a stale-version update must be
// REFUSED with zero writes. Gated on CONFLUENCE_BASE_URL /
// CONFLUENCE_EMAIL / CONFLUENCE_API_TOKEN / CONFLUENCE_PARENT_PAGE_ID; a
// skip is never a pass — T20 real-provider evidence stays blocked-env in
// environments without these.
func TestConfluenceRealPublishLoop(t *testing.T) {
	baseURL := os.Getenv("CONFLUENCE_BASE_URL")
	email := os.Getenv("CONFLUENCE_EMAIL")
	token := os.Getenv("CONFLUENCE_API_TOKEN")
	parent := os.Getenv("CONFLUENCE_PARENT_PAGE_ID")
	if baseURL == "" || email == "" || token == "" || parent == "" || strings.HasPrefix(parent, "xxxx") {
		t.Skip("confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env")
	}
	ep, err := ParseConfluenceBaseURL(baseURL, "")
	if err != nil {
		t.Fatalf("CONFLUENCE_BASE_URL: %v", err)
	}
	pol := HTTPPolicy{
		Scheme: ep.Scheme, Host: ep.Host, Port: ep.Port,
		Methods:    []string{"GET", "POST", "PUT"},
		PathPrefix: ep.APIBasePath + "/",
		Timeout:    30 * time.Second,
	}
	cred := func(ctx context.Context) (ConfluenceCredential, error) {
		return ConfluenceCredential{Username: email, Secret: token}, nil
	}
	caps := func(ctx context.Context, a Action) ([]string, error) { return []string{ConfluenceCapabilityWrite}, nil }
	stamp := time.Now().Format("150405")

	create := &ConfluenceCreateAdapter{
		Policy: pol, Credential: cred, Edition: ep.Edition, APIBasePath: ep.APIBasePath,
		ApprovedParents:        []string{parent},
		ConnectionCapabilities: caps,
	}
	createArgs, _ := json.Marshal(map[string]any{
		"parent": parent, "title": "T20 real " + stamp,
		"storage": "<p>t20 real publish " + stamp + "</p>",
	})
	createAction := Action{ID: "act_t20_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: parent, Risk: RiskWrite, Args: createArgs}
	out, err := create.Execute(context.Background(), createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	pageID := out.ExternalID
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != pageID || rcpt.ExternalVersion == "" {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL page created: id=%s version=%s", pageID, rcpt.ExternalVersion)

	// Update against the SAME page: read its CURRENT version live (the
	// create bumped the parent space's page inventory; never assume).
	live, lerr := ReadConfluencePageVersion(context.Background(), pol, cred, ep.Edition, ep.APIBasePath, pageID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	update := &ConfluenceUpdateAdapter{
		Policy: pol, Credential: cred, Edition: ep.Edition, APIBasePath: ep.APIBasePath,
		ConnectionCapabilities: caps,
	}
	updateArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": live,
		"title":   "T20 real " + stamp,
		"storage": "<p>t20 real update " + stamp + "</p>",
	})
	updateAction := Action{ID: "act_t20_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: pageID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := update.Execute(context.Background(), updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseConfluencePageReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalVersion == live {
		t.Fatalf("real update receipt must carry a NEW version: %+v (had %s)", urcpt, live)
	}
	t.Logf("REAL page updated: id=%s version %s -> %s", pageID, live, urcpt.ExternalVersion)

	// Stale-version update: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": live, // superseded by the update above
		"title":   "T20 stale " + stamp,
		"storage": "<p>t20 stale " + stamp + "</p>",
	})
	staleAction := Action{ID: "act_t20_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: pageID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := update.Execute(context.Background(), staleAction)
	if !errors.Is(serr, ErrConfluenceVersionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale update must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale update refused with zero writes (conflict on %s)", live)
}
