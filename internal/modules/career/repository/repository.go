package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUnauthorized = errors.New("career scope unauthorized")
var ErrNotFound = errors.New("career resource not found")

// Scope is constructed from authenticated server context, never client JSON.
type Scope struct {
	TenantID uint64
	OwnerID  string
}

func (s Scope) Validate() error {
	if s.TenantID == 0 || strings.TrimSpace(s.OwnerID) == "" || strings.TrimSpace(s.OwnerID) != s.OwnerID {
		return ErrUnauthorized
	}
	return nil
}

type Record struct {
	ID      string
	Payload []byte
}

// Store is the persistence seam; all methods explicitly carry authenticated
// scope to make accidental tenant/owner widening difficult.
type Store interface {
	Get(context.Context, Scope, string) (Record, error)
	Put(context.Context, Scope, Record) error
}

// GormStore is the durable repository implementation over the Career schema.
type GormStore struct{ db *gorm.DB }

func NewGormStore(db *gorm.DB) *GormStore { return &GormStore{db: db} }

type careerFactRow struct {
	TenantID uint64 `gorm:"column:tenant_id;primaryKey"`
	OwnerID  string `gorm:"column:owner_id;primaryKey"`
	FactID   string `gorm:"column:fact_id;primaryKey"`
	Payload  []byte `gorm:"column:payload"`
}

func (careerFactRow) TableName() string { return "career_profile_facts" }

func (s *GormStore) EnsureSpace(ctx context.Context, scope Scope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("career repository unavailable")
	}
	return s.db.WithContext(ctx).Table("career_spaces").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"tenant_id": scope.TenantID, "owner_id": scope.OwnerID}).Error
}

func (s *GormStore) Get(ctx context.Context, scope Scope, id string) (Record, error) {
	if err := scope.Validate(); err != nil {
		return Record{}, err
	}
	if s == nil || s.db == nil {
		return Record{}, errors.New("career repository unavailable")
	}
	var row careerFactRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND fact_id = ?", scope.TenantID, scope.OwnerID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	return Record{ID: row.FactID, Payload: append([]byte(nil), row.Payload...)}, nil
}

func (s *GormStore) Put(ctx context.Context, scope Scope, record Record) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("career repository unavailable")
	}
	if strings.TrimSpace(record.ID) == "" {
		return ErrNotFound
	}
	if err := s.EnsureSpace(ctx, scope); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&careerFactRow{TenantID: scope.TenantID, OwnerID: scope.OwnerID, FactID: record.ID, Payload: record.Payload}).Error
}

type MemoryStore struct{ rows map[Scope]map[string]Record }

func NewMemoryStore() *MemoryStore { return &MemoryStore{rows: make(map[Scope]map[string]Record)} }

func (s *MemoryStore) Get(_ context.Context, scope Scope, id string) (Record, error) {
	if err := scope.Validate(); err != nil {
		return Record{}, err
	}
	rows := s.rows[scope]
	row, ok := rows[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return Record{ID: row.ID, Payload: append([]byte(nil), row.Payload...)}, nil
}

func (s *MemoryStore) Put(_ context.Context, scope Scope, record Record) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(record.ID) == "" {
		return ErrNotFound
	}
	if s.rows[scope] == nil {
		s.rows[scope] = make(map[string]Record)
	}
	s.rows[scope][record.ID] = Record{ID: record.ID, Payload: append([]byte(nil), record.Payload...)}
	return nil
}
