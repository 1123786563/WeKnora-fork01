package session

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/execution/browserskill"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/docparser"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// Handler handles all HTTP requests related to conversation sessions
type Handler struct {
	messageService       interfaces.MessageService // Service for managing messages
	suggestionService    interfaces.MessageSuggestionService
	sessionService       interfaces.SessionService       // Service for managing sessions
	streamManager        interfaces.StreamManager        // Manager for handling streaming responses
	config               *config.Config                  // Application configuration
	knowledgebaseService interfaces.KnowledgeBaseService // Service for managing knowledge bases
	customAgentService   interfaces.CustomAgentService   // Service for managing custom agents
	tenantService        interfaces.TenantService        // Service for loading tenant (shared agent context)
	agentShareService    interfaces.AgentShareService    // Service for resolving shared agents (KB scope in retrieval)
	kbShareService       interfaces.KBShareService       // Service for resolving shared KB permissions
	fileService          interfaces.FileService          // Service for file storage (image uploads)
	resourceCatalog      interfaces.ResourceCatalog
	storageResolver      interfaces.StorageBackendResolver
	modelService         interfaces.ModelService // Service for model management (VLM access)
	attachmentProcessor  *AttachmentProcessor    // Processor for file attachments
	temporaryDocuments   interfaces.TemporaryDocumentService
	// artifactCollector drains skill-generated files from the session sandbox
	// after an agent turn completes. May be nil when the sandbox backend does
	// not support artifact collection; handlers must check before using.
	artifactCollector *service.ArtifactCollector
	memoryService     interfaces.MemoryService // Service for cross-session long-term memory
	// forkService branches a session at a chosen user or assistant message.
	// May be nil in deployments where fork is not wired; ForkSession checks.
	forkService sessionForker
	// workspaceCheckpointer commits the sandbox /workspace at the end of each
	// agent turn so session fork can roll back to a specific message. May be
	// nil when the deployment has no sandbox backend.
	workspaceCheckpointer *service.WorkspaceCheckpointer
	// sandboxIDLookup resolves a session's bound sandbox without provisioning
	// (A11 phase 3 checkpoint hook).
	sandboxIDLookup SandboxIDLookup
	// userService / memberService back the sandbox terminal's self-contained
	// handshake (browser WebSocket upgrades cannot send Authorization).
	userService   interfaces.UserService
	memberService interfaces.TenantMemberService
	// terminalService opens PTYs on the sandbox bound to a session. It also
	// owns first-use provisioning: the WS handshake carries the chat page's
	// selected agent so the sandbox is created with the same config a
	// conversation turn would use.
	terminalService *service.SandboxTerminalService
	agentRunService *service.AgentRunService
	// craftTombstoner starts the craft resource teardown of a session being
	// deleted (O03 integration wiring). Nil (craft not assembled) keeps the
	// unchanged deletion flow.
	craftTombstoner CraftSessionTombstoner
	// browserSkill is the local-browser gateway (A13). Nil-safe by design:
	// every browserskill.go handler treats the nil manager as disabled.
	browserSkill *browserskill.Manager
	// usageRecorder accumulates each finished chat turn's token usage into
	// the user's daily bucket (SP12). Nil (tests) skips accounting.
	usageRecorder interfaces.UsageRecorderService
	// queryHistoryExport backs the Admin+ async query-history CSV export
	// (SP13): the privacy gate, job admission, and the status/download reads.
	// The asynq worker body (ProcessExport) lives on the same service.
	queryHistoryExport queryHistoryExporter
	// usageRecordOnce is the one-shot gate for that accounting: the message
	// completion paths (stop watcher, QA defer, final-answer event) can race
	// on one assistant message with no lock between them, and LoadOrStore on
	// the message ID lets exactly one path record the turn — a user stop
	// racing the normal completion must not double-count the tokens. One
	// small entry per completed turn, for the handler's lifetime.
	usageRecordOnce sync.Map
	// completeMsgMu serializes completeAssistantMessage's message-mutation
	// section (UpdatedAt/IsCompleted + UpdateMessage) for the same racing
	// completion paths. Every path must still run the update — the mutex
	// orders the writes, it never skips them.
	completeMsgMu sync.Mutex
}

// queryHistoryExporter is the narrow port the export endpoints need from the
// query-history export service. Kept local so stub-based handler tests do
// not have to build the real service (which needs a database).
type queryHistoryExporter interface {
	// CheckAccess enforces the tenant query-history privacy policy: disabled
	// answers a ForbiddenError; anonymized returns the mode.
	CheckAccess(ctx context.Context, tenantID uint64) (string, error)
	// StartExport admits a pending job and enqueues the worker task.
	StartExport(ctx context.Context, tenantID uint64, requestedBy string, filter types.SessionListQuery) (uint64, error)
	// GetExportJob loads one job scoped to the caller's tenant.
	GetExportJob(ctx context.Context, tenantID uint64, jobID uint64) (*types.QueryHistoryExportJob, error)
}

// CraftSessionTombstoner starts the resource teardown of a deleted craft
// session: the tombstone mark blocks new delegation dispatches and snapshot
// restores mid-teardown before anything destructive happens.
type CraftSessionTombstoner interface {
	TombstoneSession(ctx context.Context, tenantID uint64, sessionID, reason string) (*service.CraftTombstoneResult, error)
}

// SetCraftTombstoner installs the craft lifecycle tombstone entry (O03
// integration wiring; see internal/container).
func (h *Handler) SetCraftTombstoner(t CraftSessionTombstoner) { h.craftTombstoner = t }

// tombstoneCraftSession runs the craft tombstone best-effort at the session
// deletion entrance. The periodic sweep's discovery pass re-derives the
// tombstone for sessions deleted without it, so a tombstone failure never
// blocks the user-facing delete — it is logged and left to the sweep.
func (h *Handler) tombstoneCraftSession(ctx context.Context, sessionID string) {
	if h.craftTombstoner == nil {
		return
	}
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return
	}
	if _, err := h.craftTombstoner.TombstoneSession(ctx, tenant, sessionID, "session deletion"); err != nil {
		logger.Warnf(ctx, "[CraftLifecycle] tombstone of session %s failed (sweep will re-derive): %v", sessionID, err)
	}
}

// NewHandler creates a new instance of Handler with all necessary dependencies
func NewHandler(
	sessionService interfaces.SessionService,
	messageService interfaces.MessageService,
	suggestionService interfaces.MessageSuggestionService,
	streamManager interfaces.StreamManager,
	config *config.Config,
	knowledgebaseService interfaces.KnowledgeBaseService,
	customAgentService interfaces.CustomAgentService,
	tenantService interfaces.TenantService,
	agentShareService interfaces.AgentShareService,
	kbShareService interfaces.KBShareService,
	fileService interfaces.FileService,
	resourceCatalog interfaces.ResourceCatalog,
	storageResolver interfaces.StorageBackendResolver,
	modelService interfaces.ModelService,
	documentReader interfaces.DocumentReader,
	imageResolver *docparser.ImageResolver,
	temporaryDocuments interfaces.TemporaryDocumentService,
	artifactCollector *service.ArtifactCollector,
	memoryService interfaces.MemoryService,
	// forkService branches a session at a chosen user or assistant message.
	// May be nil in deployments where fork is not wired; ForkSession checks.
	// Concrete-typed parameter so dig can inject it; the field keeps the
	// narrow interface for stub-based tests.
	forkService *service.SessionForkService,
	// workspaceCheckpointer + sandboxIDLookup back the per-turn git checkpoint
	// hook (A11 phase 3); both may be nil on sandbox-less deployments.
	workspaceCheckpointer *service.WorkspaceCheckpointer,
	sandboxIDLookup SandboxIDLookup,
	userService interfaces.UserService,
	memberService interfaces.TenantMemberService,
	terminalService *service.SandboxTerminalService,
	// browserSkill is the local-browser gateway (A13); nil-safe (see field).
	browserSkill *browserskill.Manager,
	// usageRecorder writes each finished chat turn's terminal token usage
	// into the user's user_usage daily bucket (SP12).
	usageRecorder interfaces.UsageRecorderService,
	// queryHistoryExport backs the Admin+ async query-history CSV export
	// (SP13 Task 4). Concrete-typed parameter so dig can inject it; the
	// field keeps the narrow interface for stub-based tests.
	queryHistoryExport *service.QueryHistoryExportService,
) *Handler {
	return &Handler{
		sessionService:        sessionService,
		messageService:        messageService,
		suggestionService:     suggestionService,
		streamManager:         streamManager,
		config:                config,
		knowledgebaseService:  knowledgebaseService,
		customAgentService:    customAgentService,
		tenantService:         tenantService,
		agentShareService:     agentShareService,
		kbShareService:        kbShareService,
		fileService:           fileService,
		resourceCatalog:       resourceCatalog,
		storageResolver:       storageResolver,
		modelService:          modelService,
		temporaryDocuments:    temporaryDocuments,
		artifactCollector:     artifactCollector,
		memoryService:         memoryService,
		forkService:           forkService,
		workspaceCheckpointer: workspaceCheckpointer,
		sandboxIDLookup:       sandboxIDLookup,
		userService:           userService,
		memberService:         memberService,
		terminalService:       terminalService,
		browserSkill:          browserSkill,
		usageRecorder:         usageRecorder,
		queryHistoryExport:    queryHistoryExport,
		attachmentProcessor: NewAttachmentProcessor(
			fileService,
			documentReader,
			imageResolver,
			modelService,
		),
	}
}

// CreateSession godoc
// @Summary      创建会话
// @Description  创建新的对话会话
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        request  body      CreateSessionRequest  true  "会话创建请求"
// @Success      201      {object}  map[string]interface{}  "创建的会话"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions [post]
func (h *Handler) CreateSession(c *gin.Context) {
	ctx := c.Request.Context()
	// Parse and validate the request body
	var request CreateSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to validate session creation parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Get tenant ID from context
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Sessions are now knowledge-base-independent:
	// - All configuration comes from custom agent at query time
	// - Session only stores basic info (tenant ID, title, description)
	logger.Infof(
		ctx,
		"Processing session creation request, tenant ID: %d",
		tenantID,
	)

	// Create session object with base properties
	engine, parseErr := types.ParseAgentEngine(string(request.EngineType))
	if parseErr != nil {
		c.Error(errors.NewBadRequestError(parseErr.Error()))
		return
	}
	createdSession := &types.Session{
		TenantID:    tenantID.(uint64),
		Title:       request.Title,
		Description: types.SanitizeClientSessionDescription(request.Description, ""),
		EngineType:  string(engine),
	}
	// Attach the calling user as the session owner when available.
	// API-key callers scope sessions per external user when configured;
	// otherwise they fall back to the synthetic tenant user.
	if ownerID := types.SessionOwnerIDFromContext(ctx); ownerID != "" {
		createdSession.UserID = ownerID
	}

	// Call service to create session
	logger.Infof(ctx, "Calling session service to create session")
	createdSession, err := h.sessionService.CreateSession(ctx, createdSession)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return created session
	logger.Infof(ctx, "Session created successfully, ID: %s", createdSession.ID)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    createdSession,
	})
}

// GetSession godoc
// @Summary      获取会话详情
// @Description  根据ID获取会话详情
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "会话详情"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [get]
func (h *Handler) GetSession(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start retrieving session")

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Call service to get session details
	logger.Infof(ctx, "Retrieving session, ID: %s", id)
	session, err := h.sessionService.GetSession(ctx, id)
	if err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return session data
	logger.Infof(ctx, "Session retrieved successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    session,
	})
}

// GetSessionsByTenant godoc
// @Summary      获取会话列表
// @Description  获取当前空间的会话列表，支持分页、关键字搜索、按来源/Agent 筛选；source=all 为 Admin+ 审计视图，支持按用户/时间范围/反馈筛选
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        page         query     int     false  "页码"
// @Param        page_size    query     int     false  "每页数量"
// @Param        keyword      query     string  false  "标题模糊搜索"
// @Param        source       query     string  false  "来源过滤：web / embed / api / feishu / wechat / all（api、embed、IM 渠道、all 需 Admin+）"
// @Param        agent_id     query     string  false  "按 Agent 过滤（仅对 IM 会话生效）"
// @Param        user_id      query     string  false  "按用户过滤（仅 Admin+ 的 source=all 审计视图生效）"
// @Param        start_time   query     string  false  "创建时间下界（含），RFC3339 或 2006-01-02"
// @Param        end_time     query     string  false  "创建时间上界（不含），RFC3339 或 2006-01-02"
// @Param        feedback     query     string  false  "反馈过滤：like / dislike（空为不过滤）"
// @Success      200          {object}  map[string]interface{}  "会话列表"
// @Failure      400          {object}  errors.AppError         "请求参数错误"
// @Failure      403          {object}  errors.AppError         "无权限（非 Admin+ 使用受限 source，或租户停用查询历史）"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions [get]
func (h *Handler) GetSessionsByTenant(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters from query
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		logger.Error(ctx, "Failed to parse pagination parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Audit-listing filters. Times follow the parseFilterTime layouts used by
	// the knowledge list filters; the feedback rating only accepts like /
	// dislike (empty = no filter). user_id is honored only on the Admin+
	// source=all audit view — the service overwrites it with the caller's own
	// principal on every non-admin path.
	startTime, err := parseSessionFilterTime(c.Query("start_time"))
	if err != nil {
		c.Error(errors.NewBadRequestError("invalid start_time: " + err.Error()))
		return
	}
	endTime, err := parseSessionFilterTime(c.Query("end_time"))
	if err != nil {
		c.Error(errors.NewBadRequestError("invalid end_time: " + err.Error()))
		return
	}
	feedback := strings.TrimSpace(c.Query("feedback"))
	switch feedback {
	case "", types.FeedbackRatingLike, types.FeedbackRatingDislike:
	default:
		c.Error(errors.NewBadRequestError("invalid feedback: must be like or dislike"))
		return
	}

	// Response items always include pin state and (when available) IM origin
	// fields so the frontend can render pin icons / source badges without a
	// second roundtrip. Unset filter params behave like "no filter".
	result, err := h.sessionService.ListSessions(ctx, &types.SessionListQuery{
		Keyword:        c.Query("keyword"),
		Source:         c.Query("source"),
		AgentID:        c.Query("agent_id"),
		UserID:         c.Query("user_id"),
		StartTime:      startTime,
		EndTime:        endTime,
		FeedbackRating: feedback,
		Page:           pagination.Page,
		PageSize:       pagination.PageSize,
	})
	if err != nil {
		// Policy denials (Admin+ gate, query history disabled) carry their own
		// status; everything else is an internal error.
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result.Data,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// parseSessionFilterTime parses an optional session-list timestamp filter.
// It accepts the same layouts as the knowledge list filters (parseFilterTime
// in the parent handler package; duplicated here because that package imports
// this one): RFC3339 (with or without fractional seconds), "2006-01-02
// 15:04:05", and the date-only "2006-01-02" form interpreted at start of day
// in the local timezone. An empty value leaves the range bound open.
func parseSessionFilterTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

// UpdateSession godoc
// @Summary      更新会话
// @Description  更新会话属性
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        id       path      string         true  "会话ID"
// @Param        request  body      types.Session  true  "会话信息"
// @Success      200      {object}  map[string]interface{}  "更新后的会话"
// @Failure      404      {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [put]
func (h *Handler) UpdateSession(c *gin.Context) {
	ctx := c.Request.Context()

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Verify tenant ID from context for authorization
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Parse request body to session object
	var session types.Session
	if err := c.ShouldBindJSON(&session); err != nil {
		logger.Error(ctx, "Failed to parse session data", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	session.ID = id
	session.TenantID = tenantID.(uint64)
	if existing, loadErr := h.sessionService.GetOwnedSession(ctx, id); loadErr != nil {
		c.Error(errors.NewNotFoundError(loadErr.Error()))
		return
	} else {
		current, parseErr := types.ParseAgentEngine(existing.EngineType)
		if parseErr != nil {
			c.Error(errors.NewInternalServerError(parseErr.Error()))
			return
		}
		if err := ValidateEngineUpdate(current, types.AgentEngineType(session.EngineType)); err != nil {
			c.Error(errors.NewConflictError(err.Error()))
			return
		}
		session.EngineType = string(current)
	}

	// Call service to update session
	if err := h.sessionService.UpdateSession(ctx, &session); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Reload session from database to return complete timestamps and stored fields
	updatedSession, err := h.sessionService.GetSession(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return updated session
	logger.Infof(ctx, "Session updated successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedSession,
	})
}

// fenceSessionRuns prevents durable workers from taking over while legacy
// session deletion/clearing removes the visible session state.
func (h *Handler) fenceSessionRuns(ctx context.Context, sessionID string) error {
	runs := h.runService()
	if runs == nil {
		return errors.NewServiceUnavailableError("durable agent runs are unavailable")
	}
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return errors.NewUnauthorizedError("Unauthorized")
	}
	return runs.DeleteSessionRuns(ctx, tenant, sessionID)
}

// DeleteSession godoc
// @Summary      删除会话
// @Description  删除指定的会话
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "删除成功"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [delete]
func (h *Handler) DeleteSession(c *gin.Context) {
	ctx := c.Request.Context()

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	if _, ownErr := h.sessionService.GetOwnedSession(ctx, id); ownErr != nil {
		c.Error(errors.NewNotFoundError("session not found"))
		return
	}
	// O03 wiring: the craft tombstone precedes the destructive deletes — the
	// deleting mark blocks new dispatches and restores mid-teardown.
	h.tombstoneCraftSession(ctx, id)
	if err := h.fenceSessionRuns(ctx, id); err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	// Call service to delete session
	if err := h.sessionService.DeleteSession(ctx, id); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	h.browserSkill.Forget(browserSkillScope(ctx), []string{id})

	// Return success message
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session deleted successfully",
	})
}

// ClearSessionMessages godoc
// @Summary      清空会话消息
// @Description  删除会话中的所有消息，同时清除 LLM 上下文和聊天历史知识库条目。会话本身保留。
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "清空成功"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/messages [delete]
func (h *Handler) ClearSessionMessages(c *gin.Context) {
	ctx := c.Request.Context()

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	logger.Infof(ctx, "Clearing all messages for session: %s", id)
	if _, ownErr := h.sessionService.GetOwnedSession(ctx, id); ownErr != nil {
		c.Error(errors.NewNotFoundError("session not found"))
		return
	}
	if err := h.fenceSessionRuns(ctx, id); err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	if err := h.messageService.ClearSessionMessages(ctx, id); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"session_id": id})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Session messages cleared successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session messages cleared successfully",
	})
}

// batchDeleteRequest represents the request body for batch deleting sessions
type batchDeleteRequest struct {
	IDs       []string `json:"ids"`
	DeleteAll bool     `json:"delete_all"`
}

// BatchDeleteSessions godoc
// @Summary      批量删除会话
// @Description  根据ID列表批量删除对话会话，或设置 delete_all=true 删除当前空间的所有会话
// @Tags         会话
// @Accept       json
// @Produce      json
// @Param        request  body      batchDeleteRequest  true  "批量删除请求"
// @Success      200      {object}  map[string]interface{}  "删除结果"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/batch [delete]
func (h *Handler) BatchDeleteSessions(c *gin.Context) {
	ctx := c.Request.Context()

	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf(ctx, "Invalid batch delete request: %v", err)
		c.Error(errors.NewBadRequestError("invalid request"))
		return
	}

	if req.DeleteAll {
		sessions, listErr := h.sessionService.GetSessionsByTenant(ctx)
		if listErr != nil {
			c.Error(errors.NewInternalServerError(listErr.Error()))
			return
		}
		for _, sess := range sessions {
			if sess != nil {
				h.tombstoneCraftSession(ctx, sess.ID)
				if err := h.fenceSessionRuns(ctx, sess.ID); err != nil {
					c.Error(errors.NewInternalServerError(err.Error()))
					return
				}
			}
		}
		if err := h.sessionService.DeleteAllSessions(ctx); err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError(err.Error()))
			return
		}
		h.browserSkill.ForgetAll(browserSkillScope(ctx))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "All sessions deleted successfully",
		})
		return
	}

	if len(req.IDs) == 0 {
		c.Error(errors.NewBadRequestError("ids are required when delete_all is false"))
		return
	}

	// Sanitize all IDs
	sanitizedIDs := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		sanitized := secutils.SanitizeForLog(id)
		if sanitized != "" {
			sanitizedIDs = append(sanitizedIDs, sanitized)
		}
	}

	if len(sanitizedIDs) == 0 {
		c.Error(errors.NewBadRequestError("no valid session IDs provided"))
		return
	}

	for _, id := range sanitizedIDs {
		if _, ownErr := h.sessionService.GetOwnedSession(ctx, id); ownErr != nil {
			c.Error(errors.NewNotFoundError("session not found"))
			return
		}
		h.tombstoneCraftSession(ctx, id)
		if err := h.fenceSessionRuns(ctx, id); err != nil {
			c.Error(errors.NewInternalServerError(err.Error()))
			return
		}
	}
	if err := h.sessionService.BatchDeleteSessions(ctx, sanitizedIDs); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "No visible sessions found for batch delete")
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	h.browserSkill.Forget(browserSkillScope(ctx), sanitizedIDs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sessions deleted successfully",
	})
}

// PinSession godoc
// @Summary      置顶会话
// @Description  将指定会话置顶（用户维度）
// @Tags         会话
// @Produce      json
// @Param        session_id   path      string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "置顶成功"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{session_id}/pin [post]
func (h *Handler) PinSession(c *gin.Context) {
	h.setSessionPinned(c, true)
}

// UnpinSession godoc
// @Summary      取消置顶会话
// @Description  取消指定会话的置顶
// @Tags         会话
// @Produce      json
// @Param        id   path      string  true  "会话ID"
// @Success      200  {object}  map[string]interface{}  "取消置顶成功"
// @Failure      404  {object}  errors.AppError         "会话不存在"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/pin [delete]
func (h *Handler) UnpinSession(c *gin.Context) {
	h.setSessionPinned(c, false)
}

func (h *Handler) setSessionPinned(c *gin.Context, pinned bool) {
	ctx := c.Request.Context()

	// POST and DELETE for /sessions/.../pin register under different wildcards
	// (POST :session_id, DELETE :id — see router.go). Accept whichever is set.
	rawID := c.Param("session_id")
	if rawID == "" {
		rawID = c.Param("id")
	}
	id := secutils.SanitizeForLog(rawID)
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	rows, err := h.sessionService.SetSessionPinned(ctx, id, pinned)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": id,
			"pinned":     pinned,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	// Zero rows means the session doesn't exist or isn't visible to this user;
	// tell the client rather than reporting success.
	if rows == 0 {
		c.Error(errors.NewNotFoundError(errors.ErrSessionNotFound.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"is_pinned": pinned,
	})
}
