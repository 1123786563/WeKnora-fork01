package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type applicationSeed struct {
	OpportunityID string
	SnapshotID    string
	EvaluationID  string
	Revision      uint64
}

func newApplicationOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-application.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, db, ctx
}

func seedApplicationEvaluation(t *testing.T, o *Office, ctx context.Context, jd, year, seedID string) applicationSeed {
	t.Helper()
	if year != "" {
		view, err := o.Open(ctx)
		require.NoError(t, err)
		_, err = o.Confirm(ctx, "education.graduation_year", year, seedID+"-year", view.Revision, Source{Kind: "manual"})
		require.NoError(t, err)
	}
	view, err := o.Open(ctx)
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: seedID + "-job", RawText: jd})
	require.NoError(t, err)
	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID:     seedID + "-eval",
		OpportunityID: job.OpportunityID,
		SnapshotID:    job.SnapshotID,
	})
	require.NoError(t, err)
	return applicationSeed{
		OpportunityID: job.OpportunityID,
		SnapshotID:    job.SnapshotID,
		EvaluationID:  evaluation.EvaluationID,
		Revision:      view.Revision,
	}
}

func applicationInput(seed applicationSeed, requestID, batch string) CreateApplicationInput {
	return CreateApplicationInput{
		RequestID:        requestID,
		OpportunityID:    seed.OpportunityID,
		SnapshotID:       seed.SnapshotID,
		EvaluationID:     seed.EvaluationID,
		BatchIdentity:    batch,
		ExpectedRevision: seed.Revision,
	}
}

// fakeCareerApplicationLinker records every call and returns configurable
// Ensure/Find results so the Office workflow can be driven through unknown,
// rejection, and success paths.
type fakeCareerApplicationLinker struct {
	mutex         sync.Mutex
	ensureCalls   int
	findCalls     int
	ensureErr     error
	findErr       error
	findLink      interfaces.CareerApplicationTaskLink
	lastIntent    interfaces.CareerApplicationTaskIntent
	ensureScopes  []ensureScope
	findArguments []findArgument
	taskSequence  int
	ensureStarted chan struct{}
	ensureRelease chan struct{}
}

type ensureScope struct {
	tenantID uint64
	ownerID  string
}

type findArgument struct {
	tenantID  uint64
	ownerID   string
	requestID string
}

func (f *fakeCareerApplicationLinker) EnsureCareerApplicationTask(
	_ context.Context, tenantID uint64, ownerID string, intent interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskLink, error) {
	if f.ensureStarted != nil {
		select {
		case f.ensureStarted <- struct{}{}:
		default:
		}
		<-f.ensureRelease
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.ensureCalls++
	f.lastIntent = intent
	f.ensureScopes = append(f.ensureScopes, ensureScope{tenantID: tenantID, ownerID: ownerID})
	if f.ensureErr != nil {
		return interfaces.CareerApplicationTaskLink{}, f.ensureErr
	}
	f.taskSequence++
	return interfaces.CareerApplicationTaskLink{
		TaskID: fmt.Sprintf("task-%d", f.taskSequence),
		RunID:  fmt.Sprintf("run-%d", f.taskSequence),
	}, nil
}

func TestDeleteCareerWaitsForCommittedApplicationWorkbenchLink(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1951)
	seed := seedApplicationEvaluation(t, o, ctx, "Go backend engineer", "2027", "delete-link-race")
	linker := &fakeCareerApplicationLinker{ensureStarted: make(chan struct{}, 1), ensureRelease: make(chan struct{})}
	o.SetApplicationTaskLinker(linker)
	remover := &fakeCareerTaskRemover{}
	o.SetApplicationTaskRemover(remover)
	appDone := make(chan error, 1)
	go func() {
		_, err := o.CreateApplication(ctx, applicationInput(seed, "delete-link-race-app", "batch-1"))
		appDone <- err
	}()
	<-linker.ensureStarted // Career commit is complete; linking is paused.

	deleteDone := make(chan error, 1)
	go func() {
		_, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-link-race-delete", ExpectedRevision: seed.Revision})
		deleteDone <- err
	}()
	select {
	case err := <-deleteDone:
		t.Fatalf("deletion crossed the in-flight Workbench linker: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(linker.ensureRelease)
	require.NoError(t, <-appDone)
	require.NoError(t, <-deleteDone)
	require.GreaterOrEqual(t, remover.calls, 1)
	var applications int64
	require.NoError(t, db.Table("career_applications").Count(&applications).Error)
	require.Zero(t, applications)
}

func (f *fakeCareerApplicationLinker) FindCareerApplicationTask(
	_ context.Context, tenantID uint64, ownerID string, requestID string,
) (interfaces.CareerApplicationTaskLink, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.findCalls++
	f.findArguments = append(f.findArguments, findArgument{tenantID: tenantID, ownerID: ownerID, requestID: requestID})
	if f.findErr != nil {
		return interfaces.CareerApplicationTaskLink{}, f.findErr
	}
	if f.findLink.TaskID != "" {
		return f.findLink, nil
	}
	return interfaces.CareerApplicationTaskLink{TaskID: "task-found", RunID: "run-found"}, nil
}

func (f *fakeCareerApplicationLinker) calls() (int, int) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.ensureCalls, f.findCalls
}

type storedApplicationRow struct {
	ID                         string
	RequestID                  string
	Fingerprint                string
	OpportunityID              string
	SnapshotID                 string
	EvaluationID               string
	ProfileRevision            uint64
	BatchIdentity              string
	EvidenceBody               string
	ContinueDespiteHardFailure bool
	EvaluationStatus           string
	Qualified                  bool
	WarningBody                string
	LinkState                  string
	TaskID                     string
	RunID                      string
	ReceiptBody                string
}

func readApplicationRows(t *testing.T, db *gorm.DB) []storedApplicationRow {
	t.Helper()
	var rows []storedApplicationRow
	require.NoError(t, db.Table("career_applications").
		Order("created_at ASC, id ASC").
		Find(&rows).Error)
	return rows
}

func TestCreateApplicationPinsEvidenceAndLinksTask(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1701)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "pins")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	input := applicationInput(seed, "pins-1", "batch-2027-a")
	receipt, err := o.CreateApplication(ctx, input)
	require.NoError(t, err)
	parsed, err := uuid.Parse(receipt.ApplicationID)
	require.NoError(t, err, "application id must be a UUID")
	require.Equal(t, parsed.String(), receipt.ApplicationID)
	require.Equal(t, "pins-1", receipt.RequestID)
	require.Equal(t, "ready", receipt.LinkState)
	require.Equal(t, seed.OpportunityID, receipt.PinnedEvidence.OpportunityID)
	require.Equal(t, seed.SnapshotID, receipt.PinnedEvidence.SnapshotID)
	require.Equal(t, seed.EvaluationID, receipt.PinnedEvidence.EvaluationID)
	require.Equal(t, seed.Revision, receipt.PinnedEvidence.ProfileRevision)
	require.Equal(t, EvaluationEligible, receipt.PinnedEvidence.EvaluationStatus)
	require.Equal(t, "batch-2027-a", receipt.PinnedEvidence.BatchIdentity)
	require.True(t, receipt.Qualified)
	require.Nil(t, receipt.Warning)
	require.NotEmpty(t, receipt.TaskID)
	require.NotEmpty(t, receipt.RunID)

	ensureCalls, findCalls := linker.calls()
	require.Equal(t, 1, ensureCalls)
	require.Zero(t, findCalls)
	require.Equal(t, "pins-1", linker.lastIntent.RequestID)
	require.Equal(t, receipt.ApplicationID, linker.lastIntent.ApplicationID)
	require.Equal(t, "Career application "+seed.OpportunityID, linker.lastIntent.Title)
	require.Equal(t, ensureScope{tenantID: 1701, ownerID: "owner-1"}, linker.ensureScopes[0])

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, receipt.ApplicationID, rows[0].ID)
	require.Equal(t, "pins-1", rows[0].RequestID)
	require.Len(t, rows[0].Fingerprint, 64)
	require.Equal(t, seed.EvaluationID, rows[0].EvaluationID)
	require.Equal(t, seed.Revision, rows[0].ProfileRevision)
	require.Equal(t, "batch-2027-a", rows[0].BatchIdentity)
	require.Equal(t, EvaluationEligible, rows[0].EvaluationStatus)
	require.True(t, rows[0].Qualified)
	require.Empty(t, rows[0].WarningBody)
	require.Equal(t, "ready", rows[0].LinkState)
	require.Equal(t, receipt.TaskID, rows[0].TaskID)
	require.Equal(t, receipt.RunID, rows[0].RunID)
	require.NotEmpty(t, rows[0].EvidenceBody)
	require.Contains(t, rows[0].ReceiptBody, `"linkState":"ready"`)

	encoded, err := json.Marshal(receipt)
	require.NoError(t, err)
	payload := string(encoded)
	require.Contains(t, payload, `"requestId":"pins-1"`)
	require.Contains(t, payload, `"applicationId":"`+receipt.ApplicationID+`"`)
	require.Contains(t, payload, `"linkState":"ready"`)
	require.Contains(t, payload, `"taskId":"`+receipt.TaskID+`"`)
	require.Contains(t, payload, `"qualified":true`)
	require.Contains(t, payload, `"pinnedEvidence":{`)

	stored, err := o.Application(ctx, receipt.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, receipt, stored)
}

func TestCreateApplicationHardFailureRequiresExplicitContinue(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1702)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2026", "hard")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	input := applicationInput(seed, "hard-1", "batch-2027-a")
	_, err := o.CreateApplication(ctx, input)
	require.ErrorIs(t, err, ErrApplicationHardIneligible)
	require.Empty(t, readApplicationRows(t, db))
	ensureCalls, _ := linker.calls()
	require.Zero(t, ensureCalls)

	continued := input
	continued.RequestID = "hard-2"
	continued.ContinueDespiteHardFailure = true
	receipt, err := o.CreateApplication(ctx, continued)
	require.NoError(t, err)
	require.False(t, receipt.Qualified)
	require.NotNil(t, receipt.Warning)
	require.Equal(t, "graduation_year", receipt.Warning["hardRuleId"])
	require.Equal(t, "graduation_year_mismatch", receipt.Warning["reasonCode"])
	require.Equal(t, seed.EvaluationID, receipt.Warning["evaluationId"])
	require.Equal(t, EvaluationIneligible, receipt.PinnedEvidence.EvaluationStatus)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Qualified)
	require.Equal(t, EvaluationIneligible, rows[0].EvaluationStatus)
	require.True(t, rows[0].ContinueDespiteHardFailure)
	require.Contains(t, rows[0].WarningBody, "graduation_year_mismatch")
	require.Equal(t, "ready", rows[0].LinkState)

	// The persisted hard warning and non-qualified metric are immutable:
	// an exact replay returns the identical receipt.
	replayed, err := o.CreateApplication(ctx, continued)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Len(t, readApplicationRows(t, db), 1)
	ensureCalls, _ = linker.calls()
	require.Equal(t, 1, ensureCalls)
}

func TestCreateApplicationExactReplayReturnsOriginalReceipt(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1703)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "replay")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	input := applicationInput(seed, "replay-1", "batch-2027-a")
	first, err := o.CreateApplication(ctx, input)
	require.NoError(t, err)

	// The profile moves forward after the original create; an exact replay
	// still returns the stored receipt without new side effects.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "replay-2", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)

	second, err := o.CreateApplication(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, seed.Revision, second.PinnedEvidence.ProfileRevision)

	ensureCalls, _ := linker.calls()
	require.Equal(t, 1, ensureCalls, "exact replay must not call the linker again")
	require.Len(t, readApplicationRows(t, db), 1)

	receipt, err := o.FindApplicationReceipt(ctx, "replay-1")
	require.NoError(t, err)
	require.Equal(t, first, receipt)
}

func TestCreateApplicationSameRequestChangedIntentConflicts(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1704)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "intent")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	first, err := o.CreateApplication(ctx, applicationInput(seed, "intent-1", "batch-2027-a"))
	require.NoError(t, err)
	require.Equal(t, "ready", first.LinkState)

	changed := applicationInput(seed, "intent-1", "batch-2027-b")
	_, err = o.CreateApplication(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, "batch-2027-a", rows[0].BatchIdentity)
	ensureCalls, _ := linker.calls()
	require.Equal(t, 1, ensureCalls)
}

func TestCreateApplicationSameJobAndBatchHasOneApplicationAndTask(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1705)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "unique")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	first, err := o.CreateApplication(ctx, applicationInput(seed, "unique-1", "batch-2027-a"))
	require.NoError(t, err)

	_, err = o.CreateApplication(ctx, applicationInput(seed, "unique-2", "batch-2027-a"))
	require.ErrorIs(t, err, ErrApplicationConflict)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, first.ApplicationID, rows[0].ID)
	ensureCalls, _ := linker.calls()
	require.Equal(t, 1, ensureCalls, "a losing second request must never reach the linker")
}

func TestCreateApplicationDistinctBatchesCreateDistinctTasks(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1706)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "batch")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	first, err := o.CreateApplication(ctx, applicationInput(seed, "batch-1", "batch-2027-a"))
	require.NoError(t, err)
	second, err := o.CreateApplication(ctx, applicationInput(seed, "batch-2", "batch-2026-b"))
	require.NoError(t, err)

	require.NotEqual(t, first.ApplicationID, second.ApplicationID)
	require.NotEqual(t, first.TaskID, second.TaskID)
	require.Equal(t, first.RunID, "run-1")
	require.Equal(t, second.RunID, "run-2")

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 2)
	require.Equal(t, "ready", rows[0].LinkState)
	require.Equal(t, "ready", rows[1].LinkState)
	ensureCalls, _ := linker.calls()
	require.Equal(t, 2, ensureCalls)
}

func TestCreateApplicationLinkerUnknownLeavesLinkingAndReconciles(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1707)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "unknown")
	linker := &fakeCareerApplicationLinker{ensureErr: context.Canceled}
	o.SetApplicationTaskLinker(linker)

	input := applicationInput(seed, "unknown-1", "batch-2027-a")
	receipt, err := o.CreateApplication(ctx, input)
	require.Error(t, err)
	require.Empty(t, receipt.ApplicationID)
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown)
	require.Equal(t, "unknown-1", unknown.RequestID)
	require.ErrorIs(t, err, ErrOutcomeUnknown)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	applicationID := rows[0].ID
	require.Equal(t, "linking", rows[0].LinkState)
	require.Empty(t, rows[0].TaskID)
	require.Equal(t, "unknown-1", rows[0].RequestID)

	// Reconciliation reuses the original request id and the same row.
	linker.ensureErr = nil
	reconciled, err := o.ReconcileApplicationLink(ctx, "unknown-1")
	require.NoError(t, err)
	require.Equal(t, applicationID, reconciled.ApplicationID)
	require.Equal(t, "ready", reconciled.LinkState)
	require.Equal(t, "task-found", reconciled.TaskID)
	require.Equal(t, seed.OpportunityID, reconciled.PinnedEvidence.OpportunityID)
	require.Equal(t, "batch-2027-a", reconciled.PinnedEvidence.BatchIdentity)

	require.Len(t, linker.findArguments, 1)
	require.Equal(t, findArgument{tenantID: 1707, ownerID: "owner-1", requestID: "unknown-1"}, linker.findArguments[0])
	ensureCalls, _ := linker.calls()
	require.Equal(t, 1, ensureCalls, "reconcile must consult Find, never mint a new request")

	rows = readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, applicationID, rows[0].ID)
	require.Equal(t, "ready", rows[0].LinkState)
	require.Equal(t, "task-found", rows[0].TaskID)

	stored, err := o.Application(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, reconciled, stored)
}

func TestCreateApplicationCareerLinkUpdateFailureReconcilesSameTask(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1708)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "update")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	// The Workbench projection exists, but Career's own ready update fails:
	// the application must stay linking and later reconcile to the SAME task.
	o.failApplicationReadyUpdate = func() error { return errors.New("career update failed") }
	input := applicationInput(seed, "update-1", "batch-2027-a")
	_, err := o.CreateApplication(ctx, input)
	require.ErrorIs(t, err, ErrOutcomeUnknown)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	applicationID := rows[0].ID
	require.Equal(t, "linking", rows[0].LinkState)
	ensureCalls, _ := linker.calls()
	require.Equal(t, 1, ensureCalls)

	o.failApplicationReadyUpdate = nil
	linker.findLink = interfaces.CareerApplicationTaskLink{TaskID: "task-1", RunID: "run-1"}
	reconciled, err := o.ReconcileApplicationLink(ctx, "update-1")
	require.NoError(t, err)
	require.Equal(t, applicationID, reconciled.ApplicationID)
	require.Equal(t, "ready", reconciled.LinkState)
	require.Equal(t, "task-1", reconciled.TaskID)
	require.Equal(t, "run-1", reconciled.RunID)

	rows = readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, applicationID, rows[0].ID)
	require.Equal(t, "task-1", rows[0].TaskID)
}

func TestApplicationScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-a", 1709)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "scope")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	receipt, err := o.CreateApplication(ctx, applicationInput(seed, "scope-1", "batch-2027-a"))
	require.NoError(t, err)

	sameTenantOtherOwner := WithScope(context.Background(), Scope{UserID: "owner-b", TenantID: 1709})
	// A career workspace admits one owner per tenant, so a second owner is
	// rejected at the scope gate before any application data is touched.
	for name, probe := range map[string]func() error{
		"application by id":  func() error { _, e := o.Application(sameTenantOtherOwner, receipt.ApplicationID); return e },
		"receipt by request": func() error { _, e := o.FindApplicationReceipt(sameTenantOtherOwner, "scope-1"); return e },
		"link reconcile":     func() error { _, e := o.ReconcileApplicationLink(sameTenantOtherOwner, "scope-1"); return e },
		"create application": func() error {
			_, e := o.CreateApplication(sameTenantOtherOwner, applicationInput(seed, "scope-2", "batch-2027-a"))
			return e
		},
	} {
		require.ErrorIs(t, probe(), ErrUnauthorized, name)
	}

	// The personal workspace is also unique per owner, so the cross-tenant
	// probe needs its own owner: an owned scope that simply holds no rows of
	// the first owner's applications.
	otherTenant := WithScope(context.Background(), Scope{UserID: "owner-z", TenantID: 1710})
	require.NoError(t, o.ClaimSpace(otherTenant))
	_, err = o.Application(otherTenant, receipt.ApplicationID)
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.FindApplicationReceipt(otherTenant, "scope-1")
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.ReconcileApplicationLink(otherTenant, "scope-1")
	require.ErrorIs(t, err, ErrApplicationNotFound)

	// Foreign evidence references cannot be pinned into another scope either.
	_, err = o.CreateApplication(otherTenant, applicationInput(seed, "scope-3", "batch-2027-a"))
	require.ErrorIs(t, err, ErrOpportunityNotFound)

	require.Len(t, readApplicationRows(t, db), 1)
	ensureCalls, findCalls := linker.calls()
	require.Equal(t, 1, ensureCalls)
	require.Zero(t, findCalls)
}

func TestApplicationRevisionConflictDoesNotCreateSideEffects(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1711)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "revision")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	input := applicationInput(seed, "revision-1", "batch-2027-a")
	input.ExpectedRevision = seed.Revision + 5
	_, err := o.CreateApplication(ctx, input)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, seed.Revision, conflict.CurrentRevision)

	require.Empty(t, readApplicationRows(t, db))
	ensureCalls, _ := linker.calls()
	require.Zero(t, ensureCalls)

	var evaluations, snapshots, observations int64
	require.NoError(t, db.Table("career_evaluations").Count(&evaluations).Error)
	require.NoError(t, db.Table("career_opportunity_snapshots").Count(&snapshots).Error)
	require.NoError(t, db.Table("career_opportunity_observations").Count(&observations).Error)
	require.Equal(t, int64(1), evaluations)
	require.Equal(t, int64(1), snapshots)
	require.Equal(t, int64(1), observations)
}

// ---- OCR round 1 fixes ---------------------------------------------------

// TestCreateApplicationRejectsRequestIDsBeyondWorkbenchWidth pins the
// alignment with the durable Workbench projection: request IDs of 65..128
// characters can never be linked (the projection columns are VARCHAR(64)),
// so they are rejected as invalid requests before any row is committed.
func TestCreateApplicationRejectsRequestIDsBeyondWorkbenchWidth(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1712)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "width")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	longRequest := strings.Repeat("r", maxApplicationRequestIDLen+1)
	_, err := o.CreateApplication(ctx, applicationInput(seed, longRequest, "batch-2027-a"))
	require.ErrorIs(t, err, ErrInvalidRequest)
	require.Empty(t, readApplicationRows(t, db))
	ensureCalls, _ := linker.calls()
	require.Zero(t, ensureCalls, "the linker is never reached with an unrepresentable request ID")

	// A 64-character request ID is the exact durable ceiling and must work.
	fitted := strings.Repeat("a", maxApplicationRequestIDLen)
	receipt, err := o.CreateApplication(ctx, applicationInput(seed, fitted, "batch-2027-a"))
	require.NoError(t, err)
	require.Equal(t, ApplicationLinkStateReady, receipt.LinkState)
}

// TestCreateApplicationUndecidedLinkerKeepsLinkingState pins the undecided
// outcome: when the linker exhausts its race budget without a durable task,
// Career keeps link_state=linking (a later replay reconciles) instead of
// terminally failing a request whose task may already be ready.
func TestCreateApplicationUndecidedLinkerKeepsLinkingState(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1713)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "undecided")
	linker := &fakeCareerApplicationLinker{ensureErr: interfaces.ErrCareerApplicationTaskUndecided}
	o.SetApplicationTaskLinker(linker)

	_, err := o.CreateApplication(ctx, applicationInput(seed, "undecided-1", "batch-2027-a"))
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	stored, err := o.FindApplicationReceipt(ctx, "undecided-1")
	require.NoError(t, err)
	require.Equal(t, ApplicationLinkStateLinking, stored.LinkState, "an undecided link stays linking for reconciliation")
	require.NotErrorIs(t, err, ErrApplicationConflict)
}

// TestCreateApplicationSQLiteBusyConvergesToOutcomeUnknown pins the busy
// branch: when concurrent creation hits SQLite writer-lock contention, the
// replay/occupied lookups see nothing (the transaction rolled back) and the
// caller must get the typed outcome_unknown — never a raw "database is
// locked" 500 — and the same request ID stays safely retryable.
func TestCreateApplicationSQLiteBusyConvergesToOutcomeUnknown(t *testing.T) {
	dsn := fmt.Sprintf("file:career-app-busy-%s?mode=memory&cache=shared&_busy_timeout=150", strings.ReplaceAll(uuid.NewString(), "-", ""))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	o, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: "owner-1", TenantID: 1714})
	require.NoError(t, o.ClaimSpace(ctx))
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "busy")
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})

	// Hold the SQLite writer reservation on a dedicated pooled connection so
	// the application transaction hits SQLITE_BUSY after the short timeout.
	blocker, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	_, err = blocker.ExecContext(ctx, "BEGIN IMMEDIATE")
	require.NoError(t, err)

	_, err = o.CreateApplication(ctx, applicationInput(seed, "busy-1", "batch-2027-a"))
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown, "lock contention must converge to the typed unknown outcome, got: %v", err)
	require.Equal(t, "busy-1", unknown.RequestID)

	// Release the writer: the same request ID retries cleanly to a linked
	// application — the busy branch never left partial state behind.
	_, err = blocker.ExecContext(ctx, "ROLLBACK")
	require.NoError(t, err)
	require.NoError(t, blocker.Close())

	receipt, err := o.CreateApplication(ctx, applicationInput(seed, "busy-1", "batch-2027-a"))
	require.NoError(t, err)
	require.Equal(t, ApplicationLinkStateReady, receipt.LinkState)
	require.Len(t, readApplicationRows(t, db), 1)
}

// ---- OCR round 2 fixes ---------------------------------------------------

// TestReconcileApplicationLinkKeepsFailedStateTerminal pins ocr2-019:
// link_failed is a definite rejection — reconcile must never re-open it, and
// never flip the row onto a twin's durable task.
func TestReconcileApplicationLinkKeepsFailedStateTerminal(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1715)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "terminal")
	linker := &fakeCareerApplicationLinker{ensureErr: interfaces.ErrCareerApplicationTaskConflict}
	o.SetApplicationTaskLinker(linker)

	_, err := o.CreateApplication(ctx, applicationInput(seed, "terminal-1", "batch-2027-a"))
	require.ErrorIs(t, err, ErrApplicationConflict)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	applicationID := rows[0].ID
	require.Equal(t, ApplicationLinkStateFailed, rows[0].LinkState)

	// A durable task now exists under this request ID (the winning twin's).
	// Reconcile must return the stored terminal receipt without flipping.
	linker.ensureErr = nil
	linker.findLink = interfaces.CareerApplicationTaskLink{
		TaskID: "twin-task", RunID: "twin-run", ApplicationID: uuid.NewString(),
	}
	reconciled, err := o.ReconcileApplicationLink(ctx, "terminal-1")
	require.NoError(t, err)
	require.Equal(t, applicationID, reconciled.ApplicationID)
	require.Equal(t, ApplicationLinkStateFailed, reconciled.LinkState)
	require.Empty(t, reconciled.TaskID)

	_, findCalls := linker.calls()
	require.Zero(t, findCalls, "a terminal link_failed row must not consult Find")

	rows = readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, ApplicationLinkStateFailed, rows[0].LinkState)
	require.Empty(t, rows[0].TaskID)
}

// TestReconcileApplicationLinkRejectsForeignOwnedTask pins the ownership half
// of ocr2-019: a Find hit that belongs to a different application is a
// definite rejection for this row — never a ready flip with the foreign task.
func TestReconcileApplicationLinkRejectsForeignOwnedTask(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1716)
	seed := seedApplicationEvaluation(t, o, ctx, "仅限2027届。", "2027", "foreign")
	linker := &fakeCareerApplicationLinker{ensureErr: context.Canceled}
	o.SetApplicationTaskLinker(linker)

	_, err := o.CreateApplication(ctx, applicationInput(seed, "foreign-1", "batch-2027-a"))
	require.ErrorIs(t, err, ErrOutcomeUnknown)

	rows := readApplicationRows(t, db)
	require.Len(t, rows, 1)
	applicationID := rows[0].ID
	require.Equal(t, ApplicationLinkStateLinking, rows[0].LinkState)

	// The request ID resolved to a different application's durable task.
	linker.ensureErr = nil
	linker.findLink = interfaces.CareerApplicationTaskLink{
		TaskID: "twin-task", RunID: "twin-run", ApplicationID: uuid.NewString(),
	}
	_, err = o.ReconcileApplicationLink(ctx, "foreign-1")
	require.ErrorIs(t, err, ErrApplicationConflict)

	rows = readApplicationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, applicationID, rows[0].ID)
	require.Equal(t, ApplicationLinkStateFailed, rows[0].LinkState, "foreign ownership is a definite rejection")
	require.Empty(t, rows[0].TaskID)
}

// TestCreateApplicationReplayAfterMergeReusesCanonicalTitle pins ocr2-144: a
// retry of an undecided link must re-enter the linker with the canonical
// opportunity title the first attempt pinned, not the raw input ID that a
// later merge resolved away — otherwise the Workbench replay check would
// terminally reject a legal recovery.
func TestCreateApplicationReplayAfterMergeReusesCanonicalTitle(t *testing.T) {
	o, _, ctx := newApplicationOffice(t, "owner-1", 1717)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "education.graduation_year", "2027", "app-merge-year", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)

	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "app-merge-1", "https://jobs.example.com/postings/2001", fields)
	candidate := importKnownJD(t, o, ctx, "app-merge-2", "https://other.example.net/jobs/2001", fields)
	merged, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "app-merge-req", TargetID: target.OpportunityID, CandidateID: candidate.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, merged.Decision)

	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID:     "app-merge-eval",
		OpportunityID: target.OpportunityID,
		SnapshotID:    target.SnapshotID,
	})
	require.NoError(t, err)

	linker := &fakeCareerApplicationLinker{ensureErr: context.Canceled}
	o.SetApplicationTaskLinker(linker)
	// The caller keeps using the pre-merge candidate reference; the pinned
	// intent resolves onto the canonical target.
	input := CreateApplicationInput{
		RequestID:        "app-merge-app",
		OpportunityID:    candidate.OpportunityID,
		SnapshotID:       target.SnapshotID,
		EvaluationID:     evaluation.EvaluationID,
		BatchIdentity:    "batch-2027-a",
		ExpectedRevision: view.Revision,
	}
	_, err = o.CreateApplication(ctx, input)
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	require.Equal(t, applicationTaskTitle(target.OpportunityID), linker.lastIntent.Title)

	// The retry replays the stored intent and must reuse the canonical title
	// so the Workbench projection recognizes its own request.
	linker.ensureErr = nil
	receipt, err := o.CreateApplication(ctx, input)
	require.NoError(t, err)
	require.Equal(t, ApplicationLinkStateReady, receipt.LinkState)
	require.Equal(t, applicationTaskTitle(target.OpportunityID), linker.lastIntent.Title)
	require.Equal(t, target.OpportunityID, receipt.PinnedEvidence.OpportunityID)
}
