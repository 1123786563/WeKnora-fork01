package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"

	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type approvalHTTPChecker struct{}

func (approvalHTTPChecker) IsRequired(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}

func (approvalHTTPChecker) IsEnabled(context.Context, uint64, string, string) (bool, error) {
	return true, nil
}

type commandStore struct {
	current workbench.InteractionDecision
	decided workbench.InteractionDecision
}

func (s *commandStore) List(context.Context, uint64, string, string) ([]workbench.InteractionDecision, error) {
	return []workbench.InteractionDecision{s.current}, nil
}

func (s *commandStore) Get(context.Context, uint64, string, string) (workbench.InteractionDecision, error) {
	return s.current, nil
}

func (s *commandStore) Decide(_ context.Context, _ uint64, _ string, _ string, input workbench.InteractionDecision) (workbench.InteractionDecision, error) {
	s.decided = input
	return input, nil
}

func commandRouter(h *WorkbenchCommandHandler) *gin.Engine {
	return commandRouterForIdentity(h, 7, types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
}

func commandRouterForIdentity(h *WorkbenchCommandHandler, tenant uint64, principal types.Principal) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/interactions/:id/decisions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), tenant)
		c.Set(types.UserIDContextKey.String(), principal.ID)
		c.Set(types.PrincipalContextKey.String(), principal)
		h.DecideInteraction(c)
	})
	return r
}

func TestWorkbenchCommandDecisionUsesStoredKindAndArgsHash(t *testing.T) {
	store := &commandStore{current: workbench.InteractionDecision{ID: "i1", Kind: "budget", ArgsHash: "hash"}}
	h := NewWorkbenchCommandHandler(workbenchservice.NewInteractionService(store, nil, nil))
	r := commandRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/interactions/i1/decisions", stringsReader(`{"kind":"tool_approval","action":"approve","args_hash":"hash","decision_id":"d1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	req = httptest.NewRequest(http.MethodPost, "/interactions/i1/decisions", stringsReader(`{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"d1"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "budget", store.decided.Kind)
}

func TestWorkbenchDecisionHTTPUsesGormCASAndIdempotency(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_http_cas?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE workbench_interactions (tenant_id INTEGER, id TEXT, run_id TEXT, owner_id TEXT, kind TEXT, args_hash TEXT, decision_id TEXT DEFAULT '', action TEXT DEFAULT '', status TEXT DEFAULT 'pending', expected_revision INTEGER DEFAULT 0, expires_at DATETIME, revoked BOOLEAN DEFAULT 0, created_at DATETIME, updated_at DATETIME, PRIMARY KEY (tenant_id,id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO workbench_interactions (tenant_id,id,run_id,owner_id,kind,args_hash,status,expected_revision) VALUES (7,'i1','run-1','web_user:u1','budget','hash','pending',0)`).Error)
	service := workbenchservice.NewInteractionService(workbenchservice.NewGormInteractionStore(db), nil, nil)
	h := NewWorkbenchCommandHandler(service)
	r := commandRouter(h)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/interactions/i1/decisions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusOK, post(`{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"d1","expected_revision":0}`).Code)
	require.Equal(t, http.StatusOK, post(`{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"d1","expected_revision":0}`).Code)
	require.Equal(t, http.StatusConflict, post(`{"kind":"budget","action":"extend","args_hash":"hash","decision_id":"d2","expected_revision":0}`).Code)
	var status string
	require.NoError(t, db.Raw("SELECT status FROM workbench_interactions WHERE id = ?", "i1").Scan(&status).Error)
	require.Equal(t, "resolved", status)
}

func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }

func TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w05_http_approval?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workbenchserviceInteractionRow{}))
	args := []byte(`{"x":1}`)
	hash := sha256.Sum256(args)
	store := workbenchservice.NewGormInteractionStore(db)
	gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 3}}, approvalHTTPChecker{}, nil)
	service := workbenchservice.NewInteractionServiceWithApproval(store, nil, nil, gate)
	h := NewWorkbenchCommandHandler(service)
	r := commandRouter(h)
	bus := event.NewEventBus()
	pending := make(chan string, 1)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
		pending <- evt.Data.(event.ToolApprovalRequiredData).PendingID
		return nil
	})
	result := make(chan struct {
		decision approval.Decision
		err      error
	}, 1)
	approvalCtx := types.WithPrincipal(context.Background(), types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
	go func() {
		decision, waitErr := gate.RequestAndWait(approvalCtx, approval.PendingRequest{TenantID: 7, CredentialVersion: 1, UserID: "u1", RunID: "run-1", RequestID: "request-1", EventBus: bus, Args: args})
		result <- struct {
			decision approval.Decision
			err      error
		}{decision: decision, err: waitErr}
	}()
	id := <-pending
	var row struct {
		TenantID                                                       uint64
		ID, RunID, OwnerID, Kind, ArgsHash, Status, DecisionID, Action string
		ExpectedRevision                                               int64
		ExpiresAt                                                      *time.Time
		Revoked                                                        bool
	}
	require.NoError(t, db.Table("workbench_interactions").Where("id = ?", id).Take(&row).Error)
	require.Equal(t, "run-1", row.RunID)
	require.Equal(t, "web_user:u1", row.OwnerID)
	require.Equal(t, hex.EncodeToString(hash[:]), row.ArgsHash)
	post := func(tenant uint64, owner, action, decisionID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/interactions/"+id+"/decisions", strings.NewReader(`{"kind":"tool_approval","action":"`+action+`","args_hash":"`+row.ArgsHash+`","decision_id":"`+decisionID+`","expected_revision":0}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		// commandRouter fixes the authenticated scope to tenant 7/u1. This
		// request proves the normal same-scope HTTP decision; isolation is
		// covered by the service's tenant/owner predicate below.
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusOK, post(7, "u1", "approve", "d1").Code)
	approvalResult := <-result
	require.NoError(t, approvalResult.err)
	require.True(t, approvalResult.decision.Approved)
	var status string
	require.NoError(t, db.Table("workbench_interactions").Select("status").Where("id = ?", id).Scan(&status).Error)
	require.Equal(t, "resolved", status)
	other := commandRouterForIdentity(h, 7, types.Principal{Type: types.PrincipalWebUser, ID: "u2"})
	otherReq := httptest.NewRequest(http.MethodPost, "/interactions/"+id+"/decisions", strings.NewReader(`{"kind":"tool_approval","action":"approve","args_hash":"`+row.ArgsHash+`","decision_id":"d2","expected_revision":0}`))
	otherReq.Header.Set("Content-Type", "application/json")
	otherResp := httptest.NewRecorder()
	other.ServeHTTP(otherResp, otherReq)
	require.Equal(t, http.StatusNotFound, otherResp.Code)
	_, err = store.Get(context.Background(), 8, "u1", id)
	require.ErrorIs(t, err, workbenchservice.ErrInteractionNotFound)
}

// Keep the migration shape local to this integration test without importing a
// production model that is intentionally private to the service package.
type workbenchserviceInteractionRow struct {
	TenantID                                                       uint64
	ID, RunID, OwnerID, Kind, ArgsHash, DecisionID, Action, Status string
	ExternalPendingID                                              string
	CredentialVersion                                              int64
	ExpectedRevision                                               int64
	ExpiresAt                                                      *time.Time
	Revoked                                                        bool
	CreatedAt, UpdatedAt                                           time.Time
}

func (workbenchserviceInteractionRow) TableName() string { return "workbench_interactions" }
