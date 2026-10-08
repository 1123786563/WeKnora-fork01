package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// newCraftShareService assembles the T11 (#128) restricted-share consent
// service: versions and files are the same stores the session service and
// collectors use, records is the shared CraftKnowledgeRecordRepository
// (identical to the citation gate), and task access is the persistent
// CraftAccessService so every decision and read re-checks current Task
// authority. Failures refuse application setup closed.
func newCraftShareService(
	db *gorm.DB,
	versions craft.VersionStore,
	files interfaces.FileService,
	records *repository.CraftKnowledgeRecordRepository,
	access *service.CraftAccessService,
) (*service.CraftShareService, error) {
	return service.NewCraftShareService(service.CraftShareConfig{
		DB: db, Versions: versions, Files: files, Records: records, TaskAccess: access,
	})
}

// registerCraftShareFeature mounts the share-consent read/decision/revocation
// routes through the T00 constrained feature registry (static siblings of
// the existing version-file routes; no wildcard conflicts). A nil service
// (typed nil included) refuses fail-closed.
func registerCraftShareFeature(share *service.CraftShareService, routes *session.CraftFeatureRoutes) error {
	if share == nil {
		return session.RegisterCraftShareFeature(routes, nil)
	}
	return session.RegisterCraftShareFeature(routes, share)
}
