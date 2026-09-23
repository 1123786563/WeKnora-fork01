package career

import (
	"context"
	"testing"

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
	require.Equal(t, "revision_conflict", failed.ErrorCategory)
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
