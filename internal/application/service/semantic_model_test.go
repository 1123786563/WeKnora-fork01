package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tencent/WeKnora/internal/types"
)

// controlledProvider is the A03 controlled model provider: counts real
// calls, returns deterministic usage, and can simulate a lost response
// (unknown outcome) or deadline overrun.
type controlledProvider struct {
	calls        int
	failMode     string // "", "unknown", "error"
	inputTokens  int
	outputTokens int
	lastDeadline time.Duration
}

func (p *controlledProvider) Invoke(ctx context.Context, req types.SemanticModelRequest) (types.SemanticModelResult, error) {
	p.calls++
	if dl, ok := ctx.Deadline(); ok {
		p.lastDeadline = time.Until(dl)
	}
	switch p.failMode {
	case "error":
		return types.SemanticModelResult{}, errors.New("provider rejected")
	case "unknown":
		p.failMode = "" // one-shot: the retry in the same test succeeds
		return types.SemanticModelResult{}, context.DeadlineExceeded
	}
	return types.SemanticModelResult{
		Text:              fmt.Sprintf("answer-%d", p.calls),
		InputTokens:       p.inputTokens,
		OutputTokens:      p.outputTokens,
		ProviderRequestID: fmt.Sprintf("pr-req-%d", p.calls),
		Status:            "completed",
	}, nil
}

func newSemanticModelFixture(t *testing.T) (*SemanticModelService, *controlledProvider, *gorm.DB) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "semantic-model.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	provider := &controlledProvider{inputTokens: 10, outputTokens: 20}
	svc := NewSemanticModelService(db, provider)
	return svc, provider, db
}

func modelMessages() []types.SemanticModelMessage {
	return []types.SemanticModelMessage{{Role: "user", Content: "甲公司控股乙公司？"}}
}

func TestSemanticModelInvokeHappyPath(t *testing.T) {
	svc, provider, db := newSemanticModelFixture(t)
	result, err := svc.Invoke(context.Background(), SemanticModelInvocation{
		InvocationID: "inv-1", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(),
		MaxOutputTokens: 100,
	})
	require.NoError(t, err)
	require.Equal(t, "completed", result.Status)
	require.Equal(t, 1, provider.calls)
	var row map[string]any
	require.NoError(t, db.Raw("SELECT state, input_tokens, output_tokens FROM semantic_invocations WHERE invocation_id = ?", "inv-1").Scan(&row).Error)
	require.Equal(t, "completed", row["state"])
	require.Equal(t, int64(10), row["input_tokens"])
}

func TestSemanticModelRetrySameInvocationReturnsSavedResult(t *testing.T) {
	svc, provider, _ := newSemanticModelFixture(t)
	req := SemanticModelInvocation{
		InvocationID: "inv-1", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(),
		MaxOutputTokens: 100,
	}
	first, err := svc.Invoke(context.Background(), req)
	require.NoError(t, err)
	second, err := svc.Invoke(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, provider.calls, "same invocation must not hit the provider twice")
}

func TestSemanticModelInFlightNeedsReconciliation(t *testing.T) {
	svc, _, _ := newSemanticModelFixture(t)
	req := SemanticModelInvocation{InvocationID: "inv-u", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(), MaxOutputTokens: 100}
	_, err := svc.Invoke(context.Background(), req)
	require.NoError(t, err)
	// Force the ledger into the unknown state (simulating a lost response).
	require.NoError(t, svc.MarkUnknownForTest(context.Background(), "inv-u"))
	_, err = svc.Invoke(context.Background(), req)
	require.ErrorIs(t, err, ErrSemanticInvocationNeedsReconciliation, "unknown outcomes must block blind retries")
}

func TestSemanticModelRetryGetsNewInvocationID(t *testing.T) {
	svc, provider, db := newSemanticModelFixture(t)
	base := SemanticModelInvocation{OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(), MaxOutputTokens: 100}
	first, err := svc.Invoke(context.Background(), withInvocation(base, "inv-r1"))
	require.NoError(t, err)
	// Explicit retry after a failed call: a NEW invocation ID (linked to the
	// parent operation) performs a new real provider call.
	retry, err := svc.Retry(context.Background(), "inv-r1")
	require.NoError(t, err)
	require.NotEqual(t, first.InvocationID, retry.InvocationID)
	require.Equal(t, 2, provider.calls)
	var parents int64
	db.Raw("SELECT COUNT(*) FROM semantic_invocations WHERE parent_invocation_id = ?", "inv-r1").Scan(&parents)
	require.Equal(t, int64(1), parents, "retry must be linked to its parent")
}

func TestSemanticModelBudgetExhaustionRejectsBeforeCall(t *testing.T) {
	svc, provider, _ := newSemanticModelFixture(t)
	require.NoError(t, svc.SetBudgetForTest(context.Background(), "budget-a", 30))
	req := SemanticModelInvocation{InvocationID: "inv-b", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(),
		MaxOutputTokens: 100, EstimatedTokens: 50}
	_, err := svc.Invoke(context.Background(), req)
	require.ErrorIs(t, err, ErrSemanticBudgetExhausted)
	require.Zero(t, provider.calls, "budget admission must run BEFORE the provider call")
}

func TestSemanticModelConcurrentBudgetNeverOvershoots(t *testing.T) {
	svc, _, db := newSemanticModelFixture(t)
	require.NoError(t, svc.SetBudgetForTest(context.Background(), "budget-race", 100))
	barrier := make(chan struct{})
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			<-barrier
			_, err := svc.Invoke(context.Background(), SemanticModelInvocation{
				InvocationID: fmt.Sprintf("inv-race-%d", n), OperationID: "op-r",
				TenantID: 7, KBID: "kb-x", ModelProfileRef: "m", BudgetRef: "budget-race",
				Messages: modelMessages(), EstimatedTokens: 40,
			})
			results <- err
		}(i)
	}
	close(barrier)
	// The atomic invariant: spent_units NEVER exceeds the total. With
	// concurrent finalize refunds (actual < bound), more than two
	// admissions can be legitimately allowed - the bound is the invariant,
	// not the admission count.
	for i := 0; i < 10; i++ {
		<-results
	}
	var spent int64
	db.Raw("SELECT spent_units FROM semantic_budgets WHERE budget_ref = 'budget-race'").Scan(&spent)
	require.LessOrEqual(t, spent, int64(100), "spent must never exceed the total, got %d", spent)
}

func TestSemanticModelBudgetRefusalDoesNotStrandInFlight(t *testing.T) {
	svc, _, db := newSemanticModelFixture(t)
	require.NoError(t, svc.SetBudgetForTest(context.Background(), "budget-tiny", 10))
	_, err := svc.Invoke(context.Background(), SemanticModelInvocation{
		InvocationID: "inv-strand", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-tiny",
		Messages: modelMessages(), EstimatedTokens: 50,
	})
	require.ErrorIs(t, err, ErrSemanticBudgetExhausted)
	var count int64
	db.Raw("SELECT COUNT(*) FROM semantic_invocations WHERE invocation_id = ?", "inv-strand").Scan(&count)
	require.Zero(t, count, "budget refusal must not leave the claim in_flight forever")
	// A later affordable retry of the SAME id proceeds (provider never ran).
	_, err = svc.Invoke(context.Background(), SemanticModelInvocation{
		InvocationID: "inv-strand", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-tiny",
		Messages: modelMessages(), EstimatedTokens: 5,
	})
	require.NoError(t, err, "refused-then-affordable retry must not need reconciliation")
}

type deadlineExceededAfterBlockProvider struct{ triggered chan struct{} }

func (p *deadlineExceededAfterBlockProvider) Invoke(ctx context.Context, _ types.SemanticModelRequest) (types.SemanticModelResult, error) {
	<-p.triggered
	<-ctx.Done() // wait until the REAL caller deadline fires
	return types.SemanticModelResult{}, ctx.Err()
}

func TestSemanticModelRealDeadlineRecordsUnknown(t *testing.T) {
	_, _, db := newSemanticModelFixture(t)
	provider := &deadlineExceededAfterBlockProvider{triggered: make(chan struct{})}
	svc := NewSemanticModelService(db, provider)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	go func() { close(provider.triggered) }()
	_, err := svc.Invoke(ctx, SemanticModelInvocation{
		InvocationID: "inv-dl", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a",
		Messages: modelMessages(), EstimatedTokens: 40,
	})
	require.ErrorIs(t, err, ErrSemanticInvocationNeedsReconciliation, "a real dead ctx must still produce the reconciliation error")
	var state string
	db.Raw("SELECT state FROM semantic_invocations WHERE invocation_id = ?", "inv-dl").Scan(&state)
	require.Equal(t, "unknown", state, "ledger writes must survive the dead caller context")
	var resState string
	db.Raw("SELECT state FROM semantic_budget_reservations WHERE invocation_id = ?", "inv-dl").Scan(&resState)
	require.Equal(t, "reconciling", resState)
}

func TestSemanticModelProviderFailureSurfacesNotFakeSuccess(t *testing.T) {
	_, _, db := newSemanticModelFixture(t)
	provider := &controlledProvider{inputTokens: 10, outputTokens: 20, failMode: "error"}
	svc := NewSemanticModelService(db, provider)
	_, err := svc.Invoke(context.Background(), SemanticModelInvocation{
		InvocationID: "inv-f", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a",
		Messages: modelMessages(), MaxOutputTokens: 100, EstimatedTokens: 40,
	})
	require.Error(t, err, "a provider rejection must NEVER surface as success")
	require.Contains(t, err.Error(), "provider")
	var state string
	db.Raw("SELECT state FROM semantic_invocations WHERE invocation_id = ?", "inv-f").Scan(&state)
	require.Equal(t, "failed", state)
	// The failed invocation must not be silently re-invokable as if new:
	// a retry of the SAME id is blocked (needs new id or reconciliation).
	_, err = svc.Invoke(context.Background(), SemanticModelInvocation{
		InvocationID: "inv-f", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a",
		Messages: modelMessages(), MaxOutputTokens: 100, EstimatedTokens: 40,
	})
	require.Error(t, err)
}

func TestSemanticModelUnknownOutcomeKeepsReservationForReconcile(t *testing.T) {
	_, _, db := newSemanticModelFixture(t)
	provider := &controlledProvider{inputTokens: 10, outputTokens: 20, failMode: "unknown"}
	svc2 := NewSemanticModelService(db, provider)
	_, err := svc2.Invoke(context.Background(), SemanticModelInvocation{InvocationID: "inv-k", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(), MaxOutputTokens: 100, EstimatedTokens: 40})
	require.Error(t, err)
	var state string
	db.Raw("SELECT state FROM semantic_invocations WHERE invocation_id = ?", "inv-k").Scan(&state)
	require.Equal(t, "unknown", state)
	var resState string
	db.Raw("SELECT state FROM semantic_budget_reservations WHERE invocation_id = ?", "inv-k").Scan(&resState)
	require.Equal(t, "reconciling", resState, "unknown outcomes keep the reservation for reconciliation")
}

func TestSemanticModelReconcileUnknownFinalizesUsage(t *testing.T) {
	svc, _, db := newSemanticModelFixture(t)
	// Seed an unknown invocation with a kept reservation.
	_, err := svc.Invoke(context.Background(), SemanticModelInvocation{InvocationID: "inv-c", OperationID: "op-1", TenantID: 7, KBID: "kb-x",
		ModelProfileRef: "model-a", BudgetRef: "budget-a", Messages: modelMessages(), MaxOutputTokens: 100, EstimatedTokens: 40})
	require.NoError(t, err)
	require.NoError(t, svc.MarkUnknownForTest(context.Background(), "inv-c"))
	// Reconciliation decides: usage seen by the provider becomes final.
	require.NoError(t, svc.Reconcile(context.Background(), "inv-c", 12, 8))
	var state string
	db.Raw("SELECT state FROM semantic_invocations WHERE invocation_id = ?", "inv-c").Scan(&state)
	require.Equal(t, "reconciled", state)
	var resState string
	db.Raw("SELECT state FROM semantic_budget_reservations WHERE invocation_id = ?", "inv-c").Scan(&resState)
	require.Equal(t, "finalized", resState)
}

func withInvocation(base SemanticModelInvocation, id string) SemanticModelInvocation {
	base.InvocationID = id
	return base
}
