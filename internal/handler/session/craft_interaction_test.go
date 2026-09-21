package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The C02 decide/list HTTP surface, mounted through the real route
// registration over the real interaction store. Only the OpenCode replier is
// a fake — the durable path is the production one.

type craftHTTPReplier struct {
	mu   sync.Mutex
	err  error
	call string
}

func (r *craftHTTPReplier) ReplyQuestion(_ context.Context, requestID string, answers [][]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.call = fmt.Sprintf("reply:%s:%v", requestID, answers)
	return r.err
}

func (r *craftHTTPReplier) RejectQuestion(_ context.Context, requestID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.call = "reject:" + requestID
	return r.err
}

func (r *craftHTTPReplier) ReplyPermission(_ context.Context, _, requestID, reply string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.call = "permission:" + requestID + ":" + reply
	return r.err
}

type craftInteractionEnv struct {
	engine  *gin.Engine
	svc     *service.CraftControlService
	store   *service.GormCraftInteractionStore
	db      *gorm.DB
	replier *craftHTTPReplier
}

func newCraftInteractionEnv(t *testing.T) *craftInteractionEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	store := service.NewGormCraftInteractionStore(db)
	replier := &craftHTTPReplier{}
	runs := service.NewAgentRunService(repository.NewAgentRunStore(db))
	svc := service.NewCraftControlService(runs, repository.NewCraftStore(db), nil, store, replier)
	env := &craftInteractionEnv{engine: gin.New(), svc: svc, store: store, db: db, replier: replier}
	env.engine.Use(middleware.ErrorHandler())
	env.engine.Use(func(c *gin.Context) {
		identity := c.GetHeader("X-Test-Identity")
		tenant := uint64(1)
		user := "u1"
		if identity == "other" {
			user = "u2"
		}
		c.Set(types.TenantIDContextKey.String(), tenant)
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	RegisterCraftInteractionRoutes(env.engine.Group("/api/v1/sessions"), NewCraftInteractionHandler(svc))
	return env
}

func (e *craftInteractionEnv) do(t *testing.T, method, path, identity, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if identity != "" {
		req.Header.Set("X-Test-Identity", identity)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// seedInteractionHTTP registers one pending multi-question interaction on a
// live parked run.
func (e *craftInteractionEnv) seedInteractionHTTP(t *testing.T, id string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, e.db.Exec(
		"INSERT OR IGNORE INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 's', 'u1', 'trpc')").Error)
	runs := repository.NewAgentRunStore(e.db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "run-http"}
	_, err := runs.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "req-http",
		AssistantMessageID: "asst-http", RequestHash: "rh-http",
		Snapshot:         json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage:      json.RawMessage(`{"role":"user","content":"x"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	pending := craft.PendingDecision{
		Interaction: craft.Interaction{ID: id, Kind: craft.InteractionQuestion, ArgsHash: "iargs_http", Prompt: "which?"},
		Options:     map[string][]string{"q1": {"a", "b"}, "q2": {"x", "y", "z"}},
		Multiple:    map[string]bool{"q2": true},
	}
	_, err = e.store.PutInteraction(ctx, service.CraftInteractionRecord{
		Interaction:       pending.Interaction,
		Scope:             craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
		RunID:             "run-http",
		PendingID:         id,
		OpenCodeSessionID: "oc-http",
		OpenCodeRequestID: "qst-http",
		Pending:           pending,
	})
	require.NoError(t, err)
}

func decideBody(decisionID string, answers string) string {
	return fmt.Sprintf(`{"decision_id":%q,"action":"answer","args_hash":"iargs_http","expected_revision":1,"answers":%s}`,
		decisionID, answers)
}

// TestCraftInteractionListShowsPendingPayload: the list endpoint exposes the
// full pending decision — kind, prompt, per-question options and multi flags.
func TestCraftInteractionListShowsPendingPayload(t *testing.T) {
	env := newCraftInteractionEnv(t)
	env.seedInteractionHTTP(t, "itx_http_1")

	w := env.do(t, http.MethodGet, "/api/v1/sessions/s1/craft/interactions", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Pending struct {
				Options  map[string][]string `json:"options"`
				Multiple map[string]bool     `json:"multiple"`
			} `json:"pending"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	require.Equal(t, "itx_http_1", body.Data[0].ID)
	require.Equal(t, "pending", body.Data[0].Status)
	require.Equal(t, []string{"a", "b"}, body.Data[0].Pending.Options["q1"])
	require.True(t, body.Data[0].Pending.Multiple["q2"])
}

// TestCraftInteractionDecideAnswers202AndDelivers: a legal multi-question
// answer is accepted (202 — the delivery may still be unconfirmed), forwarded
// over the locked protocol, and the recorded answers are re-readable.
func TestCraftInteractionDecideAnswers202AndDelivers(t *testing.T) {
	env := newCraftInteractionEnv(t)
	env.seedInteractionHTTP(t, "itx_http_2")

	w := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_2/decide", "",
		decideBody("dec_http_1", `[{"question_id":"q1","choices":["a"]},{"question_id":"q2","choices":["x","z"],"text":"note"}]`))
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	var outcome struct {
		Data struct {
			Decided   bool   `json:"decided"`
			Delivered bool   `json:"delivered"`
			Delivery  string `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &outcome))
	require.True(t, outcome.Data.Decided)
	require.True(t, outcome.Data.Delivered)
	require.Equal(t, "delivered", outcome.Data.Delivery)
	require.Equal(t, "reply:qst-http:[[a] [x z note]]", env.replier.call)

	listed := env.do(t, http.MethodGet, "/api/v1/sessions/s1/craft/interactions", "", "")
	require.Contains(t, listed.Body.String(), `"answers"`)
}

// TestCraftInteractionDecideConflictGoneAndInvalid: a replay with a different
// payload answers 409, an already-decided interaction answers 410, an illegal
// choice answers 400, and approving a question answers 400 — a question
// answer can never become a permission approval.
func TestCraftInteractionDecideConflictGoneAndInvalid(t *testing.T) {
	env := newCraftInteractionEnv(t)
	env.seedInteractionHTTP(t, "itx_http_3")

	illegal := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_3/decide", "",
		decideBody("dec_bad", `[{"question_id":"q1","choices":["nope"]}]`))
	require.Equal(t, http.StatusBadRequest, illegal.Code)

	approve := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_3/decide", "",
		`{"decision_id":"dec_ap","action":"approve","args_hash":"iargs_http","expected_revision":1}`)
	require.Equal(t, http.StatusBadRequest, approve.Code)

	ok := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_3/decide", "",
		decideBody("dec_ok", `[{"question_id":"q1","choices":["b"]}]`))
	require.Equal(t, http.StatusAccepted, ok.Code)

	conflict := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_3/decide", "",
		decideBody("dec_ok", `[{"question_id":"q1","choices":["a"]}]`))
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())

	gone := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_3/decide", "",
		decideBody("dec_other", `[{"question_id":"q1","choices":["b"]}]`))
	require.Equal(t, http.StatusGone, gone.Code, gone.Body.String())
}

// TestCraftInteractionDecideUnknownDeliveryStays202: when the runtime cannot
// confirm the forward, the answer stays 202 with an honest delivery_unknown
// note — the decision is recorded but the delivery is unconfirmed.
func TestCraftInteractionDecideUnknownDeliveryStays202(t *testing.T) {
	env := newCraftInteractionEnv(t)
	env.seedInteractionHTTP(t, "itx_http_4")
	env.replier.err = context.DeadlineExceeded

	w := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_4/decide", "",
		decideBody("dec_http_4", `[{"question_id":"q1","choices":["a"]}]`))
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "unconfirmed")
	require.Contains(t, w.Body.String(), "unknown")
}

// TestCraftInteractionDecideScopeAndMissing: another user is forbidden (403),
// an unknown interaction is 404, and a nil handler leaves the surface
// unmounted (fail-closed).
func TestCraftInteractionDecideScopeAndMissing(t *testing.T) {
	env := newCraftInteractionEnv(t)
	env.seedInteractionHTTP(t, "itx_http_5")

	other := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_http_5/decide", "other",
		decideBody("dec_x", `[{"question_id":"q1","choices":["a"]}]`))
	require.Equal(t, http.StatusForbidden, other.Code)

	missing := env.do(t, http.MethodPost, "/api/v1/sessions/s1/craft/interactions/itx_nope/decide", "",
		decideBody("dec_x", `[{"question_id":"q1","choices":["a"]}]`))
	require.Equal(t, http.StatusNotFound, missing.Code)
}
