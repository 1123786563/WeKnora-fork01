package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The PrepareOC suite runs on the REAL stores and the REAL migration SQL:
// 000041 for the binding tables, 000039 plus the new 000042 twin for
// app_actions — so the columns under test are the true production ones,
// not an AutoMigrate approximation.

func prepareOCExecMigration(t *testing.T, db *gorm.DB, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(raw)).Error; err != nil {
		t.Fatalf("migration %s: %v", path, err)
	}
}

func newPrepareOCFixture(t *testing.T) (*OCPreparer, *appconnectorrepo.OCStore, *OCCatalog, *appconnectorrepo.InstallationStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000&_foreign_keys=1"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&appconnectorrepo.AppVersion{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	prepareOCExecMigration(t, db, "../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	prepareOCExecMigration(t, db, "../../../../migrations/sqlite/000039_app_actions.up.sql")
	prepareOCExecMigration(t, db, "../../../../migrations/sqlite/000042_open_connector_actions.up.sql")
	oc := appconnectorrepo.NewOCStore(db)
	inst := appconnectorrepo.NewInstallationStore(db)
	catalog := NewOCCatalog(oc, oc, inst, inst)
	actions := NewActionService(appconnectorrepo.NewActionStore(db), &stubGuard{}, nil, nil, nil)
	return NewOCPreparer(actions, catalog, oc), oc, catalog, inst, db
}

// TestPrepareOCFillsIdentityFromServerSideOnly pins ruling 4: risk and app
// version come from the reviewed definition, the execution binding from the
// tenant's live binding, the auth version from the binding's mirror, and a
// targetless read carries the fixed action id as its target — nothing in
// the persisted row is client-supplied.
func TestPrepareOCFillsIdentityFromServerSideOnly(t *testing.T) {
	preparer, _, catalog, inst, db := newPrepareOCFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, preparerBindingsOf(t, db), catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	id, err := preparer.PrepareOC(ctx, appconn.OCSubject{TenantID: catalogTenant, ActorID: "u1"}, catalogConn, catalogAction, json.RawMessage(`{"q":"weknora"}`))
	if err != nil {
		t.Fatal(err)
	}
	store := appconnectorrepo.NewActionStore(db)
	row, err := store.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Risk != OCRiskRead || row.AppVersion != catalogV1 {
		t.Fatalf("risk/version not from the reviewed definition: %s@%s", row.Risk, row.AppVersion)
	}
	if row.Target != catalogAction {
		t.Fatalf("targetless read target=%q, want the fixed action id %q", row.Target, catalogAction)
	}
	if row.AuthVersion != 1 || row.DigestVersion != appconn.CurrentDigestVersion {
		t.Fatalf("auth/digest version=%d/%d", row.AuthVersion, row.DigestVersion)
	}
	var oc appconn.OCExecutionBinding
	if err := json.Unmarshal([]byte(row.OCBindingJSON), &oc); err != nil {
		t.Fatal(err)
	}
	want := appconn.OCExecutionBinding{
		RuntimeID: "rt-1", Provider: catalogProvider, ExternalID: "ext-" + catalogConn,
		Alias: fmt.Sprintf("alias-%d", catalogTenant), ActionID: catalogAction,
		SchemaDigest: catalogSchemaDigest, BindingVersion: 1,
	}
	if oc != want {
		t.Fatalf("binding=%+v, want %+v", oc, want)
	}
	// The stored digest equals the digest of the equivalent fully-built
	// action — the approval binds exactly what was persisted.
	rebuilt := appconn.Action{
		ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID, ConnectionID: row.ConnectionID,
		Version: row.AppVersion, Target: row.Target, Risk: row.Risk, AuthVersion: row.AuthVersion,
		Args: json.RawMessage(row.ArgsSnapshot), OC: &want, DigestVersion: int(row.DigestVersion),
	}
	d, err := appconn.ActionDigest(rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if row.ArgsDigest != d {
		t.Fatalf("stored digest %s != rebuilt digest %s", row.ArgsDigest, d)
	}
	if row.State != appconn.ActionAwaitingApproval {
		t.Fatalf("fresh OC action state=%s", row.State)
	}
}

// preparerBindingsOf hands the fixture the OCStore handle seedCatalogConnection
// expects (kept as a helper so the fixture signature stays stable).
func preparerBindingsOf(t *testing.T, db *gorm.DB) *appconnectorrepo.OCStore {
	t.Helper()
	return appconnectorrepo.NewOCStore(db)
}

// TestPrepareOCRejectsArgsOutsideReviewedSchema pins ruling 4's schema
// gate: normalized args must satisfy the frozen compiled schema (R2), be a
// JSON object, and be exactly one JSON value — and nothing is persisted on
// rejection.
func TestPrepareOCRejectsArgsOutsideReviewedSchema(t *testing.T) {
	preparer, _, catalog, inst, db := newPrepareOCFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, preparerBindingsOf(t, db), catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	subject := appconn.OCSubject{TenantID: catalogTenant, ActorID: "u1"}
	bad := map[string]json.RawMessage{
		"wrong type":            json.RawMessage(`{"q":1}`),
		"missing required":      json.RawMessage(`{}`),
		"extra property":        json.RawMessage(`{"q":"a","x":1}`),
		"non-object args":       json.RawMessage(`[1,2]`),
		"trailing second value": json.RawMessage(`{"q":"a"}{"q":"b"}`),
	}
	for name, args := range bad {
		_, err := preparer.PrepareOC(ctx, subject, catalogConn, catalogAction, args)
		switch name {
		case "trailing second value":
			if !errors.Is(err, appconn.ErrInvalidArgs) {
				t.Fatalf("%s: %v", name, err)
			}
		default:
			if !errors.Is(err, ErrOCArgsRejectedBySchema) {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	var count int64
	if err := db.Table("app_actions").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rejected prepares persisted %d rows", count)
	}
}

// TestPrepareOCRequiresPublishedDefinitionAndLiveBinding pins the
// reachability gates: staged (unpublished) reviews and unknown actions are
// refused by the catalog chain, and a non-active binding — even when the
// catalog already resolved the definition — is refused by the preparer's
// own defense-in-depth check.
func TestPrepareOCRequiresPublishedDefinitionAndLiveBinding(t *testing.T) {
	preparer, oc, catalog, inst, db := newPrepareOCFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, preparerBindingsOf(t, db), catalogTenant, catalogConn)
	staged := catalogDefinition(catalogAction)
	staged.Published = false
	if err := catalog.PublishOCDefinition(ctx, staged); err != nil {
		t.Fatal(err)
	}
	subject := appconn.OCSubject{TenantID: catalogTenant, ActorID: "u1"}
	if _, err := preparer.PrepareOC(ctx, subject, catalogConn, catalogAction, json.RawMessage(`{"q":"a"}`)); !errors.Is(err, ErrOCDefinitionUnpublished) {
		t.Fatalf("staged review accepted: %v", err)
	}
	if _, err := preparer.PrepareOC(ctx, subject, catalogConn, "github.nope", json.RawMessage(`{"q":"a"}`)); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("unknown action accepted: %v", err)
	}
	_ = oc
	// Defense in depth: a binding that is not active between the catalog
	// resolution and the preparer's own read still refuses (fake seam).
	stuck := &stubOCDefinitionResolver{def: catalogDefinition(catalogAction)}
	revoked := &stubBindingStore{binding: appconn.OCBinding{TenantID: catalogTenant, ConnectionID: catalogConn, State: appconn.OCBindingRevoked}}
	actions := NewActionService(appconnectorrepo.NewActionStore(db), &stubGuard{}, nil, nil, nil)
	racyPreparer := NewOCPreparer(actions, stuck, revoked)
	if _, err := racyPreparer.PrepareOC(ctx, subject, catalogConn, catalogAction, json.RawMessage(`{"q":"a"}`)); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("non-active binding accepted: %v", err)
	}
}

type stubOCDefinitionResolver struct {
	def appconn.OCDefinition
}

func (s *stubOCDefinitionResolver) GetOCDefinition(ctx context.Context, tenant uint64, connectionID, actionID string) (appconn.OCDefinition, error) {
	return s.def, nil
}

type stubBindingStore struct {
	binding appconn.OCBinding
}

func (s *stubBindingStore) GetBinding(ctx context.Context, tenant uint64, connectionID string) (appconn.OCBinding, error) {
	return s.binding, nil
}

// TestPrepareOCTargetDerivedFromReviewedTargetField pins the target rule:
// when the reviewed schema declares a "target" string input, the approval's
// target is that concrete value from the args; without it (or without the
// field in the args) the fixed action id is the target. Both variants bind
// distinct digests.
func TestPrepareOCTargetDerivedFromReviewedTargetField(t *testing.T) {
	preparer, _, catalog, inst, db := newPrepareOCFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, preparerBindingsOf(t, db), catalogTenant, catalogConn)
	targetSchema := `{"type":"object","properties":{"q":{"type":"string"},"target":{"type":"string"}},"required":["q"],"additionalProperties":false}`
	def := appconn.OCDefinition{
		AppID: catalogApp, AppVersion: catalogV1, ActionID: "github.read", Provider: catalogProvider,
		InputSchema: json.RawMessage(targetSchema), RequiredScopes: []string{"repo:read"},
		Risk: OCRiskRead, Published: true,
	}
	if err := catalog.PublishOCDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	subject := appconn.OCSubject{TenantID: catalogTenant, ActorID: "u1"}
	withTarget, err := preparer.PrepareOC(ctx, subject, catalogConn, "github.read", json.RawMessage(`{"q":"x","target":"pages/12"}`))
	if err != nil {
		t.Fatal(err)
	}
	withoutTarget, err := preparer.PrepareOC(ctx, subject, catalogConn, "github.read", json.RawMessage(`{"q":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	store := appconnectorrepo.NewActionStore(db)
	a, _ := store.FindAction(ctx, withTarget)
	b, _ := store.FindAction(ctx, withoutTarget)
	if a.Target != "pages/12" {
		t.Fatalf("derived target=%q, want pages/12", a.Target)
	}
	if b.Target != "github.read" {
		t.Fatalf("targetless target=%q, want the fixed action id", b.Target)
	}
	if a.ArgsDigest == b.ArgsDigest {
		t.Fatal("target variant did not change the digest")
	}
}

// TestPrepareOCSameArgsDifferentActionsAreIndependentApprovals pins ruling
// 6: two PrepareOC calls with identical args produce different action ids
// and different digests — one action's approval can never bless the other.
func TestPrepareOCSameArgsDifferentActionsAreIndependentApprovals(t *testing.T) {
	preparer, _, catalog, inst, db := newPrepareOCFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, preparerBindingsOf(t, db), catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	subject := appconn.OCSubject{TenantID: catalogTenant, ActorID: "u1"}
	first, err := preparer.PrepareOC(ctx, subject, catalogConn, catalogAction, json.RawMessage(`{"q":"same"}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := preparer.PrepareOC(ctx, subject, catalogConn, catalogAction, json.RawMessage(`{"q":"same"}`))
	if err != nil {
		t.Fatal(err)
	}
	store := appconnectorrepo.NewActionStore(db)
	a, _ := store.FindAction(ctx, first)
	b, _ := store.FindAction(ctx, second)
	if first == second || a.ArgsDigest == b.ArgsDigest {
		t.Fatal("identical args shared a digest across actions")
	}
	actions := NewActionService(store, &stubGuard{}, nil, nil, nil)
	if err := actions.Approve(ctx, first, "boss", a.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := actions.Approve(ctx, second, "boss", a.ArgsDigest); !errors.Is(err, ErrActionDigestMismatch) {
		t.Fatalf("first action's approval blessed the second: %v", err)
	}
}
