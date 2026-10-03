package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	OpportunityStored      = "stored"
	OpportunityNeedsReview = "needs_review"
	maxJDTextBytes         = 1024 * 1024
	maxJDRequestBodyBytes  = 6*maxJDTextBytes + 64*1024
	// maxProfileActionBodyBytes bounds the profile Act body the same way the
	// other small JSON write endpoints are bounded (16KB).
	maxProfileActionBodyBytes = 16 * 1024
	opportunityLookupWindow   = 350 * time.Millisecond
)

var ErrOpportunityNotFound = errors.New("career opportunity evidence not found")

type opportunity struct {
	ID        string `gorm:"primaryKey;size:36"`
	TenantID  uint64 `gorm:"index:idx_career_opportunity_scope"`
	UserID    string `gorm:"index:idx_career_opportunity_scope;size:512"`
	CreatedAt time.Time
}

type opportunityObservation struct {
	ID            string `gorm:"primaryKey;size:36"`
	TenantID      uint64 `gorm:"index:idx_career_opportunity_observation_scope"`
	UserID        string `gorm:"index:idx_career_opportunity_observation_scope;size:512"`
	OpportunityID string `gorm:"size:36;index"`
	SnapshotID    string `gorm:"size:36"`
	SourceKind    string `gorm:"size:32"`
	SourceLabel   string `gorm:"type:text"`
	SourceRef     string `gorm:"type:text"`
	AcquiredAt    time.Time
	CreatedAt     time.Time
	// T09 URL source evidence columns. Older manual observations keep the
	// zero values; the paired migration adds these columns additively.
	SourceStatus       string `gorm:"size:32;not null;default:''"`
	Completeness       string `gorm:"size:16;not null;default:''"`
	FailureCode        string `gorm:"size:32;not null;default:''"`
	SubmittedURL       string `gorm:"type:text;not null;default:''"`
	FinalURL           string `gorm:"type:text;not null;default:''"`
	AdapterID          string `gorm:"size:64;not null;default:''"`
	AdapterVersion     string `gorm:"size:32;not null;default:''"`
	ObservedHTTPStatus int
}

type opportunitySnapshot struct {
	ID            string `gorm:"primaryKey;size:36"`
	TenantID      uint64 `gorm:"index:idx_career_opportunity_snapshot_scope"`
	UserID        string `gorm:"index:idx_career_opportunity_snapshot_scope;size:512"`
	OpportunityID string `gorm:"size:36;index"`
	ObservationID string `gorm:"size:36"`
	RawText       string `gorm:"type:text;not null"`
	RawSHA256     string `gorm:"size:64;not null"`
	Extracted     string `gorm:"type:text;not null"`
	Status        string `gorm:"size:32;not null"`
	AcquiredAt    time.Time
	CreatedAt     time.Time
}

type opportunityReceipt struct {
	TenantID    uint64 `gorm:"uniqueIndex:career_opportunity_receipt_scope"`
	UserID      string `gorm:"uniqueIndex:career_opportunity_receipt_scope;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_opportunity_receipt_scope;size:128"`
	Fingerprint string `gorm:"size:64;not null"`
	Body        string `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

func (opportunity) TableName() string            { return "career_opportunities" }
func (opportunityObservation) TableName() string { return "career_opportunity_observations" }
func (opportunitySnapshot) TableName() string    { return "career_opportunity_snapshots" }
func (opportunityReceipt) TableName() string     { return "career_opportunity_receipts" }

type ImportJDInput struct {
	RequestID          string `json:"requestId"`
	RawText            string `json:"rawText"`
	SourceLabel        string `json:"sourceLabel,omitempty"`
	SourceReference    string `json:"sourceReference,omitempty"`
	OpportunityID      string `json:"opportunityId,omitempty"`
	PriorObservationID string `json:"priorObservationId,omitempty"`
}

type ExtractedValue struct {
	State string `json:"state"`
	Value string `json:"value,omitempty"`
}

type OpportunityFields struct {
	Title        ExtractedValue `json:"title"`
	Company      ExtractedValue `json:"company"`
	Location     ExtractedValue `json:"location"`
	Batch        ExtractedValue `json:"batch"`
	Requirements ExtractedValue `json:"requirements"`
}

type OpportunitySource struct {
	Kind        string `json:"kind"`
	Label       string `json:"label,omitempty"`
	ReferenceID string `json:"referenceId,omitempty"`
}

type OpportunityEvidence struct {
	OpportunityID string            `json:"opportunityId"`
	ObservationID string            `json:"observationId"`
	SnapshotID    string            `json:"snapshotId"`
	RawText       string            `json:"rawText"`
	RawSHA256     string            `json:"rawSha256"`
	Extracted     OpportunityFields `json:"extracted"`
	Source        OpportunitySource `json:"source"`
	AcquiredAt    time.Time         `json:"acquiredAt"`
	Status        string            `json:"status"`
}

type OpportunityReceipt struct {
	Kind          string    `json:"kind"`
	RequestID     string    `json:"requestId"`
	OpportunityID string    `json:"opportunityId"`
	ObservationID string    `json:"observationId"`
	SnapshotID    string    `json:"snapshotId"`
	Status        string    `json:"status"`
	AcquiredAt    time.Time `json:"acquiredAt"`
}

type opportunityExtractor func(string) (OpportunityFields, error)

func unknownOpportunityFields() OpportunityFields {
	unknown := ExtractedValue{State: "unknown"}
	return OpportunityFields{Title: unknown, Company: unknown, Location: unknown, Batch: unknown, Requirements: unknown}
}

func (o *Office) extractOpportunity(raw string) (OpportunityFields, string) {
	extract := o.opportunityExtractor
	if extract == nil {
		// No extraction authority is configured yet. Preserve the source as evidence
		// and ask the user to review it rather than guessing requirements.
		return unknownOpportunityFields(), OpportunityNeedsReview
	}
	fields, err := extract(raw)
	if err != nil {
		return unknownOpportunityFields(), OpportunityNeedsReview
	}
	allKnown := true
	for _, value := range []ExtractedValue{fields.Title, fields.Company, fields.Location, fields.Batch, fields.Requirements} {
		if value.State != "known" && value.State != "unknown" {
			return unknownOpportunityFields(), OpportunityNeedsReview
		}
		if value.State == "unknown" {
			allKnown = false
		}
		if value.State == "known" && strings.TrimSpace(value.Value) == "" {
			return unknownOpportunityFields(), OpportunityNeedsReview
		}
	}
	if !allKnown {
		return fields, OpportunityNeedsReview
	}
	return fields, OpportunityStored
}

func (o *Office) ImportJD(ctx context.Context, input ImportJDInput) (OpportunityReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return OpportunityReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return OpportunityReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RequestID == "" || len(input.RequestID) > 128 || strings.TrimSpace(input.RawText) == "" || len(input.RawText) > maxJDTextBytes || len(input.SourceLabel) > 512 || len(input.SourceReference) > 2048 {
		return OpportunityReceipt{}, ErrInvalidRequest
	}
	fingerprintInput, err := json.Marshal(input)
	if err != nil {
		return OpportunityReceipt{}, err
	}
	fingerprintBytes := sha256.Sum256(fingerprintInput)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	var result OpportunityReceipt
	if o.beforeOpportunityTransaction != nil {
		o.beforeOpportunityTransaction()
	}
	persistenceMayHaveCommitted := false
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing opportunityReceipt
		err := tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).First(&existing).Error
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return json.Unmarshal([]byte(existing.Body), &result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// A transient SQLite lock can mean another same-request transaction
			// is still committing. Reconcile once through the bounded scoped path;
			// permanent query failures remain ordinary persistence errors.
			persistenceMayHaveCommitted = isSQLiteBusy(err)
			return err
		}
		if err := requireGateActiveTx(tx, s); err != nil {
			return err
		}

		fields, status := o.extractOpportunity(input.RawText)
		extracted, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		acquiredAt := time.Now().UTC()
		// Optional owner-scoped append: a user JD supplied after a URL
		// observation joins the same opportunity as a new immutable snapshot.
		// The original URL observation is never rewritten or replaced.
		oppID := uuid.NewString()
		appending := false
		if input.OpportunityID != "" || input.PriorObservationID != "" {
			if input.OpportunityID == "" || input.PriorObservationID == "" {
				return ErrInvalidRequest
			}
			var prior opportunityObservation
			err := tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, input.OpportunityID, input.PriorObservationID).First(&prior).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Pre-merge references keep resolving: a merged candidate's
				// observations were re-parented onto the merge target, so
				// retry through the merge chain (T12 reconciliation).
				if canonical := canonicalOpportunityID(tx, s, input.OpportunityID); canonical != "" && canonical != input.OpportunityID {
					err = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, canonical, input.PriorObservationID).First(&prior).Error
				}
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOpportunityNotFound
			}
			if err != nil {
				return err
			}
			if prior.SourceKind != "url" {
				return ErrInvalidRequest
			}
			// The append joins the observation's canonical owner, which may be
			// the merge target after a reconciliation merged the input ID away.
			oppID = prior.OpportunityID
			appending = true
		}
		observationID, snapshotID := uuid.NewString(), uuid.NewString()
		rawDigest := sha256.Sum256([]byte(input.RawText))
		source := OpportunitySource{Kind: "manual_paste", Label: input.SourceLabel, ReferenceID: input.SourceReference}
		result = OpportunityReceipt{Kind: "opportunity_imported", RequestID: input.RequestID, OpportunityID: oppID, ObservationID: observationID, SnapshotID: snapshotID, Status: status, AcquiredAt: acquiredAt}
		body, err := json.Marshal(result)
		if err != nil {
			return err
		}
		// Once the first insert is attempted, a later database error may
		// leave the caller uncertain about whether the transaction committed.
		persistenceMayHaveCommitted = true
		if appending {
			var existing opportunity
			if err = tx.Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, oppID).First(&existing).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Create(&opportunity{ID: oppID, TenantID: s.TenantID, UserID: s.UserID, CreatedAt: acquiredAt}).Error; err != nil {
				return err
			}
		}
		if err = tx.Create(&opportunityObservation{ID: observationID, TenantID: s.TenantID, UserID: s.UserID, OpportunityID: oppID, SnapshotID: snapshotID, SourceKind: source.Kind, SourceLabel: source.Label, SourceRef: source.ReferenceID, AcquiredAt: acquiredAt, CreatedAt: acquiredAt}).Error; err != nil {
			return err
		}
		if err = tx.Create(&opportunitySnapshot{ID: snapshotID, TenantID: s.TenantID, UserID: s.UserID, OpportunityID: oppID, ObservationID: observationID, RawText: input.RawText, RawSHA256: hex.EncodeToString(rawDigest[:]), Extracted: string(extracted), Status: status, AcquiredAt: acquiredAt, CreatedAt: acquiredAt}).Error; err != nil {
			return err
		}
		return tx.Create(&opportunityReceipt{TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID, Fingerprint: fingerprint, Body: string(body), CreatedAt: acquiredAt}).Error
	})
	if err == nil && o.afterOpportunityCommit != nil {
		err = o.afterOpportunityCommit()
	}
	if err == nil {
		return result, nil
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		return OpportunityReceipt{}, err
	}
	if !persistenceMayHaveCommitted {
		return OpportunityReceipt{}, err
	}
	// The transaction may have committed even when the acknowledgement or
	// request context was lost. Reconcile with a bounded context that retains
	// scope values but is independent of the cancelled request context.
	recovered, found, lookupErr := o.reconcileOpportunityReceipt(ctx, s, input.RequestID, fingerprint)
	if errors.Is(lookupErr, ErrIdempotencyConflict) {
		return OpportunityReceipt{}, ErrIdempotencyConflict
	}
	if lookupErr != nil {
		return OpportunityReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	if found {
		return recovered, nil
	}
	return OpportunityReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
}

func (o *Office) reconcileOpportunityReceipt(ctx context.Context, scope Scope, requestID, fingerprint string) (OpportunityReceipt, bool, error) {
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opportunityLookupWindow)
	defer cancel()
	delays := [...]time.Duration{0, 20 * time.Millisecond, 40 * time.Millisecond, 80 * time.Millisecond}
	var lastErr error
	for attempt, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-reconcileCtx.Done():
				timer.Stop()
				return OpportunityReceipt{}, false, reconcileCtx.Err()
			case <-timer.C:
			}
		}
		var existing opportunityReceipt
		err := o.db.WithContext(reconcileCtx).Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).First(&existing).Error
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return OpportunityReceipt{}, false, ErrIdempotencyConflict
			}
			var result OpportunityReceipt
			if err = json.Unmarshal([]byte(existing.Body), &result); err != nil {
				return OpportunityReceipt{}, false, err
			}
			return result, true, nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			lastErr = nil
		} else {
			lastErr = err
		}
		if attempt == len(delays)-1 {
			break
		}
	}
	return OpportunityReceipt{}, false, lastErr
}

func (o *Office) OpportunityEvidence(ctx context.Context, opportunityID, snapshotID string) (OpportunityEvidence, error) {
	s, err := getScope(ctx)
	if err != nil {
		return OpportunityEvidence{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return OpportunityEvidence{}, err
	}
	var row opportunitySnapshot
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, opportunityID, snapshotID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Pre-merge references keep resolving: a merged candidate's snapshots
		// were re-parented onto the merge target, so retry through the merge
		// chain before reporting not-found (T12 reconciliation).
		if canonical := canonicalOpportunityID(o.db.WithContext(ctx), s, opportunityID); canonical != "" && canonical != opportunityID {
			err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, canonical, snapshotID).First(&row).Error
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OpportunityEvidence{}, ErrOpportunityNotFound
	}
	if err != nil {
		return OpportunityEvidence{}, err
	}
	var observation opportunityObservation
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, row.OpportunityID, row.ObservationID).First(&observation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OpportunityEvidence{}, ErrOpportunityNotFound
	}
	if err != nil {
		return OpportunityEvidence{}, err
	}
	if observation.SnapshotID != row.ID {
		return OpportunityEvidence{}, ErrOpportunityNotFound
	}
	var fields OpportunityFields
	if err = json.Unmarshal([]byte(row.Extracted), &fields); err != nil {
		return OpportunityEvidence{}, fmt.Errorf("decode opportunity extraction: %w", err)
	}
	return OpportunityEvidence{OpportunityID: row.OpportunityID, ObservationID: observation.ID, SnapshotID: row.ID, RawText: row.RawText, RawSHA256: row.RawSHA256, Extracted: fields, Source: OpportunitySource{Kind: observation.SourceKind, Label: observation.SourceLabel, ReferenceID: observation.SourceRef}, AcquiredAt: row.AcquiredAt, Status: row.Status}, nil
}

func (o *Office) FindOpportunityReceipt(ctx context.Context, requestID string) (OpportunityReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return OpportunityReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return OpportunityReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return OpportunityReceipt{}, ErrInvalidRequest
	}
	var row opportunityReceipt
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OpportunityReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return OpportunityReceipt{}, err
	}
	var result OpportunityReceipt
	if err = json.Unmarshal([]byte(row.Body), &result); err != nil {
		return OpportunityReceipt{}, fmt.Errorf("decode opportunity receipt: %w", err)
	}
	return result, nil
}
