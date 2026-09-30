package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompleteIntakeCreatesAtomicPendingBatchAndReplays(t *testing.T) {
	o, ctx := testOffice(t)
	intake, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "intake-source-1", FileName: "resume.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:test", ResourceRef: "private://resume"})
	require.NoError(t, err)
	require.Equal(t, "processing", intake.Status)
	fields := []ExtractedField{{Key: "education.school", Value: "Example University"}, {Key: "experience.company", Value: "Example Co"}, {Key: "project.name", Value: "Compiler"}, {Key: "skill.go", Value: "Go"}, {Key: "achievement.latency", Value: "Reduced latency 30%"}, {Key: "certificate.name", Value: "Cloud cert"}}
	batch, err := o.CompleteIntake(ctx, intake.ID, "source text", fields, nil, nil, "batch-1", 0)
	require.NoError(t, err)
	require.Len(t, batch.Proposals, len(fields))
	for _, proposal := range batch.Proposals {
		require.Equal(t, "pending", proposal.Status)
		require.Equal(t, intake.ID, proposal.Source.ReferenceID)
	}
	replayed, err := o.CompleteIntake(ctx, intake.ID, "source text", fields, nil, nil, "batch-1", 0)
	require.NoError(t, err)
	require.Equal(t, batch, replayed)
	reversed := append([]ExtractedField(nil), fields...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	reorderedReplay, err := o.CompleteIntake(ctx, intake.ID, "source text", reversed, nil, nil, "batch-1", 0)
	require.NoError(t, err)
	require.Equal(t, batch, reorderedReplay)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Empty(t, view.Facts)
	require.Len(t, view.Proposals, len(fields))
}

func TestCompleteIntakeRejectsChangedReplayAndStaleRevisionAtomically(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "degree", "bachelor", "confirmed-1", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	intake, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "intake-source-2", FileName: "resume.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:test", ResourceRef: "private://resume"})
	require.NoError(t, err)
	fields := []ExtractedField{{Key: "education.school", Value: "Example University"}}
	_, err = o.CompleteIntake(ctx, intake.ID, "source text", fields, nil, nil, "batch-1", 1)
	require.NoError(t, err)
	_, err = o.CompleteIntake(ctx, intake.ID, "source text", []ExtractedField{{Key: "education.school", Value: "Other University"}}, nil, nil, "batch-1", 1)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	second, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "intake-source-3", FileName: "resume2.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:other", ResourceRef: "private://resume2"})
	require.NoError(t, err)
	_, err = o.CompleteIntake(ctx, second.ID, "source text", fields, nil, nil, "batch-2", 1)
	require.ErrorIs(t, err, ErrRevisionConflict)
	failed, err := o.FinishSource(ctx, second.ID, "", nil, nil, ErrRevisionConflict)
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.Equal(t, "cleanup_pending_revision_conflict", failed.ErrorCategory)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), view.Revision)
	require.Len(t, view.Facts, 1)
	require.Equal(t, "bachelor", view.Facts[0].Value)
	require.Len(t, view.Proposals, 1)
}

func TestFailedIntakeDoesNotChangeConfirmedFactsAndModelInputIsScoped(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "skill.go", "Go", "confirm-1", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Propose(ctx, "experience.company", "unconfirmed role", "propose-1", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.CreateFailedSource(ctx, SourceUpload{FileName: "bad.pdf", MIMEType: "application/pdf", Size: 5, Digest: "sha256:bad"}, "unsupported content")
	require.NoError(t, err)
	_, err = o.CreateSource(ctx, SourceUpload{FileName: "resume.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:raw", ResourceRef: "private://raw", Text: "raw resume with 4111111111111111"}, "")
	require.NoError(t, err)
	before, err := o.Open(ctx)
	require.NoError(t, err)
	input, err := o.BuildModelInput(ctx, "profile_summary")
	require.NoError(t, err)
	require.Len(t, input.Facts, 1)
	require.Equal(t, "Go", input.Facts[0].Value)
	require.NotContains(t, input.String(), "4111111111111111")
	require.NotContains(t, input.String(), "unconfirmed")
	require.NotContains(t, input.String(), "raw resume with")
	after, err := o.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, before.Facts, after.Facts)
}

func TestModelInputOmitsIdentityFactsAndRedactsIdentityNumbers(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "education.school", "Example University", "c1", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "identity.national_id", "110105199001011234", "c2", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "graduation_year", "2027", "c3", 2, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "experience.company", "Acme 110105199001011234", "c4", 3, Source{Kind: "manual"})
	require.NoError(t, err)
	input, err := o.BuildModelInput(ctx, "profile_summary")
	require.NoError(t, err)
	require.Contains(t, input.String(), "Example University")
	require.NotContains(t, input.String(), "110105199001011234")
	require.Contains(t, input.String(), "2027")
	require.Contains(t, input.String(), "[REDACTED]")
}

func TestModelInputIncludesOnlyConfirmedInternshipCompany(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "internship.company", "Confirmed Internship Co", "internship-confirm", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.Propose(ctx, "internship.company", "Proposed Internship Co", "internship-propose", 1, Source{Kind: "resume_extraction"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "unapproved.private_note", "Do not send to model", "private-note-confirm", 2, Source{Kind: "manual"})
	require.NoError(t, err)

	for _, purpose := range []string{"profile_summary", "qualification_evaluation", "material_generation"} {
		t.Run(purpose, func(t *testing.T) {
			input, err := o.BuildModelInput(ctx, purpose)
			require.NoError(t, err)
			encoded := input.String()
			require.Contains(t, encoded, `"key":"internship.company","value":"Confirmed Internship Co"`)
			require.NotContains(t, encoded, "Proposed Internship Co")
			require.NotContains(t, encoded, "unapproved.private_note")
			require.NotContains(t, encoded, "Do not send to model")
		})
	}
}

func TestModelInputExcludesNestedDocumentFieldsAndSeparatedNumbers(t *testing.T) {
	o, ctx := testOffice(t)
	facts := []struct{ key, value string }{
		{"education.school", "Example University"},
		{"education.passport_number", "AB-12345678"},
		{"experience.id_card", "110105 19900101 1234"},
		{"education.details", "Graduated 2027; passport AB 12345678"},
		{"experience.achievement", "Reduced latency 30%, document X110105-19900101-1234Z"},
	}
	for i, fact := range facts {
		_, err := o.Confirm(ctx, fact.key, fact.value, fmt.Sprintf("safe-model-%d", i), uint64(i), Source{Kind: "manual"})
		require.NoError(t, err)
	}
	input, err := o.BuildModelInput(ctx, "profile_summary")
	require.NoError(t, err)
	encoded := input.String()
	for _, secret := range []string{"passport_number", "id_card", "AB-12345678", "AB 12345678", "110105 19900101 1234", "110105-19900101-1234"} {
		require.NotContains(t, encoded, secret)
	}
	require.Contains(t, encoded, "Example University")
	require.Contains(t, encoded, "Reduced latency 30%")
}

func TestWebConfirmedFactsReachRelevantModelPurposes(t *testing.T) {
	o, ctx := testOffice(t)
	facts := []struct{ key, value string }{
		{"毕业时间", "2027"},
		{"学历", "本科"},
		{"城市", "上海"},
		{"意向", "后端工程师"},
		{"education.passport_number", "AB-12345678"},
	}
	for i, fact := range facts {
		_, err := o.Confirm(ctx, fact.key, fact.value, fmt.Sprintf("web-safe-%d", i), uint64(i), Source{Kind: "manual"})
		require.NoError(t, err)
	}
	for _, purpose := range []string{"profile_summary", "qualification_evaluation", "material_generation"} {
		t.Run(purpose, func(t *testing.T) {
			input, err := o.BuildModelInput(ctx, purpose)
			require.NoError(t, err)
			encoded := input.String()
			for _, fact := range facts[:4] {
				require.Contains(t, encoded, fact.key)
				require.Contains(t, encoded, fact.value)
			}
			require.NotContains(t, encoded, "passport_number")
			require.NotContains(t, encoded, "AB-12345678")
		})
	}
}

func TestIntakeSourceIsTenantOwnerScoped(t *testing.T) {
	o, ctx := testOffice(t)
	created, err := o.CreateSource(ctx, SourceUpload{FileName: "resume.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:test", ResourceRef: "private://resume", Text: "source text"}, "")
	require.NoError(t, err)
	other := WithScope(context.Background(), Scope{UserID: "other", TenantID: 2})
	require.NoError(t, o.ClaimSpace(other))
	_, err = o.GetSource(other, created.ID)
	require.ErrorIs(t, err, ErrSourceNotFound)
	otherOwner := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 1})
	_, err = o.GetSource(otherOwner, created.ID)
	require.ErrorIs(t, err, ErrUnauthorized)
}

func TestSourceLifecyclePersistsProcessingReadyAndFailedWithoutReplacingFacts(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "education.school", "Confirmed University", "confirm-source-lifecycle", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	processing, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "source-processing", FileName: "resume.pdf", MIMEType: "application/pdf", Size: 100, Digest: "sha256:processing", ResourceRef: "private://processing"})
	require.NoError(t, err)
	require.Equal(t, "processing", processing.Status)
	require.NotContains(t, processing.String(), "private://processing")
	ready, err := o.FinishSource(ctx, processing.ID, "parsed resume text", nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "ready", ready.Status)
	failed, err := o.CreateFailedSource(ctx, SourceUpload{FileName: "bad.pdf", MIMEType: "application/pdf", Size: 10, Digest: "sha256:failed"}, "private parser detail")
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.NotContains(t, failed.ErrorMessage, "private parser detail")
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Len(t, view.Facts, 1)
	require.Equal(t, "Confirmed University", view.Facts[0].Value)
}

func TestSameCategoryExtractedItemsRemainDistinctAfterConfirmation(t *testing.T) {
	o, ctx := testOffice(t)
	text := "Experience: Acme, 2022-2024, backend engineer\nExperience: Beta, 2020-2022, analyst"
	fields, _, _ := ExtractResumeFields(text)
	source, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "multi-exp", FileName: "resume.txt", MIMEType: "text/plain", Size: int64(len(text)), Digest: "digest", ResourceRef: "ref"})
	require.NoError(t, err)
	_, err = o.CompleteIntake(ctx, source.ID, text, fields, nil, nil, "multi-exp-batch", 0)
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Len(t, view.Proposals, 2)
	for i, p := range view.Proposals {
		_, err = o.Act(ctx, "confirm_proposal", p.ID, "", "", fmt.Sprintf("confirm-%d", i), uint64(i+1), Source{Kind: "user_confirmation"})
		require.NoError(t, err)
	}
	view, err = o.Open(ctx)
	require.NoError(t, err)
	require.Len(t, view.Facts, 2)
	require.NotEqual(t, view.Facts[0].Key, view.Facts[1].Key)
}

func TestPurposeModelJSONOmitsIdentityFromKeyAndSourceMetadata(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "experience.company", "AcmeX110105199001011234A", "identity-meta-confirm", 0, Source{Kind: "manual", Label: "110105199001011234A", ReferenceID: "source-110105199001011234A"})
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "certificate.details", "PassportAB12345678Z", "passport-meta-confirm", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	input, err := o.BuildModelInput(ctx, "profile_summary")
	require.NoError(t, err)
	encoded := input.String()
	require.NotContains(t, encoded, "110105199001011234")
	require.Contains(t, encoded, "AcmeX[REDACTED]A")
	require.NotContains(t, encoded, "AB12345678")
	require.Contains(t, encoded, "Passport[REDACTED]Z")
	require.NotContains(t, encoded, "source")
}

func TestUploadClaimReplaysAndRejectsChangedIntentBeforeWork(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 12, Digest: "digest-a", RequestID: "request-a", IntentHash: "intent-a", ExpectedRevision: 0}
	first, terminal, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.False(t, terminal)
	require.Equal(t, "processing", first.Status)
	encoded, _ := json.Marshal(first)
	require.NotContains(t, string(encoded), first.ClaimToken)
	require.NotContains(t, string(encoded), "private://")
	replay, terminal, err := o.ClaimUpload(ctx, u)
	require.ErrorIs(t, err, ErrUploadInProgress)
	require.False(t, terminal)
	require.Equal(t, first.ID, replay.ID)
	changed := u
	changed.Digest = "digest-b"
	_, _, err = o.ClaimUpload(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	changed = u
	changed.IntentHash = "intent-b"
	_, _, err = o.ClaimUpload(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	changed = u
	changed.ExpectedRevision = 9
	_, _, err = o.ClaimUpload(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", first.ID).Update("lease_until", time.Now().Add(-time.Minute)).Error)
	recovered, terminal, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.False(t, terminal)
	require.Equal(t, first.ID, recovered.ID)
	require.NotEqual(t, first.ClaimToken, recovered.ClaimToken)
}

func TestStaleProcessingUploadBecomesVisibleTerminalFailure(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 12, Digest: "digest-stale", RequestID: "request-stale", IntentHash: "intent-stale"}
	source, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.NoError(t, o.PersistUploadResource(ctx, source.ID, source.ClaimToken, "private://stale"))
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", source.ID).Update("lease_until", time.Now().Add(-time.Hour)).Error)
	resources, err := o.FailStaleUploads(ctx, time.Now())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Equal(t, "private://stale", resources[0].ResourceRef)
	failed, err := o.GetSource(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.Equal(t, "cleanup_pending_interrupted", failed.ErrorCategory)
	replayed, terminal, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.True(t, terminal)
	require.Equal(t, failed.ID, replayed.ID)
	require.Equal(t, "failed", replayed.Status)
}

func TestUploadClaimTakeoverFencesOldWriterWithoutReplacingResource(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 12, Digest: "digest-fence", RequestID: "request-fence", IntentHash: "intent-fence"}
	old, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.NoError(t, o.PersistUploadResource(ctx, old.ID, old.ClaimToken, "private://original"))
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", old.ID).Update("lease_until", time.Now().Add(-time.Second)).Error)
	current, terminal, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.False(t, terminal)
	require.Equal(t, old.ID, current.ID)
	require.NotEqual(t, old.ClaimToken, current.ClaimToken)
	require.Equal(t, "private://original", current.ResourceRef)
	stale, err := o.FailStaleUploads(ctx, time.Now())
	require.NoError(t, err)
	require.Empty(t, stale, "a renewed claim cannot be cleaned by the stale worker")
	require.ErrorIs(t, o.PersistUploadResource(ctx, old.ID, old.ClaimToken, "private://late"), ErrUploadClaimLost)
	_, err = o.FinishSourceClaim(ctx, old.ID, old.ClaimToken, "", nil, nil, errors.New("old parser failed"))
	require.ErrorIs(t, err, ErrUploadClaimLost)
	_, err = o.CompleteIntakeClaim(ctx, current.ID, old.ClaimToken, "text", []ExtractedField{{Key: "education.school", Value: "Old"}}, nil, nil, current.ID+":batch", 0)
	require.ErrorIs(t, err, ErrUploadClaimLost)
	row, err := o.privateSource(ctx, current.ID)
	require.NoError(t, err)
	require.Equal(t, "private://original", row.ResourceRef)
	require.Equal(t, current.ClaimToken, row.ClaimToken)
	receipt, err := o.CompleteIntakeClaim(ctx, current.ID, current.ClaimToken, "text", []ExtractedField{{Key: "education.school", Value: "Current"}}, nil, nil, current.ID+":batch", 0)
	require.NoError(t, err)
	require.Len(t, receipt.Proposals, 1)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Len(t, view.Proposals, 1)
	require.Equal(t, "Current", view.Proposals[0].Value)
}

func TestUploadClaimTakeoverRequiresLeaseStillExpiredAtUpdate(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 12, Digest: "digest-renew", RequestID: "request-renew", IntentHash: "intent-renew"}
	claim, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	// Model a lease renewal after the takeover worker read the expired claim but
	// before its conditional update. The update must recheck expiry in SQL.
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", claim.ID).Update("lease_until", time.Now().UTC().Add(30*time.Minute)).Error)
	replay, terminal, err := o.ClaimUpload(ctx, u)
	require.ErrorIs(t, err, ErrUploadInProgress)
	require.False(t, terminal)
	require.Equal(t, claim.ClaimToken, replay.ClaimToken)
	require.Equal(t, "processing", replay.Status)
}
