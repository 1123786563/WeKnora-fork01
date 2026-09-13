package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"
)

// semanticOperationID derives a stable idempotency key for one document
// attempt: retries after a lost response reuse the SAME operation row.
// Long document ids are hashed to fit the VARCHAR(128) columns.
func semanticOperationID(documentID string, attempt uint64) string {
	key := fmt.Sprintf("sem-%s-a%d", documentID, attempt)
	if len(key) > 128 {
		sum := sha256.Sum256([]byte(key))
		key = "sem-" + hex.EncodeToString(sum[:16])
	}
	return key
}

// SemanticTaskCoordinator maps business document tasks to semantic
// operations with ATTEMPT isolation (I05). Go only submits and
// coordinates state; the semantic pipeline internals stay in the service.
// Completion receipts are unique per (scope, document, attempt, operation):
// a late terminal from an OLD attempt can never drain the current attempt
// counter, and duplicate terminals are idempotent.
type SemanticTaskCoordinator struct {
	db *gorm.DB
}

func NewSemanticTaskCoordinator(db *gorm.DB) *SemanticTaskCoordinator {
	return &SemanticTaskCoordinator{db: db}
}

// Submit registers one pending semantic operation for (document, attempt).
// Idempotent per idempotency key: a retry after a lost response reuses the
// SAME operation row and never double-counts pending.
func (c *SemanticTaskCoordinator) Submit(ctx context.Context, tenantID uint64, kbID, documentID string, attempt uint64, idempotencyKey string) error {
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Atomic idempotency: the unique idempotency_key index arbitrates
		// concurrent submits - only the actual inserter counts pending.
		inserted := tx.Exec(
			"INSERT INTO semantic_task_operations (tenant_id, kb_id, document_id, attempt, operation_id, idempotency_key, state) VALUES (?, ?, ?, ?, ?, ?, ?)"+
				" ON CONFLICT (idempotency_key) DO NOTHING",
			tenantID, kbID, documentID, attempt, idempotencyKey, idempotencyKey, "pending",
		)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			return tx.Exec(
				"INSERT INTO semantic_attempt_counters (tenant_id, kb_id, document_id, attempt, pending) VALUES (?, ?, ?, ?, 1)"+
					" ON CONFLICT (tenant_id, kb_id, document_id, attempt) DO UPDATE SET pending = pending + 1, updated_at = CURRENT_TIMESTAMP",
				tenantID, kbID, documentID, attempt,
			).Error
		}
		return nil
	})
}

// Complete records a terminal state for (operation, attempt). In ONE
// transaction the receipt insert (unique per scope/doc/attempt/operation)
// and the pending decrement happen atomically ONLY when this insert is the
// first for that operation: an old attempt late terminal touches nothing,
// a duplicate terminal decrements nothing, and cancelled/superseded
// terminals never drain the counter.
func (c *SemanticTaskCoordinator) Complete(ctx context.Context, tenantID uint64, kbID, documentID string, attempt uint64, operationID, state string) error {
	switch state {
	case "succeeded", "failed":
	case "cancelled", "superseded":
	default:
		return fmt.Errorf("semantic task: unknown terminal state %q", state)
	}
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if state == "cancelled" || state == "superseded" {
			return tx.Exec(
				"UPDATE semantic_task_operations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND operation_id = ?",
				state, tenantID, kbID, documentID, attempt, operationID,
			).Error
		}
		// Attempt-namespaced receipt: dedicated (attempt, operation_id)
		// columns, never overloading the I02 revision/task_key namespace.
		receipt := tx.Exec(
			"INSERT INTO semantic_completion_receipts (tenant_id, kb_id, document_id, revision, task_key, attempt, operation_id) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (tenant_id, kb_id, document_id, revision, task_key) DO NOTHING",
			tenantID, kbID, documentID, 0, "attempt:"+operationID, attempt, operationID,
		)
		if receipt.Error != nil {
			return receipt.Error
		}
		if err := tx.Exec(
			"UPDATE semantic_task_operations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND operation_id = ?",
			state, tenantID, kbID, documentID, attempt, operationID,
		).Error; err != nil {
			return err
		}
		if receipt.RowsAffected == 1 {
			return tx.Exec(
				"UPDATE semantic_attempt_counters SET pending = pending - 1, updated_at = CURRENT_TIMESTAMP"+
					" WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND pending > 0",
				tenantID, kbID, documentID, attempt,
			).Error
		}
		return nil
	})
}

// Cancel marks the operation cancelled WITHOUT touching any counter: the
// work did not complete; a superseding attempt owns the new counter.
func (c *SemanticTaskCoordinator) Cancel(ctx context.Context, tenantID uint64, kbID, documentID string, attempt uint64, operationID string) error {
	return c.db.WithContext(ctx).Exec(
		"UPDATE semantic_task_operations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND operation_id = ?",
		"cancelled", tenantID, kbID, documentID, attempt, operationID,
	).Error
}

// Reconcile repairs mapping state after a lost response by re-applying the
// recorded receipt outcome to the operation row (idempotent).
func (c *SemanticTaskCoordinator) Reconcile(ctx context.Context, tenantID uint64, kbID, documentID string, attempt uint64) error {
	var operationID string
	if err := c.db.WithContext(ctx).Raw(
		"SELECT operation_id FROM semantic_completion_receipts WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND operation_id IS NOT NULL ORDER BY completed_at LIMIT 1",
		tenantID, kbID, documentID, attempt,
	).Scan(&operationID).Error; err != nil {
		return err
	}
	if operationID != "" {
		return c.db.WithContext(ctx).Exec(
			"UPDATE semantic_task_operations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND kb_id = ? AND document_id = ? AND attempt = ? AND operation_id = ?",
			"succeeded", tenantID, kbID, documentID, attempt, operationID,
		).Error
	}
	return nil
}
