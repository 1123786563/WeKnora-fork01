package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/handler/dto"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// PluginHandler handles plugin-related HTTP requests. T02 lands the admin
// manifest preview; installation/discover/drift endpoints extend it later.
type PluginHandler struct {
	pluginService interfaces.PluginService
}

// NewPluginHandler creates a new plugin handler.
func NewPluginHandler(pluginService interfaces.PluginService) *PluginHandler {
	return &PluginHandler{pluginService: pluginService}
}

// PreviewManifest godoc
// @Summary      预览插件清单
// @Description  抓取并核验插件清单（weknora.plugin/1），返回带有效期与身份指纹的审阅数据；不创建安装
// @Tags         插件
// @Accept       json
// @Produce      json
// @Param        request  body      dto.PluginPreviewRequest  true  "清单 URL"
// @Success      200      {object}  map[string]interface{}    "预览结果"
// @Failure      400      {object}  errors.AppError           "清单 URL 非法或核验失败"
// @Failure      500      {object}  errors.AppError           "预览持久化失败"
// @Failure      503      {object}  errors.AppError           "清单抓取失败（上游不可达或异常状态）"
// @Security     Bearer
// @Router       /plugins/installations/preview [post]
func (h *PluginHandler) PreviewManifest(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.PluginPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse plugin preview request", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	actorRaw, _ := c.Get(types.UserIDContextKey.String())
	actorID, _ := actorRaw.(string)

	resp, err := h.pluginService.PreviewFromManifest(ctx, tenantID, actorID, req.ManifestURL)
	if err != nil {
		mapPluginPreviewError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    previewResponseDTO(resp),
	})
}

// previewResponseDTO maps the types-layer service result onto the HTTP DTO
// (整分支 OCR 一轮 F2): the service contract stays in the types layer and
// this handler owns the wire shape. Tools' scopes are copied, never aliased.
func previewResponseDTO(result *types.PluginPreviewResult) *dto.PluginPreviewResponse {
	if result == nil {
		return nil
	}
	tools := make([]dto.PluginPreviewTool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		// 跨任务转交 T01-R1-F1: make+copy (never append([]string(nil), ...))
		// keeps undeclared scopes as [] instead of nil — the wire field must
		// have ONE shape ("scopes":[]), never null, or TS consumers treating
		// it as string[] break at runtime (.map/.length on null).
		scopes := make([]string, len(tool.Scopes))
		copy(scopes, tool.Scopes)
		tools = append(tools, dto.PluginPreviewTool{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
		})
	}
	return &dto.PluginPreviewResponse{
		PreviewID:           result.PreviewID,
		PluginID:            result.PluginID,
		Version:             result.Version,
		Name:                result.Name,
		Description:         result.Description,
		TransportType:       result.TransportType,
		EndpointURL:         result.EndpointURL,
		Tools:               tools,
		IdentityFingerprint: result.IdentityFingerprint,
		ExpiresAt:           result.ExpiresAt,
	}
}

// mapPluginPreviewError maps service-layer preview failures onto HTTP
// verdicts. SSRF rejections and verification failures (invalid manifest,
// declaration mismatch, OAuth-protected endpoint) are deterministic
// rejections of the admin's input → 4xx. Manifest DOWNLOAD faults (DNS
// failure, egress timeout, upstream 5xx, proxy errors) are server-side
// network problems, not input problems → 503 with a fixed message; the raw
// transport error may carry internal proxy topology (HTTP(S)_PROXY) and only
// goes to the server log (跨任务转交 T01-R2-F2). Persistence faults → 500.
func mapPluginPreviewError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrManifestURLRejected):
		c.Error(errors.NewBadRequestError(err.Error()))
	case plugins.IsOAuthProtected(err):
		c.Error(errors.NewBadRequestError("插件服务要求授权才能核验工具目录：" + err.Error()))
	case stderrors.Is(err, plugins.ErrManifestFetchFailed):
		logger.Error(c.Request.Context(), "Plugin manifest fetch failed", err)
		c.Error(errors.NewServiceUnavailableError("插件清单抓取失败：清单服务不可达或返回异常状态，请稍后重试"))
	case stderrors.Is(err, service.ErrPreviewPersistFailed):
		c.Error(errors.NewInternalServerError(err.Error()))
	default:
		c.Error(errors.NewBadRequestError(err.Error()))
	}
}

// ConfirmInstallation godoc
// @Summary      确认安装插件
// @Description  消费一个已核验的预览，为空间安装固定版本：重核远端一致性、物化 MCP 服务、写入逐工具策略（写工具默认停用）并消费预览；失败自动补偿
// @Tags         插件
// @Accept       json
// @Produce      json
// @Param        request  body      dto.PluginInstallConfirmRequest  true  "预览 ID"
// @Success      200      {object}  map[string]interface{}           "安装结果"
// @Failure      400      {object}  errors.AppError                  "预览已消费/已过期/内容变化或请求非法"
// @Failure      404      {object}  errors.AppError                  "预览不存在"
// @Failure      409      {object}  errors.AppError                  "插件已安装（需走升级路径）"
// @Failure      500      {object}  errors.AppError                  "安装持久化或物化失败"
// @Failure      503      {object}  errors.AppError                  "确认时重抓清单失败"
// @Security     Bearer
// @Router       /plugins/installations [post]
func (h *PluginHandler) ConfirmInstallation(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.PluginInstallConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse plugin install confirm request", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	actorRaw, _ := c.Get(types.UserIDContextKey.String())
	actorID, _ := actorRaw.(string)

	resp, err := h.pluginService.ConfirmInstallation(ctx, tenantID, actorID, req.PreviewID)
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    installationResponseDTO(resp),
	})
}

// DisableInstallation godoc
// @Summary      停用插件安装
// @Description  停用一个已安装插件：安装状态与物化 MCP 服务同步停用，Agent 侧不再可见
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "更新后的安装"
// @Failure      400  {object}  errors.AppError         "状态非法"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "持久化失败"
// @Security     Bearer
// @Router       /plugins/installations/{id}/disable [post]
func (h *PluginHandler) DisableInstallation(c *gin.Context) {
	h.setInstallationState(c, types.PluginInstallationDisabled)
}

// EnableInstallation godoc
// @Summary      启用插件安装
// @Description  重新启用一个已停用的插件安装
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "更新后的安装"
// @Failure      400  {object}  errors.AppError         "状态非法"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "持久化失败"
// @Security     Bearer
// @Router       /plugins/installations/{id}/enable [post]
func (h *PluginHandler) EnableInstallation(c *gin.Context) {
	h.setInstallationState(c, types.PluginInstallationActive)
}

func (h *PluginHandler) setInstallationState(c *gin.Context, state string) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	resp, err := h.pluginService.SetInstallationState(ctx, tenantID, c.Param("id"), state)
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    installationResponseDTO(resp),
	})
}

// UninstallInstallation godoc
// @Summary      卸载插件安装
// @Description  彻底移除一个插件安装：硬级联删除物化 MCP 服务与逐工具策略行并释放 (tenant, plugin) 唯一槽——补偿残留的自愈入口；常规停用请用 disable
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "卸载完成"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "持久化失败"
// @Security     Bearer
// @Router       /plugins/installations/{id} [delete]
func (h *PluginHandler) UninstallInstallation(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	if err := h.pluginService.UninstallInstallation(ctx, tenantID, c.Param("id")); err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListInstallations godoc
// @Summary      列出本空间已安装插件
// @Description  成员发现面：返回本空间全部插件安装的摘要（版本/状态/漂移态/工具数），跨空间不可见
// @Tags         插件
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "安装摘要列表"
// @Failure      500  {object}  errors.AppError         "查询失败"
// @Security     Bearer
// @Router       /plugins/installations [get]
func (h *PluginHandler) ListInstallations(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	summaries, err := h.pluginService.ListInstallations(ctx, tenantID)
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	items := make([]dto.PluginInstallationSummary, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, dto.PluginInstallationSummary{
			InstallationID:       s.InstallationID,
			PluginID:             s.PluginID,
			Name:                 s.Name,
			Version:              s.Version,
			State:                s.State,
			DriftState:           s.DriftState,
			RequiresPersonalAuth: s.RequiresPersonalAuth,
			ToolCount:            s.ToolCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// GetInstallation godoc
// @Summary      查询插件安装详情
// @Description  返回本空间一个插件安装的完整视图（已接受版本、工具目录与启停策略）；跨空间 ID 返回 404
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "安装详情"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "查询失败"
// @Security     Bearer
// @Router       /plugins/installations/{id} [get]
func (h *PluginHandler) GetInstallation(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	resp, err := h.pluginService.GetInstallation(ctx, tenantID, c.Param("id"))
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    installationResponseDTO(resp),
	})
}

// installationResponseDTO maps the types-layer installation result onto the
// HTTP DTO. Tool scopes are copied, never aliased, and stay [] — one wire
// shape end to end (跨任务转交 T01-R1-F1 convention).
func installationResponseDTO(result *types.PluginInstallationResult) *dto.PluginInstallationResponse {
	if result == nil {
		return nil
	}
	tools := make([]dto.PluginInstallationTool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		scopes := make([]string, len(tool.Scopes))
		copy(scopes, tool.Scopes)
		var enabled *bool
		if tool.Enabled != nil {
			value := *tool.Enabled
			enabled = &value
		}
		tools = append(tools, dto.PluginInstallationTool{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
			Enabled:              enabled,
		})
	}
	return &dto.PluginInstallationResponse{
		InstallationID: result.InstallationID,
		PluginID:       result.PluginID,
		Name:           result.Name,
		Description:    result.Description,
		Version:        result.Version,
		State:          result.State,
		DriftState:     result.DriftState,
		TransportType:  result.TransportType,
		EndpointURL:    result.EndpointURL,
		ServiceID:      result.ServiceID,
		Tools:          tools,
	}
}

// GetMyConnection godoc
// @Summary      查询我的插件连接状态
// @Description  返回当前成员对一个插件安装的个人授权视图：三态（authorized/expired/unauthorized）、需个人授权工具清单，以及映射到物化 MCP 服务的既有 OAuth 授权/撤销端点路径；响应不含任何令牌材料
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "我的连接状态"
// @Failure      401  {object}  errors.AppError         "缺少身份上下文"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "查询失败"
// @Security     Bearer
// @Router       /plugins/installations/{id}/connections/me [get]
func (h *PluginHandler) GetMyConnection(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	principal, _ := mcpOAuthPrincipalsFromContext(c)
	if !principal.Valid() {
		c.Error(errors.NewUnauthorizedError("authentication required"))
		return
	}
	resp, err := h.pluginService.GetMyConnectionStatus(ctx, tenantID, c.Param("id"), principal)
	if err != nil {
		mapPluginConnectionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    myConnectionResponseDTO(resp),
	})
}

// myConnectionResponseDTO maps the types-layer connection view onto the HTTP
// DTO. RequiresAuthTools is copied, never aliased, and stays [] — one wire
// shape end to end (跨任务转交 T01-R1-F1 convention).
func myConnectionResponseDTO(result *types.PluginMyConnection) *dto.PluginMyConnection {
	if result == nil {
		return nil
	}
	tools := make([]string, len(result.RequiresAuthTools))
	copy(tools, result.RequiresAuthTools)
	return &dto.PluginMyConnection{
		InstallationID:       result.InstallationID,
		PluginID:             result.PluginID,
		Name:                 result.Name,
		ServiceID:            result.ServiceID,
		RequiresPersonalAuth: result.RequiresPersonalAuth,
		Authorized:           result.Authorized,
		State:                result.State,
		AuthorizeURLPath:     result.AuthorizeURLPath,
		RevokePath:           result.RevokePath,
		RequiresAuthTools:    tools,
	}
}

// PreviewUpgrade godoc
// @Summary      预览插件候选版本差异
// @Description  重抓安装行长期清单来源并核验当前声明的候选版本，返回与已接受版本相比的五维差异（新增/移除/schema/scope+读写+授权面/端点）与候选指纹；完全只读，不切换已接受版本、不改变任何安装状态
// @Tags         插件
// @Produce      json
// @Param        id   path  string  true  "安装 ID"
// @Success      200  {object}  map[string]interface{}  "差异预览结果"
// @Failure      400  {object}  errors.AppError         "候选核验失败（清单非法或声明不符）"
// @Failure      404  {object}  errors.AppError         "安装不存在"
// @Failure      500  {object}  errors.AppError         "查询失败"
// @Failure      503  {object}  errors.AppError         "候选清单抓取失败（不可达）"
// @Security     Bearer
// @Router       /plugins/installations/{id}/upgrade-preview [post]
func (h *PluginHandler) PreviewUpgrade(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	resp, err := h.pluginService.PreviewUpgrade(ctx, tenantID, c.Param("id"))
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    upgradePreviewResponseDTO(resp),
	})
}

// upgradePreviewResponseDTO maps the types-layer upgrade preview onto the
// HTTP DTO (the service contract stays in the types layer; this handler owns
// the wire shape). Snapshot scopes are copied, never aliased, and stay [] —
// one wire shape end to end (跨任务转交 T01-R1-F1 convention).
func upgradePreviewResponseDTO(result *types.PluginUpgradePreviewResult) *dto.PluginUpgradePreviewResponse {
	if result == nil {
		return nil
	}
	return &dto.PluginUpgradePreviewResponse{
		Diff:                 versionDiffDTO(result.Diff),
		CandidateFingerprint: result.CandidateFingerprint,
		CandidateToolsDigest: result.CandidateToolsDigest,
	}
}

func versionDiffDTO(diff types.PluginVersionDiff) dto.PluginVersionDiffDTO {
	added := make([]dto.PluginToolSnapshotDTO, 0, len(diff.AddedTools))
	for _, tool := range diff.AddedTools {
		added = append(added, toolSnapshotDTO(tool))
	}
	removed := make([]dto.PluginToolSnapshotDTO, 0, len(diff.RemovedTools))
	for _, tool := range diff.RemovedTools {
		removed = append(removed, toolSnapshotDTO(tool))
	}
	changed := make([]dto.PluginToolChangeDTO, 0, len(diff.ChangedTools))
	for _, change := range diff.ChangedTools {
		changed = append(changed, dto.PluginToolChangeDTO{
			Name:                  change.Name,
			SchemaChanged:         change.SchemaChanged,
			ScopeChanged:          change.ScopeChanged,
			ReadWriteClassChanged: change.ReadWriteClassChanged,
			PersonalAuthChanged:   change.PersonalAuthChanged,
			Current:               toolSnapshotDTO(change.Current),
			Candidate:             toolSnapshotDTO(change.Candidate),
		})
	}
	return dto.PluginVersionDiffDTO{
		PluginID:          diff.PluginID,
		CurrentVersion:    diff.CurrentVersion,
		CandidateVersion:  diff.CandidateVersion,
		IsDowngrade:       diff.IsDowngrade,
		EndpointChanged:   diff.EndpointChanged,
		CurrentEndpoint:   diff.CurrentEndpoint,
		CandidateEndpoint: diff.CandidateEndpoint,
		AddedTools:        added,
		RemovedTools:      removed,
		ChangedTools:      changed,
	}
}

func toolSnapshotDTO(tool types.PluginToolSnapshot) dto.PluginToolSnapshotDTO {
	scopes := make([]string, len(tool.Scopes))
	copy(scopes, tool.Scopes)
	return dto.PluginToolSnapshotDTO{
		Name:                 tool.Name,
		Description:          tool.Description,
		InputSchemaDigest:    tool.InputSchemaDigest,
		ReadOnly:             tool.ReadOnly,
		RequiresPersonalAuth: tool.RequiresPersonalAuth,
		Scopes:               scopes,
	}
}

// AcceptUpgrade godoc
// @Summary      接受插件候选版本升级
// @Description  重抓安装行长期清单来源并核验：远端当前身份指纹必须等于预览时返回的候选指纹（防「预览后远端又变」），通过后切换已接受版本（安装行快照/端点/版本 + 漂移重置）、同步物化服务仅切 URL（Name/ID 不变）并为新增工具增量写策略行（只读启用/写停用，既有决定保留）；失败补偿回写保旧版；同指纹重复接受幂等
// @Tags         插件
// @Accept       json
// @Produce      json
// @Param        id       path  string                             true  "安装 ID"
// @Param        request  body  dto.PluginUpgradeAcceptRequest     true  "预览返回的候选指纹"
// @Success      200      {object}  map[string]interface{}         "切换后的安装"
// @Failure      400      {object}  errors.AppError                "候选核验失败或请求非法"
// @Failure      404      {object}  errors.AppError                "安装不存在"
// @Failure      409      {object}  errors.AppError                "候选已变化（预览后远端又变，需重新预览）"
// @Failure      500      {object}  errors.AppError                "持久化或物化失败（已补偿回旧版）"
// @Failure      503      {object}  errors.AppError                "候选清单抓取失败（不可达）"
// @Security     Bearer
// @Router       /plugins/installations/{id}/upgrade-accept [post]
func (h *PluginHandler) AcceptUpgrade(c *gin.Context) {
	ctx := c.Request.Context()

	var req dto.PluginUpgradeAcceptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse plugin upgrade accept request", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		logger.Error(ctx, "Tenant ID is empty")
		c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	actorRaw, _ := c.Get(types.UserIDContextKey.String())
	actorID, _ := actorRaw.(string)

	resp, err := h.pluginService.AcceptUpgrade(ctx, tenantID, actorID, c.Param("id"), req.CandidateFingerprint)
	if err != nil {
		mapPluginInstallationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    installationResponseDTO(resp),
	})
}

// mapPluginConnectionError maps the connection view's service failures onto
// HTTP verdicts: a foreign/absent installation is a flat 404 (no existence
// leak); a missing principal context is 401; token-store faults are 5xx with
// sentinel-only text (driver internals stay server-side).
func mapPluginConnectionError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrInstallationNotFound):
		c.Error(errors.NewNotFoundError(err.Error()))
	case stderrors.Is(err, service.ErrConnectionPrincipalRequired):
		c.Error(errors.NewUnauthorizedError(err.Error()))
	case stderrors.Is(err, service.ErrInstallationPersistFailed),
		stderrors.Is(err, service.ErrConnectionQueryFailed):
		logger.Error(c.Request.Context(), "Plugin connection status query failed", err)
		c.Error(errors.NewInternalServerError("查询插件连接状态失败：服务端内部故障，请稍后重试"))
	default:
		logger.Error(c.Request.Context(), "Unmapped plugin connection error", err)
		c.Error(errors.NewInternalServerError("查询插件连接状态失败：服务端内部故障，请稍后重试"))
	}
}

// mapPluginInstallationError maps install-slice service failures onto HTTP
// verdicts. Deterministic rejections of the confirm input (consumed /
// expired / changed preview, duplicate install) are 4xx with the sentinel
// semantics; not-found is 404 without existence leaks; fetch faults during
// re-verification are 503 (fixed message, details server-side only — same
// hygiene as mapPluginPreviewError); persistence/materialization faults
// after compensation are 5xx with sentinel-only text.
func mapPluginInstallationError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrPluginPreviewNotFound),
		stderrors.Is(err, service.ErrInstallationNotFound):
		c.Error(errors.NewNotFoundError(err.Error()))
	case stderrors.Is(err, service.ErrPreviewAlreadyConsumed),
		stderrors.Is(err, service.ErrPreviewExpired),
		stderrors.Is(err, service.ErrPreviewContentChanged),
		stderrors.Is(err, service.ErrInstallationStateInvalid),
		stderrors.Is(err, service.ErrPluginVerifyFailed):
		c.Error(errors.NewBadRequestError(err.Error()))
	case stderrors.Is(err, service.ErrPluginAlreadyInstalled),
		// ErrUpgradeCandidateChanged (T16): the remote candidate moved on
		// since the preview — a state conflict resolved by a fresh preview,
		// not a malformed request (the fingerprint WAS the preview's own).
		stderrors.Is(err, service.ErrUpgradeCandidateChanged):
		c.Error(errors.NewConflictError(err.Error()))
	case stderrors.Is(err, plugins.ErrManifestFetchFailed):
		logger.Error(c.Request.Context(), "Plugin manifest re-fetch failed on confirm", err)
		c.Error(errors.NewServiceUnavailableError("插件清单抓取失败：清单服务不可达或返回异常状态，请稍后重试"))
	case plugins.IsOAuthProtected(err):
		c.Error(errors.NewBadRequestError(err.Error()))
	case stderrors.Is(err, service.ErrPreviewPersistFailed),
		stderrors.Is(err, service.ErrInstallationPersistFailed),
		stderrors.Is(err, service.ErrInstallationMaterializeFailed),
		stderrors.Is(err, service.ErrUpgradeWriterNotWired):
		// ErrPreviewPersistFailed joins the 5xx family (OCR round-1 R12 F02):
		// step 1's GetPreview repo fault surfaces this sentinel and it was
		// previously missing from the table — a server-side fault misread
		// as a 400 with the sentinel text in the body.
		c.Error(errors.NewInternalServerError(err.Error()))
	default:
		// Unmapped errors are conservatively SERVER-side (OCR round-1 R12 F01):
		// e.g. a non-NotFound MarkPreviewConsumed repo fault reaches here as a
		// raw driver error after compensation — its text (table names,
		// connection internals) must never reach the response body, and the
		// ConfirmInstallation godoc's @Failure 500 contract holds. Details go
		// to the server log only.
		logger.Error(c.Request.Context(), "Unmapped plugin installation error", err)
		c.Error(errors.NewInternalServerError("插件安装失败：服务端内部故障，请稍后重试"))
	}
}
