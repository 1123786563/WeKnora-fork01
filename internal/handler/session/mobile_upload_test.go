package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// W25 mobile session attachment upload: handler-level validation and
// permission wiring exercised through the real gin route, plus a service-layer
// harness (in-memory repository) covering expiry cleanup and the Agent read
// gate. No SQLite dependency is required.
// -----------------------------------------------------------------------------

const (
	mobileUploadTenant  = uint64(42)
	mobileUploadOwner   = "owner-1"
	mobileUploadSession = "sess-mobile"
)

// mobileUploadSessionStub models the production scoping: uploads and reads are
// resolved within the caller's owner scope (GetOwnedSession / user-scoped
// GetSession), so a different owner is rejected.
type mobileUploadSessionStub struct {
	interfaces.SessionService
	tenantID uint64
	ownerID  string
}

func (s *mobileUploadSessionStub) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	if types.MustTenantIDFromContext(ctx) != s.tenantID || types.SessionOwnerIDFromContext(ctx) != s.ownerID {
		return nil, apperrors.ErrSessionNotFound
	}
	if id != mobileUploadSession {
		return nil, apperrors.ErrSessionNotFound
	}
	return &types.Session{ID: id, TenantID: s.tenantID, UserID: s.ownerID}, nil
}

func (s *mobileUploadSessionStub) GetSession(ctx context.Context, id string) (*types.Session, error) {
	return s.GetOwnedSession(ctx, id)
}

type mobileUploadCreateCall struct {
	tenantID  uint64
	sessionID string
	fileName  string
	mimeType  string
	fileSize  int64
	data      []byte
}

type mobileUploadDocumentsStub struct {
	interfaces.TemporaryDocumentService
	created []mobileUploadCreateCall
}

func (s *mobileUploadDocumentsStub) Create(
	_ context.Context, tenantID uint64, sessionID, fileName, mimeType string,
	fileSize int64, reader io.Reader, _ types.TemporaryDocumentCreateOptions,
) (*types.TemporaryDocument, error) {
	data, _ := io.ReadAll(reader)
	s.created = append(s.created, mobileUploadCreateCall{
		tenantID: tenantID, sessionID: sessionID, fileName: fileName,
		mimeType: mimeType, fileSize: fileSize, data: data,
	})
	return &types.TemporaryDocument{ID: "doc-1", TenantID: tenantID, SessionID: sessionID, FileName: fileName}, nil
}

func (s *mobileUploadDocumentsStub) Get(_ context.Context, _ uint64, _, _ string) (*types.TemporaryDocument, error) {
	return &types.TemporaryDocument{ID: "doc-1", FileName: "a.pdf", Status: types.TemporaryDocumentStatusReady}, nil
}

func newMobileUploadRouter(documents interfaces.TemporaryDocumentService, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{
		sessionService:     &mobileUploadSessionStub{tenantID: mobileUploadTenant, ownerID: mobileUploadOwner},
		temporaryDocuments: documents,
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, mobileUploadTenant)
		if userID != "" {
			ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), mobileUploadTenant)
		c.Next()
	})
	r.POST("/sessions/:session_id/attachments", h.UploadTemporaryDocument)
	r.GET("/sessions/:id/attachments/:attachment_id", h.GetTemporaryDocument)
	return r
}

func mobileUploadRequest(t *testing.T, fileName, contentType string, content []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func postMobileAttachment(t *testing.T, documents interfaces.TemporaryDocumentService, userID, fileName, contentType string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentTypeHeader := mobileUploadRequest(t, fileName, contentType, content, fields)
	router := newMobileUploadRouter(documents, userID)
	req := httptest.NewRequest(http.MethodPost, "/sessions/"+mobileUploadSession+"/attachments", body)
	req.Header.Set("Content-Type", contentTypeHeader)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

var pdfBytes = append([]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"), bytes.Repeat([]byte("pdf body "), 8)...)

var pngBytes = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0x0D, 'I', 'H', 'D', 'R'}, bytes.Repeat([]byte("png data "), 8)...)

func TestMobileUploadStoresRealContentAndPreservesAttachmentID(t *testing.T) {
	documents := &mobileUploadDocumentsStub{}
	w := postMobileAttachment(t, documents, mobileUploadOwner, "report.pdf", "application/pdf", pdfBytes, nil)

	require.Equal(t, http.StatusAccepted, w.Code, "body=%s", w.Body.String())
	require.Len(t, documents.created, 1)
	call := documents.created[0]
	assert.Equal(t, mobileUploadTenant, call.tenantID)
	assert.Equal(t, mobileUploadSession, call.sessionID)
	assert.Equal(t, "report.pdf", call.fileName)
	assert.Equal(t, int64(len(pdfBytes)), call.fileSize, "handler must pass the actual byte count, not the declared size")
	assert.Equal(t, pdfBytes, call.data)
	assert.Contains(t, w.Body.String(), "doc-1", "response must keep the original attachment ID contract")
}

func TestMobileUploadRejectsAnotherOwner(t *testing.T) {
	documents := &mobileUploadDocumentsStub{}
	w := postMobileAttachment(t, documents, "owner-2", "report.pdf", "application/pdf", pdfBytes, nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, documents.created, "another owner must not reach document creation")
}

func TestMobileUploadReadRejectsAnotherOwner(t *testing.T) {
	router := newMobileUploadRouter(&mobileUploadDocumentsStub{}, "owner-2")
	req := httptest.NewRequest(http.MethodGet, "/sessions/"+mobileUploadSession+"/attachments/doc-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMobileUploadRejectsZeroByteFile(t *testing.T) {
	documents := &mobileUploadDocumentsStub{}
	w := postMobileAttachment(t, documents, mobileUploadOwner, "empty.pdf", "application/pdf", nil, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "empty")
	assert.Empty(t, documents.created)
}

func TestMobileUploadRejectsSpoofedMime(t *testing.T) {
	documents := &mobileUploadDocumentsStub{}
	// PNG bytes presented as application/pdf.
	w := postMobileAttachment(t, documents, mobileUploadOwner, "report.pdf", "application/pdf", pngBytes, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "does not match")
	assert.Empty(t, documents.created)
}

func TestMobileUploadAllowsFormatsGoCannotSniff(t *testing.T) {
	// TIFF bytes are not in Go's DetectContentType table: they must not be
	// rejected on MIME-family grounds (web upload compatibility).
	documents := &mobileUploadDocumentsStub{}
	tiffBytes := append([]byte("II*\x00\x08\x00\x00\x00"), bytes.Repeat([]byte("tiff "), 10)...)
	w := postMobileAttachment(t, documents, mobileUploadOwner, "photo.tiff", "image/tiff", tiffBytes, nil)

	require.Equal(t, http.StatusAccepted, w.Code, "body=%s", w.Body.String())
	require.Len(t, documents.created, 1)
	assert.Equal(t, "image/tiff", documents.created[0].mimeType)
}

func TestMobileUploadRejectsExecutableContent(t *testing.T) {
	documents := &mobileUploadDocumentsStub{}
	execBytes := append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"), bytes.Repeat([]byte("A"), 64)...)
	w := postMobileAttachment(t, documents, mobileUploadOwner, "report.pdf", "application/pdf", execBytes, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, documents.created)
}

func TestMobileUploadRejectsOversizedBody(t *testing.T) {
	t.Setenv("MAX_FILE_SIZE_MB", "1")
	documents := &mobileUploadDocumentsStub{}
	oversize := bytes.Repeat([]byte("a"), int(utils.GetMaxFileSizeMB())*1024*1024+16)
	w := postMobileAttachment(t, documents, mobileUploadOwner, "big.pdf", "application/pdf", oversize, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "size")
	assert.Empty(t, documents.created)
}

func TestMobileUploadVerifiesOptionalDigest(t *testing.T) {
	sum := sha256.Sum256(pdfBytes)
	digest := hex.EncodeToString(sum[:])

	documents := &mobileUploadDocumentsStub{}
	w := postMobileAttachment(t, documents, mobileUploadOwner, "report.pdf", "application/pdf", pdfBytes, map[string]string{"sha256": digest})
	require.Equal(t, http.StatusAccepted, w.Code, "body=%s", w.Body.String())

	documents = &mobileUploadDocumentsStub{}
	w = postMobileAttachment(t, documents, mobileUploadOwner, "report.pdf", "application/pdf", pdfBytes, map[string]string{"sha256": strings.Repeat("ab", 32)})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, strings.ToLower(w.Body.String()), "digest")
	assert.Empty(t, documents.created)
}

// -----------------------------------------------------------------------------
// Service-layer harness: expiry cleanup and the Agent read gate, using an
// in-memory repository so no SQLite migration state is involved.
// -----------------------------------------------------------------------------

type inMemoryTemporaryDocRepo struct {
	mu    sync.Mutex
	rows  map[string]*types.TemporaryDocument
	clock time.Time
}

func newInMemoryTemporaryDocRepo(documents ...*types.TemporaryDocument) *inMemoryTemporaryDocRepo {
	repo := &inMemoryTemporaryDocRepo{rows: map[string]*types.TemporaryDocument{}, clock: time.Now()}
	for _, document := range documents {
		clone := *document
		repo.rows[clone.ID] = &clone
	}
	return repo
}

func (r *inMemoryTemporaryDocRepo) Create(_ context.Context, document *types.TemporaryDocument) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *document
	r.rows[clone.ID] = &clone
	return nil
}

func (r *inMemoryTemporaryDocRepo) GetByID(_ context.Context, tenantID uint64, documentID string) (*types.TemporaryDocument, error) {
	return r.GetScoped(context.Background(), tenantID, "", documentID)
}

func (r *inMemoryTemporaryDocRepo) GetScoped(_ context.Context, tenantID uint64, sessionID, documentID string) (*types.TemporaryDocument, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	document, ok := r.rows[documentID]
	if !ok || document.TenantID != tenantID {
		return nil, nil
	}
	if sessionID != "" && document.SessionID != sessionID {
		return nil, nil
	}
	clone := *document
	return &clone, nil
}

func (r *inMemoryTemporaryDocRepo) ListScoped(_ context.Context, tenantID uint64, sessionID string) ([]*types.TemporaryDocument, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.TemporaryDocument
	for _, document := range r.rows {
		if document.TenantID == tenantID && document.SessionID == sessionID {
			clone := *document
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *inMemoryTemporaryDocRepo) MarkProcessing(_ context.Context, tenantID uint64, documentID string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d := r.rows[documentID]; d != nil && d.TenantID == tenantID {
		d.Status = types.TemporaryDocumentStatusProcessing
	}
	return nil
}

func (r *inMemoryTemporaryDocRepo) MarkReady(_ context.Context, tenantID uint64, documentID, content string, _, _, _ types.JSON, tokenCount, _ int, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d := r.rows[documentID]; d != nil && d.TenantID == tenantID {
		d.Status = types.TemporaryDocumentStatusReady
		d.Content = content
		d.TokenCount = tokenCount
	}
	return nil
}

func (r *inMemoryTemporaryDocRepo) MarkFailed(_ context.Context, tenantID uint64, documentID, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d := r.rows[documentID]; d != nil && d.TenantID == tenantID {
		d.Status = types.TemporaryDocumentStatusFailed
		d.ErrorMessage = message
	}
	return nil
}

func (r *inMemoryTemporaryDocRepo) DeleteScoped(_ context.Context, tenantID uint64, sessionID, documentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	document, ok := r.rows[documentID]
	if !ok || document.TenantID != tenantID || document.SessionID != sessionID {
		return nil
	}
	delete(r.rows, documentID)
	return nil
}

func (r *inMemoryTemporaryDocRepo) ListExpired(_ context.Context, before time.Time, limit int) ([]*types.TemporaryDocument, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*types.TemporaryDocument
	for _, document := range r.rows {
		if !document.ExpiresAt.After(before) {
			clone := *document
			out = append(out, &clone)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

type mobileUploadFileService struct {
	interfaces.FileService
	deleted map[string]int
}

func (f *mobileUploadFileService) DeleteFile(_ context.Context, url string) error {
	f.deleted[url]++
	return nil
}

func (f *mobileUploadFileService) GetFile(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not found")
}

func (f *mobileUploadFileService) SaveBytes(_ context.Context, _ []byte, _ uint64, _ string, _ bool) (string, error) {
	return "", nil
}

type mobileUploadEnqueuer struct{}

func (mobileUploadEnqueuer) Enqueue(_ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{}, nil
}

func newMobileUploadService(repo interfaces.TemporaryDocumentRepository, files interfaces.FileService) interfaces.TemporaryDocumentService {
	return service.NewTemporaryDocumentService(repo, files, nil, nil, nil, nil, nil, mobileUploadEnqueuer{})
}

func TestTemporaryDocumentCleanupExpiredDoesNotDeleteReferenced(t *testing.T) {
	expired := &types.TemporaryDocument{
		ID: "doc-expired", TenantID: mobileUploadTenant, SessionID: "s1",
		ResourceRef: "local://42/chat_attachment_expired.pdf", FileType: ".pdf",
		ImageRefs: types.JSON(`[{"url":"local://42/img_expired.png","mime_type":"image/png"}]`),
		ExpiresAt: time.Now().Add(-time.Hour), Status: types.TemporaryDocumentStatusReady,
	}
	// Still referenced by an active conversation: not expired, must survive.
	referenced := &types.TemporaryDocument{
		ID: "doc-referenced", TenantID: mobileUploadTenant, SessionID: "s1",
		ResourceRef: "local://42/chat_attachment_referenced.pdf", FileType: ".pdf",
		ImageRefs: types.JSON(`[{"url":"local://42/img_referenced.png","mime_type":"image/png"}]`),
		ExpiresAt: time.Now().Add(time.Hour), Status: types.TemporaryDocumentStatusReady,
	}
	repo := newInMemoryTemporaryDocRepo(expired, referenced)
	files := &mobileUploadFileService{deleted: map[string]int{}}
	svc := newMobileUploadService(repo, files)

	require.NoError(t, svc.CleanupExpired(context.Background()))

	_, err := repo.GetScoped(context.Background(), mobileUploadTenant, "s1", "doc-expired")
	require.NoError(t, err)
	kept, err := repo.GetScoped(context.Background(), mobileUploadTenant, "s1", "doc-referenced")
	require.NoError(t, err)
	require.NotNil(t, kept, "a not-yet-expired referenced document must not be deleted")

	assert.Equal(t, 1, files.deleted["local://42/chat_attachment_expired.pdf"])
	assert.Equal(t, 1, files.deleted["local://42/img_expired.png"])
	assert.Equal(t, 0, files.deleted["local://42/chat_attachment_referenced.pdf"], "referenced source file must not be deleted")
	assert.Equal(t, 0, files.deleted["local://42/img_referenced.png"], "referenced extracted image must not be deleted")
}

func TestTemporaryDocumentResolveForPromptGatesOnScanStatus(t *testing.T) {
	processing := &types.TemporaryDocument{
		ID: "doc-processing", TenantID: mobileUploadTenant, SessionID: "s1",
		Status: types.TemporaryDocumentStatusProcessing, ExpiresAt: time.Now().Add(time.Hour),
	}
	ready := &types.TemporaryDocument{
		ID: "doc-ready", TenantID: mobileUploadTenant, SessionID: "s1",
		Status: types.TemporaryDocumentStatusReady, Content: "hello attachment",
		TokenCount: 2, ExpiresAt: time.Now().Add(time.Hour),
	}
	svc := newMobileUploadService(newInMemoryTemporaryDocRepo(processing, ready), &mobileUploadFileService{deleted: map[string]int{}})

	_, err := svc.ResolveForPrompt(context.Background(), mobileUploadTenant, "s1", []string{"doc-processing"}, "what is this")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still being processed", "a scan that has not finished must not be readable by the Agent")

	result, err := svc.ResolveForPrompt(context.Background(), mobileUploadTenant, "s1", []string{"doc-ready"}, "what is this")
	require.NoError(t, err)
	require.Len(t, result.Attachments, 1)
	assert.Equal(t, "doc-ready", result.Attachments[0].ID, "the Agent reference must keep the original attachment ID")
	assert.Equal(t, "hello attachment", result.Attachments[0].Content)

	_, err = svc.ResolveForPrompt(context.Background(), 7, "s1", []string{"doc-ready"}, "what is this")
	require.Error(t, err, "another tenant must not resolve the attachment")
}

// Guard: the JSON image refs used by the cleanup test parse as expected.
func TestTemporaryDocumentImageRefsParse(t *testing.T) {
	document := &types.TemporaryDocument{ImageRefs: types.JSON(`[{"url":"u1"},{"url":"u2"}]`)}
	var images []types.TemporaryDocumentImage
	require.NoError(t, json.Unmarshal(document.ImageRefs, &images))
	assert.Len(t, images, 2)
}
