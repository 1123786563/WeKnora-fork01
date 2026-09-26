package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"

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
