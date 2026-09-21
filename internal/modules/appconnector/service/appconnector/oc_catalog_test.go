package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// fixture: the REAL repository stores on the REAL sqlite twin migration, so
// the catalog suite exercises actual persistence semantics (composite FKs,
// PK(app_id,app_version,action_id)) rather than stubs.
// ---------------------------------------------------------------------------

const (
	catalogApp      = "github-oc"
	catalogV1       = "1.0.0"
	catalogV2       = "2.0.0"
	catalogProvider = "github"
	catalogAction   = "github.search"
	catalogTenant   = uint64(7)
	catalogConn     = "c1"

	// catalogSchema is the frozen, reviewed input schema; its pinned SHA-256
	// (lowercase hex of the EXACT bytes below, precomputed independently of
	// the implementation) proves publication pins the digest from the schema
	// bytes instead of trusting a caller-supplied value.
	catalogSchema = "{\"type\":\"object\",\"properties\":{\"q\":{\"type\":\"string\"}},\"required\":[\"q\"],\"additionalProperties\":false}"
	// catalogSchemaV2 is the byte-different review of the SAME action under
	// a NEW app version.
	catalogSchemaV2       = "{\"type\":\"object\",\"properties\":{\"q\":{\"type\":\"string\"},\"n\":{\"type\":\"integer\"}},\"required\":[\"q\"],\"additionalProperties\":false}"
	catalogSchemaDigest   = "68746c7d602ce7a34ac6fb76624981eed58f14f3caca147fe0431b81a0373d6a"
	catalogSchemaV2Digest = "9e26d8481e440cb459d2156c6998f38cdbf2f7aa4b7ca2658a121044a13fff7e"
)

// ocCatalogMigrationSQL loads the real sqlite twin migration production
// uses (same convention as the repository suite), so the definition table
// under test is the true connector_action_definitions with its composite
// primary key — not an AutoMigrate approximation.
func ocCatalogMigrationSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// newCatalogFixture builds the OCCatalog service over the REAL repository
// stores on an isolated sqlite database: the parent installation tables are
// AutoMigrated exactly like the repository tests, then the real OC
// migration SQL is applied with composite foreign keys enforced. One
// pooled connection avoids SQLITE_LOCKED under shared-cache (same
// convention as the authorizer fixture).
func newCatalogFixture(t *testing.T) (*OCCatalog, *appconnectorrepo.OCStore, *appconnectorrepo.InstallationStore, *gorm.DB) {
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
	if err := db.Exec(ocCatalogMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	oc := appconnectorrepo.NewOCStore(db)
	inst := appconnectorrepo.NewInstallationStore(db)
	return NewOCCatalog(oc, oc, inst, inst), oc, inst, db
}

// seedCatalogConnection wires one tenant exactly like production: an
// active installation of the catalog app at catalogV1, a live personal
// connection, and an ACTIVE tenant-scoped binding to the github provider.
func seedCatalogConnection(t *testing.T, inst *appconnectorrepo.InstallationStore, oc *appconnectorrepo.OCStore, tenant uint64, connID string) {
	t.Helper()
	ctx := context.Background()
	instID := fmt.Sprintf("inst-%d", tenant)
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: instID, AppID: catalogApp, Version: catalogV1, State: appconn.InstallationActive, TenantID: tenant}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{
		ID: connID, InstallationID: instID, Kind: appconn.ConnectionKindPersonal,
		OwnerID: "owner", CredentialRef: "cred/" + instID, State: appconn.ConnectionActive,
		TenantID: tenant, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := oc.SaveBinding(ctx, appconn.OCBinding{
		TenantID: tenant, ConnectionID: connID, RuntimeID: "rt-1", Provider: catalogProvider,
		ExternalID: "ext-" + connID, Alias: fmt.Sprintf("alias-%d", tenant),
		AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
}

// catalogDefinition is one reviewed definition of the catalog app for the
// tenant provider; the digest is left empty so PublishOCDefinition must pin
// it from the exact schema bytes.
func catalogDefinition(action string) appconn.OCDefinition {
	return appconn.OCDefinition{
		AppID: catalogApp, AppVersion: catalogV1, ActionID: action, Provider: catalogProvider,
		InputSchema: json.RawMessage(catalogSchema), RequiredScopes: []string{"repo:read"},
		Risk: OCRiskRead, Published: true,
	}
}

// TestCatalogRejectsUnknownRisk is the plan's verbatim review gate: the
// execution risk comes from the publish record, and any category outside
// read/write/send/delete means the definition was never reviewed.
func TestCatalogRejectsUnknownRisk(t *testing.T) {
	err := ValidateOCDefinition(appconn.OCDefinition{ActionID: "a.read", Risk: "arbitrary", Published: true})
	if err == nil {
		t.Fatal("unreviewed risk")
	}
}

// TestCatalogValidateDefinitionMatrix pins the review rules: identity
// fields must be present, the risk must come from the closed vocabulary
// (case-sensitive), and a caller-supplied digest that disagrees with the
// frozen schema bytes is refused.
func TestCatalogValidateDefinitionMatrix(t *testing.T) {
	base := catalogDefinition(catalogAction)
	for _, risk := range []string{OCRiskRead, OCRiskWrite, OCRiskSend, OCRiskDelete} {
		d := base
		d.Risk = risk
		if err := ValidateOCDefinition(d); err != nil {
			t.Fatalf("reviewed risk %s rejected: %v", risk, err)
		}
		d.SchemaDigest = catalogSchemaDigest
		if err := ValidateOCDefinition(d); err != nil {
			t.Fatalf("risk %s with pinned digest rejected: %v", risk, err)
		}
	}
	wrongDigest := base
	wrongDigest.SchemaDigest = catalogSchemaV2Digest
	if err := ValidateOCDefinition(wrongDigest); !errors.Is(err, ErrOCDefinitionInvalid) {
		t.Fatalf("digest mismatch accepted: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*appconn.OCDefinition)
	}{
		{"missing app id", func(d *appconn.OCDefinition) { d.AppID = "" }},
		{"missing app version", func(d *appconn.OCDefinition) { d.AppVersion = "" }},
		{"missing action id", func(d *appconn.OCDefinition) { d.ActionID = "" }},
		{"missing provider", func(d *appconn.OCDefinition) { d.Provider = "" }},
		{"empty risk", func(d *appconn.OCDefinition) { d.Risk = "" }},
		{"uppercase risk", func(d *appconn.OCDefinition) { d.Risk = "READ" }},
		{"composite risk", func(d *appconn.OCDefinition) { d.Risk = "read/write" }},
		{"empty schema", func(d *appconn.OCDefinition) { d.InputSchema = nil }},
	} {
		d := base
		tc.mutate(&d)
		if err := ValidateOCDefinition(d); !errors.Is(err, ErrOCDefinitionInvalid) {
			t.Fatalf("%s accepted: %v", tc.name, err)
		}
	}
}

// TestCatalogRejectsFileAndURLSchemas pins the first-phase publication
// rule: file/URL-shaped action schemas never enter the catalog. The schema
// must be a JSON OBJECT at the root, must COMPILE as a real JSON Schema
// (JSON well-formedness alone is not enough), and any reference that would
// leave the document — file:// or http(s):// — fails compilation because
// the compiler runs without a loader. Embedded $defs/$ref pairs stay legal.
func TestCatalogRejectsFileAndURLSchemas(t *testing.T) {
	valid := []string{
		catalogSchema,
		"{\"type\":\"object\",\"$defs\":{\"q\":{\"type\":\"string\"}},\"properties\":{\"q\":{\"$ref\":\"#/$defs/q\"}}}",
	}
	for _, schema := range valid {
		d := catalogDefinition(catalogAction)
		d.InputSchema = json.RawMessage(schema)
		if err := ValidateOCDefinition(d); err != nil {
			t.Fatalf("valid schema rejected: %v", err)
		}
	}
	rejected := map[string]string{
		"file reference":    "{\"$ref\":\"file:///etc/passwd\"}",
		"https reference":   "{\"$ref\":\"https://schemas.evil.example/input.json\"}",
		"nested http ref":   "{\"type\":\"object\",\"properties\":{\"f\":{\"$ref\":\"http://evil.example/s.json\"}}}",
		"bare file url":     "\"file:///tmp/upload\"",
		"root array":        "[]",
		"root number":       "123",
		"root null":         "null",
		"broken json":       "{\"type\":",
		"not a real schema": "{\"type\":\"object\",\"properties\":{\"x\":{\"type\":\"strung\"}}}",
	}
	for name, schema := range rejected {
		d := catalogDefinition(catalogAction)
		d.InputSchema = json.RawMessage(schema)
		if err := ValidateOCDefinition(d); !errors.Is(err, ErrOCDefinitionInvalid) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}

// TestCatalogPublishPinsSchemaDigest proves the stored digest is the
// SHA-256 of the EXACT published schema bytes (checked against an
// independently precomputed constant), that a disagreeing supplied digest
// is refused before any write, and that the pinned schema round-trips
// byte-exactly.
func TestCatalogPublishPinsSchemaDigest(t *testing.T) {
	catalog, oc, _, _ := newCatalogFixture(t)
	ctx := context.Background()
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	stored, err := oc.GetDefinition(ctx, catalogApp, catalogV1, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SchemaDigest != catalogSchemaDigest {
		t.Fatalf("digest not pinned from schema bytes: got %s want %s", stored.SchemaDigest, catalogSchemaDigest)
	}
	if string(stored.InputSchema) != catalogSchema {
		t.Fatal("schema bytes not frozen")
	}
	conflicting := catalogDefinition("github.other")
	conflicting.SchemaDigest = catalogSchemaV2Digest
	if err := catalog.PublishOCDefinition(ctx, conflicting); !errors.Is(err, ErrOCDefinitionInvalid) {
		t.Fatalf("digest mismatch published: %v", err)
	}
	if _, err := oc.GetDefinition(ctx, catalogApp, catalogV1, "github.other"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("rejected definition was written: %v", err)
	}
}

// TestCatalogSameVersionDigestChangeRejected pins immutability: the same
// (app, app_version, action_id) row is never overwritten. Re-publishing
// the triple with different schema bytes, different scopes or a different
// risk is a conflict; only the byte-identical re-publish stays idempotent.
func TestCatalogSameVersionDigestChangeRejected(t *testing.T) {
	catalog, oc, _, _ := newCatalogFixture(t)
	ctx := context.Background()
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	changed := catalogDefinition(catalogAction)
	changed.InputSchema = json.RawMessage(catalogSchemaV2)
	if err := catalog.PublishOCDefinition(ctx, changed); !errors.Is(err, appconnectorrepo.ErrOCDefinitionConflict) {
		t.Fatalf("same-version schema change accepted: %v", err)
	}
	if err := oc.SaveDefinition(ctx, changed); !errors.Is(err, appconnectorrepo.ErrOCDefinitionConflict) {
		t.Fatalf("store overwrote frozen definition: %v", err)
	}
	riskier := catalogDefinition(catalogAction)
	riskier.Risk = OCRiskDelete
	if err := catalog.PublishOCDefinition(ctx, riskier); !errors.Is(err, appconnectorrepo.ErrOCDefinitionConflict) {
		t.Fatalf("same-version risk change accepted: %v", err)
	}
	stored, err := oc.GetDefinition(ctx, catalogApp, catalogV1, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SchemaDigest != catalogSchemaDigest || stored.Risk != OCRiskRead {
		t.Fatalf("frozen definition mutated in place: %+v", stored)
	}
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatalf("idempotent re-publish rejected: %v", err)
	}
}

// TestCatalogSameActionAcrossVersions pins the versioning model: one action
// id may exist under many app versions, each frozen independently, and the
// tenant always resolves the review pinned to its INSTALLED version.
func TestCatalogSameActionAcrossVersions(t *testing.T) {
	catalog, oc, inst, db := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	v1 := catalogDefinition(catalogAction)
	v2 := catalogDefinition(catalogAction)
	v2.AppVersion = catalogV2
	v2.InputSchema = json.RawMessage(catalogSchemaV2)
	v2.Risk = OCRiskWrite
	if err := catalog.PublishOCDefinition(ctx, v1); err != nil {
		t.Fatal(err)
	}
	if err := catalog.PublishOCDefinition(ctx, v2); err != nil {
		t.Fatalf("same action under a new version rejected: %v", err)
	}
	v1.SchemaDigest, v2.SchemaDigest = catalogSchemaDigest, catalogSchemaV2Digest
	for _, want := range []appconn.OCDefinition{v1, v2} {
		got, err := oc.GetDefinition(ctx, catalogApp, want.AppVersion, catalogAction)
		if err != nil {
			t.Fatal(err)
		}
		if got.Risk != want.Risk || got.SchemaDigest != want.SchemaDigest || string(got.InputSchema) != string(want.InputSchema) {
			t.Fatalf("version %s not frozen independently: %+v", want.AppVersion, got)
		}
	}
	// The installed version decides which review the tenant resolves.
	got, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppVersion != catalogV1 || got.SchemaDigest != catalogSchemaDigest {
		t.Fatalf("install pinned to %s resolved digest %s", catalogV1, got.SchemaDigest)
	}
	// Equal published scopes for both versions keep the upgrade a plain
	// version move rather than a scope expansion.
	for _, v := range []string{catalogV1, catalogV2} {
		if err := db.Create(&appconnectorrepo.AppVersion{AppID: catalogApp, Version: v, SchemaJSON: "{\"scopes\":[\"repo:read\"]}"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: catalogApp, Version: catalogV2, State: appconn.InstallationActive, TenantID: catalogTenant}, 1); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppVersion != catalogV2 || got.SchemaDigest != catalogSchemaV2Digest || got.Risk != OCRiskWrite {
		t.Fatalf("upgraded install resolved %+v", got)
	}
}

// TestCatalogDeScopeRequiresNewVersion pins that de-scoping (narrowing the
// required scopes) is a version-pinned change: narrowing in place on the
// same app version is a conflict, while a NEW app version may ship the
// narrowed scope set.
func TestCatalogDeScopeRequiresNewVersion(t *testing.T) {
	catalog, oc, _, _ := newCatalogFixture(t)
	ctx := context.Background()
	wide := catalogDefinition(catalogAction)
	wide.RequiredScopes = []string{"repo:read", "repo:write"}
	if err := catalog.PublishOCDefinition(ctx, wide); err != nil {
		t.Fatal(err)
	}
	narrowed := wide
	narrowed.RequiredScopes = []string{"repo:read"}
	if err := catalog.PublishOCDefinition(ctx, narrowed); !errors.Is(err, appconnectorrepo.ErrOCDefinitionConflict) {
		t.Fatalf("in-place de-scope accepted: %v", err)
	}
	stored, err := oc.GetDefinition(ctx, catalogApp, catalogV1, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.RequiredScopes) != 2 {
		t.Fatalf("scopes narrowed in place: %+v", stored.RequiredScopes)
	}
	narrowedV2 := narrowed
	narrowedV2.AppVersion = catalogV2
	if err := catalog.PublishOCDefinition(ctx, narrowedV2); err != nil {
		t.Fatalf("de-scope via new version rejected: %v", err)
	}
	v2, err := oc.GetDefinition(ctx, catalogApp, catalogV2, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if len(v2.RequiredScopes) != 1 || v2.RequiredScopes[0] != "repo:read" {
		t.Fatalf("new version did not carry narrowed scopes: %+v", v2.RequiredScopes)
	}
}

// TestCatalogRejectsEmptyActionSet pins the degenerate catalog entry: a
// definition with no action id (the empty action set) is refused by the
// review validator, by the platform publish path, and directly at the
// store — before any database write.
func TestCatalogRejectsEmptyActionSet(t *testing.T) {
	catalog, oc, _, db := newCatalogFixture(t)
	ctx := context.Background()
	empty := catalogDefinition("")
	if err := ValidateOCDefinition(empty); !errors.Is(err, ErrOCDefinitionInvalid) {
		t.Fatalf("empty action set validated: %v", err)
	}
	if err := catalog.PublishOCDefinition(ctx, empty); !errors.Is(err, ErrOCDefinitionInvalid) {
		t.Fatalf("empty action set published: %v", err)
	}
	if err := oc.SaveDefinition(ctx, empty); !errors.Is(err, appconnectorrepo.ErrOCDefinitionInvalid) {
		t.Fatalf("store accepted empty action set: %v", err)
	}
	var rows int64
	if err := db.Model(&appconnectorrepo.OCDefinitionRow{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rejected definition rows written: %d", rows)
	}
}

// TestCatalogGetRejectsUnpublishedDefinition pins that only PUBLISHED
// reviews are tenant-visible: a staged (published=false) definition exists
// in the store but GetOCDefinition refuses to hand it out, and flipping
// the workflow flag is the only field the platform may still change on the
// frozen row.
func TestCatalogGetRejectsUnpublishedDefinition(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	staged := catalogDefinition(catalogAction)
	staged.Published = false
	if err := catalog.PublishOCDefinition(ctx, staged); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); !errors.Is(err, ErrOCDefinitionUnpublished) {
		t.Fatalf("unpublished definition served: %v", err)
	}
	staged.Published = true
	if err := catalog.PublishOCDefinition(ctx, staged); err != nil {
		t.Fatalf("publishing the staged review rejected: %v", err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); err != nil {
		t.Fatal(err)
	}
}

// TestCatalogInstallVersionMismatchRejected pins version pinning from the
// tenant side: after the installation upgrades to a version whose actions
// were never published, the tenant no longer resolves the action — the
// catalog never silently falls back to another version's review.
func TestCatalogInstallVersionMismatchRejected(t *testing.T) {
	catalog, oc, inst, db := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{catalogV1, catalogV2} {
		if err := db.Create(&appconnectorrepo.AppVersion{AppID: catalogApp, Version: v, SchemaJSON: "{\"scopes\":[\"repo:read\"]}"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: catalogApp, Version: catalogV2, State: appconn.InstallationActive, TenantID: catalogTenant}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("definition from a foreign app version served: %v", err)
	}
}

// TestCatalogScopeExpansionRejected pins that the catalog never weakens the
// frozen upgrade gate: an upgrade whose published scope (app_versions
// schema_json) differs from the installed one can only land in
// reauthorization_required — never silently active — and the refused
// attempt leaves the installation untouched.
func TestCatalogScopeExpansionRejected(t *testing.T) {
	_, oc, inst, db := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	for _, av := range []appconnectorrepo.AppVersion{
		{AppID: catalogApp, Version: catalogV1, SchemaJSON: "{\"scopes\":[\"repo:read\"]}"},
		{AppID: catalogApp, Version: catalogV2, SchemaJSON: "{\"scopes\":[\"repo:read\",\"repo:write\"]}"},
	} {
		if err := db.Create(&av).Error; err != nil {
			t.Fatal(err)
		}
	}
	err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: catalogApp, Version: catalogV2, State: appconn.InstallationActive, TenantID: catalogTenant}, 1)
	if !errors.Is(err, appconnectorrepo.ErrReauthorizationRequired) {
		t.Fatalf("scope-expanding upgrade kept active: %v", err)
	}
	got, v, err := inst.GetInstallation(ctx, catalogTenant, catalogApp)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != catalogV1 || got.State != appconn.InstallationActive || v != 1 {
		t.Fatalf("refused upgrade leaked state: %+v v=%d", got, v)
	}
}

// TestCatalogScopeUpgradeBlocked pins the parked state: once the upgrade
// lands in reauthorization_required, new calls through its connections are
// rejected and the tenant catalog stops resolving its actions.
func TestCatalogScopeUpgradeBlocked(t *testing.T) {
	catalog, oc, inst, db := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	for _, av := range []appconnectorrepo.AppVersion{
		{AppID: catalogApp, Version: catalogV1, SchemaJSON: "{\"scopes\":[\"repo:read\"]}"},
		{AppID: catalogApp, Version: catalogV2, SchemaJSON: "{\"scopes\":[\"repo:read\",\"repo:write\"]}"},
	} {
		if err := db.Create(&av).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: catalogApp, Version: catalogV2, State: appconn.InstallationReauthorizationRequired, TenantID: catalogTenant}, 1); err != nil {
		t.Fatal(err)
	}
	conn, err := inst.GetConnection(ctx, catalogTenant, catalogConn)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := inst.ConnectionUsable(ctx, conn, "owner", false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("connection usable while installation awaits reauthorization")
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); !errors.Is(err, ErrInstallationNotActive) {
		t.Fatalf("catalog served a parked installation: %v", err)
	}
}

// TestCatalogDisabledInstallBlocked pins the platform kill switch: a
// disabled installation rejects every new call through its connections and
// its actions stop resolving in the tenant catalog.
func TestCatalogDisabledInstallBlocked(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: catalogApp, Version: catalogV1, State: appconn.InstallationDisabled, TenantID: catalogTenant}, 1); err != nil {
		t.Fatal(err)
	}
	conn, err := inst.GetConnection(ctx, catalogTenant, catalogConn)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := inst.ConnectionUsable(ctx, conn, "owner", false); err != nil || ok {
		t.Fatalf("connection usable on disabled installation: ok=%v err=%v", ok, err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); !errors.Is(err, ErrInstallationNotActive) {
		t.Fatalf("catalog served a disabled installation: %v", err)
	}
}

// TestCatalogTenantScopedToBinding pins the tenant boundary: a definition
// is only reachable through the CALLING tenant's own live binding. Another
// tenant re-using the same connection id sees nothing (whether it owns a
// bare connection or no rows at all), unscoped input is refused before any
// lookup, and a revoked binding stops resolving actions for its owner too.
func TestCatalogTenantScopedToBinding(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	if err := catalog.PublishOCDefinition(ctx, catalogDefinition(catalogAction)); err != nil {
		t.Fatal(err)
	}
	// Tenant 8 owns its own connection with the SAME id but NO binding.
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-8", AppID: catalogApp, Version: catalogV1, State: appconn.InstallationActive, TenantID: 8}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{ID: catalogConn, InstallationID: "inst-8", Kind: appconn.ConnectionKindPersonal, OwnerID: "owner8", CredentialRef: "cred/inst-8", State: appconn.ConnectionActive, TenantID: 8, AuthVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetOCDefinition(ctx, 8, catalogConn, catalogAction); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("tenant 8 resolved tenant 7 definition: %v", err)
	}
	if _, err := catalog.GetOCDefinition(ctx, 9, catalogConn, catalogAction); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("unknown tenant resolved a definition: %v", err)
	}
	for _, tc := range []struct {
		tenant uint64
		conn   string
		action string
	}{{0, catalogConn, catalogAction}, {catalogTenant, "", catalogAction}, {catalogTenant, catalogConn, ""}} {
		if _, err := catalog.GetOCDefinition(ctx, tc.tenant, tc.conn, tc.action); !errors.Is(err, ErrOCDefinitionInvalid) {
			t.Fatalf("unscoped lookup accepted: tenant=%d conn=%q action=%q err=%v", tc.tenant, tc.conn, tc.action, err)
		}
	}
	revoked, err := oc.GetBinding(ctx, catalogTenant, catalogConn)
	if err != nil {
		t.Fatal(err)
	}
	revoked.State = appconn.OCBindingRevoked
	revoked.BindingVersion = 2
	if err := oc.SaveBinding(ctx, revoked); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("revoked binding still resolves definitions: %v", err)
	}
}

// TestCatalogGetRoundTrip walks the full resolution chain — tenant binding →
// tenant connection → tenant installation → published definition pinned to
// the INSTALLED app version — and proves the round-trip is byte-exact.
func TestCatalogGetRoundTrip(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	published := catalogDefinition(catalogAction)
	published.SchemaDigest = catalogSchemaDigest
	if err := catalog.PublishOCDefinition(ctx, published); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, catalogAction)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppID != catalogApp || got.AppVersion != catalogV1 || got.ActionID != catalogAction || got.Provider != catalogProvider {
		t.Fatalf("identity mismatch: %+v", got)
	}
	if got.SchemaDigest != catalogSchemaDigest || string(got.InputSchema) != catalogSchema {
		t.Fatalf("schema not frozen: digest=%s schema=%s", got.SchemaDigest, got.InputSchema)
	}
	if len(got.RequiredScopes) != 1 || got.RequiredScopes[0] != "repo:read" || got.Risk != OCRiskRead || !got.Published {
		t.Fatalf("review record not preserved: %+v", got)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, "github.nope"); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("unknown action resolved: %v", err)
	}
}

// TestCatalogExcludesNoAuthActionsByDefault pins the first-phase catalog
// rule (ruling 8): no_auth-shaped actions — under the frozen OCDefinition
// the only authorization surface is RequiredScopes, so an action requiring
// NO scopes is no_auth — may be staged by the platform but are
// default-EXCLUDED from the tenant-visible catalog. Nothing here mints
// credentials or special-cases virtual connection ids.
func TestCatalogExcludesNoAuthActionsByDefault(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	noauth := catalogDefinition("arxiv.list")
	noauth.Provider = "arxiv"
	noauth.RequiredScopes = nil
	if err := ValidateOCDefinition(noauth); err != nil {
		t.Fatalf("no_auth definition failed review validation: %v", err)
	}
	if err := catalog.PublishOCDefinition(ctx, noauth); err != nil {
		t.Fatalf("platform cannot stage no_auth definition: %v", err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, "arxiv.list"); !errors.Is(err, ErrOCDefinitionNoAuthExcluded) {
		t.Fatalf("no_auth action served to tenant: %v", err)
	}
	staged, err := oc.GetDefinition(ctx, catalogApp, catalogV1, "arxiv.list")
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.RequiredScopes) != 0 {
		t.Fatalf("staged no_auth definition mutated: %+v", staged)
	}
}

// TestCatalogRejectsForeignProviderDefinition pins reachability: a
// published definition whose provider is not the one the tenant's binding
// wired up is unreachable through that binding — a tenant only ever sees
// the definitions of its OWN external connections.
func TestCatalogRejectsForeignProviderDefinition(t *testing.T) {
	catalog, oc, inst, _ := newCatalogFixture(t)
	ctx := context.Background()
	seedCatalogConnection(t, inst, oc, catalogTenant, catalogConn)
	foreign := catalogDefinition("dune.query")
	foreign.Provider = "dune"
	foreign.RequiredScopes = []string{"query:run"}
	if err := catalog.PublishOCDefinition(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetOCDefinition(ctx, catalogTenant, catalogConn, "dune.query"); !errors.Is(err, ErrOCDefinitionUnreachable) {
		t.Fatalf("foreign-provider definition served: %v", err)
	}
}
