package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrSemanticRevisionConflict = errors.New("semantic_revision_conflict")
	ErrSemanticRevisionOverflow = errors.New("semantic_revision_overflow")
	ErrSemanticEpochOverflow    = errors.New("semantic_epoch_overflow")
	ErrSemanticOutboxLeaseLost  = errors.New("semantic_outbox_lease_lost")
	ErrSemanticMutationInvalid  = errors.New("semantic_mutation_invalid")
)

type SemanticControlRepository struct{ db *gorm.DB }

func NewSemanticControlRepository(db *gorm.DB) *SemanticControlRepository {
	return &SemanticControlRepository{db: db}
}
func semanticUint(v uint64) string { return strconv.FormatUint(v, 10) }
func semanticHash(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *SemanticControlRepository) WithSemanticMutation(ctx context.Context, m types.SemanticMutation, business func(*gorm.DB) error) (uint64, error) {
	if m.TenantID == 0 || m.KBID == "" || m.DocumentID == "" || m.ContentHash == "" || business == nil || (!m.Deleted && len(m.Payload) == 0) {
		return 0, ErrSemanticMutationInvalid
	}
	var result uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant, expected := semanticUint(m.TenantID), semanticUint(m.ExpectedRevision)
		var currentText string
		var deleted bool
		query := "SELECT revision, deleted FROM semantic_document_revisions WHERE tenant_id = ? AND kb_id = ? AND document_id = ?"
		if tx.Dialector.Name() == "postgres" {
			query += " FOR UPDATE"
		}
		err := tx.Raw(query, tenant, m.KBID, m.DocumentID).Row().Scan(&currentText, &deleted)
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, fmt.Errorf("sql: no rows in result set")) {
			_ = err
		}
		if err != nil {
			// database/sql's no-rows error is intentionally detected by text to avoid leaking a second DB abstraction here.
			if err.Error() != "sql: no rows in result set" {
				return err
			}
			if m.ExpectedRevision != 0 {
				return ErrSemanticRevisionConflict
			}
			currentText = "0"
			deleted = false
		}
		current, parseErr := strconv.ParseUint(currentText, 10, 64)
		if parseErr != nil {
			return parseErr
		}
		if current != m.ExpectedRevision || expected != currentText {
			return ErrSemanticRevisionConflict
		}
		if current == math.MaxUint64 {
			return ErrSemanticRevisionOverflow
		}
		next := current + 1
		if err := business(tx); err != nil {
			return err
		}
		if current == 0 {
			if err := tx.Exec("INSERT INTO semantic_document_revisions(tenant_id,kb_id,document_id,revision,content_hash,deleted) VALUES (?,?,?,?,?,?)", tenant, m.KBID, m.DocumentID, semanticUint(next), m.ContentHash, m.Deleted).Error; err != nil {
				return err
			}
		} else {
			res := tx.Exec("UPDATE semantic_document_revisions SET revision=?,content_hash=?,deleted=? WHERE tenant_id=? AND kb_id=? AND document_id=? AND revision=?", semanticUint(next), m.ContentHash, m.Deleted, tenant, m.KBID, m.DocumentID, currentText)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrSemanticRevisionConflict
			}
		}
		scope := types.SemanticScopeKey{TenantID: m.TenantID, KBID: m.KBID}
		if m.Deleted {
			if err := tx.Exec("INSERT INTO semantic_denials(tenant_id,kb_id,document_id,revision) VALUES(?,?,?,?) ON CONFLICT(tenant_id,kb_id,document_id) DO UPDATE SET revision=excluded.revision", tenant, m.KBID, m.DocumentID, semanticUint(next)).Error; err != nil {
				return err
			}
			if _, err := r.BumpSemanticEpoch(tx, scope); err != nil {
				return err
			}
		} else if deleted {
			if err := tx.Exec("DELETE FROM semantic_denials WHERE tenant_id=? AND kb_id=? AND document_id=?", tenant, m.KBID, m.DocumentID).Error; err != nil {
				return err
			}
			if _, err := r.BumpSemanticEpoch(tx, scope); err != nil {
				return err
			}
		}
		payload := append([]byte{}, m.Payload...)
		eventID := uuid.NewString()
		if err := tx.Exec("INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,deleted,payload,payload_hash,retry_at) VALUES(?,?,?,?,?,?,?,?,?,?)", eventID, tenant, m.KBID, m.DocumentID, semanticUint(next), m.ContentHash, m.Deleted, payload, semanticHash(payload), time.Now().UTC()).Error; err != nil {
			return err
		}
		result = next
		return nil
	})
	return result, err
}
func (r *SemanticControlRepository) BumpSemanticEpoch(tx *gorm.DB, scope types.SemanticScopeKey) (uint64, error) {
	if tx == nil || scope.TenantID == 0 || scope.KBID == "" {
		return 0, ErrSemanticMutationInvalid
	}
	tenant := semanticUint(scope.TenantID)
	var text string
	err := tx.Raw("SELECT epoch FROM semantic_access_epochs WHERE tenant_id=? AND kb_id=?", tenant, scope.KBID).Row().Scan(&text)
	if err != nil {
		if err.Error() != "sql: no rows in result set" {
			return 0, err
		}
		if err = tx.Exec("INSERT INTO semantic_access_epochs(tenant_id,kb_id,epoch) VALUES(?,?,?)", tenant, scope.KBID, "1").Error; err != nil {
			return 0, err
		}
		return 1, nil
	}
	current, e := strconv.ParseUint(text, 10, 64)
	if e != nil {
		return 0, e
	}
	if current == math.MaxUint64 {
		return 0, ErrSemanticEpochOverflow
	}
	next := current + 1
	res := tx.Exec("UPDATE semantic_access_epochs SET epoch=? WHERE tenant_id=? AND kb_id=? AND epoch=?", semanticUint(next), tenant, scope.KBID, text)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected != 1 {
		return 0, ErrSemanticRevisionConflict
	}
	return next, nil
}
func (r *SemanticControlRepository) ClaimSemanticOutbox(ctx context.Context, worker string, seconds int) (*types.SemanticOutboxEvent, error) {
	if worker == "" || seconds <= 0 {
		return nil, ErrSemanticMutationInvalid
	}
	var event types.SemanticOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		lock := ""
		if tx.Dialector.Name() == "postgres" {
			lock = " FOR UPDATE SKIP LOCKED"
		}
		row := tx.Raw("SELECT event_id,tenant_id,kb_id,document_id,revision,content_hash,deleted,payload,payload_hash,attempt_count,lease_token,retry_at FROM semantic_outbox WHERE retry_at<=? ORDER BY retry_at,event_id LIMIT 1"+lock, now).Row()
		var tenant, rev string
		var attempt, token uint64
		var deleted bool
		err := row.Scan(&event.EventID, &tenant, &event.Scope.KBID, &event.DocumentID, &rev, &event.ContentHash, &deleted, &event.Payload, &event.PayloadHash, &attempt, &token, &event.RetryAt)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return nil
			}
			return err
		}
		event.Scope.TenantID, _ = strconv.ParseUint(tenant, 10, 64)
		event.Revision, _ = strconv.ParseUint(rev, 10, 64)
		event.Deleted = deleted
		event.AttemptCount = attempt + 1
		event.LeaseToken = token + 1
		event.LeaseExpiresAt = now.Add(time.Duration(seconds) * time.Second)
		res := tx.Exec("UPDATE semantic_outbox SET attempt_count=?,lease_token=?,lease_expires_at=?,retry_at=? WHERE event_id=? AND lease_token=?", event.AttemptCount, event.LeaseToken, event.LeaseExpiresAt, event.LeaseExpiresAt, event.EventID, token)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrSemanticOutboxLeaseLost
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if event.EventID == "" {
		return nil, nil
	}
	return &event, nil
}
func (r *SemanticControlRepository) AckSemanticOutbox(ctx context.Context, eventID string, token uint64) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM semantic_outbox WHERE event_id=? AND lease_token=?", eventID, token)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrSemanticOutboxLeaseLost
	}
	return nil
}
func (r *SemanticControlRepository) FailSemanticOutbox(ctx context.Context, eventID string, token uint64, retry time.Time, code string) error {
	res := r.db.WithContext(ctx).Exec("UPDATE semantic_outbox SET retry_at=?,lease_expires_at=NULL,error_code='' WHERE event_id=? AND lease_token=?", retry, eventID, token)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrSemanticOutboxLeaseLost
	}
	return nil
}
