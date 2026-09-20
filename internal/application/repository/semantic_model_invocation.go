package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

var (
	ErrSemanticModelInvocationConflict      = errors.New("semantic_model_invocation_conflict")
	ErrSemanticModelInvocationQuotaExceeded = errors.New("semantic_model_invocation_quota_exceeded")
	ErrSemanticModelInvocationNotFound      = errors.New("semantic_model_invocation_not_found")
)

type SemanticModelInvocationStore struct{ db *gorm.DB }

func NewSemanticModelInvocationStore(db *gorm.DB) *SemanticModelInvocationStore {
	return &SemanticModelInvocationStore{db: db}
}

func (s *SemanticModelInvocationStore) EnsureRun(ctx context.Context, c types.SemanticModelCapability) error {
	if s == nil || s.db == nil || !validSemanticCapability(c) {
		return ErrSemanticModelInvocationConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int
		err := tx.Raw("SELECT COUNT(*) FROM semantic_model_invocation_runs WHERE tenant_id=? AND run_id=? AND kb_id=? AND scope_hash=? AND policy_version=? AND model_id=? AND funding=? AND price_version=? AND max_input_tokens_per_call=? AND max_output_tokens_per_call=? AND per_call_upper_micro=? AND max_calls_per_task=? AND max_input_tokens_per_task=? AND max_output_tokens_per_task=?", semanticUint(c.OwnerTenantID), c.RunID, c.KBID, c.ScopeHash, semanticUint(c.PolicyVersion), c.ModelID, c.Funding, c.PriceVersion, c.MaxInputTokensPerCall, c.MaxOutputTokensPerCall, c.PerCallUpperMicro, c.MaxCallsPerTask, c.MaxInputTokensPerTask, c.MaxOutputTokensPerTask).Scan(&n).Error
		if err != nil {
			return err
		}
		if n == 1 {
			return nil
		}
		var existing int
		if err := tx.Raw("SELECT COUNT(*) FROM semantic_model_invocation_runs WHERE tenant_id=? AND run_id=?", semanticUint(c.OwnerTenantID), c.RunID).Scan(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return ErrSemanticModelInvocationConflict
		}
		return tx.Exec("INSERT INTO semantic_model_invocation_runs(tenant_id,run_id,kb_id,scope_hash,policy_version,model_id,funding,price_version,max_input_tokens_per_call,max_output_tokens_per_call,per_call_upper_micro,max_calls_per_task,max_input_tokens_per_task,max_output_tokens_per_task,expires_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", semanticUint(c.OwnerTenantID), c.RunID, c.KBID, c.ScopeHash, semanticUint(c.PolicyVersion), c.ModelID, c.Funding, c.PriceVersion, c.MaxInputTokensPerCall, c.MaxOutputTokensPerCall, c.PerCallUpperMicro, c.MaxCallsPerTask, c.MaxInputTokensPerTask, c.MaxOutputTokensPerTask, c.ExpiresAt.UTC()).Error
	})
}
func (s *SemanticModelInvocationStore) Claim(ctx context.Context, c types.SemanticModelCapability, requestHash string) (types.SemanticModelInvocationResult, error) {
	var result types.SemanticModelInvocationResult
	if s == nil || s.db == nil || !validSemanticCapability(c) || requestHash == "" {
		return result, ErrSemanticModelInvocationConflict
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.claimTx(tx, c, requestHash, &result) })
	// SQLite serializes writers. A transient lock is retried; PostgreSQL still
	// relies on its transaction isolation and unique call key for convergence.
	for attempts := 0; err != nil && strings.Contains(err.Error(), "database is locked") && attempts < 4; attempts++ {
		time.Sleep(time.Duration(attempts+1) * 10 * time.Millisecond)
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.claimTx(tx, c, requestHash, &result) })
	}
	return result, err
}
func (s *SemanticModelInvocationStore) claimTx(tx *gorm.DB, c types.SemanticModelCapability, requestHash string, result *types.SemanticModelInvocationResult) error {
	var run int
	if err := tx.Raw("SELECT COUNT(*) FROM semantic_model_invocation_runs WHERE tenant_id=? AND run_id=?", semanticUint(c.OwnerTenantID), c.RunID).Scan(&run).Error; err != nil {
		return err
	}
	if run != 1 {
		return ErrSemanticModelInvocationNotFound
	}
	var state, hash string
	var response []byte
	var in, out sql.NullInt64
	err := tx.Raw("SELECT state,request_hash,result,input_tokens,output_tokens FROM semantic_model_invocations WHERE tenant_id=? AND call_id=?", semanticUint(c.OwnerTenantID), c.CallID).Row().Scan(&state, &hash, &response, &in, &out)
	if err == nil {
		if hash != requestHash {
			return ErrSemanticModelInvocationConflict
		}
		if state == "completed" {
			*result = types.SemanticModelInvocationResult{Result: response, InputTokens: in.Int64, OutputTokens: out.Int64}
			return nil
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var calls, inputs, outputs int64
	if err := tx.Raw("SELECT COALESCE(COUNT(*),0),COALESCE(SUM(CASE WHEN state='completed' THEN input_tokens ELSE reserved_input_tokens END),0),COALESCE(SUM(CASE WHEN state='completed' THEN output_tokens ELSE reserved_output_tokens END),0) FROM semantic_model_invocations WHERE tenant_id=? AND run_id=? AND state IN ('claimed','unknown','completed')", semanticUint(c.OwnerTenantID), c.RunID).Row().Scan(&calls, &inputs, &outputs); err != nil {
		return err
	}
	if calls >= c.MaxCallsPerTask || inputs+c.MaxInputTokensPerCall > c.MaxInputTokensPerTask || outputs+c.MaxOutputTokensPerCall > c.MaxOutputTokensPerTask {
		return ErrSemanticModelInvocationQuotaExceeded
	}
	return tx.Exec("INSERT INTO semantic_model_invocations(tenant_id,run_id,call_id,request_hash,state,reserved_input_tokens,reserved_output_tokens) VALUES (?,?,?,?,?,?,?)", semanticUint(c.OwnerTenantID), c.RunID, c.CallID, requestHash, "claimed", c.MaxInputTokensPerCall, c.MaxOutputTokensPerCall).Error
}
func (s *SemanticModelInvocationStore) Complete(ctx context.Context, c types.SemanticModelCapability, result types.SemanticModelInvocationResult) error {
	return s.transition(ctx, c, "completed", result, true)
}
func (s *SemanticModelInvocationStore) FailBeforeDispatch(ctx context.Context, c types.SemanticModelCapability) error {
	return s.transition(ctx, c, "failed_before_dispatch", types.SemanticModelInvocationResult{}, false)
}
func (s *SemanticModelInvocationStore) MarkUnknown(ctx context.Context, c types.SemanticModelCapability) error {
	return s.transition(ctx, c, "unknown", types.SemanticModelInvocationResult{}, false)
}
func (s *SemanticModelInvocationStore) transition(ctx context.Context, c types.SemanticModelCapability, state string, result types.SemanticModelInvocationResult, usage bool) error {
	if s == nil || s.db == nil {
		return ErrSemanticModelInvocationNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		args := []any{state, result.Result, result.InputTokens, result.OutputTokens, semanticUint(c.OwnerTenantID), c.CallID}
		q := "UPDATE semantic_model_invocations SET state=?,result=?,input_tokens=?,output_tokens=? WHERE tenant_id=? AND call_id=? AND state='claimed'"
		if !usage {
			q = "UPDATE semantic_model_invocations SET state=? WHERE tenant_id=? AND call_id=? AND state='claimed'"
			args = []any{state, semanticUint(c.OwnerTenantID), c.CallID}
		}
		res := tx.Exec(q, args...)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrSemanticModelInvocationNotFound
		}
		return nil
	})
}
func validSemanticCapability(c types.SemanticModelCapability) bool {
	return c.OwnerTenantID != 0 && c.KBID != "" && c.ScopeHash != "" && c.RunID != "" && c.CallID != "" && c.PolicyVersion != 0 && c.MaxCallsPerTask > 0 && c.MaxInputTokensPerCall > 0 && c.MaxOutputTokensPerCall > 0 && c.MaxInputTokensPerTask >= c.MaxInputTokensPerCall && c.MaxOutputTokensPerTask >= c.MaxOutputTokensPerCall && !c.ExpiresAt.IsZero()
}
