package publish

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
)

type cfRemote struct {
	versions map[string]string
	err      error
}

func (f *cfRemote) ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.versions[pageID], nil
}

func cfScopeOK() ConfluenceConnectionScope {
	return ConfluenceConnectionScope{
		AppID: "confluence", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, WriteCapability: true,
		Edition: "cloud",
	}
}

type cfPlanEnv struct {
	svc   *ConfluencePublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
	fake  *cfWireFake
}

func newCfPlanEnv(t *testing.T, fake *cfWireFake, remote *cfRemote) *cfPlanEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	pol := cfWirePolicy(t, srv)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(pol)},
		&fakePolicies{pol: pol},
		&cfFakeTokens{cred: cfCred()},
	)
	actions := appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 13,
	}}
	svc := NewConfluencePublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("hello\n\nworld")}, remote,
		&cfFakeScopes{scope: cfScopeOK()})
	return &cfPlanEnv{svc: svc, pubs: pubs, store: store, fake: fake}
}

func cfPlanInput() PublishPlanInput {
	return PublishPlanInput{
		TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "Report",
		ParentPageID: "parent-1",
	}
}

func cfApprove(t *testing.T, env *cfPlanEnv, view PublishPlanView) {
	t.Helper()
	if err := env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest); err != nil {
		t.Fatal(err)
	}
}

func TestConfluenceFormPlanCreateBindsBaselineVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	view, err := env.svc.FormPlan(context.Background(), cfPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "create" || view.Destination != "parent-1" || view.ExpectedExternalVersion != "7" {
		t.Fatalf("plan shape: %+v", view)
	}
	row, err := env.store.FindAction(context.Background(), view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval || row.Risk != appconn.RiskWrite {
		t.Fatalf("prepared action row: %+v", row)
	}
	var snap struct {
		Parent  string `json:"parent"`
		Title   string `json:"title"`
		Storage string `json:"storage"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.Parent != "parent-1" {
		t.Fatalf("create snapshot must carry the approved parent: %s (%v)", row.ArgsSnapshot, err)
	}
	if snap.Storage != "<p>hello</p>\n<p>world</p>" {
		t.Fatalf("the snapshot must carry the derived storage body, not raw text: %s", snap.Storage)
	}
	pub, perr := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
	if perr != nil || pub.State != repoappconn.PublicationPlanned || pub.Provider != "confluence" ||
		pub.ArtifactVersionID != "ver-1" || pub.ExpectedVersion != "7" {
		t.Fatalf("planned publication must bind artifact + destination + version: %+v %v", pub, perr)
	}
}

func TestConfluenceFormPlanCreateParentOutOfScopeFailsClosed(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{}})
	in := cfPlanInput()
	in.ParentPageID = "parent-unlisted"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishDestinationOutOfScope) {
		t.Fatalf("unlisted parent must fail closed, got %v", err)
	}
}

func TestConfluenceFormPlanDestinationUnreadable(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{err: errors.New("boom")})
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishDestinationUnreadable) {
		t.Fatalf("unreadable destination must refuse plan formation, got %v", err)
	}
}

func TestConfluenceFormPlanUpdateRequiresPriorPublishedReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("update target must chain from a prior receipt, got %v", err)
	}
}

func TestConfluenceFormPlanUpdateBindsExpectedVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "update" || view.ExpectedExternalVersion != "3" {
		t.Fatalf("update plan must bind the read version: %+v", view)
	}
	row, _ := env.store.FindAction(context.Background(), view.ActionID)
	var snap struct {
		PageID          string `json:"page_id"`
		ExpectedVersion string `json:"expected_version"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "3" {
		t.Fatalf("update snapshot must carry page_id + expected_version: %s (%v)", row.ArgsSnapshot, err)
	}
}

func TestConfluenceFormPlanRejectsNotConfluenceConnection(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	// The shared service's scope port is NotionScopeSource; the confluence
	// shape rides the same adapter the constructor installs (R5-F12).
	env.svc.scopes = confluenceScopeAdapter{src: &cfFakeScopes{scope: func() ConfluenceConnectionScope {
		s := cfScopeOK()
		s.AppID = "notion"
		return s
	}()}}
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishInvalidInput) {
		t.Fatalf("a notion connection must be refused at the confluence endpoint, got %v", err)
	}
}

func TestConfluenceFormPlanRejectsBadInputs(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{}})
	cases := map[string]PublishPlanInput{
		"no title":      func() PublishPlanInput { in := cfPlanInput(); in.Title = ""; return in }(),
		"both targets":  func() PublishPlanInput { in := cfPlanInput(); in.PageID = "p"; return in }(),
		"no target":     func() PublishPlanInput { in := cfPlanInput(); in.ParentPageID = ""; return in }(),
		"no version id": func() PublishPlanInput { in := cfPlanInput(); in.ArtifactVersionID = ""; return in }(),
	}
	for name, in := range cases {
		if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishInvalidInput) {
			t.Fatalf("%s: want ErrPublishInvalidInput, got %v", name, err)
		}
	}
}

func TestConfluenceFormPlanRejectsEmptyArtifact(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	env.svc.content = &fakeContent{data: []byte("  \n ")}
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishEmptyContent) {
		t.Fatalf("an artifact with no publishable text must be refused, got %v", err)
	}
}

func TestConfluenceExecuteCreatePublishesAndSettlesReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	view, err := env.svc.FormPlan(context.Background(), cfPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionSucceeded || out.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("publish outcome: %+v", out)
	}
	if out.Receipt.ExternalID == "" || out.Receipt.ExternalVersion != "1" {
		t.Fatalf("receipt must carry external id + the version the publish produced: %+v", out.Receipt)
	}
}

func TestConfluenceExecuteUpdateConflictSettlesFailedReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	// External collaborator edits between approval and execute.
	fake.bump("page-9")
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Conflict || out.ActionState != appconn.ActionFailed {
		t.Fatalf("conflict outcome: %+v", out)
	}
	if out.Receipt.State != repoappconn.PublicationFailed {
		t.Fatalf("receipt must settle failed: %+v", out.Receipt)
	}
	fake.mu.Lock()
	puts := fake.puts
	fake.mu.Unlock()
	if puts != 0 {
		t.Fatalf("conflict must write nothing: %d puts", puts)
	}
}

func TestConfluenceReconcileResolvesUnknownWithoutRedispatch(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	// The PUT's reply is lost after the effect applied → unknown.
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionUnknown {
		t.Fatalf("lost reply must park unknown, got %s", out.ActionState)
	}
	// Blind re-publish is structurally refused (the store only claims
	// from authorized).
	if _, err := env.svc.Execute(context.Background(), 7, view.ActionID); !errors.Is(err, appconnectorsvc.ErrActionState) {
		t.Fatalf("re-publish while unknown must be refused, got %v", err)
	}
	fake.mu.Lock()
	putsBefore := fake.puts
	fake.mu.Unlock()
	rec, rerr := env.svc.Reconcile(context.Background(), 7, view.ActionID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if rec.ActionState != appconn.ActionSucceeded || rec.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("reconcile must resolve from the remote: %+v", rec)
	}
	fake.mu.Lock()
	putsAfter := fake.puts
	fake.mu.Unlock()
	if putsAfter != putsBefore {
		t.Fatalf("reconcile must not re-send: %d -> %d", putsBefore, putsAfter)
	}
}
