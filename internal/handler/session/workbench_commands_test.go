package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/interactions/:id/decisions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "u1")
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

func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }
