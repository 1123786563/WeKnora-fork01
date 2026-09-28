package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/filetransport"
	"github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"
	careerrepo "github.com/Tencent/WeKnora/internal/modules/career/repository"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type CareerArtifactCatalog interface {
	AuthorizeArtifactGrant(context.Context, careerrepo.ArtifactGrant) error
	Resolve(context.Context, careerrepo.ArtifactGrant) (careerrepo.ArtifactVersion, error)
	WithResolved(context.Context, careerrepo.ArtifactGrant, func(careerrepo.ArtifactVersion) error) error
}

type careerGrantAuthorizer struct{ catalog CareerArtifactCatalog }

func (a careerGrantAuthorizer) AuthorizeVersionGrant(ctx context.Context, grant workbench.VersionArtifactGrant) error {
	return a.catalog.AuthorizeArtifactGrant(ctx, careerrepo.ArtifactGrant{TenantID: grant.TenantID, OwnerID: grant.OwnerID, ResourceID: grant.ResourceID, VersionID: grant.VersionID, Digest: grant.Digest})
}

// CareerArtifactHandler issues owner-scoped grants and redeems them only after
// rechecking the authoritative catalog. Blob bytes are staged and verified
// before an HTTP success response can begin.
type CareerArtifactHandler struct {
	catalog     CareerArtifactCatalog
	tenants     interfaces.TenantService
	files       interfaces.FileService
	storage     interfaces.StorageBackendResolver
	key         func() ([]byte, error)
	now         func() time.Time
	stageBudget *careerArtifactStageBudget
}

const (
	careerArtifactMaxSize       = int64(256 << 20)
	careerArtifactStageCapacity = int64(512 << 20)
)

var sharedCareerArtifactStageBudget = newCareerArtifactStageBudget(careerArtifactStageCapacity)

type careerArtifactStageBudget struct {
	mu       sync.Mutex
	capacity int64
	used     int64
}

func newCareerArtifactStageBudget(capacity int64) *careerArtifactStageBudget {
	return &careerArtifactStageBudget{capacity: capacity}
}

func (b *careerArtifactStageBudget) TryReserve(size int64) (func(), bool) {
	if b == nil || size <= 0 || size > b.capacity {
		return nil, false
	}
	b.mu.Lock()
	if size > b.capacity-b.used {
		b.mu.Unlock()
		return nil, false
	}
	b.used += size
	b.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { b.mu.Lock(); b.used -= size; b.mu.Unlock() }) }, true
}

func NewCareerArtifactHandler(catalog CareerArtifactCatalog, tenants interfaces.TenantService, files interfaces.FileService, storage interfaces.StorageBackendResolver) *CareerArtifactHandler {
	return &CareerArtifactHandler{catalog: catalog, tenants: tenants, files: files, storage: storage, key: workbench.ArtifactSigningKeyFromEnv, now: time.Now, stageBudget: sharedCareerArtifactStageBudget}
}

func (h *CareerArtifactHandler) Issue(c *gin.Context) {
	ctx := c.Request.Context()
	scope, err := careerrepo.ScopeFromContext(ctx)
	if err != nil || h == nil || h.catalog == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	key, err := h.key()
	if err != nil {
		c.AbortWithStatus(http.StatusNotImplemented)
		return
	}
	resourceID, versionID := c.Param("resource_id"), c.Param("version_id")
	// Resolve once to get server-owned digest. Authority.Issue then performs a
	// second live check, ensuring the issued token only names an active binding.
	grant := workbench.VersionArtifactGrant{TenantID: scope.TenantID, OwnerID: scope.OwnerID, ResourceID: resourceID, VersionID: versionID}
	version, err := h.catalog.Resolve(ctx, careerrepo.ArtifactGrant{TenantID: grant.TenantID, OwnerID: grant.OwnerID, ResourceID: grant.ResourceID, VersionID: grant.VersionID})
	if err != nil || version.Digest == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	authority := workbench.VersionArtifactGrantAuthority{Secret: key, Authorizer: careerGrantAuthorizer{catalog: h.catalog}}
	grant, signature, err := authority.Issue(ctx, scope.TenantID, scope.OwnerID, resourceID, versionID, version.Digest, h.now(), 5*time.Minute)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	values := url.Values{}
	values.Set("tenant_id", strconv.FormatUint(grant.TenantID, 10))
	values.Set("owner_id", grant.OwnerID)
	values.Set("resource_id", grant.ResourceID)
	values.Set("version_id", grant.VersionID)
	values.Set("digest", grant.Digest)
	values.Set("expires_at", strconv.FormatInt(grant.ExpiresAt, 10))
	values.Set("signature", signature)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"url": externalURLBase(c) + "/api/v1/career/artifacts/download?" + values.Encode(), "expires_at": time.Unix(0, grant.ExpiresAt).UTC().Format(time.RFC3339Nano)}})
}

func (h *CareerArtifactHandler) Download(c *gin.Context) {
	if h == nil || h.catalog == nil || h.files == nil || h.tenants == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	grant, signature, ok := parseCareerArtifactGrant(c.Request.URL.Query())
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	key, err := h.key()
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err := workbench.VerifyVersionArtifactGrantAt(key, grant, signature, h.now()); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	err = h.catalog.WithResolved(c.Request.Context(), careerrepo.ArtifactGrant{TenantID: grant.TenantID, OwnerID: grant.OwnerID, ResourceID: grant.ResourceID, VersionID: grant.VersionID, Digest: grant.Digest}, func(version careerrepo.ArtifactVersion) error {
		return h.downloadResolved(c, grant, version)
	})
	if err != nil && !c.Writer.Written() {
		c.AbortWithStatus(http.StatusNotFound)
	}
}

func (h *CareerArtifactHandler) downloadResolved(c *gin.Context, grant workbench.VersionArtifactGrant, version careerrepo.ArtifactVersion) error {
	if version.TenantID != grant.TenantID || version.OwnerID != grant.OwnerID || version.ResourceID != grant.ResourceID || version.VersionID != grant.VersionID || version.Digest != grant.Digest || version.Size <= 0 || version.Size > careerArtifactMaxSize {
		return careerrepo.ErrNotFound
	}
	releaseStage, reserved := h.stageBudget.TryReserve(version.Size)
	if !reserved {
		return careerrepo.ErrNotFound
	}
	defer releaseStage()
	ctx := types.WithExecutionTenant(c.Request.Context(), grant.TenantID)
	tenant, err := h.tenants.GetTenantByID(ctx, grant.TenantID)
	if err != nil || tenant == nil {
		return careerrepo.ErrNotFound
	}
	backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
	if !scoped {
		providerPath = version.ObjectKey
	}
	fileService, _, ok := filesvc.ResolveTenantFileServiceWithFallback(ctx, "career artifact download", tenant, backendID, types.ParseProviderScheme(providerPath), storageurl.LocalStorageBaseDir(), h.storage, h.files)
	if !ok || fileService == nil {
		return careerrepo.ErrNotFound
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return careerrepo.ErrNotFound
	}
	defer reader.Close()
	tmp, err := os.CreateTemp("", "weknora-career-artifact-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	defer tmp.Close()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	hasher := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(contextReader{ctx: ctx, reader: reader}, version.Size+1))
	if err != nil || n != version.Size || hex.EncodeToString(hasher.Sum(nil)) != strings.ToLower(version.Digest) {
		return careerrepo.ErrNotFound
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.Header("Cache-Control", "private, no-store")
	if err := filetransport.Serve(c.Writer, c.Request, tmp, filetransport.Options{Filename: versionFilename(version.MIME), Download: true, ContentType: version.MIME, Size: version.Size, CacheControl: "private, no-store"}); err != nil {
		return err
	}
	return nil
}

func parseCareerArtifactGrant(q url.Values) (workbench.VersionArtifactGrant, string, bool) {
	tenant, err := strconv.ParseUint(q.Get("tenant_id"), 10, 64)
	if err != nil || tenant == 0 {
		return workbench.VersionArtifactGrant{}, "", false
	}
	expires, err := strconv.ParseInt(q.Get("expires_at"), 10, 64)
	if err != nil {
		return workbench.VersionArtifactGrant{}, "", false
	}
	g := workbench.VersionArtifactGrant{TenantID: tenant, OwnerID: q.Get("owner_id"), ResourceID: q.Get("resource_id"), VersionID: q.Get("version_id"), Digest: q.Get("digest"), ExpiresAt: expires}
	if _, err := g.Canonical(); err != nil || q.Get("signature") == "" {
		return workbench.VersionArtifactGrant{}, "", false
	}
	return g, q.Get("signature"), true
}

func versionFilename(mimeType string) string {
	ext := ".bin"
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "application/pdf":
		ext = ".pdf"
	case "text/plain":
		ext = ".txt"
	case "application/json":
		ext = ".json"
	}
	return "career-artifact" + ext
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
