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

type Receipt struct {
	RequestID, RequestHash, Status string
	Response                       []byte
}
type Evidence struct {
	ID, ResourceID, VersionID, Digest string
	Payload                           []byte
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
	Payload  string `gorm:"column:payload"`
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
	return Record{ID: row.FactID, Payload: []byte(row.Payload)}, nil
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
	return s.db.WithContext(ctx).Create(&careerFactRow{TenantID: scope.TenantID, OwnerID: scope.OwnerID, FactID: record.ID, Payload: string(record.Payload)}).Error
}

type careerReceiptRow struct {
	TenantID    uint64 `gorm:"column:tenant_id;primaryKey"`
	OwnerID     string `gorm:"column:owner_id;primaryKey"`
	RequestID   string `gorm:"column:request_id;primaryKey"`
	RequestHash string `gorm:"column:request_hash"`
	Status      string `gorm:"column:status"`
	Response    string `gorm:"column:response_json"`
}

func (careerReceiptRow) TableName() string { return "career_idempotency_receipts" }

func (s *GormStore) PutReceipt(ctx context.Context, scope Scope, receipt Receipt) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("career repository unavailable")
	}
	if receipt.RequestID == "" || receipt.RequestHash == "" || receipt.Status == "" {
		return errors.New("invalid career receipt")
	}
	if err := s.EnsureSpace(ctx, scope); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&careerReceiptRow{TenantID: scope.TenantID, OwnerID: scope.OwnerID, RequestID: receipt.RequestID, RequestHash: receipt.RequestHash, Status: receipt.Status, Response: string(receipt.Response)}).Error
}

func (s *GormStore) GetReceipt(ctx context.Context, scope Scope, requestID string) (Receipt, error) {
	if err := scope.Validate(); err != nil {
		return Receipt{}, err
	}
	if s == nil || s.db == nil {
		return Receipt{}, errors.New("career repository unavailable")
	}
	var row careerReceiptRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND request_id = ?", scope.TenantID, scope.OwnerID, requestID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Receipt{}, ErrNotFound
	}
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{RequestID: row.RequestID, RequestHash: row.RequestHash, Status: row.Status, Response: []byte(row.Response)}, nil
}

type careerEvidenceRow struct {
	TenantID   uint64 `gorm:"column:tenant_id;primaryKey"`
	OwnerID    string `gorm:"column:owner_id;primaryKey"`
	EvidenceID string `gorm:"column:evidence_id;primaryKey"`
	ResourceID string `gorm:"column:resource_id"`
	VersionID  string `gorm:"column:version_id"`
	Digest     string `gorm:"column:digest"`
	Payload    string `gorm:"column:payload"`
}

func (careerEvidenceRow) TableName() string { return "career_evidence" }

func (s *GormStore) AppendEvidence(ctx context.Context, scope Scope, evidence Evidence) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return errors.New("career repository unavailable")
	}
	if evidence.ID == "" || evidence.ResourceID == "" || evidence.VersionID == "" || evidence.Digest == "" {
		return errors.New("invalid career evidence")
	}
	if err := s.EnsureSpace(ctx, scope); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&careerEvidenceRow{TenantID: scope.TenantID, OwnerID: scope.OwnerID, EvidenceID: evidence.ID, ResourceID: evidence.ResourceID, VersionID: evidence.VersionID, Digest: evidence.Digest, Payload: string(evidence.Payload)}).Error
}

func (s *GormStore) GetEvidence(ctx context.Context, scope Scope, id string) (Evidence, error) {
	if err := scope.Validate(); err != nil {
		return Evidence{}, err
	}
	if s == nil || s.db == nil {
		return Evidence{}, errors.New("career repository unavailable")
	}
	var row careerEvidenceRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND evidence_id = ?", scope.TenantID, scope.OwnerID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Evidence{}, ErrNotFound
	}
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{ID: row.EvidenceID, ResourceID: row.ResourceID, VersionID: row.VersionID, Digest: row.Digest, Payload: []byte(row.Payload)}, nil
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
