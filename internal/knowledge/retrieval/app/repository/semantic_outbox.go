package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
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
	ErrSemanticOutboxOverflow   = errors.New("semantic_outbox_overflow")
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
	if m.TenantID == 0 || m.KBID == "" || m.DocumentID == "" || m.ContentHash == "" || m.ConfigDigest == "" || business == nil || (!m.Deleted && len(m.Payload) == 0) {
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
		if err := tx.Exec("INSERT INTO semantic_outbox(event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,retry_at) VALUES(?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)", eventID, tenant, m.KBID, m.DocumentID, semanticUint(next), m.ContentHash, m.ConfigDigest, m.Deleted, payload, semanticHash(payload)).Error; err != nil {
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
	if err := tx.Exec("INSERT INTO semantic_access_epochs(tenant_id,kb_id,epoch) VALUES(?,?,?) ON CONFLICT(tenant_id,kb_id) DO NOTHING", tenant, scope.KBID, "0").Error; err != nil {
		return 0, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		var text string
		if err := tx.Raw("SELECT epoch FROM semantic_access_epochs WHERE tenant_id=? AND kb_id=?", tenant, scope.KBID).Row().Scan(&text); err != nil {
			return 0, err
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
		if res.RowsAffected == 1 {
			return next, nil
		}
	}
	return 0, ErrSemanticRevisionConflict
}
func (r *SemanticControlRepository) ClaimSemanticOutbox(ctx context.Context, worker string, seconds int) (*types.SemanticOutboxEvent, error) {
	if worker == "" || seconds <= 0 {
		return nil, ErrSemanticMutationInvalid
	}
	if r.db.Dialector.Name() == "sqlite" {
		var claimed *types.SemanticOutboxEvent
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			candidate, err := r.selectSQLiteOutboxCandidateFrom(tx)
			if err != nil || candidate == nil {
				return err
			}
			claimed, err = claimSQLiteOutboxCandidateFrom(tx, candidate, worker, seconds)
			return err
		})
		if err != nil {
			return nil, err
		}
		return claimed, nil
	}
	var event types.SemanticOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := tx.Raw("SELECT event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,lease_token,retry_at,error_code FROM semantic_outbox WHERE retry_at<=CURRENT_TIMESTAMP AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP) ORDER BY retry_at,event_id LIMIT 1 FOR UPDATE SKIP LOCKED").Row()
		var tenant, rev string
		var attemptText, tokenText string
		var deleted bool
		err := row.Scan(&event.EventID, &tenant, &event.Scope.KBID, &event.DocumentID, &rev, &event.ContentHash, &event.ConfigDigest, &deleted, &event.Payload, &event.PayloadHash, &attemptText, &tokenText, &event.RetryAt, &event.ErrorCode)
		if err != nil {
			if err.Error() == "sql: no rows in result set" {
				return nil
			}
			return err
		}
		var parseErr error
		if event.Scope.TenantID, parseErr = strconv.ParseUint(tenant, 10, 64); parseErr != nil {
			return parseErr
		}
		if event.Revision, parseErr = strconv.ParseUint(rev, 10, 64); parseErr != nil {
			return parseErr
		}
		attempt, parseErr := strconv.ParseUint(attemptText, 10, 64)
		if parseErr != nil {
			return parseErr
		}
		token, parseErr := strconv.ParseUint(tokenText, 10, 64)
		if parseErr != nil {
			return parseErr
		}
		if attempt == math.MaxUint64 || token == math.MaxUint64 {
			return ErrSemanticOutboxOverflow
		}
		event.Deleted = deleted
		event.AttemptCount = attempt + 1
		event.LeaseToken = token + 1
		event.LeaseOwner = worker
		res := tx.Exec("UPDATE semantic_outbox SET attempt_count=?,lease_token=?,lease_owner=?,lease_expires_at=CURRENT_TIMESTAMP + (? * INTERVAL '1 second'),retry_at=CURRENT_TIMESTAMP + (? * INTERVAL '1 second') WHERE event_id=? AND lease_token=? AND retry_at<=CURRENT_TIMESTAMP AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP)", semanticUint(event.AttemptCount), semanticUint(event.LeaseToken), worker, seconds, seconds, event.EventID, semanticUint(token))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			event.EventID = ""
			return nil
		}
		return tx.Raw("SELECT lease_expires_at,retry_at FROM semantic_outbox WHERE event_id=? AND lease_token=?", event.EventID, semanticUint(event.LeaseToken)).Row().Scan(&event.LeaseExpiresAt, &event.RetryAt)
	})
	if err != nil {
		return nil, err
	}
	if event.EventID == "" {
		return nil, nil
	}
	return &event, nil
}

type semanticOutboxCandidate struct {
	event       types.SemanticOutboxEvent
	oldToken    string
	oldAttempts string
}

func (r *SemanticControlRepository) selectSQLiteOutboxCandidate(ctx context.Context) (*semanticOutboxCandidate, error) {
	return r.selectSQLiteOutboxCandidateFrom(r.db.WithContext(ctx))
}

func (r *SemanticControlRepository) selectSQLiteOutboxCandidateFrom(db *gorm.DB) (*semanticOutboxCandidate, error) {
	row := db.Raw("SELECT event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,lease_token,retry_at,error_code FROM semantic_outbox WHERE retry_at<=CURRENT_TIMESTAMP AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP) ORDER BY retry_at,event_id LIMIT 1").Row()
	candidate := &semanticOutboxCandidate{}
	var tenant, revision string
	var deleted bool
	if err := row.Scan(&candidate.event.EventID, &tenant, &candidate.event.Scope.KBID, &candidate.event.DocumentID, &revision, &candidate.event.ContentHash, &candidate.event.ConfigDigest, &deleted, &candidate.event.Payload, &candidate.event.PayloadHash, &candidate.oldAttempts, &candidate.oldToken, &candidate.event.RetryAt, &candidate.event.ErrorCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var err error
	if candidate.event.Scope.TenantID, err = strconv.ParseUint(tenant, 10, 64); err != nil {
		return nil, err
	}
	if candidate.event.Revision, err = strconv.ParseUint(revision, 10, 64); err != nil {
		return nil, err
	}
	attempt, err := strconv.ParseUint(candidate.oldAttempts, 10, 64)
	if err != nil {
		return nil, err
	}
	token, err := strconv.ParseUint(candidate.oldToken, 10, 64)
	if err != nil {
		return nil, err
	}
	if attempt == math.MaxUint64 || token == math.MaxUint64 {
		return nil, ErrSemanticOutboxOverflow
	}
	candidate.event.Deleted = deleted
	candidate.event.AttemptCount = attempt + 1
	candidate.event.LeaseToken = token + 1
	return candidate, nil
}

func (r *SemanticControlRepository) claimSQLiteOutboxCandidate(ctx context.Context, candidate *semanticOutboxCandidate, worker string, seconds int) (*types.SemanticOutboxEvent, error) {
	var claimed *types.SemanticOutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		claimed, err = claimSQLiteOutboxCandidateFrom(tx, candidate, worker, seconds)
		return err
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func claimSQLiteOutboxCandidateFrom(tx *gorm.DB, candidate *semanticOutboxCandidate, worker string, seconds int) (*types.SemanticOutboxEvent, error) {
	if candidate == nil || worker == "" || seconds <= 0 {
		return nil, ErrSemanticMutationInvalid
	}
	expiry := "datetime(CURRENT_TIMESTAMP, '+' || ? || ' seconds')"
	query := "UPDATE semantic_outbox SET attempt_count=?,lease_token=?,lease_owner=?,lease_expires_at=" + expiry + ",retry_at=" + expiry + " WHERE event_id=? AND lease_token=? AND retry_at<=CURRENT_TIMESTAMP AND (lease_expires_at IS NULL OR lease_expires_at<=CURRENT_TIMESTAMP)"
	res := tx.Exec(query, semanticUint(candidate.event.AttemptCount), semanticUint(candidate.event.LeaseToken), worker, seconds, seconds, candidate.event.EventID, candidate.oldToken)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, nil
	}
	event := &types.SemanticOutboxEvent{}
	var tenant, revision, attempts, token string
	var deleted bool
	row := tx.Raw("SELECT event_id,tenant_id,kb_id,document_id,revision,content_hash,config_digest,deleted,payload,payload_hash,attempt_count,lease_token,lease_owner,lease_expires_at,retry_at,error_code FROM semantic_outbox WHERE event_id=? AND lease_token=?", candidate.event.EventID, semanticUint(candidate.event.LeaseToken)).Row()
	if err := row.Scan(&event.EventID, &tenant, &event.Scope.KBID, &event.DocumentID, &revision, &event.ContentHash, &event.ConfigDigest, &deleted, &event.Payload, &event.PayloadHash, &attempts, &token, &event.LeaseOwner, &event.LeaseExpiresAt, &event.RetryAt, &event.ErrorCode); err != nil {
		return nil, err
	}
	var err error
	if event.Scope.TenantID, err = strconv.ParseUint(tenant, 10, 64); err != nil {
		return nil, err
	}
	if event.Revision, err = strconv.ParseUint(revision, 10, 64); err != nil {
		return nil, err
	}
	if event.AttemptCount, err = strconv.ParseUint(attempts, 10, 64); err != nil {
		return nil, err
	}
	if event.LeaseToken, err = strconv.ParseUint(token, 10, 64); err != nil {
		return nil, err
	}
	event.Deleted = deleted
	return event, nil
}
func (r *SemanticControlRepository) AckSemanticOutbox(ctx context.Context, eventID, worker string, token uint64) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM semantic_outbox WHERE event_id=? AND lease_owner=? AND lease_token=?", eventID, worker, semanticUint(token))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrSemanticOutboxLeaseLost
	}
	return nil
}
func (r *SemanticControlRepository) FailSemanticOutbox(ctx context.Context, eventID, worker string, token uint64, retry time.Time, code string) error {
	if eventID == "" || worker == "" || code == "" {
		return ErrSemanticMutationInvalid
	}
	res := r.db.WithContext(ctx).Exec("UPDATE semantic_outbox SET retry_at=?,lease_expires_at=NULL,lease_owner='',error_code=? WHERE event_id=? AND lease_owner=? AND lease_token=?", retry, code, eventID, worker, semanticUint(token))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrSemanticOutboxLeaseLost
	}
	return nil
}
