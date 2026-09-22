package session

import (
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/gin-gonic/gin"
)

const nativeArchiveHTTPMaxLimit = 100

// NativeArchiveHandler exposes only the three archive read operations. The
// ScopeResolver is deliberately injected: request fields never choose tenant
// or owner scope.
type NativeArchiveHandler struct {
	reader nativecontract.ArchiveReader
	scopes nativecontract.ScopeResolver
}

func NewNativeArchiveHandler(reader nativecontract.ArchiveReader, scopes nativecontract.ScopeResolver) *NativeArchiveHandler {
	return &NativeArchiveHandler{reader: reader, scopes: scopes}
}

func (h *NativeArchiveHandler) List(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	query, err := nativeArchiveHTTPQuery(c)
	if err != nil {
		writeNativeArchiveFailure(c, err)
		return
	}
	page, err := h.reader.List(c.Request.Context(), scope, query)
	if err != nil {
		writeNativeArchiveFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": page})
}

func (h *NativeArchiveHandler) GetRecord(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	record, err := h.reader.Read(c.Request.Context(), scope, strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeNativeArchiveFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": record})
}

func (h *NativeArchiveHandler) GetArtifact(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	artifact, err := h.reader.ReadArtifact(c.Request.Context(), scope, strings.TrimSpace(c.Param("id")))
	if err != nil {
		writeNativeArchiveFailure(c, err)
		return
	}
	// The storage location is intentionally absent. Clients receive stable
	// integrity metadata and an opaque reference for a future authorized
	// download capability, never a provider URL or object key.
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"artifact":           nativecontract.PublicArtifact(artifact),
		"download_reference": gin.H{"artifact_id": artifact.ID},
	}})
}

func (h *NativeArchiveHandler) scope(c *gin.Context) (nativecontract.Scope, bool) {
	if h == nil || h.reader == nil || h.scopes == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return nativecontract.Scope{}, false
	}
	scope, err := h.scopes.Resolve(c.Request.Context())
	if err != nil || scope.TenantID == 0 || strings.TrimSpace(scope.SessionOwnerID) == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": nativecontract.ErrForbidden, "error": "archive access was revoked"})
		return nativecontract.Scope{}, false
	}
	// Archive records are historical, but access is not historical.  Recheck
	// immediately before each reader call so a membership/resource revocation
	// between authentication and this endpoint cannot be used to enumerate an
	// old session or artifact.  The tenant and session owner are immutable
	// request bindings; only the resolver may refresh policy/grant metadata.
	current, err := h.scopes.Recheck(c.Request.Context(), scope, nil)
	if err != nil || current.TenantID != scope.TenantID || current.SessionOwnerID != scope.SessionOwnerID {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": nativecontract.ErrForbidden, "error": "archive access was revoked"})
		return nativecontract.Scope{}, false
	}
	return current, true
}

func nativeArchiveHTTPQuery(c *gin.Context) (nativecontract.ArchiveQuery, error) {
	query := nativecontract.ArchiveQuery{SessionID: strings.TrimSpace(c.Query("session_id")), Kind: strings.TrimSpace(c.Query("kind")), Cursor: strings.TrimSpace(c.Query("cursor")), Limit: nativeArchiveHTTPMaxLimit}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return nativecontract.ArchiveQuery{}, &nativecontract.Failure{Code: nativecontract.ErrInvalid, Message: "archive limit is invalid"}
		}
		query.Limit = limit
	}
	if query.Limit > nativeArchiveHTTPMaxLimit {
		query.Limit = nativeArchiveHTTPMaxLimit
	}
	return query, nil
}

func writeNativeArchiveFailure(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := nativecontract.ErrStore
	var failure *nativecontract.Failure
	if stderrors.As(err, &failure) {
		code = failure.Code
		switch failure.Code {
		case nativecontract.ErrForbidden:
			status = http.StatusForbidden
		case nativecontract.ErrNotFound:
			status = http.StatusNotFound
		case nativecontract.ErrInvalid:
			status = http.StatusBadRequest
		case nativecontract.ErrArchiveReadOnly:
			status = http.StatusConflict
		}
	}
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "error": err.Error()})
}

// RejectMutation is deliberately broad: every non-GET request in the archive
// namespace receives the frozen wire error instead of accidentally growing a
// write route as the old protocol evolves.
func (h *NativeArchiveHandler) RejectMutation(c *gin.Context) {
	writeNativeArchiveFailure(c, &nativecontract.Failure{
		Code:    nativecontract.ErrArchiveReadOnly,
		Message: "历史记录仅供查询",
		Effect:  nativecontract.EffectNotDispatched,
	})
}
