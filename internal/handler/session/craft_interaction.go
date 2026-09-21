package session

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// CraftInteractionHandler serves the C02 pending-decision surface:
//
//	GET  /sessions/:session_id/craft/interactions
//	POST /sessions/:session_id/craft/interactions/:interaction_id/decide
//
// The decide request mirrors the tRPC Decision vocabulary (pending id,
// decision id, expected revision) plus the C02 multi-question answer set. The
// answer is 202 Accepted: the durable decision and its delivery state are
// reported honestly — a delivery the runtime cannot confirm surfaces as
// delivery_unknown ("decision recorded, delivery unconfirmed"), never a
// fabricated success. Status codes follow craftHTTPError plus 410 for a
// canceled or terminal interaction.
type CraftInteractionHandler struct {
	svc CraftInteractionAPI
}

// CraftInteractionAPI is the handler's view of the C02 control service; the
// concrete *service.CraftControlService satisfies it.
type CraftInteractionAPI interface {
	ListInteractions(context.Context, craft.Scope, string) ([]service.CraftInteractionRecord, error)
	Decide(context.Context, service.CraftDecisionRequest) (service.CraftDecisionOutcome, error)
	Stop(context.Context, service.CraftStopRequest) (service.CraftStopStatus, error)
	DelegationStatus(context.Context, craft.Scope, agentruntime.RunKey, string) (service.CraftStopStatus, error)
}

var _ CraftInteractionAPI = (*service.CraftControlService)(nil)

// NewCraftInteractionHandler constructs the handler. A nil svc answers 503.
func NewCraftInteractionHandler(svc CraftInteractionAPI) *CraftInteractionHandler {
	return &CraftInteractionHandler{svc: svc}
}

var registeredCraftInteractionHandler CraftInteractionAPI

// RegisterCraftInteractionHandler installs the handler for route mounting.
func RegisterCraftInteractionHandler(h CraftInteractionAPI) { registeredCraftInteractionHandler = h }

// RegisteredCraftInteractionHandler returns the registered handler (nil when
// the C02 assembly is not wired — then the surface stays unmounted).
func RegisteredCraftInteractionHandler() CraftInteractionAPI {
	return registeredCraftInteractionHandler
}

// RegisterCraftInteractionRoutes mounts the C02 interaction routes inside the
// sessions group so they inherit its auth chain. A nil handler leaves the
// surface unmounted (fail-closed, no silent 404 shims).
func RegisterCraftInteractionRoutes(sessions craftRouteGroup, h *CraftInteractionHandler) {
	if sessions == nil || h == nil {
		return
	}
	// The GET tree binds :id (see RegisterSessionRoutes); handlers accept
	// both names. POST routes use :session_id like their craft siblings.
	sessions.GET("/:id/craft/interactions", h.ListCraftInteractions)
	sessions.POST("/:session_id/craft/interactions/:interaction_id/decide", h.DecideCraftInteraction)
	sessions.POST("/:session_id/craft/runs/:run_id/stop", h.StopCraftRun)
	sessions.GET("/:id/craft/runs/:run_id/delegations/:task_id/status", h.GetCraftDelegationStatus)
}

// interactionView is the wire shape of one pending decision.
type interactionView struct {
	ID            string       `json:"id"`
	Kind          string       `json:"kind"`
	Prompt        string       `json:"prompt"`
	Status        string       `json:"status"`
	Delivery      string       `json:"delivery"`
	Revision      int64        `json:"revision"`
	ArgsHash      string       `json:"args_hash"`
	PendingID     string       `json:"pending_id"`
	DecisionID    string       `json:"decision_id,omitempty"`
	DecidedAction string       `json:"decided_action,omitempty"`
	Pending       *pendingView `json:"pending,omitempty"`
	Answers       []answerView `json:"answers,omitempty"`
}

type pendingView struct {
	Options     map[string][]string  `json:"options,omitempty"`
	Multiple    map[string]bool      `json:"multiple,omitempty"`
	Permissions []permissionItemView `json:"permissions,omitempty"`
}

type permissionItemView struct {
	Tool    string `json:"tool,omitempty"`
	Command string `json:"command,omitempty"`
	Path    string `json:"path,omitempty"`
	Scope   string `json:"scope,omitempty"`
}

type answerView struct {
	QuestionID string   `json:"question_id"`
	Choices    []string `json:"choices,omitempty"`
	Text       string   `json:"text,omitempty"`
}

func interactionViewOf(record service.CraftInteractionRecord) interactionView {
	view := interactionView{
		ID: record.ID, Kind: record.Kind, Prompt: record.Prompt, Status: record.Status,
		Delivery: record.Delivery, Revision: record.Revision, ArgsHash: record.ArgsHash,
		PendingID: record.PendingID, DecisionID: record.DecisionID, DecidedAction: record.DecidedAction,
	}
	if len(record.Pending.Options) > 0 || len(record.Pending.Multiple) > 0 || len(record.Pending.Permissions) > 0 {
		pending := &pendingView{
			Options: record.Pending.Options, Multiple: record.Pending.Multiple,
		}
		for _, item := range record.Pending.Permissions {
			pending.Permissions = append(pending.Permissions, permissionItemView{
				Tool: item.Tool, Command: item.Command, Path: item.Path, Scope: item.Scope,
			})
		}
		view.Pending = pending
	}
	for _, answer := range record.RecordedAnswers {
		view.Answers = append(view.Answers, answerView{
			QuestionID: answer.QuestionID, Choices: answer.Choices, Text: answer.Text,
		})
	}
	return view
}

// ListCraftInteractions serves GET /api/v1/sessions/:session_id/craft/
// interactions: the session's pending decisions with their question payloads
// and, once decided, the recorded answers (restart re-display).
func (h *CraftInteractionHandler) ListCraftInteractions(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft interactions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	records, err := h.svc.ListInteractions(c.Request.Context(), scope, scope.SessionID)
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	views := make([]interactionView, 0, len(records))
	for _, record := range records {
		views = append(views, interactionViewOf(record))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

type decideRequestBody struct {
	DecisionID       string       `json:"decision_id"`
	Action           string       `json:"action"`
	ArgsHash         string       `json:"args_hash"`
	ExpectedRevision int64        `json:"expected_revision"`
	Answers          []answerBody `json:"answers"`
	Answer           string       `json:"answer"`
	Reason           string       `json:"reason"`
}

type answerBody struct {
	QuestionID string   `json:"question_id"`
	Choices    []string `json:"choices"`
	Text       string   `json:"text"`
}

// DecideCraftInteraction serves POST /api/v1/sessions/:session_id/craft/
// interactions/:interaction_id/decide. The durable decision precedes every
// forward; the response reports the honest delivery state.
func (h *CraftInteractionHandler) DecideCraftInteraction(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft interactions are unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body decideRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	req := service.CraftDecisionRequest{
		Scope:            scope,
		InteractionID:    c.Param("interaction_id"),
		DecisionID:       body.DecisionID,
		Action:           body.Action,
		ArgsHash:         body.ArgsHash,
		ExpectedRevision: body.ExpectedRevision,
		Answer:           body.Answer,
		Reason:           body.Reason,
		Operator:         craftOperator(c),
	}
	for _, answer := range body.Answers {
		req.Answers = append(req.Answers, craft.Answer{
			QuestionID: answer.QuestionID, Choices: answer.Choices, Text: answer.Text,
		})
	}
	outcome, err := h.svc.Decide(c.Request.Context(), req)
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{
		"decided":     outcome.Decided,
		"delivered":   outcome.Delivered,
		"delivery":    outcome.Record.Delivery,
		"note":        outcome.DeliveryNote,
		"interaction": interactionViewOf(outcome.Record),
	}})
}

// craftInteractionHTTPError extends craftHTTPError with the C02 410 mapping:
// a canceled or otherwise terminal interaction no longer accepts input.
func craftInteractionHTTPError(c *gin.Context, err error) {
	if stderrors.Is(err, craft.ErrGone) {
		c.JSON(http.StatusGone, gin.H{
			"success": false,
			"error":   gin.H{"code": "gone", "message": err.Error()},
		})
		return
	}
	craftHTTPError(c, err)
}

type stopRequestBody struct {
	TaskID string `json:"task_id"`
}

// StopCraftRun serves POST /api/v1/sessions/:session_id/craft/runs/:run_id/
// stop: the R06 verifiable stop. The response keeps the honest phase —
// "stopping" is a real answer (abort not yet confirmed), never folded into
// a boolean.
func (h *CraftInteractionHandler) StopCraftRun(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft control is unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body stopRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	if runID == "" || strings.TrimSpace(body.TaskID) == "" {
		c.Error(apperrors.NewBadRequestError("run_id and task_id are required"))
		return
	}
	status, err := h.svc.Stop(c.Request.Context(), service.CraftStopRequest{
		Scope:  scope,
		RunKey: agentruntime.RunKey{TenantID: scope.TenantID, RunID: runID},
		TaskID: strings.TrimSpace(body.TaskID),
	})
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	data := gin.H{"phase": status.Phase, "note": status.Note}
	if status.Result != nil {
		data["result_status"] = status.Result.Status
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// GetCraftDelegationStatus serves GET /api/v1/sessions/:id/craft/runs/
// :run_id/delegations/:task_id/status: the read-only poll the client runs
// after a stop answered "stopping". It writes nothing.
func (h *CraftInteractionHandler) GetCraftDelegationStatus(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft control is unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	taskID := strings.TrimSpace(c.Param("task_id"))
	if runID == "" || taskID == "" {
		c.Error(apperrors.NewBadRequestError("run_id and task_id are required"))
		return
	}
	status, err := h.svc.DelegationStatus(c.Request.Context(), scope,
		agentruntime.RunKey{TenantID: scope.TenantID, RunID: runID}, taskID)
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"phase": status.Phase, "note": status.Note}})
}

// craftOperator derives the acting user for the audit trail (identity only,
// never a secret).
func craftOperator(c *gin.Context) string {
	if owner := types.SessionOwnerIDFromContext(c.Request.Context()); owner != "" {
		return owner
	}
	user, _ := types.UserIDFromContext(c.Request.Context())
	return user
}
