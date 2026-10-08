package container

// T12 (#132) central assembly: the version-bound source-bundle export. The
// service reads immutable version facts and pinned evidence through a
// MEMBER-LEVEL adapter — the same-task relaxation the T07 integration
// recorded for the evidence read surface (bundle downloads serve every
// TaskRead member, not just the owner) — while the owner-only store stays
// untouched for every other surface. Titles degrade best-effort through the
// existing shared-aware knowledge read (Title, else FileName — the T05
// sources-projection rule); they are display-only and deliberately outside
// the manifest's identity, so library edits never move a bundle digest.

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// craftMemberVersionReader adapts the version store's owner-only ACL
// (authorizeCraftVersion) to the member-level read the T12 bundle serves:
// any member of the SAME task (tenant + session) reads the version facts
// and its pinned evidence; every other task stays invisible (404, never a
// forbidden that confirms existence). Task membership itself is enforced
// separately by the export service's fresh RequireTaskAccess(TaskRead) on
// every describe/download, so this adapter only proves the version belongs
// to the requesting task and then reads through the owner's authorization —
// the exact semantics the lane's t12MemberEvidenceStore pinned.
type craftMemberVersionReader struct {
	db    *gorm.DB
	store *repository.CraftVersionStore
}

// ownerScopeOf resolves the task (tenant + session) and owning user one
// version belongs to. Version rows are immutable once published, so a plain
// read is the consistent read; only a missing row hides the version (404)
// while any other database failure propagates as itself — a transient
// outage must not masquerade as "version does not exist" (the
// CraftAccessService.session precedent: NotFound maps, the rest travels).
func (r craftMemberVersionReader) ownerScopeOf(ctx context.Context, tenantID uint64, versionID string) (craft.Scope, error) {
	var version struct {
		TenantID    uint64
		WorkspaceID string
	}
	err := r.db.WithContext(ctx).
		Table("craft_versions").Select("tenant_id, workspace_id").
		Where("id = ?", versionID).Take(&version).Error
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Scope{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Scope{}, err
	}
	if version.TenantID != tenantID {
		return craft.Scope{}, craft.ErrNotFound
	}
	var workspace struct {
		TenantID  uint64
		SessionID string
		OwnerID   string
	}
	if err := r.db.WithContext(ctx).Table("craft_workspaces").Select("tenant_id, session_id, owner_id").
		Where("id = ?", version.WorkspaceID).Take(&workspace).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return craft.Scope{}, craft.ErrNotFound
		}
		return craft.Scope{}, err
	}
	return craft.Scope{TenantID: workspace.TenantID, UserID: workspace.OwnerID, SessionID: workspace.SessionID}, nil
}

func (r craftMemberVersionReader) Get(ctx context.Context, scope craft.Scope, versionID string) (craft.Version, error) {
	owner, err := r.ownerScopeOf(ctx, scope.TenantID, versionID)
	if err != nil {
		return craft.Version{}, err
	}
	if owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
		return craft.Version{}, craft.ErrNotFound
	}
	return r.store.Get(ctx, owner, versionID)
}

func (r craftMemberVersionReader) VersionEvidence(ctx context.Context, scope craft.Scope, versionID string) (craft.VersionEvidence, error) {
	owner, err := r.ownerScopeOf(ctx, scope.TenantID, versionID)
	if err != nil {
		return craft.VersionEvidence{}, err
	}
	if owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	return r.store.VersionEvidence(ctx, owner, versionID)
}

// craftExportTitles resolves display titles through the existing
// shared-aware knowledge read (own rows plus organization-shared
// libraries): Title, else FileName — the same rule the T05 sources
// projection applies. A missing row simply degrades to no title.
func craftExportTitles(knowledge interfaces.KnowledgeService) service.CraftExportTitleSource {
	return func(ctx context.Context, tenantID uint64, knowledgeIDs []string) (map[string]string, error) {
		if len(knowledgeIDs) == 0 {
			return map[string]string{}, nil
		}
		rows, err := knowledge.GetKnowledgeBatchWithSharedAccess(ctx, tenantID, knowledgeIDs)
		if err != nil {
			return nil, err
		}
		titles := make(map[string]string, len(rows))
		for _, row := range rows {
			if row == nil {
				continue
			}
			title := row.Title
			if title == "" {
				title = row.FileName
			}
			titles[row.ID] = title
		}
		return titles, nil
	}
}

// newCraftExportService assembles the T12 bundle export. Versions and
// evidence flow through the member-level adapter over the SAME concrete
// store every other craft surface reads; task access is the persistent
// CraftAccessService so every describe/download re-checks current Task
// membership. Member bytes never flow through the service — the download
// surface streams them through its own file reader (registered alongside).
// Failures refuse application setup closed.
func newCraftExportService(
	db *gorm.DB,
	versions craft.VersionStore,
	knowledge interfaces.KnowledgeService,
	access *service.CraftAccessService,
) (*service.CraftExportService, error) {
	concrete, ok := versions.(*repository.CraftVersionStore)
	if !ok || concrete == nil {
		return nil, fmt.Errorf("craft: export service requires the concrete craft version store")
	}
	member := craftMemberVersionReader{db: db, store: concrete}
	return service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: member, Evidence: member,
		Titles: craftExportTitles(knowledge), TaskAccess: access,
	})
}

// newCraftExportConsentService assembles the T13 (#133) consent authority:
// the SAME member-level version/evidence adapter the T12 bundle reads
// through (versions belong to the requesting task; every other task stays
// 404) plus the persistent CraftAccessService, so every consent view and
// decision re-checks current Task membership through fresh derivations.
// Failures refuse application setup closed.
func newCraftExportConsentService(
	db *gorm.DB,
	versions craft.VersionStore,
	access *service.CraftAccessService,
) (*service.CraftExportConsentService, error) {
	concrete, ok := versions.(*repository.CraftVersionStore)
	if !ok || concrete == nil {
		return nil, fmt.Errorf("craft: export consent service requires the concrete craft version store")
	}
	member := craftMemberVersionReader{db: db, store: concrete}
	return service.NewCraftExportConsentService(service.CraftExportConsentConfig{
		DB: db, Versions: member, Evidence: member, TaskAccess: access,
	})
}

// registerCraftExportFeature mounts the bundle describe/download routes
// through the T00 constrained feature registry (static siblings of the
// existing version-file and share routes) — CONSENT-GATED (T13 #133): the
// T12 bundle projector is wrapped in NewConsentGatedExportService, so no
// restricted derived byte leaves without the current owner's bound approval
// (an ungated bundle projector answering downloads was the one fail-open
// seam). A nil service (typed nil included) refuses fail-closed.
func registerCraftExportFeature(
	export *service.CraftExportService,
	consent *service.CraftExportConsentService,
	files interfaces.FileService,
	routes *session.CraftFeatureRoutes,
) error {
	if export == nil || consent == nil {
		return session.RegisterCraftExportFeature(routes, nil, files)
	}
	gated, err := service.NewConsentGatedExportService(export, consent)
	if err != nil {
		return err
	}
	return session.RegisterCraftExportFeature(routes, gated, files)
}

// registerCraftExportConsentFeature mounts the T13 consent read/decision
// routes (feature "export_consent": the GET view beside the T12 describe
// route, the decision beside every other craft mutation). A nil service
// refuses fail-closed inside the registry.
func registerCraftExportConsentFeature(consent *service.CraftExportConsentService, routes *session.CraftFeatureRoutes) error {
	return session.RegisterCraftExportConsentFeature(routes, consent)
}
