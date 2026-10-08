package career

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- shared reminder fixtures -------------------------------------------------

// sensitiveProgressNote carries every class of detail a push body must never
// leak: company, job title, and interview specifics.
const sensitiveProgressNote = "Acme 云笔记 高级后端工程师 三轮技术面 10月1日 14:00，面试官是 Go 团队负责人"

type recordedReminderPush struct {
	UserID   string
	TenantID uint64
	Notice   ReminderNotice
}

// recordingReminderNotifier is the push seam fake: it records every call and
// can be switched to failing to exercise delivery-failure semantics.
type recordingReminderNotifier struct {
	mu    sync.Mutex
	calls []recordedReminderPush
	fail  bool
}

func (r *recordingReminderNotifier) Remind(_ context.Context, s Scope, notice ReminderNotice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedReminderPush{UserID: s.UserID, TenantID: s.TenantID, Notice: notice})
	if r.fail {
		return errors.New("push channel down")
	}
	return nil
}

func (r *recordingReminderNotifier) recorded() []recordedReminderPush {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedReminderPush, len(r.calls))
	copy(out, r.calls)
	return out
}

func reminderHead(t *testing.T, o *Office, ctx context.Context) uint64 {
	t.Helper()
	view, err := o.Open(ctx)
	require.NoError(t, err)
	return view.Revision
}

func reminderInput(requestID, sourceKind, sourceID string, revision uint64) SetReminderInput {
	return SetReminderInput{RequestID: requestID, SourceKind: sourceKind, SourceID: sourceID, ExpectedRevision: revision}
}

// seedReminderEvent creates one application with one progress event whose
// note deliberately carries company, job, and interview detail.
func seedReminderEvent(t *testing.T, o *Office, ctx context.Context, seedID string) (applicationID, eventID string) {
	t.Helper()
	applicationID = seedProgressApplication(t, o, ctx, completeJDText, "2027", seedID, seedID+"-batch")
	head := reminderHead(t, o, ctx)
	event, err := o.AppendProgress(ctx, appendProgressInput(applicationID, seedID+"-evt", ProgressEventInterview, sensitiveProgressNote, 0))
	require.NoError(t, err)
	require.Equal(t, head, reminderHead(t, o, ctx), "progress events do not move the profile head")
	return applicationID, event.EventID
}

func countReminderRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where("tenant_id = ? AND user_id = ?", uint64(921), "reminder-owner").Count(&count).Error)
	return count
}

// ---- 1. one todo per opportunity event ---------------------------------------

func TestSameOpportunityAndEventProducesSingleTodo(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	applicationID, eventID := seedReminderEvent(t, o, ctx, "dedupe")
	head := reminderHead(t, o, ctx)

	// The same source event reminded through two different request IDs still
	// yields exactly one todo — the deterministic (scope, source) key rules.
	first, err := o.SetReminder(ctx, reminderInput("dedupe-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.NotEmpty(t, first.ReminderID)
	require.False(t, first.Deduplicated)
	second, err := o.SetReminder(ctx, reminderInput("dedupe-r2", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.Equal(t, first.ReminderID, second.ReminderID, "repeat triggering must return the same todo")
	require.True(t, second.Deduplicated)
	require.EqualValues(t, 1, countReminderRows(t, db, "career_reminders"))

	// Exact request replay returns the stored receipt, not a dedupe hit. The
	// stored receipt never carries the ephemeral push report.
	replay, err := o.SetReminder(ctx, reminderInput("dedupe-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.Equal(t, first.Kind, replay.Kind)
	require.Equal(t, first.RequestID, replay.RequestID)
	require.Equal(t, first.ReminderID, replay.ReminderID)
	require.Equal(t, first.Deduplicated, replay.Deduplicated)
	require.Nil(t, replay.Push)

	// A second event on the same application (same opportunity) is a distinct
	// fact and holds its own todo.
	next, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "dedupe-evt2", ProgressEventOffer, "Offer 到了", 1))
	require.NoError(t, err)
	third, err := o.SetReminder(ctx, reminderInput("dedupe-r3", ReminderSourceProgressEvent, next.EventID, head))
	require.NoError(t, err)
	require.NotEqual(t, first.ReminderID, third.ReminderID)
	require.EqualValues(t, 2, countReminderRows(t, db, "career_reminders"))

	// The durable row keeps the event binding and the frozen notice identity.
	var row struct {
		SourceKind    string
		SourceID      string
		ApplicationID string
		OpportunityID string
		NoticeKey     string
		Status        string
	}
	require.NoError(t, db.Table("career_reminders").Where("id = ?", first.ReminderID).Scan(&row).Error)
	require.Equal(t, ReminderSourceProgressEvent, row.SourceKind)
	require.Equal(t, eventID, row.SourceID)
	require.Equal(t, applicationID, row.ApplicationID)
	require.NotEmpty(t, row.OpportunityID)
	require.Equal(t, ReminderNoticeProgressUpdated, row.NoticeKey)
	require.Equal(t, ReminderStatusOpen, row.Status)
}

func TestConcurrentSetReminderDifferentRequestIDsConvergeOnOneTodo(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "concurrent-dedupe")
	head := reminderHead(t, o, ctx)
	start := make(chan struct{})
	type result struct {
		receipt ReminderReceipt
		err     error
	}
	results := make(chan result, 2)
	for _, requestID := range []string{"concurrent-r1", "concurrent-r2"} {
		go func(id string) {
			<-start
			receipt, err := o.SetReminder(ctx, reminderInput(id, ReminderSourceProgressEvent, eventID, head))
			results <- result{receipt: receipt, err: err}
		}(requestID)
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotEmpty(t, first.receipt.ReminderID)
	require.Equal(t, first.receipt.ReminderID, second.receipt.ReminderID)
	require.True(t, first.receipt.Deduplicated != second.receipt.Deduplicated)
	require.EqualValues(t, 1, countReminderRows(t, db, "career_reminders"))
	require.EqualValues(t, 2, countReminderRows(t, db, "career_reminder_receipts"))
}

func TestReconcileReminderSourceDoesNotRecreateReceiptAfterDeletion(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "reconcile-delete")
	head := reminderHead(t, o, ctx)
	_, err := o.SetReminder(ctx, reminderInput("reconcile-delete-original", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)

	_, err = o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "reconcile-delete-space", ExpectedRevision: head})
	require.NoError(t, err)

	receipt, found, err := o.reconcileReminderSource(ctx, Scope{TenantID: 921, UserID: "reminder-owner"},
		reminderInput("reconcile-delete-late", ReminderSourceProgressEvent, eventID, head), strings.Repeat("a", 64))
	// H7: a deleted space fails closed. The silent no-recreate outcome kept
	// the zero-row guarantee; the typed error adds the honest refusal.
	require.ErrorIs(t, err, ErrCareerDeleting)
	require.False(t, found)
	require.Empty(t, receipt.ReminderID)
	require.Zero(t, countReminderRows(t, db, "career_reminders"))
	require.Zero(t, countReminderRows(t, db, "career_reminder_receipts"))
}

func TestDeleteFinalizingDuringReminderReconciliationLeavesNoRows(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	_, eventID := seedReminderEvent(t, o, ctx, "reconcile-barrier")
	head := reminderHead(t, o, ctx)
	_, err := o.SetReminder(ctx, reminderInput("reconcile-barrier-original", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)

	sourceRead := make(chan struct{})
	continueReconcile := make(chan struct{})
	const callback = "test:pause-reminder-reconcile-after-source-read"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "career_reminders" || !strings.Contains(tx.Statement.SQL.String(), "source_kind") {
			return
		}
		select {
		case <-sourceRead:
			return
		default:
			close(sourceRead)
		}
		<-continueReconcile
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })

	type reconcileResult struct {
		found bool
		err   error
	}
	reconcileDone := make(chan reconcileResult, 1)
	go func() {
		_, found, err := o.reconcileReminderSource(ctx, Scope{TenantID: 921, UserID: "reminder-owner"},
			reminderInput("reconcile-barrier-late", ReminderSourceProgressEvent, eventID, head), strings.Repeat("b", 64))
		reconcileDone <- reconcileResult{found: found, err: err}
	}()
	select {
	case <-sourceRead:
	case <-time.After(5 * time.Second):
		t.Fatal("reconciliation did not reach its source-read barrier")
	}

	type deleteResult struct {
		status string
		err    error
	}
	deleteDone := make(chan deleteResult, 1)
	go func() {
		deletion, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "reconcile-barrier-delete", ExpectedRevision: head})
		deleteDone <- deleteResult{status: deletion.Status, err: err}
	}()
	firstDeleteStatus := DeletionStatusPartial
	select {
	case result := <-deleteDone:
		require.NoError(t, result.err)
		require.Contains(t, []string{DeletionStatusPartial, DeletionStatusDeleted}, result.status)
		firstDeleteStatus = result.status
	case <-time.After(5 * time.Second):
		close(continueReconcile)
		t.Fatal("deletion did not finalize while reconciliation was paused")
	}
	close(continueReconcile)
	select {
	case result := <-reconcileDone:
		// SQLite may reject the stale read transaction's later write with
		// SQLITE_BUSY; the invariant under test is that no rows survive delete.
		if result.err == nil {
			require.False(t, result.found)
		} else {
			require.True(t, isSQLiteBusy(result.err), "unexpected reconciliation error: %v", result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reconciliation did not leave its source-read barrier")
	}
	if firstDeleteStatus != DeletionStatusDeleted {
		var lastDeletion CareerDeletionReceipt
		for attempt := 0; attempt < 3 && firstDeleteStatus != DeletionStatusDeleted; attempt++ {
			deletion, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "reconcile-barrier-delete", ExpectedRevision: head})
			require.NoError(t, err)
			lastDeletion = deletion
			firstDeleteStatus = deletion.Status
		}
		require.Equal(t, DeletionStatusDeleted, firstDeleteStatus, "%+v", lastDeletion.Steps)
	}
	require.Zero(t, countReminderRows(t, db, "career_reminders"))
	require.Zero(t, countReminderRows(t, db, "career_reminder_receipts"))
}

// ---- 2. the inbox row is authoritative; push only reminds ---------------------

func TestInboxRecordIsAuthoritativeAndPushOnlyReminds(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "authority")
	head := reminderHead(t, o, ctx)
	notifier := &recordingReminderNotifier{}
	o.SetReminderNotifier(notifier)

	receipt, err := o.SetReminder(ctx, reminderInput("authority-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)

	// Push happened exactly once, addressed to the authenticated scope, and
	// carried nothing but the frozen notice.
	calls := notifier.recorded()
	require.Len(t, calls, 1)
	require.Equal(t, "reminder-owner", calls[0].UserID)
	require.EqualValues(t, 921, calls[0].TenantID)
	require.Equal(t, ReminderNoticeProgressUpdated, calls[0].Notice.TemplateKey)
	require.Equal(t, ReminderNoticeBodies[ReminderNoticeProgressUpdated], calls[0].Notice.Body)
	require.NotNil(t, receipt.Push)
	require.True(t, receipt.Push.Attempted)
	require.True(t, receipt.Push.Delivered)

	// The durable inbox fact exists independent of the channel; the stored
	// receipt never carries push state (a push is not a status update).
	var stored struct{ Body string }
	require.NoError(t, db.Table("career_reminder_receipts").Where("request_id = ?", "authority-r1").Scan(&stored).Error)
	var durable ReminderReceipt
	require.NoError(t, json.Unmarshal([]byte(stored.Body), &durable))
	require.Nil(t, durable.Push, "the durable receipt must not record push outcome as todo state")
	require.Equal(t, receipt.ReminderID, durable.ReminderID)

	// The list projection is the authoritative reading surface.
	todos, err := o.ListReminders(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 1)
	require.Equal(t, receipt.ReminderID, todos[0].ReminderID)
	require.Equal(t, ReminderStatusOpen, todos[0].Status)
	require.Equal(t, ReminderNoticeBodies[ReminderNoticeProgressUpdated], todos[0].Notice)

	// Production runs without any push channel: the inbox fact alone remains
	// complete and readable.
	production, err := NewOffice(db)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO career_progress_events
		(id,tenant_id,user_id,application_id,seq,kind,event_type,note,occurred_at,corrects_event_id,source,confirmer,request_id,fingerprint,receipt_body,created_at)
		VALUES ('authority-evt2',921,'reminder-owner', (SELECT application_id FROM career_reminders WHERE id = ?),2,'progress_appended','offer','','2026-09-25 09:00:00','','{}','reminder-owner','authority-evt2-req',?,?,'2026-09-25 09:00:00')`,
		receipt.ReminderID, strings.Repeat("a", 64), "{}").Error)
	bare, err := production.SetReminder(ctx, reminderInput("authority-r2", ReminderSourceProgressEvent, "authority-evt2", head))
	require.NoError(t, err)
	require.Nil(t, bare.Push, "with no channel configured there is no push report")
	todos, err = o.ListReminders(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 2)
}

// ---- 3. privacy: no company, job, or interview detail -------------------------

func TestNotificationBodyContainsNoCompanyJobOrInterviewDetail(t *testing.T) {
	o, _, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "privacy")
	head := reminderHead(t, o, ctx)
	notifier := &recordingReminderNotifier{}
	o.SetReminderNotifier(notifier)

	receipt, err := o.SetReminder(ctx, reminderInput("privacy-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)

	leaks := []string{"Acme", "云笔记", "高级后端工程师", "三轮技术面", "10月1日", "14:00", "Go 团队"}
	pushed := notifier.recorded()[0].Notice.Body
	require.Equal(t, ReminderNoticeBodies[ReminderNoticeProgressUpdated], pushed)
	receiptJSON, err := json.Marshal(receipt)
	require.NoError(t, err)
	todos, err := o.ListReminders(ctx)
	require.NoError(t, err)
	listJSON, err := json.Marshal(todos)
	require.NoError(t, err)
	for _, surface := range []string{pushed, string(receiptJSON), string(listJSON)} {
		for _, leak := range leaks {
			require.NotContains(t, surface, leak, "surface must not leak %q", leak)
		}
	}

	// The discovery channel gets its own frozen template: the job link never
	// reaches the push body either.
	ruleOffice, _, _, ruleCtx := newSearchRuleOffice(t)
	_, err = ruleOffice.SetRule(ruleCtx, ruleInput("privacy-rule-1", 60, RuleStatusEnabled))
	require.NoError(t, err)
	_, err = ruleOffice.TriggerDueRules(ruleCtx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	var todo searchDiscoveryTodoRecord
	require.NoError(t, ruleOffice.db.Where("link = ?", "https://jobs.example.test/j/1001").First(&todo).Error)
	ruleNotifier := &recordingReminderNotifier{}
	ruleOffice.SetReminderNotifier(ruleNotifier)
	discoveryReceipt, err := ruleOffice.SetReminder(ruleCtx, reminderInput("privacy-disc-1", ReminderSourceDiscovery, todo.ID, 0))
	require.NoError(t, err)
	discoveryBody := ruleNotifier.recorded()[0].Notice.Body
	require.Equal(t, ReminderNoticeBodies[ReminderNoticeDiscoveryFound], discoveryBody)
	require.NotContains(t, discoveryBody, "jobs.example.test")
	require.NotContains(t, discoveryBody, "1001")
	discoveryJSON, err := json.Marshal(discoveryReceipt)
	require.NoError(t, err)
	require.NotContains(t, string(discoveryJSON), "jobs.example.test")
}

// ---- 4. unsubscribe stops push, todos stay readable ---------------------------

func TestUnsubscribeStopsPushButTodosRemainReadable(t *testing.T) {
	o, _, ctx := newProgressOffice(t, "reminder-owner", 921)
	applicationID, eventID := seedReminderEvent(t, o, ctx, "unsub")
	head := reminderHead(t, o, ctx)
	notifier := &recordingReminderNotifier{}
	o.SetReminderNotifier(notifier)

	before, err := o.SetReminder(ctx, reminderInput("unsub-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.Len(t, notifier.recorded(), 1)

	// Unsubscribe is a durable profile fact written through the house flow.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, ReminderPushFactKey, ReminderPushUnsubscribedValue, "unsub-set", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	// The confirm moved the profile head; the next reminder CASes against it.
	head = reminderHead(t, o, ctx)

	next, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "unsub-evt2", ProgressEventAssessment, "在线测评", 1))
	require.NoError(t, err)
	after, err := o.SetReminder(ctx, reminderInput("unsub-r2", ReminderSourceProgressEvent, next.EventID, head))
	require.NoError(t, err)
	require.Len(t, notifier.recorded(), 1, "no push after unsubscribe")
	require.NotNil(t, after.Push)
	require.False(t, after.Push.Attempted)
	require.Equal(t, ReminderPushReasonUnsubscribed, after.Push.Reason)

	// The in-station todo keeps being produced, and existing todos stay readable.
	todos, err := o.ListReminders(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 2)
	ids := map[string]bool{}
	for _, todo := range todos {
		ids[todo.ReminderID] = true
	}
	require.True(t, ids[before.ReminderID], "todos created before unsubscribing stay readable")
	require.True(t, ids[after.ReminderID], "todos created after unsubscribing still land in the inbox")
}

// ---- 5. push delivery failure never loses or mutates the inbox fact ----------

func TestPushDeliveryFailureKeepsInboxFactAndDoesNotMutateState(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "failpush")
	head := reminderHead(t, o, ctx)
	notifier := &recordingReminderNotifier{fail: true}
	o.SetReminderNotifier(notifier)

	receipt, err := o.SetReminder(ctx, reminderInput("failpush-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err, "a failed push must never fail the inbox write")
	require.NotNil(t, receipt.Push)
	require.True(t, receipt.Push.Attempted)
	require.False(t, receipt.Push.Delivered)
	require.Equal(t, ReminderPushReasonDeliveryFailed, receipt.Push.Reason)

	// The durable fact is intact and carries no push-mutated state.
	require.EqualValues(t, 1, countReminderRows(t, db, "career_reminders"))
	var row struct{ Status string }
	require.NoError(t, db.Table("career_reminders").Where("id = ?", receipt.ReminderID).Scan(&row).Error)
	require.Equal(t, ReminderStatusOpen, row.Status)
	todos, err := o.ListReminders(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 1)
	require.Equal(t, ReminderStatusOpen, todos[0].Status)

	// A repeat trigger is a dedupe hit and never retries the push.
	again, err := o.SetReminder(ctx, reminderInput("failpush-r2", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.True(t, again.Deduplicated)
	require.Nil(t, again.Push, "dedupe hits never push again")
	require.Len(t, notifier.recorded(), 1)
	require.EqualValues(t, 1, countReminderRows(t, db, "career_reminders"))
}

// ---- 6. house idempotency: exact replay vs changed intent --------------------

func TestSetReminderExactReplayAndChangedIntentConflict(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	applicationID, eventID := seedReminderEvent(t, o, ctx, "replay")
	head := reminderHead(t, o, ctx)

	first, err := o.SetReminder(ctx, reminderInput("replay-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	replayed, err := o.SetReminder(ctx, reminderInput("replay-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.Equal(t, first, replayed)

	next, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "replay-evt2", ProgressEventOffer, "", 1))
	require.NoError(t, err)
	_, err = o.SetReminder(ctx, reminderInput("replay-r1", ReminderSourceProgressEvent, next.EventID, head))
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	// Dedupe responses are durable too: the same request ID may not change
	// intent even when its first answer was a dedupe hit.
	_, err = o.SetReminder(ctx, reminderInput("replay-r2", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	_, err = o.SetReminder(ctx, reminderInput("replay-r2", ReminderSourceProgressEvent, next.EventID, head))
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	receipt, err := o.FindReminderReceipt(ctx, "replay-r1")
	require.NoError(t, err)
	require.Equal(t, first.ReminderID, receipt.ReminderID)
	require.EqualValues(t, 2, countReminderRows(t, db, "career_reminder_receipts"))
}

// ---- 7. scope isolation -------------------------------------------------------

func TestReminderScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, _, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "scope")
	head := reminderHead(t, o, ctx)
	mine, err := o.SetReminder(ctx, reminderInput("scope-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)

	// The career space is owner-only per tenant: a second user of the same
	// tenant cannot even claim the space, so every write is rejected.
	otherUser := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 921})
	require.ErrorIs(t, o.ClaimSpace(otherUser), ErrUnauthorized)
	_, err = o.SetReminder(otherUser, reminderInput("scope-foreign-1", ReminderSourceProgressEvent, eventID, 0))
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.ListReminders(otherUser)
	require.ErrorIs(t, err, ErrUnauthorized)

	// A different tenant with its own owner holds its own space, but the
	// first owner's event is simply not visible under that scope.
	otherTenant := WithScope(context.Background(), Scope{UserID: "other-owner", TenantID: 922})
	require.NoError(t, o.ClaimSpace(otherTenant))
	_, err = o.SetReminder(otherTenant, reminderInput("scope-foreign-2", ReminderSourceProgressEvent, eventID, 0))
	require.ErrorIs(t, err, ErrReminderSourceNotFound)

	// The foreign tenant reads only its own (empty) inbox and receipts.
	empty, err := o.ListReminders(otherTenant)
	require.NoError(t, err)
	require.Empty(t, empty)
	_, err = o.FindReminderReceipt(otherTenant, "scope-r1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// The owner's facts are untouched by the foreign attempts.
	todos, err := o.ListReminders(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 1)
	require.Equal(t, mine.ReminderID, todos[0].ReminderID)
}

// ---- 8. revision conflict -----------------------------------------------------

func TestReminderRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	_, eventID := seedReminderEvent(t, o, ctx, "revision")
	head := reminderHead(t, o, ctx)

	_, err := o.SetReminder(ctx, reminderInput("revision-r1", ReminderSourceProgressEvent, eventID, head+3))
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, head, conflict.CurrentRevision)
	require.EqualValues(t, 0, countReminderRows(t, db, "career_reminders"), "a conflicting write creates nothing")

	ok, err := o.SetReminder(ctx, reminderInput("revision-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	require.False(t, ok.Deduplicated)
	require.EqualValues(t, 1, countReminderRows(t, db, "career_reminders"))
}

// ---- 9. deletion purge and boundary disclosure --------------------------------

func TestReminderTableIncludedInDeletionPurgeAndBoundary(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "reminder-owner", 921)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	applicationID, eventID := seedReminderEvent(t, o, ctx, "purge")
	head := reminderHead(t, o, ctx)
	first, err := o.SetReminder(ctx, reminderInput("purge-r1", ReminderSourceProgressEvent, eventID, head))
	require.NoError(t, err)
	next, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "purge-evt2", ProgressEventOffer, "", 1))
	require.NoError(t, err)
	_, err = o.SetReminder(ctx, reminderInput("purge-r2", ReminderSourceProgressEvent, next.EventID, head))
	require.NoError(t, err)

	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	sections := map[string]CareerDeletionSection{}
	for _, section := range boundary.InSpace {
		sections[section.Section] = section
	}
	require.Contains(t, sections, "reminders")
	require.NotEmpty(t, sections["reminders"].Description)
	require.Equal(t, 2, sections["reminders"].Count)

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "purge-delete", ExpectedRevision: head})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	var todoRows, receiptRows int64
	require.NoError(t, db.Table("career_reminders").Where("tenant_id = ?", uint64(921)).Count(&todoRows).Error)
	require.NoError(t, db.Table("career_reminder_receipts").Where("tenant_id = ?", uint64(921)).Count(&receiptRows).Error)
	require.Zero(t, todoRows, "complete deletion must purge the inbox todos")
	require.Zero(t, receiptRows, "complete deletion must purge the reminder receipts")
	_ = first
}
