package container

// T12 (#132) central wiring: the export feature registration mounts the
// bundle describe/download routes through the T00 constrained registry
// (fail-closed on a nil service), the member-level version/evidence adapter
// serves exactly the SAME task (tenant + session) members the lane pinned —
// every other task stays invisible — and the assembly refuses anything but
// the concrete craft version store.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type exportStubFiles struct{ interfaces.FileService }

type exportStubKnowledge struct{ interfaces.KnowledgeService }

type exportStubAccess struct{ craft.TaskAccessChecker }

func (exportStubAccess) CheckTaskAccess(context.Context, craft.Scope, craft.TaskAction) error {
	return craft.ErrForbidden
}

func t12Hex(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// newT12ExportDB builds the minimal real-schema database the adapter and
// the concrete store read (workspaces, versions, files, evidence).
func newT12ExportDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_workspaces (id text PRIMARY KEY, tenant_id integer, session_id text, owner_id text, sandbox_id text, generation text, opencode_session_id text, runtime_digest text, created_at datetime, updated_at datetime)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_versions (id text PRIMARY KEY, tenant_id integer, workspace_id text, run_id text, kind text, manifest_hash text, checks_json text, created_at datetime)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_version_files (version_id text, path text, resource_ref text, file_hash text, file_bytes integer, mime text, created_at datetime, PRIMARY KEY (version_id, path))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS craft_version_evidence (version_id text PRIMARY KEY, tenant_id integer, evidence_json text, digest text, acquired_at datetime, pinned_at datetime)`).Error)
	return db
}

// seedT12Version plants one published version with pinned evidence owned by
// (tenant 1, session s-t12, user u-owner) and returns its id.
func seedT12Version(t *testing.T, db *gorm.DB) string {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO craft_workspaces (id, tenant_id, session_id, owner_id, sandbox_id, generation, opencode_session_id, runtime_digest, created_at, updated_at)
		VALUES ('ws-t12', 1, 's-t12', 'u-owner', 'sbx', '0', 'oc', 'sha256:r', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	versionID := "ver_" + t12Hex("t12-version")
	require.NoError(t, db.Exec(`INSERT INTO craft_versions (id, tenant_id, workspace_id, run_id, kind, manifest_hash, checks_json, created_at)
		VALUES (?, 1, 'ws-t12', 'run-t12', 'web', ?, '[]', CURRENT_TIMESTAMP)`, versionID, t12Hex("manifest")).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_version_files (version_id, path, resource_ref, file_hash, file_bytes, mime, created_at)
		VALUES (?, 'index.html', 'obj-t12', ?, 4, 'text/html', CURRENT_TIMESTAMP)`, versionID, t12Hex("index")).Error)

	pinned := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	evidence := craft.VersionEvidence{
		VersionID: versionID, RunID: "run-t12", Empty: true,
		RequestDigest: t12Hex("req"), PackageDigest: t12Hex("pkg"), PinnedAt: pinned,
	}
	raw, digest, err := craft.EncodeVersionEvidence(evidence)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO craft_version_evidence (version_id, tenant_id, evidence_json, digest, acquired_at, pinned_at)
		VALUES (?, 1, ?, ?, ?, ?)`, versionID, string(raw), digest, pinned, pinned).Error)
	return versionID
}

// TestCraftExportMemberAdapterServesSameTaskMembers pins the member-level
// adapter the production assembly wires: every member of the SAME task
// (tenant + session) reads the version facts and pinned evidence through
// the owner's authorization; foreign tasks and tenants stay invisible with
// a plain 404, and a missing version answers 404.
func TestCraftExportMemberAdapterServesSameTaskMembers(t *testing.T) {
	db := newT12ExportDB(t)
	versionID := seedT12Version(t, db)
	store, ok := repository.NewCraftVersionStore(db).(*repository.CraftVersionStore)
	require.True(t, ok)
	member := craftMemberVersionReader{db: db, store: store}
	ctx := context.Background()

	viewer := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-t12"}
	version, err := member.Get(ctx, viewer, versionID)
	require.NoError(t, err, "a same-task member reads the version facts")
	require.Equal(t, versionID, version.ID)
	evidence, err := member.VersionEvidence(ctx, viewer, versionID)
	require.NoError(t, err, "a same-task member reads the pinned evidence")
	require.Equal(t, versionID, evidence.VersionID)

	foreignSession := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-other"}
	_, err = member.Get(ctx, foreignSession, versionID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign task never sees the version")
	_, err = member.VersionEvidence(ctx, foreignSession, versionID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign task never sees the evidence")

	foreignTenant := craft.Scope{TenantID: 2, UserID: "u-owner", SessionID: "s-t12"}
	_, err = member.Get(ctx, foreignTenant, versionID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign tenant never sees the version")

	_, err = member.Get(ctx, viewer, "ver_"+t12Hex("missing"))
	require.ErrorIs(t, err, craft.ErrNotFound, "a missing version answers 404")

	// A transient database failure propagates as itself — it must not
	// masquerade as a missing version (the CraftAccessService.session
	// precedent: NotFound maps, the rest travels for a 503).
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = member.Get(ctx, viewer, versionID)
	require.Error(t, err, "a closed database is an error")
	require.NotErrorIs(t, err, craft.ErrNotFound, "a transient failure is never a 404")
}

// TestCraftExportFeatureAssemblyRegistersRoutes pins the T12+T13 central
// wiring: the assembly requires the concrete craft version store, the
// feature registration mounts the CONSENT-GATED bundle routes through the
// T00 registry (fail-closed on a nil service — a nil consent service fails
// the gate closed exactly like a nil bundle), the consent read/decision
// feature mounts beside it, and a real service instance registers and
// mounts the static export routes.
func TestCraftExportFeatureAssemblyRegistersRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newT12ExportDB(t)
	seedT12Version(t, db)

	store, ok := repository.NewCraftVersionStore(db).(*repository.CraftVersionStore)
	require.True(t, ok)
	access := &service.CraftAccessService{}
	exportSvc, err := newCraftExportService(db, store, exportStubKnowledge{}, access)
	require.NoError(t, err)
	require.NotNil(t, exportSvc)
	consentSvc, err := newCraftExportConsentService(db, store, access)
	require.NoError(t, err)
	require.NotNil(t, consentSvc)

	routes := session.NewCraftFeatureRoutes()
	require.Error(t, registerCraftExportFeature(nil, nil, exportStubFiles{}, routes), "a nil service is refused fail-closed")
	require.Error(t, registerCraftExportFeature(exportSvc, nil, exportStubFiles{}, routes), "a nil consent service refuses the gate fail-closed")
	require.NoError(t, registerCraftExportFeature(exportSvc, consentSvc, exportStubFiles{}, routes))
	require.NoError(t, registerCraftExportConsentFeature(consentSvc, routes))

	engine := gin.New()
	group := engine.Group("/sessions")
	require.NoError(t, routes.Mount(group))
	recorded := map[string]bool{}
	for _, route := range engine.Routes() {
		recorded[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /sessions/:id/craft/versions/:version_id/export",
		"GET /sessions/:id/craft/versions/:version_id/export/download",
		// T13 (#133): the consent view rides the GET tree beside the T12
		// describe route; the owner decision rides the POST tree.
		"GET /sessions/:id/craft/versions/:version_id/export/consent",
		"POST /sessions/:session_id/craft/versions/:version_id/export/consent/decision",
	} {
		require.True(t, recorded[want], "export route %s must be mounted", want)
	}

	// The assembly refuses a non-concrete version store (a stub must never
	// silently serve bundles — the consent authority holds the same bar).
	_, err = newCraftExportService(db, shareStubVersions{}, exportStubKnowledge{}, &service.CraftAccessService{})
	require.Error(t, err, "the export service requires the concrete craft version store")
	_, err = newCraftExportConsentService(db, shareStubVersions{}, access)
	require.Error(t, err, "the export consent service requires the concrete craft version store")
}
