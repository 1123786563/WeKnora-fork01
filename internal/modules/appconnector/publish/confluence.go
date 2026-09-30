package publish

// The Confluence publish service (T20 #50). R5-F12 collapsed the parallel
// service body onto the shared provider-neutral service over the
// Confluence profile — the same collapse feishu took in #49: the plan/
// receipt views, the publication store and the error sentinels were
// already shared verbatim; every provider decision now lives in
// ConfluenceProfile (provider.go) or the bridge layer (confluence_bridge.go).
// The service body itself is plan.go's NotionPublishService.

import (
	"context"

	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// ConfluenceRemoteReader is the plan-formation pre-read port (satisfied
// by *ConfluenceBridge.ReadConfluencePageVersion).
type ConfluenceRemoteReader interface {
	ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error)
}

// ConfluencePublishService is the SHARED provider-neutral publish service
// over the Confluence profile. The alias keeps the handler, container and
// test call sites compiling verbatim ("Notion" in the underlying name is
// the historical shared-service name, like ErrGitHubTransport for the
// provider-neutral code-platform family).
type ConfluencePublishService = NotionPublishService

// confluenceScopeAdapter adapts the Confluence scope source onto the shared
// service's NotionScopeSource port. The plan face consumes exactly
// AppID/AuthVersion/ApprovedParents; the Confluence-specific wire fields
// (edition, api base path, policy origin) stay in the Confluence types the
// bridge itself reads.
type confluenceScopeAdapter struct {
	src ConfluenceScopeSource
}

func (a confluenceScopeAdapter) NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) {
	s, err := a.src.ConfluenceScope(ctx, connectionID)
	if err != nil {
		return NotionConnectionScope{}, err
	}
	return NotionConnectionScope{
		AppID:           s.AppID,
		AuthVersion:     s.AuthVersion,
		ApprovedParents: s.ApprovedParents,
	}, nil
}

// NewConfluencePublishService builds the publish service over the
// Confluence profile — the thin #49-shape constructor (signature unchanged
// from the pre-collapse service).
func NewConfluencePublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote ConfluenceRemoteReader,
	scopes ConfluenceScopeSource,
) *ConfluencePublishService {
	return NewProviderPublishService(actions, store, pubs, artifacts, content,
		confluenceScopeAdapter{src: scopes}, ConfluenceProfile(remote))
}
