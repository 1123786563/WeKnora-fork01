package session

// T34 #64 Task 9 (File B): AgentQA HTTP/claim E2E. The chat turn entry must
// refuse revoked releases / exact revoked dependencies with 409 and zero SSE
// and zero claim/message/upload effects, and an in-flight claimed turn must
// follow the revocation disposition: cancel interrupts and terminalizes,
// allow completes. Only the external AgentQA executor is a deterministic
// barrier adapter; everything else is the real stack.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedAgentSecurityE2ELineage plants one adopted-Agent lineage (published
// Variant pinned to an immutable Version + Release whose dependency lock
// names one model dependency) WITHOUT any revocation, so revocations can be
// driven through the real security service afterwards. tag separates the
// cancel and allow lineages: one release can only be revoked once.
func seedAgentSecurityE2ELineage(t *testing.T, db *gorm.DB, tag string) (*types.CustomAgent, types.AgentReleaseDependency) {
	t.Helper()
	agent := &types.CustomAgent{
		ID: "agent-e2e-" + tag, Name: "E2E Adopted " + tag, TenantID: 1, CreatedBy: "u1",
		Config: types.CustomAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}
	require.NoError(t, db.Create(agent).Error)
	now := time.Now().UTC()
	// The real AgentVersionService verifies sha256(snapshot) == source_sha256
	// and executes the frozen snapshot, not the mutable agent row.
	snapshotJSON, err := json.Marshal(agent)
	require.NoError(t, err)
	snapshotSum := sha256.Sum256(snapshotJSON)
	require.NoError(t, db.Create(&types.AgentVersionEntity{
		ID: "e2e-ver-" + tag, TenantID: 1, AgentID: agent.ID, VersionNumber: 1,
		Snapshot: string(snapshotJSON), SourceSHA256: hex.EncodeToString(snapshotSum[:]), FrozenBy: "u1", CreatedAt: now,
	}).Error)
	dependency := types.AgentReleaseDependency{Type: "model", ID: "gpt-x", Version: "1.0.0", Digest: strings.Repeat("d", 64)}
	lockJSON, err := json.Marshal(types.DependencyLock{Dependencies: []types.AgentReleaseDependency{dependency}})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_marketplace_listings (id, tenant_id, source_agent_id, display_name, state)
		VALUES (?, 1, ?, 'E2E listing', 'listed')`, "e2e-ls-"+tag, agent.ID).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_release_submissions (id, tenant_id, listing_id, agent_version_id, source_agent_id, author_id, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, status)
		VALUES (?, 1, ?, ?, ?, 'publisher', '1.0.0', 'e2e-digest', '{}', ?, X'7B7D', 'approved')`, "e2e-sub-"+tag, "e2e-ls-"+tag, "e2e-ver-"+tag, agent.ID, string(lockJSON)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_releases (id, tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, published_by)
		VALUES (?, 1, ?, ?, ?, ?, 1, '1.0.0', 'e2e-digest', '{}', ?, X'7B7D', 'publisher')`, "e2e-rel-"+tag, "e2e-ls-"+tag, "e2e-sub-"+tag, "e2e-ver-"+tag, agent.ID, string(lockJSON)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by)
		VALUES (?, 1, ?, ?, 'active', 'u1')`, "e2e-ad-"+tag, "e2e-ls-"+tag, "e2e-rel-"+tag).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoption_variants (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, published_by, published_at)
		VALUES (?, 1, ?, ?, 'v1', 'published', ?, ?, 'u1', ?)`, "e2e-var-"+tag, "e2e-ad-"+tag, "e2e-rel-"+tag, agent.ID, "e2e-ver-"+tag, now).Error)
	return agent, dependency
}

func seedAgentSecurityE2ESession(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-chat', 1, 'security e2e', 'u1', 'builtin')`).Error)
}

// agentSecurityE2EQASessions keeps the real owner-scoped session read; the
// external AgentQA executor is the only barrier: it parks until released or
// the engine's cancellation propagates.
type agentSecurityE2EQASessions struct {
	interfaces.SessionService
	repo    interfaces.SessionRepository
	release chan struct{}
	calls   int32
}

func (s *agentSecurityE2EQASessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	return s.repo.Get(ctx, tenantID, userID, id)
}

func (s *agentSecurityE2EQASessions) AgentQA(ctx context.Context, _ *types.QARequest, _ *event.EventBus) error {
	atomic.AddInt32(&s.calls, 1)
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	return nil
}

func (s *agentSecurityE2EQASessions) UpdateSessionLastRequestState(context.Context, string, *types.SessionLastRequestState) error {
	return nil
}

// agentSecurityE2EFiles counts upload writes so a blocked turn's zero upload
// side effects are observable.
type agentSecurityE2EFiles struct {
	interfaces.FileService
	saved int32
}

func (f *agentSecurityE2EFiles) SaveBytes(context.Context, []byte, uint64, string, bool) (string, error) {
	atomic.AddInt32(&f.saved, 1)
	return "files://e2e", nil
}

// newAgentSecurityE2EEngine assembles the real chat surface (Handler over
// real services/stores) plus revocation routes backed by the real
// AgentSecurityService. The handler package cannot be imported here (import
// cycle), so the route bodies mirror the production handler semantics.
func newAgentSecurityE2EEngine(t *testing.T, db *gorm.DB, qa *agentSecurityE2EQASessions, files *agentSecurityE2EFiles) (*gin.Engine, *appservice.AgentSecurityService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	customAgents := appservice.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := appservice.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	security := appservice.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(versions)

	h := &Handler{
		sessionService:          qa,
		customAgentService:      customAgents,
		agentVersionService:     versions,
		agentChatTurnClaimStore: repository.NewAgentChatTurnClaimRepository(db),
		messageService:          &claimNoopMessages{},
		streamManager:           &claimReplayStream{release: qa.release},
		fileService:             files,
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	r.POST("/agent-chat/:session_id", h.AgentQA)
	r.POST("/api/v1/marketplace/tenant/security-revocations/releases", func(c *gin.Context) {
		var body struct {
			ReleaseID            string `json:"release_id"`
			Reason               string `json:"reason"`
			ReplacementReleaseID string `json:"replacement_release_id,omitempty"`
			InFlightDisposition  string `json:"in_flight_disposition,omitempty"`
		}
		require.NoError(t, c.ShouldBindJSON(&body))
		actor, _ := types.UserIDFromContext(c.Request.Context())
		view, err := security.RevokeRelease(c.Request.Context(), 1, actor, interfaces.ReleaseRevocationInput{
			ReleaseID: body.ReleaseID, Reason: body.Reason,
			ReplacementReleaseID: body.ReplacementReleaseID, InFlightDisposition: body.InFlightDisposition,
		})
		if err != nil {
			c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
	})
	r.POST("/api/v1/marketplace/tenant/security-revocations/dependencies", func(c *gin.Context) {
		var body struct {
			Dependency          *agentSecurityE2EDependencyBody `json:"dependency"`
			Reason              string                          `json:"reason"`
			ReplacementVersion  string                          `json:"replacement_version,omitempty"`
			InFlightDisposition string                          `json:"in_flight_disposition,omitempty"`
		}
		require.NoError(t, c.ShouldBindJSON(&body))
		if body.Dependency == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false})
			return
		}
		actor, _ := types.UserIDFromContext(c.Request.Context())
		view, err := security.RevokeDependency(c.Request.Context(), 1, actor, interfaces.DependencyRevocationInput{
			Dependency: types.AgentReleaseDependency{Type: body.Dependency.Type, ID: body.Dependency.ID, Version: body.Dependency.Version, Digest: body.Dependency.Digest},
			Reason:     body.Reason, ReplacementVersion: body.ReplacementVersion, InFlightDisposition: body.InFlightDisposition,
		})
		if err != nil {
			c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
	})
	return r, security
}

type agentSecurityE2EDependencyBody struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

func agentSecurityE2EPost(r *gin.Engine, path, body, requestID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func agentSecurityE2EChatBody(agentID, query string) string {
	return `{"query":"` + query + `","agent_id":"` + agentID + `","agent_enabled":true,` +
		`"images":[{"data":"data:image/png;base64,aGk="}],"attachment_uploads":[{"file_name":"notes.txt","data":"aGVsbG8="}]}`
}

// agentSecurityE2EPlainBody omits attachments: in-flight scenarios must not
// trip the post-admission image-enable 400 before the executor is reached.
func agentSecurityE2EPlainBody(agentID, query string) string {
	return `{"query":"` + query + `","agent_id":"` + agentID + `","agent_enabled":true}`
}

// agentSecurityE2EAssertZeroEffects asserts the blocked-turn boundary: no SSE
// stream was opened and no claim/message/upload side effect landed.
func agentSecurityE2EAssertZeroEffects(t *testing.T, w *httptest.ResponseRecorder, db *gorm.DB, files *agentSecurityE2EFiles) {
	t.Helper()
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.NotContains(t, w.Header().Get("Content-Type"), "text/event-stream", "a refused turn must not open SSE")
	var claims, messages int64
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Count(&claims).Error)
	require.NoError(t, db.Model(&types.Message{}).Count(&messages).Error)
	require.Zero(t, claims, "a refused turn must not persist a claim row")
	require.Zero(t, messages, "a refused turn must not persist a message row")
	require.Zero(t, atomic.LoadInt32(&files.saved), "a refused turn must not persist uploads")
}

// TestAgentSecurityE2EAgentChatTurnRefusedForRevokedRelease proves the chat
// entry fails closed on a revoked release: the turn is refused with 409, no
// SSE byte is sent, and no claim/message/upload side effect lands.
func TestAgentSecurityE2EAgentChatTurnRefusedForRevokedRelease(t *testing.T) {
	db := openCraftHTTPDB(t)
	seedAgentSecurityE2ESession(t, db)
	seedAgentSecurityE2ELineage(t, db, "rel")
	qa := &agentSecurityE2EQASessions{repo: repository.NewSessionRepository(db), release: make(chan struct{})}
	files := &agentSecurityE2EFiles{}
	r, _ := newAgentSecurityE2EEngine(t, db, qa, files)

	revoke := agentSecurityE2EPost(r, "/api/v1/marketplace/tenant/security-revocations/releases",
		`{"release_id":"e2e-rel-rel","reason":"e2e chat revoked","in_flight_disposition":"cancel"}`, "rid-revoke-release")
	require.Equal(t, http.StatusCreated, revoke.Code, revoke.Body.String())

	w := agentSecurityE2EPost(r, "/agent-chat/s-chat", agentSecurityE2EChatBody("agent-e2e-rel", "blocked by release revocation"), "rid-chat-release")
	agentSecurityE2EAssertZeroEffects(t, w, db, files)
	require.Zero(t, atomic.LoadInt32(&qa.calls), "a refused turn must never reach the executor")
}

// TestAgentSecurityE2EAgentChatInFlightCancelAndAllow proves the in-flight
// disposition through the durable claim: cancel revocation interrupts the
// active turn and terminalizes its assistant state, allow revocation leaves
// the in-flight turn alone so it completes normally.
func TestAgentSecurityE2EAgentChatInFlightCancelAndAllow(t *testing.T) {
	db := openCraftHTTPDB(t)
	seedAgentSecurityE2ESession(t, db)
	agentCancel, _ := seedAgentSecurityE2ELineage(t, db, "cancel")
	agentAllow, _ := seedAgentSecurityE2ELineage(t, db, "allow")
	qa := &agentSecurityE2EQASessions{repo: repository.NewSessionRepository(db), release: make(chan struct{})}
	files := &agentSecurityE2EFiles{}
	r, _ := newAgentSecurityE2EEngine(t, db, qa, files)

	// --- cancel: the revocation interrupts the in-flight claimed turn.
	cancelCtx, disconnect := context.WithCancel(context.Background())
	cancelDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/agent-chat/s-chat", strings.NewReader(agentSecurityE2EPlainBody(agentCancel.ID, "in flight cancel")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "rid-chat-cancel")
		ctx := context.WithValue(cancelCtx, types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		cancelDone <- w
	}()
	var activeClaim types.AgentChatTurnClaimEntity
	require.Eventually(t, func() bool {
		return db.Where("request_id = ?", "rid-chat-cancel").Take(&activeClaim).Error == nil && activeClaim.State == "active"
	}, 5*time.Second, 50*time.Millisecond, "the in-flight turn must hold an active claim")
	require.NotNil(t, activeClaim.LocalAgentVersionID, "the claim must pin the immutable Version")
	require.Equal(t, "e2e-ver-cancel", *activeClaim.LocalAgentVersionID)
	require.Equal(t, "e2e-rel-cancel", *activeClaim.ReleaseID)
	require.Equal(t, agentCancel.ID, activeClaim.AgentID)

	cancelRevoke := agentSecurityE2EPost(r, "/api/v1/marketplace/tenant/security-revocations/releases",
		`{"release_id":"e2e-rel-cancel","reason":"e2e cancel in flight","in_flight_disposition":"cancel"}`, "rid-revoke-cancel")
	require.Equal(t, http.StatusCreated, cancelRevoke.Code, cancelRevoke.Body.String())

	require.Eventually(t, func() bool {
		var claim types.AgentChatTurnClaimEntity
		if db.Where("id = ?", activeClaim.ID).Take(&claim).Error != nil {
			return false
		}
		return claim.State == "cancelled"
	}, 5*time.Second, 50*time.Millisecond, "cancel disposition must interrupt the active claim")
	var cancelledClaim types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ?", activeClaim.ID).Take(&cancelledClaim).Error)
	require.Equal(t, "cancelled", cancelledClaim.State)
	require.Greater(t, cancelledClaim.Generation, activeClaim.Generation, "cancellation must bump the fencing generation")
	var cancelledAssistant types.Message
	require.NoError(t, db.Where("id = ?", activeClaim.AssistantMessageID).Take(&cancelledAssistant).Error)
	require.True(t, cancelledAssistant.IsCompleted, "cancel must terminalize the assistant placeholder")
	disconnect()
	select {
	case <-cancelDone:
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled turn request did not finish")
	}
	cancelReplay := agentSecurityE2EPost(r, "/agent-chat/s-chat", agentSecurityE2EPlainBody(agentCancel.ID, "in flight cancel"), "rid-chat-cancel")
	require.Equal(t, http.StatusConflict, cancelReplay.Code, cancelReplay.Body.String())
	require.EqualValues(t, 1, atomic.LoadInt32(&qa.calls), "the cancelled turn must have reached the executor exactly once")

	// --- allow: the revocation records the ledger but the in-flight turn
	// completes; its assistant state terminalizes as completed.
	allowDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/agent-chat/s-chat", strings.NewReader(agentSecurityE2EPlainBody(agentAllow.ID, "in flight allow")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "rid-chat-allow")
		ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		allowDone <- w
	}()
	var allowClaim types.AgentChatTurnClaimEntity
	require.Eventually(t, func() bool {
		return db.Where("request_id = ?", "rid-chat-allow").Take(&allowClaim).Error == nil && allowClaim.State == "active"
	}, 5*time.Second, 50*time.Millisecond, "the allowed turn must hold an active claim")

	allowRevoke := agentSecurityE2EPost(r, "/api/v1/marketplace/tenant/security-revocations/releases",
		`{"release_id":"e2e-rel-allow","reason":"e2e allow in flight","in_flight_disposition":"allow"}`, "rid-revoke-allow")
	require.Equal(t, http.StatusCreated, allowRevoke.Code, allowRevoke.Body.String())
	require.NoError(t, db.Where("id = ?", allowClaim.ID).Take(&allowClaim).Error)
	require.Equal(t, "active", allowClaim.State, "allow disposition must not interrupt the in-flight claim")

	close(qa.release)
	var allowResp *httptest.ResponseRecorder
	select {
	case allowResp = <-allowDone:
	case <-time.After(10 * time.Second):
		t.Fatal("allowed turn request did not finish")
	}
	require.Equal(t, http.StatusOK, allowResp.Code, allowResp.Body.String())
	require.Contains(t, allowResp.Header().Get("Content-Type"), "text/event-stream", "an admitted turn streams over SSE")
	require.Eventually(t, func() bool {
		var claim types.AgentChatTurnClaimEntity
		if db.Where("id = ?", allowClaim.ID).Take(&claim).Error != nil {
			return false
		}
		return claim.State == "completed"
	}, 5*time.Second, 50*time.Millisecond, "allow disposition must let the turn complete")
	var allowedAssistant types.Message
	require.NoError(t, db.Where("id = ?", allowClaim.AssistantMessageID).Take(&allowedAssistant).Error)
	require.True(t, allowedAssistant.IsCompleted, "the allowed turn's assistant state must be terminal")
	var allowUserMessages int64
	require.NoError(t, db.Model(&types.Message{}).Where("session_id = ? AND role = ?", "s-chat", "user").Count(&allowUserMessages).Error)
	require.EqualValues(t, 2, allowUserMessages, "both turns' user messages must persist")
}

// TestAgentSecurityE2EAgentChatTurnRefusedForRevokedDependency proves the
// chat entry fails closed on the EXACT locked dependency identity: revoking
// (model, gpt-x, 1.0.0, digest) refuses the turn with zero side effects.
func TestAgentSecurityE2EAgentChatTurnRefusedForRevokedDependency(t *testing.T) {
	db := openCraftHTTPDB(t)
	seedAgentSecurityE2ESession(t, db)
	_, dependency := seedAgentSecurityE2ELineage(t, db, "dep")
	qa := &agentSecurityE2EQASessions{repo: repository.NewSessionRepository(db), release: make(chan struct{})}
	files := &agentSecurityE2EFiles{}
	r, _ := newAgentSecurityE2EEngine(t, db, qa, files)

	body, err := json.Marshal(map[string]any{
		"dependency": map[string]any{"type": dependency.Type, "id": dependency.ID, "version": dependency.Version, "digest": dependency.Digest},
		"reason":     "e2e vulnerable dependency", "in_flight_disposition": "cancel"})
	require.NoError(t, err)
	revoke := agentSecurityE2EPost(r, "/api/v1/marketplace/tenant/security-revocations/dependencies", string(body), "rid-revoke-dep")
	require.Equal(t, http.StatusCreated, revoke.Code, revoke.Body.String())

	w := agentSecurityE2EPost(r, "/agent-chat/s-chat", agentSecurityE2EChatBody("agent-e2e-dep", "blocked by dependency revocation"), "rid-chat-dep")
	agentSecurityE2EAssertZeroEffects(t, w, db, files)
	require.Zero(t, atomic.LoadInt32(&qa.calls), "a refused turn must never reach the executor")
}
