package service

import (
	"context"
	"encoding/json"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubTenantRepoForHistory serves exactly one tenant row (or an error) for
// CheckQueryHistoryAccess / ListSessions policy tests.
type stubTenantRepoForHistory struct {
	interfaces.TenantRepository
	tenant *types.Tenant
	err    error
}

func (s *stubTenantRepoForHistory) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return s.tenant, s.err
}

func tenantWithQueryHistoryMode(t *testing.T, mode string) *types.Tenant {
	t.Helper()
	cfg := &types.QueryHistoryConfig{Mode: mode}
	cfg.Normalize()
	return &types.Tenant{ID: 1, QueryHistoryConfig: cfg}
}

func TestCheckQueryHistoryAccessModes(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name        string
		repo        *stubTenantRepoForHistory
		wantMode    string
		wantErr     bool
		wantErrCode apperrors.ErrorCode
		wantErrMsg  string
	}{
		{
			name:     "nil repo falls back to normal",
			repo:     nil,
			wantMode: types.QueryHistoryModeNormal,
		},
		{
			name:     "tenant without config is normal",
			repo:     &stubTenantRepoForHistory{tenant: &types.Tenant{ID: 1}},
			wantMode: types.QueryHistoryModeNormal,
		},
		{
			name:     "nil tenant row is normal",
			repo:     &stubTenantRepoForHistory{tenant: nil},
			wantMode: types.QueryHistoryModeNormal,
		},
		{
			name:     "corrupt mode normalizes to normal",
			repo:     &stubTenantRepoForHistory{tenant: &types.Tenant{ID: 1, QueryHistoryConfig: &types.QueryHistoryConfig{Mode: "bogus"}}},
			wantMode: types.QueryHistoryModeNormal,
		},
		{
			name:     "normal passes through",
			repo:     &stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeNormal)},
			wantMode: types.QueryHistoryModeNormal,
		},
		{
			name:     "anonymized passes through with mode",
			repo:     &stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeAnonymized)},
			wantMode: types.QueryHistoryModeAnonymized,
		},
		{
			name:       "disabled is forbidden",
			repo:       &stubTenantRepoForHistory{tenant: tenantWithQueryHistoryMode(t, types.QueryHistoryModeDisabled)},
			wantMode:   types.QueryHistoryModeDisabled,
			wantErr:    true,
			wantErrCode: apperrors.ErrForbidden,
			wantErrMsg: "query history is disabled for this tenant",
		},
		{
			name:    "repository error propagates",
			repo:    &stubTenantRepoForHistory{err: context.DeadlineExceeded},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var repo interfaces.TenantRepository
			if tc.repo != nil {
				repo = tc.repo
			}
			mode, err := CheckQueryHistoryAccess(ctx, repo, 1)
			if !tc.wantErr {
				require.NoError(t, err)
				require.Equal(t, tc.wantMode, mode)
				return
			}
			require.Error(t, err)
			if tc.wantErrCode != 0 {
				var appErr *apperrors.AppError
				require.ErrorAs(t, err, &appErr)
				require.Equal(t, tc.wantErrCode, appErr.Code)
				require.Contains(t, appErr.Message, tc.wantErrMsg)
			}
		})
	}
}

func TestAnonymizeSessionOwner(t *testing.T) {
	rows := []*types.SessionListItem{
		{Session: types.Session{ID: "s1", UserID: "alice"}, IMUserID: "im-alice"},
		{Session: types.Session{ID: "s2", UserID: types.SessionOwnerAPITenantKeyPrefix + "1:10"}, IMUserID: "im-bob"},
		nil,
	}

	// Anonymized masks every owner id and clears the IM principal id,
	// tolerating nil rows.
	AnonymizeSessionOwner(types.QueryHistoryModeAnonymized, rows)
	require.Equal(t, "anonymous", rows[0].UserID)
	require.Equal(t, "anonymous", rows[1].UserID)
	require.Equal(t, "", rows[0].IMUserID, "anonymized clears the IM principal id")
	require.Equal(t, "", rows[1].IMUserID)

	// IMUserID is omitempty on the wire: once cleared it disappears from the
	// serialized listing row instead of lingering as an empty placeholder.
	wire, err := json.Marshal(rows[0])
	require.NoError(t, err)
	require.NotContains(t, string(wire), "im_user_id")
	require.Contains(t, string(wire), `"user_id":"anonymous"`)

	// Any other mode leaves rows untouched.
	rows2 := []*types.SessionListItem{{Session: types.Session{ID: "s3", UserID: "bob"}, IMUserID: "im-bob"}}
	AnonymizeSessionOwner(types.QueryHistoryModeNormal, rows2)
	require.Equal(t, "bob", rows2[0].UserID)
	require.Equal(t, "im-bob", rows2[0].IMUserID)
	AnonymizeSessionOwner(types.QueryHistoryModeDisabled, rows2)
	require.Equal(t, "bob", rows2[0].UserID)
	require.Equal(t, "im-bob", rows2[0].IMUserID)
}
