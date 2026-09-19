package interfaces

import (
	"context"
	"time"
)

// QueryTrendPoint is one UTC day of the question-volume vs feedback trend:
// how many user questions were asked and how many of the answers collected
// likes / dislikes.
type QueryTrendPoint struct {
	Date     string `json:"date"`
	Queries  int64  `json:"queries"`
	Likes    int64  `json:"likes"`
	Dislikes int64  `json:"dislikes"`
}

// ActiveUsersPoint is one UTC day of the distinct-session-owners count.
type ActiveUsersPoint struct {
	Date        string `json:"date"`
	ActiveUsers int64  `json:"active_users"`
}

// ChannelSessionsPoint is one UTC day of new sessions broken down by source
// bucket ("web", "api", "embed"; empty source buckets render as "web").
type ChannelSessionsPoint struct {
	Date     string `json:"date"`
	Source   string `json:"source"`
	Sessions int64  `json:"sessions"`
}

// AgentUsagePoint is one UTC day of one custom agent's assistant messages
// and the distinct session owners that triggered them.
type AgentUsagePoint struct {
	Date        string `json:"date"`
	Messages    int64  `json:"messages"`
	UniqueUsers int64  `json:"unique_users"`
}

// AnalyticsRepository serves the console analytics dashboard with read-time
// aggregations over sessions / messages / message_feedback. All windows are
// half-open: [from, to).
type AnalyticsRepository interface {
	// QueryTrend counts user questions per day plus like/dislike feedback
	// rows attached to the day's messages.
	QueryTrend(ctx context.Context, tenantID uint64, from, to time.Time) ([]QueryTrendPoint, error)
	// ActiveUsers counts distinct session owners that asked at least one
	// question per day.
	ActiveUsers(ctx context.Context, tenantID uint64, from, to time.Time) ([]ActiveUsersPoint, error)
	// ChannelSessions counts newly created sessions per day and source bucket.
	ChannelSessions(ctx context.Context, tenantID uint64, from, to time.Time) ([]ChannelSessionsPoint, error)
	// AgentMessages counts one agent's assistant messages and distinct users per day.
	AgentMessages(ctx context.Context, tenantID uint64, agentID string, from, to time.Time) ([]AgentUsagePoint, error)
}
