package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Voice session lifecycle states. `unknown` is the W30 issuance-timeout
// state: a provider open whose outcome could not be observed may still have
// opened a billable session, so the row is parked for reconciliation and no
// new charged session of the same product session may be opened until it is
// closed and settled.
const (
	VoiceSessionStateOpen    = "open"
	VoiceSessionStateUnknown = "unknown"
	VoiceSessionStateClosed  = "closed"
)

var (
	// ErrVoiceSessionInvalid rejects structurally invalid session rows.
	ErrVoiceSessionInvalid = errors.New("voice session row invalid")
	// ErrVoiceSessionNotFound answers a missing row AND a row owned by
	// another owner: cross-owner reads are indistinguishable from absent.
	ErrVoiceSessionNotFound = errors.New("voice session not found")
	// ErrVoiceSessionExists reports an idempotent create hitting an
	// already-recorded session (the caller reconciles instead of re-opening).
	ErrVoiceSessionExists = errors.New("voice session already exists")
	// ErrVoiceSessionUnknownOpen refuses a NEW charged session while an
	// un-reconciled unknown session of the same product session exists.
	ErrVoiceSessionUnknownOpen = errors.New("voice session unknown pending reconciliation")
	// ErrVoiceSessionStateConflict reports a CAS transition raced by another
	// writer.
	ErrVoiceSessionStateConflict = errors.New("voice session state conflict")
)

// VoiceSessionRow persists one authorized voice session: the product
// ownership scope (tenant/owner/session/run), the provider session mapping
// (provider_ref), the admission verdict (deadline/expires/max_seconds plus
// the commercial reservation key), and the settled usage. The grant token is
// stored ONLY as TokenHash — the plaintext token exists exactly once, in the
// creation response, and never in a log or a later read.
type VoiceSessionRow struct {
	TenantID       uint64     `gorm:"column:tenant_id;not null;uniqueIndex:uq_voice_session_identity,priority:1"`
	ID             string     `gorm:"column:id;not null;uniqueIndex:uq_voice_session_identity,priority:2"`
	OwnerID        string     `gorm:"column:owner_id;not null;index:idx_voice_session_owner,priority:2"`
	SessionID      string     `gorm:"column:session_id;not null;default:''"`
	RunID          string     `gorm:"column:run_id;not null;default:''"`
	ProviderRef    string     `gorm:"column:provider_ref;not null;default:''"`
	State          string     `gorm:"column:state;not null;default:'open'"`
	Deadline       time.Time  `gorm:"column:deadline;not null"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null"`
	MaxSeconds     int        `gorm:"column:max_seconds;not null;default:0"`
	ReservationKey string     `gorm:"column:reservation_key;not null;default:''"`
	TokenHash      string     `gorm:"column:token_hash;not null;default:''"`
	AudioSeconds   int64      `gorm:"column:audio_seconds;not null;default:0"`
	SettledAt      *time.Time `gorm:"column:settled_at"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP"`
}

func (VoiceSessionRow) TableName() string { return "voice_sessions" }

// VoiceSessionStore persists voice sessions in the business database scope.
type VoiceSessionStore struct{ db *gorm.DB }

// NewVoiceSessionStore builds the store over the business database.
func NewVoiceSessionStore(db *gorm.DB) *VoiceSessionStore { return &VoiceSessionStore{db: db} }

// HashVoiceToken reduces a grant token to its storable digest.
func HashVoiceToken(token string) string {
	return HashMobileExchangeValue(token)
}

// CreateVoiceSession records a newly authorized session idempotently.
// Replaying the same (tenant, id) returns the existing row with
// ErrVoiceSessionExists — the caller reconciles instead of opening a second
// charged session. A DIFFERENT id for the same product session is refused
// while an unknown-state row of that product session is pending
// reconciliation.
func (s *VoiceSessionStore) CreateVoiceSession(ctx context.Context, row VoiceSessionRow) (VoiceSessionRow, error) {
	if s == nil || s.db == nil ||
		row.TenantID == 0 || strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.OwnerID) == "" ||
		row.State == "" || row.Deadline.IsZero() || row.ExpiresAt.IsZero() || row.MaxSeconds <= 0 {
		return VoiceSessionRow{}, ErrVoiceSessionInvalid
	}
	var existing VoiceSessionRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(row.SessionID) != "" {
			var pending VoiceSessionRow
			if err := tx.Where("tenant_id = ? AND session_id = ? AND state = ?", row.TenantID, row.SessionID, VoiceSessionStateUnknown).
				Where("id <> ?", row.ID).First(&pending).Error; err == nil {
				return ErrVoiceSessionUnknownOpen
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := tx.Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).First(&existing).Error; err == nil {
			return ErrVoiceSessionExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		if errors.Is(err, ErrVoiceSessionExists) {
			return existing, err
		}
		return VoiceSessionRow{}, err
	}
	return row, nil
}

// GetOwnedVoiceSession resolves one session under the ownership predicate:
// a missing row and another owner's row both answer ErrVoiceSessionNotFound.
func (s *VoiceSessionStore) GetOwnedVoiceSession(ctx context.Context, tenantID uint64, ownerID, id string) (VoiceSessionRow, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(id) == "" {
		return VoiceSessionRow{}, ErrVoiceSessionInvalid
	}
	var row VoiceSessionRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VoiceSessionRow{}, ErrVoiceSessionNotFound
	}
	if err != nil {
		return VoiceSessionRow{}, err
	}
	return row, nil
}

// MarkVoiceSessionUnknown parks an issuance whose provider outcome could
// not be observed. The row keeps its reservation so settlement still runs.
func (s *VoiceSessionStore) MarkVoiceSessionUnknown(ctx context.Context, tenantID uint64, id string) error {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(id) == "" {
		return ErrVoiceSessionInvalid
	}
	result := s.db.WithContext(ctx).Model(&VoiceSessionRow{}).
		Where("tenant_id = ? AND id = ? AND state = ?", tenantID, id, VoiceSessionStateOpen).
		Update("state", VoiceSessionStateUnknown)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVoiceSessionNotFound
	}
	return nil
}

// PendingUnknownVoiceSession reports whether the product session still has
// an un-reconciled unknown-state voice session. Callers use it to refuse a
// NEW charged session before any budget or provider call.
func (s *VoiceSessionStore) PendingUnknownVoiceSession(ctx context.Context, tenantID uint64, sessionID string) (bool, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(sessionID) == "" {
		return false, ErrVoiceSessionInvalid
	}
	var count int64
	err := s.db.WithContext(ctx).Model(&VoiceSessionRow{}).
		Where("tenant_id = ? AND session_id = ? AND state = ?", tenantID, sessionID, VoiceSessionStateUnknown).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordVoiceSessionUsage accumulates trusted provider-reported usage
// (transcription durations, provider callbacks) onto a still-live session
// row. Closed rows take no further usage — their settlement is final.
func (s *VoiceSessionStore) RecordVoiceSessionUsage(ctx context.Context, tenantID uint64, id string, audioSeconds int64) error {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(id) == "" || audioSeconds < 0 {
		return ErrVoiceSessionInvalid
	}
	result := s.db.WithContext(ctx).Model(&VoiceSessionRow{}).
		Where("tenant_id = ? AND id = ? AND state <> ?", tenantID, id, VoiceSessionStateClosed).
		Updates(map[string]any{
			"audio_seconds": gorm.Expr("audio_seconds + ?", audioSeconds),
			"updated_at":    time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVoiceSessionNotFound
	}
	return nil
}

// CloseVoiceSessionWithUsage closes a session and records its settled
// usage atomically. It is idempotent: closing an already-closed row adds
// nothing and reports closed=false so the caller skips a second settlement
// round; usage on a live row only ever accumulates.
func (s *VoiceSessionStore) CloseVoiceSessionWithUsage(ctx context.Context, tenantID uint64, id string, audioSeconds int64, settledAt time.Time) (bool, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(id) == "" || audioSeconds < 0 {
		return false, ErrVoiceSessionInvalid
	}
	closed := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row VoiceSessionRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVoiceSessionNotFound
			}
			return err
		}
		if row.State == VoiceSessionStateClosed {
			return nil
		}
		updates := map[string]any{
			"state":         VoiceSessionStateClosed,
			"settled_at":    settledAt.UTC(),
			"audio_seconds": row.AudioSeconds + audioSeconds,
			"updated_at":    time.Now().UTC(),
		}
		result := tx.Model(&VoiceSessionRow{}).
			Where("tenant_id = ? AND id = ? AND state <> ?", tenantID, id, VoiceSessionStateClosed).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVoiceSessionStateConflict
		}
		closed = true
		return nil
	})
	return closed, err
}
