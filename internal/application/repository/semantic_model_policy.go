package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"gorm.io/gorm"
)

var (
	ErrSemanticModelPolicyNotFound = errors.New("semantic_model_policy_not_found")
	ErrSemanticModelPolicyInvalid  = errors.New("semantic_model_policy_invalid")
	ErrSemanticModelPolicyConflict = errors.New("semantic_model_policy_conflict")
)

// SemanticModelPolicy is the durable, source-owner policy. It deliberately
// contains only a price-version reference and derived integer bounds: rate
// cards and provider credentials never enter this store.
type SemanticModelPolicy struct {
	TenantID, PolicyVersion                                        uint64
	KBID, ModelID, Funding, PriceVersion, UpdatedBy                string
	ModelCallsEnabled                                              bool
	MaxInputTokensPerCall, MaxOutputTokensPerCall                  int64
	MaxCallsPerTask, MaxInputTokensPerTask, MaxOutputTokensPerTask int64
	PerCallUpperMicro                                              int64
	TaskUpperMicro                                                 *int64
	UpdatedAt                                                      time.Time
}

type SemanticModelPolicyRepository struct{ db *gorm.DB }

func NewSemanticModelPolicyRepository(db *gorm.DB) *SemanticModelPolicyRepository {
	return &SemanticModelPolicyRepository{db: db}
}

func (r *SemanticModelPolicyRepository) Get(ctx context.Context, tenantID uint64, kbID string) (*SemanticModelPolicy, error) {
	if r == nil || r.db == nil || tenantID == 0 || kbID == "" {
		return nil, ErrSemanticModelPolicyInvalid
	}
	return r.scanOne(r.db.WithContext(ctx).Raw(policySelect+" WHERE tenant_id=? AND kb_id=?", semanticUint(tenantID), kbID))
}

func (r *SemanticModelPolicyRepository) ListRevisions(ctx context.Context, tenantID uint64, kbID string) ([]SemanticModelPolicy, error) {
	if r == nil || r.db == nil || tenantID == 0 || kbID == "" {
		return nil, ErrSemanticModelPolicyInvalid
	}
	rows, err := r.db.WithContext(ctx).Raw(policyRevisionSelect+" WHERE tenant_id=? AND kb_id=? ORDER BY policy_version", semanticUint(tenantID), kbID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SemanticModelPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *p)
	}
	return result, rows.Err()
}

func (r *SemanticModelPolicyRepository) Put(ctx context.Context, policy SemanticModelPolicy) (*SemanticModelPolicy, error) {
	if r == nil || r.db == nil || !validPolicy(policy) {
		return nil, ErrSemanticModelPolicyInvalid
	}
	var saved *SemanticModelPolicy
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current *SemanticModelPolicy
		q := policySelect + " WHERE tenant_id=? AND kb_id=?"
		if tx.Dialector.Name() == "postgres" {
			q += " FOR UPDATE"
		}
		var err error
		current, err = r.scanOne(tx.Raw(q, semanticUint(policy.TenantID), policy.KBID))
		if errors.Is(err, ErrSemanticModelPolicyNotFound) {
			current = nil
		} else if err != nil {
			return err
		}
		if current == nil {
			policy.PolicyVersion = 1
		} else {
			if current.PolicyVersion == math.MaxUint64 {
				return ErrSemanticModelPolicyConflict
			}
			policy.PolicyVersion = current.PolicyVersion + 1
		}
		policy.UpdatedAt = time.Now().UTC()
		args := policyArgs(policy)
		if current == nil {
			if err := tx.Exec(policyInsert, args...).Error; err != nil {
				return err
			}
		} else {
			res := tx.Exec(policyUpdate, append(policyUpdateArgs(policy), semanticUint(policy.TenantID), policy.KBID, semanticUint(current.PolicyVersion))...)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrSemanticModelPolicyConflict
			}
		}
		if err := tx.Exec(policyRevisionInsert, args...).Error; err != nil {
			return err
		}
		copy := policy
		saved = &copy
		return nil
	})
	return saved, err
}

const policyColumns = "tenant_id,kb_id,model_calls_enabled,model_id,funding,price_version,max_input_tokens_per_call,max_output_tokens_per_call,max_calls_per_task,max_input_tokens_per_task,max_output_tokens_per_task,per_call_upper_micro,task_upper_micro,policy_version,updated_by,updated_at"
const policySelect = "SELECT " + policyColumns + " FROM semantic_model_policies"
const policyRevisionSelect = "SELECT " + policyColumns + " FROM semantic_model_policy_revisions"
const policyInsert = "INSERT INTO semantic_model_policies(" + policyColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
const policyRevisionInsert = "INSERT INTO semantic_model_policy_revisions(" + policyColumns + ") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
const policyUpdate = "UPDATE semantic_model_policies SET model_calls_enabled=?,model_id=?,funding=?,price_version=?,max_input_tokens_per_call=?,max_output_tokens_per_call=?,max_calls_per_task=?,max_input_tokens_per_task=?,max_output_tokens_per_task=?,per_call_upper_micro=?,task_upper_micro=?,policy_version=?,updated_by=?,updated_at=? WHERE tenant_id=? AND kb_id=? AND policy_version=?"

func policyArgs(p SemanticModelPolicy) []any {
	return []any{semanticUint(p.TenantID), p.KBID, p.ModelCallsEnabled, p.ModelID, p.Funding, p.PriceVersion, p.MaxInputTokensPerCall, p.MaxOutputTokensPerCall, p.MaxCallsPerTask, p.MaxInputTokensPerTask, p.MaxOutputTokensPerTask, p.PerCallUpperMicro, p.TaskUpperMicro, semanticUint(p.PolicyVersion), p.UpdatedBy, p.UpdatedAt}
}
func policyUpdateArgs(p SemanticModelPolicy) []any {
	return []any{p.ModelCallsEnabled, p.ModelID, p.Funding, p.PriceVersion, p.MaxInputTokensPerCall, p.MaxOutputTokensPerCall, p.MaxCallsPerTask, p.MaxInputTokensPerTask, p.MaxOutputTokensPerTask, p.PerCallUpperMicro, p.TaskUpperMicro, semanticUint(p.PolicyVersion), p.UpdatedBy, p.UpdatedAt}
}
func validPolicy(p SemanticModelPolicy) bool {
	return p.TenantID > 0 && p.KBID != "" && p.ModelID != "" && (p.Funding == "platform" || p.Funding == "byok") && p.PriceVersion != "" && p.MaxInputTokensPerCall > 0 && p.MaxOutputTokensPerCall > 0 && p.MaxCallsPerTask > 0 && p.MaxInputTokensPerTask > 0 && p.MaxOutputTokensPerTask > 0 && p.PerCallUpperMicro >= 0 && p.UpdatedBy != ""
}
func (r *SemanticModelPolicyRepository) scanOne(q *gorm.DB) (*SemanticModelPolicy, error) {
	row, err := q.Rows()
	if err != nil {
		return nil, err
	}
	defer row.Close()
	if !row.Next() {
		return nil, ErrSemanticModelPolicyNotFound
	}
	return scanPolicy(row)
}

type policyScanner interface{ Scan(...any) error }

func scanPolicy(s policyScanner) (*SemanticModelPolicy, error) {
	var p SemanticModelPolicy
	var tenant, version string
	var upper *int64
	if err := s.Scan(&tenant, &p.KBID, &p.ModelCallsEnabled, &p.ModelID, &p.Funding, &p.PriceVersion, &p.MaxInputTokensPerCall, &p.MaxOutputTokensPerCall, &p.MaxCallsPerTask, &p.MaxInputTokensPerTask, &p.MaxOutputTokensPerTask, &p.PerCallUpperMicro, &upper, &version, &p.UpdatedBy, &p.UpdatedAt); err != nil {
		return nil, err
	}
	var err error
	if p.TenantID, err = strconv.ParseUint(tenant, 10, 64); err != nil {
		return nil, fmt.Errorf("invalid policy tenant: %w", err)
	}
	if p.PolicyVersion, err = strconv.ParseUint(version, 10, 64); err != nil {
		return nil, fmt.Errorf("invalid policy version: %w", err)
	}
	p.TaskUpperMicro = upper
	return &p, nil
}
