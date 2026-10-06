package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ─── stubs ───────────────────────────────────────────────────────────────────

type artifactRefReaderStub struct {
	refs   []types.SessionArtifactRef
	err    error
	calls  int
	lastID string
}

func (s *artifactRefReaderStub) GetSessionArtifactRefs(_ context.Context, sessionID string) ([]types.SessionArtifactRef, error) {
	s.calls++
	s.lastID = sessionID
	return s.refs, s.err
}

type artifactVersionReaderStub struct {
	version repository.ArtifactVersion
	err     error
	calls   int
}

func (s *artifactVersionReaderStub) ReadableArtifactVersion(_ context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) {
	s.calls++
	if tenantID != s.version.TenantID || sessionID != s.version.SessionID || versionID != s.version.ID {
		return repository.ArtifactVersion{}, repository.ErrArtifactVersionNotFound
	}
	return s.version, s.err
}

// grantMessageServiceStub satisfies exactly what DownloadWorkbenchArtifactGrant
// needs. Embedding the interface keeps the stub honest without implementing
// every message operation.
type grantMessageServiceStub struct {
	interfaces.MessageService
	refs []types.SessionArtifactRef
	err  error
}

func (s *grantMessageServiceStub) GetSessionArtifactRefs(context.Context, string) ([]types.SessionArtifactRef, error) {
	return s.refs, s.err
}

func artifactRunStub() *workbenchRunReaderStub {
	return &workbenchRunReaderStub{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "sess-1",
		UserID:    "u1",
	}}
}

type artifactMembershipStub struct {
	member *types.TenantMember
	err    error
	calls  int
	userID string
	tenant uint64
}

func (s *artifactMembershipStub) Get(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	s.calls++
	s.userID = userID
	s.tenant = tenantID
	return s.member, s.err
}

func TestCreateWorkbenchArtifactVersionSignedURLBindsFixedReadyVersion(t *testing.T) {
	signingEnv(t)
	version := repository.ArtifactVersion{TenantID: 1, ID: "version-7", RunID: "run-1", SessionID: "sess-1", Digest: strings.Repeat("a", 64), ScanState: repository.ArtifactScanReady, Size: 42, MIME: "application/pdf"}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &artifactRefReaderStub{}).WithArtifactVersions(&artifactVersionReaderStub{version: version})
	c, rec := artifactContext()
	c.Request.Method = http.MethodPost
	c.Params = append(c.Params, gin.Params{{Key: "version_id", Value: version.ID}}...)
	h.CreateWorkbenchArtifactVersionSignedURL(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			URL       string `json:"url"`
			VersionID string `json:"version_id"`
			Digest    string `json:"digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, version.ID, body.Data.VersionID)
	require.Equal(t, version.Digest, body.Data.Digest)
	require.Contains(t, body.Data.URL, "grant_type=artifact_version")
	require.Contains(t, body.Data.URL, "owner_id=u1")
	require.Contains(t, body.Data.URL, "version_id=version-7")

	// Decode the emitted link and verify the signature covers the authenticated
	// tenant/owner and the exact owned run/session/version tuple.
	link, err := url.Parse(body.Data.URL)
	require.NoError(t, err)
	query := link.Query()
	require.Equal(t, "1", query.Get("tenant_id"))
	require.Equal(t, "u1", query.Get("owner_id"))
	require.Equal(t, "run-1", query.Get("run_id"))
	require.Equal(t, "sess-1", query.Get("session_id"))
	require.Equal(t, version.ID, query.Get("version_id"))
	expiresAt, err := strconv.ParseInt(query.Get("expires_at"), 10, 64)
	require.NoError(t, err)
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	grant := workbench.ArtifactVersionGrant{
		TenantID: 1, OwnerID: "u1", RunID: "run-1", SessionID: "sess-1",
		VersionID: version.ID, ExpiresAt: expiresAt,
	}
	require.NoError(t, workbench.VerifyArtifactVersionGrantAt(key, grant, query.Get("signature"), time.Now()))
	for _, tampered := range []workbench.ArtifactVersionGrant{
		{TenantID: 2, OwnerID: grant.OwnerID, RunID: grant.RunID, SessionID: grant.SessionID, VersionID: grant.VersionID, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, OwnerID: "u2", RunID: grant.RunID, SessionID: grant.SessionID, VersionID: grant.VersionID, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, OwnerID: grant.OwnerID, RunID: "run-2", SessionID: grant.SessionID, VersionID: grant.VersionID, ExpiresAt: grant.ExpiresAt},
	} {
		require.Error(t, workbench.VerifyArtifactVersionGrantAt(key, tampered, query.Get("signature"), time.Now()))
	}
}

func TestDownloadArtifactVersionGrantRechecksOwnerAndRevocation(t *testing.T) {
	signingEnv(t)
	version := repository.ArtifactVersion{TenantID: 1, ID: "version-7", RunID: "run-1", SessionID: "sess-1", ScanState: repository.ArtifactScanReady}
	grant := workbench.ArtifactVersionGrant{TenantID: 1, OwnerID: "u1", RunID: "run-1", SessionID: "sess-1", VersionID: version.ID, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	signature, err := workbench.SignArtifactVersionGrant(key, grant)
	require.NoError(t, err)
	query := "grant_type=artifact_version&tenant_id=1&owner_id=u1&run_id=run-1&session_id=sess-1&version_id=version-7&expires_at=" + strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + signature
	versions := &artifactVersionReaderStub{version: version}
	runs := artifactRunStub()
	versionHandler := NewArtifactVersionDownloadHandler(nil, nil, nil, nil, versions, runs).WithTenantMembership(&artifactMembershipStub{member: &types.TenantMember{UserID: "u1", TenantID: 1, Status: types.TenantMemberStatusActive}})
	RegisterArtifactVersionDownloadHandler(versionHandler)
	t.Cleanup(func() { RegisterArtifactVersionDownloadHandler(nil) })
	h := &Handler{}
	c, _ := grantDownloadContext(query)
	h.DownloadWorkbenchArtifactGrant(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status()) // no file service is wired in this authorization test
	require.Equal(t, 1, runs.calls)
	require.Equal(t, 1, versions.calls)

	// Revoked/unpublished versions are intentionally indistinguishable from a
	// missing version, even while the grant's signature remains valid.
	revoked := NewArtifactVersionDownloadHandler(nil, nil, nil, nil, &artifactVersionReaderStub{version: version, err: repository.ErrArtifactVersionNotFound}, artifactRunStub())
	RegisterArtifactVersionDownloadHandler(revoked)
	c2, _ := grantDownloadContext(query)
	h.DownloadWorkbenchArtifactGrant(c2)
	require.Equal(t, http.StatusNotFound, c2.Writer.Status())

	// Owner revocation/deletion is also rechecked against the current run.
	wrongOwner := artifactRunStub()
	wrongOwner.err = agentruntime.ErrNotFound
	revokedOwner := NewArtifactVersionDownloadHandler(nil, nil, nil, nil, &artifactVersionReaderStub{version: version}, wrongOwner)
	RegisterArtifactVersionDownloadHandler(revokedOwner)
	c3, _ := grantDownloadContext(query)
	h.DownloadWorkbenchArtifactGrant(c3)
	require.Equal(t, http.StatusNotFound, c3.Writer.Status())
	require.Zero(t, revokedOwner.versions.(*artifactVersionReaderStub).calls)
}

func TestDownloadArtifactVersionGrantRequiresCurrentActiveMembership(t *testing.T) {
	signingEnv(t)
	version := repository.ArtifactVersion{TenantID: 1, ID: "version-7", RunID: "run-1", SessionID: "sess-1", ScanState: repository.ArtifactScanReady}
	grant := workbench.ArtifactVersionGrant{TenantID: 1, OwnerID: "u1", RunID: "run-1", SessionID: "sess-1", VersionID: version.ID, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	signature, err := workbench.SignArtifactVersionGrant(key, grant)
	require.NoError(t, err)
	query := "grant_type=artifact_version&tenant_id=1&owner_id=u1&run_id=run-1&session_id=sess-1&version_id=version-7&expires_at=" + strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + signature

	for _, tc := range []struct {
		name   string
		member *types.TenantMember
		err    error
	}{
		{name: "removed", member: nil},
		{name: "suspended", member: &types.TenantMember{UserID: "u1", TenantID: 1, Status: types.TenantMemberStatusSuspended}},
		{name: "reader error", err: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			members := &artifactMembershipStub{member: tc.member, err: tc.err}
			handler := NewArtifactVersionDownloadHandler(nil, nil, nil, nil, &artifactVersionReaderStub{version: version}, artifactRunStub()).WithTenantMembership(members)
			RegisterArtifactVersionDownloadHandler(handler)
			t.Cleanup(func() { RegisterArtifactVersionDownloadHandler(nil) })
			c, _ := grantDownloadContext(query)
			(&Handler{}).DownloadWorkbenchArtifactGrant(c)
			require.Equal(t, http.StatusNotFound, c.Writer.Status())
			require.Equal(t, 1, members.calls)
			require.Equal(t, "u1", members.userID)
			require.Equal(t, uint64(1), members.tenant)
		})
	}
}

func TestDownloadArtifactVersionGrantStreamsExactBytesAndRevokedIssuedLinkFails(t *testing.T) {
	signingEnv(t)
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	_, err = sqlDB.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, session_id TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id) VALUES (1, 'run-1', 'sess-1')`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`CREATE TABLE artifact_versions (
		tenant_id INTEGER NOT NULL, id TEXT NOT NULL, run_id TEXT NOT NULL, session_id TEXT NOT NULL,
		digest TEXT NOT NULL, object_key TEXT NOT NULL, mime TEXT NOT NULL, scan_state TEXT NOT NULL,
		size INTEGER NOT NULL, revoked BOOLEAN NOT NULL DEFAULT FALSE, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (tenant_id, id))`)
	require.NoError(t, err)

	data := []byte("exact artifact payload\n")
	digest := sha256.Sum256(data)
	version := repository.ArtifactVersion{TenantID: 1, ID: "version-7", RunID: "run-1", SessionID: "sess-1", Digest: hex.EncodeToString(digest[:]), ObjectKey: "artifact-key", MIME: "text/plain", ScanState: repository.ArtifactScanPending, Size: int64(len(data))}
	versions := repository.NewArtifactVersionStore(db)
	require.NoError(t, versions.Insert(ctx, version))
	require.NoError(t, versions.MarkUploaded(ctx, 1, version.ID))
	require.NoError(t, versions.MarkScanned(ctx, 1, version.ID, true))
	require.NoError(t, versions.MarkReady(ctx, 1, version.ID))
	version.ScanState = repository.ArtifactScanReady

	grant := workbench.ArtifactVersionGrant{TenantID: 1, OwnerID: "u1", RunID: "run-1", SessionID: "sess-1", VersionID: version.ID, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	signature, err := workbench.SignArtifactVersionGrant(key, grant)
	require.NoError(t, err)
	query := "grant_type=artifact_version&tenant_id=1&owner_id=u1&run_id=run-1&session_id=sess-1&version_id=version-7&expires_at=" + strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + signature
	file := &fakeArtifactFileService{url: version.ObjectKey, data: data}
	members := &artifactMembershipStub{member: &types.TenantMember{UserID: "u1", TenantID: 1, Status: types.TenantMemberStatusActive}}
	versionHandler := NewArtifactVersionDownloadHandler(nil, nil, file, nil, versions, artifactRunStub()).WithTenantMembership(members)
	RegisterArtifactVersionDownloadHandler(versionHandler)
	t.Cleanup(func() { RegisterArtifactVersionDownloadHandler(nil) })

	serve := func() *httptest.ResponseRecorder {
		c, recorder := grantDownloadContext(query)
		(&Handler{}).DownloadWorkbenchArtifactGrant(c)
		return recorder
	}
	response := serve()
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, data, response.Body.Bytes())
	gotDigest := sha256.Sum256(response.Body.Bytes())
	require.Equal(t, version.Digest, hex.EncodeToString(gotDigest[:]))
	require.Equal(t, "text/plain", response.Header().Get("Content-Type"))
	require.Contains(t, response.Header().Get("Content-Disposition"), "attachment; filename=\"artifact-version-7.txt\"")
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, strconv.Itoa(len(data)), response.Header().Get("Content-Length"))

	artifactHandler := NewWorkbenchArtifactHandler(artifactRunStub(), nil).
		WithArtifactVersions(versions).
		WithArtifactVersionRevoker(versions)
	denied, deniedRecorder := workbenchRequest(t, "not-owner")
	denied.Request.Method = http.MethodDelete
	denied.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "version_id", Value: version.ID}}
	artifactHandler.RevokeWorkbenchArtifactVersion(denied)
	require.Equal(t, http.StatusNotFound, deniedRecorder.Code, "a non-owner must not revoke another user's version")
	_, err = versions.ReadableArtifactVersion(ctx, 1, version.SessionID, version.ID)
	require.NoError(t, err, "denied revocation must leave the issued link readable")

	revokeRequest, revokeRecorder := workbenchRequest(t, "u1")
	revokeRequest.Request.Method = http.MethodDelete
	revokeRequest.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "version_id", Value: version.ID}}
	artifactHandler.RevokeWorkbenchArtifactVersion(revokeRequest)
	require.Equal(t, http.StatusOK, revokeRecorder.Code, "the run owner can revoke this version")
	response = serve()
	require.Equal(t, http.StatusNotFound, response.Code, "the already-issued link must fail after persisted revocation")
}

func artifactRefs() []types.SessionArtifactRef {
	return []types.SessionArtifactRef{
		{MessageID: "msg-1", Index: 0, Artifact: types.MessageArtifact{
			URL: "local://tenant/1/blob-a.md", FileName: "summary.md", FileType: ".md", FileSize: 128,
			CreatedAt: time.Unix(1_700_000_000, 0),
		}},
		{MessageID: "msg-2", Index: 1, Artifact: types.MessageArtifact{
			URL: "local://tenant/1/blob-b.csv", FileName: "data.csv", FileType: ".csv", FileSize: 2048,
			CreatedAt: time.Unix(1_700_000_001, 0),
		}},
	}
}

func artifactContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/run-1/artifacts", nil)
	c.Request.Host = "weknora.example.com"
	// Signed links must honour the forwarding proxy scheme, not the hop scheme.
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = c.Request.WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	return c, recorder
}

// ─── list ────────────────────────────────────────────────────────────────────

func TestListWorkbenchArtifactsProjectsMessageBoundRefs(t *testing.T) {
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Equal(t, "sess-1", refs.lastID)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []workbenchArtifactItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Items, 2)

	first := body.Data.Items[0]
	require.Equal(t, 0, first.Index)
	require.Equal(t, "msg-1:0", first.ID)
	require.Equal(t, "summary.md", first.Name)
	require.Equal(t, "application/octet-stream", first.Mime) // .md has no stdlib mime; unknown types force download
	require.Equal(t, int64(128), first.Size)
	require.Equal(t, "run-1", first.SourceRun)
	require.Equal(t, "msg-1:0", first.Version) // 无 ContentHash 时以 (message, index) 绑定地址为版本身份
}

func TestListWorkbenchArtifactsDeclaresTerminalAvailabilityFromWiring(t *testing.T) {
	refs := &artifactRefReaderStub{refs: artifactRefs()}
	runs := artifactRunStub()

	// 未接线 TerminalLogReader 的装配：terminal.available 必须如实为 false（B3-F76）。
	h := NewWorkbenchArtifactHandler(runs, refs)
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), `"terminal":{"available":false}`, "未接线时能力位必须如实为 false")

	// 接线后：available 为 true（与 terminal-log 端点的 501 判定同源）。
	h2 := NewWorkbenchArtifactHandler(runs, refs).WithTerminalLog(&terminalReaderStub{})
	c2, rec2 := artifactContext()
	h2.ListWorkbenchArtifacts(c2)
	require.Equal(t, http.StatusOK, c2.Writer.Status())
	require.Contains(t, rec2.Body.String(), `"terminal":{"available":true}`)
}

func TestListWorkbenchArtifactsScopesByOwner(t *testing.T) {
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, _ := artifactContext()
	// Different tenant: the run reader answers not-found, refs never queried.
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(2))
	c.Request = c.Request.WithContext(ctx)
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
	require.Zero(t, refs.calls)
}

func TestListWorkbenchArtifactsDerivesVersionFromDigest(t *testing.T) {
	digest := "aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd"
	refs := artifactRefReaderStub{refs: []types.SessionArtifactRef{
		{MessageID: "msg-9", Index: 0, Artifact: types.MessageArtifact{
			URL: "local://tenant/1/report.md", FileName: "report.md", FileType: ".md", FileSize: 10,
			ContentHash: digest,
			CreatedAt:   time.Unix(1_700_000_000, 0),
		}},
	}}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Items []workbenchArtifactItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 1)
	// 内容寻址：版本取 digest 前缀，digest 原文随行下发——内容变则两者变，不可原地改写。
	require.Equal(t, digest[:16], body.Data.Items[0].Version)
	require.Equal(t, digest, body.Data.Items[0].Digest)
}

func TestListWorkbenchArtifactsDeclaresTerminalAvailability(t *testing.T) {
	refs := artifactRefReaderStub{refs: artifactRefs()}
	// The production container wires the read-side snapshot repository as the
	// terminal reader; the flag now mirrors the wiring instead of hardcoding
	// true, so this assembly-level case asserts the wired shape (B3-F76).
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs).WithTerminalLog(&terminalReaderStub{})
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Terminal struct {
				Available bool `json:"available"`
			} `json:"terminal"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Data.Terminal.Available, "a wired terminal reader declares availability")
}

// ─── signed url ──────────────────────────────────────────────────────────────

func signingEnv(t *testing.T) {
	t.Helper()
	t.Setenv(workbench.ArtifactSigningKeyEnvVar, strings.Repeat("ab", 32))
}

func TestCreateWorkbenchArtifactSignedURLMintsScopedGrant(t *testing.T) {
	signingEnv(t)
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, rec := artifactContext()
	c.Request.Method = http.MethodPost
	c.Params = append(c.Params, gin.Params{{Key: "index", Value: "1"}}...)
	h.CreateWorkbenchArtifactSignedURL(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			URL       string                `json:"url"`
			ExpiresAt string                `json:"expires_at"`
			Artifact  workbenchArtifactItem `json:"artifact"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body.Data.URL, "https://weknora.example.com/api/v1/workbench/artifacts/download?")
	require.Contains(t, body.Data.URL, "session_id=sess-1")
	require.Contains(t, body.Data.URL, "message_id=msg-2")
	require.Contains(t, body.Data.URL, "index=1")
	require.Contains(t, body.Data.URL, "tenant_id=1")
	require.Contains(t, body.Data.URL, "signature=")
	// Expiry is close to the TTL cap and no-store is honoured.
	expiresAt, err := time.Parse(time.RFC3339, body.Data.ExpiresAt)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(workbench.MaxArtifactGrantTTL), expiresAt, time.Minute)
	require.Equal(t, "data.csv", body.Data.Artifact.Name)
}

func TestCreateWorkbenchArtifactSignedURLClampsTTLAndRangeChecks(t *testing.T) {
	signingEnv(t)
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)

	// ttl beyond the cap is clamped, not honoured.
	c, rec2 := artifactContext()
	c.Request.Method = http.MethodPost
	c.Params = append(c.Params, gin.Params{{Key: "index", Value: "0"}}...)
	q := c.Request.URL.Query()
	q.Set("ttl_seconds", "999999")
	c.Request.URL.RawQuery = q.Encode()
	h.CreateWorkbenchArtifactSignedURL(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var ttlBody struct {
		Data struct {
			ExpiresAt string `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &ttlBody))
	expiresAt, err := time.Parse(time.RFC3339, ttlBody.Data.ExpiresAt)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(workbench.MaxArtifactGrantTTL), expiresAt, time.Minute)

	// Out-of-range index answers 404 without leaking artifact count.
	c2, _ := artifactContext()
	c2.Request.Method = http.MethodPost
	c2.Params = append(c2.Params, gin.Params{{Key: "index", Value: "9"}}...)
	h.CreateWorkbenchArtifactSignedURL(c2)
	require.Equal(t, http.StatusNotFound, c2.Writer.Status())
}

func TestCreateWorkbenchArtifactSignedURLFailsClosedWithoutKey(t *testing.T) {
	t.Setenv(workbench.ArtifactSigningKeyEnvVar, "")
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, _ := artifactContext()
	c.Request.Method = http.MethodPost
	c.Params = append(c.Params, gin.Params{{Key: "index", Value: "0"}}...)
	h.CreateWorkbenchArtifactSignedURL(c)
	require.Equal(t, http.StatusNotImplemented, c.Writer.Status())
}

// ─── grant download ──────────────────────────────────────────────────────────

func grantDownloadContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/artifacts/download?"+query, nil)
	return c, recorder
}

func TestDownloadWorkbenchArtifactGrantRejectsTamperedAndExpired(t *testing.T) {
	signingEnv(t)
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	msgs := &grantMessageServiceStub{refs: artifactRefs()}
	h := &Handler{messageService: msgs}

	// Tampered signature (rebound to another message) is rejected.
	grant := workbench.ArtifactGrant{TenantID: 1, SessionID: "sess-1", MessageID: "msg-1", Index: 0, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	sig, err := workbench.SignArtifactGrant(key, grant)
	require.NoError(t, err)
	q := "tenant_id=1&session_id=sess-1&message_id=msg-2&index=1&expires_at=" +
		strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + sig
	c, _ := grantDownloadContext(q)
	h.DownloadWorkbenchArtifactGrant(c)
	require.Equal(t, http.StatusUnauthorized, c.Writer.Status())

	// Expired grant reports the dedicated code so clients can re-authorize.
	expired := workbench.ArtifactGrant{TenantID: 1, SessionID: "sess-1", MessageID: "msg-1", Index: 0, ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	expSig, err := workbench.SignArtifactGrant(key, expired)
	require.NoError(t, err)
	c2, rec2 := grantDownloadContext("tenant_id=1&session_id=sess-1&message_id=msg-1&index=0&expires_at=" +
		strconv.FormatInt(expired.ExpiresAt, 10) + "&signature=" + expSig)
	h.DownloadWorkbenchArtifactGrant(c2)
	require.Equal(t, http.StatusUnauthorized, c2.Writer.Status())
	require.Contains(t, rec2.Body.String(), "artifact_grant_expired")
}

func TestDownloadWorkbenchArtifactGrantFailsClosedWithoutKey(t *testing.T) {
	t.Setenv(workbench.ArtifactSigningKeyEnvVar, "")
	h := &Handler{messageService: &grantMessageServiceStub{}}
	c, _ := grantDownloadContext("tenant_id=1&session_id=s&message_id=m&index=0&expires_at=9999999999&signature=deadbeef")
	h.DownloadWorkbenchArtifactGrant(c)
	require.Equal(t, http.StatusNotImplemented, c.Writer.Status())
}

func TestDownloadWorkbenchArtifactGrantResolvesAndStreams(t *testing.T) {
	signingEnv(t)
	key, err := workbench.ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	grant := workbench.ArtifactGrant{TenantID: 1, SessionID: "sess-1", MessageID: "msg-2", Index: 1, ExpiresAt: time.Now().Add(time.Minute).Unix()}
	sig, err := workbench.SignArtifactGrant(key, grant)
	require.NoError(t, err)

	msgs := &grantMessageServiceStub{refs: artifactRefs()}
	// No file service wired: a verified grant that resolves the artifact must
	// reach the streaming stage. The test context has no gin error middleware,
	// so the handler records "file service unavailable" on c.Errors instead of
	// writing 500 — asserting on the recorded error proves signature, expiry,
	// and message re-resolution all passed. Byte streaming itself is the same
	// code path as the authenticated per-message download.
	h := &Handler{messageService: msgs}
	c, _ := grantDownloadContext("tenant_id=1&session_id=sess-1&message_id=msg-2&index=1&expires_at=" +
		strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + sig)
	h.DownloadWorkbenchArtifactGrant(c)
	require.NotEmpty(t, c.Errors)
	require.Contains(t, c.Errors.String(), "file service unavailable")

	// Valid signature but deleted message → plain 404, existence not leaked.
	msgsEmpty := &grantMessageServiceStub{refs: nil}
	h2 := &Handler{messageService: msgsEmpty}
	other := grant
	other.MessageID = "msg-9"
	other.Index = 0
	otherSig, sigErr := workbench.SignArtifactGrant(key, other)
	require.NoError(t, sigErr)
	c2, _ := grantDownloadContext("tenant_id=1&session_id=sess-1&message_id=msg-9&index=0&expires_at=" +
		strconv.FormatInt(grant.ExpiresAt, 10) + "&signature=" + otherSig)
	h2.DownloadWorkbenchArtifactGrant(c2)
	require.Equal(t, http.StatusNotFound, c2.Writer.Status())
}
