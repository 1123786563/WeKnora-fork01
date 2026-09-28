package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	careerrepo "github.com/Tencent/WeKnora/internal/modules/career/repository"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerArtifactDownloadCleansStageWhenCatalogCommitFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	before := careerArtifactTempFiles(t)
	db, store := careerArtifactDB(t)
	body := []byte("stage removed after commit failure")
	hash := sha256.Sum256(body)
	digest := hex.EncodeToString(hash[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'commit-v','r','s',?,'local://tenant/12/o','text/plain',?,'ready')`, digest, len(body)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('s',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BindVersion(context.Background(), careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}, "resume", "commit-v"); err != nil {
		t.Fatal(err)
	}
	h := NewCareerArtifactHandler(failCommitCatalog{store}, careerArtifactTestTenant{}, &careerArtifactTestFiles{bytes: body}, nil)
	h.stageBudget = newCareerArtifactStageBudget(int64(len(body)))
	secret := []byte(strings.Repeat("c", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	h.now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
	g := workbench.VersionArtifactGrant{TenantID: 12, OwnerID: "owner-a", ResourceID: "resume", VersionID: "commit-v", Digest: digest, ExpiresAt: h.now().Add(time.Minute).UnixNano()}
	sig, err := workbench.SignVersionArtifactGrant(secret, g)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"tenant_id": {"12"}, "owner_id": {"owner-a"}, "resource_id": {"resume"}, "version_id": {"commit-v"}, "digest": {digest}, "expires_at": {strconv.FormatInt(g.ExpiresAt, 10)}, "signature": {sig}}
	r := gin.New()
	r.GET("/download", h.Download)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download?"+values.Encode(), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if h.stageBudget.used != 0 {
		t.Fatalf("staging reservation leaked: %d", h.stageBudget.used)
	}
	for name := range careerArtifactTempFiles(t) {
		if _, existed := before[name]; !existed {
			t.Fatalf("staged temp file leaked: %s", name)
		}
	}
}

func careerArtifactTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]struct{}{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "weknora-career-artifact-") {
			out[entry.Name()] = struct{}{}
		}
	}
	return out
}

type failCommitCatalog struct{ CareerArtifactCatalog }

func (f failCommitCatalog) WithResolved(ctx context.Context, grant careerrepo.ArtifactGrant, use func(careerrepo.ArtifactVersion) error) error {
	if err := f.CareerArtifactCatalog.WithResolved(ctx, grant, use); err != nil {
		return err
	}
	return stderrors.New("simulated commit failure")
}

type careerArtifactTestFiles struct {
	interfaces.FileService
	bytes []byte
	opens int
}

func TestCareerArtifactHTTPDownloadWithProductionTenantServiceAndSingleSQLiteConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, store := careerArtifactDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(`CREATE TABLE tenants (id INTEGER PRIMARY KEY, name TEXT, description TEXT, status TEXT, retriever_engines TEXT, business TEXT, storage_quota INTEGER, storage_used INTEGER, deleted_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tenants (id,name,status) VALUES (12,'workspace','active')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE tenants ADD COLUMN default_storage_backend_id TEXT`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE tenants ADD COLUMN storage_engine_config TEXT`).Error; err != nil {
		t.Fatal(err)
	}
	storageDir := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", storageDir)
	config := `{"default_provider":"local","local":{"path_prefix":"` + strings.ReplaceAll(storageDir, `\`, `\\`) + `"}}`
	if err := db.Exec(`UPDATE tenants SET storage_engine_config=? WHERE id=12`, config).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&types.StoredResource{}, &types.StorageBackend{}); err != nil {
		t.Fatal(err)
	}
	body := []byte("single-connection artifact")
	physical := filepath.Join(storageDir, "resume.txt")
	if err := os.WriteFile(physical, body, 0600); err != nil {
		t.Fatal(err)
	}
	resource := types.StoredResource{ID: "resource-1", Handle: "AbCdEfGhIjKlMnOpQrStUv", TenantID: 12, Provider: "local", PhysicalPath: physical, LocationHash: strings.Repeat("a", 64), Kind: "file", OriginalName: "resume.txt", Size: int64(len(body)), State: types.ResourceStateActive}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'version-a','run','session',?,'resource://AbCdEfGhIjKlMnOpQrStUv','text/plain',?,'ready')`, digest, len(body)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('session',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BindVersion(context.Background(), careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}, "resume", "version-a"); err != nil {
		t.Fatal(err)
	}
	tenants := appservice.NewTenantService(apprepo.NewTenantRepository(db), nil)
	resourceCatalog := appservice.NewResourceCatalog(apprepo.NewResourceRepository(db))
	storage := appservice.NewStorageBackendServiceWithResources(apprepo.NewStorageBackendRepository(db), db, resourceCatalog)
	preparedTenant, err := tenants.GetTenantByID(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	resolved, _, err := storage.ResolveFileService(context.Background(), preparedTenant, "", "", "")
	if err != nil {
		t.Fatalf("resolve production file service: %v tenant=%+v", err, preparedTenant)
	}
	if _, err := resolved.GetFile(context.Background(), "resource://AbCdEfGhIjKlMnOpQrStUv"); err != nil {
		t.Fatalf("resolve production resource object: %v", err)
	}
	h := NewCareerArtifactHandler(store, tenants, &careerArtifactTestFiles{bytes: body}, storage)
	secret := []byte(strings.Repeat("s", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	now := time.Date(2026, 9, 29, 12, 30, 0, 123456789, time.UTC)
	h.now = func() time.Time { return now }
	grant := workbench.VersionArtifactGrant{TenantID: 12, OwnerID: "owner-a", ResourceID: "resume", VersionID: "version-a", Digest: digest, ExpiresAt: now.Add(time.Minute).UnixNano()}
	signature, err := workbench.SignVersionArtifactGrant(secret, grant)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"tenant_id": {"12"}, "owner_id": {"owner-a"}, "resource_id": {"resume"}, "version_id": {"version-a"}, "digest": {digest}, "expires_at": {strconv.FormatInt(grant.ExpiresAt, 10)}, "signature": {signature}}
	r := gin.New()
	r.GET("/download", h.Download)
	result := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/download?"+values.Encode(), nil)
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() { r.ServeHTTP(result, req); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("download deadlocked with one SQLite connection")
	}
	if result.Code != http.StatusOK || !bytesEqual(result.Body.Bytes(), body) {
		t.Fatalf("download status=%d body=%q", result.Code, result.Body.Bytes())
	}
}

type blockingArtifactWriter struct {
	header        http.Header
	firstWrite    chan struct{}
	continueWrite chan struct{}
	mu            sync.Mutex
	body          []byte
	status        int
}

func (w *blockingArtifactWriter) Header() http.Header  { return w.header }
func (w *blockingArtifactWriter) WriteHeader(code int) { w.status = code }
func (w *blockingArtifactWriter) Write(p []byte) (int, error) {
	select {
	case <-w.firstWrite:
	default:
		close(w.firstWrite)
		<-w.continueWrite
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bodyWrite(p)
}
func (w *blockingArtifactWriter) bodyWrite(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}

func TestCareerArtifactHTTPDownloadReleasesSessionLockBeforeSlowResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, store := careerArtifactDB(t)
	body := []byte("slow response snapshot")
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'version-a','run','session',?,'local://tenant/12/object','text/plain',?,'ready')`, digest, len(body)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('session',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BindVersion(context.Background(), careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}, "resume", "version-a"); err != nil {
		t.Fatal(err)
	}
	files := &careerArtifactTestFiles{bytes: body}
	h := NewCareerArtifactHandler(store, careerArtifactTestTenant{}, files, nil)
	secret := []byte(strings.Repeat("s", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	now := time.Now().UTC()
	h.now = func() time.Time { return now }
	grant := workbench.VersionArtifactGrant{TenantID: 12, OwnerID: "owner-a", ResourceID: "resume", VersionID: "version-a", Digest: digest, ExpiresAt: now.Add(time.Minute).UnixNano()}
	sig, err := workbench.SignVersionArtifactGrant(secret, grant)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"tenant_id": {"12"}, "owner_id": {"owner-a"}, "resource_id": {"resume"}, "version_id": {"version-a"}, "digest": {digest}, "expires_at": {strconv.FormatInt(grant.ExpiresAt, 10)}, "signature": {sig}}
	r := gin.New()
	r.GET("/download", h.Download)
	w := &blockingArtifactWriter{header: make(http.Header), firstWrite: make(chan struct{}), continueWrite: make(chan struct{})}
	req := httptest.NewRequest(http.MethodGet, "/download?"+values.Encode(), nil)
	done := make(chan struct{})
	go func() { r.ServeHTTP(w, req); close(done) }()
	select {
	case <-w.firstWrite:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not reach response write")
	}
	updated := make(chan error, 1)
	go func() {
		updated <- db.Exec(`UPDATE sessions SET user_id='owner-b' WHERE tenant_id=12 AND id='session'`).Error
	}()
	select {
	case err := <-updated:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(w.continueWrite)
		t.Fatal("session owner update blocked behind slow response")
	}
	close(w.continueWrite)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("download response did not finish")
	}
	if !bytesEqual(w.body, body) {
		t.Fatalf("staged snapshot body=%q", w.body)
	}
}

func (f *careerArtifactTestFiles) GetFile(context.Context, string) (io.ReadCloser, error) {
	f.opens++
	return io.NopCloser(strings.NewReader(string(f.bytes))), nil
}

type careerArtifactTestTenant struct{ interfaces.TenantService }

func (careerArtifactTestTenant) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

func TestCareerArtifactHTTPIssueDownloadDigestAndRevoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, store := careerArtifactDB(t)
	ctx := context.Background()
	body := []byte("actual immutable artifact bytes")
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'version-a','run','session',?,'local://tenant/12/object','text/plain',?,'ready')`, digest, len(body)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('session',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	scope := careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}
	if _, err := store.BindVersion(ctx, scope, "resume", "version-a"); err != nil {
		t.Fatal(err)
	}
	files := &careerArtifactTestFiles{bytes: body}
	h := NewCareerArtifactHandler(store, careerArtifactTestTenant{}, files, nil)
	secret := []byte(strings.Repeat("s", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	now := time.Date(2026, 9, 29, 12, 30, 0, 123456789, time.UTC)
	h.now = func() time.Time { return now }
	r := gin.New()
	r.POST("/api/v1/career/resources/:resource_id/versions/:version_id/signed-url", func(c *gin.Context) {
		ctx := types.WithCaller(c.Request.Context(), types.Caller{TenantID: 12, UserID: "owner-a"})
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(12))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
		ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "owner-a"})
		c.Request = c.Request.WithContext(ctx)
		h.Issue(c)
	})
	r.GET("/api/v1/career/artifacts/download", h.Download)
	issue := httptest.NewRecorder()
	r.ServeHTTP(issue, httptest.NewRequest(http.MethodPost, "/api/v1/career/resources/resume/versions/version-a/signed-url", nil))
	if issue.Code != http.StatusOK {
		t.Fatalf("issue status=%d body=%s", issue.Code, issue.Body.String())
	}
	var response struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(issue.Body.Bytes(), &response); err != nil || response.Data.URL == "" {
		t.Fatalf("issue response=%s err=%v", issue.Body.String(), err)
	}
	u := strings.TrimPrefix(response.Data.URL, "http://example.com")
	download := httptest.NewRecorder()
	r.ServeHTTP(download, httptest.NewRequest(http.MethodGet, u, nil))
	if download.Code != http.StatusOK || !bytesEqual(download.Body.Bytes(), body) {
		t.Fatalf("download status=%d body=%q", download.Code, download.Body.Bytes())
	}
	gotDigest := sha256.Sum256(download.Body.Bytes())
	if hex.EncodeToString(gotDigest[:]) != digest {
		t.Fatal("download digest differs from immutable version digest")
	}
	if files.opens != 1 {
		t.Fatalf("blob opens=%d, want 1", files.opens)
	}
	for _, mutation := range []string{
		`UPDATE sessions SET user_id='owner-b' WHERE tenant_id=12 AND id='session'`,
		`UPDATE sessions SET user_id='owner-a', deleted_at=CURRENT_TIMESTAMP WHERE tenant_id=12 AND id='session'`,
	} {
		if err := db.Exec(mutation).Error; err != nil {
			t.Fatal(err)
		}
		deniedDownload := httptest.NewRecorder()
		r.ServeHTTP(deniedDownload, httptest.NewRequest(http.MethodGet, u, nil))
		deniedIssue := httptest.NewRecorder()
		r.ServeHTTP(deniedIssue, httptest.NewRequest(http.MethodPost, "/api/v1/career/resources/resume/versions/version-a/signed-url", nil))
		if deniedDownload.Code != http.StatusNotFound || deniedDownload.Body.Len() != 0 || deniedIssue.Code != http.StatusNotFound || deniedIssue.Body.Len() != 0 || files.opens != 1 {
			t.Fatalf("inactive Task accepted grant: download=%d/%q issue=%d/%q blob opens=%d", deniedDownload.Code, deniedDownload.Body.String(), deniedIssue.Code, deniedIssue.Body.String(), files.opens)
		}
	}
	issuedURL, err := url.Parse(response.Data.URL)
	if err != nil {
		t.Fatal(err)
	}
	baseGrant, _, ok := parseCareerArtifactGrant(issuedURL.Query())
	if !ok {
		t.Fatal("issued URL did not contain a complete grant")
	}
	denials := map[string]url.Values{}
	mutations := map[string]func(workbench.VersionArtifactGrant) workbench.VersionArtifactGrant{
		"foreign tenant": func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant { g.TenantID = 99; return g },
		"foreign owner":  func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant { g.OwnerID = "owner-b"; return g },
		"wrong resource": func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant {
			g.ResourceID = "other"
			return g
		},
		"wrong version": func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant { g.VersionID = "other"; return g },
		"wrong digest": func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant {
			g.Digest = strings.Repeat("b", 64)
			return g
		},
		"expired": func(g workbench.VersionArtifactGrant) workbench.VersionArtifactGrant {
			g.ExpiresAt = now.Add(-time.Second).UnixNano()
			return g
		},
	}
	for name, mutate := range mutations {
		changed := mutate(baseGrant)
		sig, err := workbench.SignVersionArtifactGrant(secret, changed)
		if err != nil {
			t.Fatal(err)
		}
		query := url.Values{"tenant_id": {strconv.FormatUint(changed.TenantID, 10)}, "owner_id": {changed.OwnerID}, "resource_id": {changed.ResourceID}, "version_id": {changed.VersionID}, "digest": {changed.Digest}, "expires_at": {strconv.FormatInt(changed.ExpiresAt, 10)}, "signature": {sig}}
		denials[name] = query
	}
	tampered := issuedURL.Query()
	tampered.Set("signature", strings.Repeat("0", 64))
	denials["tampered signature"] = tampered
	for name, query := range denials {
		denied := httptest.NewRecorder()
		r.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/career/artifacts/download?"+query.Encode(), nil))
		if denied.Code != http.StatusNotFound || denied.Body.Len() != 0 || files.opens != 1 {
			t.Errorf("%s denial status=%d body=%q opens=%d", name, denied.Code, denied.Body.String(), files.opens)
		}
	}
	if err := store.Revoke(ctx, scope, "resume", "version-a"); err != nil {
		t.Fatal(err)
	}
	revoked := httptest.NewRecorder()
	r.ServeHTTP(revoked, httptest.NewRequest(http.MethodGet, u, nil))
	if revoked.Code != http.StatusNotFound || files.opens != 1 {
		t.Fatalf("revoked status=%d blob opens=%d", revoked.Code, files.opens)
	}
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/career/artifacts/download?tenant_id=12&owner_id=x&resource_id=missing&version_id=missing&digest="+digest+"&expires_at=9999999999999999999&signature=bad", nil))
	if missing.Code != revoked.Code || missing.Body.String() != revoked.Body.String() || files.opens != 1 {
		t.Fatalf("denials enumerate resources: revoked=%d/%q missing=%d/%q opens=%d", revoked.Code, revoked.Body.String(), missing.Code, missing.Body.String(), files.opens)
	}
	unauth := httptest.NewRecorder()
	r.ServeHTTP(unauth, httptest.NewRequest(http.MethodPost, "/api/v1/career/resources/resume/versions/version-a/signed-url", nil))
	if unauth.Code != http.StatusNotFound || unauth.Body.Len() != 0 {
		t.Fatalf("unauthenticated issue status=%d body=%q", unauth.Code, unauth.Body.String())
	}
}

func TestCareerArtifactDownloadRejectsCorruptBytesBeforeSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, store := careerArtifactDB(t)
	ctx := context.Background()
	good := []byte("expected bytes")
	hash := sha256.Sum256(good)
	digest := hex.EncodeToString(hash[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'v','r','s',?,'local://tenant/12/o','text/plain',?,'ready')`, digest, len(good)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('s',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BindVersion(ctx, careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}, "resume", "v"); err != nil {
		t.Fatal(err)
	}
	files := &careerArtifactTestFiles{bytes: []byte("tampered bytes")}
	h := NewCareerArtifactHandler(store, careerArtifactTestTenant{}, files, nil)
	secret := []byte(strings.Repeat("k", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	h.now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
	g := workbench.VersionArtifactGrant{TenantID: 12, OwnerID: "owner-a", ResourceID: "resume", VersionID: "v", Digest: digest, ExpiresAt: h.now().Add(time.Minute).UnixNano()}
	sig, err := workbench.SignVersionArtifactGrant(secret, g)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"tenant_id": {"12"}, "owner_id": {"owner-a"}, "resource_id": {"resume"}, "version_id": {"v"}, "digest": {digest}, "expires_at": {strconv.FormatInt(g.ExpiresAt, 10)}, "signature": {sig}}
	r := gin.New()
	r.GET("/download", h.Download)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download?"+values.Encode(), nil))
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("corrupt bytes response status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCareerArtifactDownloadRejectsWhenStageBudgetIsFullBeforeBlobOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, store := careerArtifactDB(t)
	body := []byte("budgeted bytes")
	hash := sha256.Sum256(body)
	digest := hex.EncodeToString(hash[:])
	if err := db.Exec(`INSERT INTO artifact_versions (tenant_id,id,run_id,session_id,digest,object_key,mime,size,scan_state) VALUES (12,'budget-v','r','s',?,'local://tenant/12/o','text/plain',?,'ready')`, digest, len(body)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('s',12,'owner-a')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.BindVersion(context.Background(), careerrepo.Scope{TenantID: 12, OwnerID: "owner-a"}, "resume", "budget-v"); err != nil {
		t.Fatal(err)
	}
	files := &careerArtifactTestFiles{bytes: body}
	h := NewCareerArtifactHandler(store, careerArtifactTestTenant{}, files, nil)
	secret := []byte(strings.Repeat("b", 32))
	h.key = func() ([]byte, error) { return secret, nil }
	h.now = func() time.Time { return time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC) }
	h.stageBudget = newCareerArtifactStageBudget(int64(len(body)))
	grant := workbench.VersionArtifactGrant{TenantID: 12, OwnerID: "owner-a", ResourceID: "resume", VersionID: "budget-v", Digest: digest, ExpiresAt: h.now().Add(time.Minute).UnixNano()}
	sig, err := workbench.SignVersionArtifactGrant(secret, grant)
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{"tenant_id": {"12"}, "owner_id": {grant.OwnerID}, "resource_id": {grant.ResourceID}, "version_id": {grant.VersionID}, "digest": {digest}, "expires_at": {strconv.FormatInt(grant.ExpiresAt, 10)}, "signature": {sig}}
	// Simulate another authorized request holding the only staging capacity.
	release, ok := h.stageBudget.TryReserve(int64(len(body)))
	if !ok {
		t.Fatal("could not reserve test staging capacity")
	}
	r := gin.New()
	r.GET("/download", h.Download)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/download?"+values.Encode(), nil))
	release()
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 || files.opens != 0 {
		t.Fatalf("capacity denial status=%d body=%q blob opens=%d", rec.Code, rec.Body.String(), files.opens)
	}
}

func careerArtifactDB(t *testing.T) (*gorm.DB, *careerrepo.ArtifactCatalogStore) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "career-artifact.db")+"?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, file := range []string{"000124_career_foundation.up.sql", "000067_artifact_versions.up.sql", "000125_career_artifact_bindings.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(root, "migrations/sqlite", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	if err := db.Exec(`CREATE TABLE sessions (id TEXT NOT NULL, tenant_id INTEGER NOT NULL, user_id TEXT, deleted_at DATETIME, PRIMARY KEY (tenant_id,id))`).Error; err != nil {
		t.Fatal(err)
	}
	return db, careerrepo.NewArtifactCatalogStore(db)
}
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var _ interfaces.TenantService = careerArtifactTestTenant{}
