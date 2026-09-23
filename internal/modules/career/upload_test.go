package career

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type careerUploadFiles struct {
	interfaces.FileService
	stored, deleted string
	temporary       bool
}

func (f *careerUploadFiles) SaveBytes(_ context.Context, _ []byte, _ uint64, name string, temporary bool) (string, error) {
	f.stored, f.temporary = name, temporary
	return "private://" + name, nil
}
func (f *careerUploadFiles) DeleteFile(_ context.Context, ref string) error {
	f.deleted = ref
	return nil
}

type careerUploadCatalog struct {
	interfaces.ResourceCatalog
	bound, released string
}

func (c *careerUploadCatalog) Bind(_ context.Context, reference, _, _, _ string) error {
	c.bound = reference
	return nil
}
func (c *careerUploadCatalog) Release(_ context.Context, reference, _, _ string) (int64, error) {
	c.released = reference
	return 0, nil
}

type careerUploadReader struct {
	interfaces.DocumentReader
	result *types.ReadResult
	err    error
}

func (r careerUploadReader) Read(context.Context, *types.ReadRequest) (*types.ReadResult, error) {
	return r.result, r.err
}

func TestUploadAdapterValidatesStoresAndParsesDurableCareerSource(t *testing.T) {
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{}
	reader := careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}}
	adapter := NewUploadAdapter(files, catalog, reader)
	processing := false
	result, err := adapter.StoreAndParse(context.Background(), 9, "resume.pdf", "application/pdf", []byte("%PDF-1.7 resume"), func(result UploadResult) error {
		processing = true
		require.NotEmpty(t, result.SourceID)
		require.Equal(t, result.SourceID, result.Upload.ID)
		require.Equal(t, "private://"+files.stored, result.Upload.ResourceRef)
		return nil
	})
	require.NoError(t, err)
	require.True(t, processing)
	require.False(t, files.temporary, "career sources must not use expiring chat storage")
	require.Equal(t, "Education: Example University", result.Upload.Text)
	require.Equal(t, "private://"+files.stored, catalog.bound)
	require.Empty(t, files.deleted)
}

func TestUploadAdapterRejectsMismatchedAndFailedParserInputs(t *testing.T) {
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{}
	adapter := NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: "should not parse"}})
	_, err := adapter.StoreAndParse(context.Background(), 9, "resume.pdf", "application/pdf", []byte("MZ executable"), nil)
	require.ErrorContains(t, err, "does not match PDF")
	require.Empty(t, files.stored)

	parseFailure := errors.New("parser unavailable")
	adapter = NewUploadAdapter(files, catalog, careerUploadReader{err: parseFailure})
	stored := false
	result, err := adapter.StoreAndParse(context.Background(), 9, "resume.pdf", "application/pdf", []byte("%PDF-1.7 resume"), func(result UploadResult) error {
		stored = true
		return nil
	})
	require.ErrorIs(t, err, parseFailure)
	require.True(t, stored)
	require.NotEmpty(t, result.SourceID)
	require.Equal(t, "private://"+files.stored, catalog.released)
	require.Equal(t, "private://"+files.stored, files.deleted)
}

func TestDeterministicUploadExtractionCreatesOnlyPendingEvidenceLinkedProposals(t *testing.T) {
	office, ctx := testOffice(t)
	text := "Education: Example University\nExperience: Acme 2022-2024\nExperience: Acme 2023-2025\nProject: Compiler optimization\nSkill: Go\nAchievement: Reduced latency 30%\nCertificate: Cloud Architect"
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{}
	adapter := NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: text}})
	result, err := adapter.StoreAndParse(ctx, 1, "resume.pdf", "application/pdf", []byte("%PDF-1.7 resume"), func(upload UploadResult) error {
		_, createErr := office.CreateProcessingSource(ctx, upload.Upload)
		return createErr
	})
	require.NoError(t, err)
	receipt, err := office.CompleteIntake(ctx, result.SourceID, result.Upload.Text, result.Fields, result.MissingCategories, result.ReviewFlags, "upload-batch", 0)
	require.NoError(t, err)
	source, err := office.GetSource(ctx, result.SourceID)
	require.NoError(t, err)
	require.Equal(t, "ready", source.Status)
	require.Contains(t, source.MissingCategories, "education.graduation_year")
	require.Contains(t, source.ReviewFlags, "multiple_experience_claims_require_review")
	require.Len(t, receipt.Proposals, 7)
	for _, proposal := range receipt.Proposals {
		require.Equal(t, "pending", proposal.Status)
		require.Equal(t, result.SourceID, proposal.Source.ReferenceID)
		require.NotEmpty(t, proposal.Evidence)
	}
	view, err := office.Open(ctx)
	require.NoError(t, err)
	require.Empty(t, view.Facts)
	require.Len(t, view.Proposals, 7)
}
