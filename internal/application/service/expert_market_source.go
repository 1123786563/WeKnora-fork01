package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/experts"
)

// MarketExpertSource is the production ExpertSource: it scans each tenant's
// expert-market directory (the trees WriteMaterializedExpert produces) with
// the same scanner the builtin library uses. Scanning is per-request by
// design — installs are rare, listings are cheap directory reads, and a
// short request-time window of staleness after an install is the same
// trade-off Octop's ExpertCatalog makes.
type MarketExpertSource struct {
	dataRoot string
}

// NewMarketExpertSource builds the source over one expert-market data root;
// dataRoot is the tenant-parent directory (<dataRoot>/<tenantID>/<slug>).
func NewMarketExpertSource(dataRoot string) *MarketExpertSource {
	return &MarketExpertSource{dataRoot: dataRoot}
}

// InstalledExperts implements ExpertSource. Degraded by construction:
// experts.InstalledExperts never fails, so a missing or unreadable directory
// answers "nothing installed" rather than breaking the catalog.
func (s *MarketExpertSource) InstalledExperts(_ context.Context, tenantID uint64) []*experts.Expert {
	return experts.InstalledExperts(s.dataRoot, tenantID)
}

var _ ExpertSource = (*MarketExpertSource)(nil)
