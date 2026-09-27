package career

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// ---- T12 fixtures ----------------------------------------------------------

func newReconciliationOffice(t *testing.T) (*Office, *scriptedTransport, context.Context) {
	t.Helper()
	o, ctx := newSourceImportOffice(t, "owner", 121)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{
		"jobs.example.com":  true,
		"other.example.net": true,
	}}
	transport := &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{}}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	return o, transport, ctx
}

func knownValue(v string) ExtractedValue { return ExtractedValue{State: "known", Value: v} }

func knownFields(title, company, location, batch, requirements string) OpportunityFields {
	return OpportunityFields{
		Title:        knownValue(title),
		Company:      knownValue(company),
		Location:     knownValue(location),
		Batch:        knownValue(batch),
		Requirements: knownValue(requirements),
	}
}

// importKnownJD pastes a JD whose extractor output is fully pinned; the source
// reference carries the posting URL evidence used by the frozen job-code rule.
func importKnownJD(t *testing.T, o *Office, ctx context.Context, requestID, sourceRef string, fields OpportunityFields) OpportunityReceipt {
	t.Helper()
	o.opportunityExtractor = func(string) (OpportunityFields, error) { return fields, nil }
	receipt, err := o.ImportJD(ctx, ImportJDInput{
		RequestID:       requestID,
		RawText:         "岗位 " + requestID + "\n工作职责：\n负责核心服务\n任职要求：\n本科及以上学历",
		SourceLabel:     "Reconcile Fixture",
		SourceReference: sourceRef,
	})
	require.NoError(t, err)
	return receipt
}

// importURLComplete seeds a complete URL observation through the real import
// chain (claim → policy → transport → commit).
func importURLComplete(t *testing.T, o *Office, transport *scriptedTransport, ctx context.Context, requestID, rawURL string) ImportURLReceipt {
	t.Helper()
	transport.scripts[rawURL] = scriptedFetch{result: SourceFetchResult{
		StatusCode: 200, ContentType: "text/html",
		Text:     "高级后端工程师 工作职责： 负责核心服务 任职要求： 本科及以上学历 三年以上后端开发经验 熟悉 Go 生态",
		FinalURL: rawURL, Complete: true,
	}}
	receipt, err := o.ImportURLReceipt(ctx, ImportURLInput{RequestID: requestID, URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusComplete, receipt.SourceStatus)
	return receipt
}

// importURLCompleteForOffice wires a stub policy and scripted transport onto
// an office created elsewhere (e.g. the application fixture) and imports one
// complete URL observation through the real chain.
func importURLCompleteForOffice(t *testing.T, o *Office, ctx context.Context, requestID, rawURL string) ImportURLReceipt {
	t.Helper()
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{
		"jobs.example.com": true, "other.example.net": true,
	}}
	transport := &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{}}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	return importURLComplete(t, o, transport, ctx, requestID, rawURL)
}

// seedObservation inserts a raw observation+snapshot pair for state-comparison
// scenarios the import chain cannot produce on demand (expiry markers,
// requirement drift, failed rechecks on the same opportunity).
func seedObservation(t *testing.T, o *Office, ctx context.Context, opportunityID, sourceStatus, failureCode, rawText string, fields OpportunityFields, acquiredAt time.Time) (string, string) {
	t.Helper()
	scope, err := getScope(ctx)
	require.NoError(t, err)
	observationID, snapshotID := uuid.NewString(), uuid.NewString()
	extracted, err := json.Marshal(fields)
	require.NoError(t, err)
	require.NoError(t, o.db.Create(&opportunityObservation{
		ID: observationID, TenantID: scope.TenantID, UserID: scope.UserID,
		OpportunityID: opportunityID, SnapshotID: snapshotID,
		SourceKind: "url", SourceLabel: "fixture.example", SourceRef: "https://fixture.example/jobs/1001",
		SourceStatus: sourceStatus, Completeness: CompletenessComplete, FailureCode: failureCode,
		SubmittedURL: "https://fixture.example/jobs/1001", FinalURL: "https://fixture.example/jobs/1001",
		AcquiredAt: acquiredAt, CreatedAt: acquiredAt,
	}).Error)
	require.NoError(t, o.db.Create(&opportunitySnapshot{
		ID: snapshotID, TenantID: scope.TenantID, UserID: scope.UserID,
		OpportunityID: opportunityID, ObservationID: observationID,
		RawText: rawText, RawSHA256: "seed", Extracted: string(extracted),
		Status: OpportunityStored, AcquiredAt: acquiredAt, CreatedAt: acquiredAt,
	}).Error)
	return observationID, snapshotID
}

// ---- 1. merge threshold ----------------------------------------------------

func TestReconcileMergesOnlyWithSufficientIdentityEvidence(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-target-1", "https://jobs.example.com/postings/1001", fields)
	dual := importKnownJD(t, o, ctx, "rec-cand-1", "https://other.example.net/jobs/1001", fields)

	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-1", TargetID: target.OpportunityID, CandidateID: dual.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)
	// The frozen identity quadruple is reported as the merge evidence.
	require.Equal(t, "1001", receipt.Evidence.Target.JobCode)
	require.Equal(t, "1001", receipt.Evidence.Candidate.JobCode)
	require.Equal(t, "示例科技", receipt.Evidence.Target.Company)
	require.Equal(t, "示例科技", receipt.Evidence.Candidate.Company)
	require.Equal(t, "杭州", receipt.Evidence.Target.Location)
	require.Equal(t, "2027届秋招", receipt.Evidence.Target.Batch)
	require.False(t, receipt.SuspectedDuplicate, "a merged pair is not an uncertain duplicate")

	// The merged target owns both observations.
	observations, err := o.OpportunityObservations(ctx, target.OpportunityID)
	require.NoError(t, err)
	require.Len(t, observations, 2)
	var merged int64
	require.NoError(t, o.db.Model(&opportunityObservation{}).Where("opportunity_id = ?", dual.OpportunityID).Count(&merged).Error)
	require.Zero(t, merged, "the candidate keeps no observations after merging")

	// Missing one identity dimension (unknown location) is not sufficient
	// evidence: the pair must stay separate even when titles look identical.
	vagueFields := knownFields("平台后端工程师", "示例科技", "", "2027届秋招", "本科及以上学历")
	vagueFields.Location = ExtractedValue{State: "unknown"}
	vague := importKnownJD(t, o, ctx, "rec-vague-1", "https://jobs.example.com/postings/7777", vagueFields)
	uncertain, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-2", TargetID: target.OpportunityID, CandidateID: vague.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionSideBySide, uncertain.Decision)
	require.True(t, uncertain.SuspectedDuplicate)
	afterVague, err := o.OpportunityObservations(ctx, target.OpportunityID)
	require.NoError(t, err)
	require.Len(t, afterVague, 2, "an uncertain duplicate must not merge into the target")
}

// ---- 2. uncertain duplicates stay side by side ------------------------------

func TestReconcileUncertainDuplicatesStaySideBySide(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	first := importKnownJD(t, o, ctx, "rec-side-1", "https://jobs.example.com/postings/2001",
		knownFields("数据工程师", "示例科技", "杭州", "2026届春招", "本科及以上学历"))
	second := importKnownJD(t, o, ctx, "rec-side-2", "https://other.example.net/jobs/2001",
		knownFields("数据工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历"))

	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-side-req-1", TargetID: first.OpportunityID, CandidateID: second.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionSideBySide, receipt.Decision)
	require.True(t, receipt.SuspectedDuplicate, "same title and company with conflicting batch evidence stays flagged")

	// Both records remain independently visible with their own observation.
	firstObs, err := o.OpportunityObservations(ctx, first.OpportunityID)
	require.NoError(t, err)
	require.Len(t, firstObs, 1)
	secondObs, err := o.OpportunityObservations(ctx, second.OpportunityID)
	require.NoError(t, err)
	require.Len(t, secondObs, 1)

	// The decision is durably listed for both opportunities.
	firstDecisions, err := o.OpportunityReconciliations(ctx, first.OpportunityID)
	require.NoError(t, err)
	require.Len(t, firstDecisions, 1)
	require.Equal(t, ReconcileDecisionSideBySide, firstDecisions[0].Decision)
	secondDecisions, err := o.OpportunityReconciliations(ctx, second.OpportunityID)
	require.NoError(t, err)
	require.Len(t, secondDecisions, 1)
	require.Equal(t, second.OpportunityID, secondDecisions[0].CandidateID)
}

// ---- 3. immutable links and check times ------------------------------------

func TestReconcilePreservesAllOriginalLinksAndCheckTimes(t *testing.T) {
	o, transport, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")

	targetURL := "https://jobs.example.com/postings/1001"
	targetImport := importURLComplete(t, o, transport, ctx, "rec-link-target", targetURL)
	o.opportunityExtractor = func(string) (OpportunityFields, error) { return fields, nil }
	appended, err := o.ImportJD(ctx, ImportJDInput{
		RequestID: "rec-link-target-2", RawText: "岗位 1001 补充\n工作职责：\n负责核心服务\n任职要求：\n本科及以上学历",
		SourceLabel: "User supplement", SourceReference: "https://jobs.example.com/postings/1001?from=profile",
		OpportunityID: targetImport.OpportunityID, PriorObservationID: targetImport.ObservationID,
	})
	require.NoError(t, err)

	candidateURL := "https://other.example.net/jobs/1001"
	candidateImport := importURLComplete(t, o, transport, ctx, "rec-link-cand", candidateURL)
	o.opportunityExtractor = func(string) (OpportunityFields, error) { return fields, nil }
	candidateAppended, err := o.ImportJD(ctx, ImportJDInput{
		RequestID: "rec-link-cand-2", RawText: "岗位 1001 补充 B\n工作职责：\n负责核心服务\n任职要求：\n本科及以上学历",
		SourceLabel: "User supplement B", SourceReference: "https://other.example.net/jobs/1001#detail",
		OpportunityID: candidateImport.OpportunityID, PriorObservationID: candidateImport.ObservationID,
	})
	require.NoError(t, err)

	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-link-1", TargetID: targetImport.OpportunityID, CandidateID: candidateImport.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)

	observations, err := o.OpportunityObservations(ctx, targetImport.OpportunityID)
	require.NoError(t, err)
	require.Len(t, observations, 4, "every original observation survives the merge")
	byRef := map[string]OpportunityObservationView{}
	for _, observation := range observations {
		key := observation.Source.ReferenceID
		require.NotEmpty(t, observation.Source.ReferenceID)
		_, dup := byRef[key]
		require.False(t, dup, "each original link appears exactly once")
		byRef[key] = observation
	}
	require.Contains(t, byRef, targetURL)
	require.Contains(t, byRef, "https://jobs.example.com/postings/1001?from=profile")
	require.Contains(t, byRef, candidateURL)
	require.Contains(t, byRef, "https://other.example.net/jobs/1001#detail")
	// Original submitted/final URLs and check times are untouched.
	require.Equal(t, targetURL, byRef[targetURL].SubmittedURL)
	require.Equal(t, targetURL, byRef[targetURL].FinalURL)
	require.Equal(t, targetImport.AcquiredAt.UTC(), byRef[targetURL].AcquiredAt.UTC())
	require.Equal(t, candidateURL, byRef[candidateURL].SubmittedURL)
	require.Equal(t, candidateURL, byRef[candidateURL].FinalURL)
	require.Equal(t, candidateImport.AcquiredAt.UTC(), byRef[candidateURL].AcquiredAt.UTC())
	require.Equal(t, appended.AcquiredAt.UTC(), byRef["https://jobs.example.com/postings/1001?from=profile"].AcquiredAt.UTC())
	require.Equal(t, candidateAppended.AcquiredAt.UTC(), byRef["https://other.example.net/jobs/1001#detail"].AcquiredAt.UTC())

	// No snapshot row is deleted or rewritten by the decision.
	var snapshots int64
	require.NoError(t, o.db.Model(&opportunitySnapshot{}).Where("opportunity_id = ?", targetImport.OpportunityID).Count(&snapshots).Error)
	require.EqualValues(t, 4, snapshots)
}

// ---- 4. explicit expiry / delisting / requirement-change annotations -------

func TestReconcileMarksExpiryDelistingAndRequirementChanges(t *testing.T) {
	o, transport, ctx := newReconciliationOffice(t)

	// Delisting: the latest URL recheck returned 404 — a real import chain
	// observation with a frozen failure classification.
	goneURL := "https://jobs.example.com/postings/4041"
	transport.scripts[goneURL] = scriptedFetch{err: &SourceFetchError{Code: FailureNotFound}}
	gone, err := o.ImportURLReceipt(ctx, ImportURLInput{RequestID: "rec-delist-1", URL: goneURL})
	require.NoError(t, err)
	goneStatus, err := o.OpportunityStatus(ctx, gone.OpportunityID)
	require.NoError(t, err)
	require.Contains(t, goneStatus.Annotations, AnnotationDelisted)
	require.True(t, goneStatus.Stale)
	require.NotZero(t, goneStatus.LastCheckedAt)

	// Expiry and requirement drift: one opportunity, two complete snapshots —
	// the second carries an explicit closing marker and changed requirements.
	scope, err := getScope(ctx)
	require.NoError(t, err)
	expiredID := uuid.NewString()
	require.NoError(t, o.db.Create(&opportunity{ID: expiredID, TenantID: scope.TenantID, UserID: scope.UserID, CreatedAt: time.Now().UTC()}).Error)
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	seedObservation(t, o, ctx, expiredID, SourceStatusComplete, "", "岗位 工程师 工作职责 负责服务 任职要求 本科",
		knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历"), base)
	seedObservation(t, o, ctx, expiredID, SourceStatusComplete, "", "此岗位已截止申请 工作职责 负责服务 任职要求 硕士及以上学历",
		knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "硕士及以上学历"), base.Add(24*time.Hour))
	status, err := o.OpportunityStatus(ctx, expiredID)
	require.NoError(t, err)
	require.Contains(t, status.Annotations, AnnotationExpired)
	require.Contains(t, status.Annotations, AnnotationRequirementsChanged)
	require.NotContains(t, status.Annotations, AnnotationDelisted)
	require.False(t, status.Stale)
	require.Equal(t, base.Add(24*time.Hour).UTC(), status.LastCheckedAt.UTC())
	require.Equal(t, base.Add(24*time.Hour).UTC(), status.LastHealthyAt.UTC())
}

// ---- 5. old applications keep their old snapshot ----------------------------

func TestReconcileOldApplicationsStillShowOldSnapshots(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner", 122)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	seed := seedApplicationEvaluationWithFields(t, o, ctx, fields, "rec-app")

	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)
	application, err := o.CreateApplication(ctx, applicationInput(seed, "rec-app-1", "2027-autumn"))
	require.NoError(t, err)
	require.NotEmpty(t, application.ApplicationID)

	// A later, duplicate posting of the same job arrives from another source
	// and is merged into the applied opportunity.
	duplicate := importKnownJD(t, o, ctx, "rec-app-dup", "https://other.example.net/jobs/1001", fields)
	var appliedRaw string
	require.NoError(t, db.Model(&opportunitySnapshot{}).Where("id = ?", seed.SnapshotID).Pluck("raw_text", &appliedRaw).Error)
	require.NotEmpty(t, appliedRaw)

	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-app-merge-1", TargetID: duplicate.OpportunityID, CandidateID: seed.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)

	// The application keeps showing its pinned old snapshot untouched.
	stored, err := o.Application(ctx, application.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, seed.OpportunityID, stored.PinnedEvidence.OpportunityID)
	require.Equal(t, seed.SnapshotID, stored.PinnedEvidence.SnapshotID)
	evidence, err := o.OpportunityEvidence(ctx, duplicate.OpportunityID, seed.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, appliedRaw, evidence.RawText, "the pinned snapshot body is never rewritten by reconciliation")

	// The merged-away opportunity reports where it went.
	candidateStatus, err := o.OpportunityStatus(ctx, seed.OpportunityID)
	require.NoError(t, err)
	require.Equal(t, duplicate.OpportunityID, candidateStatus.MergedInto)

	// Pre-merge references keep resolving through the merge chain: the old
	// opportunity ID + pinned snapshot ID still returns the untouched body,
	// now attributed to the canonical owner.
	legacy, err := o.OpportunityEvidence(ctx, seed.OpportunityID, seed.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, appliedRaw, legacy.RawText)
	require.Equal(t, duplicate.OpportunityID, legacy.OpportunityID)

	// Re-evaluating the pinned snapshot under the old opportunity ID keeps
	// working instead of 404, and the evaluation lands on the canonical
	// owner so it stays usable for application creation.
	reEvaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID: "rec-app-re-eval", OpportunityID: seed.OpportunityID, SnapshotID: seed.SnapshotID,
	})
	require.NoError(t, err)
	require.Equal(t, duplicate.OpportunityID, reEvaluation.OpportunityID, "a re-evaluation belongs to the canonical owner")

	// The merged-away application row migrates onto the merge target so the
	// one-job-one-batch uniqueness keeps holding across merges, while the
	// stored receipt keeps the pre-merge pinned reference (history intact).
	var migrated applicationRecord
	require.NoError(t, db.Where("id = ?", application.ApplicationID).First(&migrated).Error)
	require.Equal(t, duplicate.OpportunityID, migrated.OpportunityID)
	stillPinned, err := o.Application(ctx, application.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, seed.OpportunityID, stillPinned.PinnedEvidence.OpportunityID, "the pinned evidence body is never rewritten")

	// Creating another application for the same job and batch — evaluated on
	// the canonical target's own snapshot — is refused: one job and batch
	// admits exactly one application even across merges.
	duplicateEvaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID: "rec-app-dup-eval", OpportunityID: duplicate.OpportunityID, SnapshotID: duplicate.SnapshotID,
	})
	require.NoError(t, err)
	_, err = o.CreateApplication(ctx, CreateApplicationInput{
		RequestID: "rec-app-same-batch", OpportunityID: duplicate.OpportunityID, SnapshotID: duplicate.SnapshotID,
		EvaluationID: duplicateEvaluation.EvaluationID, BatchIdentity: "2027-autumn", ExpectedRevision: seed.Revision,
	})
	require.ErrorIs(t, err, ErrApplicationConflict, "same job and batch must not admit a second application across a merge")

	// A different batch still creates a fresh application through the old
	// opportunity ID — the pre-merge reference stays a usable path, not a
	// dead end (the evaluation above was created under the old ID).
	second, err := o.CreateApplication(ctx, CreateApplicationInput{
		RequestID: "rec-app-new-batch", OpportunityID: seed.OpportunityID, SnapshotID: seed.SnapshotID,
		EvaluationID: reEvaluation.EvaluationID, BatchIdentity: "2028-spring", ExpectedRevision: seed.Revision,
	})
	require.NoError(t, err)
	require.NotEmpty(t, second.ApplicationID)
	require.Equal(t, duplicate.OpportunityID, second.PinnedEvidence.OpportunityID, "new applications pin the canonical owner")
	var applications int64
	require.NoError(t, db.Model(&applicationRecord{}).Where("opportunity_id = ?", duplicate.OpportunityID).Count(&applications).Error)
	require.EqualValues(t, 2, applications, "both application rows live under the canonical owner")

	// Appending a user JD through the old opportunity ID joins the merge
	// target's history (canonical owner), not an orphaned pre-merge record.
	// The prior URL observation moved to the target with the merge; resolving
	// it under the old ID must go through the merge chain.
	targetObservations, err := o.OpportunityObservations(ctx, duplicate.OpportunityID)
	require.NoError(t, err)
	var priorObservationID string
	for _, observation := range targetObservations {
		if observation.Source.Kind == "url" && observation.Source.ReferenceID == "https://jobs.example.com/postings/1001" {
			priorObservationID = observation.ObservationID
		}
	}
	require.NotEmpty(t, priorObservationID, "the merged history must carry the pre-merge URL observation")
	o.opportunityExtractor = func(string) (OpportunityFields, error) { return fields, nil }
	appended, err := o.ImportJD(ctx, ImportJDInput{
		RequestID: "rec-app-append", RawText: "岗位 1001 补充\n工作职责：\n负责服务\n任职要求：\n本科",
		SourceLabel: "Post-merge supplement", SourceReference: "https://jobs.example.com/postings/1001?after=merge",
		OpportunityID: seed.OpportunityID, PriorObservationID: priorObservationID,
	})
	require.NoError(t, err)
	require.Equal(t, duplicate.OpportunityID, appended.OpportunityID, "the append joins the canonical owner")
	appendedObs, err := o.OpportunityObservations(ctx, duplicate.OpportunityID)
	require.NoError(t, err)
	require.Len(t, appendedObs, 4, "the merged history keeps growing on the target")
}

func seedApplicationEvaluationWithFields(t *testing.T, o *Office, ctx context.Context, fields OpportunityFields, seedID string) applicationSeed {
	t.Helper()
	// Confirm one skill fact first: applications pin a profile revision and
	// require the profile row to exist.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", seedID+"-fact", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)
	// Seed the opportunity through the URL chain plus a known-fields user
	// supplement, so it owns a URL observation for later append paths.
	urlReceipt := importURLCompleteForOffice(t, o, ctx, seedID+"-url", "https://jobs.example.com/postings/1001")
	o.opportunityExtractor = func(string) (OpportunityFields, error) { return fields, nil }
	job, err := o.ImportJD(ctx, ImportJDInput{
		RequestID: seedID + "-job", RawText: "岗位 1001\n工作职责：\n负责核心服务\n任职要求：\n本科及以上学历",
		SourceLabel: "User supplement", SourceReference: "https://jobs.example.com/postings/1001",
		OpportunityID: urlReceipt.OpportunityID, PriorObservationID: urlReceipt.ObservationID,
	})
	require.NoError(t, err)
	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID: seedID + "-eval", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID,
	})
	require.NoError(t, err)
	return applicationSeed{
		OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID,
		EvaluationID: evaluation.EvaluationID, Revision: view.Revision,
	}
}

// TestReconcileMergeKeepsPreMergeEvaluationsUsableForApplications pins
// ocr3-016: an evaluation created before a merge must keep backing
// application creation afterwards. The merge migrates the evaluation rows
// onto the target, and the application-time comparison resolves the merge
// chain so evaluations merged under older builds (rows still naming the
// merged-away candidate) self-heal instead of failing ErrInvalidRequest
// forever with no recovery path.
func TestReconcileMergeKeepsPreMergeEvaluationsUsableForApplications(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner", 123)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	seed := seedApplicationEvaluationWithFields(t, o, ctx, fields, "eval-merge")
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)

	// A duplicate posting of the same job is merged into the seeded one; the
	// seeded opportunity becomes the merged-away candidate.
	duplicate := importKnownJD(t, o, ctx, "eval-merge-dup", "https://other.example.net/jobs/1001", fields)
	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "eval-merge-1", TargetID: duplicate.OpportunityID, CandidateID: seed.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)

	// The pre-merge evaluation row migrates onto the canonical owner.
	var evaluation evaluationRecord
	require.NoError(t, db.Where("id = ?", seed.EvaluationID).First(&evaluation).Error)
	require.Equal(t, duplicate.OpportunityID, evaluation.OpportunityID,
		"the merge migrates the pre-merge evaluation onto the target")

	// A pre-merge evaluation keeps backing application creation through the
	// old (merged-away) opportunity ID.
	application, err := o.CreateApplication(ctx, applicationInput(seed, "eval-merge-app", "2028-spring"))
	require.NoError(t, err)
	require.Equal(t, duplicate.OpportunityID, application.PinnedEvidence.OpportunityID)

	// Legacy self-heal: evaluations merged under older builds still name the
	// candidate (simulate by rewriting the row back). The application-time
	// comparison resolves the merge chain instead of stranding the row.
	require.NoError(t, db.Model(&evaluationRecord{}).
		Where("id = ?", seed.EvaluationID).
		Update("opportunity_id", seed.OpportunityID).Error)
	legacy, err := o.CreateApplication(ctx, applicationInput(seed, "eval-merge-app-2", "2029-autumn"))
	require.NoError(t, err, "a stranded pre-merge evaluation must stay usable through the merge chain")
	require.Equal(t, duplicate.OpportunityID, legacy.PinnedEvidence.OpportunityID)
}

// ---- 6. source coverage ----------------------------------------------------

func TestReconcileExposesSourceCoverageAndCities(t *testing.T) {
	o, transport, ctx := newReconciliationOffice(t)
	o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{
		{ID: "src-a", Label: "Official Campus", SearchURLTemplate: "https://jobs.example.com/listings?q={query}", AccessMethods: []string{"public https listing"}, Cities: []string{"北京", "杭州"}},
		{ID: "src-b", Label: "National Platform", SearchURLTemplate: "https://other.example.net/jobs?q={query}", AccessMethods: []string{"public https listing"}, Cities: []string{"上海"}},
	}}

	importURLComplete(t, o, transport, ctx, "rec-cov-1", "https://jobs.example.com/postings/1001")
	importKnownJD(t, o, ctx, "rec-cov-2", "https://jobs.example.com/postings/1002",
		knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历"))
	importKnownJD(t, o, ctx, "rec-cov-3", "https://jobs.example.com/postings/1003",
		knownFields("数据工程师", "示例科技", "北京", "2027届秋招", "本科及以上学历"))

	coverage, err := o.SourceCoverage(ctx)
	require.NoError(t, err)
	// Configured sources are the vetted registry projection with real cities.
	require.Len(t, coverage.ConfiguredSources, 2)
	configured := map[string]SearchSourceCoverage{}
	for _, source := range coverage.ConfiguredSources {
		configured[source.SourceID] = source
	}
	require.Equal(t, []string{"北京", "杭州"}, configured["src-a"].Cities)
	require.Equal(t, []string{"上海"}, configured["src-b"].Cities)
	require.True(t, configured["src-a"].Available)

	// Observed sources aggregate what actually produced observations here.
	observed := map[string]ObservedSourceCoverage{}
	for _, source := range coverage.ObservedSources {
		observed[source.SourceKind] = source
	}
	require.Len(t, coverage.ObservedSources, 2)
	require.Equal(t, 1, observed["url"].Observations)
	require.Equal(t, 2, observed["manual_paste"].Observations)
	require.False(t, observed["url"].LastCheckedAt.IsZero())
	require.False(t, observed["manual_paste"].LastCheckedAt.IsZero())

	// Observed cities come from known location evidence, deduplicated.
	require.Equal(t, []string{"北京", "杭州"}, coverage.ObservedCities)
}

// ---- 7. failed rechecks keep the last observation with a stale time ---------

func TestReconcileCheckFailureKeepsLastObservationWithStaleTime(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	scope, err := getScope(ctx)
	require.NoError(t, err)
	opportunityID := uuid.NewString()
	require.NoError(t, o.db.Create(&opportunity{ID: opportunityID, TenantID: scope.TenantID, UserID: scope.UserID, CreatedAt: time.Now().UTC()}).Error)
	healthyAt := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	failedAt := healthyAt.Add(48 * time.Hour)
	seedObservation(t, o, ctx, opportunityID, SourceStatusComplete, "", "岗位 工程师 工作职责 负责服务 任职要求 本科",
		knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历"), healthyAt)
	seedObservation(t, o, ctx, opportunityID, SourceStatusTimedOut, FailureTimeout, "",
		unknownOpportunityFields(), failedAt)

	status, err := o.OpportunityStatus(ctx, opportunityID)
	require.NoError(t, err)
	require.True(t, status.Stale, "a failed recheck marks the data stale, never silently dropped")
	require.Equal(t, failedAt.UTC(), status.LastCheckedAt.UTC())
	require.Equal(t, healthyAt.UTC(), status.LastHealthyAt.UTC(), "the stale time points at the last successful observation")
	require.Empty(t, status.Annotations, "a timeout is not delisting or expiry")

	// Both observations remain visible with their original links and times.
	observations, err := o.OpportunityObservations(ctx, opportunityID)
	require.NoError(t, err)
	require.Len(t, observations, 2)
	require.Equal(t, healthyAt.UTC(), observations[0].AcquiredAt.UTC())
	require.Equal(t, failedAt.UTC(), observations[1].AcquiredAt.UTC())
	require.Equal(t, SourceStatusComplete, observations[0].SourceStatus)
	require.Equal(t, SourceStatusTimedOut, observations[1].SourceStatus)
}

// ---- 8. contract: same job dual sources, distinct batches -------------------

func TestReconcileSameJobDualSourcesAndDistinctBatches(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	// The same job surfaced by two different sources: the posting code,
	// company, location, and batch all agree, so the pair merges.
	dualFields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	first := importKnownJD(t, o, ctx, "rec-con-1", "https://jobs.example.com/postings/1001", dualFields)
	second := importKnownJD(t, o, ctx, "rec-con-2", "https://other.example.net/jobs/1001", dualFields)
	merged, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-con-merge", TargetID: first.OpportunityID, CandidateID: second.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, merged.Decision)
	require.Equal(t, "1001", merged.Evidence.Target.JobCode)
	require.Equal(t, "1001", merged.Evidence.Candidate.JobCode)
	require.NotEqual(t, merged.Evidence.Target.SourceRef, merged.Evidence.Candidate.SourceRef, "both original source links are kept as evidence")

	// Same title and company in a different recruiting batch never merges:
	// distinct batches are separate application targets by spec.
	batchA := knownFields("数据工程师", "示例科技", "杭州", "2026届秋招", "本科及以上学历")
	batchB := knownFields("数据工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	firstBatch := importKnownJD(t, o, ctx, "rec-con-3", "https://jobs.example.com/postings/3001", batchA)
	secondBatch := importKnownJD(t, o, ctx, "rec-con-4", "https://jobs.example.com/postings/3001", batchB)
	split, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-con-split", TargetID: firstBatch.OpportunityID, CandidateID: secondBatch.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionSideBySide, split.Decision)
	require.True(t, split.SuspectedDuplicate)
	require.Equal(t, "2026届秋招", split.Evidence.Target.Batch)
	require.Equal(t, "2027届秋招", split.Evidence.Candidate.Batch)
	firstObs, err := o.OpportunityObservations(ctx, firstBatch.OpportunityID)
	require.NoError(t, err)
	require.Len(t, firstObs, 1, "distinct batches stay separate records")
}

// ---- 9. house semantics: replay, conflict, unknown recovery ------------------

func TestReconcileExactReplayAndChangedIntentConflict(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-house-1", "https://jobs.example.com/postings/1001", fields)
	candidate := importKnownJD(t, o, ctx, "rec-house-2", "https://other.example.net/jobs/1001", fields)
	input := ReconcileInput{RequestID: "rec-house-req", TargetID: target.OpportunityID, CandidateID: candidate.OpportunityID}

	first, err := o.ReconcileOpportunities(ctx, input)
	require.NoError(t, err)

	// Exact replay returns the stored receipt and stores nothing new.
	replay, err := o.ReconcileOpportunities(ctx, input)
	require.NoError(t, err)
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	require.JSONEq(t, string(firstJSON), string(replayJSON))
	var decisions int64
	require.NoError(t, o.db.Model(&reconciliationRecord{}).Count(&decisions).Error)
	require.EqualValues(t, 1, decisions)

	// The same request ID with changed content is a typed conflict.
	third := importKnownJD(t, o, ctx, "rec-house-3", "https://other.example.net/jobs/1001?x=1", fields)
	changedCandidate := input
	changedCandidate.CandidateID = third.OpportunityID
	_, err = o.ReconcileOpportunities(ctx, changedCandidate)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	// The stored receipt replays through the receipt endpoint too.
	stored, err := o.FindReconciliationReceipt(ctx, input.RequestID)
	require.NoError(t, err)
	require.Equal(t, first.RequestID, stored.RequestID)
	require.Equal(t, first.Decision, stored.Decision)

	// Unknown outcome recovery: a commit failure after the decision row is
	// written surfaces as an error, and retrying the same request ID settles
	// into exactly one durable decision.
	fourth := importKnownJD(t, o, ctx, "rec-house-4", "https://jobs.example.com/postings/1001?y=1", fields)
	retryInput := ReconcileInput{RequestID: "rec-house-retry", TargetID: target.OpportunityID, CandidateID: fourth.OpportunityID}
	o.failReconcileCommit = func() error { return fmt.Errorf("injected commit failure") }
	_, err = o.ReconcileOpportunities(ctx, retryInput)
	require.EqualError(t, err, "injected commit failure")
	o.failReconcileCommit = nil
	recovered, err := o.ReconcileOpportunities(ctx, retryInput)
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, recovered.Decision)
	require.NoError(t, o.db.Model(&reconciliationRecord{}).Where("request_id = ?", retryInput.RequestID).Count(&decisions).Error)
	require.EqualValues(t, 1, decisions)
}

// ---- purge and boundary inclusion (T19/T20/T21 precedent) -------------------

func TestReconciliationTableIncludedInDeletionPurgeAndBoundary(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-purge-1", "https://jobs.example.com/postings/1001", fields)
	candidate := importKnownJD(t, o, ctx, "rec-purge-2", "https://other.example.net/jobs/1001", fields)
	_, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-purge-req", TargetID: target.OpportunityID, CandidateID: candidate.OpportunityID,
	})
	require.NoError(t, err)

	require.Contains(t, careerPurgeTables, "career_reconciliations", "the purge list must include reconciliation decisions")

	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	var section *CareerDeletionSection
	for i := range boundary.InSpace {
		if boundary.InSpace[i].Section == "reconciliations" {
			section = &boundary.InSpace[i]
		}
	}
	require.NotNil(t, section, "the deletion boundary must disclose reconciliation decisions")
	require.Equal(t, 1, section.Count)
}

// ---- merge with pre-existing same-batch applications on both records -------

func TestReconcileMergeWithDualSameBatchApplicationsKeepsHistoryReachable(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner", 123)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	firstSeed := seedApplicationEvaluationWithFields(t, o, ctx, fields, "rec-dual-a")
	secondSeed := seedApplicationEvaluationWithFields(t, o, ctx, fields, "rec-dual-b")

	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	currentRevision := func() uint64 {
		view, err := o.Open(ctx)
		require.NoError(t, err)
		return view.Revision
	}
	firstInput := CreateApplicationInput{
		RequestID: "rec-dual-app-a", OpportunityID: firstSeed.OpportunityID, SnapshotID: firstSeed.SnapshotID,
		EvaluationID: firstSeed.EvaluationID, BatchIdentity: "2027-autumn", ExpectedRevision: currentRevision(),
	}
	firstReceipt, err := o.CreateApplication(ctx, firstInput)
	require.NoError(t, err)
	secondInput := CreateApplicationInput{
		RequestID: "rec-dual-app-b", OpportunityID: secondSeed.OpportunityID, SnapshotID: secondSeed.SnapshotID,
		EvaluationID: secondSeed.EvaluationID, BatchIdentity: "2027-autumn", ExpectedRevision: currentRevision(),
	}
	secondReceipt, err := o.CreateApplication(ctx, secondInput)
	require.NoError(t, err)

	// Both records legitimately held a same-batch application while they were
	// distinct jobs. The sufficient-evidence merge must still execute — not
	// die on the one-job-one-batch unique index with an unclassified 500.
	receipt, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-dual-merge", TargetID: secondSeed.OpportunityID, CandidateID: firstSeed.OpportunityID,
	})
	require.NoError(t, err)
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)
	require.Equal(t, []string{"2027-autumn"}, receipt.ConflictingBatches, "the receipt discloses the conflicting batch")

	// The decision is durable and recoverable by replay.
	var decisions int64
	require.NoError(t, o.db.Model(&reconciliationRecord{}).Count(&decisions).Error)
	require.EqualValues(t, 1, decisions)

	// The conflicting application row stays on the merged-away record —
	// honest, reachable history; the non-conflicting one migrated.
	var onTarget, onCandidate int64
	require.NoError(t, db.Model(&applicationRecord{}).Where("opportunity_id = ?", secondSeed.OpportunityID).Count(&onTarget).Error)
	require.EqualValues(t, 1, onTarget)
	require.NoError(t, db.Model(&applicationRecord{}).Where("opportunity_id = ?", firstSeed.OpportunityID).Count(&onCandidate).Error)
	require.EqualValues(t, 1, onCandidate)

	// Both applications remain fully readable with their pinned evidence.
	storedFirst, err := o.Application(ctx, firstReceipt.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, firstSeed.SnapshotID, storedFirst.PinnedEvidence.SnapshotID)
	storedSecond, err := o.Application(ctx, secondReceipt.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, secondSeed.SnapshotID, storedSecond.PinnedEvidence.SnapshotID)

	// A NEW same-batch application on the canonical owner is still refused:
	// one job and batch admits exactly one application going forward.
	duplicateEvaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID: "rec-dual-eval", OpportunityID: secondSeed.OpportunityID, SnapshotID: secondSeed.SnapshotID,
	})
	require.NoError(t, err)
	_, err = o.CreateApplication(ctx, CreateApplicationInput{
		RequestID: "rec-dual-app-c", OpportunityID: secondSeed.OpportunityID, SnapshotID: secondSeed.SnapshotID,
		EvaluationID: duplicateEvaluation.EvaluationID, BatchIdentity: "2027-autumn", ExpectedRevision: secondSeed.Revision,
	})
	require.ErrorIs(t, err, ErrApplicationConflict)

	// A different batch on the canonical owner still creates freely.
	third, err := o.CreateApplication(ctx, CreateApplicationInput{
		RequestID: "rec-dual-app-d", OpportunityID: secondSeed.OpportunityID, SnapshotID: secondSeed.SnapshotID,
		EvaluationID: duplicateEvaluation.EvaluationID, BatchIdentity: "2028-spring", ExpectedRevision: secondSeed.Revision,
	})
	require.NoError(t, err)
	require.NotEmpty(t, third.ApplicationID)
}

// ---- scope ------------------------------------------------------------------

func TestReconcileRejectsForeignAndMalformedRequests(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-scope-1", "https://jobs.example.com/postings/1001", fields)

	_, err := o.ReconcileOpportunities(ctx, ReconcileInput{RequestID: "", TargetID: target.OpportunityID, CandidateID: target.OpportunityID})
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = o.ReconcileOpportunities(ctx, ReconcileInput{RequestID: "rec-scope-req", TargetID: target.OpportunityID, CandidateID: target.OpportunityID})
	require.ErrorIs(t, err, ErrInvalidRequest, "an opportunity cannot merge with itself")
	_, err = o.ReconcileOpportunities(ctx, ReconcileInput{RequestID: "rec-scope-req2", TargetID: uuid.NewString(), CandidateID: target.OpportunityID})
	require.ErrorIs(t, err, ErrOpportunityNotFound)

	// Requests without an authenticated scope are rejected.
	_, err = o.SourceCoverage(context.Background())
	require.ErrorIs(t, err, ErrUnauthorized)

	// Cross-scope isolation: another owner's space sees none of these rows.
	foreign, foreignCtx := newSourceImportOffice(t, "intruder", 121)
	_, err = foreign.OpportunityStatus(foreignCtx, target.OpportunityID)
	require.ErrorIs(t, err, ErrOpportunityNotFound)
	_, err = foreign.ReconcileOpportunities(foreignCtx, ReconcileInput{
		RequestID: "rec-foreign", TargetID: target.OpportunityID, CandidateID: uuid.NewString(),
	})
	require.ErrorIs(t, err, ErrOpportunityNotFound)
}

// ---- OCR round 2 fixes ---------------------------------------------------

// TestReconcileEvidenceReadFailureAbortsDecision pins ocr2-148: a failed
// evidence read aborts the whole decision — empty evidence must never degrade
// into a durable side_by_side receipt that the same request ID replays
// forever.
func TestReconcileEvidenceReadFailureAbortsDecision(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-evfail-1", "https://jobs.example.com/postings/3001", fields)
	candidate := importKnownJD(t, o, ctx, "rec-evfail-2", "https://other.example.net/jobs/3001", fields)

	// The evidence source becomes unreadable mid-decision.
	require.NoError(t, o.db.Exec("DROP TABLE career_opportunity_snapshots").Error)

	_, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-evfail-req", TargetID: target.OpportunityID, CandidateID: candidate.OpportunityID,
	})
	require.Error(t, err, "an unreadable evidence source must abort the decision")
	require.NotErrorIs(t, err, ErrOutcomeUnknown)

	var decisions int64
	require.NoError(t, o.db.Model(&reconciliationRecord{}).Count(&decisions).Error)
	require.Zero(t, decisions, "no decision may persist when the evidence read failed")
}

// TestReconcileBusyExhaustionIsTypedUnknownOutcome pins ocr2-149: when the
// bounded busy-retry budget runs out while the parent context stays healthy,
// the caller gets the typed outcome_unknown carrying the request ID — never a
// raw "database is locked" 500 — and nothing durable was decided.
func TestReconcileBusyExhaustionIsTypedUnknownOutcome(t *testing.T) {
	o, _, ctx := newReconciliationOffice(t)
	fields := knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历")
	target := importKnownJD(t, o, ctx, "rec-busy-1", "https://jobs.example.com/postings/3002", fields)
	candidate := importKnownJD(t, o, ctx, "rec-busy-2", "https://other.example.net/jobs/3002", fields)

	o.failReconcileCommit = func() error { return sqlite3.Error{Code: sqlite3.ErrBusy} }
	_, err := o.ReconcileOpportunities(ctx, ReconcileInput{
		RequestID: "rec-busy-req", TargetID: target.OpportunityID, CandidateID: candidate.OpportunityID,
	})
	o.failReconcileCommit = nil

	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown, "busy exhaustion must converge to the typed unknown outcome, got: %v", err)
	require.Equal(t, "rec-busy-req", unknown.RequestID)
	require.NotContains(t, err.Error(), "database is locked", "the raw lock error must not leak")

	var decisions int64
	require.NoError(t, o.db.Model(&reconciliationRecord{}).Count(&decisions).Error)
	require.Zero(t, decisions, "the busy transaction rolled back whole")
}
