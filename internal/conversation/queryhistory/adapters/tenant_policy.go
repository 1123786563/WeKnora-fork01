package adapters

import (
	"context"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TenantPolicy satisfies ports.PolicyReader over the legacy TenantRepository:
// it answers the workspace's current query-history visibility mode. The
// normalization rules mirror the legacy CheckQueryHistoryAccess lookup — a
// nil repo (deployment wired without tenant lookup), a missing tenant row,
// or a nil config all answer domain.Normal, and an empty/unknown stored mode
// normalizes to Normal so a corrupt row can never silently disable or
// de-identify workspace history.
//
// Unlike the legacy helper, Mode does NOT refuse on domain.Disabled: the port
// reports the mode, and turning "disabled" into a 403 (or masking on
// "anonymized") is the application layer's call.
type TenantPolicy struct {
	tenants interfaces.TenantRepository
}

// NewTenantPolicy builds the policy adapter. A nil repository is accepted and
// normalizes every lookup to domain.Normal (legacy behavior).
func NewTenantPolicy(tenants interfaces.TenantRepository) *TenantPolicy {
	return &TenantPolicy{tenants: tenants}
}

// compile-time port conformance.
var _ ports.PolicyReader = (*TenantPolicy)(nil)

// Mode answers the workspace's current query-history visibility mode.
func (p *TenantPolicy) Mode(ctx context.Context, tenantID uint64) (domain.Mode, error) {
	// A nil repo means the deployment wired the module without tenant lookup;
	// the least-privacy-restrictive default (normal) keeps the audit surfaces
	// functional rather than failing every request.
	if p == nil || p.tenants == nil {
		return domain.Normal, nil
	}

	tenant, err := p.tenants.GetTenantByID(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if tenant == nil || tenant.QueryHistoryConfig == nil {
		return domain.Normal, nil
	}
	return domain.NormalizeMode(tenant.QueryHistoryConfig.Mode), nil
}
