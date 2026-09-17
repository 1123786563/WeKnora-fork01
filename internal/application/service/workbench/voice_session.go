package workbench

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

/**
 * 语音会话服务端读模型（MX-029，voice）。
 * - 媒体会话与产品 Run 绑定（mapping 持久）；短期媒体令牌（过期重授权，无长期密钥）；
 * - 用量上报与会话结束解耦：迟到用量仍入账（沿既有商业结算入口，不建新钱包）；
 * - 语音不携带审批权（决定只走 interactions 通道）。
 */

// VoiceSessionRow 映射 workbench_voice_sessions 表。
type VoiceSessionRow struct {
	TenantID  uint64
	SessionID string
	RunID     string
	OwnerID   string
	// 短期媒体令牌（产品 API 签发；过期重授权——服务端不存明文长期密钥）
	MediaTokenHash string
	TokenExpiresAt time.Time
	OutputActive   bool
	SessionActive  bool
	CreatedAt      time.Time
	EndedAt        *time.Time
}

func (VoiceSessionRow) TableName() string { return "workbench_voice_sessions" }

// VoiceUsageRow 映射 workbench_voice_usage（迟到用量仍可入账）。
type VoiceUsageRow struct {
	TenantID   uint64
	SessionID  string
	Units      int64
	ReportedAt time.Time
	Settled    bool
}

func (VoiceUsageRow) TableName() string { return "workbench_voice_usage" }

var ErrVoiceSessionExpired = errors.New("voice_media_token_expired")

type VoiceSessionService struct {
	db    *gorm.DB
	clock func() time.Time
}

func NewWorkbenchVoiceSessionService(db *gorm.DB, clock func() time.Time) *VoiceSessionService {
	if clock == nil {
		clock = time.Now
	}
	return &VoiceSessionService{db: db, clock: clock}
}

// Authorize 为 Run 签发短期媒体会话（token 过期时间短；客户端只拿短期凭据）。
func (s *VoiceSessionService) Authorize(ctx context.Context, tenantID uint64, ownerID, runID, mediaTokenHash string, ttl time.Duration) (VoiceSessionRow, error) {
	if tenantID == 0 || ownerID == "" || runID == "" || mediaTokenHash == "" {
		return VoiceSessionRow{}, errors.New("identity, run and token required")
	}
	now := s.clock()
	row := VoiceSessionRow{
		TenantID: tenantID, SessionID: "vs-" + runID + "-" + now.UTC().Format("150405.000000000"),
		RunID: runID, OwnerID: ownerID, MediaTokenHash: mediaTokenHash,
		TokenExpiresAt: now.Add(ttl), OutputActive: true, SessionActive: true, CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return VoiceSessionRow{}, err
	}
	return row, nil
}

// ValidateToken 短期令牌校验（过期→ErrVoiceSessionExpired，客户端重授权）。
func (s *VoiceSessionService) ValidateToken(ctx context.Context, tenantID uint64, sessionID, mediaTokenHash string) (VoiceSessionRow, error) {
	var row VoiceSessionRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VoiceSessionRow{}, gorm.ErrRecordNotFound
	}
	if err != nil {
		return VoiceSessionRow{}, err
	}
	if !row.SessionActive {
		return VoiceSessionRow{}, errors.New("voice_session_ended")
	}
	if row.MediaTokenHash != mediaTokenHash || s.clock().After(row.TokenExpiresAt) {
		return VoiceSessionRow{}, ErrVoiceSessionExpired
	}
	return row, nil
}

// InterruptOutput 仅停止播报：输出位清零，会话与任务保持。
func (s *VoiceSessionService) InterruptOutput(ctx context.Context, tenantID uint64, sessionID string) error {
	return s.db.WithContext(ctx).Model(&VoiceSessionRow{}).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Update("output_active", false).Error
}

// EndSession 结束语音会话（令牌/流失效）；任务不受影响。
func (s *VoiceSessionService) EndSession(ctx context.Context, tenantID uint64, sessionID string) error {
	now := s.clock()
	return s.db.WithContext(ctx).Model(&VoiceSessionRow{}).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Updates(map[string]any{"session_active": false, "output_active": false, "ended_at": now}).Error
}

// ReportUsage 迟到用量入账（会话结束不丢弃——结算沿既有入口）。
func (s *VoiceSessionService) ReportUsage(ctx context.Context, tenantID uint64, sessionID string, units int64) error {
	if units < 0 {
		return errors.New("usage units must be non-negative")
	}
	return s.db.WithContext(ctx).Create(&VoiceUsageRow{
		TenantID: tenantID, SessionID: sessionID, Units: units, ReportedAt: s.clock(),
	}).Error
}
