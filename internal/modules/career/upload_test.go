package career

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type careerUploadFiles struct {
	interfaces.FileService
	stored, deleted string
	temporary       bool
	saves           int
	entered, resume chan struct{}
}

func (f *careerUploadFiles) SaveBytes(_ context.Context, _ []byte, _ uint64, name string, temporary bool) (string, error) {
	f.stored, f.temporary = name, temporary
	f.saves++
	if f.entered != nil {
		close(f.entered)
		<-f.resume
	}
	return "private://" + name, nil
}

func TestUploadHTTPRequestIDReplayAndChangedBytesConflictBeforeStorage(t *testing.T) {
	o, _ := testOffice(t)
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}})}
	request := func(data, requestID string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
		partHeader.Set("Content-Type", "text/plain")
		part, _ := mw.CreatePart(partHeader)
		_, _ = part.Write([]byte(data))
		_ = mw.WriteField("requestId", requestID)
		_ = mw.WriteField("expectedRevision", "0")
		_ = mw.Close()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
		req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		c.Request = req.WithContext(ctx)
		h.Upload(c)
		return rec
	}
	first := request("Resume plaintext", "resume-request-1")
	require.Equal(t, 201, first.Code)
	var one UploadResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &one))
	require.Equal(t, "ready", one.Source.Status)
	require.NotNil(t, one.Receipt)
	require.Equal(t, 1, files.saves)
	replay := request("Resume plaintext", "resume-request-1")
	require.Equal(t, 200, replay.Code)
	var two UploadResponse
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &two))
	require.Equal(t, one.Source.ID, two.Source.ID)
	require.Equal(t, *one.Receipt, *two.Receipt)
	require.Equal(t, 1, files.saves)
	changed := request("Different resume", "resume-request-1")
	require.Equal(t, 409, changed.Code)
	require.Equal(t, 1, files.saves)
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
	calls  *int
}

func (r careerUploadReader) Read(context.Context, *types.ReadRequest) (*types.ReadResult, error) {
	if r.calls != nil {
		*r.calls++
	}
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

func TestConcurrentSameUploadRequestOnlyClaimsOneParser(t *testing.T) {
	o, _ := testOffice(t)
	files := &careerUploadFiles{entered: make(chan struct{}), resume: make(chan struct{})}
	catalog := &careerUploadCatalog{}
	calls := 0
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}, calls: &calls})}
	request := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		head := make(textproto.MIMEHeader)
		head.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
		head.Set("Content-Type", "text/plain")
		part, _ := mw.CreatePart(head)
		_, _ = part.Write([]byte("Resume plaintext"))
		_ = mw.WriteField("requestId", "concurrent-request")
		_ = mw.WriteField("expectedRevision", "0")
		_ = mw.Close()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
		req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		c.Request = req.WithContext(ctx)
		h.Upload(c)
		return rec
	}
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- request() }()
	<-files.entered
	second := request()
	require.Equal(t, 202, second.Code)
	var response UploadResponse
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &response))
	require.Equal(t, "processing", response.Source.Status)
	close(files.resume)
	first := <-firstDone
	require.Equal(t, 201, first.Code)
	require.Equal(t, 1, files.saves)
	require.Equal(t, 1, calls)
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
