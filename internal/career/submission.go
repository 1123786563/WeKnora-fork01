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

// Submission channels (T18) are the closed enum a user picks from when
// confirming what they did externally. The product itself never submits,
// mails, or fills any external form; it only records the user's claim.
const (
	SubmissionChannelEmail = "email"
	SubmissionChannelWeb   = "web"
	SubmissionChannelOther = "other"

	SubmissionKindRecorded = "submission_recorded"

	maxSubmissionNoteBytes    = 4096
	maxSubmissionRequestIDLen = 128
)

var (
	// ErrSubmissionNotFound answers a submission ID that does not exist under
	// the authenticated scope (including foreign owners' submissions).
	ErrSubmissionNotFound = errors.New("career submission not found")
	// ErrSubmissionAlreadyConfirmed is the typed conflict for a repeat
	// confirmation of an application that already holds one submission record.
	ErrSubmissionAlreadyConfirmed = errors.New("career application already has a confirmed submission")
)

// submissionChannels is the closed channel vocabulary: email/web/other,
// selected by the user.
var submissionChannels = map[string]bool{
	SubmissionChannelEmail: true,
	SubmissionChannelWeb:   true,
	SubmissionChannelOther: true,
}

// RecordSubmissionInput records one user-confirmed submission fact. Either the
// user binds an exact submittable material export (materialId+exportId) or
// they explicitly mark the version unknown (versionUnknown=true); the product
// never fabricates a version reference and never infers one from downloads.
type RecordSubmissionInput struct {
	RequestID        string     `json:"requestId"`
	ApplicationID    string     `json:"applicationId"`
	Channel          string     `json:"channel"`
	OccurredAt       *time.Time `json:"occurredAt,omitempty"`
	MaterialID       string     `json:"materialId,omitempty"`
	ExportID         string     `json:"exportId,omitempty"`
	VersionUnknown   bool       `json:"versionUnknown"`
	Note             string     `json:"note,omitempty"`
	ExpectedRevision uint64     `json:"expectedRevision"`
}

// SubmissionVersionBinding is the immutable version reference frozen at
// recording time: the material, the submittable export, the version number,
// and the content digest both rendered files shared. Downstream preparation
// resolves exactly this reference; it is never rewritten.
type SubmissionVersionBinding struct {
	MaterialID    string `json:"materialId"`
	ExportID      string `json:"exportId"`
	Version       uint64 `json:"version"`
	ContentDigest string `json:"contentDigest"`
}

// SubmissionReceipt is the frozen durable receipt of one submission
// confirmation and the contract served by the receipt endpoint.
type SubmissionReceipt struct {
	Kind             string                    `json:"kind"`
	RequestID        string                    `json:"requestId"`
	ApplicationID    string                    `json:"applicationId"`
	SubmissionID     string                    `json:"submissionId"`
	Channel          string                    `json:"channel"`
	OccurredAt       time.Time                 `json:"occurredAt"`
	VersionConfirmed bool                      `json:"versionConfirmed"`
	BoundVersion     *SubmissionVersionBinding `json:"boundVersion,omitempty"`
	Note             string                    `json:"note,omitempty"`
	Confirmer        string                    `json:"confirmer"`
	Revision         uint64                    `json:"revision"`
	CreatedAt        time.Time                 `json:"createdAt"`
}

// submissionRecord is the durable user-confirmed submission fact. One
// application holds at most one row; the version columns freeze the binding
// (empty and version_confirmed=false record the explicit unknown marker).
type submissionRecord struct {
	ID               string    `gorm:"primaryKey;size:36"`
	TenantID         uint64    `gorm:"uniqueIndex:career_submission_scope_application,priority:1;uniqueIndex:career_submission_scope_request,priority:1;index:idx_career_submission_scope,priority:1"`
	UserID           string    `gorm:"uniqueIndex:career_submission_scope_application,priority:2;uniqueIndex:career_submission_scope_request,priority:2;index:idx_career_submission_scope,priority:2;size:512"`
	ApplicationID    string    `gorm:"uniqueIndex:career_submission_scope_application,priority:3;index:idx_career_submission_scope,priority:3;size:36"`
	RequestID        string    `gorm:"size:128;uniqueIndex:career_submission_scope_request,priority:3;not null"`
	Fingerprint      string    `gorm:"size:64;not null"`
	Channel          string    `gorm:"size:16;not null"`
	OccurredAt       time.Time `gorm:"not null"`
	VersionConfirmed bool      `gorm:"not null"`
	MaterialID       string    `gorm:"size:36;not null;default:''"`
	ExportID         string    `gorm:"size:36;not null;default:''"`
	Version          uint64    `gorm:"not null;default:0"`
	ContentDigest    string    `gorm:"size:64;not null;default:''"`
	Note             string    `gorm:"type:text;not null;default:''"`
	Confirmer        string    `gorm:"size:512;not null"`
	ReceiptBody      string    `gorm:"type:text;not null"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (submissionRecord) TableName() string { return "career_submissions" }

// RecordSubmission persists one user-declared submission fact: the channel the
// user picked, the time they claim it happened, and either an exact submittable
// material export or the explicit unknown marker. It performs no external
// action (no mail, no form fill, no fetch) and infers nothing from downloads.
func (o *Office) RecordSubmission(ctx context.Context, input RecordSubmissionInput) (SubmissionReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SubmissionReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.ApplicationID = strings.TrimSpace(input.ApplicationID)
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	input.ExportID = strings.TrimSpace(input.ExportID)
	input.Note = strings.TrimSpace(input.Note)
	if err = validateSubmissionIntent(input); err != nil {
		return SubmissionReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		if isSQLiteBusy(err) {
			return SubmissionReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return SubmissionReceipt{}, err
	}
	// The fingerprint covers the intent as submitted (a nil occurredAt stays
	// nil), so an exact replay reproduces it and changed content conflicts.
	fingerprint, err := materialFingerprint(SubmissionKindRecorded, input.RequestID, input.ApplicationID,
		input.Channel, input.OccurredAt, input.MaterialID, input.ExportID, input.VersionUnknown,
		input.Note, input.ExpectedRevision)
	if err != nil {
		return SubmissionReceipt{}, err
	}
	if replay, found, lookupErr := o.replaySubmissionReceipt(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return SubmissionReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	// Keep lock acquisition finite even when the HTTP client supplied no
	// deadline.
	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for attempt := 0; ; attempt++ {
		receipt, txErr := o.attemptSubmissionWrite(operationCtx, s, input, fingerprint)
		if txErr == nil {
			return receipt, nil
		}
		if errors.Is(txErr, ErrIdempotencyConflict) || errors.Is(txErr, ErrInvalidRequest) ||
			errors.Is(txErr, ErrApplicationNotFound) || errors.Is(txErr, ErrExportNotFound) ||
			errors.Is(txErr, ErrExportNotSubmittable) || errors.Is(txErr, ErrSubmissionAlreadyConfirmed) {
			return SubmissionReceipt{}, txErr
		}
		var revisionConflict *RevisionConflictError
		ambiguous := errors.As(txErr, &revisionConflict) || isSQLiteBusy(txErr) || isReceiptRaceError(txErr)
		if ambiguous {
			// Never retry blindly with a new ID: the original request ID
			// decides whether the identical intent already committed.
			replay, found, lookupErr := o.replaySubmissionReceipt(operationCtx, s, input.RequestID, fingerprint)
			if lookupErr != nil {
				if operationCtx.Err() != nil || errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
					return SubmissionReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
				}
				return SubmissionReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
		}
		if operationCtx.Err() != nil || ctx.Err() != nil {
			return SubmissionReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if isSQLiteBusy(txErr) && attempt < progressWriteBusyRetries {
			time.Sleep(progressWriteBackoff)
			continue
		}
		if isSQLiteBusy(txErr) {
			return SubmissionReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return SubmissionReceipt{}, txErr
	}
}

func validateSubmissionIntent(input RecordSubmissionInput) error {
	if input.RequestID == "" || len(input.RequestID) > maxSubmissionRequestIDLen {
		return ErrInvalidRequest
	}
	if input.ApplicationID == "" || len(input.ApplicationID) > 36 {
		return ErrInvalidRequest
	}
	if !submissionChannels[input.Channel] {
		return ErrInvalidRequest
	}
	if len(input.Note) > maxSubmissionNoteBytes {
		return ErrInvalidRequest
	}
	if input.VersionUnknown {
		// The unknown marker is explicit: no version reference may accompany it.
		if input.MaterialID != "" || input.ExportID != "" {
			return ErrInvalidRequest
		}
		return nil
	}
	if input.MaterialID == "" || len(input.MaterialID) > 36 || input.ExportID == "" || len(input.ExportID) > 36 {
		return ErrInvalidRequest
	}
	return nil
}

// attemptSubmissionWrite owns the single transaction: replay check, application
// ownership, one-submission-per-application guard, the profile revision CAS,
// and the version binding frozen from the submittable export.
func (o *Office) attemptSubmissionWrite(ctx context.Context, s Scope, input RecordSubmissionInput, fingerprint string) (SubmissionReceipt, error) {
	var receipt SubmissionReceipt
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Exact replay precedes every side effect: a stored request ID with the
		// same fingerprint returns the stored record, different content conflicts.
		var stored submissionRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return decodeSubmissionReceipt(stored.ReceiptBody, &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		// The submission binds to exactly one application under the
		// authenticated scope.
		var application applicationRecord
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, input.ApplicationID).
			First(&application).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		if e != nil {
			return e
		}
		// Repeat confirmation never creates a second submission record: a new
		// request ID against a confirmed application is a typed conflict.
		var existing submissionRecord
		e = tx.Where("tenant_id=? AND user_id=? AND application_id=?",
			s.TenantID, s.UserID, input.ApplicationID).First(&existing).Error
		if e == nil {
			return ErrSubmissionAlreadyConfirmed
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		// House expected-revision CAS against the profile head.
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		// Resolve the version binding from the durable export under the lock:
		// only a submittable, non-revoked export may be bound.
		binding := SubmissionVersionBinding{}
		if !input.VersionUnknown {
			var row materialExportRecord
			e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id=? AND user_id=? AND material_id=? AND id=?",
					s.TenantID, s.UserID, input.MaterialID, input.ExportID).
				First(&row).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrExportNotFound
			}
			if e != nil {
				return e
			}
			if row.Status != ExportStatusSubmittable || row.RevokedAt != nil {
				return ErrExportNotSubmittable
			}
			binding = SubmissionVersionBinding{
				MaterialID:    row.MaterialID,
				ExportID:      row.ID,
				Version:       row.Version,
				ContentDigest: row.ContentDigest,
			}
		}
		now := time.Now().UTC()
		occurredAt := now
		if input.OccurredAt != nil {
			occurredAt = input.OccurredAt.UTC()
		}
		receipt = SubmissionReceipt{
			Kind:             SubmissionKindRecorded,
			RequestID:        input.RequestID,
			ApplicationID:    input.ApplicationID,
			SubmissionID:     uuid.NewString(),
			Channel:          input.Channel,
			OccurredAt:       occurredAt,
			VersionConfirmed: !input.VersionUnknown,
			Note:             input.Note,
			Confirmer:        s.UserID,
			Revision:         head.Revision,
			CreatedAt:        now,
		}
		if !input.VersionUnknown {
			receipt.BoundVersion = &binding
		}
		receiptBody, e := marshalSubmissionReceipt(receipt)
		if e != nil {
			return e
		}
		row := submissionRecord{
			ID: receipt.SubmissionID, TenantID: s.TenantID, UserID: s.UserID,
			ApplicationID: input.ApplicationID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Channel: input.Channel, OccurredAt: occurredAt,
			VersionConfirmed: receipt.VersionConfirmed,
			MaterialID:       binding.MaterialID, ExportID: binding.ExportID,
			Version: binding.Version, ContentDigest: binding.ContentDigest,
			Note: input.Note, Confirmer: s.UserID,
			ReceiptBody: receiptBody, CreatedAt: now, UpdatedAt: now,
		}
		if e = tx.Create(&row).Error; e != nil {
			return e
		}
		var progressHead struct{ Count int64 }
		if e = tx.Model(&progressEventRecord{}).Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, input.ApplicationID).Count(&progressHead.Count).Error; e != nil {
			return e
		}
		if progressHead.Count < 0 {
			return ErrInvalidRequest
		}
		progressSeq := uint64(progressHead.Count) + 1
		// The timeline event and submission fact share this transaction. Its
		// deterministic request identity prevents replay from creating a second
		// submitted event, while the linked submission ID preserves provenance.
		progressReceipt := ProgressReceipt{
			Kind: ProgressKindAppended, RequestID: submissionProgressRequestID(receipt.SubmissionID),
			ApplicationID: input.ApplicationID, EventID: uuid.NewString(), Seq: progressSeq, Revision: progressSeq,
			EventType: ProgressEventSubmitted, Stage: ProgressStageSubmitted,
			Note: input.Note, OccurredAt: occurredAt, Source: Source{Kind: "manual"},
			Confirmer: s.UserID, CreatedAt: now,
		}
		progressSource, e := json.Marshal(progressReceipt.Source)
		if e != nil {
			return e
		}
		progressBody, e := json.Marshal(progressReceipt)
		if e != nil {
			return e
		}
		progressRow := progressEventRecord{
			ID: progressReceipt.EventID, TenantID: s.TenantID, UserID: s.UserID,
			ApplicationID: input.ApplicationID, Seq: progressSeq, Kind: ProgressKindAppended,
			EventType: ProgressEventSubmitted, Note: input.Note, OccurredAt: occurredAt,
			Source: string(progressSource), Confirmer: s.UserID,
			RequestID: progressReceipt.RequestID, Fingerprint: fingerprint,
			ReceiptBody: string(progressBody), CreatedAt: now,
		}
		if e = tx.Create(&progressRow).Error; e != nil {
			return e
		}
		if o.afterSubmissionPersist != nil {
			if hookErr := o.afterSubmissionPersist(); hookErr != nil {
				return hookErr
			}
		}
		if o.afterSubmissionProgressPersist != nil {
			if hookErr := o.afterSubmissionProgressPersist(); hookErr != nil {
				return hookErr
			}
		}
		return nil
	})
	if err != nil {
		return SubmissionReceipt{}, err
	}
	return receipt, nil
}

func submissionProgressRequestID(submissionID string) string {
	return "submission:" + submissionID
}

// FindSubmissionReceipt replays the stored submission receipt by request ID
// under the authenticated scope.
func (o *Office) FindSubmissionReceipt(ctx context.Context, requestID string) (SubmissionReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SubmissionReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SubmissionReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxSubmissionRequestIDLen {
		return SubmissionReceipt{}, ErrInvalidRequest
	}
	var row submissionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmissionReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return SubmissionReceipt{}, err
	}
	var receipt SubmissionReceipt
	if err = decodeSubmissionReceipt(row.ReceiptBody, &receipt); err != nil {
		return SubmissionReceipt{}, err
	}
	return receipt, nil
}

// ApplicationSubmissions lists the submissions of one application under the
// authenticated scope.
func (o *Office) ApplicationSubmissions(ctx context.Context, applicationID string) ([]SubmissionReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" || len(applicationID) > 36 {
		return nil, ErrInvalidRequest
	}
	var application applicationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, applicationID).
		First(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApplicationNotFound
	}
	if err != nil {
		return nil, err
	}
	var rows []submissionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, applicationID).
		Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]SubmissionReceipt, 0, len(rows))
	for _, row := range rows {
		var receipt SubmissionReceipt
		if err = decodeSubmissionReceipt(row.ReceiptBody, &receipt); err != nil {
			return nil, err
		}
		out = append(out, receipt)
	}
	return out, nil
}

// SubmissionBoundVersion resolves the immutable version reference of one
// submission for downstream preparation (interview prep and friends): the
// answer is exactly what was recorded, never rewritten by later versions,
// export revocations, or profile moves.
func (o *Office) SubmissionBoundVersion(ctx context.Context, submissionID string) (SubmissionVersionBinding, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SubmissionVersionBinding{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SubmissionVersionBinding{}, err
	}
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || len(submissionID) > 36 {
		return SubmissionVersionBinding{}, ErrInvalidRequest
	}
	var row submissionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, submissionID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmissionVersionBinding{}, ErrSubmissionNotFound
	}
	if err != nil {
		return SubmissionVersionBinding{}, err
	}
	if !row.VersionConfirmed {
		return SubmissionVersionBinding{}, ErrSubmissionNotFound
	}
	return SubmissionVersionBinding{
		MaterialID:    row.MaterialID,
		ExportID:      row.ExportID,
		Version:       row.Version,
		ContentDigest: row.ContentDigest,
	}, nil
}

func (o *Office) replaySubmissionReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (SubmissionReceipt, bool, error) {
	var row submissionRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SubmissionReceipt{}, false, nil
	}
	if err != nil {
		return SubmissionReceipt{}, false, err
	}
	if fingerprint != "" && row.Fingerprint != fingerprint {
		return SubmissionReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt SubmissionReceipt
	if err = decodeSubmissionReceipt(row.ReceiptBody, &receipt); err != nil {
		return SubmissionReceipt{}, true, err
	}
	return receipt, true, nil
}

func decodeSubmissionReceipt(body string, receipt *SubmissionReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career submission receipt: %w", err)
	}
	return nil
}

func marshalSubmissionReceipt(receipt SubmissionReceipt) (string, error) {
	body, err := json.Marshal(receipt)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
