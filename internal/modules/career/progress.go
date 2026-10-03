package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Progress stages (T17) are the frozen projection vocabulary from the approved
// spec: 准备中、待投递、已投递、测评或笔试、面试、Offer、已结束. Event types
// record what actually happened; the terminal facts (rejected, withdrawn,
// retracted) project onto the closed stage.
const (
	ProgressStagePreparing         = "preparing"
	ProgressStagePendingSubmission = "pending_submission"
	ProgressStageSubmitted         = "submitted"
	ProgressStageAssessment        = "assessment"
	ProgressStageInterview         = "interview"
	ProgressStageOffer             = "offer"
	ProgressStageClosed            = "closed"

	ProgressEventPendingSubmission = "pending_submission"
	ProgressEventSubmitted         = "submitted"
	ProgressEventAssessment        = "assessment"
	ProgressEventInterview         = "interview"
	ProgressEventOffer             = "offer"
	ProgressEventResubmitted       = "resubmitted"
	ProgressEventRejected          = "rejected"
	ProgressEventWithdrawn         = "withdrawn"
	ProgressEventRetracted         = "retracted"

	ProgressKindAppended  = "progress_appended"
	ProgressKindCorrected = "progress_corrected"

	maxProgressNoteBytes     = 4096
	maxProgressRequestIDLen  = 128
	progressWriteBusyRetries = 4
	progressWriteBackoff     = 10 * time.Millisecond
)

var (
	ErrProgressEventNotFound = errors.New("career progress event not found")
)

// progressEventStage maps the closed event dictionary onto the closed stage
// vocabulary. The stage of the latest effective event is the projection.
var progressEventStage = map[string]string{
	ProgressEventPendingSubmission: ProgressStagePendingSubmission,
	ProgressEventSubmitted:         ProgressStageSubmitted,
	ProgressEventAssessment:        ProgressStageAssessment,
	ProgressEventInterview:         ProgressStageInterview,
	ProgressEventOffer:             ProgressStageOffer,
	ProgressEventResubmitted:       ProgressStageSubmitted,
	ProgressEventRejected:          ProgressStageClosed,
	ProgressEventWithdrawn:         ProgressStageClosed,
	ProgressEventRetracted:         ProgressStageClosed,
}

// progressSourceKinds is the closed provenance enum: 用户录入 (manual) and
// 系统导入 (system_import). HTTP callers may only claim manual entry.
var progressSourceKinds = map[string]bool{
	"manual":        true,
	"user":          true,
	"system_import": true,
}

// AppendProgressInput appends one progress event bound to a single application.
// The confirmer is always the authenticated scope user and never client input.
type AppendProgressInput struct {
	RequestID        string     `json:"requestId"`
	ApplicationID    string     `json:"applicationId"`
	EventType        string     `json:"eventType"`
	Note             string     `json:"note,omitempty"`
	OccurredAt       *time.Time `json:"occurredAt,omitempty"`
	Source           Source     `json:"source"`
	ExpectedRevision uint64     `json:"expectedRevision"`
}

// CorrectProgressInput appends a correction event referencing the corrected
// event. The original row is never modified or deleted.
type CorrectProgressInput struct {
	RequestID        string     `json:"requestId"`
	ApplicationID    string     `json:"applicationId"`
	CorrectsEventID  string     `json:"correctsEventId"`
	EventType        string     `json:"eventType"`
	Note             string     `json:"note,omitempty"`
	OccurredAt       *time.Time `json:"occurredAt,omitempty"`
	Source           Source     `json:"source"`
	ExpectedRevision uint64     `json:"expectedRevision"`
}

// ProgressReceipt is the frozen durable receipt of every progress write and the
// contract served by the receipt endpoint.
type ProgressReceipt struct {
	Kind            string    `json:"kind"`
	RequestID       string    `json:"requestId"`
	ApplicationID   string    `json:"applicationId"`
	EventID         string    `json:"eventId"`
	Seq             uint64    `json:"seq"`
	Revision        uint64    `json:"revision"`
	CorrectsEventID string    `json:"correctsEventId,omitempty"`
	EventType       string    `json:"eventType"`
	Stage           string    `json:"stage"`
	Note            string    `json:"note,omitempty"`
	OccurredAt      time.Time `json:"occurredAt"`
	Source          Source    `json:"source"`
	Confirmer       string    `json:"confirmer"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ProgressEventView is one immutable history row with its correction flag.
type ProgressEventView struct {
	EventID           string    `json:"eventId"`
	Seq               uint64    `json:"seq"`
	Kind              string    `json:"kind"`
	EventType         string    `json:"eventType"`
	Note              string    `json:"note,omitempty"`
	OccurredAt        time.Time `json:"occurredAt"`
	Source            Source    `json:"source"`
	Confirmer         string    `json:"confirmer"`
	CorrectsEventID   string    `json:"correctsEventId,omitempty"`
	Corrected         bool      `json:"corrected"`
	RequestID         string    `json:"requestId"`
	CreatedAt         time.Time `json:"createdAt"`
	SubmissionID      string    `json:"submissionId,omitempty"`
	SubmissionChannel string    `json:"submissionChannel,omitempty"`
	VersionConfirmed  *bool     `json:"versionConfirmed,omitempty"`
}

// ProgressView is the per-application history plus the deterministic stage
// projection. It is always recomputed from the stored events.
type ProgressView struct {
	ApplicationID string              `json:"applicationId"`
	Revision      uint64              `json:"revision"`
	Stage         string              `json:"stage"`
	Events        []ProgressEventView `json:"events"`
}

// progressEventRecord is the append-only event log. Rows are immutable after
// insert: corrections append new rows that reference the corrected event.
type progressEventRecord struct {
	ID              string    `gorm:"primaryKey;size:36"`
	TenantID        uint64    `gorm:"uniqueIndex:career_progress_scope_seq,priority:1;uniqueIndex:career_progress_scope_request,priority:1;index:idx_career_progress_scope,priority:1"`
	UserID          string    `gorm:"uniqueIndex:career_progress_scope_seq,priority:2;uniqueIndex:career_progress_scope_request,priority:2;index:idx_career_progress_scope,priority:2;size:512"`
	ApplicationID   string    `gorm:"uniqueIndex:career_progress_scope_seq,priority:3;index:idx_career_progress_scope,priority:3;size:36"`
	Seq             uint64    `gorm:"uniqueIndex:career_progress_scope_seq,priority:4"`
	Kind            string    `gorm:"size:32;not null"`
	EventType       string    `gorm:"size:32;not null"`
	Note            string    `gorm:"type:text;not null;default:''"`
	OccurredAt      time.Time `gorm:"not null"`
	CorrectsEventID string    `gorm:"size:36;not null;default:''"`
	Source          string    `gorm:"type:text;not null"`
	Confirmer       string    `gorm:"size:512;not null"`
	RequestID       string    `gorm:"size:128;uniqueIndex:career_progress_scope_request,priority:3;not null"`
	Fingerprint     string    `gorm:"size:64;not null"`
	ReceiptBody     string    `gorm:"type:text;not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (progressEventRecord) TableName() string { return "career_progress_events" }

// AppendProgress appends one immutable progress event to a single application.
// The write carries the original request ID and the expected event revision;
// an exact replay returns the stored receipt without a second event.
func (o *Office) AppendProgress(ctx context.Context, input AppendProgressInput) (ProgressReceipt, error) {
	return o.writeProgress(ctx, progressIntent{
		kind:             ProgressKindAppended,
		requestID:        input.RequestID,
		applicationID:    input.ApplicationID,
		eventType:        input.EventType,
		note:             input.Note,
		occurredAt:       input.OccurredAt,
		source:           input.Source,
		expectedRevision: input.ExpectedRevision,
	})
}

// CorrectProgress appends a correction event referencing the corrected event.
// The original row is preserved byte-for-byte; the projection adopts the
// corrected semantics at the original event's position in the timeline.
func (o *Office) CorrectProgress(ctx context.Context, input CorrectProgressInput) (ProgressReceipt, error) {
	return o.writeProgress(ctx, progressIntent{
		kind:             ProgressKindCorrected,
		requestID:        input.RequestID,
		applicationID:    input.ApplicationID,
		correctsEventID:  input.CorrectsEventID,
		eventType:        input.EventType,
		note:             input.Note,
		occurredAt:       input.OccurredAt,
		source:           input.Source,
		expectedRevision: input.ExpectedRevision,
	})
}

type progressIntent struct {
	kind             string
	requestID        string
	applicationID    string
	correctsEventID  string
	eventType        string
	note             string
	occurredAt       *time.Time
	source           Source
	expectedRevision uint64
}

func (intent progressIntent) validate() error {
	if intent.requestID == "" || len(intent.requestID) > maxProgressRequestIDLen {
		return ErrInvalidRequest
	}
	if intent.applicationID == "" || len(intent.applicationID) > 36 {
		return ErrInvalidRequest
	}
	if _, ok := progressEventStage[intent.eventType]; !ok {
		return ErrInvalidRequest
	}
	// Submission facts are recorded through RecordSubmission, which binds the
	// user-confirmed channel, actual submission time, and material version (or
	// an explicit unknown-version marker). The generic event contract cannot.
	if intent.eventType == ProgressEventSubmitted || intent.eventType == ProgressEventResubmitted {
		return ErrInvalidRequest
	}
	if len(intent.note) > maxProgressNoteBytes {
		return ErrInvalidRequest
	}
	if !progressSourceKinds[intent.source.Kind] {
		return ErrInvalidRequest
	}
	if intent.kind == ProgressKindCorrected && (intent.correctsEventID == "" || len(intent.correctsEventID) > 36) {
		return ErrInvalidRequest
	}
	return nil
}

// writeProgress owns the shared append/correct flow: scope, idempotent replay,
// application ownership, per-application revision CAS, and the append-only
// insert — all in one transaction with typed unknown-outcome recovery.
func (o *Office) writeProgress(ctx context.Context, intent progressIntent) (ProgressReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ProgressReceipt{}, err
	}
	intent.requestID = strings.TrimSpace(intent.requestID)
	intent.applicationID = strings.TrimSpace(intent.applicationID)
	intent.correctsEventID = strings.TrimSpace(intent.correctsEventID)
	intent.note = strings.TrimSpace(intent.note)
	if err = intent.validate(); err != nil {
		return ProgressReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		if isSQLiteBusy(err) {
			// Space validation hit a transient SQLite lock before any write
			// started; the same request ID may be retried, so report a typed
			// unknown outcome instead of a raw lock error.
			return ProgressReceipt{}, &OutcomeUnknownError{RequestID: intent.requestID}
		}
		return ProgressReceipt{}, err
	}
	// The fingerprint covers the intent as submitted (a nil occurredAt stays
	// nil), so an exact replay of the same request reproduces it.
	fingerprint, err := materialFingerprint(intent.kind, intent.requestID, intent.applicationID,
		intent.correctsEventID, intent.eventType, intent.note, intent.occurredAt, intent.source, intent.expectedRevision)
	if err != nil {
		return ProgressReceipt{}, err
	}

	// Keep lock acquisition finite even when the HTTP client supplied no
	// deadline.
	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for attempt := 0; ; attempt++ {
		receipt, txErr := o.attemptProgressWrite(operationCtx, s, intent, fingerprint)
		if txErr == nil {
			return receipt, nil
		}
		if errors.Is(txErr, ErrIdempotencyConflict) || errors.Is(txErr, ErrInvalidRequest) ||
			errors.Is(txErr, ErrApplicationNotFound) || errors.Is(txErr, ErrProgressEventNotFound) {
			return ProgressReceipt{}, txErr
		}
		var revisionConflict *RevisionConflictError
		ambiguous := errors.As(txErr, &revisionConflict) || isSQLiteBusy(txErr) || isReceiptRaceError(txErr)
		if ambiguous {
			// Never retry blindly with a new ID: the original request ID
			// decides whether the identical intent already committed.
			replay, found, lookupErr := o.replayProgressReceipt(operationCtx, s, intent.requestID, fingerprint)
			if lookupErr != nil {
				if operationCtx.Err() != nil || errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
					return ProgressReceipt{}, &OutcomeUnknownError{RequestID: intent.requestID}
				}
				return ProgressReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
		}
		if operationCtx.Err() != nil || ctx.Err() != nil {
			return ProgressReceipt{}, &OutcomeUnknownError{RequestID: intent.requestID}
		}
		if isSQLiteBusy(txErr) && attempt < progressWriteBusyRetries {
			time.Sleep(progressWriteBackoff)
			continue
		}
		if isSQLiteBusy(txErr) {
			// The transaction rolled back, but the lock state of a borrowed
			// SQLite connection makes the outcome class unknown to the caller;
			// the receipt lookup above is the recovery path.
			return ProgressReceipt{}, &OutcomeUnknownError{RequestID: intent.requestID}
		}
		return ProgressReceipt{}, txErr
	}
}

func (o *Office) attemptProgressWrite(ctx context.Context, s Scope, intent progressIntent, fingerprint string) (ProgressReceipt, error) {
	var receipt ProgressReceipt
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Exact replay precedes every side effect: a stored request ID with the
		// same fingerprint returns the stored event, different content conflicts.
		var stored progressEventRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, intent.requestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return decodeProgressReceipt(stored.ReceiptBody, &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		// The event binds to exactly one application that must exist under the
		// authenticated scope; the row lock also serializes seq allocation.
		var application applicationRecord
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, intent.applicationID).
			First(&application).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		if e != nil {
			return e
		}
		var head struct{ Count int64 }
		if e = tx.Model(&progressEventRecord{}).
			Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, intent.applicationID).
			Count(&head.Count).Error; e != nil {
			return e
		}
		if head.Count < 0 || uint64(head.Count) != intent.expectedRevision {
			return &RevisionConflictError{CurrentRevision: uint64(head.Count)}
		}
		// A correction may only reference a plain event of the SAME
		// application; foreign events are simply not visible here.
		if intent.kind == ProgressKindCorrected {
			var target progressEventRecord
			e = tx.Where("tenant_id=? AND user_id=? AND application_id=? AND id=?",
				s.TenantID, s.UserID, intent.applicationID, intent.correctsEventID).
				First(&target).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrProgressEventNotFound
			}
			if e != nil {
				return e
			}
			if target.CorrectsEventID != "" {
				return ErrInvalidRequest
			}
		}
		now := time.Now().UTC()
		occurredAt := now
		if intent.occurredAt != nil {
			occurredAt = intent.occurredAt.UTC()
		}
		nextRevision := uint64(head.Count) + 1
		receipt = ProgressReceipt{
			Kind:            intent.kind,
			RequestID:       intent.requestID,
			ApplicationID:   intent.applicationID,
			EventID:         uuid.NewString(),
			Seq:             nextRevision,
			Revision:        nextRevision,
			CorrectsEventID: intent.correctsEventID,
			EventType:       intent.eventType,
			Stage:           progressEventStage[intent.eventType],
			Note:            intent.note,
			OccurredAt:      occurredAt,
			Source:          intent.source,
			Confirmer:       s.UserID,
			CreatedAt:       now,
		}
		sourceBody, e := json.Marshal(receipt.Source)
		if e != nil {
			return e
		}
		receiptBody, e := json.Marshal(receipt)
		if e != nil {
			return e
		}
		row := progressEventRecord{
			ID: receipt.EventID, TenantID: s.TenantID, UserID: s.UserID,
			ApplicationID: intent.applicationID, Seq: receipt.Seq, Kind: intent.kind,
			EventType: intent.eventType, Note: intent.note, OccurredAt: occurredAt,
			CorrectsEventID: intent.correctsEventID, Source: string(sourceBody),
			Confirmer: s.UserID, RequestID: intent.requestID, Fingerprint: fingerprint,
			ReceiptBody: string(receiptBody), CreatedAt: now,
		}
		if e = tx.Create(&row).Error; e != nil {
			return e
		}
		if o.afterProgressEventPersist != nil {
			if hookErr := o.afterProgressEventPersist(); hookErr != nil {
				return hookErr
			}
		}
		return nil
	})
	if err != nil {
		return ProgressReceipt{}, err
	}
	return receipt, nil
}

// FindProgressReceipt replays the stored progress receipt by request ID under
// the authenticated scope.
func (o *Office) FindProgressReceipt(ctx context.Context, requestID string) (ProgressReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ProgressReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ProgressReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxProgressRequestIDLen {
		return ProgressReceipt{}, ErrInvalidRequest
	}
	var row progressEventRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProgressReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return ProgressReceipt{}, err
	}
	var receipt ProgressReceipt
	if err = decodeProgressReceipt(row.ReceiptBody, &receipt); err != nil {
		return ProgressReceipt{}, err
	}
	return receipt, nil
}

// ApplicationProgress serves the immutable event history of one application
// plus the deterministic stage projection recomputed from those events.
func (o *Office) ApplicationProgress(ctx context.Context, applicationID string) (ProgressView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ProgressView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ProgressView{}, err
	}
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" || len(applicationID) > 36 {
		return ProgressView{}, ErrInvalidRequest
	}
	var application applicationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, applicationID).
		First(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProgressView{}, ErrApplicationNotFound
	}
	if err != nil {
		return ProgressView{}, err
	}
	var rows []progressEventRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, applicationID).
		Order("seq ASC").Find(&rows).Error
	if err != nil {
		return ProgressView{}, err
	}
	view := buildProgressView(applicationID, rows)
	var submission submissionRecord
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, applicationID).First(&submission).Error
	if err == nil {
		var linked progressEventRecord
		err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND application_id=? AND request_id=?", s.TenantID, s.UserID, applicationID, submissionProgressRequestID(submission.ID)).First(&linked).Error
		if err == nil {
			for i := range view.Events {
				if view.Events[i].RequestID == linked.RequestID {
					view.Events[i].SubmissionID = submission.ID
					view.Events[i].SubmissionChannel = submission.Channel
					view.Events[i].VersionConfirmed = boolPointer(submission.VersionConfirmed)
					view.Events[i].Note = submission.Note
					view.Events[i].Confirmer = submission.Confirmer
				}
			}
			view.Stage = projectProgressStageWithSubmission(rows, linked)
			view.Revision++
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ProgressView{}, err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ProgressView{}, err
	}
	return view, nil
}

func boolPointer(value bool) *bool { return &value }

func projectProgressStageWithSubmission(rows []progressEventRecord, submission progressEventRecord) string {
	rows = append(append([]progressEventRecord(nil), rows...), submission)
	return projectProgressStage(rows)
}

// buildProgressView folds the stored rows into history plus projection. The
// corrected flag marks originals superseded by a later correction event.
func buildProgressView(applicationID string, rows []progressEventRecord) ProgressView {
	superseded := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.CorrectsEventID != "" {
			superseded[row.CorrectsEventID] = true
		}
	}
	events := make([]ProgressEventView, 0, len(rows))
	for _, row := range rows {
		view := ProgressEventView{
			EventID: row.ID, Seq: row.Seq, Kind: row.Kind, EventType: row.EventType,
			Note: row.Note, OccurredAt: row.OccurredAt, Source: decodeSource(row.Source),
			Confirmer: row.Confirmer, CorrectsEventID: row.CorrectsEventID,
			Corrected: superseded[row.ID], RequestID: row.RequestID, CreatedAt: row.CreatedAt,
		}
		if strings.HasPrefix(row.RequestID, "submission:") {
			view.SubmissionID = strings.TrimPrefix(row.RequestID, "submission:")
		}
		events = append(events, view)
	}
	return ProgressView{
		ApplicationID: applicationID,
		Revision:      uint64(len(rows)),
		Stage:         projectProgressStage(rows),
		Events:        events,
	}
}

// projectProgressStage is the pure, deterministic fold from the confirmed
// event set onto the closed stage vocabulary: a correction replaces its
// target's contribution at the target's timeline position (corrections never
// chain and never reorder), and the latest effective event defines the stage.
// The result depends only on the event set, never on row iteration order.
func projectProgressStage(events []progressEventRecord) string {
	if len(events) == 0 {
		return ProgressStagePreparing
	}
	latestCorrection := make(map[string]progressEventRecord, len(events))
	for _, event := range events {
		if event.CorrectsEventID == "" {
			continue
		}
		if prior, ok := latestCorrection[event.CorrectsEventID]; !ok || event.Seq > prior.Seq {
			latestCorrection[event.CorrectsEventID] = event
		}
	}
	var topSeq uint64
	stage := ProgressStagePreparing
	for _, event := range events {
		if event.CorrectsEventID != "" {
			continue
		}
		effective := event
		if correction, ok := latestCorrection[event.ID]; ok {
			effective = correction
		}
		if event.Seq <= topSeq {
			continue
		}
		topSeq = event.Seq
		if mapped, ok := progressEventStage[effective.EventType]; ok {
			stage = mapped
		}
	}
	return stage
}

func (o *Office) replayProgressReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (ProgressReceipt, bool, error) {
	var row progressEventRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProgressReceipt{}, false, nil
	}
	if err != nil {
		return ProgressReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return ProgressReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt ProgressReceipt
	if err = decodeProgressReceipt(row.ReceiptBody, &receipt); err != nil {
		return ProgressReceipt{}, true, err
	}
	return receipt, true, nil
}

func decodeProgressReceipt(body string, receipt *ProgressReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career progress receipt: %w", err)
	}
	return nil
}
