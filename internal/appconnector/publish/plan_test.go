package publish

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- scenario doubles for the two non-DB ports ----

type fakeArtifacts struct {
	version repository.ArtifactVersion
	err     error
}

func (f *fakeArtifacts) ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) {
	return f.version, f.err
}

type fakeContent struct{ data []byte }

func (f *fakeContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	return f.data, nil
}

type fakeRemote struct {
	versions map[string]string
	err      error
}

func (f *fakeRemote) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.versions[pageID], nil
}

type passGuard struct{}

func (passGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

func openPlanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.PreAuthorizationRow{}, &repoappconn.PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type planEnv struct {
	svc   *NotionPublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
	fake  *bridgeFakeNotion
}

func newPlanEnv(t *testing.T, fake *bridgeFakeNotion, remote *fakeRemote) *planEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	// The dedicated publish ActionService: the bridge is BOTH dispatcher
	// and unknown resolver — the production composition (container wires
	// the same shape).
	pol := loopbackPolicyFor(t, fake)
	bridge := NewNotionBridge(
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}},
		&fakePolicies{pol: pol},
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
	}}
	svc := NewNotionPublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("hello\n\nworld")}, remote,
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}})
	return &planEnv{svc: svc, pubs: pubs, store: store, fake: fake}
}

func loopbackPolicyFor(t *testing.T, fake *bridgeFakeNotion) appconn.HTTPPolicy {
	t.Helper()
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	return appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network},
	}
}

func planInput() PublishPlanInput {
	return PublishPlanInput{
		TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "Report",
		ParentPageID: "parent-1",
	}
}

func approvePlan(t *testing.T, env *planEnv, view PublishPlanView) {
	t.Helper()
	if err := env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest); err != nil {
		t.Fatal(err)
	}
}

func TestFormPlanCreateBindsBaselineVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("parent-1", "root", "Parent")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "2026-09-24T08:00:00.000Z"}})
	view, err := env.svc.FormPlan(context.Background(), planInput())
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "create" || view.Destination != "parent-1" {
		t.Fatalf("plan shape: %+v", view)
	}
	// AC1 baseline: the plan records the external version it read.
	if view.ExpectedExternalVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("baseline version must be bound: %+v", view)
	}
	row, err := env.store.FindAction(context.Background(), view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval || row.Risk != appconn.RiskWrite {
		t.Fatalf("prepared action row: %+v", row)
	}
	var snap struct {
		Parent string `json:"parent"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.Parent != "parent-1" {
		t.Fatalf("create snapshot must carry the approved parent: %s (%v)", row.ArgsSnapshot, err)
	}
	pub, perr := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
	if perr != nil || pub.State != repoappconn.PublicationPlanned || pub.ArtifactVersionID != "ver-1" ||
		pub.ExpectedVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("planned publication must bind artifact + destination + version: %+v %v", pub, perr)
	}
}

func TestFormPlanCreateParentOutOfScopeFailsClosed(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{}})
	in := planInput()
	in.ParentPageID = "parent-unlisted"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishDestinationOutOfScope) {
		t.Fatalf("unlisted parent must fail closed, got %v", err)
	}
}

func TestFormPlanDestinationUnreadable(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{err: errors.New("boom")})
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishDestinationUnreadable) {
		t.Fatalf("unreadable destination must refuse plan formation, got %v", err)
	}
}

func TestFormPlanUpdateRequiresPriorPublishedReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "v"}})
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("update target must chain from a prior receipt, got %v", err)
	}
}

func TestFormPlanUpdateBindsExpectedVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}})
	// The prior published receipt that authorizes this target.
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "update" || view.ExpectedExternalVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("update plan must bind the read version: %+v", view)
	}
	row, _ := env.store.FindAction(context.Background(), view.ActionID)
	var snap struct {
		PageID          string `json:"page_id"`
		ExpectedVersion string `json:"expected_version"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("update snapshot must carry page_id + expected_version: %s (%v)", row.ArgsSnapshot, err)
	}
}

func TestExecuteCreatePublishesAndSettlesReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("parent-1", "root", "Parent")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "2026-09-24T09:00:00.000Z"}})
	view, err := env.svc.FormPlan(context.Background(), planInput())
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionSucceeded || out.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("publish outcome: %+v", out)
	}
	if out.Receipt.ExternalID == "" || out.Receipt.ExternalVersion == "" {
		t.Fatalf("receipt must carry external id + version: %+v", out.Receipt)
	}
}

func TestExecuteUpdateConflictSettlesFailedReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	remote := &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}}
	env := newPlanEnv(t, fake, remote)
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	// External collaborator edits between approval and execute — the
	// bridge's pre-read (through the REAL fake server) observes the drift.
	fake.touch("page-9", "2026-09-24T10:30:00.000Z")
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
	if fake.patchCalls != 0 {
		t.Fatalf("conflict must write nothing: %d patches", fake.patchCalls)
	}
}

func TestPublishReconcileResolvesUnknownWithoutRedispatch(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	// The append's reply is lost after the effect applied → unknown.
	fake.mu.Lock()
	fake.dropNextAppend = true
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
	appendsBefore := fake.appendCalls
	rec, rerr := env.svc.Reconcile(context.Background(), 7, view.ActionID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if rec.ActionState != appconn.ActionSucceeded || rec.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("reconcile must resolve from the remote: %+v", rec)
	}
	if fake.appendCalls != appendsBefore {
		t.Fatalf("reconcile must not re-send: %d -> %d", appendsBefore, fake.appendCalls)
	}
}

func TestFormPlanRejectsBadInputs(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{}})
	cases := map[string]PublishPlanInput{
		"no title":      func() PublishPlanInput { in := planInput(); in.Title = ""; return in }(),
		"both targets":  func() PublishPlanInput { in := planInput(); in.PageID = "p"; return in }(),
		"no target":     func() PublishPlanInput { in := planInput(); in.ParentPageID = ""; return in }(),
		"no version id": func() PublishPlanInput { in := planInput(); in.ArtifactVersionID = ""; return in }(),
	}
	for name, in := range cases {
		if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishInvalidInput) {
			t.Fatalf("%s: want ErrPublishInvalidInput, got %v", name, err)
		}
	}
}

func TestFormPlanRejectsUnsupportedArtifact(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "v"}})
	env.svc.artifacts = &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-2", SessionID: "sess-1", MIME: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		ScanState: repository.ArtifactScanReady, Size: 10, Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}}
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishUnsupportedArtifact) {
		t.Fatalf("binary artifact must be refused, got %v", err)
	}
}

func TestFormPlanRejectsNotReadyArtifact(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "v"}})
	env.svc.artifacts = &fakeArtifacts{err: errors.New("not found")}
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishArtifactNotReady) {
		t.Fatalf("unreadable version must be refused, got %v", err)
	}
}
