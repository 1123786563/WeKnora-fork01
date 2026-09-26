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
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
// version belongs to, under a read lock so the read is consistent.
func (r craftMemberVersionReader) ownerScopeOf(ctx context.Context, tenantID uint64, versionID string) (craft.Scope, error) {
	var version struct {
		TenantID    uint64
		WorkspaceID string
	}
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Table("craft_versions").Select("tenant_id, workspace_id").
		Where("id = ?", versionID).Take(&version).Error
	if err != nil {
		return craft.Scope{}, craft.ErrNotFound
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
		return craft.Scope{}, craft.ErrNotFound
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
// store every other craft surface reads; files are the shared file
// service; task access is the persistent CraftAccessService so every
// describe/download re-checks current Task membership. Failures refuse
// application setup closed.
func newCraftExportService(
	db *gorm.DB,
	versions craft.VersionStore,
	files interfaces.FileService,
	knowledge interfaces.KnowledgeService,
	access *service.CraftAccessService,
) (*service.CraftExportService, error) {
	concrete, ok := versions.(*repository.CraftVersionStore)
	if !ok || concrete == nil {
		return nil, fmt.Errorf("craft: export service requires the concrete craft version store")
	}
	member := craftMemberVersionReader{db: db, store: concrete}
	return service.NewCraftExportService(service.CraftExportConfig{
		DB: db, Versions: member, Evidence: member, Files: files,
		Titles: craftExportTitles(knowledge), TaskAccess: access,
	})
}

// registerCraftExportFeature mounts the bundle describe/download routes
// through the T00 constrained feature registry (static siblings of the
// existing version-file and share routes). A nil service (typed nil
// included) refuses fail-closed.
func registerCraftExportFeature(export *service.CraftExportService, files interfaces.FileService, routes *session.CraftFeatureRoutes) error {
	if export == nil {
		return session.RegisterCraftExportFeature(routes, nil, files)
	}
	return session.RegisterCraftExportFeature(routes, export, files)
}
