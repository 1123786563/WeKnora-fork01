package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/application/repository"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// fixture: the REAL repository stores plus the REAL sqlite twin migration
// 000044 (the tool binding table), so the suite exercises actual persistence
// semantics — composite PK, uniqueness, one-transaction action+binding —
// rather than AutoMigrate approximations.
// ---------------------------------------------------------------------------

const (
	toolBindingApp      = "github-oc"
	toolBindingVersion  = "1.0.0"
	toolBindingProvider = "github"
	toolBindingAction   = "github.search"
	toolBindingSchema   = "{\"type\":\"object\",\"properties\":{\"q\":{\"type\":\"string\"}},\"required\":[\"q\"],\"additionalProperties\":false}"
)

func openToolBindingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000&_foreign_keys=1"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// One pooled connection serializes transactions (SQLITE_LOCKED guard,
	// same convention as the catalog and action suites).
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&repoappconn.AppVersion{}, &repoappconn.InstallationRow{}, &repoappconn.ConnectionRow{},
		&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.PreAuthorizationRow{},
		&types.TenantMember{},
	); err != nil {
		t.Fatal(err)
	}
	// The REAL twin migrations, in order: 000041 creates the OC binding and
	// definition tables the catalog chain reads, 000044 the tool binding
	// table under test.
	for _, migration := range []string{
		"../../../../migrations/sqlite/000041_open_connector_bindings.up.sql",
		"../../../../migrations/sqlite/000044_open_connector_tool_bindings.up.sql",
	} {
		raw, err := os.ReadFile(migration)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// newToolBindingFixture wires the real chain over one sqlite database: the
// real stores, the real catalog chain (installation → connection → binding →
// published definition), an active tenant member, and the tool binding
// facade under test.
func newToolBindingFixture(t *testing.T) (*OCToolBindingService, *OCToolBindingStore, *gorm.DB, *repoappconn.OCStore, *repoappconn.InstallationStore) {
	t.Helper()
	db := openToolBindingDB(t)
	ctx := context.Background()
	oc := repoappconn.NewOCStore(db)
	inst := repoappconn.NewInstallationStore(db)
	if err := inst.ApplyInstallation(ctx, appconn.Installation{
		ID: "inst-7", AppID: toolBindingApp, Version: toolBindingVersion,
		State: appconn.InstallationActive, TenantID: 7,
	}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{
		ID: "c1", InstallationID: "inst-7", Kind: appconn.ConnectionKindPersonal,
		OwnerID: "user-1", CredentialRef: "cred/inst-7", State: appconn.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := oc.SaveBinding(ctx, appconn.OCBinding{
		TenantID: 7, ConnectionID: "c1", RuntimeID: "rt-1", Provider: toolBindingProvider,
		ExternalID: "ext-1", Alias: "alias-1", AuthVersion: 1, BindingVersion: 1,
		State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
	catalog := NewOCCatalog(oc, oc, inst, inst)
	def := appconn.OCDefinition{
		AppID: toolBindingApp, AppVersion: toolBindingVersion, ActionID: toolBindingAction,
		Provider: toolBindingProvider, InputSchema: json.RawMessage(toolBindingSchema),
		RequiredScopes: []string{"repo:read"}, Risk: OCRiskRead, Published: true,
	}
	if err := catalog.PublishOCDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&types.TenantMember{
		TenantID: 7, UserID: "user-1", Status: types.TenantMemberStatusActive, Role: types.TenantRoleOwner,
	}).Error; err != nil {
		t.Fatal(err)
	}
	bindings := NewOCToolBindingStore(db)
	svc := NewOCToolBindingService(repoappconn.NewActionStore(db), catalog, oc, bindings)
	return svc, bindings, db, oc, inst
}

func TestOCToolBindingPrepareIsIdempotentPerCall(t *testing.T) {
	svc, bindings, db, _, _ := newToolBindingFixture(t)
	subject := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	ctx := context.Background()

	first, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"weknora"}`))
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	second, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"weknora"}`))
	if err != nil {
		t.Fatalf("replay prepare: %v", err)
	}
	if first != second {
		t.Fatalf("same (tenant, session, tool call) must return the same action: %s vs %s", first, second)
	}
	var actionRows, bindingRows int64
	db.Model(&repoappconn.ActionRow{}).Count(&actionRows)
	db.Model(&OCToolBindingRow{}).Count(&bindingRows)
	if actionRows != 1 || bindingRows != 1 {
		t.Fatalf("idempotent replay must not create rows: actions=%d bindings=%d", actionRows, bindingRows)
	}
	got, found, err := bindings.FindBinding(ctx, 7, "sess-1", "call-1")
	if err != nil || !found {
		t.Fatalf("binding readback: found=%v err=%v", found, err)
	}
	if got.ActionID != first || got.ArgsDigest == "" {
		t.Fatalf("binding must persist the action id and args digest: %+v", got)
	}
	// The action itself is a real OC action: server-filled binding, digest
	// under the current generation, parked awaiting human approval.
	row, err := repoappconn.NewActionStore(db).FindAction(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval || row.TenantID != 7 || row.ActorID != "user-1" {
		t.Fatalf("unexpected action row: %+v", row)
	}
	if row.OCBindingJSON == "" || row.DigestVersion != appconn.CurrentDigestVersion {
		t.Fatalf("action must carry the OC execution binding at the current digest generation: %+v", row)
	}
}

func TestOCToolBindingChangedArgsConflict(t *testing.T) {
	svc, _, db, _, _ := newToolBindingFixture(t)
	subject := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	ctx := context.Background()
	first, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"weknora"}`))
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	// The exact same tool call id presenting DIFFERENT args is a conflict:
	// the persisted binding is immutable and must not be rebound.
	if _, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"other"}`)); !errors.Is(err, ErrOCToolArgsConflict) {
		t.Fatalf("changed args must conflict, got %v", err)
	}
	// The original binding survives untouched.
	again, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"weknora"}`))
	if err != nil || again != first {
		t.Fatalf("original binding must keep answering: %v %s", err, again)
	}
	var actionRows int64
	db.Model(&repoappconn.ActionRow{}).Count(&actionRows)
	if actionRows != 1 {
		t.Fatalf("conflict must not create a second action: %d", actionRows)
	}
}

func TestOCToolBindingRacingIdenticalCallsReadBackSingleWinner(t *testing.T) {
	svc, bindings, db, _, _ := newToolBindingFixture(t)
	const racers = 16
	subject := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	ctx := context.Background()

	ids := make([]string, racers)
	errs := make([]error, racers)
	var wg sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start.Wait()
			id, err := svc.PrepareForTool(ctx, subject, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"race"}`))
			ids[i], errs[i] = id, err
		}(i)
	}
	start.Done()
	wg.Wait()

	winner := ""
	for i := 0; i < racers; i++ {
		if errs[i] != nil {
			t.Fatalf("racer %d failed: %v", i, errs[i])
		}
		if winner == "" {
			winner = ids[i]
		} else if ids[i] != winner {
			t.Fatalf("racers disagree: %s vs %s", winner, ids[i])
		}
	}
	var actionRows, bindingRows int64
	db.Model(&repoappconn.ActionRow{}).Count(&actionRows)
	db.Model(&OCToolBindingRow{}).Count(&bindingRows)
	if actionRows != 1 || bindingRows != 1 {
		t.Fatalf("race must leave exactly one winner: actions=%d bindings=%d", actionRows, bindingRows)
	}
	got, found, err := bindings.FindBinding(ctx, 7, "sess-1", "call-1")
	if err != nil || !found || got.ActionID != winner {
		t.Fatalf("winner binding mismatch: found=%v err=%v want=%s got=%+v", found, err, winner, got)
	}
}

// The action row and the binding row land in ONE transaction: when the
// binding insert loses the uniqueness race, the action insert rolls back —
// no orphan action rows are left behind.
func TestOCToolBindingActionInsertIsAtomicWithBinding(t *testing.T) {
	_, bindings, db, _, _ := newToolBindingFixture(t)
	ctx := context.Background()
	winner := appconn.Action{
		ID: "ocact_winner", TenantID: 7, ActorID: "user-1", ConnectionID: "c1",
		Version: toolBindingVersion, Target: toolBindingAction, Risk: OCRiskRead,
		Args: json.RawMessage(`{"q":"x"}`), AuthVersion: 1, DigestVersion: appconn.CurrentDigestVersion,
	}
	if err := bindings.CreateWithAction(ctx, OCToolBinding{
		TenantID: 7, SessionID: "sess-1", ToolCallID: "call-1",
		ConnectionID: "c1", ActionName: toolBindingAction, ActionID: "ocact_winner",
		ArgsDigest: "digest-winner",
	}, winner, `{"q":"x"}`, "digest-winner", appconn.ActionAwaitingApproval); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	loser := winner
	loser.ID = "ocact_loser"
	err := bindings.CreateWithAction(ctx, OCToolBinding{
		TenantID: 7, SessionID: "sess-1", ToolCallID: "call-1",
		ConnectionID: "c1", ActionName: toolBindingAction, ActionID: "ocact_loser",
		ArgsDigest: "digest-loser",
	}, loser, `{"q":"x"}`, "digest-loser", appconn.ActionAwaitingApproval)
	if !errors.Is(err, ErrOCToolBindingExists) {
		t.Fatalf("second insert must lose the uniqueness race, got %v", err)
	}
	var actionRows int64
	db.Model(&repoappconn.ActionRow{}).Where("id = ?", "ocact_loser").Count(&actionRows)
	if actionRows != 0 {
		t.Fatalf("loser action insert must roll back with the binding conflict, found %d", actionRows)
	}
}

func TestOCToolBindingStatusIsCrossSpaceInvisible(t *testing.T) {
	svc, _, _, _, _ := newToolBindingFixture(t)
	ctx := context.Background()
	owner := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	id, err := svc.PrepareForTool(ctx, owner, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"secret"}`))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	state, err := svc.StatusForTool(ctx, owner, id)
	if err != nil || state != appconn.ActionAwaitingApproval {
		t.Fatalf("owner status: state=%s err=%v", state, err)
	}
	// Another tenant asking for the same action id learns NOTHING — not even
	// that the row exists.
	foreign := appconn.OCSubject{TenantID: 8, ActorID: "user-8"}
	if _, err := svc.StatusForTool(ctx, foreign, id); !errors.Is(err, repoappconn.ErrActionNotFound) {
		t.Fatalf("cross-space status must read as not-found, got %v", err)
	}
	// A different tool call key is an independent binding even for another
	// actor of the same tenant; the SAME key with identical args keeps
	// answering with the SAME action regardless of the presenting actor
	// (the key is the identity the approval bound).
	same, err := svc.PrepareForTool(ctx, appconn.OCSubject{TenantID: 7, ActorID: "user-2"}, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"secret"}`))
	if err != nil || same != id {
		t.Fatalf("same key + same args must return the bound action: %v %s", err, same)
	}
}

func TestOCToolBindingRejectsMissingIdentity(t *testing.T) {
	svc, _, _, _, _ := newToolBindingFixture(t)
	ctx := context.Background()
	if _, err := svc.PrepareForTool(ctx, appconn.OCSubject{}, "sess-1", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"x"}`)); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("missing subject must be rejected, got %v", err)
	}
	subject := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	if _, err := svc.PrepareForTool(ctx, subject, "", "call-1", "c1", toolBindingAction, json.RawMessage(`{"q":"x"}`)); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("missing session must be rejected, got %v", err)
	}
	if _, err := svc.PrepareForTool(ctx, subject, "sess-1", "", "c1", toolBindingAction, json.RawMessage(`{"q":"x"}`)); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("missing tool call must be rejected, got %v", err)
	}
}

// countingDispatcher records every outbound POST the ActionService makes.
type countingDispatcher struct {
	mu     sync.Mutex
	posts  int
	bodies []string
}

func (d *countingDispatcher) Dispatch(_ context.Context, snap ActionSnapshot, _ string) (DispatchOutcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.posts++
	d.bodies = append(d.bodies, string(snap.Args))
	return DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: fmt.Sprintf("ok-%d", d.posts)}, nil
}

func (d *countingDispatcher) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.posts
}

func isOCWait(err error) bool {
	var wait *agentruntime.OCActionWaitError
	return errors.As(err, &wait)
}

// The E2E single-POST discipline: a REAL ToolRegistry mounts the tool over
// the REAL facade backed by the REAL ActionService; the only external write
// is the single dispatch of the human-approved action. Tool-layer replays
// (recovery), changed args, and revocation each cause NO duplicate external
// writes.
func TestOCToolE2ESinglePostDiscipline(t *testing.T) {
	svcFacade, _, db, oc, inst := newToolBindingFixture(t)
	dispenser := &countingDispatcher{}
	credSrc := repository.NewMCPOAuthBindingStore(db)
	guard := NewOCSubjectGuard(credSrc, NewInstallationStateSource(inst), nil, oc)
	actions := NewActionService(repoappconn.NewActionStore(db), guard, nil, dispenser, nil)

	registry := tools.NewToolRegistry()
	registry.RegisterTool(tools.NewAppConnectorTool(svcFacade))

	tenant := uint64(7)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenant)
	execCtx := tools.WithToolExecContext(ctx, &tools.ToolExecContext{
		UserID: "user-1", SessionID: "sess-e2e", ToolCallID: "call-e2e",
	})
	call := json.RawMessage(fmt.Sprintf(`{"connection_id":"c1","action_id":%q,"input":{"q":"hello"}}`, toolBindingAction))

	// 1. First tool call: the action is prepared and parked awaiting human
	//    approval. No external write happened — the model cannot approve.
	_, err := registry.ExecuteTool(execCtx, "app_connector", call)
	if err == nil || !isOCWait(err) {
		t.Fatalf("awaiting approval must surface as the OC wait error, got %v", err)
	}
	if got := dispenser.count(); got != 0 {
		t.Fatalf("prepare must not dispatch, posts=%d", got)
	}

	// The binding idempotently resolves to the prepared action.
	subject := appconn.OCSubject{TenantID: 7, ActorID: "user-1"}
	actionID, err := svcFacade.PrepareForTool(context.Background(), subject, "sess-e2e", "call-e2e", "c1", toolBindingAction, json.RawMessage(`{"q":"hello"}`))
	if err != nil {
		t.Fatalf("binding lookup: %v", err)
	}

	// 2. The HUMAN approves through the actions surface (out of band for the
	//    agent), and the approved action executes exactly once.
	row, err := repoappconn.NewActionStore(db).FindAction(context.Background(), actionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := actions.Approve(context.Background(), row.ID, "user-1", row.ArgsDigest); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := actions.Execute(context.Background(), row.ID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := dispenser.count(); got != 1 {
		t.Fatalf("approved action must dispatch exactly once, posts=%d", got)
	}

	// 3. Recovery replay of the SAME tool call: the binding digests match, the
	//    same action is resumed read-only, no new external write.
	result, err := registry.ExecuteTool(execCtx, "app_connector", call)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !result.Success || result.Data["state"] != appconn.ActionSucceeded {
		t.Fatalf("replay must read back the recorded outcome: %+v", result)
	}
	if got := dispenser.count(); got != 1 {
		t.Fatalf("replay must not re-dispatch, posts=%d", got)
	}

	// 4. Changed args under the same tool call id: conflict, no external write.
	changed := json.RawMessage(fmt.Sprintf(`{"connection_id":"c1","action_id":%q,"input":{"q":"mutated"}}`, toolBindingAction))
	if _, err := registry.ExecuteTool(execCtx, "app_connector", changed); err == nil {
		t.Fatal("changed args must conflict")
	}
	if got := dispenser.count(); got != 1 {
		t.Fatalf("changed args must not dispatch, posts=%d", got)
	}

	// 5. Revocation: the binding flips revoked; a NEW tool call cannot even
	//    prepare, and the recorded outcome of the earlier call stays the only
	//    external write.
	if err := oc.SaveBinding(context.Background(), appconn.OCBinding{
		TenantID: 7, ConnectionID: "c1", RuntimeID: "rt-1", Provider: toolBindingProvider,
		ExternalID: "ext-1", Alias: "alias-1", AuthVersion: 2, BindingVersion: 2,
		State: appconn.OCBindingRevoked,
	}); err != nil {
		t.Fatal(err)
	}
	revokedCtx := tools.WithToolExecContext(ctx, &tools.ToolExecContext{
		UserID: "user-1", SessionID: "sess-e2e", ToolCallID: "call-after-revoke",
	})
	if _, err := registry.ExecuteTool(revokedCtx, "app_connector", call); err == nil {
		t.Fatal("revoked binding must refuse new prepares")
	}
	if got := dispenser.count(); got != 1 {
		t.Fatalf("revocation must not cause new external writes, posts=%d", got)
	}
}
