package session

// T34 #64 Task 8D: AgentQA durable turn claim handler wiring. The claim is
// admitted after authorized agent resolution but BEFORE any attachment
// persistence, message row, or SSE byte; Stop cancels it through
// CancelByOwner; every message write is fenced by claim generation.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openAgentClaimTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "agent-claim.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedAgentClaimSession(t *testing.T, db *gorm.DB) *types.Session {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'claim-t1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'claim session', 'u1', 'builtin')`).Error)
	var session types.Session
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), "s1").Take(&session).Error)
	return &session
}

func seedClaimPlainAgent(t *testing.T, db *gorm.DB) *types.CustomAgent {
	t.Helper()
	agent := &types.CustomAgent{
		ID: "agent-plain", Name: "Plain", TenantID: 1, CreatedBy: "u1",
		Config: types.CustomAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}
	require.NoError(t, db.Create(agent).Error)
	return agent
}

// seedClaimAdoptedRevokedAgent plants the adopted-Agent lineage (published
// Variant pinned to an immutable Version + Release) plus an append-only
// release revocation: admission must evaluate to ErrAgentSecurityReleaseBlocked.
// Raw SQL keeps the FK chain (listing → submission → release → adoption →
// variant) minimal while staying inside the real migrated schema.
func seedClaimAdoptedRevokedAgent(t *testing.T, db *gorm.DB) *types.CustomAgent {
	t.Helper()
	agent := &types.CustomAgent{
		ID: "agent-adopted", Name: "Adopted", TenantID: 1, CreatedBy: "u1",
		Config: types.CustomAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}
	require.NoError(t, db.Create(agent).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&types.AgentVersionEntity{
		ID: "ver-1", TenantID: 1, AgentID: "agent-adopted", VersionNumber: 1,
		Snapshot: "{}", SourceSHA256: strings.Repeat("0", 64), FrozenBy: "u1", CreatedAt: now,
	}).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_marketplace_listings (id, tenant_id, source_agent_id, display_name, state)
		VALUES ('ls-1', 1, 'agent-adopted', 'Adopted listing', 'listed')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_release_submissions (id, tenant_id, listing_id, agent_version_id, source_agent_id, author_id, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, status)
		VALUES ('sub-1', 1, 'ls-1', 'ver-1', 'agent-adopted', 'publisher', '1.0.0', 'digest-1', '{}', '{"dependencies":[]}', X'7B7D', 'approved')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_releases (id, tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, published_by)
		VALUES ('rel-1', 1, 'ls-1', 'sub-1', 'ver-1', 'agent-adopted', 1, '1.0.0', 'digest-1', '{}', '{"dependencies":[]}', X'7B7D', 'publisher')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by)
		VALUES ('ad-1', 1, 'ls-1', 'rel-1', 'active', 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoption_variants (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, published_by, published_at)
		VALUES ('var-1', 1, 'ad-1', 'rel-1', 'v1', 'published', 'agent-adopted', 'ver-1', 'u1', ?)`, now).Error)
	require.NoError(t, db.Create(&types.AgentReleaseRevocationEntity{
		ID: "rev-1", TenantID: 1, ListingID: "ls-1", ReleaseID: "rel-1",
		Reason: "security test revocation", InFlightDisposition: "cancel",
		RunCancellationState: "complete", RevokedBy: "admin", RevokedAt: now, CreatedAt: now,
	}).Error)
	return agent
}

// agentClaimQASessions keeps the real owner-scoped session read but blocks the
// AgentQA service call until the test releases it, so the first request holds
// an active claim while the replay arrives.
type agentClaimQASessions struct {
	interfaces.SessionService
	repo    interfaces.SessionRepository
	release chan struct{}
	calls   int32
}

func (s *agentClaimQASessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	userID, _ := types.UserIDFromContext(ctx)
	return s.repo.Get(ctx, tenantID, userID, id)
}

func (s *agentClaimQASessions) AgentQA(context.Context, *types.QARequest, *event.EventBus) error {
	atomic.AddInt32(&s.calls, 1)
	<-s.release
	return nil
}

func (s *agentClaimQASessions) UpdateSessionLastRequestState(context.Context, string, *types.SessionLastRequestState) error {
	return nil
}

// claimNoopMessages satisfies the completion path's message-service calls
// (fenced writes go through the claim store, not here).
type claimNoopMessages struct {
	interfaces.MessageService
}

func (s *claimNoopMessages) UpdateMessage(context.Context, *types.Message) error { return nil }
func (s *claimNoopMessages) IndexMessageToKB(context.Context, string, string, string, string) {
}

// claimReplayStream serves no events until the test releases the first turn,
// then one terminal "complete" event so the SSE loop finishes.
type claimReplayStream struct {
	stubStreamManager
	release chan struct{}
}

func (s *claimReplayStream) GetEvents(_ context.Context, _, _ string, fromOffset int) ([]interfaces.StreamEvent, int, error) {
	select {
	case <-s.release:
	default:
		return nil, fromOffset, nil
	}
	if fromOffset > 0 {
		return nil, fromOffset, nil
	}
	return []interfaces.StreamEvent{{ID: "done-1", Type: types.ResponseTypeComplete, Done: true}}, 1, nil
}

func newAgentClaimRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	r.POST("/agent-chat/:session_id", h.AgentQA)
	r.POST("/sessions/:session_id/stop", h.StopSession)
	return r
}

func postAgentClaim(r *gin.Engine, path, body, requestID string) *httptest.ResponseRecorder {
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

func TestAgentQAClaimAdmittedBeforeAttachmentPersistence(t *testing.T) {
	db := openAgentClaimTestDB(t)
	seedAgentClaimSession(t, db)
	agent := seedClaimAdoptedRevokedAgent(t, db)
	h := &Handler{
		sessionService:          &runGateSessions{repo: repository.NewSessionRepository(db)},
		customAgentService:      &resolveOwnAgentStub{agent: agent},
		agentChatTurnClaimStore: repository.NewAgentChatTurnClaimRepository(db),
	}
	r := newAgentClaimRouter(h)

	w := postAgentClaim(r, "/agent-chat/s1",
		`{"query":"blocked turn","agent_id":"agent-adopted","agent_enabled":true,"images":[{"data":"data:image/png;base64,aGk="}]}`,
		"rid-blocked-1")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var messages, claims int64
	require.NoError(t, db.Model(&types.Message{}).Count(&messages).Error)
	require.NoError(t, db.Model(&types.AgentChatTurnClaimEntity{}).Count(&claims).Error)
	require.EqualValues(t, 0, messages, "blocked admission must not persist any message row")
	require.EqualValues(t, 0, claims, "blocked admission must not persist any claim row")
}

func TestAgentQAClaimReplayActiveReturnsConflictWithAssistantID(t *testing.T) {
	db := openAgentClaimTestDB(t)
	seedAgentClaimSession(t, db)
	agent := seedClaimPlainAgent(t, db)
	store := repository.NewAgentChatTurnClaimRepository(db)
	release := make(chan struct{})
	h := &Handler{
		sessionService:          &agentClaimQASessions{repo: repository.NewSessionRepository(db), release: release},
		customAgentService:      &resolveOwnAgentStub{agent: agent},
		streamManager:           &claimReplayStream{release: release},
		messageService:          &claimNoopMessages{},
		agentChatTurnClaimStore: store,
	}
	r := newAgentClaimRouter(h)
	body := `{"query":"replay me","agent_id":"agent-plain","agent_enabled":true}`

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- postAgentClaim(r, "/agent-chat/s1", body, "rid-replay-1") }()

	require.Eventually(t, func() bool {
		var active int64
		db.Model(&types.AgentChatTurnClaimEntity{}).Where("state = 'active'").Count(&active)
		return active == 1
	}, 5*time.Second, 50*time.Millisecond, "first request must admit an active claim")

	second := postAgentClaim(r, "/agent-chat/s1", body, "rid-replay-1")
	require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	require.NotContains(t, second.Header().Get("Content-Type"), "text/event-stream", "replay must not open a second SSE stream")
	var conflictBody struct {
		Error struct {
			Details struct {
				AssistantMessageID string `json:"assistant_message_id"`
				State              string `json:"state"`
			} `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &conflictBody))
	require.NotEmpty(t, conflictBody.Error.Details.AssistantMessageID)
	require.Equal(t, "active", conflictBody.Error.Details.State)

	close(release)
	select {
	case <-firstDone:
	case <-time.After(10 * time.Second):
		t.Fatal("first request did not complete")
	}
	require.EqualValues(t, 1, atomic.LoadInt32(&h.sessionService.(*agentClaimQASessions).calls),
		"replayed request must not execute a second turn")

	var claims []types.AgentChatTurnClaimEntity
	require.NoError(t, db.Find(&claims).Error)
	require.Len(t, claims, 1)
	var userMessages int64
	require.NoError(t, db.Model(&types.Message{}).Where("session_id = ? AND role = 'user'", "s1").Count(&userMessages).Error)
	require.EqualValues(t, 1, userMessages, "replayed request must not create a second user message")
}

func TestAgentQAStopCancelsClaimByOwner(t *testing.T) {
	db := openAgentClaimTestDB(t)
	seedAgentClaimSession(t, db)
	seedClaimPlainAgent(t, db)
	store := repository.NewAgentChatTurnClaimRepository(db)
	claim, replay, err := store.Admit(context.Background(), repository.AgentChatTurnClaimInput{
		SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1",
		RequestID: "rid-stop-1", RequestHash: strings.Repeat("a", 64),
		LeaseOwner: "lease-stop", AgentID: "agent-plain",
		AssistantPlaceholder: &types.Message{Role: "assistant"},
	})
	require.NoError(t, err)
	require.Equal(t, repository.ReplayNew, replay)

	h := &Handler{
		sessionService:          &runGateSessions{repo: repository.NewSessionRepository(db)},
		messageService:          &stubMessageServiceForStream{},
		streamManager:           &stubStreamManager{},
		agentChatTurnClaimStore: store,
	}
	r := newAgentClaimRouter(h)

	w := postAgentClaim(r, "/sessions/s1/stop",
		`{"message_id":"`+claim.AssistantMessageID+`"}`, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var cancelled types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ?", claim.ID).Take(&cancelled).Error)
	require.Equal(t, "cancelled", cancelled.State)
	require.Equal(t, uint64(2), cancelled.Generation, "cancellation must bump the fencing generation")
	var placeholder types.Message
	require.NoError(t, db.Where("id = ?", claim.AssistantMessageID).Take(&placeholder).Error)
	require.True(t, placeholder.IsCompleted, "stop must terminalize the assistant placeholder")
}

func TestAgentQAFencedWriteRejectedOnGenerationChange(t *testing.T) {
	db := openAgentClaimTestDB(t)
	session := seedAgentClaimSession(t, db)
	seedClaimPlainAgent(t, db)
	store := repository.NewAgentChatTurnClaimRepository(db)
	claim, replay, err := store.Admit(context.Background(), repository.AgentChatTurnClaimInput{
		SourceTenantID: 1, SessionTenantID: 1, SessionID: "s1", OwnerID: "u1",
		RequestID: "rid-fence-1", RequestHash: strings.Repeat("b", 64),
		LeaseOwner: "lease-fence", AgentID: "agent-plain",
		AssistantPlaceholder: &types.Message{Role: "assistant"},
	})
	require.NoError(t, err)
	require.Equal(t, repository.ReplayNew, replay)

	h := &Handler{agentChatTurnClaimStore: store}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	newReqCtx := func() *qaRequestContext {
		return &qaRequestContext{
			ctx: ctx, sessionID: "s1", requestID: claim.RequestID, query: "fenced",
			session: session, claim: &claim,
			assistantMessage: &types.Message{ID: claim.AssistantMessageID, SessionID: "s1", Role: "assistant"},
		}
	}

	require.NoError(t, h.persistTurnMessages(ctx, newReqCtx()))
	var fenced types.AgentChatTurnClaimEntity
	require.NoError(t, db.Where("id = ?", claim.ID).Take(&fenced).Error)
	require.NotNil(t, fenced.UserMessageID, "fenced write must record the user message on the claim")

	_, changed, err := store.CancelByOwner(ctx, 1, "s1", "u1", claim.AssistantMessageID, "user stop")
	require.NoError(t, err)
	require.True(t, changed, "cancelling the active claim must transition it")

	require.Error(t, h.persistTurnMessages(ctx, newReqCtx()),
		"a stale generation snapshot must not create another user message")
	err = store.UpdateMessage(ctx, claim.SourceTenantID, claim.ID, claim.Generation, claim.LeaseOwner,
		&types.Message{ID: claim.AssistantMessageID, SessionID: "s1", RequestID: claim.RequestID, Role: "assistant", Content: "tampered"})
	require.Error(t, err, "a stale generation snapshot must not update the assistant message")

	var userMessages int64
	require.NoError(t, db.Model(&types.Message{}).Where("session_id = ? AND role = 'user'", "s1").Count(&userMessages).Error)
	require.EqualValues(t, 1, userMessages)
	var placeholder types.Message
	require.NoError(t, db.Where("id = ?", claim.AssistantMessageID).Take(&placeholder).Error)
	require.Empty(t, placeholder.Content, "fenced-off write must leave the assistant message unchanged")
}
