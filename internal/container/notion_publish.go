package container

import (
	"context"
	"fmt"
	"io"

	"github.com/Tencent/WeKnora/internal/application/repository"
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/appconnector/plan"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/storageurl"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"gorm.io/gorm"
)

// newNotionPublishHandler is the T18 (#48) dig constructor: it builds the
// dedicated publish ActionService (same ActionStore authority, same A02
// guard, same U05 gate; its dispatcher AND unknown resolver are the
// Notion bridge), the publish service, and the HTTP handler. The frozen
// OC-armed ActionService is untouched — the two services share the store,
// which the design names as the authority (action.go:158-160).
func newNotionPublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppNotionPublishHandler, *handler.AppActionPlanHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	bridge := publish.NewNotionBridge(
		publish.NewDBNotionScopeSource(db),
		publish.NewConstantNotionPolicyProvider(),
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewNotionPublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, bridge,
		publish.NewDBNotionScopeSource(db))
	h := handler.NewAppNotionPublishHandler(db)
	h.SetNotionPublishService(svc)
	// T21 (#51): the plan layer over the SAME publish ActionService —
	// per-item digests/approvals/dispatch claims stay on the A03
	// authority; the plan adds the set digest, exclusions and ordered
	// partial-success execution.
	planSvc := plan.NewService(repoappconn.NewPlanStore(db), store, actions, svc)
	planHandler := handler.NewAppActionPlanHandler(db)
	planHandler.SetActionPlanService(planSvc)
	return h, planHandler, nil
}

// tenantStorageArtifactContent reads one artifact version's bytes through
// the same tenant storage resolution the versioned download uses
// (artifact_download.go:556-586): tenant backend resolution first, the
// global service as fallback. v1 publishes text artifacts only.
type tenantStorageArtifactContent struct {
	files   interfaces.FileService
	tenants interfaces.TenantService
	storage interfaces.StorageBackendResolver
}

func (a *tenantStorageArtifactContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	fileService := a.files
	if a.tenants != nil {
		tenant, err := a.tenants.GetTenantByID(ctx, tenantID)
		if err != nil || tenant == nil {
			return nil, fmt.Errorf("artifact workspace unavailable")
		}
		backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
		if !scoped {
			providerPath = version.ObjectKey
		}
		var ok bool
		fileService, _, ok = filesvc.ResolveTenantFileServiceWithFallback(
			ctx, "notion publish", tenant, backendID,
			types.ParseProviderScheme(providerPath), storageurl.LocalStorageBaseDir(), a.storage, a.files,
		)
		if !ok {
			return nil, fmt.Errorf("artifact storage unavailable")
		}
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, publish.MaxPublishArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > publish.MaxPublishArtifactBytes {
		return nil, fmt.Errorf("%w", publish.ErrPublishContentTooLarge)
	}
	return data, nil
}
