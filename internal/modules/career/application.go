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

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Application link states. `linking` is the durable pre-external-call state;
// `ready` means Workbench holds the task; `link_failed` is a definite
// rejection that still preserves the application row.
const (
	ApplicationLinkStateLinking = "linking"
	ApplicationLinkStateReady   = "ready"
	ApplicationLinkStateFailed  = "link_failed"
)

// maxApplicationRequestIDLen matches the Workbench durable projection width
// (workbench_application_tasks.origin_request_id and agent_runs.request_id
// are VARCHAR(64)): a longer request ID can never be linked, so it is
// rejected as an invalid request up front instead of deterministically
// failing after the application row is committed.
const maxApplicationRequestIDLen = 64

var (
	ErrApplicationNotFound       = errors.New("career application not found")
	ErrApplicationConflict       = errors.New("career application conflict")
	ErrApplicationHardIneligible = errors.New("career application requires explicit continuation after a hard-ineligible evaluation")
	ErrApplicationLinkerMissing  = errors.New("career application linker unavailable")
)

type CreateApplicationInput struct {
	RequestID                  string `json:"requestId"`
	OpportunityID              string `json:"opportunityId"`
	SnapshotID                 string `json:"snapshotId"`
	EvaluationID               string `json:"evaluationId"`
	BatchIdentity              string `json:"batchIdentity"`
	ContinueDespiteHardFailure bool   `json:"continueDespiteHardFailure"`
	ExpectedRevision           uint64 `json:"expectedRevision"`
}

type ApplicationReceipt struct {
	ApplicationID  string                 `json:"applicationId"`
	RequestID      string                 `json:"requestId"`
	LinkState      string                 `json:"linkState"`
	TaskID         string                 `json:"taskId,omitempty"`
	RunID          string                 `json:"runId,omitempty"`
	Qualified      bool                   `json:"qualified"`
	Warning        map[string]any         `json:"warning,omitempty"`
	PinnedEvidence ApplicationEvidencePin `json:"pinnedEvidence"`
}

type ApplicationEvidencePin struct {
	OpportunityID    string `json:"opportunityId"`
	SnapshotID       string `json:"snapshotId"`
	EvaluationID     string `json:"evaluationId"`
	ProfileRevision  uint64 `json:"profileRevision"`
	EvaluationStatus string `json:"evaluationStatus"`
	BatchIdentity    string `json:"batchIdentity"`
}

type applicationRecord struct {
	ID                         string    `gorm:"primaryKey;size:36"`
	TenantID                   uint64    `gorm:"uniqueIndex:career_application_scope_request;uniqueIndex:career_application_scope_job_batch,priority:1;index:idx_career_application_scope;index:idx_career_application_link_state,priority:1"`
	UserID                     string    `gorm:"uniqueIndex:career_application_scope_request;uniqueIndex:career_application_scope_job_batch,priority:2;index:idx_career_application_scope;index:idx_career_application_link_state,priority:2;size:512"`
	RequestID                  string    `gorm:"uniqueIndex:career_application_scope_request;size:128"`
	Fingerprint                string    `gorm:"size:64;not null"`
	OpportunityID              string    `gorm:"size:36;not null;uniqueIndex:career_application_scope_job_batch,priority:3"`
	SnapshotID                 string    `gorm:"size:36;not null"`
	EvaluationID               string    `gorm:"size:36;not null"`
	ProfileRevision            uint64    `gorm:"not null"`
	EvidenceBody               string    `gorm:"type:text;not null"`
	BatchIdentity              string    `gorm:"size:255;not null;uniqueIndex:career_application_scope_job_batch,priority:4"`
	ContinueDespiteHardFailure bool      `gorm:"not null"`
	EvaluationStatus           string    `gorm:"size:32;not null"`
	Qualified                  bool      `gorm:"not null"`
	WarningBody                string    `gorm:"type:text;not null"`
	LinkState                  string    `gorm:"size:32;not null;index:idx_career_application_link_state,priority:3"`
	TaskID                     string    `gorm:"size:36;not null;default:''"`
	RunID                      string    `gorm:"size:64;not null;default:''"`
	ReceiptBody                string    `gorm:"type:text;not null"`
	CreatedAt                  time.Time `gorm:"not null"`
	UpdatedAt                  time.Time `gorm:"not null"`
}

func (applicationRecord) TableName() string { return "career_applications" }

// SetApplicationTaskLinker binds the Workbench boundary. The linker is the
// only channel through which Career may create or find durable tasks.
func (o *Office) SetApplicationTaskLinker(linker interfaces.CareerApplicationTaskLinker) {
	o.linker = linker
}

func canonicalApplicationBatchIdentity(batch string) string {
	return strings.ToLower(strings.Join(strings.Fields(batch), " "))
}

func applicationTaskTitle(opportunityID string) string {
	return "Career application " + opportunityID
}

// CreateApplication pins immutable evidence (opportunity snapshot, evaluation,
// profile revision, batch identity) and links exactly one durable Workbench
// task per job and batch. The Career row with link_state=linking is committed
// before any external call; the Workbench linker is invoked only after that
// commit, always with the original request ID.
func (o *Office) CreateApplication(ctx context.Context, input CreateApplicationInput) (ApplicationReceipt, error) {
	o.lifecycleMu.RLock()
	defer o.lifecycleMu.RUnlock()
	scope, err := getScope(ctx)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return ApplicationReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.OpportunityID = strings.TrimSpace(input.OpportunityID)
	input.SnapshotID = strings.TrimSpace(input.SnapshotID)
	input.EvaluationID = strings.TrimSpace(input.EvaluationID)
	input.BatchIdentity = canonicalApplicationBatchIdentity(input.BatchIdentity)
	if input.RequestID == "" || len(input.RequestID) > maxApplicationRequestIDLen ||
		input.OpportunityID == "" || input.SnapshotID == "" || input.EvaluationID == "" ||
		input.BatchIdentity == "" || len(input.BatchIdentity) > 255 {
		return ApplicationReceipt{}, ErrInvalidRequest
	}
	if o.linker == nil {
		return ApplicationReceipt{}, ErrApplicationLinkerMissing
	}
	intentBytes, err := json.Marshal(input)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	intentSum := sha256.Sum256(intentBytes)
	fingerprint := hex.EncodeToString(intentSum[:])

	var receipt ApplicationReceipt
	// resolvedOpportunityID is filled inside the transaction with the
	// snapshot's canonical owner; post-transaction fallbacks reuse it.
	resolvedOpportunityID := input.OpportunityID
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Exact replay precedes every side effect: a stored request ID with the
		// same fingerprint returns the stored state, different content conflicts.
		var prior applicationRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, input.RequestID).
			First(&prior).Error
		if e == nil {
			if prior.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			// A replay of an undecided (linking) row re-enters the linker
			// below; the intent title must again carry the canonical owner.
			// The stored row pins it (merges migrate applications onto the
			// merge target), so reusing the raw input ID here would replay a
			// different title than the first attempt and be rejected.
			resolvedOpportunityID = prior.OpportunityID
			return decodeApplicationReceipt(prior.ReceiptBody, &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		// The deletion fence runs before the first Career commit: a fresh
		// application must never strand a linking row the claim admission
		// below would immediately reject.
		if e := requireGateOpenTx(tx, scope); e != nil {
			return e
		}

		// Resolve the opportunity through the merge chain first: after a
		// reconciliation merged the input ID away, its snapshots, evaluations,
		// and applications live under the canonical owner (T12).
		resolvedOpportunityID = canonicalOpportunityID(tx, scope, input.OpportunityID)

		var snapshot opportunitySnapshot
		e = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?",
			scope.TenantID, scope.UserID, input.OpportunityID, input.SnapshotID).First(&snapshot).Error
		if errors.Is(e, gorm.ErrRecordNotFound) && resolvedOpportunityID != input.OpportunityID {
			e = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?",
				scope.TenantID, scope.UserID, resolvedOpportunityID, input.SnapshotID).First(&snapshot).Error
		}
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrOpportunityNotFound
		}
		if e != nil {
			return e
		}
		resolvedOpportunityID = snapshot.OpportunityID
		var evaluation evaluationRecord
		e = tx.Where("tenant_id=? AND user_id=? AND id=?", scope.TenantID, scope.UserID, input.EvaluationID).
			First(&evaluation).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrEvaluationNotFound
		}
		if e != nil {
			return e
		}
		// An evaluation created before a merge still names the merged-away
		// opportunity. Resolve its owner through the merge chain before
		// comparing: merges now migrate evaluation rows, but evaluations
		// merged under older builds must also self-heal here instead of
		// becoming permanently unusable for application creation (ocr3-016).
		if canonicalOpportunityID(tx, scope, evaluation.OpportunityID) != snapshot.OpportunityID ||
			evaluation.SnapshotID != input.SnapshotID {
			return ErrInvalidRequest
		}

		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", scope.TenantID, scope.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return &RevisionConflictError{CurrentRevision: 0}
		}
		if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}

		var evaluationReceipt EvaluationReceipt
		if e = json.Unmarshal([]byte(evaluation.ReceiptBody), &evaluationReceipt); e != nil {
			return e
		}

		qualified := true
		var warning map[string]any
		if evaluationReceipt.Status == EvaluationIneligible {
			if !input.ContinueDespiteHardFailure {
				return ErrApplicationHardIneligible
			}
			qualified = false
			warning = applicationHardWarning(input.EvaluationID, evaluation.EvaluationBody)
		}

		// One job and batch admits exactly one application. The check runs
		// against the canonical owner because merges migrate earlier
		// application rows onto the merge target.
		var sameBatch applicationRecord
		e = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND batch_identity=?",
			scope.TenantID, scope.UserID, snapshot.OpportunityID, input.BatchIdentity).
			First(&sameBatch).Error
		if e == nil {
			return ErrApplicationConflict
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}

		applicationID := uuid.NewString()
		pin := ApplicationEvidencePin{
			OpportunityID:    snapshot.OpportunityID,
			SnapshotID:       input.SnapshotID,
			EvaluationID:     input.EvaluationID,
			ProfileRevision:  head.Revision,
			EvaluationStatus: evaluationReceipt.Status,
			BatchIdentity:    input.BatchIdentity,
		}
		evidenceBody, e := json.Marshal(pin)
		if e != nil {
			return e
		}
		warningBody := ""
		if warning != nil {
			body, e := json.Marshal(warning)
			if e != nil {
				return e
			}
			warningBody = string(body)
		}
		receipt = ApplicationReceipt{
			ApplicationID:  applicationID,
			RequestID:      input.RequestID,
			LinkState:      ApplicationLinkStateLinking,
			Qualified:      qualified,
			Warning:        warning,
			PinnedEvidence: pin,
		}
		receiptBody, e := json.Marshal(receipt)
		if e != nil {
			return e
		}
		now := time.Now().UTC()
		record := applicationRecord{
			ID: applicationID, TenantID: scope.TenantID, UserID: scope.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint,
			// The row belongs to the canonical owner so the one-job-one-batch
			// uniqueness holds across merges; the pinned evidence above keeps
			// the reference the caller actually used.
			OpportunityID: snapshot.OpportunityID, SnapshotID: input.SnapshotID,
			EvaluationID: input.EvaluationID, ProfileRevision: head.Revision,
			EvidenceBody: string(evidenceBody), BatchIdentity: input.BatchIdentity,
			ContinueDespiteHardFailure: input.ContinueDespiteHardFailure,
			EvaluationStatus:           evaluationReceipt.Status, Qualified: qualified,
			WarningBody: warningBody, LinkState: ApplicationLinkStateLinking,
			ReceiptBody: string(receiptBody), CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&record).Error
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrApplicationConflict) ||
			errors.Is(err, ErrApplicationHardIneligible) || errors.Is(err, ErrInvalidRequest) ||
			errors.Is(err, ErrOpportunityNotFound) || errors.Is(err, ErrEvaluationNotFound) ||
			errors.Is(err, ErrApplicationLinkerMissing) {
			return ApplicationReceipt{}, err
		}
		var revisionConflict *RevisionConflictError
		if errors.As(err, &revisionConflict) {
			return ApplicationReceipt{}, err
		}
		if isReceiptRaceError(err) {
			// A concurrent writer may have committed this request (identical or
			// different content) or the same job/batch. Re-read under the same
			// scope to decide; only the stored state is ever returned.
			replay, found, lookupErr := o.replayApplication(ctx, scope, input.RequestID, fingerprint)
			if lookupErr != nil {
				return ApplicationReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
			var occupied applicationRecord
			if e := o.db.WithContext(ctx).
				Where("tenant_id=? AND user_id=? AND opportunity_id=? AND batch_identity=?",
					scope.TenantID, scope.UserID, resolvedOpportunityID, input.BatchIdentity).
				First(&occupied).Error; e == nil {
				return ApplicationReceipt{}, ErrApplicationConflict
			}
		}
		if isSQLiteBusy(err) {
			// A writer-lock failure rolled the whole transaction back and the
			// replay/occupied lookups above cannot see a committed row, so the
			// outcome is unknown but retry-safe: the same request ID replays
			// idempotently instead of surfacing a raw "database is locked".
			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if ctx.Err() != nil {
			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ApplicationReceipt{}, err
	}
	if receipt.LinkState != ApplicationLinkStateLinking || receipt.ApplicationID == "" {
		// Exact replay of an already-linked (or failed) application: the stored
		// state is final for this request ID.
		var existing lifecycleClaim
		if lookup := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", scope.TenantID, scope.UserID, "application_link", input.RequestID).First(&existing).Error; lookup == nil {
			ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, scope, "application_link", input.RequestID, fingerprint)
			if claimErr != nil {
				return ApplicationReceipt{}, claimErr
			}
			defer unlockAttempt()
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", input.RequestID, ownerToken); releaseErr != nil {
				return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
		}
		return receipt, nil
	}
	ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, scope, "application_link", input.RequestID, fingerprint)
	if claimErr != nil {
		return ApplicationReceipt{}, claimErr
	}
	defer unlockAttempt()

	// External call happens strictly after the Career commit and always with
	// the original request ID; an unknown outcome keeps the linking state.
	link, ensureErr := o.linker.EnsureCareerApplicationTask(ctx, scope.TenantID, scope.UserID, interfaces.CareerApplicationTaskIntent{
		ApplicationID: receipt.ApplicationID,
		RequestID:     input.RequestID,
		Title:         applicationTaskTitle(resolvedOpportunityID),
	})
	if ensureErr == nil {
		updated, updateErr := o.updateApplicationLink(ctx, scope, input.RequestID, ApplicationLinkStateReady, link)
		if updateErr == nil {
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", input.RequestID, ownerToken); releaseErr != nil {
				return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
			return updated, nil
		}
		return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	if errors.Is(ensureErr, interfaces.ErrCareerApplicationTaskConflict) {
		if _, failErr := o.updateApplicationLink(ctx, scope, input.RequestID, ApplicationLinkStateFailed, interfaces.CareerApplicationTaskLink{}); failErr == nil {
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", input.RequestID, ownerToken); releaseErr != nil {
				return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
			return ApplicationReceipt{}, fmt.Errorf("%w: workbench rejected the task link: %v", ErrApplicationConflict, ensureErr)
		}
		return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	if errors.Is(ensureErr, interfaces.ErrCareerApplicationTaskInvalid) {
		// A pure input-validation rejection is deterministic: no task was (or
		// ever will be) created for this intent, so the link is terminally
		// failed and the client sees an invalid request, not a conflict.
		if _, failErr := o.updateApplicationLink(ctx, scope, input.RequestID, ApplicationLinkStateFailed, interfaces.CareerApplicationTaskLink{}); failErr == nil {
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", input.RequestID, ownerToken); releaseErr != nil {
				return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
			return ApplicationReceipt{}, fmt.Errorf("%w: workbench rejected the task link: %v", ErrInvalidRequest, ensureErr)
		}
		return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	// Anything else — including ErrCareerApplicationTaskUndecided after the
	// linker's bounded race budget ran out — leaves the row in linking state:
	// a replay of the same request ID re-enters the linker and reconciles
	// against whatever became durable, instead of forking a terminal failure
	// away from a task that may already be ready.
	return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
}

// ReconcileApplicationLink resolves an undecided link without ever changing
// the pinned intent. It reuses the original request ID: if Workbench already
// holds the task, the same row becomes ready; if nothing durable exists the
// receipt still reports linking.
func (o *Office) ReconcileApplicationLink(ctx context.Context, requestID string) (ApplicationReceipt, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return ApplicationReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxApplicationRequestIDLen {
		return ApplicationReceipt{}, ErrInvalidRequest
	}
	if o.linker == nil {
		return ApplicationReceipt{}, ErrApplicationLinkerMissing
	}
	row, err := o.findApplicationByRequest(ctx, scope, requestID)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	var receipt ApplicationReceipt
	if err = decodeApplicationReceipt(row.ReceiptBody, &receipt); err != nil {
		return ApplicationReceipt{}, err
	}
	if row.LinkState == ApplicationLinkStateFailed {
		// link_failed is a definite rejection: reconcile never re-opens a
		// terminally failed link, and never re-associates the request ID
		// with whatever durable task a concurrent twin may have created.
		var existing lifecycleClaim
		if lookup := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", scope.TenantID, scope.UserID, "application_link", requestID).First(&existing).Error; lookup == nil {
			ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, scope, "application_link", requestID, row.Fingerprint)
			if claimErr != nil {
				return ApplicationReceipt{}, claimErr
			}
			defer unlockAttempt()
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", requestID, ownerToken); releaseErr != nil {
				return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
			}
		}
		return receipt, nil
	}
	ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, scope, "application_link", requestID, row.Fingerprint)
	if claimErr != nil {
		return ApplicationReceipt{}, claimErr
	}
	defer unlockAttempt()
	link, findErr := o.linker.FindCareerApplicationTask(ctx, scope.TenantID, scope.UserID, requestID)
	if findErr == nil {
		if link.ApplicationID != "" && link.ApplicationID != row.ID {
			// The durable task found under this request ID belongs to a
			// different application: the request ID is bound to foreign
			// content, which is a definite rejection for this row.
			if _, failErr := o.updateApplicationLink(ctx, scope, requestID, ApplicationLinkStateFailed, interfaces.CareerApplicationTaskLink{}); failErr == nil {
				if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", requestID, ownerToken); releaseErr != nil {
					return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
				}
				return ApplicationReceipt{}, fmt.Errorf(
					"%w: workbench task %s belongs to application %s",
					ErrApplicationConflict, link.TaskID, link.ApplicationID,
				)
			}
			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
		}
		updated, updateErr := o.updateApplicationLink(ctx, scope, requestID, ApplicationLinkStateReady, link)
		if updateErr != nil {
			return ApplicationReceipt{}, updateErr
		}
		if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", requestID, ownerToken); releaseErr != nil {
			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
		}
		return updated, nil
	}
	if errors.Is(findErr, interfaces.ErrCareerApplicationTaskNotFound) {
		if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), scope, "application_link", requestID, ownerToken); releaseErr != nil {
			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
		}
		return receipt, nil
	}
	return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: requestID}
}

// FindApplicationReceipt replays the stored application receipt by request ID.
func (o *Office) FindApplicationReceipt(ctx context.Context, requestID string) (ApplicationReceipt, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return ApplicationReceipt{}, err
	}
	row, err := o.findApplicationByRequest(ctx, scope, strings.TrimSpace(requestID))
	if err != nil {
		return ApplicationReceipt{}, err
	}
	var receipt ApplicationReceipt
	if err = decodeApplicationReceipt(row.ReceiptBody, &receipt); err != nil {
		return ApplicationReceipt{}, err
	}
	return receipt, nil
}

// Application returns the stored application state by its UUID.
func (o *Office) Application(ctx context.Context, applicationID string) (ApplicationReceipt, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return ApplicationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return ApplicationReceipt{}, err
	}
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" {
		return ApplicationReceipt{}, ErrInvalidRequest
	}
	var row applicationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", scope.TenantID, scope.UserID, applicationID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ApplicationReceipt{}, ErrApplicationNotFound
	}
	if err != nil {
		return ApplicationReceipt{}, err
	}
	var receipt ApplicationReceipt
	if err = decodeApplicationReceipt(row.ReceiptBody, &receipt); err != nil {
		return ApplicationReceipt{}, err
	}
	return receipt, nil
}

func (o *Office) findApplicationByRequest(ctx context.Context, scope Scope, requestID string) (applicationRecord, error) {
	var row applicationRecord
	if requestID == "" {
		return applicationRecord{}, ErrApplicationNotFound
	}
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return applicationRecord{}, ErrApplicationNotFound
	}
	if err != nil {
		return applicationRecord{}, err
	}
	return row, nil
}

func (o *Office) replayApplication(ctx context.Context, scope Scope, requestID, fingerprint string) (ApplicationReceipt, bool, error) {
	row, err := o.findApplicationByRequest(ctx, scope, requestID)
	if err != nil {
		if errors.Is(err, ErrApplicationNotFound) {
			return ApplicationReceipt{}, false, nil
		}
		return ApplicationReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return ApplicationReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt ApplicationReceipt
	if err = decodeApplicationReceipt(row.ReceiptBody, &receipt); err != nil {
		return ApplicationReceipt{}, true, err
	}
	return receipt, true, nil
}

// updateApplicationLink rewrites the mutable link columns of the one scoped
// row and its stored receipt. The pinned evidence columns are never touched.
func (o *Office) updateApplicationLink(
	ctx context.Context, scope Scope, requestID, linkState string, link interfaces.CareerApplicationTaskLink,
) (ApplicationReceipt, error) {
	var updated ApplicationReceipt
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row applicationRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).
			First(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		if e != nil {
			return e
		}
		var receipt ApplicationReceipt
		if e = decodeApplicationReceipt(row.ReceiptBody, &receipt); e != nil {
			return e
		}
		receipt.LinkState = linkState
		receipt.TaskID = link.TaskID
		receipt.RunID = link.RunID
		body, e := json.Marshal(receipt)
		if e != nil {
			return e
		}
		changes := map[string]any{
			"link_state":   linkState,
			"task_id":      link.TaskID,
			"run_id":       link.RunID,
			"receipt_body": string(body),
			"updated_at":   time.Now().UTC(),
		}
		if e = tx.Model(&applicationRecord{}).
			Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).
			Updates(changes).Error; e != nil {
			return e
		}
		updated = receipt
		// Test hook: simulate Career's own ready update failing after the
		// Workbench projection already exists.
		if linkState == ApplicationLinkStateReady && o.failApplicationReadyUpdate != nil {
			if hookErr := o.failApplicationReadyUpdate(); hookErr != nil {
				return hookErr
			}
		}
		return nil
	})
	if err != nil {
		return ApplicationReceipt{}, err
	}
	return updated, nil
}

func applicationHardWarning(evaluationID, evaluationBody string) map[string]any {
	warning := map[string]any{
		"evaluationId":     evaluationID,
		"evaluationStatus": EvaluationIneligible,
	}
	var evaluation Evaluation
	if err := json.Unmarshal([]byte(evaluationBody), &evaluation); err == nil && len(evaluation.Hard.Rules) > 0 {
		warning["hardRuleId"] = evaluation.Hard.Rules[0].RuleID
		warning["reasonCode"] = evaluation.Hard.Rules[0].ReasonCode
	}
	return warning
}

func decodeApplicationReceipt(body string, receipt *ApplicationReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career application receipt: %w", err)
	}
	return nil
}
