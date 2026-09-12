package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

var (
	// ErrSemanticRevisionConflict: the document moved on (or the expected
	// revision never existed); the caller must re-read and retry.
	ErrSemanticRevisionConflict = errors.New("semantic revision conflict")
	// ErrSemanticPayloadConflict: the same event identity was presented with
	// a different payload hash (defense in depth; the CAS normally makes
	// this unreachable).
	ErrSemanticPayloadConflict = errors.New("semantic payload conflict")
)

// SemanticMutation is re-exported so callers of this repository use the
// plan's name directly (alias of types.SemanticMutation).
type SemanticMutation = types.SemanticMutation

// SemanticControlRepository owns the business-side semantic control tables.
// The semantic Python service never writes these.
type SemanticControlRepository struct {
	db *gorm.DB
}

func NewSemanticControlRepository(db *gorm.DB) *SemanticControlRepository {
	return &SemanticControlRepository{db: db}
}

// WithSemanticMutation runs fn inside ONE transaction that also performs the
// document revision CAS, appends the outbox event and (for deletions) the
// denial barrier plus KB epoch bump. If fn fails, NOTHING commits - no
// revision, no event, no barrier.
func (r *SemanticControlRepository) WithSemanticMutation(
	ctx context.Context, mutation types.SemanticMutation, fn func(tx *gorm.DB) error,
) (uint64, error) {
	var newRevision uint64
	txErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		revision, err := advanceRevision(tx, mutation)
		if err != nil {
			return err
		}
		newRevision = revision

		if err := fn(tx); err != nil {
			return err
		}

		if err := appendOutboxEvent(tx, mutation, newRevision); err != nil {
			return err
		}
		if mutation.Deleted {
			if err := insertDenial(tx, mutation, newRevision); err != nil {
				return err
			}
			if err := bumpEpoch(tx, mutation.TenantID, mutation.KBID); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return 0, txErr
	}
	return newRevision, nil
}

// BumpSemanticEpoch raises the KB authorization epoch in its own
// transaction (standalone permission events).
func (r *SemanticControlRepository) BumpSemanticEpoch(tenantID uint64, kbID string) error {
	return bumpEpoch(r.db, tenantID, kbID)
}

// BumpSemanticEpochTx raises the KB authorization epoch INSIDE the caller's
// transaction - the entry point A01 wires to permission changes so the bump
// commits atomically with the ACL write and rolls back with it (a
// post-commit bump would leave a stale-epoch crash window).
func (r *SemanticControlRepository) BumpSemanticEpochTx(tx *gorm.DB, tenantID uint64, kbID string) error {
	return bumpEpoch(tx, tenantID, kbID)
}

func advanceRevision(tx *gorm.DB, mutation types.SemanticMutation) (uint64, error) {
	if mutation.ExpectedRevision == 0 {
		// First mutation: INSERT guarded by the primary key.
		result := tx.Exec(
			"INSERT INTO semantic_document_revisions (tenant_id, kb_id, document_id, revision, deleted) VALUES (?, ?, ?, 1, ?)",
			mutation.TenantID, mutation.KBID, mutation.DocumentID, mutation.Deleted,
		)
		if result.Error != nil {
			if isUniqueViolation(result.Error) {
				return 0, fmt.Errorf("%w: document %s already has a revision", ErrSemanticRevisionConflict, mutation.DocumentID)
			}
			return 0, result.Error
		}
		if result.RowsAffected != 1 {
			return 0, fmt.Errorf("unexpected insert rowcount %d", result.RowsAffected)
		}
		return 1, nil
	}
	result := tx.Exec(
		"UPDATE semantic_document_revisions SET revision = revision + 1, deleted = ?, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND revision = ?",
		mutation.Deleted, mutation.TenantID, mutation.KBID, mutation.DocumentID, mutation.ExpectedRevision,
	)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, fmt.Errorf(
			"%w: expected revision %d for document %s (tenant %d kb %s)",
			ErrSemanticRevisionConflict, mutation.ExpectedRevision, mutation.DocumentID, mutation.TenantID, mutation.KBID,
		)
	}
	return mutation.ExpectedRevision + 1, nil
}

func appendOutboxEvent(tx *gorm.DB, mutation types.SemanticMutation, revision uint64) error {
	eventID := fmt.Sprintf("sem-ev:%d:%s:%s:%d", mutation.TenantID, mutation.KBID, mutation.DocumentID, revision)
	// payload_hash is computed over the RAW bytes BEFORE any column-type
	// normalization (PG JSONB reorders keys); receivers must only compare
	// it against stored hashes, never re-hash the transported text.
	digest := sha256.Sum256(mutation.Payload)
	payloadHash := hex.EncodeToString(digest[:])

	// Same identity with a different payload is a conflict; the same payload
	// is a no-op (idempotent replay).
	var existingHash string
	err := tx.Raw(
		"SELECT payload_hash FROM semantic_outbox WHERE event_id = ?", eventID,
	).Scan(&existingHash).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existingHash != "" {
		if existingHash != payloadHash {
			return fmt.Errorf("%w: event %s payload changed", ErrSemanticPayloadConflict, eventID)
		}
		return nil
	}
	return tx.Exec(
		"INSERT INTO semantic_outbox (tenant_id, kb_id, document_id, revision, event_id, payload_hash, payload) VALUES (?, ?, ?, ?, ?, ?, ?)",
		mutation.TenantID, mutation.KBID, mutation.DocumentID, revision, eventID, payloadHash, string(mutation.Payload),
	).Error
}

func insertDenial(tx *gorm.DB, mutation types.SemanticMutation, revision uint64) error {
	return tx.Exec(
		"INSERT INTO semantic_denials (tenant_id, kb_id, document_id, revision) VALUES (?, ?, ?, ?) ON CONFLICT DO NOTHING",
		mutation.TenantID, mutation.KBID, mutation.DocumentID, revision,
	).Error
}

func bumpEpoch(tx *gorm.DB, tenantID uint64, kbID string) error {
	// Single-statement upsert: concurrent FIRST bumps both succeed instead
	// of racing UPDATE-then-INSERT (portable across PG and SQLite).
	return tx.Exec(
		"INSERT INTO semantic_access_epochs (tenant_id, kb_id, epoch) VALUES (?, ?, 1) ON CONFLICT (tenant_id, kb_id) DO UPDATE SET epoch = semantic_access_epochs.epoch + 1, updated_at = CURRENT_TIMESTAMP",
		tenantID, kbID,
	).Error
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// Match the unique-violation shape only (NOT NULL/FK/CHECK constraint
	// failures must NOT masquerade as revision conflicts). PG 23505 says
	// "duplicate key value violates unique constraint"; SQLite says
	// "UNIQUE constraint failed: <index>".
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate key")
}

// SemanticDocumentState is the visibility-relevant projection of one
// document row for scope snapshots.
type SemanticDocumentState struct {
	DocumentID string
	Revision   uint64
	Deleted    bool
}

// CurrentEpoch returns the KB's authorization epoch (0 when never bumped).
// A read error must be treated fail-closed by callers.
func (r *SemanticControlRepository) CurrentEpoch(ctx context.Context, tenantID uint64, kbID string) (uint64, error) {
	var epoch int64
	err := r.db.WithContext(ctx).Raw(
		"SELECT epoch FROM semantic_access_epochs WHERE tenant_id = ? AND kb_id = ?",
		tenantID, kbID,
	).Scan(&epoch).Error
	if err != nil {
		return 0, err
	}
	return uint64(epoch), nil
}

// ListDocumentStates returns every document revision row of the KB.
func (r *SemanticControlRepository) ListDocumentStates(ctx context.Context, tenantID uint64, kbID string) ([]SemanticDocumentState, error) {
	var rows []struct {
		DocumentID string
		Revision   int64
		Deleted    bool
	}
	if err := r.db.WithContext(ctx).Raw(
		"SELECT document_id, revision, deleted FROM semantic_document_revisions WHERE tenant_id = ? AND kb_id = ?",
		tenantID, kbID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	states := make([]SemanticDocumentState, 0, len(rows))
	for _, row := range rows {
		states = append(states, SemanticDocumentState{
			DocumentID: row.DocumentID, Revision: uint64(row.Revision), Deleted: row.Deleted,
		})
	}
	return states, nil
}

// ListDenials returns the deletion barriers of the KB as document -> highest
// denied revision.
func (r *SemanticControlRepository) ListDenials(ctx context.Context, tenantID uint64, kbID string) (map[string]uint64, error) {
	var rows []struct {
		DocumentID string
		Revision   int64
	}
	if err := r.db.WithContext(ctx).Raw(
		"SELECT document_id, MAX(revision) AS revision FROM semantic_denials WHERE tenant_id = ? AND kb_id = ? GROUP BY document_id",
		tenantID, kbID,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	denials := make(map[string]uint64, len(rows))
	for _, row := range rows {
		denials[row.DocumentID] = uint64(row.Revision)
	}
	return denials, nil
}

// BumpTenantSemanticEpochsTx raises the authorization epoch of EVERY KB the
// tenant owns, inside the caller's transaction. Member/role changes affect
// all of a tenant's KBs at once; the same-transaction bump guarantees no
// scope survives an ACL write (A01 wiring point).
func BumpTenantSemanticEpochsTx(tx *gorm.DB, tenantID uint64) error {
	var kbIDs []string
	if err := tx.Raw(
		"SELECT id FROM knowledge_bases WHERE tenant_id = ?", tenantID,
	).Scan(&kbIDs).Error; err != nil {
		return err
	}
	for _, kbID := range kbIDs {
		if err := bumpEpoch(tx, tenantID, kbID); err != nil {
			return err
		}
	}
	return nil
}

// BumpKBSemanticEpochs raises the named KBs' epochs in its own short
// transaction - the service-layer entry for transfer flows (clone/move
// source+target) where the business writes are resumable multi-document
// operations without one enclosing transaction.
// BackendStateView is the persisted backend state machine snapshot.
type BackendStateView struct {
	DesiredBackend     string
	ActiveBackend      string
	ActiveGeneration   string
	NativeCheckpoint   int64
	SemanticCheckpoint int64
}

// SetDesiredBackend records the desired backend WITHOUT touching active
// (W03: shadow-build intent only; deployment images never auto-switch).
func (r *SemanticControlRepository) SetDesiredBackend(ctx context.Context, scope types.SemanticScopeKey, backend string) error {
	return r.db.WithContext(ctx).Exec(
		"INSERT INTO semantic_backend_states (tenant_id, kb_id, desired_backend, active_backend) VALUES (?, ?, ?, 'native')"+
			" ON CONFLICT (tenant_id, kb_id) DO UPDATE SET desired_backend = excluded.desired_backend, updated_at = CURRENT_TIMESTAMP",
		scope.TenantID, scope.KBID, backend,
	).Error
}

// BackendState loads the persisted state machine row for a scope.
func (r *SemanticControlRepository) BackendState(ctx context.Context, scope types.SemanticScopeKey) (BackendStateView, error) {
	var view BackendStateView
	err := r.db.WithContext(ctx).Raw(
		"SELECT desired_backend, active_backend, active_generation, native_checkpoint, semantic_checkpoint"+
			" FROM semantic_backend_states WHERE tenant_id = ? AND kb_id = ?",
		scope.TenantID, scope.KBID,
	).Scan(&view).Error
	if err != nil {
		return BackendStateView{}, err
	}
	if view.ActiveBackend == "" {
		view.DesiredBackend = "native"
		view.ActiveBackend = "native"
	}
	return view, nil
}

// CompareAndSwapBackend atomically switches the active backend iff the
// current row matches expectedBackend AND expectedGeneration. Returns
// false when the precondition fails (never a blind switch).
func (r *SemanticControlRepository) CompareAndSwapBackend(ctx context.Context, scope types.SemanticScopeKey,
	expectedBackend, newBackend, expectedGeneration string) (bool, error) {
	result := r.db.WithContext(ctx).Exec(
		"UPDATE semantic_backend_states SET active_backend = ?, updated_at = CURRENT_TIMESTAMP"+
			" WHERE tenant_id = ? AND kb_id = ? AND active_backend = ? AND active_generation = ?",
		newBackend, scope.TenantID, scope.KBID, expectedBackend, expectedGeneration,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// LatestDocumentRevision is the newest semantic document revision in the
// scope (the source-of-truth the native index must catch up to).
func (r *SemanticControlRepository) LatestDocumentRevision(ctx context.Context, scope types.SemanticScopeKey) (int64, error) {
	var latest int64
	err := r.db.WithContext(ctx).Raw(
		"SELECT COALESCE(MAX(revision), 0) FROM semantic_document_revisions WHERE tenant_id = ? AND kb_id = ?",
		scope.TenantID, scope.KBID,
	).Scan(&latest).Error
	return latest, err
}

func (r *SemanticControlRepository) BumpKBSemanticEpochs(ctx context.Context, tenantID uint64, kbIDs ...string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return BumpKBSemanticEpochsTx(tx, tenantID, kbIDs...)
	})
}

// BumpOrgSharedKBSemanticEpochsTx raises epochs for every KB shared to the
// organization (org membership changes re-scope all of them at once),
// inside the caller's transaction.
func BumpOrgSharedKBSemanticEpochsTx(tx *gorm.DB, orgID string) error {
	var rows []struct {
		SourceTenantID  uint64
		KnowledgeBaseID string
	}
	if err := tx.Raw(
		"SELECT source_tenant_id, knowledge_base_id FROM kb_shares WHERE organization_id = ? AND deleted_at IS NULL",
		orgID,
	).Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := bumpEpoch(tx, row.SourceTenantID, row.KnowledgeBaseID); err != nil {
			return err
		}
	}
	return nil
}

// BumpKBSemanticEpochsTx raises the epoch of the named KBs (source AND
// target of transfers, shared KBs) inside the caller's transaction.
func BumpKBSemanticEpochsTx(tx *gorm.DB, tenantID uint64, kbIDs ...string) error {
	for _, kbID := range kbIDs {
		if kbID == "" {
			continue
		}
		if err := bumpEpoch(tx, tenantID, kbID); err != nil {
			return err
		}
	}
	return nil
}

// ClaimOutboxEvents returns up to limit dispatchable events (unconfirmed,
// past their next attempt, not held by another claim lease). Claiming sets
// claimed_until = now + lease so a crashed dispatcher's events become
// claimable again after the lease expires. Timestamps are computed in Go
// and passed as parameters - no dialect-specific date arithmetic.
func (r *SemanticControlRepository) ClaimOutboxEvents(ctx context.Context, limit int, claimLease time.Duration) ([]types.SemanticOutboxEvent, error) {
	now := time.Now().UTC()
	var rows []struct {
		EventID     string
		TenantID    uint64
		KBID        string
		DocumentID  string
		Revision    uint64
		PayloadHash string
		Payload     []byte
		Attempts    int
	}
	if err := r.db.WithContext(ctx).Raw(
		"SELECT event_id, tenant_id, kb_id, document_id, revision, payload_hash, payload, attempts FROM semantic_outbox WHERE confirmed_at IS NULL AND next_attempt_at <= ? AND (claimed_until IS NULL OR claimed_until <= ?) ORDER BY created_at LIMIT ?",
		now, now, limit,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	claimUntil := time.Now().UTC().Add(claimLease)
	events := make([]types.SemanticOutboxEvent, 0, len(rows))
	for _, row := range rows {
		// Atomic claim: the conditional UPDATE re-checks the claim
		// predicate, so two concurrent dispatchers cannot both own an
		// event; RowsAffected==0 means another dispatcher won it. Reclaims
		// (expired lease from a crashed dispatcher) count as an attempt so
		// poison events cannot crash-loop forever with attempts=0.
		result := r.db.WithContext(ctx).Exec(
			"UPDATE semantic_outbox SET claimed_until = ?, attempts = attempts + 1 WHERE event_id = ? AND confirmed_at IS NULL AND next_attempt_at <= ? AND (claimed_until IS NULL OR claimed_until <= ?)",
			claimUntil, row.EventID, now, now,
		)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			continue
		}
		events = append(events, types.SemanticOutboxEvent{
			EventID:     row.EventID,
			TenantID:    row.TenantID,
			KBID:        row.KBID,
			DocumentID:  row.DocumentID,
			Revision:    row.Revision,
			PayloadHash: row.PayloadHash,
			Payload:     row.Payload,
			Attempts:    row.Attempts + 1,
		})
	}
	return events, nil
}

// ConfirmOutboxEvent marks durable acceptance by the receiver; confirmed
// events never dispatch again.
func (r *SemanticControlRepository) ConfirmOutboxEvent(ctx context.Context, eventID string) error {
	return r.db.WithContext(ctx).Exec(
		"UPDATE semantic_outbox SET confirmed_at = CURRENT_TIMESTAMP, claimed_until = NULL WHERE event_id = ? AND confirmed_at IS NULL",
		eventID,
	).Error
}

// FailOutboxEvent records a failed dispatch attempt with a caller-chosen
// backoff before the next claim. The next-attempt timestamp is computed in
// Go (portable across PG and SQLite).
func (r *SemanticControlRepository) FailOutboxEvent(ctx context.Context, eventID string, backoff time.Duration) error {
	nextAttempt := time.Now().UTC().Add(backoff)
	return r.db.WithContext(ctx).Exec(
		"UPDATE semantic_outbox SET attempts = attempts + 1, next_attempt_at = ?, claimed_until = NULL WHERE event_id = ? AND confirmed_at IS NULL",
		nextAttempt, eventID,
	).Error
}
