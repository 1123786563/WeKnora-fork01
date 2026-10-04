package commercial

import (
	"context"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// ParkedGateway is the #105 [Lago 33] end-state of the CommercialGateway
// rail: the OpenMeter official_v3 adapter and its deployment are REMOVED,
// and until the Lago usage-event channel lands (residual R-101c — the
// per-call Settle/ConfirmSettlement ingestion into Lago), the only
// provider behind this seam is a fail-closed placeholder with exactly the
// posture the unconfigured OpenMeter gateway had: construction always
// succeeds, every call returns domain.ErrGatewayUnconfigured, boot never
// depends on it, and recovery keeps records pending/attention instead of
// guessing. Environment selection for the old gateway no longer exists;
// there is no runtime configuration that can reach OpenMeter.
type ParkedGateway struct{}

// NewParkedGateway keeps the provider a plain value so dig can wire it.
func NewParkedGateway() *ParkedGateway { return &ParkedGateway{} }

// compile-time proof the parked rail still satisfies the seam.
var _ domain.CommercialGateway = (*ParkedGateway)(nil)

func (g *ParkedGateway) ApplyBenefit(_ context.Context, _ domain.BenefitRequest) (domain.BenefitReceipt, error) {
	return domain.BenefitReceipt{}, domain.ErrGatewayUnconfigured
}

func (g *ParkedGateway) FindBenefit(_ context.Context, _ string) (domain.BenefitReceipt, error) {
	return domain.BenefitReceipt{}, domain.ErrGatewayUnconfigured
}

func (g *ParkedGateway) RevokeBenefit(_ context.Context, _ string, _ domain.Credits) error {
	return domain.ErrGatewayUnconfigured
}

func (g *ParkedGateway) Settle(_ context.Context, _ domain.Settlement) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{}, domain.ErrGatewayUnconfigured
}

func (g *ParkedGateway) ConfirmSettlement(_ context.Context, _ string) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{}, domain.ErrGatewayUnconfigured
}
