package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Frozen reconciliation decisions and status annotations (T12). A pair of
// opportunities is only merged when the frozen identity quadruple — posting
// code, company, location, and recruiting batch — is fully known on both
// sides and pairwise equal. Anything short of that stays side by side.
const (
	ReconcileDecisionMerged     = "merged"
	ReconcileDecisionSideBySide = "side_by_side"

	reconcileKind = "opportunities_reconciled"

	AnnotationExpired             = "expired"
	AnnotationDelisted            = "delisted"
	AnnotationRequirementsChanged = "requirements_changed"
)

// Frozen expiry markers: a complete observation whose text carries one of
// these explicit closing statements marks the opportunity expired. Partial or
// failed observations never claim expiry.
var reconciliationExpiryMarkers = []string{
	"已截止", "已过期", "岗位已关闭", "position closed", "no longer accepting applications",
}

// Frozen failed recheck statuses: the latest observation in one of these
// states leaves the last successful observation in place and marks the data
// stale instead of discarding anything. A manual paste observation carries an
// empty status and counts as healthy.
var reconciliationFailedStatuses = map[string]bool{
	SourceStatusLoginRequired:    true,
	SourceStatusBlocked:          true,
	SourceStatusNotFound:         true,
	SourceStatusTimedOut:         true,
	SourceStatusFetchFailed:      true,
	SourceStatusPolicyUnverified: true,
}

var errReconciliationReceipt = errors.New("unreadable career reconciliation receipt")

// ReconcileInput is the frozen request body of a reconciliation decision. The
// target keeps its identity; the candidate merges into it when — and only
// when — the identity evidence is sufficient.
type ReconcileInput struct {
	RequestID   string `json:"requestId"`
	TargetID    string `json:"targetId"`
	CandidateID string `json:"candidateId"`
}

// IdentityEvidence is the frozen matching evidence of one opportunity. Empty
// fields mean "no known evidence for this dimension"; a merge requires every
// dimension to be non-empty and equal on both sides.
type IdentityEvidence struct {
	JobCode    string `json:"jobCode"`
	Title      string `json:"title,omitempty"`
	Company    string `json:"company,omitempty"`
	Location   string `json:"location,omitempty"`
	Batch      string `json:"batch,omitempty"`
	SourceRef  string `json:"sourceRef,omitempty"`
	SnapshotID string `json:"snapshotId,omitempty"`
}

type ReconcileEvidence struct {
	Target    IdentityEvidence `json:"target"`
	Candidate IdentityEvidence `json:"candidate"`
}

// ReconcileReceipt is the durable terminal receipt of one reconciliation
// decision, replayed verbatim for repeated request IDs.
type ReconcileReceipt struct {
	Kind               string            `json:"kind"`
	RequestID          string            `json:"requestId"`
	Decision           string            `json:"decision"`
	TargetID           string            `json:"targetId"`
	CandidateID        string            `json:"candidateId"`
	SuspectedDuplicate bool              `json:"suspectedDuplicate"`
	Evidence           ReconcileEvidence `json:"evidence"`
	// ConflictingBatches discloses the batches for which both records already
	// held an application at merge time. Those pre-existing rows stay on the
	// merged-away record (honest history); one-job-one-batch still governs
	// every new application on the canonical owner.
	ConflictingBatches []string  `json:"conflictingBatches,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
}

// OpportunityStatusView is the explicit status projection of one opportunity:
// the annotations computed from comparing later observations against earlier
// snapshots, the stale window of the last successful check, and the full
// immutable observation history with every original link and check time.
type OpportunityStatusView struct {
	OpportunityID string                       `json:"opportunityId"`
	Annotations   []string                     `json:"annotations"`
	Stale         bool                         `json:"stale"`
	LastCheckedAt time.Time                    `json:"lastCheckedAt"`
	LastHealthyAt time.Time                    `json:"lastHealthyAt"`
	Observations  []OpportunityObservationView `json:"observations"`
	MergedInto    string                       `json:"mergedInto,omitempty"`
}

// ObservedSourceCoverage aggregates the sources that actually produced
// observations in this space, with the honest counts and last check times.
type ObservedSourceCoverage struct {
	SourceKind    string    `json:"sourceKind"`
	Label         string    `json:"label"`
	Observations  int       `json:"observations"`
	LastCheckedAt time.Time `json:"lastCheckedAt"`
}

// CareerCoverageView is the frozen coverage disclosure: the vetted source
// registry (production starts empty) plus what this space actually observed.
// Nothing here may invent a source or a city.
type CareerCoverageView struct {
	ConfiguredSources []SearchSourceCoverage   `json:"configuredSources"`
	ObservedSources   []ObservedSourceCoverage `json:"observedSources"`
	ObservedCities    []string                 `json:"observedCities"`
}

// reconciliationRecord is the durable decision row. It is also the request-ID
// receipt carrier: one scoped request ID writes exactly one decision.
type reconciliationRecord struct {
	ID           string `gorm:"primaryKey;size:36"`
	TenantID     uint64 `gorm:"uniqueIndex:career_reconciliation_scope_request;index:idx_career_reconciliation_scope;index:idx_career_reconciliation_target,priority:1;index:idx_career_reconciliation_candidate,priority:1"`
	UserID       string `gorm:"uniqueIndex:career_reconciliation_scope_request;index:idx_career_reconciliation_scope;index:idx_career_reconciliation_target,priority:2;index:idx_career_reconciliation_candidate,priority:2;size:512"`
	RequestID    string `gorm:"uniqueIndex:career_reconciliation_scope_request;size:128"`
	Fingerprint  string `gorm:"size:64;not null"`
	Decision     string `gorm:"size:16;not null"`
	TargetID     string `gorm:"size:36;not null;index:idx_career_reconciliation_target,priority:3"`
	CandidateID  string `gorm:"size:36;not null;index:idx_career_reconciliation_candidate,priority:3"`
	EvidenceBody string `gorm:"type:text;not null"`
	ReceiptBody  string `gorm:"type:text;not null"`
	CreatedAt    time.Time
}

func (reconciliationRecord) TableName() string { return "career_reconciliations" }

// jobCodeTokenPattern is the frozen posting-code rule: after trimming the
// query and fragment, the final non-empty path segment qualifies only when it
// is a plain alphanumeric token (optionally with '-' or '_').
var jobCodeTokenPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

// jobCodeFromRef extracts the frozen posting code from a source reference.
// It returns "" (unknown) when the reference carries no decidable code.
func jobCodeFromRef(ref string) string {
	trimmed := ref
	if idx := strings.IndexAny(trimmed, "?#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	segments := strings.Split(trimmed, "/")
	last := segments[len(segments)-1]
	if !jobCodeTokenPattern.MatchString(last) {
		return ""
	}
	return strings.ToLower(last)
}

// collectIdentityEvidence derives the frozen identity quadruple from one
// opportunity's immutable history: each dimension takes its most recent
// known snapshot value, and the posting code comes from the most recent
// source reference that yields one.
func collectIdentityEvidence(tx *gorm.DB, scope Scope, opportunityID string) (IdentityEvidence, error) {
	evidence := IdentityEvidence{}
	var snapshots []opportunitySnapshot
	if err := tx.Where("tenant_id=? AND user_id=? AND opportunity_id=?", scope.TenantID, scope.UserID, opportunityID).
		Order("acquired_at DESC, id DESC").Find(&snapshots).Error; err != nil {
		// A transient read failure must not degrade into "no evidence": the
		// decision is only safe to persist when the evidence is complete.
		return IdentityEvidence{}, err
	}
	for _, snapshot := range snapshots {
		var fields OpportunityFields
		if err := json.Unmarshal([]byte(snapshot.Extracted), &fields); err != nil {
			continue
		}
		if evidence.SnapshotID == "" {
			evidence.SnapshotID = snapshot.ID
		}
		take := func(current string, value ExtractedValue) string {
			if current != "" || value.State != "known" {
				return current
			}
			return value.Value
		}
		evidence.Title = take(evidence.Title, fields.Title)
		evidence.Company = take(evidence.Company, fields.Company)
		evidence.Location = take(evidence.Location, fields.Location)
		evidence.Batch = take(evidence.Batch, fields.Batch)
		if evidence.JobCode != "" && evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
			break
		}
	}
	var observations []opportunityObservation
	if err := tx.Where("tenant_id=? AND user_id=? AND opportunity_id=?", scope.TenantID, scope.UserID, opportunityID).
		Order("acquired_at DESC, id DESC").Find(&observations).Error; err != nil {
		return IdentityEvidence{}, err
	}
	for _, observation := range observations {
		if evidence.SourceRef == "" {
			evidence.SourceRef = observation.SourceRef
		}
		if evidence.JobCode == "" {
			evidence.JobCode = jobCodeFromRef(observation.SourceRef)
		}
		if evidence.JobCode != "" && evidence.SourceRef != "" {
			break
		}
	}
	return evidence, nil
}

// sufficientIdentityEvidence is the frozen merge threshold: both sides must
// carry a complete, pairwise-equal identity quadruple.
func sufficientIdentityEvidence(target, candidate IdentityEvidence) bool {
	for _, pair := range [][2]string{
		{target.JobCode, candidate.JobCode},
		{target.Company, candidate.Company},
		{target.Location, candidate.Location},
		{target.Batch, candidate.Batch},
	} {
		if pair[0] == "" || pair[0] != pair[1] {
			return false
		}
	}
	return true
}

// suspectedDuplicate flags a side-by-side pair whose title and company both
// match on known evidence — an honest "possibly the same job" hint that never
// merges anything by itself.
func suspectedDuplicate(target, candidate IdentityEvidence) bool {
	return target.Title != "" && target.Title == candidate.Title &&
		target.Company != "" && target.Company == candidate.Company
}

// ReconcileOpportunities decides, under the frozen evidence rules, whether two
// opportunity records describe the same job. A merge re-parents the
// candidate's immutable observations and snapshots onto the target without
// deleting or rewriting any row; an uncertain pair stays side by side with an
// explicit suspected-duplicate flag. The decision is durable per request ID:
// exact replays return the stored receipt and changed content conflicts.
func (o *Office) ReconcileOpportunities(ctx context.Context, input ReconcileInput) (ReconcileReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ReconcileReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ReconcileReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.CandidateID = strings.TrimSpace(input.CandidateID)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.TargetID == "" || input.CandidateID == "" || input.TargetID == input.CandidateID {
		return ReconcileReceipt{}, ErrInvalidRequest
	}
	intent, err := json.Marshal([]any{reconcileKind, input.RequestID, input.TargetID, input.CandidateID})
	if err != nil {
		return ReconcileReceipt{}, err
	}
	sum := sha256.Sum256(intent)
	fingerprint := hex.EncodeToString(sum[:])

	var receipt ReconcileReceipt
	transaction := func(tx *gorm.DB) error {
		var prior reconciliationRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return json.Unmarshal([]byte(prior.ReceiptBody), &receipt)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireGateActiveTx(tx, s); err != nil {
			return err
		}
		for _, id := range []string{input.TargetID, input.CandidateID} {
			var row opportunity
			if err := tx.Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, id).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOpportunityNotFound
			} else if err != nil {
				return err
			}
		}
		targetEvidence, evidenceErr := collectIdentityEvidence(tx, s, input.TargetID)
		if evidenceErr != nil {
			return evidenceErr
		}
		candidateEvidence, evidenceErr := collectIdentityEvidence(tx, s, input.CandidateID)
		if evidenceErr != nil {
			return evidenceErr
		}
		decision := ReconcileDecisionSideBySide
		conflictingBatches := []string{}
		if sufficientIdentityEvidence(targetEvidence, candidateEvidence) {
			decision = ReconcileDecisionMerged
			// Re-parent the candidate's immutable history. No observation,
			// snapshot, or application row is deleted or rewritten beyond
			// its owning opportunity reference.
			if err := tx.Model(&opportunityObservation{}).
				Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, input.CandidateID).
				Update("opportunity_id", input.TargetID).Error; err != nil {
				return err
			}
			if err := tx.Model(&opportunitySnapshot{}).
				Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, input.CandidateID).
				Update("opportunity_id", input.TargetID).Error; err != nil {
				return err
			}
			// Evaluations created before the merge still name the candidate;
			// they migrate onto the target like snapshots so every consumer
			// comparing evaluation.OpportunityID (application creation) keeps
			// working. No unique index spans opportunity ownership, so no
			// conflicting-batch carve-out is needed here.
			if err := tx.Model(&evaluationRecord{}).
				Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, input.CandidateID).
				Update("opportunity_id", input.TargetID).Error; err != nil {
				return err
			}
			// One job and batch admits exactly one application: the merged
			// pair is one job now, so earlier application rows migrate onto
			// the merge target to keep both the uniqueness check and the
			// database constraint meaningful. Pinned evidence bodies inside
			// those rows are never rewritten.
			//
			// Carve-out: when both records already hold an application for
			// the same batch (each was a distinct, legal job when created),
			// migrating that row would collide with the one-job-one-batch
			// unique index and strand the merge. Those conflicting rows stay
			// on the merged-away record — honest history, still reachable
			// through the merge chain — and the receipt discloses the batches.
			targetBatches := tx.Model(&applicationRecord{}).
				Select("batch_identity").
				Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, input.TargetID)
			if err := tx.Model(&applicationRecord{}).
				Where("tenant_id=? AND user_id=? AND opportunity_id=? AND batch_identity IN (?)",
					s.TenantID, s.UserID, input.CandidateID, targetBatches).
				Pluck("batch_identity", &conflictingBatches).Error; err != nil {
				return err
			}
			if err := tx.Model(&applicationRecord{}).
				Where("tenant_id=? AND user_id=? AND opportunity_id=? AND batch_identity NOT IN (?)",
					s.TenantID, s.UserID, input.CandidateID, targetBatches).
				Updates(map[string]any{"opportunity_id": input.TargetID, "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		receipt = ReconcileReceipt{
			Kind:        reconcileKind,
			RequestID:   input.RequestID,
			Decision:    decision,
			TargetID:    input.TargetID,
			CandidateID: input.CandidateID,
			// The suspected-duplicate hint only applies to uncertain pairs; a
			// merged pair is a confirmed identity, not a suspicion.
			SuspectedDuplicate: decision == ReconcileDecisionSideBySide && suspectedDuplicate(targetEvidence, candidateEvidence),
			Evidence:           ReconcileEvidence{Target: targetEvidence, Candidate: candidateEvidence},
			ConflictingBatches: conflictingBatches,
			CreatedAt:          now,
		}
		evidenceBody, err := json.Marshal(receipt.Evidence)
		if err != nil {
			return err
		}
		receiptBody, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		record := reconciliationRecord{
			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint,
			Decision: decision, TargetID: input.TargetID, CandidateID: input.CandidateID,
			EvidenceBody: string(evidenceBody), ReceiptBody: string(receiptBody), CreatedAt: now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if o.failReconcileCommit != nil {
			// Test seam only: inject a post-decision failure so the caller
			// retries the same request ID and recovers exactly one decision.
			if hookErr := o.failReconcileCommit(); hookErr != nil {
				return hookErr
			}
		}
		return nil
	}
	if err := o.runImportTransaction(ctx, transaction); err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrOpportunityNotFound) {
			return ReconcileReceipt{}, err
		}
		if isReceiptRaceError(err) {
			if replay, found, lookupErr := o.replayReconciliation(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
				return ReconcileReceipt{}, lookupErr
			} else if found {
				return replay, nil
			}
		}
		if isSQLiteBusy(err) {
			// The decision transaction rolled back whole (the bounded busy
			// retry budget may have run out while the parent context stayed
			// healthy): nothing durable was decided, so the same request ID
			// retries safely instead of surfacing a raw "database is locked".
			return ReconcileReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if ctx.Err() != nil {
			return ReconcileReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ReconcileReceipt{}, err
	}
	return receipt, nil
}

func (o *Office) replayReconciliation(ctx context.Context, s Scope, requestID, fingerprint string) (ReconcileReceipt, bool, error) {
	var row reconciliationRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReconcileReceipt{}, false, nil
	}
	if err != nil {
		return ReconcileReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint && fingerprint != "" {
		return ReconcileReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt ReconcileReceipt
	if err := json.Unmarshal([]byte(row.ReceiptBody), &receipt); err != nil {
		return ReconcileReceipt{}, true, errReconciliationReceipt
	}
	return receipt, true, nil
}

// FindReconciliationReceipt replays the stored decision receipt by request ID.
func (o *Office) FindReconciliationReceipt(ctx context.Context, requestID string) (ReconcileReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ReconcileReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ReconcileReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return ReconcileReceipt{}, ErrInvalidRequest
	}
	receipt, found, err := o.replayReconciliation(ctx, s, requestID, "")
	if err != nil {
		return ReconcileReceipt{}, err
	}
	if !found {
		return ReconcileReceipt{}, ErrReceiptNotFound
	}
	return receipt, nil
}

// OpportunityReconciliations lists every durable decision that involved one
// opportunity, either as target or as candidate.
func (o *Office) OpportunityReconciliations(ctx context.Context, opportunityID string) ([]ReconcileReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" || len(opportunityID) > 36 {
		return nil, ErrInvalidRequest
	}
	var rows []reconciliationRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND (target_id=? OR candidate_id=?)", s.TenantID, s.UserID, opportunityID, opportunityID).
		Order("created_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	decisions := make([]ReconcileReceipt, 0, len(rows))
	for _, row := range rows {
		var receipt ReconcileReceipt
		if err := json.Unmarshal([]byte(row.ReceiptBody), &receipt); err != nil {
			return nil, errReconciliationReceipt
		}
		decisions = append(decisions, receipt)
	}
	return decisions, nil
}

// mergedInto resolves where an opportunity's history went after a merge; ""
// means it was never merged away.
func (o *Office) mergedInto(ctx context.Context, s Scope, opportunityID string) string {
	var row reconciliationRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND candidate_id=? AND decision=?", s.TenantID, s.UserID, opportunityID, ReconcileDecisionMerged).
		Order("created_at DESC, id DESC").First(&row).Error
	if err != nil {
		return ""
	}
	return row.TargetID
}

// canonicalOpportunityID resolves an opportunity ID through merged
// reconciliation decisions (bounded hops, loop-safe) so pre-merge references
// keep resolving after their observations and snapshots moved to the merge
// target. It returns the input unchanged when no merge chain applies or the
// lookup fails; callers then fall through to their ordinary not-found path.
func canonicalOpportunityID(db *gorm.DB, scope Scope, opportunityID string) string {
	current := opportunityID
	for hop := 0; hop < 8; hop++ {
		var row reconciliationRecord
		err := db.Where("tenant_id=? AND user_id=? AND candidate_id=? AND decision=?",
			scope.TenantID, scope.UserID, current, ReconcileDecisionMerged).
			Order("created_at DESC, id DESC").First(&row).Error
		if err != nil || row.TargetID == "" || row.TargetID == current {
			return current
		}
		current = row.TargetID
	}
	return current
}

// OpportunityStatus projects the explicit status of one opportunity from its
// immutable observation history: delisting, expiry, and requirement changes
// are annotations computed by comparing later observations with earlier
// snapshots; a failed latest recheck keeps the last successful observation
// and exposes the stale window instead of discarding anything.
func (o *Office) OpportunityStatus(ctx context.Context, opportunityID string) (OpportunityStatusView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return OpportunityStatusView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return OpportunityStatusView{}, err
	}
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" || len(opportunityID) > 36 {
		return OpportunityStatusView{}, ErrInvalidRequest
	}
	var owner opportunity
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, opportunityID).First(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OpportunityStatusView{}, ErrOpportunityNotFound
	}
	if err != nil {
		return OpportunityStatusView{}, err
	}
	observations, err := o.OpportunityObservations(ctx, opportunityID)
	if err != nil {
		return OpportunityStatusView{}, err
	}
	view := OpportunityStatusView{
		OpportunityID: opportunityID,
		Annotations:   []string{},
		Observations:  observations,
		MergedInto:    o.mergedInto(ctx, s, opportunityID),
	}
	if len(observations) == 0 {
		return view, nil
	}
	latest := observations[len(observations)-1]
	view.LastCheckedAt = latest.AcquiredAt
	view.Stale = reconciliationFailedStatuses[latest.SourceStatus]
	if latest.SourceStatus == SourceStatusNotFound {
		view.Annotations = append(view.Annotations, AnnotationDelisted)
	}
	// Walk back to the last healthy observation for the stale window and the
	// expiry check; a healthy observation is anything outside the frozen
	// failure set (manual pastes carry an empty status).
	var lastHealthy *OpportunityObservationView
	for i := len(observations) - 1; i >= 0; i-- {
		if !reconciliationFailedStatuses[observations[i].SourceStatus] {
			lastHealthy = &observations[i]
			break
		}
	}
	if lastHealthy != nil {
		view.LastHealthyAt = lastHealthy.AcquiredAt
	}
	if lastHealthy != nil && containsExpiryMarker(o, ctx, s, opportunityID, lastHealthy.SnapshotID) {
		view.Annotations = append(view.Annotations, AnnotationExpired)
	}
	if requirementsChanged(o, ctx, s, opportunityID) {
		view.Annotations = append(view.Annotations, AnnotationRequirementsChanged)
	}
	return view, nil
}

func containsExpiryMarker(o *Office, ctx context.Context, s Scope, opportunityID, snapshotID string) bool {
	var snapshot opportunitySnapshot
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", s.TenantID, s.UserID, opportunityID, snapshotID).
		First(&snapshot).Error
	if err != nil {
		return false
	}
	lower := strings.ToLower(snapshot.RawText)
	for _, marker := range reconciliationExpiryMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// requirementsChanged compares consecutive known requirement values across the
// snapshot history in acquisition order; any drift is an explicit annotation.
func requirementsChanged(o *Office, ctx context.Context, s Scope, opportunityID string) bool {
	var snapshots []opportunitySnapshot
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, opportunityID).
		Order("acquired_at, id").Find(&snapshots).Error
	if err != nil {
		return false
	}
	previous, seen := "", false
	for _, snapshot := range snapshots {
		var fields OpportunityFields
		if err := json.Unmarshal([]byte(snapshot.Extracted), &fields); err != nil {
			continue
		}
		if fields.Requirements.State != "known" || fields.Requirements.Value == "" {
			continue
		}
		if seen && previous != fields.Requirements.Value {
			return true
		}
		previous, seen = fields.Requirements.Value, true
	}
	return false
}

// SourceCoverage discloses the honest coverage facts: the vetted search
// registry (production starts empty) plus the sources and cities this space
// actually observed. Nothing is invented: configured sources come only from
// the registry seam, observed sources only from stored observations, and
// observed cities only from known location evidence.
func (o *Office) SourceCoverage(ctx context.Context) (CareerCoverageView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerCoverageView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerCoverageView{}, err
	}
	configured := []SearchSourceCoverage{}
	if o.searchRegistry != nil {
		for _, source := range o.searchRegistry.SearchSources() {
			configured = append(configured, SearchSourceCoverage{
				SourceID: source.ID, Label: source.Label,
				AccessMethods: append([]string{}, source.AccessMethods...),
				Cities:        append([]string{}, source.Cities...),
				Available:     true,
			})
		}
	}
	var observationRows []opportunityObservation
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
		Order("source_kind, acquired_at, id").Find(&observationRows).Error
	if err != nil {
		return CareerCoverageView{}, err
	}
	// Aggregate in memory: the honest per-source counts and last check times
	// derive directly from the stored observation rows.
	observedIndex := map[string]int{}
	observedSources := []ObservedSourceCoverage{}
	for _, row := range observationRows {
		if idx, ok := observedIndex[row.SourceKind]; ok {
			observedSources[idx].Observations++
			if row.AcquiredAt.After(observedSources[idx].LastCheckedAt) {
				observedSources[idx].LastCheckedAt = row.AcquiredAt
			}
			continue
		}
		observedIndex[row.SourceKind] = len(observedSources)
		observedSources = append(observedSources, ObservedSourceCoverage{
			SourceKind: row.SourceKind, Label: row.SourceLabel,
			Observations: 1, LastCheckedAt: row.AcquiredAt,
		})
	}
	sort.Slice(observedSources, func(i, j int) bool { return observedSources[i].SourceKind < observedSources[j].SourceKind })
	var snapshots []opportunitySnapshot
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Find(&snapshots).Error
	if err != nil {
		return CareerCoverageView{}, err
	}
	citySet := map[string]bool{}
	for _, snapshot := range snapshots {
		var fields OpportunityFields
		if err := json.Unmarshal([]byte(snapshot.Extracted), &fields); err != nil {
			continue
		}
		if fields.Location.State == "known" && strings.TrimSpace(fields.Location.Value) != "" {
			citySet[fields.Location.Value] = true
		}
	}
	cities := make([]string, 0, len(citySet))
	for city := range citySet {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	return CareerCoverageView{
		ConfiguredSources: configured,
		ObservedSources:   observedSources,
		ObservedCities:    cities,
	}, nil
}
