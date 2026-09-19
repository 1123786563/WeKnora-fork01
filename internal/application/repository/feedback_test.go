package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

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
