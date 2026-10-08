package application

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/stretchr/testify/require"
)

// fakePolicyReader is the test double for ports.PolicyReader: it records the
// tenant ids it was consulted about and serves a fixed mode or error.
type fakePolicyReader struct {
	mode domain.Mode
	err  error

	modeCalls    int
	lastTenantID uint64
}

func (f *fakePolicyReader) Mode(_ context.Context, tenantID uint64) (domain.Mode, error) {
	f.modeCalls++
	f.lastTenantID = tenantID
	return f.mode, f.err
}

var _ ports.PolicyReader = (*fakePolicyReader)(nil)

// TestPolicyCheckAccessModes ports the legacy CheckQueryHistoryAccess mode
// table to the module's policy seam: normal and anonymized pass through with
// their mode, disabled answers the exact legacy forbidden AppError, and a
// policy lookup error propagates unchanged.
func TestPolicyCheckAccessModes(t *testing.T) {
	ctx := context.Background()
	errSentinel := errors.New("policy lookup failed")

	cases := []struct {
		name        string
		mode        domain.Mode
		policyErr   error
		wantMode    domain.Mode
		wantErr     bool
		wantErrCode apperrors.ErrorCode
		wantErrMsg  string
	}{
		{
			name:     "normal passes through",
			mode:     domain.Normal,
			wantMode: domain.Normal,
		},
		{
			name:     "anonymized passes through with mode",
			mode:     domain.Anonymized,
			wantMode: domain.Anonymized,
		},
		{
			name:        "disabled is forbidden",
			mode:        domain.Disabled,
			wantMode:    domain.Disabled,
			wantErr:     true,
			wantErrCode: apperrors.ErrForbidden,
			wantErrMsg:  "query history is disabled for this tenant",
		},
		{
			name:      "policy error propagates",
			policyErr: errSentinel,
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := &fakePolicyReader{mode: tc.mode, err: tc.policyErr}
			svc := NewAuditService(policy, &fakeAuditReader{})

			mode, err := svc.CheckAccess(ctx, 1)
			if !tc.wantErr {
				require.NoError(t, err)
				require.Equal(t, tc.wantMode, mode)
				require.Equal(t, 1, policy.modeCalls, "the policy reader is consulted exactly once")
				require.Equal(t, uint64(1), policy.lastTenantID, "the caller's tenant id reaches the policy reader")
				return
			}
			require.Error(t, err)
			require.Equal(t, tc.wantMode, mode)
			if tc.wantErrCode != 0 {
				var appErr *apperrors.AppError
				require.ErrorAs(t, err, &appErr)
				require.Equal(t, tc.wantErrCode, appErr.Code)
				require.Equal(t, tc.wantErrMsg, appErr.Message, "the forbidden text is byte-identical to the legacy error")
			} else {
				require.ErrorIs(t, err, errSentinel)
			}
		})
	}
}

// TestPolicyCheckAccessZeroTenant covers the missing/zero tenant validation:
// the exact legacy error text, a plain (non-AppError) error, and no policy
// lookup for an unscoped request.
func TestPolicyCheckAccessZeroTenant(t *testing.T) {
	policy := &fakePolicyReader{mode: domain.Normal}
	svc := NewAuditService(policy, &fakeAuditReader{})

	mode, err := svc.CheckAccess(context.Background(), 0)
	require.Error(t, err)
	require.EqualError(t, err, "workspace id is required")
	require.Empty(t, mode)
	require.Equal(t, 0, policy.modeCalls, "a zero tenant id never reaches the policy reader")

	// Byte-identical to the legacy stderrors.New validation failure: not an
	// AppError, so it maps to a plain 500 as before.
	var appErr *apperrors.AppError
	require.False(t, errors.As(err, &appErr))
}
