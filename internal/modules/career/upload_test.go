package career

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type careerUploadFiles struct {
	interfaces.FileService
	stored, deleted string
	temporary       bool
	saves           int
	entered, resume chan struct{}
	data            []byte
	deleteErr       error
	db              *gorm.DB
	register        func(name string, data []byte, saveNumber int) (string, error)
}

func (f *careerUploadFiles) SaveBytes(_ context.Context, data []byte, _ uint64, name string, temporary bool) (string, error) {
	f.stored, f.temporary = name, temporary
	f.data = append([]byte(nil), data...)
	f.saves++
	if f.entered != nil {
		close(f.entered)
		<-f.resume
	}
	if f.register != nil {
		return f.register(name, data, f.saves)
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

func TestUploadExactRetryWithoutExpectedRevisionUsesStoredClaimRevision(t *testing.T) {
	o, _ := testOffice(t)
	files := &careerUploadFiles{}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, &careerUploadCatalog{}, careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}})}
	request := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		head := make(textproto.MIMEHeader)
		head.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
		head.Set("Content-Type", "text/plain")
		part, _ := mw.CreatePart(head)
		_, _ = part.Write([]byte("Resume plaintext"))
		_ = mw.WriteField("requestId", "implicit-replay")
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
	first := request()
	require.Equal(t, 201, first.Code)
	var original UploadResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &original))
	require.Equal(t, uint64(1), original.Receipt.Revision)
	replay := request()
	require.Equal(t, 200, replay.Code)
	var repeated UploadResponse
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &repeated))
	require.Equal(t, original.Source.ID, repeated.Source.ID)
	require.Equal(t, *original.Receipt, *repeated.Receipt)
	require.Equal(t, 1, files.saves)
}

func TestFailedUploadRetainsReferenceUntilReleaseAndDeleteSucceed(t *testing.T) {
	o, _ := testOffice(t)
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{releaseErr: errors.New("catalog unavailable")}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, catalog, careerUploadReader{err: errors.New("parser unavailable")})}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	head := make(textproto.MIMEHeader)
	head.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
	head.Set("Content-Type", "text/plain")
	part, _ := mw.CreatePart(head)
	_, _ = part.Write([]byte("Resume plaintext"))
	_ = mw.WriteField("requestId", "cleanup-retry")
	_ = mw.Close()
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req.WithContext(ctx)
	h.Upload(c)
	require.Equal(t, 200, rec.Code)
	var response UploadResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "cleanup_pending_parse_failed", response.Source.ErrorCategory)
	row, err := o.privateSource(WithScope(ctx, Scope{UserID: "u1", TenantID: 1}), response.Source.ID)
	require.NoError(t, err)
	require.Equal(t, "private://"+files.stored, row.ResourceRef)
	releaseRequest := func() *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		cc, _ := gin.CreateTestContext(r)
		cc.Request = httptest.NewRequest("GET", "/api/v1/career/sources", nil).WithContext(ctx)
		h.Sources(cc)
		return r
	}
	require.Equal(t, 200, releaseRequest().Code)
	var nextBody bytes.Buffer
	nextWriter := multipart.NewWriter(&nextBody)
	nextHeader := make(textproto.MIMEHeader)
	nextHeader.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
	nextHeader.Set("Content-Type", "text/plain")
	nextPart, _ := nextWriter.CreatePart(nextHeader)
	_, _ = nextPart.Write([]byte("Different resume content"))
	_ = nextWriter.WriteField("requestId", "unrelated-upload")
	_ = nextWriter.Close()
	next := httptest.NewRecorder()
	nextContext, _ := gin.CreateTestContext(next)
	nextRequest := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &nextBody)
	nextRequest.Header.Set("Content-Type", nextWriter.FormDataContentType())
	nextContext.Request = nextRequest.WithContext(ctx)
	h.Upload(nextContext)
	require.Equal(t, 200, next.Code, next.Body.String())
	require.Equal(t, 2, files.saves, "unrelated upload must progress while old cleanup is pending")
	catalog.releaseErr = nil
	files.deleteErr = errors.New("disk busy")
	require.Equal(t, 200, releaseRequest().Code)
	row, err = o.privateSource(WithScope(ctx, Scope{UserID: "u1", TenantID: 1}), response.Source.ID)
	require.NoError(t, err)
	require.NotEmpty(t, row.ResourceRef)
	files.deleteErr = nil
	require.Equal(t, 200, releaseRequest().Code)
	row, err = o.privateSource(WithScope(ctx, Scope{UserID: "u1", TenantID: 1}), response.Source.ID)
	require.NoError(t, err)
	require.Empty(t, row.ResourceRef)
	require.Equal(t, "parse_failed", row.ErrorCategory)
}

func TestTakeoverParsesExistingResourceWithoutSavingAnotherBlob(t *testing.T) {
	o, ctx := testOffice(t)
	data := []byte("Resume plaintext")
	digest := sha256.Sum256(data)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: int64(len(data)), Digest: hex.EncodeToString(digest[:]), RequestID: "reuse-existing", IntentHash: "intent-existing"}
	first, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	files := &careerUploadFiles{}
	catalog := &careerUploadCatalog{}
	reader := careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}}
	adapter := NewUploadAdapter(files, catalog, reader)
	original, err := adapter.StoreAndParseWithID(ctx, 1, first.ID, "resume.txt", "text/plain", data, func(result UploadResult) error {
		return o.PersistUploadResource(ctx, result.SourceID, first.ClaimToken, result.Upload.ResourceRef)
	})
	require.NoError(t, err)
	require.Equal(t, 1, files.saves)
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", first.ID).Update("lease_until", time.Now().Add(-time.Second)).Error)
	takeover, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	require.Equal(t, original.Upload.ResourceRef, takeover.ResourceRef)
	resumed, err := adapter.ResumeAndParse(ctx, 1, takeover.ID, "resume.txt", "text/plain", data, takeover.ResourceRef)
	require.NoError(t, err)
	require.Equal(t, original.Upload.ResourceRef, resumed.Upload.ResourceRef)
	require.Equal(t, 1, files.saves)
}
func (f *careerUploadFiles) DeleteFile(_ context.Context, ref string) error {
	f.deleted = ref
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if f.db != nil {
		handle, _ := types.ParseResourcePath(ref)
		return f.db.Model(&types.StoredResource{}).Where("handle=?", handle).Update("state", types.ResourceStateDeleted).Error
	}
	return nil
}
func (f *careerUploadFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.data)), nil
}

type careerUploadCatalog struct {
	interfaces.ResourceCatalog
	bound, released string
	releaseErr      error
	db              *gorm.DB
}

func (c *careerUploadCatalog) Bind(ctx context.Context, reference, ownerType, ownerID, relation string) error {
	c.bound = reference
	if c.db != nil {
		handle, _ := types.ParseResourcePath(reference)
		var resource types.StoredResource
		if err := c.db.WithContext(ctx).Where("handle=?", handle).First(&resource).Error; err != nil {
			return err
		}
		return c.db.WithContext(ctx).Create(&types.ResourceBinding{ResourceID: resource.ID, TenantID: resource.TenantID, OwnerType: ownerType, OwnerID: ownerID, Relation: relation}).Error
	}
	return nil
}
func (c *careerUploadCatalog) Release(_ context.Context, reference, _, _ string) (int64, error) {
	c.released = reference
	return 0, c.releaseErr
}

func catalogingCareerFiles(t *testing.T, db *gorm.DB) *careerUploadFiles {
	t.Helper()
	return &careerUploadFiles{register: func(name string, data []byte, saveNumber int) (string, error) {
		digest := sha256.Sum256(data)
		handle := fmt.Sprintf("%022d", saveNumber)
		resource := &types.StoredResource{Handle: handle, TenantID: 1, Provider: "test", PhysicalPath: "test://" + name, LocationHash: fmt.Sprintf("location-%d", saveNumber), OriginalName: name, Size: int64(len(data)), ContentHash: hex.EncodeToString(digest[:])}
		if err := db.Create(resource).Error; err != nil {
			return "", err
		}
		return types.BuildResourcePath(handle), nil
	}}
}

func TestUploadRetryAdoptsCatalogedRefAfterClaimLoss(t *testing.T) {
	o, ctx := testOffice(t)
	require.NoError(t, o.db.AutoMigrate(&types.StoredResource{}, &types.ResourceBinding{}))
	files := catalogingCareerFiles(t, o.db)
	catalog := &careerUploadCatalog{db: o.db, releaseErr: errors.New("release unavailable")}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}})}
	files.register = func(name string, data []byte, saveNumber int) (string, error) {
		digest := sha256.Sum256(data)
		handle := fmt.Sprintf("%022d", saveNumber)
		resource := &types.StoredResource{Handle: handle, TenantID: 1, Provider: "test", PhysicalPath: "test://" + name, LocationHash: fmt.Sprintf("location-%d", saveNumber), OriginalName: name, Size: int64(len(data)), ContentHash: hex.EncodeToString(digest[:])}
		if err := o.db.Create(resource).Error; err != nil {
			return "", err
		}
		var row sourceRevision
		if err := o.db.Where("tenant_id=? AND user_id=? AND request_id=?", 1, "u1", "recover-request").First(&row).Error; err != nil {
			return "", err
		}
		if err := o.db.Model(&sourceRevision{}).Where("id=?", row.ID).Update("lease_until", time.Now().Add(-time.Second)).Error; err != nil {
			return "", err
		}
		if _, _, err := o.ClaimUpload(ctx, SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: int64(len(data)), Digest: hex.EncodeToString(digest[:]), RequestID: "recover-request", IntentHash: row.IntentHash}); err != nil {
			return "", err
		}
		return types.BuildResourcePath(handle), nil
	}
	request := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		head := make(textproto.MIMEHeader)
		head.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
		head.Set("Content-Type", "text/plain")
		part, _ := mw.CreatePart(head)
		_, _ = part.Write([]byte("Resume plaintext"))
		_ = mw.WriteField("requestId", "recover-request")
		_ = mw.Close()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		base := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
		base = context.WithValue(base, types.TenantIDContextKey, uint64(1))
		req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		c.Request = req.WithContext(base)
		h.Upload(c)
		return rec
	}
	first := request()
	require.Equal(t, 202, first.Code, first.Body.String())
	var response UploadResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &response))
	require.Empty(t, files.deleted)
	row, err := o.privateSource(ctx, response.Source.ID)
	require.NoError(t, err)
	require.Empty(t, row.ResourceRef)
	var candidates []types.StoredResource
	require.NoError(t, o.db.Where("tenant_id=? AND original_name=? AND state=?", 1, "career_source_"+row.ID+".txt", types.ResourceStateActive).Find(&candidates).Error)
	require.Len(t, candidates, 1)
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", row.ID).Update("lease_until", time.Now().Add(-time.Second)).Error)
	second := request()
	require.Equal(t, 201, second.Code, second.Body.String())
	require.Equal(t, 1, files.saves)
	row, err = o.privateSource(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, types.BuildResourcePath(candidates[0].Handle), row.ResourceRef)
}

func TestUploadUnknownDBOutcomeKeepsCatalogedRef(t *testing.T) {
	o, ctx := testOffice(t)
	files := catalogingCareerFiles(t, o.db)
	catalog := &careerUploadCatalog{db: o.db}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, catalog, careerUploadReader{result: &types.ReadResult{MarkdownContent: "Education: Example University"}})}
	active, denyRead := false, false
	require.NoError(t, o.db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("career_test_lost_update_response", func(tx *gorm.DB) {
		if active && tx.Statement.Table == "career_source_revisions" {
			denyRead = true
			tx.AddError(errors.New("database response lost"))
		}
	}))
	require.NoError(t, o.db.Callback().Query().Before("gorm:query").Register("career_test_failed_followup_read", func(tx *gorm.DB) {
		if denyRead && tx.Statement.Table == "career_source_revisions" {
			tx.AddError(errors.New("database read unavailable"))
		}
	}))
	files.register = func(name string, data []byte, saveNumber int) (string, error) {
		digest := sha256.Sum256(data)
		handle := fmt.Sprintf("%022d", saveNumber)
		resource := &types.StoredResource{Handle: handle, TenantID: 1, Provider: "test", PhysicalPath: "test://" + name, LocationHash: fmt.Sprintf("location-%d", saveNumber), OriginalName: name, Size: int64(len(data)), ContentHash: hex.EncodeToString(digest[:])}
		if err := o.db.Create(resource).Error; err != nil {
			return "", err
		}
		active = true
		return types.BuildResourcePath(handle), nil
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	head := make(textproto.MIMEHeader)
	head.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
	head.Set("Content-Type", "text/plain")
	part, _ := mw.CreatePart(head)
	_, _ = part.Write([]byte("Resume plaintext"))
	_ = mw.WriteField("requestId", "unknown-db-outcome")
	_ = mw.Close()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	base := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(1))
	req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req.WithContext(base)
	h.Upload(c)
	require.Equal(t, 504, rec.Code, rec.Body.String())
	active, denyRead = false, false
	var source sourceRevision
	require.NoError(t, o.db.WithContext(ctx).Where("request_id=?", "unknown-db-outcome").First(&source).Error)
	require.NotEmpty(t, source.ResourceRef, "the resource update committed despite its lost response")
	require.Empty(t, files.deleted)
	var registered types.StoredResource
	require.NoError(t, o.db.Where("handle=? AND state=?", strings.TrimPrefix(source.ResourceRef, types.ResourceScheme), types.ResourceStateActive).First(&registered).Error)
}

func TestCatalogRecoveryUsesOnlyExactScopedSourceResource(t *testing.T) {
	o, ctx := testOffice(t)
	data := []byte("Resume plaintext")
	digest := sha256.Sum256(data)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: int64(len(data)), Digest: hex.EncodeToString(digest[:]), RequestID: "isolation", IntentHash: "isolation-intent"}
	claim, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	name := "career_source_" + claim.ID + ".txt"
	insert := func(handle string, tenant uint64, originalName, hash string, size int64) {
		require.NoError(t, o.db.Create(&types.StoredResource{Handle: handle, TenantID: tenant, Provider: "test", PhysicalPath: "test://" + handle, LocationHash: handle, OriginalName: originalName, ContentHash: hash, Size: size}).Error)
	}
	insert("0000000000000000000001", 2, name, u.Digest, u.Size)
	insert("0000000000000000000002", 1, name+"-forged", u.Digest, u.Size)
	insert("0000000000000000000003", 1, name, "wrong-digest", u.Size)
	insert("0000000000000000000004", 1, name, u.Digest, u.Size+1)
	insert("0000000000000000000005", 1, name, u.Digest, u.Size)
	h := &Handler{office: o, upload: NewUploadAdapter(&careerUploadFiles{}, &careerUploadCatalog{db: o.db}, careerUploadReader{})}
	ref, err := h.recoverCatalogRef(ctx, claim.ID, claim.ClaimToken, u.RequestID)
	require.NoError(t, err)
	require.Equal(t, "resource://0000000000000000000005", ref)
	row, err := o.privateSource(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, ref, row.ResourceRef)
}

func TestCatalogCleanupKeepsReadySourceAndRetriesFailedDeletion(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 16, Digest: "same-digest", RequestID: "ready-cleanup", IntentHash: "ready-intent"}
	claim, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	name := "career_source_" + claim.ID + ".txt"
	for _, handle := range []string{"0000000000000000000011", "0000000000000000000012", "0000000000000000000013"} {
		require.NoError(t, o.db.Create(&types.StoredResource{Handle: handle, TenantID: 1, Provider: "test", PhysicalPath: "test://" + handle, LocationHash: handle, OriginalName: name, ContentHash: u.Digest, Size: u.Size}).Error)
	}
	active := "resource://0000000000000000000011"
	require.NoError(t, o.PersistUploadResource(ctx, claim.ID, claim.ClaimToken, active))
	_, err = o.FinishSourceClaim(ctx, claim.ID, claim.ClaimToken, "resume text", nil, nil, nil)
	require.NoError(t, err)
	var foreign types.StoredResource
	require.NoError(t, o.db.Where("handle=?", "0000000000000000000013").First(&foreign).Error)
	require.NoError(t, o.db.Create(&types.ResourceBinding{ResourceID: foreign.ID, TenantID: 1, OwnerType: "other", OwnerID: "foreign"}).Error)
	files := &careerUploadFiles{db: o.db, deleteErr: errors.New("disk busy")}
	h := &Handler{office: o, upload: NewUploadAdapter(files, &careerUploadCatalog{db: o.db}, careerUploadReader{})}
	require.ErrorContains(t, h.cleanupCatalogCandidates(ctx, claim.ID), "disk busy")
	require.Equal(t, "resource://0000000000000000000012", files.deleted)
	files.deleteErr = nil
	require.NoError(t, h.cleanupCatalogCandidates(ctx, claim.ID))
	require.Equal(t, "resource://0000000000000000000012", files.deleted, "active and foreign-owned refs must survive")
	row, err := o.privateSource(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, active, row.ResourceRef)
}

func TestStaleSourceWithUnrecordedCatalogRefShowsCleanupPending(t *testing.T) {
	o, ctx := testOffice(t)
	u := SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 16, Digest: "registered-digest", RequestID: "unrecorded-stale", IntentHash: "stale-intent"}
	claim, _, err := o.ClaimUpload(ctx, u)
	require.NoError(t, err)
	ref := "resource://0000000000000000000021"
	require.NoError(t, o.db.Create(&types.StoredResource{Handle: "0000000000000000000021", TenantID: 1, Provider: "test", PhysicalPath: "test://stale", LocationHash: "stale", OriginalName: "career_source_" + claim.ID + ".txt", ContentHash: u.Digest, Size: u.Size}).Error)
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("id=?", claim.ID).Update("lease_until", time.Now().Add(-time.Second)).Error)
	catalog := &careerUploadCatalog{releaseErr: errors.New("release unavailable")}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(&careerUploadFiles{db: o.db}, catalog, careerUploadReader{})}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	base := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(1))
	c.Request = httptest.NewRequest("GET", "/api/v1/career/sources", nil).WithContext(base)
	h.Sources(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "cleanup_pending_interrupted")
	row, err := o.privateSource(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, "cleanup_pending_interrupted", row.ErrorCategory)
	require.Equal(t, ref, row.ResourceRef)
	catalog.releaseErr = nil
	next := httptest.NewRecorder()
	nextContext, _ := gin.CreateTestContext(next)
	nextContext.Request = httptest.NewRequest("GET", "/api/v1/career/sources", nil).WithContext(base)
	h.Sources(nextContext)
	require.Equal(t, 200, next.Code, next.Body.String())
	row, err = o.privateSource(ctx, claim.ID)
	require.NoError(t, err)
	require.Empty(t, row.ResourceRef)
	require.Equal(t, "interrupted", row.ErrorCategory)
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
	require.Empty(t, catalog.released, "the handler owns cleanup after it durably records a failed source")
	require.Empty(t, files.deleted)
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
