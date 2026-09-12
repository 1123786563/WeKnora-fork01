package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

var (
	ErrSemanticInvocationNeedsReconciliation = errors.New("semantic invocation needs reconciliation")
	ErrSemanticBudgetExhausted               = errors.New("semantic budget exhausted")
	ErrSemanticInvocationConflict            = errors.New("semantic invocation conflict")
)

type SemanticModelInvocation struct {
	InvocationID    string
	OperationID     string
	TenantID        uint64
	KBID            string
	ModelProfileRef string
	BudgetRef       string
	Messages        []types.SemanticModelMessage
	MaxOutputTokens int
	EstimatedTokens int
}

const defaultEstimatedTokens = 4096

type SemanticModelService struct {
	db       *gorm.DB
	provider types.SemanticModelProvider
}

func NewSemanticModelService(db *gorm.DB, provider types.SemanticModelProvider) *SemanticModelService {
	return &SemanticModelService{db: db, provider: provider}
}

func (s *SemanticModelService) Invoke(ctx context.Context, req SemanticModelInvocation) (types.SemanticModelResult, error) {
	claim, err := s.claim(ctx, req)
	if err != nil {
		return types.SemanticModelResult{}, err
	}
	switch claim.state {
	case "completed":
		return claim.savedResult, nil
	case "in_flight", "unknown":
		return types.SemanticModelResult{}, fmt.Errorf("%w: invocation %s is %s", ErrSemanticInvocationNeedsReconciliation, req.InvocationID, claim.state)
	}
	estimate := req.EstimatedTokens
	if estimate <= 0 {
		estimate = defaultEstimatedTokens
	}
	if err := s.reserve(ctx, req.BudgetRef, req.InvocationID, int64(estimate)); err != nil {
		// The claim must not stay in_flight: the provider never ran. A
		// budget refusal is retryable with the SAME id once affordable -
		// remove the claim row so the retry claims fresh.
		markCtx, markCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer markCancel()
		if errors.Is(err, ErrSemanticBudgetExhausted) {
			_ = s.db.WithContext(markCtx).Exec("DELETE FROM semantic_invocations WHERE invocation_id = ?", req.InvocationID).Error
		} else {
			_ = s.saveFailed(markCtx, req.InvocationID, err)
		}
		return types.SemanticModelResult{}, err
	}
	result, callErr := s.provider.Invoke(ctx, types.SemanticModelRequest{
		InvocationID: req.InvocationID, OperationID: req.OperationID,
		ModelProfileRef: req.ModelProfileRef, Messages: req.Messages,
		MaxOutputTokens: req.MaxOutputTokens,
	})
	result.InvocationID = req.InvocationID
	if callErr != nil {
		// The caller context may already be dead (deadline/cancel): ledger
		// writes on failure paths MUST use a detached context with their own
		// short budget so the ledger survives the very failure it records.
		ledgerCtx, ledgerCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer ledgerCancel()
		if errors.Is(callErr, context.DeadlineExceeded) || errors.Is(callErr, context.Canceled) {
			if saveErr := s.saveUnknown(ledgerCtx, req.InvocationID, callErr); saveErr != nil {
				return types.SemanticModelResult{}, saveErr
			}
			return types.SemanticModelResult{}, fmt.Errorf(
				"%w: invocation %s outcome unknown (provider may have executed); reconcile before any retry",
				ErrSemanticInvocationNeedsReconciliation, req.InvocationID)
		}
		// Definite pre-execution failure: release the reservation and
		// SURFACE the provider error - never fake success.
		_ = s.release(ledgerCtx, req.BudgetRef, req.InvocationID)
		if saveErr := s.saveFailed(ledgerCtx, req.InvocationID, callErr); saveErr != nil {
			return types.SemanticModelResult{}, fmt.Errorf("provider failure (ledger save also failed: %v): %w", saveErr, callErr)
		}
		return types.SemanticModelResult{}, fmt.Errorf("provider failure for invocation %s: %w", req.InvocationID, callErr)
	}
	if err := s.saveCompleted(ctx, req.InvocationID, result); err != nil {
		return types.SemanticModelResult{}, err
	}
	if err := s.finalize(ctx, req.BudgetRef, req.InvocationID, int64(result.InputTokens+result.OutputTokens)); err != nil {
		return types.SemanticModelResult{}, err
	}
	return result, nil
}

func (s *SemanticModelService) Retry(ctx context.Context, parentInvocationID string) (types.SemanticModelResult, error) {
	var parent struct {
		OperationID     string
		TenantID        uint64
		KBID            string
		ModelProfileRef string
		BudgetRef       string
		Messages        string
		MaxOutputTokens int
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT operation_id, tenant_id, kb_id, model_profile_ref, budget_ref, messages, max_output_tokens FROM semantic_invocations WHERE invocation_id = ?",
		parentInvocationID,
	).Scan(&parent).Error; err != nil {
		return types.SemanticModelResult{}, err
	}
	newID := parentInvocationID + "-r" + time.Now().UTC().Format("20060102150405.000000000")
	if _, err := s.Invoke(ctx, SemanticModelInvocation{
		InvocationID: newID, OperationID: parent.OperationID,
		TenantID: parent.TenantID, KBID: parent.KBID,
		ModelProfileRef: parent.ModelProfileRef, BudgetRef: parent.BudgetRef,
		Messages: parseMessagesJSON(parent.Messages), MaxOutputTokens: parent.MaxOutputTokens,
	}); err != nil {
		return types.SemanticModelResult{}, err
	}
	return types.SemanticModelResult{InvocationID: newID, Status: "completed"}, s.db.WithContext(ctx).Exec(
		"UPDATE semantic_invocations SET parent_invocation_id = ? WHERE invocation_id = ? AND parent_invocation_id IS NULL",
		parentInvocationID, newID,
	).Error
}

func (s *SemanticModelService) Reconcile(ctx context.Context, invocationID string, inputTokens, outputTokens int) error {
	total := int64(inputTokens + outputTokens)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE semantic_invocations SET state = ?, input_tokens = ?, output_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ? AND state = ?",
			"reconciled", inputTokens, outputTokens, invocationID, "unknown",
		).Error; err != nil {
			return err
		}
		// Refund the unspent part of the kept reservation (same accounting
		// as finalize) so reconciled budgets match actual usage.
		var upper int64
		if err := tx.Raw("SELECT upper_bound FROM semantic_budget_reservations WHERE invocation_id = ?", invocationID).Scan(&upper).Error; err != nil {
			return err
		}
		if upper > total {
			var budgetRef string
			if err := tx.Raw("SELECT budget_ref FROM semantic_budget_reservations WHERE invocation_id = ?", invocationID).Scan(&budgetRef).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE semantic_budgets SET spent_units = spent_units - ? WHERE budget_ref = ?", upper-total, budgetRef).Error; err != nil {
				return err
			}
		}
		return tx.Exec(
			"UPDATE semantic_budget_reservations SET state = ?, actual_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ? AND state = ?",
			"finalized", total, invocationID, "reconciling",
		).Error
	})
}

type invocationClaim struct {
	state       string
	savedResult types.SemanticModelResult
}

func (s *SemanticModelService) claim(ctx context.Context, req SemanticModelInvocation) (invocationClaim, error) {
	requestHash := hashMessages(req.Messages)
	var existing struct {
		State             string
		Status            string
		Text              *string
		InputTokens       *int64
		OutputTokens      *int64
		ProviderRequestID *string
		RequestHash       string
	}
	if err := s.db.WithContext(ctx).Raw(
		"SELECT state, status, text, input_tokens, output_tokens, provider_request_id, request_hash FROM semantic_invocations WHERE invocation_id = ?",
		req.InvocationID,
	).Scan(&existing).Error; err != nil {
		return invocationClaim{}, err
	}
	if existing.State != "" {
		if existing.RequestHash != requestHash {
			return invocationClaim{}, fmt.Errorf("%w: invocation %s payload changed", ErrSemanticInvocationConflict, req.InvocationID)
		}
		if existing.State == "failed" || existing.State == "reconciled" {
			// Terminal non-completed states block same-ID retries: mint a
			// new invocation ID via Retry (never silently re-execute).
			return invocationClaim{}, fmt.Errorf("%w: invocation %s already %s; retry with a new invocation ID", ErrSemanticInvocationNeedsReconciliation, req.InvocationID, existing.State)
		}
		if existing.State == "completed" {
			return invocationClaim{state: "completed", savedResult: types.SemanticModelResult{
				InvocationID: req.InvocationID, Text: derefString(existing.Text),
				InputTokens: int(derefInt64(existing.InputTokens)), OutputTokens: int(derefInt64(existing.OutputTokens)),
				ProviderRequestID: derefString(existing.ProviderRequestID), Status: existing.Status,
			}}, nil
		}
		return invocationClaim{state: existing.State}, nil
	}
	if err := s.db.WithContext(ctx).Exec(
		"INSERT INTO semantic_invocations (invocation_id, operation_id, tenant_id, kb_id, model_profile_ref, budget_ref, request_hash, state, messages, max_output_tokens) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.InvocationID, req.OperationID, req.TenantID, req.KBID, req.ModelProfileRef, req.BudgetRef, requestHash, "in_flight", messagesToJSON(req.Messages), req.MaxOutputTokens,
	).Error; err != nil {
		return invocationClaim{}, err
	}
	return invocationClaim{state: "new"}, nil
}

func (s *SemanticModelService) reserve(ctx context.Context, budgetRef, invocationID string, upperBound int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"INSERT INTO semantic_budgets (budget_ref, total_units) VALUES (?, 1000000000) ON CONFLICT (budget_ref) DO NOTHING",
			budgetRef,
		).Error; err != nil {
			return err
		}
		// Atomic conditional admission: the guard runs inside the UPDATE,
		// so concurrent reserves can never overshoot (check-then-act would
		// race on PostgreSQL where readers don't block writers).
		result := tx.Exec(
			"UPDATE semantic_budgets SET spent_units = spent_units + ? WHERE budget_ref = ? AND spent_units + ? <= total_units",
			upperBound, budgetRef, upperBound,
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("%w: budget %s cannot admit bound=%d", ErrSemanticBudgetExhausted, budgetRef, upperBound)
		}
		return tx.Exec(
			"INSERT INTO semantic_budget_reservations (budget_ref, invocation_id, upper_bound, state) VALUES (?, ?, ?, ?) ON CONFLICT (budget_ref, invocation_id) DO NOTHING",
			budgetRef, invocationID, upperBound, "reserved",
		).Error
	})
}

func (s *SemanticModelService) finalize(ctx context.Context, budgetRef, invocationID string, actual int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var upper int64
		if err := tx.Raw("SELECT upper_bound FROM semantic_budget_reservations WHERE budget_ref = ? AND invocation_id = ?", budgetRef, invocationID).Scan(&upper).Error; err != nil {
			return err
		}
		if upper > actual {
			if err := tx.Exec("UPDATE semantic_budgets SET spent_units = spent_units - ? WHERE budget_ref = ?", upper-actual, budgetRef).Error; err != nil {
				return err
			}
		}
		return tx.Exec(
			"UPDATE semantic_budget_reservations SET state = ?, actual_tokens = ?, updated_at = CURRENT_TIMESTAMP WHERE budget_ref = ? AND invocation_id = ?",
			"finalized", actual, budgetRef, invocationID,
		).Error
	})
}

func (s *SemanticModelService) release(ctx context.Context, budgetRef, invocationID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var upper int64
		if err := tx.Raw("SELECT upper_bound FROM semantic_budget_reservations WHERE budget_ref = ? AND invocation_id = ?", budgetRef, invocationID).Scan(&upper).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE semantic_budgets SET spent_units = spent_units - ? WHERE budget_ref = ?", upper, budgetRef).Error; err != nil {
			return err
		}
		return tx.Exec(
			"UPDATE semantic_budget_reservations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE budget_ref = ? AND invocation_id = ?",
			"released", budgetRef, invocationID,
		).Error
	})
}

func (s *SemanticModelService) saveCompleted(ctx context.Context, invocationID string, result types.SemanticModelResult) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE semantic_invocations SET state = ?, status = ?, text = ?, input_tokens = ?, output_tokens = ?, provider_request_id = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ?",
		"completed", result.Status, result.Text, result.InputTokens, result.OutputTokens, result.ProviderRequestID, invocationID,
	).Error
}

func (s *SemanticModelService) saveFailed(ctx context.Context, invocationID string, callErr error) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE semantic_invocations SET state = ?, error = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ?",
		"failed", callErr.Error(), invocationID,
	).Error
}

func (s *SemanticModelService) saveUnknown(ctx context.Context, invocationID string, callErr error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"UPDATE semantic_invocations SET state = ?, error = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ?",
			"unknown", callErr.Error(), invocationID,
		).Error; err != nil {
			return err
		}
		return tx.Exec(
			"UPDATE semantic_budget_reservations SET state = ?, updated_at = CURRENT_TIMESTAMP WHERE invocation_id = ?",
			"reconciling", invocationID,
		).Error
	})
}

func (s *SemanticModelService) MarkUnknownForTest(ctx context.Context, invocationID string) error {
	return s.saveUnknown(ctx, invocationID, errors.New("forced unknown for test"))
}

func (s *SemanticModelService) SetBudgetForTest(ctx context.Context, budgetRef string, total int64) error {
	return s.db.WithContext(ctx).Exec(
		"INSERT INTO semantic_budgets (budget_ref, total_units) VALUES (?, ?) ON CONFLICT (budget_ref) DO UPDATE SET total_units = excluded.total_units",
		budgetRef, total,
	).Error
}

func hashMessages(messages []types.SemanticModelMessage) string {
	sum := sha256.Sum256([]byte(messagesToJSON(messages)))
	return hex.EncodeToString(sum[:])
}

func messagesToJSON(messages []types.SemanticModelMessage) string {
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		parts = append(parts, fmt.Sprintf("{%q:%q}", m.Role, m.Content))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func parseMessagesJSON(raw string) []types.SemanticModelMessage {
	if raw == "" {
		return nil
	}
	var messages []types.SemanticModelMessage
	_ = json.Unmarshal([]byte(raw), &messages)
	return messages
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
