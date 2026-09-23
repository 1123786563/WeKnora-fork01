package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

var (
	ErrUnauthorized        = errors.New("career scope missing")
	ErrRevisionConflict    = errors.New("career revision conflict")
	ErrReceiptNotFound     = errors.New("career receipt not found")
	ErrIdempotencyConflict = errors.New("request id was already used with different content")
	ErrInvalidRequest      = errors.New("invalid career request")
)

type Scope struct {
	UserID   string
	TenantID uint64
}
type scopeKey struct{}

func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}
func getScope(ctx context.Context) (Scope, error) {
	s, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || strings.TrimSpace(s.UserID) == "" || s.TenantID == 0 {
		return Scope{}, ErrUnauthorized
	}
	return s, nil
}

type profile struct {
	TenantID uint64 `gorm:"primaryKey"`
	UserID   string `gorm:"primaryKey;size:512"`
	Revision uint64 `gorm:"not null"`
}
type fact struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint64 `gorm:"uniqueIndex:career_fact_scope_key"`
	UserID    string `gorm:"uniqueIndex:career_fact_scope_key;size:512"`
	Key       string `gorm:"uniqueIndex:career_fact_scope_key;size:128"`
	Value     string `gorm:"type:text;not null"`
	Revision  uint64
	RequestID string `gorm:"size:128"`
	CreatedAt time.Time
}
type proposal struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint64 `gorm:"index:career_proposal_scope"`
	UserID    string `gorm:"index:career_proposal_scope;size:512"`
	Key       string `gorm:"size:128"`
	Value     string `gorm:"type:text"`
	CreatedAt time.Time
}
type receipt struct {
	TenantID    uint64 `gorm:"uniqueIndex:career_receipt_scope_id"`
	UserID      string `gorm:"uniqueIndex:career_receipt_scope_id;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_receipt_scope_id;size:128"`
	Fingerprint string `gorm:"size:64"`
	Body        string `gorm:"type:text"`
	CreatedAt   time.Time
}
type Fact struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Revision    uint64    `json:"revision"`
	ConfirmedAt time.Time `json:"confirmedAt"`
}
type Proposal struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"createdAt"`
}
type View struct {
	Revision  uint64     `json:"revision"`
	Facts     []Fact     `json:"facts"`
	Proposals []Proposal `json:"proposals"`
}
type Receipt struct {
	RequestID string `json:"requestId"`
	Revision  uint64 `json:"revision"`
	Fact      Fact   `json:"fact"`
}
type RevisionConflictError struct{ CurrentRevision uint64 }

func (e *RevisionConflictError) Error() string        { return ErrRevisionConflict.Error() }
func (e *RevisionConflictError) Is(target error) bool { return target == ErrRevisionConflict }
func (profile) TableName() string                     { return "career_profiles" }
func (fact) TableName() string                        { return "career_facts" }
func (proposal) TableName() string                    { return "career_proposals" }
func (receipt) TableName() string                     { return "career_receipts" }

type Office struct{ db *gorm.DB }

func NewOffice(db *gorm.DB) (*Office, error) {
	if db == nil {
		return nil, errors.New("career database required")
	}
	if e := db.AutoMigrate(&profile{}, &fact{}, &proposal{}, &receipt{}); e != nil {
		return nil, e
	}
	return &Office{db}, nil
}
func (o *Office) Open(ctx context.Context) (View, error) {
	s, e := getScope(ctx)
	if e != nil {
		return View{}, e
	}
	v := View{Facts: []Fact{}, Proposals: []Proposal{}}
	var p profile
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return v, e
	}
	v.Revision = p.Revision
	var fs []fact
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("key").Find(&fs).Error; e != nil {
		return v, e
	}
	for _, f := range fs {
		v.Facts = append(v.Facts, Fact{f.Key, f.Value, f.Revision, f.CreatedAt})
	}
	var ps []proposal
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("id").Find(&ps).Error; e != nil {
		return v, e
	}
	for _, p := range ps {
		v.Proposals = append(v.Proposals, Proposal{p.Key, p.Value, p.CreatedAt})
	}
	return v, nil
}
func (o *Office) Propose(ctx context.Context, k, v, r string, rev uint64) (Receipt, error) {
	return o.act(ctx, k, v, r, rev, false)
}
func (o *Office) Confirm(ctx context.Context, k, v, r string, rev uint64) (Receipt, error) {
	return o.act(ctx, k, v, r, rev, true)
}
func (o *Office) act(ctx context.Context, k, v, r string, rev uint64, confirmed bool) (out Receipt, err error) {
	s, e := getScope(ctx)
	if e != nil {
		return out, e
	}
	k = strings.TrimSpace(k)
	if k == "" || len(k) > 128 || strings.TrimSpace(r) == "" || len(r) > 128 {
		return out, ErrInvalidRequest
	}
	fb, _ := json.Marshal([]any{k, v, rev, confirmed})
	hash := sha256.Sum256(fb)
	fp := hex.EncodeToString(hash[:])
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old receipt
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, r).First(&old).Error
		if e == nil {
			if old.Fingerprint != fp {
				return ErrIdempotencyConflict
			}
			return json.Unmarshal([]byte(old.Body), &out)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		p := profile{TenantID: s.TenantID, UserID: s.UserID}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; e != nil {
			return e
		}
		next := rev + 1
		updated := tx.Model(&profile{}).Where("tenant_id = ? AND user_id = ? AND revision = ?", s.TenantID, s.UserID, rev).Update("revision", next)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			if e = tx.Where("tenant_id = ? AND user_id = ?", s.TenantID, s.UserID).First(&p).Error; e != nil {
				return e
			}
			return &RevisionConflictError{CurrentRevision: p.Revision}
		}
		if confirmed {
			f := fact{TenantID: s.TenantID, UserID: s.UserID, Key: k, Value: v, Revision: next, RequestID: r}
			if e = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "revision", "request_id", "created_at"})}).Create(&f).Error; e != nil {
				return e
			}
			out.Fact = Fact{k, v, next, f.CreatedAt}
		} else if e = tx.Create(&proposal{TenantID: s.TenantID, UserID: s.UserID, Key: k, Value: v}).Error; e != nil {
			return e
		}
		out.RequestID = r
		out.Revision = next
		body, e := json.Marshal(out)
		if e != nil {
			return e
		}
		return tx.Create(&receipt{TenantID: s.TenantID, UserID: s.UserID, RequestID: r, Fingerprint: fp, Body: string(body)}).Error
	})
	return
}
func (o *Office) Receipt(ctx context.Context, r string) (Receipt, error) {
	s, e := getScope(ctx)
	if e != nil {
		return Receipt{}, e
	}
	var row receipt
	e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, r).First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return Receipt{}, ErrReceiptNotFound
	}
	if e != nil {
		return Receipt{}, e
	}
	var out Receipt
	e = json.Unmarshal([]byte(row.Body), &out)
	return out, e
}
