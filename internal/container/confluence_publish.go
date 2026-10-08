package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"gorm.io/gorm"
)

// newConfluencePublishHandler is the T20 (#50) dig constructor: it builds
// the dedicated confluence publish ActionService (same ActionStore
// authority, same A02 guard, same U05 gate; its dispatcher AND unknown
// resolver are the Confluence bridge), the publish service, and the HTTP
// handler. The frozen OC-armed ActionService and the #48 Notion publish
// instance are untouched — the three services share the store, which the
// design names as the authority (action.go:158-160).
func newConfluencePublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppConfluencePublishHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	scopeSrc := publish.NewDBConfluenceScopeSource(db)
	bridge := publish.NewConfluenceBridge(
		scopeSrc,
		publish.NewConfluencePolicyProvider(scopeSrc),
		publish.NewConfluenceCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewConfluencePublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, bridge,
		scopeSrc)
	h := handler.NewAppConfluencePublishHandler(db)
	h.SetConfluencePublishService(svc)
	return h, nil
}
