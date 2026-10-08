package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/codedelivery/repository/codedelivery"
	"github.com/gin-gonic/gin"
)

// DeliveryService is the codedelivery seam this handler drives. The
// production *codedelivery.CodeDeliveryService satisfies it; tests stub it.
type DeliveryService interface {
	MaterializeBaseline(ctx context.Context, in codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error)
	PrepareDelivery(ctx context.Context, in codedelivery.PrepareInput) (codedelivery.DeliveryView, error)
	DispatchDelivery(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error)
	ResolveDeliveryUnknown(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error)
	GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (codedelivery.DeliveryView, error)
}

// WorkbenchDeliveryHandler owns the developer-delivery endpoints. Writes
// (baseline/prepare/dispatch/resolve) are owner-only; the read face reuses
// the strict-owner + task-grant fallback predicate (#42 read face).
type WorkbenchDeliveryHandler struct {
	runs    OwnedRunReader
	granted GrantedRunReader
	service DeliveryService
}

func NewWorkbenchDeliveryHandler(runs OwnedRunReader, granted GrantedRunReader, service DeliveryService) *WorkbenchDeliveryHandler {
	return &WorkbenchDeliveryHandler{runs: runs, granted: granted, service: service}
}

// caller resolves the authenticated tenant and actor the same way
// resolveOwnedRun does (workbenchCaller: request context first, then the gin
// keys the auth middleware writes in lockstep).
func (h *WorkbenchDeliveryHandler) caller(c *gin.Context) (uint64, string, bool) {
	tenantID, userID := workbenchCaller(c)
	if tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized", "error": "tenant and user identity required"})
		return 0, "", false
	}
	return tenantID, userID, true
}

type deliveryBaselineInput struct {
	ConnectionID string `json:"connection_id"`
	Repo         string `json:"repo"`
	BaselineSHA  string `json:"baseline_sha"`
}

// MaterializeBaseline POST /workbench/executions/:run_id/baseline — owner-only.
func (h *WorkbenchDeliveryHandler) MaterializeBaseline(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input deliveryBaselineInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.BaselineSHA == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_request", "error": "connection_id, repo and baseline_sha are required"})
		return
	}
	repo, err := codedelivery.ParseRepoRef(input.Repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_repo", "error": "repo must be owner/name"})
		return
	}
	receipt, err := h.service.MaterializeBaseline(c.Request.Context(), codedelivery.BaselineInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID,
		ConnectionID: input.ConnectionID, Repo: repo, BaselineSHA: input.BaselineSHA,
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"files": receipt.Files, "root": receipt.Root}})
}

type deliveryPrepareInput struct {
	ConnectionID  string `json:"connection_id"`
	Repo          string `json:"repo"`
	BaselineSHA   string `json:"baseline_sha"`
	Branch        string `json:"branch"`
	CommitMessage string `json:"commit_message"`
	PRTitle       string `json:"pr_title"`
}

// PrepareDelivery POST /workbench/executions/:run_id/delivery — owner-only.
func (h *WorkbenchDeliveryHandler) PrepareDelivery(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input deliveryPrepareInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.BaselineSHA == "" ||
		input.CommitMessage == "" || input.PRTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_request", "error": "connection_id, repo, baseline_sha, commit_message and pr_title are required"})
		return
	}
	repo, err := codedelivery.ParseRepoRef(input.Repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_repo", "error": "repo must be owner/name"})
		return
	}
	view, err := h.service.PrepareDelivery(c.Request.Context(), codedelivery.PrepareInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID,
		ConnectionID: input.ConnectionID, Repo: repo, BaselineSHA: input.BaselineSHA,
		Branch: strings.TrimSpace(input.Branch), CommitMessage: input.CommitMessage, PRTitle: input.PRTitle,
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// GetDelivery GET /workbench/executions/:run_id/delivery — owner + granted.
func (h *WorkbenchDeliveryHandler) GetDelivery(c *gin.Context) {
	tenantID, _, ok := h.caller(c)
	if !ok {
		return
	}
	if _, readable := h.resolveReadable(c); !readable {
		return
	}
	view, err := h.service.GetDeliveryForRun(c.Request.Context(), tenantID, c.Param("run_id"))
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// DispatchDelivery POST /workbench/executions/:run_id/delivery/:delivery_id/dispatch — owner-only.
func (h *WorkbenchDeliveryHandler) DispatchDelivery(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	view, err := h.service.DispatchDelivery(c.Request.Context(), codedelivery.DispatchInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID, DeliveryID: c.Param("delivery_id"),
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// ResolveDeliveryUnknown POST /workbench/executions/:run_id/delivery/:delivery_id/resolve — owner-only.
func (h *WorkbenchDeliveryHandler) ResolveDeliveryUnknown(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var body struct {
		ConfirmNoMatchingPR bool `json:"confirm_no_matching_pr"`
	}
	if c.Request.Body != nil {
		raw, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid resolve body"})
			return
		}
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) > 0 && trimmed[0] != '{' {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid resolve body"})
			return
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid resolve body"})
			return
		}
		if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid resolve body"})
			return
		}
	}
	view, err := h.service.ResolveDeliveryUnknown(c.Request.Context(), codedelivery.DispatchInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID, DeliveryID: c.Param("delivery_id"), ConfirmNoMatchingPR: body.ConfirmNoMatchingPR,
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// resolveReadable mirrors WorkbenchReadHandler.resolveReadableRun (strict
// owner first, task-grant fallback) without importing its private state.
func (h *WorkbenchDeliveryHandler) resolveReadable(c *gin.Context) (agentruntime.Run, bool) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return agentruntime.Run{}, false
	}
	run, err := h.runs.GetOwnedRun(c.Request.Context(), tenantID, userID, c.Param("run_id"))
	if err == nil {
		return run, true
	}
	if h.granted != nil {
		if granted, gerr := h.granted.GetRunForGrantedReader(c.Request.Context(), tenantID, userID, c.Param("run_id")); gerr == nil {
			return granted, true
		}
	}
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "run_not_found", "error": "execution not found"})
	return agentruntime.Run{}, false
}

// writeDeliveryError maps codedelivery failures onto a fixed code table;
// no upstream text or credential-adjacent string crosses the wire.
func writeDeliveryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, codedelivery.ErrNotDeliveryOwner),
		errors.Is(err, codedelivery.ErrConnectionNotUsable):
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "code_delivery_forbidden", "error": "delivery requires the run owner's personal connection"})
	case errors.Is(err, codedelivery.ErrProtectedBranch):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_protected_branch", "error": "the target branch is protected or is the default branch"})
	case errors.Is(err, codedelivery.ErrDeliveryState):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_state_conflict", "error": "delivery state does not allow this operation"})
	case errors.Is(err, codedelivery.ErrDeliveryConfirmationRequired):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_owner_confirmation_required", "error": "inspect the remote repository and confirm no matching pull request exists"})
	case errors.Is(err, deliveryrepo.ErrDeliveryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "code_delivery_not_found", "error": "delivery not found"})
	case errors.Is(err, codedelivery.ErrInvalidMaterial),
		errors.Is(err, codedelivery.ErrInvalidBranch),
		errors.Is(err, codedelivery.ErrInvalidBaselineSHA),
		errors.Is(err, codedelivery.ErrRepoRefInvalid),
		errors.Is(err, codedelivery.ErrBaselineTooLarge):
		// 输入类错误（材料/分支/基线/仓库引用非法或超限）：400，不落 500。
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "code_delivery_invalid_material", "error": "the delivery material or target is invalid"})
	case errors.Is(err, codedelivery.ErrDeliveryDispatchRejected):
		// 可证未出网的前置门拒绝（审批已消费）：409，重试须重新 Prepare。
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_dispatch_rejected", "error": "the delivery was refused before any remote call; prepare a new delivery to retry"})
	case errors.Is(err, codedelivery.ErrUnsupportedProvider):
		// 连接背后的安装 app 不是代码平台（T24 #54）：可证未出网的输入类
		// 错误，400 固定码，不落 5xx。
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "code_delivery_unsupported_provider", "error": "the connection is not a code platform connection"})
	default:
		var apiErr *codedelivery.GitHubAPIError
		if errors.As(err, &apiErr) {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "code": "code_delivery_provider_refused", "error": "the code platform refused the delivery"})
			return
		}
		if errors.Is(err, codedelivery.ErrGitHubRequestInvalid) {
			// 请求构建失败 = 可证未出网：这是我方请求畸形，不是平台不可达。
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "code_delivery_request_invalid", "error": "the delivery request could not be constructed; nothing reached the code platform"})
			return
		}
		if errors.Is(err, codedelivery.ErrGitHubTransport) {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "code": "code_delivery_provider_unreachable", "error": "the code platform outcome is unobservable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "code_delivery_failed", "error": "delivery failed"})
	}
}
