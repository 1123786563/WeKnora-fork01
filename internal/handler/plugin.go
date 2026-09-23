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
