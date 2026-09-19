package handler

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Pagination defaults and bounds for the by-user listing. The clamps are a
// Task 1 review requirement: AggregateAllUsers takes raw limit/offset, so the
// handler must never forward a missing, negative, or unbounded value.
const (
	usageDefaultPageSize = 50
	usageMaxPageSize     = 200
)

// usageExportColumns is the fixed CSV column order of the admin export.
var usageExportColumns = []string{
	"user_id", "model", "flow", "window_start", "input_tokens",
	"output_tokens", "cache_read_tokens", "cache_write_tokens", "cost_microcredits",
}

// UsageHandler serves the SP12 usage surfaces over the UsageRepository: the
// caller's own daily/model buckets, the tenant-wide paged listing, and the
// per-user CSV export. All windows reuse the analytics range semantics
// ([start_time, end_time) over UTC days, date-only end covers the whole day).
type UsageHandler struct {
	UsageRepo interfaces.UsageRepository
}

// NewUsageHandler creates a usage handler with its repository.
func NewUsageHandler(repo interfaces.UsageRepository) *UsageHandler {
	return &UsageHandler{UsageRepo: repo}
}

// tenantID resolves the execution tenant from the request context. Usage is a
// tenant-scoped surface, so a missing tenant fails closed with 403.
func (h *UsageHandler) tenantID(c *gin.Context) (uint64, bool) {
	tid, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tid == 0 {
		c.Error(errors.NewForbiddenError("tenant context required"))
		return 0, false
	}
	return tid, true
}

// usagePageParams resolves page/page_size into repo-safe limit/offset:
//   - page_size defaults to 50 when absent or unparsable, and clamps to
//     1..200 (huge pages would degenerate the paged listing into a full scan);
//   - page is 0-based and clamps at >= 0, so offset = page*page_size is
//     always non-negative (SQL OFFSET rejects negatives with a 500).
//
// Malformed values fall back to the defaults rather than 400: the dashboard
// sends these from free-form inputs and the clamped result is always sane.
func usagePageParams(c *gin.Context) (limit, offset int) {
	limit = usageDefaultPageSize
	if raw := c.Query("page_size"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	if limit < 1 {
		limit = usageDefaultPageSize
	}
	if limit > usageMaxPageSize {
		limit = usageMaxPageSize
	}
	page := 0
	if raw := c.Query("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	return limit, page * limit
}

// MyUsage returns the caller's own per-day x per-model usage buckets. Tenant
// and user identity come from the request context only — never the query — so
// a Viewer cannot read another user's buckets.
// GET /api/v1/usage/me?start_time&end_time
func (h *UsageHandler) MyUsage(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	uid, ok := types.UserIDFromContext(c.Request.Context())
	if !ok {
		c.Error(errors.NewForbiddenError("user context required"))
		return
	}
	data, err := h.UsageRepo.AggregateByUser(c.Request.Context(), tid, uid, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("usage query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}

// AllUsers returns the tenant-wide per-day x per-model x user buckets, paged
// with the clamped limit/offset above. Rows carry user_id.
// GET /api/v1/admin/usage/by-user?page&page_size&start_time&end_time
func (h *UsageHandler) AllUsers(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	limit, offset := usagePageParams(c)
	data, err := h.UsageRepo.AggregateAllUsers(c.Request.Context(), tid, from, to, limit, offset)
	if err != nil {
		c.Error(errors.NewInternalServerError("usage query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}

// Export streams the window's per-user totals as CSV (one row per user; the
// model / flow / window_start dimensions are empty on export rows and are
// emitted verbatim). Uses encoding/csv into a buffer so delimiter/quote
// escaping in user ids is correct by construction, then serves it with the
// same header+BOM pattern as the FAQ export (BOM for Excel UTF-8 handling).
// GET /api/v1/admin/usage/export?start_time&end_time
func (h *UsageHandler) Export(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	rows, err := h.UsageRepo.ExportRows(c.Request.Context(), tid, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("usage export failed"))
		return
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(usageExportColumns); err != nil {
		c.Error(errors.NewInternalServerError("usage export failed"))
		return
	}
	for _, row := range rows {
		record := []string{
			row.UserID, row.Model, "", row.WindowStart,
			strconv.FormatInt(row.InputTokens, 10),
			strconv.FormatInt(row.OutputTokens, 10),
			strconv.FormatInt(row.CacheReadTokens, 10),
			strconv.FormatInt(row.CacheWriteTokens, 10),
			strconv.FormatInt(row.CostMicrocredits, 10),
		}
		if err := w.Write(record); err != nil {
			c.Error(errors.NewInternalServerError("usage export failed"))
			return
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		c.Error(errors.NewInternalServerError("usage export failed"))
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=usage_export.csv")
	bom := []byte{0xEF, 0xBB, 0xBF}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", append(bom, buf.Bytes()...))
}
