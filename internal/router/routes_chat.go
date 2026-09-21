package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
	// Alias: RegisterSessionRoutes' `handler *session.Handler` parameter
	// shadows the package name inside the function body; the W27 preview
	// issue route still needs the top-level handler package.
	handlerapi "github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
)

// RegisterMessageRoutes 注册消息相关的路由。
//
// Per-session ownership is already enforced inside each handler (the
// user must own the session). We add Viewer+ here so non-members
// (e.g. revoked accounts retained in the tenant for audit) cannot
// reach the endpoints at all once RBAC is on.
func RegisterMessageRoutes(r *gin.RouterGroup, handler *handler.MessageHandler, g *rbacGuards) {
	// Message history is tenant-wide and not attributable to a KB, so it is
	// a full-access surface for API keys by default. The narrow
	// exceptions are explicit capabilities:
	//   - chat: load/delete messages inside the caller's own session, where
	//     ownership is enforced by the message service.
	//   - message_history: search/read tenant chat-history metadata without
	//     granting every other full-access API.
	messages := g.apiKeyGroup(r.Group("/messages"), apiKeyFullAccess())
	chatMessages := messages.With(apiKeyChat(apiKeyFullAccess()))
	historyMessages := messages.With(apiKeyMessageHistory(apiKeyFullAccess()))
	{
		historyMessages.POST("/search", g.Viewer(), handler.SearchMessages)
		historyMessages.GET("/chat-history-stats", g.Viewer(), handler.GetChatHistoryKBStats)
		chatMessages.GET("/:session_id/load", g.Viewer(), handler.LoadMessages)
		chatMessages.DELETE("/:session_id/:id", g.Viewer(), handler.DeleteMessage)
	}
}

// RegisterFeedbackRoutes 注册消息反馈路由（SP11）。
//
// Feedback is a per-session surface like the message routes above: the
// feedback service enforces owner-or-Admin+ access, so Viewer+ gating here
// only keeps non-members out once RBAC is on. NOTE on wildcard names: gin
// keeps one radix tree per HTTP verb and requires identical wildcard names
// at the same position; the existing DELETE /messages/:session_id/:id owns
// that tree position, so the DELETE feedback route must reuse :id — the
// handler resolves the message id with a :message_id/:id fallback (same
// pattern as the sessions pin/artifact routes).
func RegisterFeedbackRoutes(r *gin.RouterGroup, handler *handler.FeedbackHandler, g *rbacGuards) {
	// Feedback rides the chat surface: a scoped key needs the chat capability
	// (or full tenant access), exactly like loading/deleting messages.
	messages := g.apiKeyGroup(r.Group("/messages"), apiKeyFullAccess())
	chatMessages := messages.With(apiKeyChat(apiKeyFullAccess()))
	{
		chatMessages.POST("/:session_id/:message_id/feedback", g.Viewer(), handler.SubmitFeedback)
		chatMessages.DELETE("/:session_id/:id/feedback", g.Viewer(), handler.RemoveFeedback)
		chatMessages.GET("/:session_id/feedback/mine", g.Viewer(), handler.ListMyFeedback)
	}
}

// RegisterSessionShareRoutes 注册会话分享路由（SP13 Task 5）。
//
// Mint/revoke ride the sessions group like the pin routes (Viewer+ at the
// route, owner-or-Admin+ inside the service — the SP11 canFeedback gate).
// The read side is its own /shared tree so a share link reads as a link,
// with the same Viewer+ / chat-capability chain: the token only resolves
// inside the minting tenant, so a leaked link is worthless logged-out or
// cross-tenant. NOTE on wildcard names: gin keeps one radix tree per verb —
// the POST sessions tree binds :session_id at this position, the DELETE
// tree binds :id, so each route reuses its tree's name and the handler
// resolves the param with a :session_id/:id fallback.
func RegisterSessionShareRoutes(r *gin.RouterGroup, handler *session.Handler, g *rbacGuards) {
	// Sharing is a per-session chat action: a scoped key needs the chat
	// capability (or full tenant access), exactly like pinning.
	sessions := g.apiKeyGroup(r.Group("/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		sessions.POST("/:session_id/share", handler.ShareSession)
		sessions.DELETE("/:id/share", handler.UnshareSession)
	}
	// /shared is a new static top-level segment (no verb tree conflict with
	// the existing routes); :token is a fresh wildcard position.
	shared := g.apiKeyGroup(r.Group("/shared", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		shared.GET("/sessions/:token", handler.GetSharedSession)
	}
}

// RegisterSessionRoutes 注册路由。
//
// Sessions are per-user resources; the handler enforces user ownership.
// We gate at Viewer+ to keep non-members out once RBAC is on, matching
// the message routes above. A future refactor can introduce
// per-session ownership in the middleware layer the same way KB/agent
// routes do today.
func RegisterSessionRoutes(
	r *gin.RouterGroup,
	handler *session.Handler,
	suggestionHandler *handler.MessageSuggestionHandler,
	g *rbacGuards,
) {
	// Sessions are per-user chat state, not knowledge-base content. The
	// chat capability lets a scoped key run the full conversation flow
	// (create/manage its own sessions) without full tenant access.
	sessions := g.apiKeyGroup(r.Group("/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		sessions.POST("", handler.CreateSession)
		sessions.DELETE("/batch", handler.BatchDeleteSessions)
		sessions.GET("/:id", handler.GetSession)
		sessions.GET("", handler.GetSessionsByTenant)
		sessions.PUT("/:id", handler.UpdateSession)
		sessions.DELETE("/:id", handler.DeleteSession)
		sessions.DELETE("/:id/messages", handler.ClearSessionMessages)
		sessions.POST("/:session_id/generate_title", handler.GenerateTitle)
		sessions.POST("/:session_id/fork", handler.ForkSession)
		sessions.GET("/:id/local-browser", handler.BrowserSkillConnection)
		sessions.POST("/:session_id/local-browser", handler.BrowserSkillConnection)
		sessions.POST("/:session_id/attachments", handler.UploadTemporaryDocument)
		sessions.GET("/:id/attachments", handler.ListTemporaryDocuments)
		sessions.GET("/:id/attachments/:attachment_id", handler.GetTemporaryDocument)
		sessions.GET("/:id/attachments/:attachment_id/preview", handler.PreviewTemporaryDocument)
		sessions.DELETE("/:id/attachments/:attachment_id", handler.DeleteTemporaryDocument)
		sessions.POST("/:session_id/stop", handler.StopSession)
		// Durable tRPC run controls. These routes remain behind the chat API-key
		// capability and handlers enforce the persisted owner/tenant scope.
		sessions.GET("/:id/runs/:run_id", handler.GetAgentRun)
		sessions.GET("/:id/runs/:run_id/events", handler.GetAgentRunEvents)
		sessions.POST("/:session_id/runs/:run_id/decisions", handler.PostAgentRunDecision)
		sessions.POST("/:session_id/runs/:run_id/cancel", handler.CancelAgentRun)
		sessions.POST("/:session_id/sandbox/terminal-ticket", handler.IssueSandboxTerminalTicket)
		// Mid-run message injection: append a user message to the turn that is
		// currently generating. Accepts even when no run is live (the client
		// then falls back to a normal send), mirroring StopSession's ownership
		// rules.
		sessions.POST("/:session_id/steer", handler.SteerMessage)
		sessions.GET("/:id/steer", handler.ListSteerMessages)
		sessions.DELETE("/:id/steer/:steer_id", handler.DeleteSteerMessage)
		sessions.POST("/:session_id/steer/:steer_id/inject", handler.PromoteSteerMessage)
		// POST and DELETE share this path but gin maintains a separate radix tree
		// per HTTP verb, and the existing trees use different wildcard names
		// (POST uses :session_id, DELETE uses :id). Use whatever matches each
		// tree to avoid "wildcard conflicts" panic at route registration.
		sessions.POST("/:session_id/pin", handler.PinSession)
		sessions.DELETE("/:id/pin", handler.UnpinSession)
		// 继续接收活跃流
		sessions.GET("/continue-stream/:session_id", handler.ContinueStream)
		if suggestionHandler != nil {
			// Gin requires wildcard names to be identical within the same HTTP-method
			// radix tree. Existing GET session routes use :id, so keep that name here.
			sessions.GET("/:id/messages/:message_id/suggestions", suggestionHandler.Get)
			sessions.POST("/:session_id/messages/:message_id/suggestions", suggestionHandler.Ensure)
			sessions.POST("/:session_id/suggestion-events", suggestionHandler.RecordEvent)
		}

		// Skill-generated file artifacts. The list endpoints only expose
		// metadata; the actual bytes are streamed via /artifacts/:index/download
		// so the storage URL never appears on the wire.
		//
		// NOTE: gin builds a separate radix tree per HTTP verb but every
		// path in the same tree must share the same wildcard name. The GET
		// tree already binds :id via /sessions/:id (GetSession); reusing
		// :id here (instead of :session_id) avoids the
		// "wildcard conflicts" panic at route registration. The handlers
		// read the URL param via c.Param("session_id") with a fallback to
		// c.Param("id") for exactly this reason.
		sessions.GET("/:id/artifacts", handler.ListSessionArtifacts)
		sessions.GET("/:id/messages/:message_id/artifacts", handler.ListMessageArtifacts)
		sessions.GET("/:id/messages/:message_id/artifacts/:index/download", handler.DownloadMessageArtifact)

		// W26 immutable artifact version downloads. The explicit version ID
		// (instead of the message index above) keeps a regenerated message
		// from resolving a stale index to different bytes. Mounted only when
		// the container-level assembly registered a version source
		// (fail-closed: without wiring no route exists, like the craft
		// routes below). Same :id wildcard as the sibling GET routes.
		if artifactVersions := session.RegisteredArtifactVersionDownloadHandler(); artifactVersions != nil {
			sessions.GET("/:id/artifact-versions/:version_id/download", artifactVersions.DownloadArtifactVersion)
		}

		// W27 isolated artifact preview: ticket issuance runs on the MAIN
		// origin with this group's full auth chain (Viewer+ / chat API-key
		// capability); the opaque short-lived ticket is redeemed on the
		// ISOLATED preview origin, whose /ap route router.go mounts before
		// the global Auth middleware (craft preview shape). Mounted only
		// when the container-level assembly registered a handler
		// (fail-closed, like the versioned download above).
		if artifactPreview := handlerapi.RegisteredArtifactPreviewHandler(); artifactPreview != nil {
			handlerapi.RegisterArtifactPreviewIssueRoute(sessions, artifactPreview)
		}
	}

	// Craft workbench API (W03): the create/list group carries the same
	// Viewer+ guard and chat API-key capability as the sessions group, and
	// the per-session craft routes mount inside the sessions group itself so
	// they inherit its auth chain. Handlers come from the container-level
	// registration — when the craft assembly is not wired both are nil and
	// nothing is mounted (fail-closed, no silent shims).
	craftSessions := g.apiKeyGroup(r.Group("/craft/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	var craftHandler *session.CraftSessionHandler
	if api := session.RegisteredCraftSessionHandler(); api != nil {
		craftHandler = session.NewCraftSessionHandler(api)
	}
	session.RegisterCraftSessionRoutes(craftSessions, sessions, craftHandler, session.RegisteredCraftPreviewRouteHandler())

	// C02 interaction decide surface: mounted only when the interaction
	// assembly is wired (fail-closed, no 503 shims); it inherits the sessions
	// group auth chain.
	if interactionAPI := session.RegisteredCraftInteractionHandler(); interactionAPI != nil {
		session.RegisterCraftInteractionRoutes(sessions, session.NewCraftInteractionHandler(interactionAPI))
	}

	// O02 credential issuance plane (spec §3.4: /api/v1/craft/model-gateway/
	// credentials): its own group carries the same auth-chain shape as the
	// craft sessions table (post-Auth, Viewer+ and the chat API-key
	// capability — issuance needs the tenant context); mounted only when the
	// gateway is assembled (fail-closed, no 503 shims).
	if gw := handlerapi.RegisteredCraftModelGateway(); gw != nil {
		gwGroup := g.apiKeyGroup(r.Group("/craft/model-gateway", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
		gwGroup.POST("/credentials", gw.IssueCredential)
		gwGroup.POST("/credentials/revoke", gw.RevokeCredential)
	}

	// SP3 (C-23): the scheduled-task table (spec §3) — the same Viewer+
	// guard and chat API-key capability shape as the craft sessions group
	// above; per-task ownership (wrong owner, deleted and missing all read
	// as one 404) is enforced inside the service/store. Mounted only when
	// the container assembly registered the handler (fail-closed, no 503
	// shims).
	if scheduled := session.RegisteredCraftScheduledHandler(); scheduled != nil {
		scheduledTasks := g.apiKeyGroup(r.Group("/craft/scheduled-tasks", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
		session.RegisterCraftScheduledTaskRoutes(scheduledTasks, session.NewCraftScheduledHandler(scheduled))
	}
}

// RegisterChatRoutes 注册路由。Chat endpoints are tenant-member usage
// surfaces; Viewer+ is sufficient because per-session/per-agent
// authorisation is enforced inside the handlers.
func RegisterChatRoutes(r *gin.RouterGroup, handler *session.Handler, g *rbacGuards) {
	// These POST routes append messages and run generation, so a scoped key
	// needs the explicit chat capability unless it has full tenant access.
	knowledgeChat := g.apiKeyGroup(r.Group("/knowledge-chat", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		knowledgeChat.POST("/:session_id", handler.KnowledgeQA)
	}

	// Agent-based chat
	agentChat := g.apiKeyGroup(r.Group("/agent-chat", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		agentChat.POST("/:session_id", handler.AgentQA)
	}

	// 新增知识检索接口，不需要session_id
	knowledgeSearch := g.apiKeyGroup(r.Group("/knowledge-search", g.Viewer()), apiKeyRetrieve(apiKeyFullAccess()))
	{
		knowledgeSearch.POST("", handler.SearchKnowledge)
	}
}

// RegisterSandboxTerminalRoutes registers the interactive-terminal WebSocket.
//
// Like the IM callback routes this is registered BEFORE the global auth
// middleware: a browser WebSocket handshake cannot carry the
// Authorization / X-API-Key headers, so a short-lived session-bound ticket
// travels in the ticket query parameter. The handler authenticates itself
// via service.ParseSandboxTerminalTicket + CheckSandboxTerminalAuth +
// middleware.AttachAuthenticatedUser (not the 24h access JWT). The ticket
// is bound to the minting access-token id; the open PTY rechecks that
// token, user, membership, and session ownership about once a minute.
//
// The wildcard is :id because this GET joins the same radix tree as
// /sessions/:id (gin requires identical wildcard names per tree).
func RegisterSandboxTerminalRoutes(r *gin.Engine, sessionHandler *session.Handler) {
	r.GET("/api/v1/sessions/:id/sandbox/terminal", sessionHandler.SandboxTerminalWS)
}

// RegisterLocalBrowserRoutes mounts the extension-facing local-browser
// endpoints on the bare engine, before the global auth middleware. The
// WebSocket upgrade carries a device credential in its subprotocol (not the
// access JWT), and the authorize/internal endpoints authenticate via the
// BrowserSkill manager itself (one-use pairing tokens / HMAC-signed cluster
// RPC). The manager rejects anything unauthenticated, so these routes must
// stay out of the authed group.
func RegisterLocalBrowserRoutes(r *gin.Engine, sessionHandler *session.Handler) {
	r.GET("/api/v1/local-browser/extension", sessionHandler.BrowserSkillExtension)
	r.POST("/api/v1/local-browser/extension/authorize", sessionHandler.BrowserSkillAuthorize)
	r.POST("/api/v1/local-browser/internal", sessionHandler.BrowserSkillInternal)
}

// RegisterMyBrowserRoutes mounts the member's own browser-connection
// management endpoints (pairing link, status, revoke, extension download) on
// the authenticated group. The handler derives the member scope from the
// request context; normal auth and tenant membership apply upstream.
func RegisterMyBrowserRoutes(r *gin.RouterGroup, sessionHandler *session.Handler) {
	r.GET("/me/browser", sessionHandler.BrowserSkillAccount)
	r.POST("/me/browser", sessionHandler.BrowserSkillAccount)
	r.GET("/me/browser/extension", sessionHandler.BrowserSkillDownload)
}
