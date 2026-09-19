package repository

// Session-share repository tests (SP13 Task 5): SetShareToken
// set/rotate/revoke semantics with tenant scoping, GetByShareToken's
// tenant + soft-delete + revocation filters, and the partial unique index
// that keeps one token bound to at most one session.

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestSessionRepoSetAndGetShareToken(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	session := createSessionForTest(t, db, 1, "alice")

	// Mint.
	token := "tok-mint-1"
	affected, err := repo.SetShareToken(ctx, 1, session.ID, &token)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)

	got, err := repo.GetByShareToken(ctx, 1, "tok-mint-1")
	require.NoError(t, err)
	require.Equal(t, session.ID, got.ID)
	require.Equal(t, "tok-mint-1", got.ShareToken)

	// Rotate: plain overwrite, one affected row.
	next := "tok-mint-2"
	affected, err = repo.SetShareToken(ctx, 1, session.ID, &next)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	got, err = repo.GetByShareToken(ctx, 1, "tok-mint-1")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound, "the rotated (old) token stops resolving")
	got, err = repo.GetByShareToken(ctx, 1, "tok-mint-2")
	require.NoError(t, err)
	require.Equal(t, session.ID, got.ID)

	// Revoke: NULL kills the link.
	affected, err = repo.SetShareToken(ctx, 1, session.ID, nil)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	_, err = repo.GetByShareToken(ctx, 1, "tok-mint-2")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound, "the revoked token stops resolving")

	var stored *string
	require.NoError(t, db.Raw("SELECT share_token FROM sessions WHERE id = ?", session.ID).
		Scan(&stored).Error)
	require.Nil(t, stored, "revoke writes SQL NULL")
}

func TestSessionRepoShareTokenTenantScopeAndMisses(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	session := createSessionForTest(t, db, 1, "alice")
	token := "tok-x"
	_, err := repo.SetShareToken(ctx, 1, session.ID, &token)
	require.NoError(t, err)

	// Cross-tenant probe: same token, other tenant -> plain miss.
	_, err = repo.GetByShareToken(ctx, 2, "tok-x")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	// Unknown id / unknown token -> 0 rows / miss.
	affected, err := repo.SetShareToken(ctx, 1, "no-such-session", &token)
	require.NoError(t, err)
	require.EqualValues(t, 0, affected)
	affected, err = repo.SetShareToken(ctx, 2, session.ID, &token)
	require.NoError(t, err)
	require.EqualValues(t, 0, affected, "cross-tenant write touches nothing")
	_, err = repo.GetByShareToken(ctx, 1, "tok-unknown")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	// Empty token never matches (guards against share_token = '' rows).
	_, err = repo.GetByShareToken(ctx, 1, "")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestSessionRepoGetByShareTokenSkipsSoftDeleted(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	session := createSessionForTest(t, db, 1, "alice")
	token := "tok-live"
	_, err := repo.SetShareToken(ctx, 1, session.ID, &token)
	require.NoError(t, err)

	// Soft-delete the session (the production delete path); the token must
	// stop resolving even though the row keeps its share_token value.
	require.NoError(t, db.Delete(&types.Session{}, "id = ?", session.ID).Error)
	_, err = repo.GetByShareToken(ctx, 1, "tok-live")
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound,
		"a soft-deleted session's share link dies with the session")
}

func TestSessionRepoShareTokenUniqueAcrossSessions(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()

	// Create through the repository (the production path): fresh rows must
	// land on the column's NULL default — Create omits share_token because a
	// zero-value '' would collide under the partial index below and break
	// every session INSERT after the first.
	mkSession := func(userID string) *types.Session {
		s, err := repo.Create(ctx, &types.Session{TenantID: 1, UserID: userID, Title: userID})
		require.NoError(t, err)
		return s
	}
	one := mkSession("alice")
	two := mkSession("bob")

	var nulls int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM sessions WHERE share_token IS NULL").Scan(&nulls).Error)
	require.EqualValues(t, 2, nulls, "created sessions store NULL, not zero-value ''")

	// Recreate the production partial unique index (migration 000170).
	require.NoError(t, db.Exec(
		"CREATE UNIQUE INDEX uq_sessions_share_token ON sessions (share_token) " +
			"WHERE share_token IS NOT NULL AND share_token <> ''",
	).Error)

	token := "tok-clash"
	_, err := repo.SetShareToken(ctx, 1, one.ID, &token)
	require.NoError(t, err)

	_, err = repo.SetShareToken(ctx, 1, two.ID, &token)
	require.Error(t, err, "a second session cannot hold the same token")
	require.True(t, isUniqueViolation(err), "the failure is the unique index, got: %v", err)

	// Distinct tokens coexist, and NULL (revoked) rows never collide.
	other := "tok-other"
	_, err = repo.SetShareToken(ctx, 1, two.ID, &other)
	require.NoError(t, err)
	_, err = repo.SetShareToken(ctx, 1, two.ID, nil)
	require.NoError(t, err)
	_, err = repo.SetShareToken(ctx, 1, one.ID, nil)
	require.NoError(t, err, "two NULL tokens never violate the partial index")
}
