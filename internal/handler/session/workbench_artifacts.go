package session

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/workbench"
	"github.com/gin-gonic/gin"
)

// ArtifactRefReader resolves a session's artifacts together with the message
// binding signed download grants are pinned to.
type ArtifactRefReader interface {
	GetSessionArtifactRefs(ctx context.Context, sessionID string) ([]types.SessionArtifactRef, error)
}

// WorkbenchArtifactHandler serves the mobile execution-artifact surfaces:
// a metadata list scoped to an owned run, and short-lived signed download
// links. Storage URLs never leave the server; the signed link carries an
// HMAC grant (tenant/session/message/index/expiry) instead of credentials.
type WorkbenchArtifactHandler struct {
	runs       OwnedRunReader
	refs       ArtifactRefReader
	signingKey func() ([]byte, error)
	ttl        time.Duration
}

func NewWorkbenchArtifactHandler(runs OwnedRunReader, refs ArtifactRefReader) *WorkbenchArtifactHandler {
	return &WorkbenchArtifactHandler{
		runs:       runs,
		refs:       refs,
		signingKey: workbench.ArtifactSigningKeyFromEnv,
		ttl:        workbench.MaxArtifactGrantTTL,
	}
}

// workbenchArtifactItem is the wire shape of one list entry. It mirrors what
// the mobile contracts decoder accepts (items envelope, name/mime/size) and
// adds the session-wide index used to address the artifact on the signed-URL
// endpoint.
type workbenchArtifactItem struct {
	Index     int    `json:"index"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mime      string `json:"mime"`
	Version   string `json:"version"`
	Size      int64  `json:"size"`
	SourceRun string `json:"source_run"`
	CreatedAt any    `json:"created_at"`
}

func artifactListItemFromRef(runID string, position int, ref types.SessionArtifactRef) workbenchArtifactItem {
	contentType := strings.TrimSpace(ref.Artifact.FileType)
	if contentType != "" && contentType[0] == '.' {
		if ct := mime.TypeByExtension(strings.ToLower(contentType)); ct != "" {
			contentType = ct
		} else {
			contentType = "application/octet-stream"
		}
	} else if contentType == "" {
		contentType = "application/octet-stream"
	}
	return workbenchArtifactItem{
		Index:     position,
		ID:        ref.MessageID + ":" + strconv.Itoa(ref.Index),
		Name:      ref.Artifact.FileName,
		Mime:      contentType,
		Version:   "1",
		Size:      ref.Artifact.FileSize,
		SourceRun: runID,
		CreatedAt: ref.Artifact.CreatedAt,
	}
}

// ListWorkbenchArtifacts godoc
// @Summary      列出执行生成的成果文件
// @Description  按所属执行聚合会话中 assistant 消息产生的成果元数据（不含存储地址）；仅返回元数据，字节流须经签名链接下载
// @Tags         工作台
// @Produce      json
// @Param        run_id  path  string  true  "执行ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Security     Bearer
// @Router       /workbench/executions/{run_id}/artifacts [get]
func (h *WorkbenchArtifactHandler) ListWorkbenchArtifacts(c *gin.Context) {
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	refs, err := h.refs.GetSessionArtifactRefs(c.Request.Context(), run.SessionID)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	items := make([]workbenchArtifactItem, 0, len(refs))
	for position, ref := range refs {
		items = append(items, artifactListItemFromRef(run.Key.RunID, position, ref))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

// CreateWorkbenchArtifactSignedURL godoc
// @Summary      为成果文件签发短时效下载链接
// @Description  校验执行归属后签发 HMAC 签名的一次性时效下载链接（默认 15 分钟内有效）；链接不携带登录态，过期后客户端应重新走本端点授权
// @Tags         工作台
// @Produce      json
// @Param        run_id  path  string  true  "执行ID"
// @Param        index   path  int     true  "成果在会话列表中的序号"
// @Param        ttl_seconds  query  int  false  "有效期秒数（上限 900）"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      501  {object}  errors.AppError
// @Security     Bearer
// @Router       /workbench/executions/{run_id}/artifacts/{index}/signed-url [post]
func (h *WorkbenchArtifactHandler) CreateWorkbenchArtifactSignedURL(c *gin.Context) {
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	refs, err := h.refs.GetSessionArtifactRefs(c.Request.Context(), run.SessionID)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	if index >= len(refs) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	secret, keyErr := h.signingKey()
	if keyErr != nil {
		// Capability not configured on this deployment: report honestly
		// instead of minting links with a default secret.
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"code":    "artifact_signing_disabled",
			"error":   "artifact signing key not configured",
		})
		return
	}
	ttl := h.ttl
	if raw := strings.TrimSpace(c.Query("ttl_seconds")); raw != "" {
		if requested, parseErr := strconv.Atoi(raw); parseErr == nil && requested > 0 {
			ttl = time.Duration(requested) * time.Second
		}
	}
	if ttl > workbench.MaxArtifactGrantTTL || ttl <= 0 {
		ttl = workbench.MaxArtifactGrantTTL
	}
	ref := refs[index]
	expiresAt := time.Now().Add(ttl).UTC()
	grant := workbench.ArtifactGrant{
		TenantID:  run.Key.TenantID,
		SessionID: run.SessionID,
		MessageID: ref.MessageID,
		Index:     ref.Index,
		ExpiresAt: expiresAt.Unix(),
	}
	signature, signErr := workbench.SignArtifactGrant(secret, grant)
	if signErr != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": signErr.Error()})
		return
	}
	values := url.Values{}
	values.Set("tenant_id", strconv.FormatUint(grant.TenantID, 10))
	values.Set("session_id", grant.SessionID)
	values.Set("message_id", grant.MessageID)
	values.Set("index", strconv.Itoa(grant.Index))
	values.Set("expires_at", strconv.FormatInt(grant.ExpiresAt, 10))
	values.Set("signature", signature)
	link := externalURLBase(c) + "/api/v1/workbench/artifacts/download?" + values.Encode()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url":        link,
			"expires_at": expiresAt.Format(time.RFC3339),
			"artifact":   artifactListItemFromRef(run.Key.RunID, index, ref),
		},
	})
}

// externalURLBase derives the scheme://host the client used to reach this
// deployment, honouring the standard forwarding headers so signed links stay
// valid behind TLS-terminating proxies. It never inspects or echoes query
// parameters.
func externalURLBase(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
		first := strings.SplitN(forwarded, ",", 2)[0]
		if first == "http" || first == "https" {
			scheme = strings.TrimSpace(first)
		}
	}
	host := c.GetHeader("Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}
