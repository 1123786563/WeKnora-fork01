package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type feedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) interfaces.FeedbackRepository {
	return &feedbackRepository{db: db}
}

func (r *feedbackRepository) UpsertFeedback(ctx context.Context, fb *types.MessageFeedback) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "comment", "updated_at"}),
	}).Create(fb).Error
}

func (r *feedbackRepository) RemoveFeedback(ctx context.Context, tenantID uint64, messageID, userID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND message_id = ? AND user_id = ?", tenantID, messageID, userID).
		Delete(&types.MessageFeedback{}).Error
}

func (r *feedbackRepository) ListBySessionAndUser(ctx context.Context, tenantID uint64, sessionID, userID string) ([]types.MessageFeedback, error) {
	var list []types.MessageFeedback
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND user_id = ?", tenantID, sessionID, userID).
		Find(&list).Error
	return list, err
}
