package service

// T13 (#43) Task 3: the compliance access flow itself. The admin predicate
// (TenantRole admin+), the reason/TTL validation, the audit-first fail-closed
// rule, and the structural fact that a compliance window NEVER writes a
// task_grants row (the service holds no grant port at all — AC1 is
// type-level).

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubComplianceStore struct {
	policy   *types.TenantTaskPolicy
	facts    *types.TaskMetadataFacts
	factsErr error
	opened   []types.TaskComplianceAccess
	window   *types.TaskComplianceAccess
	messages []types.TaskMessageFact
	purgeFn  func() error
}

func (s *stubComplianceStore) GetTaskPolicy(context.Context, uint64) (*types.TenantTaskPolicy, error) {
	return s.policy, nil
}

func (s *stubComplianceStore) UpsertTaskPolicy(_ context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error) {
	s.policy = &policy
	return policy, nil
}

func (s *stubComplianceStore) TaskMetadataFacts(context.Context, uint64, string) (*types.TaskMetadataFacts, error) {
	return s.facts, s.factsErr
}

func (s *stubComplianceStore) OpenComplianceAccess(_ context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error) {
	access.ID = uint64(len(s.opened) + 1)
	s.opened = append(s.opened, access)
	return access, nil
}

func (s *stubComplianceStore) ActiveComplianceAccess(context.Context, uint64, string, string, time.Time) (*types.TaskComplianceAccess, error) {
	return s.window, nil
}

func (s *stubComplianceStore) ListTaskMessages(context.Context, string, int) ([]types.TaskMessageFact, error) {
	return s.messages, nil
}

func (s *stubComplianceStore) PurgeTask(context.Context, uint64, string) error {
	if s.purgeFn != nil {
		return s.purgeFn()
	}
	return nil
}

type recordingAudit struct {
	interfaces.AuditLogService
	entries []types.AuditLog
	err     error
}

func (r *recordingAudit) Log(_ context.Context, entry *types.AuditLog) error {
	if r.err != nil {
		return r.err
	}
	r.entries = append(r.entries, *entry)
	return nil
}

func complianceFixtures() (*stubComplianceStore, *recordingAudit, *TaskComplianceService) {
	store := &stubComplianceStore{
		facts: &types.TaskMetadataFacts{TaskID: "s1", Title: "task-s1", OwnerID: "u1", RunCount: 1, LastRunState: "succeeded"},
		window: &types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9",
			Reason: "security incident review", ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC()},
		messages: []types.TaskMessageFact{{ID: "m1", Role: "user", Content: "private question"}},
	}
	audit := &recordingAudit{}
	return store, audit, NewTaskComplianceService(store, audit)
}

func adminCaller() types.Caller {
	return types.Caller{TenantID: 1, UserID: "u9", Role: types.TenantRoleAdmin}
}

func TestSetTaskPolicyRequiresAdminAndValidates(t *testing.T) {
	_, audit, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.SetTaskPolicy(ctx, types.Caller{TenantID: 1, UserID: "u1", Role: types.TenantRoleContributor}, 30, false)
	require.Error(t, err, "non-admin cannot touch the tenant policy")
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, appErr.HTTPCode)

	policy, err := svc.SetTaskPolicy(ctx, adminCaller(), -1, false)
	require.Error(t, err, "negative retention days are refused")
	require.Nil(t, policy)

	policy, err = svc.SetTaskPolicy(ctx, adminCaller(), 30, true)
	require.NoError(t, err)
	require.Equal(t, 30, policy.RetentionDays)
	require.True(t, policy.LegalHold)
	require.Len(t, audit.entries, 1, "every policy write leaves an audit row")
	require.Equal(t, types.AuditActionTaskPolicyUpdated, audit.entries[0].Action)
	require.Equal(t, string(types.TenantRoleAdmin), audit.entries[0].ActorRole)
}

func TestRequestContentAccessValidatesReasonAndTTL(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.RequestContentAccess(ctx, adminCaller(), "s1", "   ", 2*time.Hour)
	require.Error(t, err, "a blank reason is refused")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", 0)
	require.Error(t, err, "zero TTL is refused")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", MaxComplianceAccessTTL+time.Hour)
	require.Error(t, err, "TTL above the 7-day ceiling is refused")

	access, err := svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit trail", 2*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "audit trail", access.Reason)
	require.Len(t, store.opened, 1, "one window row")
	require.Len(t, audit.entries, 1, "the window is fully audited (reason + horizon)")
	require.Equal(t, types.AuditActionComplianceAccessRequested, audit.entries[0].Action)
	require.Equal(t, types.AuditOutcomeSuccess, audit.entries[0].Outcome)
	require.Equal(t, "s1", audit.entries[0].TargetID)

	// A task miss is one uniform 404 — unknown and cross-tenant probes are
	// indistinguishable (T12 convention).
	store.factsErr = types.ErrTaskComplianceNotFound
	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s-missing", "audit", time.Hour)
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusNotFound, appErr.HTTPCode)
}

func TestReadTaskContentRequiresActiveWindow(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	content, err := svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", content.TaskID)
	require.Len(t, content.Messages, 1)
	require.Equal(t, "private question", content.Messages[0].Content)
	require.Len(t, audit.entries, 1)
	require.Equal(t, types.AuditActionComplianceContentRead, audit.entries[0].Action)

	// Review Focus 2: an expired (absent) window refuses the read, 403.
	store.window = nil
	_, err = svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusForbidden, appErr.HTTPCode)
}

func TestComplianceAuditFailClosed(t *testing.T) {
	// Review Focus 3: when the audit trail write fails, every compliance
	// operation refuses instead of silently proceeding without a record.
	store := &stubComplianceStore{
		facts:  &types.TaskMetadataFacts{TaskID: "s1", OwnerID: "u1"},
		window: &types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9", ExpiresAt: time.Now().Add(time.Hour)},
	}
	audit := &recordingAudit{err: context.DeadlineExceeded}
	svc := NewTaskComplianceService(store, audit)
	ctx := context.Background()

	_, err := svc.SetTaskPolicy(ctx, adminCaller(), 30, true)
	require.Error(t, err, "policy write fails closed without its audit row")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", time.Hour)
	require.Error(t, err, "no window opens without its audit row")
	require.Empty(t, store.opened, "fail closed means the store was never called")

	_, err = svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.Error(t, err, "content stays sealed without its audit row")

	// A nil audit service is a mis-assembly: fail closed with 503.
	svcNoAudit := NewTaskComplianceService(store, nil)
	_, err = svcNoAudit.RequestContentAccess(ctx, adminCaller(), "s1", "audit", time.Hour)
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusServiceUnavailable, appErr.HTTPCode)
}

func TestTaskMetadataIsAdminOnlyAndCarriesNoContent(t *testing.T) {
	_, _, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.TaskMetadata(ctx, types.Caller{TenantID: 1, UserID: "u1", Role: types.TenantRoleViewer}, "s1")
	require.Error(t, err, "the metadata lane is still admin-only")

	view, err := svc.TaskMetadata(ctx, adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "task-s1", view.Metadata.Title)
	require.Equal(t, int64(1), view.Metadata.RunCount)

	// Marker tripwire (compile-time): this assignment fails to compile only
	// if the MetadataOnly() method itself is removed — Go method sets ignore
	// fields, so it is a deliberate-change tripwire, NOT a field guarantee.
	var noContent interface{ MetadataOnly() } = view
	_ = noContent

	// Field-level guard (runtime — the real one): reflect over the view and
	// fail on ANY field outside the content-free whitelist, so adding a
	// content field to TaskMetadataView is a visible test change instead of
	// a silent drift (metadata needs no reason; content does).
	allowed := map[string]bool{"Metadata": true, "Policy": true}
	viewType := reflect.TypeOf(view)
	for i := 0; i < viewType.NumField(); i++ {
		require.True(t, allowed[viewType.Field(i).Name],
			"TaskMetadataView grew non-whitelisted field %q: content fields require a spec change (T13 #43)",
			viewType.Field(i).Name)
	}
}

// T13 (#43) Task 5: the legal-hold deletion gate. No policy → ungated
// (today's behavior preserved); hold on → refused AND audited; a broken
// audit trail never un-gates the hold (the audit is the trail, not the
// authority — the deletion is already denied).
func TestAllowsTaskDeletionUnderLegalHold(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	// No policy → ungated (today's behavior preserved).
	require.NoError(t, svc.AllowsTaskDeletion(ctx, 1, "u1", "s1"))

	// Policy without hold → allowed.
	store.policy = &types.TenantTaskPolicy{TenantID: 1}
	require.NoError(t, svc.AllowsTaskDeletion(ctx, 1, "u1", "s1"))

	// Legal hold → refused, and the refusal itself is audited.
	store.policy.LegalHold = true
	err := svc.AllowsTaskDeletion(ctx, 1, "u1", "s1")
	require.ErrorIs(t, err, ErrTaskLegalHold)
	require.Len(t, audit.entries, 1)
	require.Equal(t, types.AuditActionTaskDeleteDenied, audit.entries[0].Action)
	require.Equal(t, types.AuditOutcomeDenied, audit.entries[0].Outcome)
	require.Equal(t, "s1", audit.entries[0].TargetID)

	// A broken audit trail must not silently un-gate the hold: the refusal
	// stands even when its audit write fails (deletion is already denied).
	store.policy.LegalHold = true
	audit.err = context.DeadlineExceeded
	err = svc.AllowsTaskDeletion(ctx, 1, "u1", "s2")
	require.ErrorIs(t, err, ErrTaskLegalHold)
}

// Final-review finding (issue30-sweep t43): AllowsTaskDeletion called
// s.audit.Log without a nil check while its comment promised a nil audit
// merely "skips the trail but keeps the refusal" — in reality it panicked.
// Regression: a nil audit (mis-assembled deployment; production always
// Provides one) must skip the row AND still refuse, never panic.
func TestAllowsTaskDeletionWithNilAuditStillRefuses(t *testing.T) {
	store := &stubComplianceStore{
		facts: &types.TaskMetadataFacts{TaskID: "s1", Title: "task-s1", OwnerID: "u1", RunCount: 1, LastRunState: "succeeded"},
	}
	svc := NewTaskComplianceService(store, nil)
	ctx := context.Background()

	// Hold off → ungated even without an audit service.
	require.NoError(t, svc.AllowsTaskDeletion(ctx, 1, "u1", "s1"))

	// Hold on → refused (not panicked) with no audit row to write.
	store.policy = &types.TenantTaskPolicy{TenantID: 1, LegalHold: true}
	err := svc.AllowsTaskDeletion(ctx, 1, "u1", "s1")
	require.ErrorIs(t, err, ErrTaskLegalHold)
}

// T13 (#43) Task 6: the permanent-deletion check chain. Legal hold refuses
// first; a live (not soft-deleted) task refuses; the retention window
// refuses; past the horizon proceeds with the audit row written BEFORE the
// destructive transaction.
func TestPurgeTaskPolicyChain(t *testing.T) {
	deletedAt := time.Now().UTC().Add(-10 * 24 * time.Hour) // soft-deleted 10 days ago
	now := func() time.Time { return time.Now().UTC() }

	newStore := func() *stubComplianceStore {
		return &stubComplianceStore{
			facts: &types.TaskMetadataFacts{TaskID: "s1", OwnerID: "u1", DeletedAt: &deletedAt},
		}
	}
	audit := &recordingAudit{}

	// Legal hold refuses first, even before the soft-delete check.
	store := newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, LegalHold: true}
	_, err := NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskLegalHold)

	// A live (not soft-deleted) task refuses: purge never hard-deletes what
	// the user still sees.
	store = newStore()
	store.facts.DeletedAt = nil
	_, err = NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskNotSoftDeleted)

	// Inside the retention window (30 days, deleted 10 days ago) refuses.
	store = newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, RetentionDays: 30}
	_, err = NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskRetentionActive)

	// Past the horizon (retention 7 days, deleted 10 days ago) proceeds,
	// with the audit row written BEFORE the destructive transaction.
	store = newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, RetentionDays: 7}
	deleted := false
	store.purgeFn = func() error { deleted = true; return nil }
	receipt, err := NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", receipt.TaskID)
	require.True(t, deleted)
	require.NotEmpty(t, audit.entries, "purge leaves an audit row")
	require.Equal(t, types.AuditActionTaskPurged, audit.entries[len(audit.entries)-1].Action)
}
