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

// newFeishuPublishHandler is the T19 (#49) dig constructor — the feishu
// twin of newNotionPublishHandler (notion_publish.go): the same ActionStore
// authority, A02 guard and U05 gate; its dispatcher AND unknown resolver
// are the Feishu bridge; the provider difference is the FeishuProfile.
func newFeishuPublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppFeishuPublishHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewFeishuBridge(
		scopeSrc,
		publish.NewConstantFeishuPolicyProvider(),
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewProviderPublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, scopeSrc,
		publish.FeishuProfile(bridge))
	h := handler.NewAppFeishuPublishHandler(db)
	h.SetFeishuPublishService(svc)
	return h, nil
}
