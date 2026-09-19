package types

import "time"

const (
	FeedbackRatingLike    = "like"
	FeedbackRatingDislike = "dislike"
)

type MessageFeedback struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID  uint64    `json:"tenant_id" gorm:"index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(512);not null"`
	MessageID string    `json:"message_id" gorm:"type:varchar(36);not null"`
	SessionID string    `json:"session_id" gorm:"type:varchar(36);not null;default:''"`
	Rating    string    `json:"rating" gorm:"type:varchar(16);not null"`
	Comment   string    `json:"comment" gorm:"type:text;not null;default:''"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MessageFeedback) TableName() string { return "message_feedback" }

func IsValidFeedbackRating(rating string) bool {
	return rating == FeedbackRatingLike || rating == FeedbackRatingDislike
}
