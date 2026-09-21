package commercial

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrAccountMappingConflict = errors.New("account_mapping_conflict")

// Account is the durable one-to-one mapping between a tenant and its external
// billing Customer. Customer IDs are intentionally opaque strings.
type Account struct {
	TenantID   uint64 `gorm:"primaryKey;column:tenant_id"`
	CustomerID string `gorm:"column:customer_id;uniqueIndex;not null"`
	Version    int64  `gorm:"column:version;not null;default:1"`
}

func (Account) TableName() string { return "commercial_accounts" }

type Grant struct {
	TenantID   uint64 `gorm:"primaryKey;column:tenant_id"`
	UserID     string `gorm:"primaryKey;column:user_id"`
	Capability string `gorm:"primaryKey;column:capability"`
	GrantedBy  string `gorm:"column:granted_by;not null"`
	Version    int64  `gorm:"column:version;not null;default:1"`
}

func (Grant) TableName() string { return "commercial_grants" }

type AccountStore struct{ db *gorm.DB }

func NewAccountStore(db *gorm.DB) *AccountStore { return &AccountStore{db: db} }

// Bind is idempotent for an existing identical mapping. The database unique
// constraints serialize competing tenants and customers; no external call is
// made while holding a transaction.
func (s *AccountStore) Bind(ctx context.Context, a Account) error {
	if a.TenantID == 0 || a.CustomerID == "" {
		return ErrAccountMappingConflict
	}
	if a.Version == 0 {
		a.Version = 1
	}
	res := s.db.WithContext(ctx).Exec(
		"INSERT INTO commercial_accounts (tenant_id, customer_id, version) VALUES (?, ?, ?) ON CONFLICT DO NOTHING",
		a.TenantID, a.CustomerID, a.Version,
	)
	if res.Error != nil {
		return res.Error
	}
	var got Account
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", a.TenantID).First(&got).Error; err != nil {
		// A customer unique-key collision can leave this tenant without a row.
		// Normalize that case to the stable domain error instead of leaking
		// gorm.ErrRecordNotFound.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var owner Account
			if lookupErr := s.db.WithContext(ctx).Where("customer_id = ?", a.CustomerID).First(&owner).Error; lookupErr == nil {
				return ErrAccountMappingConflict
			}
		}
		return err
	}
	if got.CustomerID != a.CustomerID {
		return ErrAccountMappingConflict
	}
	return nil
}

func (s *AccountStore) Get(ctx context.Context, tenant uint64) (Account, error) {
	var a Account
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenant).First(&a).Error
	return a, err
}
