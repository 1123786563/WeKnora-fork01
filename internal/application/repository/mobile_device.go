package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMobileDeviceNotFound = errors.New("mobile device not found")
	ErrMobileDeviceRevision = errors.New("mobile device revision conflict")
	ErrMobileDeviceInvalid  = errors.New("invalid mobile device registration")
)

// DeviceTokenHash returns the non-reversible lookup key for a push token. The
// token itself is sealed by the HTTP boundary before it reaches this store.
func DeviceTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// DeviceRegistration is the durable, tenant-scoped identity of one native
// installation. TokenCiphertext is deliberately excluded from JSON responses
// and is never logged. TenantID is optional only for backwards-compatible
// repository callers; HTTP callers must always provide it from auth context.
type DeviceRegistration struct {
	TenantID        uint64     `json:"-"`
	SpaceID         string     `json:"-"`
	DeviceID        string     `json:"device_id"`
	OwnerID         string     `json:"owner_id"`
	Environment     string     `json:"environment"`
	Platform        string     `json:"platform"`
	TokenCiphertext string     `json:"-"`
	TokenHash       string     `json:"-"`
	Revision        int64      `json:"revision"`
	ScopeGeneration int64      `json:"scope_generation"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
}

type mobileDeviceRow struct {
	TenantID        uint64     `gorm:"primaryKey"`
	OwnerID         string     `gorm:"primaryKey;type:varchar(512)"`
	DeviceID        string     `gorm:"primaryKey;type:varchar(128)"`
	Environment     string     `gorm:"primaryKey;type:varchar(32)"`
	SpaceID         string     `gorm:"type:varchar(128);not null;default:''"`
	Platform        string     `gorm:"type:varchar(16);not null"`
	TokenCiphertext string     `gorm:"type:text;not null"`
	TokenHash       string     `gorm:"type:varchar(64);not null"`
	Revision        int64      `gorm:"not null"`
	ScopeGeneration int64      `gorm:"not null;default:0"`
	RevokedAt       *time.Time `gorm:"index"`
	LastSeenAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (mobileDeviceRow) TableName() string { return "mobile_devices" }

func (r mobileDeviceRow) registration() DeviceRegistration {
	return DeviceRegistration{
		TenantID: r.TenantID, SpaceID: r.SpaceID, DeviceID: r.DeviceID,
		OwnerID: r.OwnerID, Environment: r.Environment, Platform: r.Platform,
		TokenCiphertext: r.TokenCiphertext, TokenHash: r.TokenHash,
		Revision: r.Revision, ScopeGeneration: r.ScopeGeneration,
		RevokedAt: r.RevokedAt, LastSeenAt: r.LastSeenAt,
	}
}

// MobileDeviceStore persists device bindings in the configured application
// environment. Keeping environment in every predicate prevents development
// registrations from being reused by a production worker.
type MobileDeviceStore struct {
	db          *gorm.DB
	environment string
}

func NewMobileDeviceStore(db *gorm.DB, environment string) *MobileDeviceStore {
	return &MobileDeviceStore{db: db, environment: strings.TrimSpace(environment)}
}

func (s *MobileDeviceStore) validate(in DeviceRegistration) error {
	if s == nil || s.db == nil || s.environment == "" || in.Environment != s.environment ||
		strings.TrimSpace(in.DeviceID) == "" || strings.TrimSpace(in.OwnerID) == "" ||
		strings.TrimSpace(in.TokenCiphertext) == "" || strings.TrimSpace(in.TokenHash) == "" ||
		strings.TrimSpace(in.Platform) == "" || in.Revision < 0 || in.ScopeGeneration < 0 {
		return ErrMobileDeviceInvalid
	}
	if in.Platform != "ios" && in.Platform != "android" {
		return ErrMobileDeviceInvalid
	}
	return nil
}

func (s *MobileDeviceStore) scoped(db *gorm.DB, tenantID uint64, ownerID, deviceID string) *gorm.DB {
	q := db.Where("environment = ? AND owner_id = ? AND device_id = ?", s.environment, ownerID, deviceID)
	if tenantID != 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return q
}

// Bind is an idempotent registration and token-rotation operation. A token
// actively bound to another owner is revoked in the same transaction before
// this binding is written. A stale rebind cannot resurrect a revoked row.
func (s *MobileDeviceStore) Bind(ctx context.Context, in DeviceRegistration) error {
	if in.TokenHash == "" && in.TokenCiphertext != "" {
		// Repository-only callers may already have a sealed token but no
		// separate lookup hash. HTTP callers always provide the hash of the
		// plaintext token so rotations remain exclusive across ciphertexts.
		in.TokenHash = DeviceTokenHash(in.TokenCiphertext)
	}
	if err := s.validate(in); err != nil {
		return err
	}
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		err = s.bindOnce(ctx, in)
		if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "locked") && !strings.Contains(strings.ToLower(err.Error()), "busy")) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 10 * time.Millisecond):
		}
	}
	return err
}

func (s *MobileDeviceStore) bindOnce(ctx context.Context, in DeviceRegistration) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing mobileDeviceRow
		err := s.scoped(tx, in.TenantID, in.OwnerID, in.DeviceID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if existing.RevokedAt != nil && in.Revision <= existing.Revision {
				return ErrMobileDeviceRevision
			}
			if existing.RevokedAt == nil && in.Revision > 0 && in.Revision < existing.Revision {
				return ErrMobileDeviceRevision
			}
			// Repeating the same registration is a true idempotent no-op.
			if existing.RevokedAt == nil && existing.TokenHash == in.TokenHash &&
				existing.Platform == in.Platform && existing.ScopeGeneration == in.ScopeGeneration {
				return nil
			}
		}

		// Token ownership is exclusive within an app environment. This query is
		// intentionally by hash only; plaintext tokens never enter SQL or logs.
		var sameToken []mobileDeviceRow
		if err := tx.Where("environment = ? AND token_hash = ? AND revoked_at IS NULL", s.environment, in.TokenHash).
			Clauses(clause.Locking{Strength: "UPDATE"}).Find(&sameToken).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, row := range sameToken {
			if row.TenantID == in.TenantID && row.OwnerID == in.OwnerID && row.DeviceID == in.DeviceID {
				continue
			}
			if err := tx.Model(&mobileDeviceRow{}).Where("tenant_id = ? AND owner_id = ? AND device_id = ? AND environment = ? AND revision = ? AND revoked_at IS NULL",
				row.TenantID, row.OwnerID, row.DeviceID, s.environment, row.Revision).
				Updates(map[string]any{"revoked_at": now, "revision": row.Revision + 1, "updated_at": now}).Error; err != nil {
				return err
			}
		}

		if err == nil {
			next := existing.Revision + 1
			if existing.Revision == 0 {
				next = 1
			}
			return tx.Model(&mobileDeviceRow{}).Where("tenant_id = ? AND owner_id = ? AND device_id = ? AND environment = ? AND revision = ?",
				existing.TenantID, existing.OwnerID, existing.DeviceID, existing.Environment, existing.Revision).
				Updates(map[string]any{
					"space_id": in.SpaceID, "platform": in.Platform, "token_ciphertext": in.TokenCiphertext,
					"token_hash": in.TokenHash, "revision": next, "scope_generation": in.ScopeGeneration,
					"revoked_at": nil, "updated_at": now,
				}).Error
		}

		revision := in.Revision
		if revision == 0 {
			revision = 1
		}
		row := mobileDeviceRow{TenantID: in.TenantID, OwnerID: in.OwnerID, DeviceID: in.DeviceID,
			Environment: s.environment, SpaceID: in.SpaceID, Platform: in.Platform,
			TokenCiphertext: in.TokenCiphertext, TokenHash: in.TokenHash, Revision: revision,
			ScopeGeneration: in.ScopeGeneration, CreatedAt: now, UpdatedAt: now}
		return tx.Create(&row).Error
	})
}

// Revoke performs an owner-scoped compare-and-swap. A zero expected revision
// means "revoke the currently active revision" and is used only for an
// idempotent logout; callers with a known revision get stale-write detection.
func (s *MobileDeviceStore) Revoke(ctx context.Context, owner, device string, revision int64) error {
	return s.revoke(ctx, 0, owner, device, revision)
}

func (s *MobileDeviceStore) RevokeForTenant(ctx context.Context, tenant uint64, owner, device string, revision int64) error {
	return s.revoke(ctx, tenant, owner, device, revision)
}

func (s *MobileDeviceStore) revoke(ctx context.Context, tenant uint64, owner, device string, revision int64) error {
	if s == nil || s.db == nil || s.environment == "" || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return ErrMobileDeviceInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row mobileDeviceRow
		q := s.scoped(tx, tenant, owner, device).Clauses(clause.Locking{Strength: "UPDATE"})
		if err := q.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMobileDeviceNotFound
			}
			return err
		}
		if row.RevokedAt != nil {
			if revision > 0 && revision < row.Revision {
				return ErrMobileDeviceRevision
			}
			return nil
		}
		if revision > 0 && revision != row.Revision {
			return ErrMobileDeviceRevision
		}
		now := time.Now().UTC()
		if updated := s.scoped(tx.Model(&mobileDeviceRow{}), tenant, owner, device).Where("revision = ? AND revoked_at IS NULL", row.Revision).
			Updates(map[string]any{"revoked_at": now, "revision": row.Revision + 1, "updated_at": now}); updated.Error != nil {
			return updated.Error
		} else if updated.RowsAffected != 1 {
			return ErrMobileDeviceRevision
		}
		return nil
	})
}

func (s *MobileDeviceStore) ListActive(ctx context.Context, owner, environment string) ([]DeviceRegistration, error) {
	if strings.TrimSpace(environment) == "" || environment != s.environment {
		return nil, ErrMobileDeviceInvalid
	}
	return s.list(ctx, 0, owner)
}

func (s *MobileDeviceStore) ListActiveForTenant(ctx context.Context, tenant uint64, owner, environment string) ([]DeviceRegistration, error) {
	if strings.TrimSpace(environment) == "" || environment != s.environment || tenant == 0 || strings.TrimSpace(owner) == "" {
		return nil, ErrMobileDeviceInvalid
	}
	return s.list(ctx, tenant, owner)
}

func (s *MobileDeviceStore) GetActiveForTenant(ctx context.Context, tenant uint64, owner, device string) (DeviceRegistration, error) {
	if tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return DeviceRegistration{}, ErrMobileDeviceInvalid
	}
	var row mobileDeviceRow
	err := s.scoped(s.db.WithContext(ctx), tenant, owner, device).Where("revoked_at IS NULL").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeviceRegistration{}, ErrMobileDeviceNotFound
	}
	if err != nil {
		return DeviceRegistration{}, err
	}
	return row.registration(), nil
}

func (s *MobileDeviceStore) list(ctx context.Context, tenant uint64, owner string) ([]DeviceRegistration, error) {
	if s == nil || s.db == nil || strings.TrimSpace(owner) == "" {
		return nil, ErrMobileDeviceInvalid
	}
	var rows []mobileDeviceRow
	q := s.db.WithContext(ctx).Where("environment = ? AND owner_id = ? AND revoked_at IS NULL", s.environment, owner)
	if tenant != 0 {
		q = q.Where("tenant_id = ?", tenant)
	}
	if err := q.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]DeviceRegistration, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.registration())
	}
	return result, nil
}

// RevokeBeforeScopeGeneration is used on account/space switches. It advances
// every older active binding atomically, so delayed token-refresh requests
// cannot resurrect a previous authenticated scope.
func (s *MobileDeviceStore) RevokeBeforeScopeGeneration(ctx context.Context, tenant uint64, owner string, generation int64) error {
	if tenant == 0 || strings.TrimSpace(owner) == "" || generation < 0 {
		return ErrMobileDeviceInvalid
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&mobileDeviceRow{}).
		Where("tenant_id = ? AND owner_id = ? AND environment = ? AND revoked_at IS NULL AND scope_generation < ?", tenant, owner, s.environment, generation).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (s *MobileDeviceStore) MarkPresence(ctx context.Context, tenant uint64, owner, device string, revision int64) error {
	if tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return ErrMobileDeviceInvalid
	}
	q := s.scoped(s.db.WithContext(ctx).Model(&mobileDeviceRow{}), tenant, owner, device).Where("revoked_at IS NULL")
	if revision > 0 {
		q = q.Where("revision = ?", revision)
	}
	result := q.Update("last_seen_at", time.Now().UTC())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrMobileDeviceNotFound
	}
	return nil
}

func (s *MobileDeviceStore) String() string {
	if s == nil {
		return "mobile-device-store(nil)"
	}
	return fmt.Sprintf("mobile-device-store(%s)", s.environment)
}
