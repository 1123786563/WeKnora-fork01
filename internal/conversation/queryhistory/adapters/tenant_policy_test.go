package adapters

// Tenant-policy adapter tests (Wave 1, Task 7, brief Step 1): the
// ports.PolicyReader contract over the legacy TenantRepository. A nil repo, a
// missing tenant row, or a nil config all normalize to domain.Normal; an
// empty/unknown stored mode normalizes to Normal too (a corrupt row can never
// silently disable or de-identify workspace history); disabled is REPORTED as
// a mode, not refused — refusing is the application layer's call.

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubPolicyTenantRepo answers GetTenantByID from fixtures; the wide legacy
// interface is embedded so only the method the adapter consumes is provided.
type stubPolicyTenantRepo struct {
	interfaces.TenantRepository
	tenant *types.Tenant
	err    error
}

func (s *stubPolicyTenantRepo) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return s.tenant, s.err
}

func TestTenantPolicyMode(t *testing.T) {
	ctx := context.Background()
	tenantWith := func(mode string) *types.Tenant {
		return &types.Tenant{ID: 1, QueryHistoryConfig: &types.QueryHistoryConfig{Mode: mode}}
	}

	t.Run("nil repository normalizes to normal", func(t *testing.T) {
		mode, err := NewTenantPolicy(nil).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Normal, mode)
	})

	t.Run("nil tenant row normalizes to normal", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Normal, mode)
	})

	t.Run("nil config normalizes to normal", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{tenant: &types.Tenant{ID: 1}}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Normal, mode)
	})

	t.Run("empty mode normalizes to normal", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{tenant: tenantWith("")}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Normal, mode)
	})

	t.Run("unknown mode normalizes to normal", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{tenant: tenantWith("bogus")}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Normal, mode,
			"a corrupt row can never silently disable or de-identify workspace history")
	})

	t.Run("anonymized passes through", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{tenant: tenantWith(domain.Anonymized)}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Anonymized, mode)
	})

	t.Run("disabled passes through without an error", func(t *testing.T) {
		mode, err := NewTenantPolicy(&stubPolicyTenantRepo{tenant: tenantWith(domain.Disabled)}).Mode(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, domain.Disabled, mode,
			"the port reports the mode; refusing the read is the application layer's call")
	})

	t.Run("lookup error propagates unchanged", func(t *testing.T) {
		boom := errors.New("db down")
		_, err := NewTenantPolicy(&stubPolicyTenantRepo{err: boom}).Mode(ctx, 1)
		require.ErrorIs(t, err, boom)
	})
}
