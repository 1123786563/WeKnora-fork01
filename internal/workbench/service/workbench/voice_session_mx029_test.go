package workbench

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MX-029 服务端：短期令牌/三停语义/迟到用量（voice session mapping）。
func TestMX029VoiceSessionLifecycle(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/voice.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&VoiceSessionRow{}, &VoiceUsageRow{}))
	base := time.Now().UTC()
	clock := base
	service := NewWorkbenchVoiceSessionService(db, func() time.Time { return clock })

	row, err := service.Authorize(context.Background(), 7, "u1", "run-1", "hash-token", 10*time.Minute)
	require.NoError(t, err)
	require.True(t, row.SessionActive)
	require.True(t, row.OutputActive)
	require.True(t, row.TokenExpiresAt.After(base), "token must be short-lived but valid")

	// 令牌校验（有效）
	valid, err := service.ValidateToken(context.Background(), 7, row.SessionID, "hash-token")
	require.NoError(t, err)
	require.Equal(t, "run-1", valid.RunID)

	// 停止播报：会话保持（仅输出停止）
	require.NoError(t, service.InterruptOutput(context.Background(), 7, row.SessionID))
	afterInterrupt, err := service.ValidateToken(context.Background(), 7, row.SessionID, "hash-token")
	require.NoError(t, err)
	require.True(t, afterInterrupt.SessionActive, "interrupt must not end the session")

	// 结束会话：后续令牌校验失败
	require.NoError(t, service.EndSession(context.Background(), 7, row.SessionID))
	_, err = service.ValidateToken(context.Background(), 7, row.SessionID, "hash-token")
	require.Error(t, err, "ended session must reject token validation")

	// 迟到用量：会话结束后仍入账（结算解耦）
	require.NoError(t, service.ReportUsage(context.Background(), 7, row.SessionID, 120))
	var usage []VoiceUsageRow
	require.NoError(t, db.Where("session_id = ?", row.SessionID).Find(&usage).Error)
	require.Len(t, usage, 1)
	require.Equal(t, int64(120), usage[0].Units)

	// 过期令牌：时钟前进 → ErrVoiceSessionExpired
	row2, err := service.Authorize(context.Background(), 7, "u1", "run-2", "hash-2", time.Minute)
	require.NoError(t, err)
	clock = base.Add(2 * time.Minute)
	_, err = service.ValidateToken(context.Background(), 7, row2.SessionID, "hash-2")
	require.ErrorIs(t, err, ErrVoiceSessionExpired)
}
