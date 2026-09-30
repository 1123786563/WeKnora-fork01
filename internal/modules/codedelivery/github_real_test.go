package codedelivery

import (
	"context"
	"fmt"
	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGitHubClientAgainstRealGitHub 需要 WEKNORA_GITHUB_TEST_TOKEN（经典
// PAT，repo 只读作用域）与 WEKNORA_GITHUB_TEST_REPO（owner/name，公开仓库）。
// 缺 env 即 skip——真实 GitHub 属 true external（blocked-env），不伪造。
func TestGitHubClientAgainstRealGitHub(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)
	client := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)(token, repo)
	ctx := context.Background()

	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real github: repo=%s default=%s login=%s", repo, info.DefaultBranch, login)
}

// realLoopConnections mirrors fixtureConnections (service_prepare_test.go:41)
// but hands out the REAL token from the environment — credentials are read
// from env only, never literals.
type realLoopConnections struct {
	db      *gorm.DB
	members map[string]bool
}

// realWriteCountingTransport records write methods and escaped paths only;
// request headers and query values (including credentials) are never retained.
type realWriteCountingTransport struct {
	base   http.RoundTripper
	mu     sync.Mutex
	writes map[string]int
}

func (t *realWriteCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		key := req.Method + " " + req.URL.EscapedPath()
		t.mu.Lock()
		t.writes[key]++
		t.mu.Unlock()
	}
	return t.base.RoundTrip(req)
}

func (t *realWriteCountingTransport) writeSnapshot() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]int, len(t.writes))
	for key, count := range t.writes {
		out[key] = count
	}
	return out
}

// appConnAdapter 把 realLoopConnections 适配为 appconnector 侧的
// ConnectionCredentialSource（两侧接口的 FindConnectionByID 返回类型不同）。
type appConnAdapter struct{ base *realLoopConnections }

func (a appConnAdapter) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	ident, err := a.base.FindConnectionByID(ctx, id)
	if err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: ident.ID, InstallationID: ident.InstallationID, Kind: ident.Kind,
		OwnerID: ident.OwnerID, State: ident.State, TenantID: ident.TenantID, AuthVersion: ident.AuthVersion,
	}, nil
}

func (s *realLoopConnections) FindConnectionByID(ctx context.Context, id string) (ConnectionIdentity, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return ConnectionIdentity{}, err
	}
	return ConnectionIdentity{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID,
		State:   row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *realLoopConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

func (s *realLoopConnections) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *realLoopConnections) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (s *realLoopConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

// realLoopBaselineSHA anchors the delivery on the default branch's CURRENT
// head (a real 40-hex commit sha the real API accepts).
func realLoopBaselineSHA(t *testing.T, client GitHubClient) string {
	t.Helper()
	info, err := client.Repository(context.Background())
	require.NoError(t, err)
	head, exists, err := client.BranchHead(context.Background(), info.DefaultBranch)
	require.NoError(t, err)
	require.True(t, exists, "the default branch must exist on the real repo")
	require.Len(t, head, 40)
	return head
}

// TestGitHubRealRecoveryLoopNoRepeatPush needs a PAT with write access to the
// dedicated WEKNORA_GITHUB_TEST_REPO repository. It creates a task branch and
// draft PR (weknora/task/<session>), so never point it at a production repo.
// All three opt-in environment variables are required; missing configuration
// is an honest blocked-env skip. Real GitHub cannot deterministically inject
// PR creation failure, so the pushed recovery branch is covered by the wire
// contract e2e; this loop covers a real delivery's delivered convergence,
// duplicate-dispatch rejection, and idempotent read-back.
func TestGitHubRealRecoveryLoopNoRepeatPush(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" || os.Getenv("WEKNORA_GITHUB_TEST_WRITABLE") != "1" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO/WEKNORA_GITHUB_TEST_WRITABLE not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)

	dsn := "file:" + filepath.Join(t.TempDir(), "real.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &deliveryrepo.DeliveryRow{},
	))
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	root := t.TempDir()
	sessionID := fmt.Sprintf("s-real-%d", time.Now().UnixNano())
	dir := filepath.Join(root, repo.Owner, repo.Name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# real loop\n"), 0o644))
	workspace, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)

	writeTransport := &realWriteCountingTransport{base: http.DefaultTransport, writes: map[string]int{}}
	httpClient := httpClientDefault()
	httpClient.Transport = writeTransport
	factory := NewGitHubClientFactory(httpClient, GitHubAPIBaseURL)
	client := factory(token, repo)
	baseline := realLoopBaselineSHA(t, client)
	info, err := client.Repository(context.Background())
	require.NoError(t, err)
	baselineTree, err := client.Tree(context.Background(), baseline)
	require.NoError(t, err)
	require.Len(t, baselineTree, 1, "dedicated writable test repo baseline must contain only README.md to prevent unrelated file deletions")
	require.Contains(t, baselineTree, "README.md", "dedicated writable test repo baseline must contain the file mirrored by the fixture")

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := &realLoopConnections{db: db, members: map[string]bool{"u1": true}}
	guard := appconnectorsvc.NewSubjectGuard(appConnAdapter{base: connections})
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guardAdapter{g: guard},
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStoreAdapter{s: actionStore}, Runs: fixtureRun{sessionID: sessionID},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcherAdapter{d: dispatcher}, dispatcherAdapter{d: dispatcher})
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: lifecycleAdapter{s: actions}, ActionRows: actionStoreAdapter{s: actionStore},
		Connections: connections, Creds: connections,
		GitHub: factory, Providers: installStoreAdapter{s: appconnectorrepo.NewInstallationStore(db)},
		Workspace: workspace, Runs: fixtureRun{sessionID: sessionID}, Dispatcher: dispatcher,
	})

	ctx := context.Background()
	view, err := svc.PrepareDelivery(ctx, PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo: repo, BaselineSHA: baseline, CommitMessage: "t25 real recovery loop", PRTitle: "t25 real recovery loop",
	})
	require.NoError(t, err)
	require.NoError(t, actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	view, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.PRURL)

	// Confirm the delivered receipt agrees with facts fetched from GitHub.
	remoteHead, exists, err := client.BranchHead(ctx, view.Branch)
	require.NoError(t, err)
	require.True(t, exists, "the task branch must exist remotely")
	require.Equal(t, view.CommitSHA, remoteHead)
	remoteHeadRef := repo.Owner + ":" + view.Branch
	remotePR, err := client.PullRequestForHead(ctx, remoteHeadRef, info.DefaultBranch)
	require.NoError(t, err)
	require.NotNil(t, remotePR, "the delivered draft PR must be visible remotely")
	require.Equal(t, view.PRNumber, remotePR.Number)
	require.Equal(t, view.PRURL, remotePR.URL)
	require.True(t, remotePR.Draft, "the delivered PR must remain a draft")

	// The duplicate dispatch must be refused by the state machine — no
	// second push leaves the process. Re-read the branch and PR afterward to
	// prove the remote receipts stay converged with the delivered record. The
	// transport retains only write method/path counts, never headers or tokens.
	writesBeforeDuplicate := writeTransport.writeSnapshot()
	_, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.ErrorIs(t, err, ErrDeliveryState)
	writesAfterDuplicate := writeTransport.writeSnapshot()
	require.Equal(t, writesBeforeDuplicate, writesAfterDuplicate, "a rejected duplicate dispatch must emit no provider write request")
	remoteHeadAfter, exists, err := client.BranchHead(ctx, view.Branch)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, remoteHead, remoteHeadAfter, "duplicate dispatch must not move or repush the task branch")
	remotePRAfter, err := client.PullRequestForHead(ctx, remoteHeadRef, info.DefaultBranch)
	require.NoError(t, err)
	require.NotNil(t, remotePRAfter)
	require.Equal(t, remotePR.Number, remotePRAfter.Number, "duplicate dispatch must not create another PR")
	require.Equal(t, remotePR.URL, remotePRAfter.URL, "duplicate dispatch must not change the PR URL")
	require.True(t, remotePRAfter.Draft, "the repeated remote PR read must still report draft")
	again, err := svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), again.State, "the read face stays idempotent after the refused duplicate")
	require.Equal(t, view.CommitSHA, again.CommitSHA)
	require.Equal(t, view.PRNumber, again.PRNumber)
}

func (a appConnAdapter) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return a.base.LoadCredential(ctx, c)
}
func (a appConnAdapter) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return a.base.MemberActive(ctx, tenantID, userID)
}
func (a appConnAdapter) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return a.base.TryAcquireRefreshLease(ctx, c, leaseID, until)
}

// guardAdapter 把 appconnectorsvc.A02Guard 适配为 A02Guard（subject 类型名不同、字段同构）。
type guardAdapter struct{ g appconnectorsvc.A02Guard }

func (a guardAdapter) Check(ctx context.Context, s ActionSubject, actionID string, authVersion int64) error {
	return a.g.Check(ctx, appconnector.OCSubject{TenantID: s.TenantID, ActorID: s.ActorID}, actionID, authVersion)
}

// actionStoreAdapter 把 appconnectorrepo.ActionStore 适配为 ActionStoreSource。
type actionStoreAdapter struct{ s *appconnectorrepo.ActionStore }

func (a actionStoreAdapter) FindAction(ctx context.Context, id string) (ActionRecord, error) {
	r, err := a.s.FindAction(ctx, id)
	if err != nil {
		return ActionRecord{}, err
	}
	return ActionRecord{
		ID: r.ID, TenantID: r.TenantID, ActorID: r.ActorID, ConnectionID: r.ConnectionID,
		AppVersion: r.AppVersion, Target: r.Target, Risk: r.Risk, ArgsDigest: r.ArgsDigest,
		State: r.State, Fence: r.Fence, ArgsSnapshot: r.ArgsSnapshot, AuthVersion: r.AuthVersion,
		DigestVersion: int(r.DigestVersion), ProviderResult: r.ProviderResult,
	}, nil
}

// runsAdapter 把 AgentRunStore 适配为 RunReader。
type runsAdapter struct{ r *repository.AgentRunStore }

func (a runsAdapter) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (RunIdentity, error) {
	run, err := a.r.GetOwnedRun(ctx, tenantID, ownerID, runID)
	if err != nil {
		return RunIdentity{}, err
	}
	return RunIdentity{SessionID: run.SessionID}, nil
}

// dispatcherAdapter 把 DeliveryDispatcher 适配为 appconnectorsvc.ActionDispatcher。
type dispatcherAdapter struct {
	d *DeliveryDispatcher
}

func (a dispatcherAdapter) Dispatch(ctx context.Context, s appconnectorsvc.ActionSnapshot, reservationID string) (appconnectorsvc.DispatchOutcome, error) {
	out, err := a.d.Dispatch(ctx, ActionSnapshot{
		ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target,
		AuthVersion: s.AuthVersion, Args: s.Args,
	}, reservationID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{Status: out.Status, ProviderResult: out.ProviderResult}, nil
}
func (a dispatcherAdapter) QueryProvider(ctx context.Context, s appconnectorsvc.ActionSnapshot, executionID string) (appconnectorsvc.DispatchOutcome, error) {
	out, err := a.d.QueryProvider(ctx, ActionSnapshot{
		ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target,
		AuthVersion: s.AuthVersion, Args: s.Args,
	}, executionID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{Status: out.Status, ProviderResult: out.ProviderResult}, nil
}

// lifecycleAdapter 把 appconnectorsvc.ActionService 适配为 ActionLifecycle。
type lifecycleAdapter struct {
	s *appconnectorsvc.ActionService
}

func (a lifecycleAdapter) Prepare(ctx context.Context, in ActionInput) (string, error) {
	return a.s.Prepare(ctx, appconnector.Action{
		TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Target: in.Target, Risk: in.Risk, Args: in.Args, AuthVersion: in.AuthVersion,
	})
}
func (a lifecycleAdapter) Execute(ctx context.Context, id string) error { return a.s.Execute(ctx, id) }
func (a lifecycleAdapter) ResolveUnknown(ctx context.Context, id string) error {
	return a.s.ResolveUnknown(ctx, id)
}

// installStoreAdapter 把 appconnectorrepo.InstallationStore 适配为 ProviderSource。
type installStoreAdapter struct {
	s *appconnectorrepo.InstallationStore
}

func (a installStoreAdapter) GetInstallationByID(ctx context.Context, tenantID uint64, id string) (ProviderInstallation, error) {
	inst, err := a.s.GetInstallationByID(ctx, tenantID, id)
	if err != nil {
		return ProviderInstallation{}, err
	}
	return ProviderInstallation{AppID: inst.AppID}, nil
}
