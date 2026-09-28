package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	careerrepo "github.com/Tencent/WeKnora/internal/modules/career/repository"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type careerArtifactTestFiles struct {
	interfaces.FileService
	bytes []byte
	opens int
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
