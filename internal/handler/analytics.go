package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// analyticsDefaultLookbackDays is the window used when the caller does not
// pass start_time/end_time: now-30d to now, evaluated in UTC.
const analyticsDefaultLookbackDays = 30

// AnalyticsHandler serves the admin console analytics dashboard: read-time
// aggregations over sessions / messages / message_feedback, scoped to the
// caller's tenant and an optional [start_time, end_time) window.
type AnalyticsHandler struct {
	AnalyticsRepo interfaces.AnalyticsRepository
}

// NewAnalyticsHandler creates an analytics handler with its repository.
func NewAnalyticsHandler(repo interfaces.AnalyticsRepository) *AnalyticsHandler {
	return &AnalyticsHandler{AnalyticsRepo: repo}
}

// parseAnalyticsRange resolves the shared start_time/end_time query filter.
// Missing values fall back to a 30-day lookback ending at now (UTC); values
// that parseFilterTime cannot interpret reject the request with 400, with the
// offending parameter name in the message.
//
// Date-only end_time values ("2006-01-02") are treated as an inclusive end
// day: the window upper bound moves to the following UTC midnight so the
// [from, to) SQL predicate covers the whole end day. The dashboard's default
// window is [today-30, today] with date-only bounds — without this rule every
// chart would silently exclude the current day's data.
func parseAnalyticsRange(c *gin.Context) (from, to time.Time, ok bool) {
	now := time.Now().UTC()
	from = now.AddDate(0, 0, -analyticsDefaultLookbackDays)
	to = now
	if raw := c.Query("start_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid start_time: " + err.Error()))
			return time.Time{}, time.Time{}, false
		}
		from = t.UTC()
	}
	if raw := c.Query("end_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid end_time: " + err.Error()))
			return time.Time{}, time.Time{}, false
		}
		to = t.UTC()
		if analyticsIsDateOnly(raw) {
			to = to.AddDate(0, 0, 1)
		}
	}
	return from, to, true
}

// analyticsIsDateOnly reports whether raw is a bare YYYY-MM-DD date (no time
// component), the layout the analytics dashboard sends for both bounds.
func analyticsIsDateOnly(raw string) bool {
	if len(raw) != 10 {
		return false
	}
	for i, r := range raw {
		switch i {
		case 4, 7:
			if r != '-' {
				return false
			}
		default:
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// analyticsRows guarantees the envelope's data field is a JSON array: GORM's
// Scan leaves a nil slice when a window has no rows, and "data":null breaks
// the api-client contract (parsers reject non-array data).
func analyticsRows[T any](rows []T) []T {
	if rows == nil {
		return []T{}
	}
	return rows
}

// tenantID resolves the execution tenant from the request context. Analytics
// is a tenant-scoped surface, so a missing tenant fails closed with 403.
func (h *AnalyticsHandler) tenantID(c *gin.Context) (uint64, bool) {
	tid, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tid == 0 {
		c.Error(errors.NewForbiddenError("tenant context required"))
		return 0, false
	}
	return tid, true
}

// QueryTrend returns per-day question volume vs like/dislike feedback.
// GET /api/v1/analytics/queries?start_time&end_time
func (h *AnalyticsHandler) QueryTrend(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	data, err := h.AnalyticsRepo.QueryTrend(c.Request.Context(), tid, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("analytics query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}

// ActiveUsers returns the per-day distinct active session owners.
// GET /api/v1/analytics/users?start_time&end_time
func (h *AnalyticsHandler) ActiveUsers(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	data, err := h.AnalyticsRepo.ActiveUsers(c.Request.Context(), tid, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("analytics query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}

// ChannelSessions returns per-day new sessions broken down by source bucket.
// GET /api/v1/analytics/channels?start_time&end_time
func (h *AnalyticsHandler) ChannelSessions(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	data, err := h.AnalyticsRepo.ChannelSessions(c.Request.Context(), tid, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("analytics query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}

// AgentMessages returns one custom agent's per-day usage. The :agent_id path
// parameter is required — an empty id rejects the request with 400.
// GET /api/v1/analytics/agents/:agent_id?start_time&end_time
func (h *AnalyticsHandler) AgentMessages(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	agentID := c.Param("agent_id")
	if agentID == "" {
		c.Error(errors.NewBadRequestError("agent_id is required"))
		return
	}
	data, err := h.AnalyticsRepo.AgentMessages(c.Request.Context(), tid, agentID, from, to)
	if err != nil {
		c.Error(errors.NewInternalServerError("analytics query failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": analyticsRows(data)})
}
