package session

// T20 (#139) deferred item (review #8): the budget-pause HTTP surface. A
// budget-exhausted Run parks durably (status waiting_user, wait_reason
// budget_exhausted); the workbench must be able to READ that pause and the
// Task owner / tenant billing admin must be able to EXTEND and safely resume
// through the authenticated sessions tree — exactly the seams the pause wire
// ({run_id, reason, limit, used, extension_action}) and the service's
// ExtendAndResume contract define. The journey below runs the real service
// over the migrated HTTP database: pause projection, member-visible
// refusal, owner extension and the unknown-reconcile refusal.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t20BudgetChecker allows the session owner and one viewer; a same-tenant
// non-member is refused at the Task gate (403) and a FOREIGN tenant maps to
// ErrNotFound — the T08 invisibility discipline (a foreigner never learns
// the Task exists).
type t20BudgetChecker struct{}

func (t20BudgetChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, _ craft.TaskAction) error {
	if scope.TenantID != 1 {
		return craft.ErrNotFound
	}
	if scope.UserID == "u1" || scope.UserID == "u2" {
		return nil
	}
	return craft.ErrForbidden
}

// t20LazyPauseAPI mirrors production ordering: the pause API is REGISTERED
// before route assembly (container Invoke → router), while the concrete
// budget service is built over the env database right after. The forwarder
// fails closed until the inner service lands.
type t20LazyPauseAPI struct{ inner CraftBudgetPauseAPI }

func (l *t20LazyPauseAPI) BudgetPause(ctx context.Context, scope craft.Scope, runID string) (craft.BudgetPause, error) {
	if l.inner == nil {
		return craft.BudgetPause{}, craft.ErrNotFound
	}
	return l.inner.BudgetPause(ctx, scope, runID)
}

func (l *t20LazyPauseAPI) ExtendAndResume(ctx context.Context, scope craft.Scope, runID, key string, extraCalls int, extraCredits commercial.Credits) error {
	if l.inner == nil {
		return craft.ErrNotFound
	}
	return l.inner.ExtendAndResume(ctx, scope, runID, key, extraCalls, extraCredits)
}

func (l *t20LazyPauseAPI) MayExtendBudget(ctx context.Context, scope craft.Scope) error {
	if l.inner == nil {
		return craft.ErrForbidden
	}
	return l.inner.MayExtendBudget(ctx, scope)
}

func TestCraftT20BudgetPauseHTTPJourney(t *testing.T) {
	lazy := &t20LazyPauseAPI{}
	RegisterCraftBudgetPauseHandler(lazy, t20BudgetChecker{})
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-t20-budget", "region sales page", "web")
	sessionID := created["session_id"].(string)

	// The commercial budget tables back Admit/AuthorizeBinding/Extend; the
	// migrated HTTP database plus this explicit AutoMigrate keeps the journey
	// independent of migration-chain ordering for these rows.
	require.NoError(t, env.db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
	))
	end := time.Now().UTC().Add(2 * time.Hour)
	require.NoError(t, env.db.Create(&repocommercial.BudgetAccountRow{
		TenantID: 1, VerifiedMicro: 100000, Watermark: "w0", Version: 1, VerifiedUntil: end,
	}).Error)
	require.NoError(t, env.db.Create(&repocommercial.BudgetLotRow{
		TenantID: 1, LotID: "lot-t20-budget", RemainingMicro: 100000, IssuedAt: time.Now().UTC(),
	}).Error)

	policy := service.CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 1,
		CallUpper: commercial.Credits(500), TaskLimit: commercial.Credits(5000),
	}
	budget, err := service.NewCraftBudgetService(env.db, nil, policy)
	require.NoError(t, err)
	// The central assembly registers the pause API with the Task read gate;
	// a nil checker fails closed (RequireTaskAccess → 403).
	lazy.inner = budget

	const runID = "run-t20-budget"
	now := time.Now().UTC()
	require.NoError(t, env.db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (1, ?, ?, 'u1', 'req-t20-budget', 'am-t20-budget', 'rh', '{}', 'running', '', ?, ?, ?)`,
		runID, sessionID, now.Add(time.Hour), now, now).Error)

	// Exhaust the one-call grant: the second authorization is denied and the
	// durable pause is committed (waiting_user / budget_exhausted).
	ctx := context.Background()
	ownerScope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	grant, err := budget.Admit(ctx, ownerScope, runID)
	require.NoError(t, err)
	_, err = budget.AuthorizeBinding(ctx, grant.ID, service.CraftCallBinding{ModelID: "m-chat", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	_, err = budget.AuthorizeBinding(ctx, grant.ID, service.CraftCallBinding{ModelID: "m-chat", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted)

	pausePath := "/api/v1/sessions/" + sessionID + "/craft/runs/" + runID + "/budget/pause"
	extendPath := "/api/v1/sessions/" + sessionID + "/craft/runs/" + runID + "/budget/extend"

	// The owner reads the frozen pause wire plus the server-projected
	// extension authority (can_extend).
	ownerPause := env.do(t, http.MethodGet, pausePath, "", "")
	require.Equal(t, http.StatusOK, ownerPause.Code, ownerPause.Body.String())
	var ownerView struct {
		Data struct {
			RunID           string `json:"run_id"`
			Reason          string `json:"reason"`
			Limit           int64  `json:"limit"`
			Used            int64  `json:"used"`
			ExtensionAction *struct {
				Key          string `json:"key"`
				ExtraCalls   int    `json:"extra_calls"`
				ExtraCredits int64  `json:"extra_credits"`
			} `json:"extension_action"`
		} `json:"data"`
		CanExtend bool `json:"can_extend"`
	}
	require.NoError(t, json.Unmarshal(ownerPause.Body.Bytes(), &ownerView))
	require.Equal(t, runID, ownerView.Data.RunID)
	require.Equal(t, "exhausted", ownerView.Data.Reason)
	require.Equal(t, int64(1), ownerView.Data.Limit)
	require.Equal(t, int64(1), ownerView.Data.Used)
	require.True(t, ownerView.CanExtend, "the session owner may extend")
	require.NotNil(t, ownerView.Data.ExtensionAction)
	require.NotEmpty(t, ownerView.Data.ExtensionAction.Key)
	require.Equal(t, 10, ownerView.Data.ExtensionAction.ExtraCalls)
	require.Equal(t, int64(policy.TaskLimit), ownerView.Data.ExtensionAction.ExtraCredits)
	extensionBody, err := json.Marshal(ownerView.Data.ExtensionAction)
	require.NoError(t, err)

	// A Task viewer can read the pause fact but cannot extend the budget.
	viewerPause := env.do(t, http.MethodGet, pausePath, "viewer", "")
	require.Equal(t, http.StatusOK, viewerPause.Code, viewerPause.Body.String())
	var viewerView struct {
		Data struct {
			RunID           string `json:"run_id"`
			Reason          string `json:"reason"`
			Limit           int64  `json:"limit"`
			Used            int64  `json:"used"`
			ExtensionAction *struct {
				Key          string `json:"key"`
				ExtraCalls   int    `json:"extra_calls"`
				ExtraCredits int64  `json:"extra_credits"`
			} `json:"extension_action"`
		} `json:"data"`
		CanExtend bool `json:"can_extend"`
	}
	require.NoError(t, json.Unmarshal(viewerPause.Body.Bytes(), &viewerView))
	require.Equal(t, runID, viewerView.Data.RunID)
	require.Equal(t, "exhausted", viewerView.Data.Reason)
	require.Equal(t, int64(1), viewerView.Data.Limit)
	require.Equal(t, int64(1), viewerView.Data.Used)
	require.False(t, viewerView.CanExtend, "a Task viewer cannot extend the budget")
	require.Nil(t, viewerView.Data.ExtensionAction, "non-authorized readers do not receive an actionable intent")

	// A same-tenant non-member is refused at the Task gate, and an unknown
	// Run answers 404 without leaking the pause.
	require.Equal(t, http.StatusForbidden, env.do(t, http.MethodGet, pausePath, "nonmember", "").Code)
	require.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet,
		"/api/v1/sessions/"+sessionID+"/craft/runs/run-never/budget/pause", "", "").Code)

	// The viewer's extension attempt is denied by the server's own actor
	// check (403), not by hiding the entrance.
	viewerExtend := env.do(t, http.MethodPost, extendPath, "viewer",
		string(extensionBody))
	require.Equal(t, http.StatusForbidden, viewerExtend.Code, viewerExtend.Body.String())

	// While the dispatched reservation is still unsettled, the owner's
	// extension is refused with the reconcile-pending conflict: an unknown
	// dispatched effect is never blindly replayed on resume.
	ownerPending := env.do(t, http.MethodPost, extendPath, "",
		string(extensionBody))
	require.Equal(t, http.StatusConflict, ownerPending.Code, ownerPending.Body.String())
	refetchedPause := env.do(t, http.MethodGet, pausePath, "", "")
	require.Equal(t, http.StatusOK, refetchedPause.Code, refetchedPause.Body.String())
	var refetchedView struct {
		Data struct {
			ExtensionAction *struct {
				Key          string `json:"key"`
				ExtraCalls   int    `json:"extra_calls"`
				ExtraCredits int64  `json:"extra_credits"`
			} `json:"extension_action"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(refetchedPause.Body.Bytes(), &refetchedView))
	require.Equal(t, ownerView.Data.ExtensionAction, refetchedView.Data.ExtensionAction)
	extensionBody, err = json.Marshal(refetchedView.Data.ExtensionAction)
	require.NoError(t, err)

	// Settle the reservation: the same owner request now extends once and
	// resumes the Run durably (status recovering, wait reason cleared).
	require.NoError(t, env.db.Model(&repocommercial.ReservationRow{}).
		Where("tenant_id = ?", uint64(1)).
		Update("state", commercial.ReservationStateSettled).Error)
	ownerExtend := env.do(t, http.MethodPost, extendPath, "",
		string(extensionBody))
	require.Equal(t, http.StatusOK, ownerExtend.Code, ownerExtend.Body.String())
	var run struct {
		Status     string
		WaitReason string
	}
	require.NoError(t, env.db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", uint64(1), runID).Take(&run).Error)
	require.Equal(t, "recovering", run.Status)
	require.Empty(t, run.WaitReason)

	// The pause view is gone once the Run left the budget wait.
	require.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet, pausePath, "", "").Code)

	// Replaying the same extension key after the resume is refused honestly:
	// the Run is no longer budget-paused, so there is nothing to extend
	// (404, not a phantom second extension and not a silent success).
	replay := env.do(t, http.MethodPost, extendPath, "",
		string(extensionBody))
	require.Equal(t, http.StatusNotFound, replay.Code, "a resumed Run no longer matches the paused extension precondition")

	// A malformed body never reaches the service.
	require.Equal(t, http.StatusBadRequest, env.do(t, http.MethodPost, extendPath, "", `{"key":""}`).Code)
}

// The nil-service registration fails closed: no registered pause API leaves
// the surface unmounted rather than serving a silent stub. The process-global
// registry is reset FIRST (production sets it exactly once at container
// assembly; tests must not inherit the previous journey's service).
func TestCraftT20BudgetPauseRoutesFailClosedWithoutService(t *testing.T) {
	RegisterCraftBudgetPauseHandler(nil, nil)
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-t20-budget-closed", "fail closed", "web")
	sessionID := created["session_id"].(string)
	w := env.do(t, http.MethodGet,
		"/api/v1/sessions/"+sessionID+"/craft/runs/run-x/budget/pause", "", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}
