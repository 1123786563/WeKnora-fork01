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
		scopes := append([]string(nil), tool.Scopes...)
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
// declaration mismatch, OAuth-protected endpoint, unreachable endpoint) are
// deterministic rejections of the admin's input → 4xx; only persistence
// faults are server errors.
func mapPluginPreviewError(c *gin.Context, err error) {
	switch {
	case stderrors.Is(err, service.ErrManifestURLRejected):
		c.Error(errors.NewBadRequestError(err.Error()))
	case plugins.IsOAuthProtected(err):
		c.Error(errors.NewBadRequestError("插件服务要求授权才能核验工具目录：" + err.Error()))
	case stderrors.Is(err, service.ErrPreviewPersistFailed):
		c.Error(errors.NewInternalServerError(err.Error()))
	default:
		c.Error(errors.NewBadRequestError(err.Error()))
	}
}
