package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/handler/dto"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// DataSourceHandler handles HTTP requests for data source management
type DataSourceHandler struct {
	service   interfaces.DataSourceService
	kbService interfaces.KnowledgeBaseService
}

// NewDataSourceHandler creates a new data source handler
func NewDataSourceHandler(
	service interfaces.DataSourceService,
	kbService interfaces.KnowledgeBaseService,
) *DataSourceHandler {
	return &DataSourceHandler{
		service:   service,
		kbService: kbService,
	}
}

// getTenantID safely extracts and validates tenant ID from context
// Returns 0 if tenant ID is not found (caller should return 401)
func (h *DataSourceHandler) getTenantID(c *gin.Context) uint64 {
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	return tenantID
}

// Data source settings contain connector credentials, so only the owning tenant can access them.
func (h *DataSourceHandler) getOwnedKnowledgeBase(
	ctx context.Context,
	tenantID uint64,
	kbID string,
) (*types.KnowledgeBase, int, string) {
	if kbID == "" {
		return nil, http.StatusBadRequest, "kb_id is required"
	}

	kb, err := h.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil || kb == nil {
		return nil, http.StatusNotFound, "knowledge base not found"
	}

	if kb.TenantID != tenantID {
		return nil, http.StatusForbidden, "access denied"
	}
	if err := types.AuthorizeTenantAPIKeyKnowledgeBases(ctx, kbID); err != nil {
		return nil, http.StatusForbidden, err.Error()
	}

	return kb, http.StatusOK, ""
}

func (h *DataSourceHandler) getOwnedDataSource(
	ctx context.Context,
	tenantID uint64,
	id string,
) (*types.DataSource, int, string) {
	ds, err := h.service.GetDataSource(ctx, id)
	if err != nil {
		return nil, http.StatusNotFound, "data source not found"
	}

	if _, status, msg := h.getOwnedKnowledgeBase(ctx, tenantID, ds.KnowledgeBaseID); status != http.StatusOK {
		return nil, status, msg
	}

	return ds, http.StatusOK, ""
}

// CreateDataSource godoc
// @Summary Create a new data source
// @Description Create a new data source configuration for a knowledge base
// @Tags DataSource
// @Accept json
// @Produce json
// @Param request body types.DataSource true "Data source configuration"
// @Success 201 {object} types.DataSource
// @Failure 400 {object} map[string]string
// @Router /datasource [post]
func (h *DataSourceHandler) CreateDataSource(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract tenant ID from context (set by auth middleware)
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
		return
	}

	var req types.DataSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if _, status, msg := h.getOwnedKnowledgeBase(ctx, tenantID, req.KnowledgeBaseID); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	// Enforce tenant isolation
	req.TenantID = tenantID

	ds, err := h.service.CreateDataSource(ctx, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, dto.NewDataSourceResponse(ds))
}

// GetDataSource godoc
// @Summary Get a data source by ID
// @Description Retrieve a data source configuration by ID
// @Tags DataSource
// @Produce json
// @Param id path string true "Data source ID"
// @Success 200 {object} types.DataSource
// @Failure 404 {object} map[string]string
// @Router /datasource/{id} [get]
func (h *DataSourceHandler) GetDataSource(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	ds, status, msg := h.getOwnedDataSource(ctx, tenantID, id)
	if status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, dto.NewDataSourceResponse(ds))
}

// ListDataSources godoc
// @Summary List data sources for a knowledge base
// @Description List all data sources for a specific knowledge base
// @Tags DataSource
// @Produce json
// @Param kb_id query string true "Knowledge base ID"
// @Success 200 {object} []types.DataSource
// @Failure 400 {object} map[string]string
// @Router /datasource [get]
func (h *DataSourceHandler) ListDataSources(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract tenant ID from context
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: workspace context missing"})
		return
	}

	kbID := c.Query("kb_id")
	if _, status, msg := h.getOwnedKnowledgeBase(ctx, tenantID, kbID); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	dataSources, err := h.service.ListDataSources(ctx, kbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list data sources"})
		return
	}

	if dataSources == nil {
		dataSources = make([]*types.DataSource, 0)
	}
	c.JSON(http.StatusOK, dto.NewDataSourceResponses(dataSources))
}

// UpdateDataSource godoc
// @Summary Update a data source
// @Description Update an existing data source configuration
// @Tags DataSource
// @Accept json
// @Produce json
// @Param id path string true "Data source ID"
// @Param request body types.DataSource true "Updated configuration"
// @Success 200 {object} types.DataSource
// @Failure 400 {object} map[string]string
// @Router /datasource/{id} [put]
func (h *DataSourceHandler) UpdateDataSource(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	var req types.DataSource
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	existing, status, msg := h.getOwnedDataSource(ctx, tenantID, id)
	if status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	req.ID = id
	req.TenantID = existing.TenantID
	req.KnowledgeBaseID = existing.KnowledgeBaseID
	ds, err := h.service.UpdateDataSource(ctx, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.NewDataSourceResponse(ds))
}

// DeleteDataSource godoc
// @Summary Delete a data source
// @Description Delete a data source (soft delete). With purge_documents=true the synced documents are purged asynchronously; without it they are kept (pre-SP2-a behavior).
// @Tags DataSource
// @Param id path string true "Data source ID"
// @Param purge_documents query boolean false "Cascade-delete every document the source synced (async, best-effort)"
// @Success 204
// @Failure 404 {object} map[string]string
// @Router /datasource/{id} [delete]
func (h *DataSourceHandler) DeleteDataSource(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	// purge_documents=true (SP2-a spec §4.1) cascades the delete to every
	// document the source synced into its KB; anything else — absent, false,
	// lookalike values — keeps the legacy keep-documents behavior (fully
	// backward compatible). The purge itself is async and best-effort: this
	// 204 only means the data source delete is durable and the purge task was
	// accepted. An enqueue failure keeps the documents; it is recorded in the
	// data_source_deleted audit as purge_documents="enqueue_failed" (and
	// logged), not surfaced in this response — there is deliberately no
	// progress endpoint (YAGNI, spec §4.1).
	purgeDocuments := c.Query("purge_documents") == "true"
	if err := h.service.DeleteDataSource(ctx, id, purgeDocuments); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete data source"})
		return
	}

	c.Status(http.StatusNoContent)
}

// CountDocuments godoc
// @Summary Count a data source's synced documents
// @Description Count the live documents one data source synced into its knowledge base — the number the delete-source confirmation dialog shows before the purge choice.
// @Tags DataSource
// @Produce json
// @Param id path string true "Data source ID"
// @Success 200 {object} map[string]int64
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /datasource/{id}/documents-count [get]
func (h *DataSourceHandler) CountDocuments(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	count, err := h.service.CountDataSourceDocuments(ctx, tenantID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// ValidateConnection godoc
// @Summary Test data source connection
// @Description Validate the connection to an external data source
// @Tags DataSource
// @Param id path string true "Data source ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/validate [post]
func (h *DataSourceHandler) ValidateConnection(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	if err := h.service.ValidateConnection(ctx, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

// ValidateCredentials godoc
// @Summary Test connection with raw credentials (no persistence)
// @Description Validate connectivity to an external data source using type + credentials
//
//	without creating or updating any database records.
//	Used by the frontend "Test Connection" button during data source creation.
//
// @Tags DataSource
// @Accept json
// @Produce json
// @Param request body object true "type and credentials"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /datasource/validate-credentials [post]
func (h *DataSourceHandler) ValidateCredentials(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Type        string                 `json:"type" binding:"required"`
		Credentials map[string]interface{} `json:"credentials" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: type and credentials are required"})
		return
	}

	if err := h.service.ValidateCredentials(ctx, req.Type, req.Credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "connected"})
}

// @Summary List available resources in data source
// @Description List resources available for sync in the external system. Pass parent_id to lazily load the direct children of a resource (used for large hierarchical sources such as Feishu wiki).
// @Tags DataSource
// @Produce json
// @Param id path string true "Data source ID"
// @Param parent_id query string false "Parent resource ExternalID; empty lists the top level"
// @Success 200 {object} []types.Resource
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/resources [get]
func (h *DataSourceHandler) ListAvailableResources(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")
	parentID := c.Query("parent_id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	resources, err := h.service.ListAvailableResources(ctx, id, parentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if resources == nil {
		resources = make([]types.Resource, 0)
	}
	c.JSON(http.StatusOK, resources)
}

// @Summary Resolve resource ancestors
// @Description Resolve the ancestor ExternalIDs that must be expanded to reveal the given (possibly deeply nested) resources in a lazily-loaded picker. Used to restore an existing selection when editing a data source.
// @Tags DataSource
// @Accept json
// @Produce json
// @Param id path string true "Data source ID"
// @Param request body resolveAncestorsRequest true "Resource IDs to resolve"
// @Success 200 {object} map[string][]string
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/resource-ancestors [post]
func (h *DataSourceHandler) ResolveResourceAncestors(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	var req resolveAncestorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ancestors, err := h.service.ResolveResourceAncestors(ctx, id, req.ResourceIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if ancestors == nil {
		ancestors = make([]string, 0)
	}
	c.JSON(http.StatusOK, gin.H{"ancestors": ancestors})
}

// resolveAncestorsRequest is the body for ResolveResourceAncestors.
type resolveAncestorsRequest struct {
	ResourceIDs []string `json:"resource_ids"`
}

// manualSyncRequest is the optional body of ManualSync. An absent or empty
// body is legal and means a regular (cursor-respecting) sync.
type manualSyncRequest struct {
	// ForceFull drops the persisted cursor and reconciles the whole source.
	ForceFull bool `json:"force_full"`
}

// reindexRequest is the body of ReindexItems (SP2-b §5.3).
type reindexRequest struct {
	// ExternalIDs lists the source items to refetch; a non-empty list is
	// required (an empty list is a client mistake, answered 400).
	ExternalIDs []string `json:"external_ids"`
	// RequestID makes the enqueue idempotent when non-empty: a repeat with the
	// same id while the run is still queued is rejected as a duplicate (409)
	// instead of queueing the items twice.
	RequestID string `json:"request_id"`
}

// ReindexItems godoc
// @Summary Retry specific items (scoped reindex)
// @Description Schedule a targeted reindex run that refetches only the listed external ids. A repeated request_id is rejected as a duplicate while the first run is still queued.
// @Tags DataSource
// @Accept json
// @Param id path string true "Data source ID"
// @Param request body reindexRequest true "{\"external_ids\": [\"...\"], \"request_id\": \"...\"}"
// @Success 202 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /datasource/{id}/reindex [post]
func (h *DataSourceHandler) ReindexItems(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	var req reindexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.ExternalIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "external_ids must be a non-empty list"})
		return
	}

	syncLogID, err := h.service.ReindexItems(ctx, id, req.ExternalIDs, req.RequestID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrReindexDuplicateRequest):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, datasource.ErrDataSourceNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "data source not found"})
		default:
			// Mirrors ManualSync: pause/authorization rejections and queue
			// outages surface as their message on a 400.
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"sync_log_id": syncLogID})
}

// ManualSync godoc
// @Summary Trigger immediate sync
// @Description Trigger an immediate sync for a data source
// @Tags DataSource
// @Accept json
// @Param id path string true "Data source ID"
// @Param request body manualSyncRequest false "Optional: {\"force_full\": bool} — defaults to false"
// @Success 200 {object} types.SyncLog
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/sync [post]
func (h *DataSourceHandler) ManualSync(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	// The body is optional: empty/absent payloads keep the legacy behaviour.
	var req manualSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	syncLog, err := h.service.ManualSync(ctx, id, req.ForceFull)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, syncLog)
}

// PauseDataSource godoc
// @Summary Pause data source
// @Description Pause a data source's scheduled syncs
// @Tags DataSource
// @Param id path string true "Data source ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/pause [post]
func (h *DataSourceHandler) PauseDataSource(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	if err := h.service.PauseDataSource(ctx, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// ResumeDataSource godoc
// @Summary Resume data source
// @Description Resume a paused data source
// @Tags DataSource
// @Param id path string true "Data source ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/resume [post]
func (h *DataSourceHandler) ResumeDataSource(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	if err := h.service.ResumeDataSource(ctx, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

// GetSyncLogs godoc
// @Summary Get sync logs
// @Description Retrieve sync history for a data source
// @Tags DataSource
// @Produce json
// @Param id path string true "Data source ID"
// @Param limit query int false "Limit (default: 10)"
// @Param offset query int false "Offset (default: 0)"
// @Success 200 {object} []types.SyncLog
// @Failure 400 {object} map[string]string
// @Router /datasource/{id}/logs [get]
func (h *DataSourceHandler) GetSyncLogs(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	limit := 10
	offset := 0

	if l := c.Query("limit"); l != "" {
		v, err := strconv.Atoi(l)
		if err != nil || v <= 0 || v > maxListPageSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and " + strconv.Itoa(maxListPageSize)})
			return
		}
		limit = v
	}

	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	logs, err := h.service.GetSyncLogs(ctx, id, limit, offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if logs == nil {
		logs = make([]*types.SyncLog, 0)
	}
	c.JSON(http.StatusOK, logs)
}

// GetSyncLog godoc
// @Summary Get specific sync log
// @Description Retrieve a specific sync log entry
// @Tags DataSource
// @Produce json
// @Param log_id path string true "Sync log ID"
// @Success 200 {object} types.SyncLog
// @Failure 404 {object} map[string]string
// @Router /datasource/logs/{log_id} [get]
func (h *DataSourceHandler) GetSyncLog(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	logID := c.Param("log_id")

	log, err := h.service.GetSyncLog(ctx, logID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync log not found"})
		return
	}

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, log.DataSourceID); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, log)
}

// CancelSyncLog godoc
// @Summary Cancel a running sync
// @Description Request cooperative cancellation of a running sync. The sync loop
// @Description observes the flag at its next checkpoint/batch boundary and exits
// @Description gracefully (status=canceled, cursor preserved for resume).
// @Tags DataSource
// @Param id path string true "Data source ID"
// @Param log_id path string true "Sync log ID"
// @Success 202 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /datasource/{id}/logs/{log_id}/cancel [post]
func (h *DataSourceHandler) CancelSyncLog(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID := h.getTenantID(c)
	if tenantID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id := c.Param("id")
	logID := c.Param("log_id")

	if _, status, msg := h.getOwnedDataSource(ctx, tenantID, id); status != http.StatusOK {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	if err := h.service.CancelSyncLog(ctx, tenantID, id, logID); err != nil {
		if errors.Is(err, service.ErrSyncLogNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "sync log not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel sync"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "cancel_requested"})
}

// GetAvailableConnectors godoc
// @Summary Get available connectors
// @Description Get list of available data source connectors
// @Tags DataSource
// @Produce json
// @Success 200 {object} []datasource.ConnectorMetadata
// @Router /datasource/types [get]
func (h *DataSourceHandler) GetAvailableConnectors(c *gin.Context) {
	connectors := datasource.ListAvailableConnectors()
	c.JSON(http.StatusOK, connectors)
}
