package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Preparation (T19) generates the cover letter and the interview preparation
// draft of one application. Generation is a pure function of durable
// evidence: it anchors to the version the user actually submitted, cites the
// application's frozen snapshot and confirmed facts, and never performs any
// external action. The generated draft is a material-domain body (the
// edit_material closed intent); revision and immutable versioning flow
// through the existing material seams.
const (
	PreparationFocusCoverLetter = "cover_letter"
	PreparationFocusInterview   = "interview_prep"

	PreparationKindGenerated = "preparation_generated"

	PreparationStatusDraft      = "draft"
	PreparationStatusGenerating = "generating"
	PreparationStatusFailed     = "failed"

	PreparationFailureGeneration = "generation_failed"

	maxPreparationRequestIDLen = 128
)

var (
	// ErrPreparationVersionUnknown is the typed prompt state: the
	// application's submission does not pin a version, so generation refuses
	// instead of guessing the latest one.
	ErrPreparationVersionUnknown = errors.New("career preparation requires an explicitly confirmed submitted version")
	// ErrPreparationGenerationFailed answers a generation seam failure: the
	// request stays durable and recoverable, and no blank success product is
	// ever published.
	ErrPreparationGenerationFailed = errors.New("career preparation generation failed")

	// errPreparationClaimRefused marks a composed body that failed claim
	// review inside the write transaction; the failure state persists in a
	// separate write after the rollback.
	errPreparationClaimRefused = errors.New("career preparation claim refused")
)

// preparationFocuses is the closed focus vocabulary: the cover letter and the
// interview preparation are the two T19 drafts.
var preparationFocuses = map[string]bool{
	PreparationFocusCoverLetter: true,
	PreparationFocusInterview:   true,
}

// PreparationVersionUnknownError is the visible prompt state served to the
// client: the application has no confirmed submitted version to anchor to.
type PreparationVersionUnknownError struct {
	ApplicationID      string
	SubmissionRecorded bool
}

func (e *PreparationVersionUnknownError) Error() string { return ErrPreparationVersionUnknown.Error() }
func (e *PreparationVersionUnknownError) Is(target error) bool {
	return target == ErrPreparationVersionUnknown
}

// PreparationAnchor is the immutable version reference the preparation is
// anchored to: exactly what the user-confirmed submission bound, never
// rewritten by later versions or export revocations.
type PreparationAnchor struct {
	SubmissionID  string `json:"submissionId"`
	MaterialID    string `json:"materialId"`
	ExportID      string `json:"exportId"`
	Version       uint64 `json:"version"`
	ContentDigest string `json:"contentDigest"`
}

// PreparationSnapshotRef cites the application's frozen opportunity snapshot.
type PreparationSnapshotRef struct {
	OpportunityID  string `json:"opportunityId"`
	SnapshotID     string `json:"snapshotId"`
	SnapshotSHA256 string `json:"snapshotSha256"`
}

// PreparationSources is the full citation chain of one preparation: the
// submitted version, the frozen snapshot, and the confirmed fact keys the
// draft references.
type PreparationSources struct {
	SubmittedVersion PreparationAnchor      `json:"submittedVersion"`
	Snapshot         PreparationSnapshotRef `json:"snapshot"`
	FactKeys         []string               `json:"factKeys"`
	ProfileRevision  uint64                 `json:"profileRevision"`
}

// GeneratePreparationInput is the closed generation intent.
type GeneratePreparationInput struct {
	RequestID        string `json:"requestId"`
	ApplicationID    string `json:"applicationId"`
	Focus            string `json:"focus"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// PreparationReceipt is the durable receipt of one generation and the frozen
// contract served by the receipt endpoint. A failed generation keeps the
// typed failure fields and an empty body — never a blank success product.
type PreparationReceipt struct {
	Kind           string               `json:"kind"`
	RequestID      string               `json:"requestId"`
	ApplicationID  string               `json:"applicationId"`
	PreparationID  string               `json:"preparationId"`
	Focus          string               `json:"focus"`
	Status         string               `json:"status"`
	Anchor         PreparationAnchor    `json:"anchor"`
	MaterialID     string               `json:"materialId,omitempty"`
	Body           MaterialBody         `json:"body"`
	ReviewRisks    []MaterialReviewRisk `json:"reviewRisks"`
	Sources        PreparationSources   `json:"sources"`
	FailureCode    string               `json:"failureCode,omitempty"`
	FailureMessage string               `json:"failureMessage,omitempty"`
	Revision       uint64               `json:"revision"`
	CreatedAt      time.Time            `json:"createdAt"`
}

// preparationRecord is the durable preparation ledger row: the anchor, the
// generation status, and the replay receipt. One request ID admits exactly
// one intent; a failed row keeps the request recoverable for a retry.
type preparationRecord struct {
	ID                  string    `gorm:"primaryKey;size:36"`
	TenantID            uint64    `gorm:"uniqueIndex:career_preparation_scope_request,priority:1;index:idx_career_preparation_scope,priority:1"`
	UserID              string    `gorm:"uniqueIndex:career_preparation_scope_request,priority:2;index:idx_career_preparation_scope,priority:2;size:512"`
	ApplicationID       string    `gorm:"index:idx_career_preparation_scope,priority:3;size:36"`
	RequestID           string    `gorm:"size:128;uniqueIndex:career_preparation_scope_request,priority:3;not null"`
	Fingerprint         string    `gorm:"size:64;not null"`
	Focus               string    `gorm:"size:32;not null"`
	Status              string    `gorm:"size:16;not null"`
	SubmissionID        string    `gorm:"size:36;not null;default:''"`
	SubmittedMaterialID string    `gorm:"size:36;not null;default:''"`
	SubmittedExportID   string    `gorm:"size:36;not null;default:''"`
	SubmittedVersion    uint64    `gorm:"not null;default:0"`
	SubmittedDigest     string    `gorm:"size:64;not null;default:''"`
	SnapshotID          string    `gorm:"size:36;not null;default:''"`
	SnapshotSHA256      string    `gorm:"size:64;not null;default:''"`
	ProfileRevision     uint64    `gorm:"not null;default:0"`
	MaterialID          string    `gorm:"size:36;not null;default:''"`
	FailureCode         string    `gorm:"size:64;not null;default:''"`
	FailureMessage      string    `gorm:"type:text;not null;default:''"`
	ReceiptBody         string    `gorm:"type:text;not null;default:''"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

func (preparationRecord) TableName() string { return "career_preparations" }

// PreparationGenerationRequest carries the durable evidence to the
// generation seam: the anchor, the exact submitted body, the frozen snapshot,
// and the confirmed facts.
type PreparationGenerationRequest struct {
	Focus          string
	ApplicationID  string
	Anchor         PreparationAnchor
	SubmittedBody  MaterialBody
	Snapshot       PreparationSnapshotRef
	ConfirmedFacts []Fact
}

// PreparationGenerator is the injected generation seam. Production wires the
// deterministic local composer below — there is no external LLM dependency
// on the production path; richer generators are plugged in explicitly.
type PreparationGenerator interface {
	GeneratePreparation(ctx context.Context, req PreparationGenerationRequest) (MaterialBody, error)
}

// SetPreparationGenerator replaces the generation seam (tests and explicit
// production wiring only).
func (o *Office) SetPreparationGenerator(generator PreparationGenerator) {
	o.preparationGenerator = generator
}

// GeneratePreparation anchors to the application's actually submitted
// version, composes the draft through the injected seam, and materializes it
// as a draft material pinned to the application's frozen snapshot. An
// unconfirmed or missing submission answers the typed prompt state instead
// of guessing; a generation failure keeps the request recoverable.
func (o *Office) GeneratePreparation(ctx context.Context, input GeneratePreparationInput) (PreparationReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return PreparationReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		if isSQLiteBusy(err) {
			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return PreparationReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.ApplicationID = strings.TrimSpace(input.ApplicationID)
	input.Focus = strings.TrimSpace(input.Focus)
	if input.RequestID == "" || len(input.RequestID) > maxPreparationRequestIDLen ||
		input.ApplicationID == "" || len(input.ApplicationID) > 36 || !preparationFocuses[input.Focus] {
		return PreparationReceipt{}, ErrInvalidRequest
	}

	// The application must exist under the authenticated scope before any
	// version resolution: foreign applications answer not-found.
	var application applicationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, input.ApplicationID).
		First(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PreparationReceipt{}, ErrApplicationNotFound
	}
	if err != nil {
		return PreparationReceipt{}, err
	}

	// Version anchoring: only a confirmed submission binds a version. An
	// unconfirmed or missing submission is the typed prompt state — the
	// latest material version is never guessed.
	var submission submissionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, input.ApplicationID).
		First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PreparationReceipt{}, &PreparationVersionUnknownError{ApplicationID: input.ApplicationID, SubmissionRecorded: false}
	}
	if err != nil {
		return PreparationReceipt{}, err
	}
	if !submission.VersionConfirmed {
		return PreparationReceipt{}, &PreparationVersionUnknownError{ApplicationID: input.ApplicationID, SubmissionRecorded: true}
	}
	anchor := PreparationAnchor{
		SubmissionID:  submission.ID,
		MaterialID:    submission.MaterialID,
		ExportID:      submission.ExportID,
		Version:       submission.Version,
		ContentDigest: submission.ContentDigest,
	}

	// The submitted version body resolves exactly what was sent; the frozen
	// snapshot cites the job evidence the application pinned.
	submittedView, err := o.MaterialVersion(ctx, anchor.MaterialID, anchor.Version)
	if err != nil {
		return PreparationReceipt{}, err
	}
	var snapshot opportunitySnapshot
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, application.SnapshotID).
		First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PreparationReceipt{}, ErrOpportunityNotFound
	}
	if err != nil {
		return PreparationReceipt{}, err
	}
	snapshotRef := PreparationSnapshotRef{
		OpportunityID:  snapshot.OpportunityID,
		SnapshotID:     snapshot.ID,
		SnapshotSHA256: snapshot.RawSHA256,
	}

	headRevision, err := materialHeadRevision(o.db.WithContext(ctx), s)
	if err != nil {
		return PreparationReceipt{}, err
	}
	facts, err := confirmedFactsAtRevision(o.db.WithContext(ctx), s, headRevision)
	if err != nil {
		return PreparationReceipt{}, err
	}
	// The fingerprint covers the full intent: the anchor makes the resolved
	// version part of the idempotency contract.
	fingerprint, err := materialFingerprint("generate_preparation", input.RequestID, input.ApplicationID,
		input.Focus, anchor, snapshotRef, input.ExpectedRevision)
	if err != nil {
		return PreparationReceipt{}, err
	}

	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for attempt := 0; ; attempt++ {
		reserved, txErr := o.attemptPreparationReserve(operationCtx, s, input, anchor, snapshotRef, headRevision, fingerprint)
		if txErr == nil {
			if reserved.ReceiptBody != "" {
				var stored PreparationReceipt
				if err = decodePreparationReceipt(reserved.ReceiptBody, &stored); err != nil {
					return PreparationReceipt{}, err
				}
				return stored, nil
			}
			break
		}
		if errors.Is(txErr, ErrIdempotencyConflict) || errors.Is(txErr, ErrInvalidRequest) ||
			errors.Is(txErr, ErrApplicationNotFound) {
			return PreparationReceipt{}, txErr
		}
		var revisionConflict *RevisionConflictError
		if errors.As(txErr, &revisionConflict) {
			return PreparationReceipt{}, txErr
		}
		if operationCtx.Err() != nil || ctx.Err() != nil {
			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if isSQLiteBusy(txErr) && attempt < progressWriteBusyRetries {
			time.Sleep(progressWriteBackoff)
			continue
		}
		if isSQLiteBusy(txErr) {
			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		stored, _, resolveErr := o.resolvePreparationReserveError(operationCtx, s, input, fingerprint, txErr)
		if resolveErr != nil {
			return PreparationReceipt{}, resolveErr
		}
		return stored, nil
	}

	// The generation seam runs outside any transaction: it may be slow and
	// never holds database locks. Production wires the deterministic local
	// composer; nothing here performs an external action.
	generator := o.preparationGenerator
	if generator == nil {
		generator = deterministicPreparationGenerator{}
	}
	body, genErr := generator.GeneratePreparation(operationCtx, PreparationGenerationRequest{
		Focus:          input.Focus,
		ApplicationID:  input.ApplicationID,
		Anchor:         anchor,
		SubmittedBody:  submittedView.Body,
		Snapshot:       snapshotRef,
		ConfirmedFacts: facts,
	})
	if genErr != nil {
		if persistErr := o.persistPreparationFailure(operationCtx, s, input.RequestID, fingerprint,
			PreparationFailureGeneration, genErr.Error()); persistErr != nil {
			return PreparationReceipt{}, persistErr
		}
		return PreparationReceipt{}, fmt.Errorf("%w: %s", ErrPreparationGenerationFailed, PreparationFailureGeneration)
	}
	if shapeErr := validateMaterialShape(body); shapeErr != nil {
		if persistErr := o.persistPreparationFailure(operationCtx, s, input.RequestID, fingerprint,
			PreparationFailureGeneration, shapeErr.Error()); persistErr != nil {
			return PreparationReceipt{}, persistErr
		}
		return PreparationReceipt{}, fmt.Errorf("%w: %s", ErrPreparationGenerationFailed, PreparationFailureGeneration)
	}

	return o.finalizePreparation(operationCtx, s, input, anchor, snapshotRef, headRevision, fingerprint, body)
}

// attemptPreparationReserve performs the replay check, the profile
// expectedRevision CAS, and the durable reservation of the request. An empty
// ReceiptBody on the returned row marks a failed or in-flight row the same
// intent may retry.
func (o *Office) attemptPreparationReserve(ctx context.Context, s Scope, input GeneratePreparationInput,
	anchor PreparationAnchor, snapshotRef PreparationSnapshotRef, headRevision uint64, fingerprint string) (preparationRecord, error) {
	var row preparationRecord
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&row).Error
		if e == nil {
			if row.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			if row.ReceiptBody != "" {
				return nil
			}
			// A failed or interrupted generation under the same intent stays
			// recoverable: refresh the reservation and re-run.
			return tx.Model(&preparationRecord{}).
				Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
				Updates(map[string]any{
					"status":                PreparationStatusGenerating,
					"failure_code":          "",
					"failure_message":       "",
					"submission_id":         anchor.SubmissionID,
					"submitted_material_id": anchor.MaterialID,
					"submitted_export_id":   anchor.ExportID,
					"submitted_version":     anchor.Version,
					"submitted_digest":      anchor.ContentDigest,
					"snapshot_id":           snapshotRef.SnapshotID,
					"snapshot_sha256":       snapshotRef.SnapshotSHA256,
					"profile_revision":      headRevision,
					"updated_at":            time.Now().UTC(),
				}).Error
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
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
		now := time.Now().UTC()
		row = preparationRecord{
			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
			ApplicationID: input.ApplicationID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Focus: input.Focus,
			Status:              PreparationStatusGenerating,
			SubmissionID:        anchor.SubmissionID,
			SubmittedMaterialID: anchor.MaterialID, SubmittedExportID: anchor.ExportID,
			SubmittedVersion: anchor.Version, SubmittedDigest: anchor.ContentDigest,
			SnapshotID: snapshotRef.SnapshotID, SnapshotSHA256: snapshotRef.SnapshotSHA256,
			ProfileRevision: headRevision,
			CreatedAt:       now, UpdatedAt: now,
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return preparationRecord{}, err
	}
	return row, nil
}

// resolvePreparationReserveError settles a failed reserve attempt that is
// not a typed rejection: the losing side of a same-request-ID Create race
// (unique-index violation) reconciles through the winner's durable row
// instead of leaking the raw database error as a 500 (ocr3-129). A stored
// receipt replays; a missing or still-generating row leaves the outcome
// undecided and answers OutcomeUnknown.
func (o *Office) resolvePreparationReserveError(ctx context.Context, s Scope, input GeneratePreparationInput, fingerprint string, txErr error) (PreparationReceipt, bool, error) {
	if !isReceiptRaceError(txErr) {
		return PreparationReceipt{}, false, txErr
	}
	stored, found, lookupErr := o.replayPreparationReceipt(ctx, s, input.RequestID, fingerprint)
	if lookupErr != nil {
		return PreparationReceipt{}, false, lookupErr
	}
	if found {
		return stored, true, nil
	}
	return PreparationReceipt{}, false, &OutcomeUnknownError{RequestID: input.RequestID}
}

// persistPreparationFailure records the typed failure without publishing any
// product: the request row survives with its reason for a same-ID retry.
func (o *Office) persistPreparationFailure(ctx context.Context, s Scope, requestID, fingerprint, code, message string) error {
	return o.db.WithContext(ctx).Model(&preparationRecord{}).
		Where("tenant_id=? AND user_id=? AND request_id=? AND fingerprint=?", s.TenantID, s.UserID, requestID, fingerprint).
		Updates(map[string]any{
			"status":          PreparationStatusFailed,
			"failure_code":    code,
			"failure_message": message,
			"updated_at":      time.Now().UTC(),
		}).Error
}

// finalizePreparation revalidates the composed body against the confirmed
// facts inside the write transaction, materializes the draft material, and
// stamps the receipt. A body that references an unconfirmed claim is a typed
// generation failure — never a persisted draft.
func (o *Office) finalizePreparation(ctx context.Context, s Scope, input GeneratePreparationInput,
	anchor PreparationAnchor, snapshotRef PreparationSnapshotRef, headRevision uint64,
	fingerprint string, body MaterialBody) (PreparationReceipt, error) {
	var receipt PreparationReceipt
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row preparationRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrIdempotencyConflict
		}
		if e != nil {
			return e
		}
		if row.Fingerprint != fingerprint {
			return ErrIdempotencyConflict
		}
		if row.ReceiptBody != "" {
			return decodePreparationReceipt(row.ReceiptBody, &receipt)
		}
		// The same request ID may not also own a material from another seam.
		var existing materialRecord
		e = tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&existing).Error
		if e == nil {
			return ErrIdempotencyConflict
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		// Claims are revalidated under the lock against the current head:
		// confirmations only add facts, so carried claims stay confirmed.
		currentHead, headErr := materialHeadRevision(tx, s)
		if headErr != nil {
			return headErr
		}
		if claimErr := validateMaterialClaims(tx, s, currentHead, body); claimErr != nil {
			return fmt.Errorf("%w: %v", errPreparationClaimRefused, claimErr)
		}
		now := time.Now().UTC()
		pin := MaterialEvidencePin{
			OpportunityID:   snapshotRef.OpportunityID,
			SnapshotID:      snapshotRef.SnapshotID,
			SnapshotSHA256:  snapshotRef.SnapshotSHA256,
			ProfileRevision: headRevision,
		}
		materialID := uuid.NewString()
		materialReceipt, buildErr := buildMaterialReceipt(MaterialKindEdited, input.RequestID, materialID, MaterialStatusDraft, 0, pin, body)
		if buildErr != nil {
			return buildErr
		}
		if e = tx.Create(&materialRecord{
			ID: materialID, TenantID: s.TenantID, UserID: s.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint,
			OpportunityID: pin.OpportunityID, SnapshotID: pin.SnapshotID,
			ProfileRevision: pin.ProfileRevision,
			EvidenceBody:    string(mustJSON(pin)),
			Status:          MaterialStatusDraft,
			DraftBody:       string(mustJSON(materialReceipt.Body)),
			ReceiptBody:     string(mustJSON(materialReceipt)),
			CreatedAt:       now, UpdatedAt: now,
		}).Error; e != nil {
			return e
		}
		receipt = PreparationReceipt{
			Kind:          PreparationKindGenerated,
			RequestID:     input.RequestID,
			ApplicationID: input.ApplicationID,
			PreparationID: row.ID,
			Focus:         input.Focus,
			Status:        PreparationStatusDraft,
			Anchor:        anchor,
			MaterialID:    materialID,
			Body:          body,
			ReviewRisks:   materialReviewRisks(body),
			Sources: PreparationSources{
				SubmittedVersion: anchor,
				Snapshot:         snapshotRef,
				FactKeys:         preparationFactKeys(body),
				ProfileRevision:  headRevision,
			},
			Revision: headRevision,
		}
		receipt.CreatedAt = now
		if e = tx.Model(&preparationRecord{}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			Updates(map[string]any{
				"status":       PreparationStatusDraft,
				"material_id":  materialID,
				"receipt_body": string(mustJSON(receipt)),
				"updated_at":   now,
			}).Error; e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errPreparationClaimRefused) {
			// The refused body rolled back; the typed failure state persists
			// in its own write so the request stays readable and recoverable.
			if persistErr := o.persistPreparationFailure(ctx, s, input.RequestID, fingerprint,
				MaterialFailureClaimUnconfirmed, err.Error()); persistErr != nil {
				return PreparationReceipt{}, persistErr
			}
			return PreparationReceipt{}, fmt.Errorf("%w: %s", ErrPreparationGenerationFailed, MaterialFailureClaimUnconfirmed)
		}
		if errors.Is(err, ErrIdempotencyConflict) {
			return PreparationReceipt{}, err
		}
		if isReceiptRaceError(err) {
			// A concurrent writer under the same request ID decided the
			// outcome; the stored receipt is the truth. With no durable
			// decision observable, the outcome stays unknown instead of
			// leaking the raw race error (ocr3-129).
			stored, found, lookupErr := o.replayPreparationReceipt(ctx, s, input.RequestID, fingerprint)
			if lookupErr != nil {
				return PreparationReceipt{}, lookupErr
			}
			if found {
				return stored, nil
			}
			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if ctx.Err() != nil {
			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return PreparationReceipt{}, err
	}
	return receipt, nil
}

// FindPreparationReceipt replays the stored preparation receipt by request
// ID. A failed or interrupted generation answers its typed failure state —
// the request is preserved and never a blank success product.
func (o *Office) FindPreparationReceipt(ctx context.Context, requestID string) (PreparationReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return PreparationReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return PreparationReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxPreparationRequestIDLen {
		return PreparationReceipt{}, ErrInvalidRequest
	}
	var row preparationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PreparationReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return PreparationReceipt{}, err
	}
	if row.ReceiptBody != "" {
		var receipt PreparationReceipt
		if err = decodePreparationReceipt(row.ReceiptBody, &receipt); err != nil {
			return PreparationReceipt{}, err
		}
		return receipt, nil
	}
	return preparationFailureReceipt(row), nil
}

// ApplicationPreparations lists the preparations of one application under
// the authenticated scope, failures included.
func (o *Office) ApplicationPreparations(ctx context.Context, applicationID string) ([]PreparationReceipt, error) {
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
	var rows []preparationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, applicationID).
		Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]PreparationReceipt, 0, len(rows))
	for _, row := range rows {
		if row.ReceiptBody != "" {
			var receipt PreparationReceipt
			if err = decodePreparationReceipt(row.ReceiptBody, &receipt); err != nil {
				return nil, err
			}
			out = append(out, receipt)
			continue
		}
		out = append(out, preparationFailureReceipt(row))
	}
	return out, nil
}

// preparationFailureReceipt builds the typed failure state of a row without
// a stored receipt: the request is visible with its reason, and the body
// stays empty — no blank success product exists.
func preparationFailureReceipt(row preparationRecord) PreparationReceipt {
	anchor := PreparationAnchor{
		SubmissionID:  row.SubmissionID,
		MaterialID:    row.SubmittedMaterialID,
		ExportID:      row.SubmittedExportID,
		Version:       row.SubmittedVersion,
		ContentDigest: row.SubmittedDigest,
	}
	return PreparationReceipt{
		Kind:          PreparationKindGenerated,
		RequestID:     row.RequestID,
		ApplicationID: row.ApplicationID,
		PreparationID: row.ID,
		Focus:         row.Focus,
		Status:        row.Status,
		Anchor:        anchor,
		Body:          MaterialBody{},
		ReviewRisks:   []MaterialReviewRisk{},
		Sources: PreparationSources{
			SubmittedVersion: anchor,
			Snapshot: PreparationSnapshotRef{
				SnapshotID:     row.SnapshotID,
				SnapshotSHA256: row.SnapshotSHA256,
			},
			ProfileRevision: row.ProfileRevision,
		},
		FailureCode:    row.FailureCode,
		FailureMessage: row.FailureMessage,
		Revision:       row.ProfileRevision,
		CreatedAt:      row.CreatedAt,
	}
}

func (o *Office) replayPreparationReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (PreparationReceipt, bool, error) {
	var row preparationRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PreparationReceipt{}, false, nil
	}
	if err != nil {
		return PreparationReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return PreparationReceipt{}, true, ErrIdempotencyConflict
	}
	if row.ReceiptBody == "" {
		return PreparationReceipt{}, false, nil
	}
	var receipt PreparationReceipt
	if err = decodePreparationReceipt(row.ReceiptBody, &receipt); err != nil {
		return PreparationReceipt{}, true, err
	}
	return receipt, true, nil
}

// preparationFactKeys collects the confirmed fact keys the draft cites,
// sorted for deterministic receipts.
func preparationFactKeys(body MaterialBody) []string {
	keys := map[string]bool{}
	for _, section := range body.Sections {
		for _, claim := range section.Claims {
			if claim.FactKey != "" {
				keys[claim.FactKey] = true
			}
		}
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func decodePreparationReceipt(body string, receipt *PreparationReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career preparation receipt: %w", err)
	}
	return nil
}

// deterministicPreparationGenerator is the production generation seam: a
// local, deterministic composer with no external LLM dependency. It carries
// the submitted version's claims (already fact-validated at their
// confirmation) into the focus draft and keeps needs_review placeholders
// exactly as placeholders — nothing is fabricated and nothing is promised.
type deterministicPreparationGenerator struct{}

func (deterministicPreparationGenerator) GeneratePreparation(_ context.Context, req PreparationGenerationRequest) (MaterialBody, error) {
	heading, lead := preparationLead(req.Focus)
	sections := make([]MaterialSection, 0, len(req.SubmittedBody.Sections))
	for index, source := range req.SubmittedBody.Sections {
		title := heading
		if source.Heading != "" {
			title = truncatePreparationHeading(heading + "：" + source.Heading)
		} else if len(req.SubmittedBody.Sections) > 1 {
			title = truncatePreparationHeading(fmt.Sprintf("%s（%d）", heading, index+1))
		}
		claims := make([]MaterialClaim, len(source.Claims))
		copy(claims, source.Claims)
		sections = append(sections, MaterialSection{
			Heading: title,
			Content: lead,
			Claims:  claims,
		})
	}
	if len(sections) == 0 {
		// A submitted version always carries at least one section; the guard
		// keeps the composer honest if that invariant ever changes.
		sections = append(sections, MaterialSection{
			Heading: heading,
			Content: lead,
			Claims:  nil,
		})
	}
	return MaterialBody{Sections: sections}, nil
}

func preparationLead(focus string) (heading, lead string) {
	if focus == PreparationFocusInterview {
		return "面试准备（草稿）",
			"以下要点固定自实际投递版本与岗位快照，仅引用已确认事实；缺失信息以待补充占位呈现，由本人审阅补充。"
	}
	return "求职信（草稿）",
		"本求职信草稿固定自实际投递版本与岗位快照，仅引用已确认事实；缺失信息以待补充占位呈现，由本人审阅补充，不代为承诺。"
}

// truncatePreparationHeading bounds a composed heading to the material
// section byte budget without ever splitting a multi-byte rune: the lead
// string is Chinese and source headings routinely carry CJK text, so a raw
// byte slice lands mid-character and json.Marshal would persist the U+FFFD
// replacement into a heading that is frozen into immutable versions.
func truncatePreparationHeading(heading string) string {
	if len(heading) <= maxMaterialHeadingBytes {
		return heading
	}
	var truncated strings.Builder
	for _, r := range heading {
		size := utf8.RuneLen(r)
		if size < 0 {
			size = utf8.UTFMax // an invalid sequence encodes as U+FFFD
		}
		if truncated.Len()+size > maxMaterialHeadingBytes {
			break
		}
		truncated.WriteRune(r)
	}
	return truncated.String()
}
