package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrMobileExchangeInvalid = errors.New("mobile auth exchange is invalid or already consumed")

type MobileExchange struct {
	CodeHash    string    `gorm:"primaryKey;size:64"`
	StateHash   string    `gorm:"size:64;not null;index"`
	RedirectURI string    `gorm:"size:2048;not null"`
	Challenge   string    `gorm:"size:128;not null"`
	Subject     string    `gorm:"size:255;not null"`
	ExpiresAt   time.Time `gorm:"not null;index"`
	ConsumedAt  *time.Time
}

func (MobileExchange) TableName() string { return "mobile_auth_exchanges" }

type MobileExchangeStore struct{ db *gorm.DB }

func NewMobileExchangeStore(db *gorm.DB) *MobileExchangeStore { return &MobileExchangeStore{db: db} }

func HashMobileExchangeValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *MobileExchangeStore) Put(ctx context.Context, row MobileExchange) error {
	if s == nil || s.db == nil || strings.TrimSpace(row.CodeHash) == "" || strings.TrimSpace(row.StateHash) == "" || strings.TrimSpace(row.RedirectURI) == "" || strings.TrimSpace(row.Challenge) == "" || strings.TrimSpace(row.Subject) == "" || row.ExpiresAt.IsZero() {
		return ErrMobileExchangeInvalid
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// ConsumeMobileExchange atomically claims a short-lived exchange. It returns
// only the product subject; bearer credentials are never stored in this table.
func (s *MobileExchangeStore) ConsumeMobileExchange(ctx context.Context, codeHash, stateHash, redirectURI, challenge string, now time.Time) (string, error) {
	if s == nil || s.db == nil || codeHash == "" || stateHash == "" || strings.TrimSpace(redirectURI) == "" || strings.TrimSpace(challenge) == "" {
		return "", ErrMobileExchangeInvalid
	}
	var subject string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		consumed := now.UTC()
		result := tx.Model(&MobileExchange{}).
			Where("code_hash = ? AND state_hash = ? AND redirect_uri = ? AND challenge = ? AND consumed_at IS NULL AND expires_at > ?", codeHash, stateHash, redirectURI, challenge, now.UTC()).
			Updates(map[string]any{"consumed_at": consumed})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return ErrMobileExchangeInvalid
		}
		var row MobileExchange
		if err := tx.Where("code_hash = ?", codeHash).First(&row).Error; err != nil {
			return err
		}
		subject = row.Subject
		return nil
	})
	if err != nil {
		return "", ErrMobileExchangeInvalid
	}
	return subject, nil
}
