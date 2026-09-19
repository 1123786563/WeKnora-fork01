package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestFeedbackListBySessionAcrossUsers pins the admin-audit read: every
// feedback row of one session inside the tenant, across users, never rows of
// another session, and never a same-session-id row recorded under another
// tenant.
func TestFeedbackListBySessionAcrossUsers(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			repo := NewFeedbackRepository(db)
			ctx := context.Background()
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingLike}))
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 1, UserID: "u2", MessageID: "m2", SessionID: "s1", Rating: types.FeedbackRatingDislike}))
			// 另一会话的行不得混入
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m3", SessionID: "s2", Rating: types.FeedbackRatingLike}))
			// 同 session id 落在其它租户的行不得跨租户泄漏
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 2, UserID: "u9", MessageID: "m9", SessionID: "s1", Rating: types.FeedbackRatingLike}))

			got, err := repo.ListBySession(ctx, 1, "s1")
			require.NoError(t, err)
			require.Len(t, got, 2, "ListBySession must return both users' rows of the session, tenant-scoped only")
			users := []string{got[0].UserID, got[1].UserID}
			require.ElementsMatch(t, []string{"u1", "u2"}, users)

			missing, err := repo.ListBySession(ctx, 1, "no-such-session")
			require.NoError(t, err)
			require.Empty(t, missing)
		})
	}
}

func TestFeedbackUpsertIsIdempotentPerUser(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			repo := NewFeedbackRepository(db)
			ctx := context.Background()
			fb1 := &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingLike}
			require.NoError(t, repo.UpsertFeedback(ctx, fb1))
			updated := &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingDislike, Comment: "bad"}
			require.NoError(t, repo.UpsertFeedback(ctx, updated)) // 同 (message,user) 应更新而非新增
			list, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u1")
			require.NoError(t, err)
			require.Len(t, list, 1)
			require.Equal(t, types.FeedbackRatingDislike, list[0].Rating)
			// 另一用户互不影响
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 1, UserID: "u2", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingLike}))
			all, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u2")
			require.NoError(t, err)
			require.Len(t, all, 1)
			// 移除
			require.NoError(t, repo.RemoveFeedback(ctx, 1, "m1", "u1"))
			after, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u1")
			require.NoError(t, err)
			require.Len(t, after, 0)
		})
	}
}
