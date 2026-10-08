package career

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newProgressOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-progress.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, db, ctx
}

// progressDBPath returns the on-disk path of the office database so a test can
// reopen the identical file through a fresh connection.
func progressDBPath(t *testing.T, db *gorm.DB) string {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	rows, err := sqlDB.Query("PRAGMA database_list")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, path string
		require.NoError(t, rows.Scan(&seq, &name, &path))
		if name == "main" && path != "" {
			return path
		}
	}
	t.Fatal("progress test requires a file-backed sqlite database")
	return ""
}

// seedProgressApplication creates one durable application under the scope and
// returns its ID. Progress events bind to exactly this application.
func seedProgressApplication(t *testing.T, o *Office, ctx context.Context, jd, year, seedID, batch string) string {
	t.Helper()
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	seed := seedApplicationEvaluation(t, o, ctx, jd, year, seedID)
	receipt, err := o.CreateApplication(ctx, applicationInput(seed, seedID+"-app", batch))
	require.NoError(t, err)
	return receipt.ApplicationID
}

func progressMoment() time.Time {
	moment, err := time.Parse(time.RFC3339, "2026-09-20T10:00:00Z")
	if err != nil {
		panic(err)
	}
	return moment
}

func appendProgressInput(applicationID, requestID, eventType, note string, revision uint64) AppendProgressInput {
	moment := progressMoment()
	return AppendProgressInput{
		RequestID:        requestID,
		ApplicationID:    applicationID,
		EventType:        eventType,
		Note:             note,
		OccurredAt:       &moment,
		Source:           Source{Kind: "manual"},
		ExpectedRevision: revision,
	}
}

type storedProgressRow struct {
	ID              string
	TenantID        uint64
	UserID          string
	ApplicationID   string
	Seq             uint64
	Kind            string
	EventType       string
	Note            string
	OccurredAt      time.Time
	CorrectsEventID string
	Source          string
	Confirmer       string
	RequestID       string
	Fingerprint     string
	ReceiptBody     string
	CreatedAt       time.Time
}

func readProgressRows(t *testing.T, db *gorm.DB) []storedProgressRow {
	t.Helper()
	var rows []storedProgressRow
	require.NoError(t, db.Table("career_progress_events").Order("seq ASC").Find(&rows).Error)
	return rows
}

func TestGenericProgressRejectsSubmissionEvents(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1920)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "submission-binding", "batch-a")
	for _, eventType := range []string{ProgressEventSubmitted, ProgressEventResubmitted} {
		t.Run("append_"+eventType, func(t *testing.T) {
			_, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "append-"+eventType, eventType, "已投递", 0))
			require.ErrorIs(t, err, ErrInvalidRequest)
		})
		t.Run("correction_"+eventType, func(t *testing.T) {
			_, err := o.CorrectProgress(ctx, CorrectProgressInput{
				RequestID: "correct-" + eventType, ApplicationID: applicationID, CorrectsEventID: "00000000-0000-0000-0000-000000000000",
				EventType: eventType, Source: Source{Kind: "manual"}, ExpectedRevision: 0,
			})
			require.ErrorIs(t, err, ErrInvalidRequest)
		})
	}
	require.Empty(t, readProgressRows(t, db), "unbound submission claims must not persist")
}

func TestAppendProgressPersistsImmutableEventAndReplayIsIdempotent(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1901)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "imm", "batch-a")

	receipt, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "imm-1", ProgressEventPendingSubmission, "通过官网投递", 0))
	require.NoError(t, err)
	require.Equal(t, ProgressKindAppended, receipt.Kind)
	require.Equal(t, applicationID, receipt.ApplicationID)
	require.Equal(t, ProgressEventPendingSubmission, receipt.EventType)
	require.Equal(t, ProgressStagePendingSubmission, receipt.Stage)
	require.EqualValues(t, 1, receipt.Seq)
	require.EqualValues(t, 1, receipt.Revision)
	require.NotEmpty(t, receipt.EventID)
	require.Equal(t, "owner-1", receipt.Confirmer)
	require.Equal(t, "manual", receipt.Source.Kind)

	rows := readProgressRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, receipt.EventID, rows[0].ID)
	require.Equal(t, applicationID, rows[0].ApplicationID)
	require.EqualValues(t, 1, rows[0].Seq)
	require.Equal(t, ProgressKindAppended, rows[0].Kind)
	require.Equal(t, ProgressEventPendingSubmission, rows[0].EventType)
	require.Empty(t, rows[0].CorrectsEventID)

	// Exact replay returns the stored receipt without a second event.
	replayed, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "imm-1", ProgressEventPendingSubmission, "通过官网投递", 0))
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Len(t, readProgressRows(t, db), 1)

	// The same request ID with different content is a typed conflict.
	mutated := appendProgressInput(applicationID, "imm-1", ProgressEventPendingSubmission, "内容已变", 0)
	_, err = o.AppendProgress(ctx, mutated)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	require.Len(t, readProgressRows(t, db), 1)
}

func TestCorrectProgressAppendsCorrectionWithoutOverwritingOriginal(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1902)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "corr", "batch-a")

	original, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "corr-1", ProgressEventPendingSubmission, "官网投递", 0))
	require.NoError(t, err)
	snapshot := readProgressRows(t, db)[0]

	moment := progressMoment()
	correction := CorrectProgressInput{
		RequestID:        "corr-2",
		ApplicationID:    applicationID,
		CorrectsEventID:  original.EventID,
		EventType:        ProgressEventAssessment,
		Note:             "更正：当天已完成在线测评",
		OccurredAt:       &moment,
		Source:           Source{Kind: "manual"},
		ExpectedRevision: 1,
	}
	receipt, err := o.CorrectProgress(ctx, correction)
	require.NoError(t, err)
	require.Equal(t, ProgressKindCorrected, receipt.Kind)
	require.Equal(t, original.EventID, receipt.CorrectsEventID)
	require.EqualValues(t, 2, receipt.Seq)
	require.EqualValues(t, 2, receipt.Revision)
	require.Equal(t, ProgressStageAssessment, receipt.Stage)

	// The correction appends a new row; the original row is byte-identical.
	after := readProgressRows(t, db)
	require.Len(t, after, 2)
	require.Equal(t, snapshot, after[0], "the original event must never be modified")
	require.Equal(t, original.EventID, after[1].CorrectsEventID)
	require.Equal(t, ProgressKindCorrected, after[1].Kind)

	view, err := o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Len(t, view.Events, 2)
	require.True(t, view.Events[0].Corrected, "the superseded original is flagged")
	require.Equal(t, original.EventID, view.Events[1].CorrectsEventID)
}

func TestProgressProjectionDerivesStageDeterministicallyFromConfirmedEvents(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1903)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "proj", "batch-a")

	_, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "proj-1", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)
	_, err = o.AppendProgress(ctx, appendProgressInput(applicationID, "proj-2", ProgressEventAssessment, "", 1))
	require.NoError(t, err)
	_, err = o.AppendProgress(ctx, appendProgressInput(applicationID, "proj-3", ProgressEventInterview, "一面", 2))
	require.NoError(t, err)

	view, err := o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, ProgressStageInterview, view.Stage, "the latest confirmed event defines the stage")
	require.EqualValues(t, 3, view.Revision)

	// The fold is a pure function of the event set: row order in the input
	// cannot change the projected stage.
	rows := readProgressRows(t, db)
	reversed := make([]progressEventRecord, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		reversed = append(reversed, progressRowToRecord(rows[i]))
	}
	require.Equal(t, ProgressStageInterview, projectProgressStage(reversed))

	_, err = o.AppendProgress(ctx, appendProgressInput(applicationID, "proj-4", ProgressEventRejected, "岗位已关闭", 3))
	require.NoError(t, err)
	view, err = o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, ProgressStageClosed, view.Stage)

	// Correcting the terminal event replaces its contribution in place.
	terminal := view.Events[len(view.Events)-1]
	moment := progressMoment()
	_, err = o.CorrectProgress(ctx, CorrectProgressInput{
		RequestID: "proj-c1", ApplicationID: applicationID, CorrectsEventID: terminal.EventID,
		EventType: ProgressEventOffer, Note: "更正：收到口头 Offer", OccurredAt: &moment,
		Source: Source{Kind: "manual"}, ExpectedRevision: 4,
	})
	require.NoError(t, err)
	view, err = o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, ProgressStageOffer, view.Stage, "a corrected terminal event projects the corrected stage")

	// Correcting a middle event does not reorder the timeline.
	_, err = o.CorrectProgress(ctx, CorrectProgressInput{
		RequestID: "proj-c2", ApplicationID: applicationID, CorrectsEventID: view.Events[1].EventID,
		EventType: ProgressEventOffer, Note: "更正：当时其实已拿 Offer", OccurredAt: &moment,
		Source: Source{Kind: "manual"}, ExpectedRevision: 5,
	})
	require.NoError(t, err)
	view, err = o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, ProgressStageOffer, view.Stage, "a corrected middle event keeps the later effective stage")
}

func progressRowToRecord(row storedProgressRow) progressEventRecord {
	return progressEventRecord{
		ID: row.ID, TenantID: row.TenantID, UserID: row.UserID, ApplicationID: row.ApplicationID,
		Seq: row.Seq, Kind: row.Kind, EventType: row.EventType, Note: row.Note,
		OccurredAt: row.OccurredAt, CorrectsEventID: row.CorrectsEventID, Source: row.Source,
		Confirmer: row.Confirmer, RequestID: row.RequestID, Fingerprint: row.Fingerprint,
		ReceiptBody: row.ReceiptBody, CreatedAt: row.CreatedAt,
	}
}

func TestProgressProjectionReopenYieldsSameResult(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1904)
	path := progressDBPath(t, db)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "reopen", "batch-a")

	_, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "reopen-1", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)
	second, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "reopen-2", ProgressEventInterview, "一面", 1))
	require.NoError(t, err)
	moment := progressMoment()
	_, err = o.CorrectProgress(ctx, CorrectProgressInput{
		RequestID: "reopen-c1", ApplicationID: applicationID, CorrectsEventID: second.EventID,
		EventType: ProgressEventAssessment, OccurredAt: &moment,
		Source: Source{Kind: "manual"}, ExpectedRevision: 2,
	})
	require.NoError(t, err)

	first, err := o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)

	// Reopen the same database through a fresh connection: the projection must
	// be rebuilt to the identical value without any mutable cached state.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	reopened, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	office2, err := NewOffice(reopened)
	require.NoError(t, err)
	ctx2 := WithScope(context.Background(), Scope{UserID: "owner-1", TenantID: 1904})
	require.NoError(t, office2.ClaimSpace(ctx2))
	secondView, err := office2.ApplicationProgress(ctx2, applicationID)
	require.NoError(t, err)
	require.Equal(t, first, secondView)
}

func TestProgressEventsTraceSourceAndConfirmer(t *testing.T) {
	o, _, ctx := newProgressOffice(t, "trace-owner", 1905)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "trace", "batch-a")

	receipt, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "trace-1", ProgressEventOffer, "口头 Offer", 0))
	require.NoError(t, err)
	require.Equal(t, "manual", receipt.Source.Kind)
	require.Equal(t, "trace-owner", receipt.Confirmer)

	view, err := o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, receipt.Source, view.Events[0].Source)
	require.Equal(t, "trace-owner", view.Events[0].Confirmer)

	// Server-side imports record their own provenance; arbitrary client-spoofed
	// provenance kinds are refused by the closed enum.
	imported := appendProgressInput(applicationID, "trace-2", ProgressEventWithdrawn, "", 1)
	imported.Source = Source{Kind: "system_import"}
	_, err = o.AppendProgress(ctx, imported)
	require.NoError(t, err)

	spoofed := appendProgressInput(applicationID, "trace-3", ProgressEventAssessment, "", 2)
	spoofed.Source = Source{Kind: "resume_extraction"}
	_, err = o.AppendProgress(ctx, spoofed)
	require.ErrorIs(t, err, ErrInvalidRequest)

	view, err = o.ApplicationProgress(ctx, applicationID)
	require.NoError(t, err)
	require.Equal(t, "system_import", view.Events[1].Source.Kind)
}

func TestProgressEventsDoNotChainAcrossApplications(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1906)
	applicationA := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "chain-a", "batch-a")
	applicationB := seedProgressApplication(t, o, ctx, "招聘Go工程师。", "2027", "chain-b", "batch-b")

	inA, err := o.AppendProgress(ctx, appendProgressInput(applicationA, "chain-1", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)

	// A correction inside B may never reference an event that belongs to A.
	moment := progressMoment()
	_, err = o.CorrectProgress(ctx, CorrectProgressInput{
		RequestID: "chain-2", ApplicationID: applicationB, CorrectsEventID: inA.EventID,
		EventType: ProgressEventAssessment, OccurredAt: &moment,
		Source: Source{Kind: "manual"}, ExpectedRevision: 0,
	})
	require.ErrorIs(t, err, ErrProgressEventNotFound)

	viewB, err := o.ApplicationProgress(ctx, applicationB)
	require.NoError(t, err)
	require.Empty(t, viewB.Events, "application B never absorbs events of application A")
	require.EqualValues(t, 0, viewB.Revision)

	// B keeps its own independent sequence numbering.
	inB, err := o.AppendProgress(ctx, appendProgressInput(applicationB, "chain-3", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)
	require.EqualValues(t, 1, inB.Seq)

	rows := readProgressRows(t, db)
	require.Len(t, rows, 2)
	require.Equal(t, applicationA, rows[0].ApplicationID)
	require.Equal(t, applicationB, rows[1].ApplicationID)
}

func TestProgressScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1907)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "scope", "batch-a")

	// A different owner holds their own space in their own tenant (one space
	// per owner, one owner per tenant): their application lookup may never see
	// owner-1's application.
	intruder := WithScope(context.Background(), Scope{UserID: "owner-2", TenantID: 1909})
	require.NoError(t, o.ClaimSpace(intruder))
	_, err := o.AppendProgress(intruder, appendProgressInput(applicationID, "scope-1", ProgressEventPendingSubmission, "", 0))
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.ApplicationProgress(intruder, applicationID)
	require.ErrorIs(t, err, ErrApplicationNotFound)

	// A second user of the same tenant has no personal space at all: the scope
	// gate refuses them before any application access.
	sameTenant := WithScope(context.Background(), Scope{UserID: "owner-3", TenantID: 1907})
	_, err = o.AppendProgress(sameTenant, appendProgressInput(applicationID, "scope-2", ProgressEventPendingSubmission, "", 0))
	require.ErrorIs(t, err, ErrUnauthorized)

	// A missing application answers with the same error: no existence leak.
	_, err = o.ApplicationProgress(ctx, "0b7e6d64-6cd4-4df2-9b01-000000000000")
	require.ErrorIs(t, err, ErrApplicationNotFound)

	require.Empty(t, readProgressRows(t, db), "no foreign write may persist an event")
}

func TestProgressRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1909)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "rev", "batch-a")

	first, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "rev-1", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Revision)

	// A stale expected revision returns the current revision, not a blank 409.
	_, err = o.AppendProgress(ctx, appendProgressInput(applicationID, "rev-2", ProgressEventAssessment, "", 0))
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, 1, conflict.CurrentRevision)

	moment := progressMoment()
	_, err = o.CorrectProgress(ctx, CorrectProgressInput{
		RequestID: "rev-3", ApplicationID: applicationID, CorrectsEventID: first.EventID,
		EventType: ProgressEventInterview, OccurredAt: &moment,
		Source: Source{Kind: "manual"}, ExpectedRevision: 0,
	})
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, 1, conflict.CurrentRevision)

	require.Len(t, readProgressRows(t, db), 1, "a losing revision must not append")
}

func TestProgressUnknownOutcomeRecoversViaReceipt(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "owner-1", 1910)
	applicationID := seedProgressApplication(t, o, ctx, "仅限2027届。", "2027", "unknown", "batch-a")

	// Cancel the operation right after the event insert: the outcome of the
	// write is unknown to the caller, never a blind retry with a new ID.
	writeCtx, cancel := context.WithCancel(ctx)
	o.afterProgressEventPersist = func() error {
		cancel()
		return writeCtx.Err()
	}
	_, err := o.AppendProgress(writeCtx, appendProgressInput(applicationID, "unknown-1", ProgressEventPendingSubmission, "", 0))
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown)
	require.Equal(t, "unknown-1", unknown.RequestID)

	// The original request ID decides the truth: nothing committed here.
	_, err = o.FindProgressReceipt(ctx, "unknown-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// Retrying the SAME request ID is safe and produces exactly one event.
	o.afterProgressEventPersist = nil
	recovered, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "unknown-1", ProgressEventPendingSubmission, "", 0))
	require.NoError(t, err)
	require.Len(t, readProgressRows(t, db), 1)

	// A committed write replays through the receipt lookup with its own ID.
	committed, err := o.AppendProgress(ctx, appendProgressInput(applicationID, "unknown-2", ProgressEventInterview, "一面", 1))
	require.NoError(t, err)
	replayed, err := o.FindProgressReceipt(ctx, "unknown-2")
	require.NoError(t, err)
	require.Equal(t, committed, replayed)
	require.Equal(t, recovered.EventID, readProgressRows(t, db)[0].ID)
}
