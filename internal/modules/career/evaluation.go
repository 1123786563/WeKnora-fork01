package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	EvaluationEligible   = "eligible"
	EvaluationIneligible = "ineligible"
	EvaluationUnknown    = "unknown"
	evaluationRuleset    = "career-qualification-v1"
	evaluationLookupTime = 350 * time.Millisecond
)

var (
	ErrEvaluationNotFound = errors.New("career evaluation not found")
	graduationOnlyPattern = regexp.MustCompile(`^仅限[ \t]*([0-9]{4})[ \t]*届`)
	graduationYearPattern = regexp.MustCompile(`^([0-9]{4})(?:届|年)?$`)
)

type EvaluateInput struct {
	RequestID       string  `json:"requestId"`
	OpportunityID   string  `json:"opportunityId"`
	SnapshotID      string  `json:"snapshotId"`
	ProfileRevision *uint64 `json:"profileRevision,omitempty"`
}

type EvaluationReceipt struct {
	Kind            string `json:"kind"`
	RequestID       string `json:"requestId"`
	EvaluationID    string `json:"evaluationId"`
	OpportunityID   string `json:"opportunityId"`
	SnapshotID      string `json:"snapshotId"`
	ProfileRevision uint64 `json:"profileRevision"`
	Status          string `json:"status"`
}

type EvaluationSnapshotEvidence struct {
	OpportunityID string            `json:"opportunityId"`
	ObservationID string            `json:"observationId"`
	SnapshotID    string            `json:"snapshotId"`
	RawText       string            `json:"rawText"`
	RawSHA256     string            `json:"rawSha256"`
	Source        OpportunitySource `json:"source"`
	AcquiredAt    time.Time         `json:"acquiredAt"`
}

type EvaluationFactEvidence struct {
	FactKey      string       `json:"factKey"`
	Value        string       `json:"value"`
	Revision     uint64       `json:"revision"`
	FactRevision uint64       `json:"factRevision"`
	Source       Source       `json:"source"`
	Confirmation Confirmation `json:"confirmation"`
	ConfirmedAt  time.Time    `json:"confirmedAt"`
}

type EvaluationJobEvidence struct {
	SnapshotID    string    `json:"snapshotId"`
	ObservationID string    `json:"observationId"`
	AcquiredAt    time.Time `json:"acquiredAt"`
	RawSHA256     string    `json:"rawSha256"`
	SpanStart     int       `json:"spanStart"`
	SpanEnd       int       `json:"spanEnd"`
	QuotedText    string    `json:"quotedText"`
}

type EvaluationHardRule struct {
	RuleID          string                  `json:"ruleId"`
	Criterion       string                  `json:"criterion"`
	Outcome         string                  `json:"outcome"`
	ReasonCode      string                  `json:"reasonCode"`
	JobEvidence     *EvaluationJobEvidence  `json:"jobEvidence,omitempty"`
	ProfileEvidence *EvaluationFactEvidence `json:"profileEvidence,omitempty"`
}

type EvaluationHardResult struct {
	Overall string               `json:"overall"`
	Rules   []EvaluationHardRule `json:"rules"`
}

type EvaluationSoftMatch struct {
	Kind            string                 `json:"kind"`
	Value           string                 `json:"value"`
	JobEvidence     EvaluationJobEvidence  `json:"jobEvidence"`
	ProfileEvidence EvaluationFactEvidence `json:"profileEvidence"`
}

type EvaluationSoftResult struct {
	Matches []EvaluationSoftMatch `json:"matches"`
}

type Evaluation struct {
	EvaluationReceipt
	CreatedAt      time.Time                  `json:"createdAt"`
	RulesetVersion string                     `json:"rulesetVersion"`
	Snapshot       EvaluationSnapshotEvidence `json:"snapshot"`
	Hard           EvaluationHardResult       `json:"hard"`
	Soft           EvaluationSoftResult       `json:"soft"`
	Facts          []EvaluationFactEvidence   `json:"facts"`
}

func (o *Office) EvaluateOpportunity(ctx context.Context, input EvaluateInput) (EvaluationReceipt, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return EvaluationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return EvaluationReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.OpportunityID = strings.TrimSpace(input.OpportunityID)
	input.SnapshotID = strings.TrimSpace(input.SnapshotID)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.OpportunityID == "" || input.SnapshotID == "" {
		return EvaluationReceipt{}, ErrInvalidRequest
	}
	intentBytes, err := json.Marshal(input)
	if err != nil {
		return EvaluationReceipt{}, err
	}
	fingerprintBytes := sha256.Sum256(intentBytes)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	var result EvaluationReceipt
	var persistenceMayHaveCommitted bool
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prior evaluationRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, input.RequestID).First(&prior).Error
		if err == nil {
			if prior.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return json.Unmarshal([]byte(prior.ReceiptBody), &result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireGateActiveTx(tx, scope); err != nil {
			return err
		}
		if o.afterEvaluationReceiptMiss != nil {
			o.afterEvaluationReceiptMiss()
		}

		var head profile
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", scope.TenantID, scope.UserID).First(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if err != nil {
			return err
		}
		pinnedRevision := head.Revision
		if input.ProfileRevision != nil {
			pinnedRevision = *input.ProfileRevision
			if pinnedRevision > head.Revision {
				return &RevisionConflictError{CurrentRevision: head.Revision}
			}
		}

		var snapshot opportunitySnapshot
		err = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", scope.TenantID, scope.UserID, input.OpportunityID, input.SnapshotID).First(&snapshot).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Pre-merge references keep resolving: a merged candidate's
			// snapshots were re-parented onto the merge target, so retry
			// through the merge chain (T12 reconciliation).
			if canonical := canonicalOpportunityID(tx, scope, input.OpportunityID); canonical != "" && canonical != input.OpportunityID {
				err = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", scope.TenantID, scope.UserID, canonical, input.SnapshotID).First(&snapshot).Error
			}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOpportunityNotFound
		}
		if err != nil {
			return err
		}
		var observation opportunityObservation
		err = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?", scope.TenantID, scope.UserID, snapshot.OpportunityID, snapshot.ObservationID).First(&observation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || observation.SnapshotID != snapshot.ID {
			return ErrOpportunityNotFound
		}
		if err != nil {
			return err
		}
		// The evaluation is new evidence and belongs to the snapshot's
		// canonical owner. After a merge re-parented this snapshot, the
		// evaluation must land on the merge target so downstream application
		// creation (which compares evaluation.OpportunityID) stays usable.
		// The request fingerprint above was computed from the original input
		// and stays stable for replays.
		input.OpportunityID = snapshot.OpportunityID

		facts, err := confirmedFactsAtRevision(tx, scope, pinnedRevision)
		if err != nil {
			return err
		}
		evaluated := evaluateSnapshot(input, snapshot, observation, pinnedRevision, facts)
		result = evaluated.EvaluationReceipt
		receiptBody, err := json.Marshal(result)
		if err != nil {
			return err
		}
		evaluationBody, err := json.Marshal(evaluated)
		if err != nil {
			return err
		}
		intent := string(intentBytes)
		record := evaluationRecord{ID: result.EvaluationID, TenantID: scope.TenantID, UserID: scope.UserID, RequestID: input.RequestID, Fingerprint: fingerprint, Intent: intent, OpportunityID: input.OpportunityID, SnapshotID: input.SnapshotID, ProfileRevision: pinnedRevision, ReceiptBody: string(receiptBody), EvaluationBody: string(evaluationBody), CreatedAt: evaluated.CreatedAt}
		persistenceMayHaveCommitted = true
		return tx.Create(&record).Error
	})
	if err == nil && o.afterEvaluationCommit != nil {
		err = o.afterEvaluationCommit()
	}
	if err == nil {
		return result, nil
	}
	if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrRevisionConflict) || errors.Is(err, ErrOpportunityNotFound) || errors.Is(err, ErrInvalidRequest) {
		return EvaluationReceipt{}, err
	}
	if !persistenceMayHaveCommitted && ctx.Err() == nil {
		return EvaluationReceipt{}, err
	}
	if replay, found, lookupErr := o.reconcileEvaluation(ctx, scope, input.RequestID, fingerprint); errors.Is(lookupErr, ErrIdempotencyConflict) {
		return EvaluationReceipt{}, ErrIdempotencyConflict
	} else if lookupErr == nil && found {
		return replay, nil
	}
	return EvaluationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
}

func confirmedFactsAtRevision(tx *gorm.DB, scope Scope, revision uint64) ([]Fact, error) {
	var rows []factVersion
	if err := tx.Where("tenant_id=? AND user_id=? AND revision<=?", scope.TenantID, scope.UserID, revision).Order("key ASC, revision DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(rows))
	out := make([]Fact, 0, len(rows))
	for _, row := range rows {
		if seen[row.Key] {
			continue
		}
		seen[row.Key] = true
		confirmation := decodeConfirmation(row.Confirmation)
		if confirmation.UserID != scope.UserID || confirmation.ConfirmedAt.IsZero() {
			continue
		}
		out = append(out, Fact{Key: row.Key, Value: row.Value, Revision: row.Revision, Source: decodeSource(row.Source), Confirmation: confirmation, ConfirmedAt: confirmation.ConfirmedAt})
	}
	return out, nil
}

func evaluateSnapshot(input EvaluateInput, snapshot opportunitySnapshot, observation opportunityObservation, revision uint64, facts []Fact) Evaluation {
	createdAt := time.Now().UTC()
	receipt := EvaluationReceipt{Kind: "evaluation_created", RequestID: input.RequestID, EvaluationID: uuid.NewString(), OpportunityID: input.OpportunityID, SnapshotID: snapshot.ID, ProfileRevision: revision}
	job := EvaluationSnapshotEvidence{OpportunityID: snapshot.OpportunityID, ObservationID: observation.ID, SnapshotID: snapshot.ID, RawText: snapshot.RawText, RawSHA256: snapshot.RawSHA256, Source: OpportunitySource{Kind: observation.SourceKind, Label: observation.SourceLabel, ReferenceID: observation.SourceRef}, AcquiredAt: snapshot.AcquiredAt}
	gradRule, overall := evaluateGraduationRule(job, revision, facts)
	soft := softEvidence(job, revision, facts)
	used := make(map[string]EvaluationFactEvidence)
	if gradRule.ProfileEvidence != nil {
		used[gradRule.ProfileEvidence.FactKey] = *gradRule.ProfileEvidence
	}
	for _, match := range soft.Matches {
		used[match.ProfileEvidence.FactKey] = match.ProfileEvidence
	}
	factEvidence := make([]EvaluationFactEvidence, 0, len(used))
	for _, fact := range facts {
		if evidence, ok := used[fact.Key]; ok {
			factEvidence = append(factEvidence, evidence)
		}
	}
	receipt.Status = overall
	return Evaluation{EvaluationReceipt: receipt, CreatedAt: createdAt, RulesetVersion: evaluationRuleset, Snapshot: job, Hard: EvaluationHardResult{Overall: overall, Rules: []EvaluationHardRule{gradRule}}, Soft: soft, Facts: factEvidence}
}

func evaluateGraduationRule(job EvaluationSnapshotEvidence, revision uint64, facts []Fact) (EvaluationHardRule, string) {
	rule := EvaluationHardRule{RuleID: "graduation_year", Criterion: "graduation year", Outcome: EvaluationUnknown, ReasonCode: "graduation_requirement_missing_or_unsupported"}
	trimmed := strings.TrimSpace(job.RawText)
	match := graduationOnlyPattern.FindStringSubmatchIndex(trimmed)
	if match == nil {
		return rule, EvaluationUnknown
	}
	// A hard conclusion requires the entire JD to be one affirmative clause.
	// Any other text may qualify it, including apparently unrelated fields.
	suffix := trimmed[match[1]:]
	if suffix != "" && suffix != "。" && suffix != "." {
		rule.ReasonCode = "graduation_requirement_ambiguous_or_qualified"
		return rule, EvaluationUnknown
	}
	spanStart := len(job.RawText) - len(strings.TrimLeftFunc(job.RawText, unicode.IsSpace))
	spanEnd := spanStart + match[1]
	rule.JobEvidence = evaluationJobEvidence(job, spanStart, spanEnd)
	targetYear, _ := strconv.Atoi(trimmed[match[2]:match[3]])
	if targetYear < 1900 || targetYear > 2200 {
		rule.ReasonCode = "graduation_requirement_invalid"
		return rule, EvaluationUnknown
	}
	var candidates []struct {
		fact Fact
		year int
	}
	malformed := false
	for _, f := range facts {
		if !isGraduationFactKey(f.Key) {
			continue
		}
		year, ok := normalizeGraduationYear(f.Value)
		if !ok {
			malformed = true
			continue
		}
		candidates = append(candidates, struct {
			fact Fact
			year int
		}{fact: f, year: year})
	}
	if malformed {
		rule.ReasonCode = "graduation_fact_ambiguous"
		return rule, EvaluationUnknown
	}
	if len(candidates) == 0 {
		rule.ReasonCode = "confirmed_graduation_year_missing"
		return rule, EvaluationUnknown
	}
	selected := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.year != selected.year {
			rule.ReasonCode = "graduation_fact_ambiguous"
			return rule, EvaluationUnknown
		}
		if graduationFactPriority(candidate.fact.Key) < graduationFactPriority(selected.fact.Key) {
			selected = candidate
		}
	}
	rule.ProfileEvidence = evaluationFact(selected.fact, revision)
	if selected.year != targetYear {
		rule.Outcome = EvaluationIneligible
		rule.ReasonCode = "graduation_year_mismatch"
		return rule, EvaluationIneligible
	}
	rule.Outcome = EvaluationEligible
	rule.ReasonCode = "graduation_year_matches"
	return rule, EvaluationEligible
}

func isGraduationFactKey(key string) bool {
	switch key {
	case "education.graduation_year", "graduation_year", "毕业时间", "education.graduation_date":
		return true
	default:
		return false
	}
}

func graduationFactPriority(key string) int {
	switch key {
	case "education.graduation_year":
		return 0
	case "graduation_year":
		return 1
	case "毕业时间":
		return 2
	case "education.graduation_date":
		return 3
	default:
		return 4
	}
}

func normalizeGraduationYear(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if match := graduationYearPattern.FindStringSubmatch(value); len(match) == 2 {
		if year, err := strconv.Atoi(match[1]); err == nil && year >= 1900 && year <= 2200 {
			return year, true
		}
	}
	for _, layout := range []string{"2006-01-02", "2006-01", "2006/01/02", "2006/01", "2006年1月2日", "2006年1月"} {
		date, err := time.Parse(layout, value)
		if err == nil && date.Year() >= 1900 && date.Year() <= 2200 {
			return date.Year(), true
		}
	}
	return 0, false
}

func evaluationJobEvidence(job EvaluationSnapshotEvidence, start, end int) *EvaluationJobEvidence {
	return &EvaluationJobEvidence{SnapshotID: job.SnapshotID, ObservationID: job.ObservationID, AcquiredAt: job.AcquiredAt, RawSHA256: job.RawSHA256, SpanStart: start, SpanEnd: end, QuotedText: job.RawText[start:end]}
}

func evaluationFact(fact Fact, profileRevision uint64) *EvaluationFactEvidence {
	return &EvaluationFactEvidence{FactKey: fact.Key, Value: fact.Value, Revision: profileRevision, FactRevision: fact.Revision, Source: fact.Source, Confirmation: fact.Confirmation, ConfirmedAt: fact.ConfirmedAt}
}

func softEvidence(job EvaluationSnapshotEvidence, revision uint64, facts []Fact) EvaluationSoftResult {
	result := EvaluationSoftResult{Matches: []EvaluationSoftMatch{}}
	for _, fact := range facts {
		kind := ""
		switch {
		case strings.HasPrefix(fact.Key, "skill."):
			kind = "skill"
		case strings.HasPrefix(fact.Key, "project."):
			kind = "project"
		case strings.HasPrefix(fact.Key, "preference."):
			kind = "intent"
		}
		value := strings.TrimSpace(fact.Value)
		if kind == "" || value == "" {
			continue
		}
		start, end, found := findSoftEvidenceSpan(job.RawText, value)
		if !found {
			continue
		}
		jobEvidence := *evaluationJobEvidence(job, start, end)
		result.Matches = append(result.Matches, EvaluationSoftMatch{Kind: kind, Value: value, JobEvidence: jobEvidence, ProfileEvidence: *evaluationFact(fact, revision)})
	}
	return result
}

func findSoftEvidenceSpan(raw, value string) (int, int, bool) {
	shortLatin := len(value) > 0 && len(value) <= 4
	for i := 0; shortLatin && i < len(value); i++ {
		if !((value[i] >= 'a' && value[i] <= 'z') || (value[i] >= 'A' && value[i] <= 'Z')) {
			shortLatin = false
		}
	}
	for offset := 0; offset <= len(raw)-len(value); {
		relative := strings.Index(raw[offset:], value)
		if relative < 0 {
			return 0, 0, false
		}
		start := offset + relative
		end := start + len(value)
		if !shortLatin || (hasASCIITokenBoundary(raw, start, end)) {
			return start, end, true
		}
		offset = start + 1
	}
	return 0, 0, false
}

func hasASCIITokenBoundary(raw string, start, end int) bool {
	return (start == 0 || !isASCIIWordByte(raw[start-1])) && (end == len(raw) || !isASCIIWordByte(raw[end]))
}

func isASCIIWordByte(value byte) bool {
	return value == '_' || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func (o *Office) Evaluation(ctx context.Context, evaluationID string) (Evaluation, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return Evaluation{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return Evaluation{}, err
	}
	evaluationID = strings.TrimSpace(evaluationID)
	if evaluationID == "" {
		return Evaluation{}, ErrInvalidRequest
	}
	var row evaluationRecord
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=?", scope.TenantID, scope.UserID, evaluationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Evaluation{}, ErrEvaluationNotFound
	}
	if err != nil {
		return Evaluation{}, err
	}
	var result Evaluation
	if err = json.Unmarshal([]byte(row.EvaluationBody), &result); err != nil {
		return Evaluation{}, fmt.Errorf("decode career evaluation: %w", err)
	}
	return result, nil
}

func (o *Office) FindEvaluationReceipt(ctx context.Context, requestID string) (EvaluationReceipt, error) {
	scope, err := getScope(ctx)
	if err != nil {
		return EvaluationReceipt{}, err
	}
	if err = o.requireSpace(ctx, scope); err != nil {
		return EvaluationReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return EvaluationReceipt{}, ErrInvalidRequest
	}
	var row evaluationRecord
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return EvaluationReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return EvaluationReceipt{}, err
	}
	var result EvaluationReceipt
	if err = json.Unmarshal([]byte(row.ReceiptBody), &result); err != nil {
		return EvaluationReceipt{}, fmt.Errorf("decode career evaluation receipt: %w", err)
	}
	return result, nil
}

func (o *Office) reconcileEvaluation(ctx context.Context, scope Scope, requestID, fingerprint string) (EvaluationReceipt, bool, error) {
	reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), evaluationLookupTime)
	defer cancel()
	var row evaluationRecord
	err := o.db.WithContext(reconcileCtx).Where("tenant_id=? AND user_id=? AND request_id=?", scope.TenantID, scope.UserID, requestID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return EvaluationReceipt{}, false, nil
	}
	if err != nil {
		return EvaluationReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return EvaluationReceipt{}, false, ErrIdempotencyConflict
	}
	var result EvaluationReceipt
	err = json.Unmarshal([]byte(row.ReceiptBody), &result)
	return result, err == nil, err
}
