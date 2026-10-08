package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- scenario doubles (self-contained). The publish seam's own tests
// double the Notion WIRE; plan-level unit tests script the DISPATCH
// outcomes directly, so no Notion wire is needed here. ----

type stubArtifacts struct{}

func (stubArtifacts) ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) {
	return repository.ArtifactVersion{ID: "ver-1", Digest: "d1", MIME: "text/plain", Size: 26}, nil
}

type stubContent struct{}

func (stubContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	return []byte("第一段。\n\n第二段。"), nil
}

type stubRemote struct{ versions map[string]string }

func (f stubRemote) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	return f.versions[pageID], nil
}

type stubScopes struct{}

func (stubScopes) NotionScope(ctx context.Context, connectionID string) (publish.NotionConnectionScope, error) {
	return publish.NotionConnectionScope{
		AppID: "notion", ConnectionKind: "personal", OwnerID: "user-a", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, InsertCapability: true,
	}, nil
}

type stubGuard struct{}

func (stubGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

// scriptedDispatcher answers Dispatch by snap.ID and COUNTS every call —
// AC2's zero-re-dispatch assertions count here.
type scriptedDispatcher struct {
	outcomes map[string]appconnectorsvc.DispatchOutcome
	errs     map[string]error
	calls    map[string]int
}

func newScriptedDispatcher() *scriptedDispatcher {
	return &scriptedDispatcher{
		outcomes: map[string]appconnectorsvc.DispatchOutcome{},
		errs:     map[string]error{},
		calls:    map[string]int{},
	}
}

func (d *scriptedDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	d.calls[snap.ID]++
	if err := d.errs[snap.ID]; err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return d.outcomes[snap.ID], nil
}

// okReceipt is a parseable succeeded ProviderResult (NotionPageReceipt
// shape) so the publication settles to published.
func okReceipt(id string) string {
	return `{"object":"page","id":"` + id + `","last_edited_time":"2026-09-25T08:00:00.000Z"}`
}

func openPlanSvcDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{},
		&repoappconn.PreAuthorizationRow{}, &repoappconn.PublicationRow{},
		&repoappconn.ActionPlanRow{}, &repoappconn.ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type planSvcEnv struct {
	db       *gorm.DB
	svc      *Service
	store    *repoappconn.ActionStore
	plans    *repoappconn.PlanStore
	pubs     *repoappconn.PublicationStore
	dispatch *scriptedDispatcher
}

func newPlanSvcEnv(t *testing.T) *planSvcEnv {
	t.Helper()
	db := openPlanSvcDB(t)
	store := repoappconn.NewActionStore(db)
	plans := repoappconn.NewPlanStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	dispatch := newScriptedDispatcher()
	actions := appconnectorsvc.NewActionService(store, stubGuard{}, nil, dispatch, nil)
	publishSvc := publish.NewNotionPublishService(actions, store, pubs,
		stubArtifacts{}, stubContent{},
		stubRemote{versions: map[string]string{
			"parent-1": "2026-09-24T08:00:00.000Z",
			"page-9":   "2026-09-24T09:00:00.000Z",
		}},
		stubScopes{})
	svc := NewService(plans, store, actions, publishSvc)
	return &planSvcEnv{db: db, svc: svc, store: store, plans: plans, pubs: pubs, dispatch: dispatch}
}

func item(title string) ItemInput {
	return ItemInput{ConnectionID: "conn-notion", SessionID: "sess-1",
		ArtifactVersionID: "ver-1", Title: title, ParentPageID: "parent-1"}
}

func (e *planSvcEnv) formTwo(t *testing.T) PlanView {
	t.Helper()
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B")}})
	if err != nil {
		t.Fatal(err)
	}
	return pv
}

// TestPlanDigestBindsSetOrderAndContent pins the AC1 anchor at the pure
// function level: the digest changes when the set, the order, any item's
// content digest, the connection or the actor changes — and only then.
func TestPlanDigestBindsSetOrderAndContent(t *testing.T) {
	base := []DigestItem{
		{Seq: 1, ActionID: "act-1", ActionDigest: "d1", Connection: "conn-notion", Target: "parent-1", Risk: "write"},
		{Seq: 2, ActionID: "act-2", ActionDigest: "d2", Connection: "conn-notion", Target: "parent-1", Risk: "write"},
	}
	want, err := PlanDigest(7, "user-a", base)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := PlanDigest(7, "user-a", base); again != want {
		t.Fatal("same material must hash identically")
	}
	if actor, _ := PlanDigest(7, "user-b", base); actor == want {
		t.Fatal("actor identity must bind the digest")
	}
	reordered := []DigestItem{base[1], base[0]}
	if re, _ := PlanDigest(7, "user-a", reordered); re == want {
		t.Fatal("order must bind the digest（有序外部副作用）")
	}
	dropped := []DigestItem{base[0]}
	if fewer, _ := PlanDigest(7, "user-a", dropped); fewer == want {
		t.Fatal("the set must bind the digest（操作集合变化失效）")
	}
	edited := []DigestItem{base[0], base[1]}
	edited[1].ActionDigest = "d2-edited"
	if ed, _ := PlanDigest(7, "user-a", edited); ed == want {
		t.Fatal("any item's content digest must bind the plan digest（内容变化失效）")
	}
	reconn := []DigestItem{base[0], base[1]}
	reconn[0].Connection = "conn-other"
	if rc, _ := PlanDigest(7, "user-a", reconn); rc == want {
		t.Fatal("connection must bind the digest（连接变化失效）")
	}
	retarget := []DigestItem{base[0], base[1]}
	retarget[0].Target = "parent-2"
	if rt, _ := PlanDigest(7, "user-a", retarget); rt == want {
		t.Fatal("target must bind the digest（目标变化失效）")
	}
	if _, err := PlanDigest(7, "user-a", nil); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("zero-item plan must be refused, got %v", err)
	}
}

// TestPlanFormBuildsOrderedDigestBoundPlan: formation runs every item
// through the single-action publish seam, records each planned
// publication row, and returns the plan digest bound to the
// AUTHORITATIVE stored rows.
func TestPlanFormBuildsOrderedDigestBoundPlan(t *testing.T) {
	e := newPlanSvcEnv(t)
	ctx := context.Background()
	pv := e.formTwo(t)
	if pv.State != PlanStateAwaitingApproval || len(pv.Items) != 2 {
		t.Fatalf("formed plan: %+v", pv)
	}
	if pv.Items[0].Seq != 1 || pv.Items[1].Seq != 2 || pv.Items[0].ActionID == pv.Items[1].ActionID {
		t.Fatalf("items must be distinct and ordered: %+v", pv.Items)
	}
	if pv.Items[0].Digest == pv.Items[1].Digest {
		t.Fatal("different titles must produce different action digests")
	}
	if pv.Items[0].Mode != "create" || pv.Items[0].Destination != "parent-1" || pv.Items[0].ExpectedExternalVersion == "" {
		t.Fatalf("per-item view must carry the #48 formation facts: %+v", pv.Items[0])
	}
	// The plan digest binds the stored rows — recompute from the store.
	items, err := e.plans.ListPlanItems(ctx, 7, pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	var digests []DigestItem
	for i, it := range items {
		row, ferr := e.store.FindAction(ctx, it.ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		digests = append(digests, DigestItem{Seq: i + 1, ActionID: row.ID,
			ActionDigest: row.ArgsDigest, Connection: row.ConnectionID,
			Target: row.Target, Risk: row.Risk})
	}
	recomputed, err := PlanDigest(7, "user-a", digests)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != pv.Digest {
		t.Fatalf("plan digest must bind the stored rows: %s != %s", recomputed, pv.Digest)
	}
	// Every item recorded its planned publication row（逐项持久记录）.
	for _, it := range pv.Items {
		if _, err := e.pubs.FindByAction(ctx, 7, it.ActionID); err != nil {
			t.Fatalf("item %s must have its planned publication row: %v", it.ActionID, err)
		}
	}
	// Every item's action is parked awaiting_approval: nothing is
	// approved or dispatched by formation.
	for _, it := range pv.Items {
		row, ferr := e.store.FindAction(ctx, it.ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		if row.State != appconn.ActionAwaitingApproval {
			t.Fatalf("formation must never authorize: %s = %s", it.ActionID, row.State)
		}
	}
	if len(e.dispatch.calls) != 0 {
		t.Fatalf("formation must not dispatch: %v", e.dispatch.calls)
	}
}

// TestPlanFormMidItemFailureLeavesNoPlanRow: a failing item (an update
// destination with no prior publication receipt → the #48
// ErrPublishUpdateTargetNotPublished authority rule) aborts formation
// AFTER earlier items were prepared — the orphans stay awaiting_approval
// (harmless, TTL expiry) and NO plan row exists, so nothing can ever be
// approved or dispatched.
func TestPlanFormMidItemFailureLeavesNoPlanRow(t *testing.T) {
	e := newPlanSvcEnv(t)
	_, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B"), {
			// An update destination the tenant never published through
			// this connection — refused by LatestPublishedByDestination.
			ConnectionID: "conn-notion", SessionID: "sess-1",
			ArtifactVersionID: "ver-1", Title: "Doc C", PageID: "page-404",
		}}})
	if !errors.Is(err, publish.ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("the unpublished update target must abort formation: %v", err)
	}
	if err == nil {
		t.Fatal("the unreadable destination must abort formation")
	}
	// NO plan row or plan item may exist for this tenant — counted
	// straight off the tables. (A lookup of an arbitrary MISSING plan id
	// returns zero rows even if the failed formation had leaked one, so
	// an id-based probe can never go red; the raw count is the assertion
	// that actually pins the fail-closed property.)
	var planCount, itemCount int64
	if err := e.db.Model(&repoappconn.ActionPlanRow{}).
		Where("tenant_id = ?", uint64(7)).Count(&planCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := e.db.Model(&repoappconn.ActionPlanItemRow{}).
		Where("tenant_id = ?", uint64(7)).Count(&itemCount).Error; err != nil {
		t.Fatal(err)
	}
	if planCount != 0 || itemCount != 0 {
		t.Fatalf("a failed formation must leave NO plan rows/items, got plans=%d items=%d", planCount, itemCount)
	}
}

func TestPlanFormRejectsInvalidInput(t *testing.T) {
	e := newPlanSvcEnv(t)
	if _, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a"}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("zero-item plan refused, got %v", err)
	}
	if _, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 0, ActorID: "user-a", Items: []ItemInput{item("A")}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("missing tenant refused, got %v", err)
	}
}

func (e *planSvcEnv) approveAll(t *testing.T, pv PlanView) {
	t.Helper()
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a", ApproveInput{Digest: pv.Digest}); err != nil {
		t.Fatal(err)
	}
}

// TestPlanApproveWholeApprovesEveryIncludedItem: the whole-plan decision
// approves every included item with its own digest-bound A03 approval.
func TestPlanApproveWholeApprovesEveryIncludedItem(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.approveAll(t, pv)
	for _, it := range pv.Items {
		row, err := e.store.FindAction(context.Background(), it.ActionID)
		if err != nil {
			t.Fatal(err)
		}
		if row.State != appconn.ActionAuthorized {
			t.Fatalf("included item %s must be authorized, got %s", it.ActionID, row.State)
		}
	}
	got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
	if got.State != PlanStateAuthorized || got.ApprovedBy != "user-a" || got.ApprovedAt == nil {
		t.Fatalf("plan approval must be recorded: %+v", got)
	}
}

// TestPlanApproveExcludesItemNeverApprovesIt (排除单项): the excluded
// item stays awaiting_approval forever and can never be dispatched by
// this plan; the exclusion is recorded on the plan row.
func TestPlanApproveExcludesItemNeverApprovesIt(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); err != nil {
		t.Fatal(err)
	}
	first, _ := e.store.FindAction(context.Background(), pv.Items[0].ActionID)
	second, _ := e.store.FindAction(context.Background(), pv.Items[1].ActionID)
	if first.State != appconn.ActionAuthorized {
		t.Fatalf("included item must be authorized, got %s", first.State)
	}
	if second.State != appconn.ActionAwaitingApproval {
		t.Fatalf("excluded item must stay awaiting_approval, got %s", second.State)
	}
	got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
	if got.ExcludedJSON != "[2]" {
		t.Fatalf("exclusion must be recorded on the plan row: %q", got.ExcludedJSON)
	}
}

// TestPlanApproveRejectsForeignDigest is the AC1 approval-side anchor: a
// digest issued for DIFFERENT plan content (here: another plan's digest —
// 计划内容变化) authorizes nothing, and a refusal moves nothing.
func TestPlanApproveRejectsForeignDigest(t *testing.T) {
	e := newPlanSvcEnv(t)
	p1 := e.formTwo(t)
	// A second plan over the same connection/version but CHANGED content
	// (different titles → different items → different digests).
	p2, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A v2"), item("Doc B v2")}})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Digest == p2.Digest {
		t.Fatal("计划内容变化必须产生新 plan digest")
	}
	// The OLD digest must not approve the NEW content — the plan-level
	// mirror of TestApprovalLifecycleHappyPath's per-action refusal.
	if _, err := e.svc.Approve(context.Background(), 7, p2.ID, "user-a", ApproveInput{Digest: p1.Digest}); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("old plan digest approved new content: %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, p1.ID, "user-a", ApproveInput{Digest: "deadbeef"}); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("wrong digest accepted: %v", err)
	}
	for _, pv := range []PlanView{p1, p2} {
		got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
		if got.State != PlanStateAwaitingApproval {
			t.Fatalf("refused approve must not move state: %s", got.State)
		}
	}
}

func TestPlanApproveRejectsBadExclusions(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{3}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("out-of-range exclude seq refused, got %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{1, 1}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("duplicate exclude seq refused, got %v", err)
	}
}

// TestPlanApproveConcurrentDistinctExclusionsSingleWinner: the store CAS
// pins the exclusion-set freeze, so under TRUE concurrency two approvals
// carrying DIFFERENT exclusion sets cannot both win — the first writer's
// decision stands, the racing writer is refused, and the per-item outcome
// always matches the recorded decision (the excluded item is never
// approved, whoever writes last).
func TestPlanApproveConcurrentDistinctExclusionsSingleWinner(t *testing.T) {
	ctx := context.Background()
	for round := 0; round < 20; round++ {
		e := newPlanSvcEnv(t)
		pv := e.formTwo(t)
		errs := make(chan error, 2)
		for _, seqs := range [][]int{nil, {2}} {
			go func(seqs []int) {
				_, err := e.svc.Approve(ctx, 7, pv.ID, "user-a", ApproveInput{Digest: pv.Digest, ExcludeSeqs: seqs})
				errs <- err
			}(seqs)
		}
		wins := 0
		for i := 0; i < 2; i++ {
			err := <-errs
			if err == nil {
				wins++
				continue
			}
			if !errors.Is(err, ErrPlanState) && !errors.Is(err, repoappconn.ErrPlanState) {
				t.Fatalf("round %d: racing approve must fail with a plan-state error, got %v", round, err)
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: exactly one concurrent approval must win, got %d", round, wins)
		}
		final, err := e.plans.FindPlan(ctx, 7, pv.ID)
		if err != nil {
			t.Fatal(err)
		}
		if final.ExcludedJSON != "[]" && final.ExcludedJSON != "[2]" {
			t.Fatalf("round %d: recorded set must be one of the contenders: %q", round, final.ExcludedJSON)
		}
		first, ferr := e.store.FindAction(ctx, pv.Items[0].ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		second, ferr := e.store.FindAction(ctx, pv.Items[1].ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		if final.ExcludedJSON == "[2]" {
			if first.State != appconn.ActionAuthorized || second.State != appconn.ActionAwaitingApproval {
				t.Fatalf("round %d: the excluded item 2 must stay awaiting_approval: %s/%s", round, first.State, second.State)
			}
		} else {
			if first.State != appconn.ActionAuthorized || second.State != appconn.ActionAuthorized {
				t.Fatalf("round %d: both included items must be authorized: %s/%s", round, first.State, second.State)
			}
		}
	}
}

// TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts: one pass
// runs every included authorized item in seq order and each item
// settles its own published receipt.
func TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.dispatch.outcomes[pv.Items[1].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-b")}
	e.approveAll(t, pv)
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[0].Seq != 1 || out.Items[1].Seq != 2 {
		t.Fatalf("outcomes must be ordered: %+v", out.Items)
	}
	for i, oc := range out.Items {
		if oc.Disposition != ItemExecuted || oc.ActionState != appconn.ActionSucceeded {
			t.Fatalf("item %d must execute to success: %+v", i+1, oc)
		}
		if oc.Publication.State != "published" || oc.Publication.ExternalID == "" || oc.Publication.ExternalVersion == "" {
			t.Fatalf("receipt must settle published with external version: %+v", oc.Publication)
		}
	}
	if e.dispatch.calls[pv.Items[0].ActionID] != 1 || e.dispatch.calls[pv.Items[1].ActionID] != 1 {
		t.Fatalf("each item dispatched exactly once: %v", e.dispatch.calls)
	}
}

// TestPlanExecuteSkipsConfirmedOutcomesOnResume is the AC2 core: a
// second pass over a partially-successful plan re-dispatches NOTHING
// already confirmed (succeeded/failed/unknown) — the dispatch counts
// prove it.
func TestPlanExecuteSkipsConfirmedOutcomesOnResume(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B"), item("Doc C")}})
	if err != nil {
		t.Fatal(err)
	}
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.dispatch.outcomes[pv.Items[1].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionFailed, ProviderResult: publish.PublishVersionConflictResult + ": expected v1 remote v2"}
	e.dispatch.errs[pv.Items[2].ActionID] = errors.New("transport lost")
	e.approveAll(t, pv)
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].Disposition != ItemExecuted || out.Items[0].ActionState != appconn.ActionSucceeded {
		t.Fatalf("item 1: %+v", out.Items[0])
	}
	if out.Items[1].Disposition != ItemExecuted || out.Items[1].ActionState != appconn.ActionFailed || !out.Items[1].Conflict {
		t.Fatalf("item 2 must record the conflict failure: %+v", out.Items[1])
	}
	if out.Items[2].Disposition != ItemExecuted || out.Items[2].ActionState != appconn.ActionUnknown {
		t.Fatalf("item 3 must park unknown: %+v", out.Items[2])
	}
	// Resume with the SAME digest: zero new dispatches anywhere.
	out2, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out2.Items[0].Disposition != ItemSkippedSucceeded {
		t.Fatalf("AC2: succeeded item must be skipped, got %+v", out2.Items[0])
	}
	if out2.Items[1].Disposition != ItemSettled || out2.Items[1].ActionState != appconn.ActionFailed {
		t.Fatalf("failed item must stay settled, never re-dispatched: %+v", out2.Items[1])
	}
	if out2.Items[2].Disposition != ItemSettled || out2.Items[2].ActionState != appconn.ActionUnknown {
		t.Fatalf("unknown item must stay parked: %+v", out2.Items[2])
	}
	for _, it := range pv.Items {
		if e.dispatch.calls[it.ActionID] != 1 {
			t.Fatalf("AC2: item %s dispatched %d times, want exactly 1", it.ActionID, e.dispatch.calls[it.ActionID])
		}
	}
}

// TestPlanExecuteRefusesUnapprovedOrForeignDigest: the plan-level gates —
// an awaiting_approval plan never executes; a foreign digest never
// executes (AC1 at execute time); a missing plan is not-found.
func TestPlanExecuteRefusesUnapprovedOrForeignDigest(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); !errors.Is(err, ErrPlanState) {
		t.Fatalf("unapproved plan must not execute, got %v", err)
	}
	e.approveAll(t, pv)
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, "stale-digest"); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("execute with foreign digest must be refused, got %v", err)
	}
	if _, err := e.svc.Execute(context.Background(), 7, "plan-x", pv.Digest); !errors.Is(err, repoappconn.ErrPlanNotFound) {
		t.Fatalf("unknown plan must be not-found, got %v", err)
	}
}

// TestPlanExecuteSkipsUnapprovedItemFailClosed: an included item whose
// single-item approval is absent (partial-approve window) is never
// dispatched by the plan pass.
func TestPlanExecuteSkipsUnapprovedItemFailClosed(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.approveAll(t, pv)
	// Simulate the partial-approval window: item 2 back to awaiting.
	if err := e.store.SetActionState(context.Background(), pv.Items[1].ActionID,
		appconn.ActionAuthorized, appconn.ActionAwaitingApproval); err != nil {
		t.Fatal(err)
	}
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[1].Disposition != ItemSkippedUnapproved {
		t.Fatalf("unapproved item must be skipped fail-closed: %+v", out.Items[1])
	}
	if e.dispatch.calls[pv.Items[1].ActionID] != 0 {
		t.Fatalf("unapproved item dispatched: %v", e.dispatch.calls)
	}
}

// TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter pins the
// queued/dispatched→skipped_in_flight branch directly（终审 Finding 2 的
// 直接覆盖腿）: an item another live writer already owns（queued 或
// dispatched）never re-dispatches, keeps its state untouched, and does
// not stop the pass（authorized 控制腿照常执行，逐项独立）.
func TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B"), item("Doc C")}})
	if err != nil {
		t.Fatal(err)
	}
	e.dispatch.outcomes[pv.Items[2].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-c")}
	e.approveAll(t, pv)
	// Simulate the in-flight window: a concurrent writer queued item 1
	// and has already dispatched item 2.
	if err := e.store.SetActionState(context.Background(), pv.Items[0].ActionID,
		appconn.ActionAuthorized, appconn.ActionQueued); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActionState(context.Background(), pv.Items[1].ActionID,
		appconn.ActionAuthorized, appconn.ActionDispatched); err != nil {
		t.Fatal(err)
	}
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].Disposition != ItemSkippedInFlight || out.Items[0].ActionState != appconn.ActionQueued {
		t.Fatalf("queued item must be skipped in flight: %+v", out.Items[0])
	}
	if out.Items[1].Disposition != ItemSkippedInFlight || out.Items[1].ActionState != appconn.ActionDispatched {
		t.Fatalf("dispatched item must be skipped in flight: %+v", out.Items[1])
	}
	if out.Items[2].Disposition != ItemExecuted || out.Items[2].ActionState != appconn.ActionSucceeded {
		t.Fatalf("in-flight items must not stop the pass: %+v", out.Items[2])
	}
	if e.dispatch.calls[pv.Items[0].ActionID] != 0 || e.dispatch.calls[pv.Items[1].ActionID] != 0 {
		t.Fatalf("in-flight items must never be re-dispatched: %v", e.dispatch.calls)
	}
	// The plan pass must not mutate the live writer's state machine.
	for i, want := range []string{appconn.ActionQueued, appconn.ActionDispatched, appconn.ActionSucceeded} {
		row, err := e.store.FindAction(context.Background(), pv.Items[i].ActionID)
		if err != nil {
			t.Fatal(err)
		}
		if row.State != want {
			t.Fatalf("item %d state mutated by plan pass: got %s want %s", i+1, row.State, want)
		}
	}
}

// TestPlanStatusProjectsPerItemResults: the durable projection — plan
// identity, the frozen exclusion set, and each item's authoritative
// action state + receipt.
func TestPlanStatusProjectsPerItemResults(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B")}})
	if err != nil {
		t.Fatal(err)
	}
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Status(context.Background(), 7, pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.ID != pv.ID || st.State != PlanStateAuthorized || len(st.Excluded) != 1 || st.Excluded[0] != 2 {
		t.Fatalf("plan projection: %+v excluded=%v", st.PlanView, st.Excluded)
	}
	if len(st.Outcomes) != 2 {
		t.Fatalf("per-item outcomes: %+v", st.Outcomes)
	}
	if st.Outcomes[0].ActionState != appconn.ActionSucceeded || st.Outcomes[0].Publication.State != "published" {
		t.Fatalf("item 1 projection: %+v", st.Outcomes[0])
	}
	if st.Outcomes[1].ActionState != appconn.ActionAwaitingApproval {
		t.Fatalf("excluded item projects its untouched action state: %+v", st.Outcomes[1])
	}
}

// TestPlanApproveRecoveryExclusionFrozen: after execution started the
// recorded exclusion set is frozen — a different set is a state
// conflict; the same-set re-approve stays legal (recovery path).
// （Review Focus 3 的排除集冻结腿；依赖本任务的 Execute，故归此。）
func TestPlanApproveRecoveryExclusionFrozen(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.approveAll(t, pv)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-1")}
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); !errors.Is(err, ErrPlanState) {
		t.Fatalf("exclusion rewrite after execution must be refused, got %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a", ApproveInput{Digest: pv.Digest}); err != nil {
		t.Fatalf("idempotent re-approve refused: %v", err)
	}
}
