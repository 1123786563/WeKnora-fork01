package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppSyncHandler serves the A07 sync-status projection under
// /api/v1/apps/datasources/:id/sync-status.
type AppSyncHandler struct {
	db *gorm.DB
}

func NewAppSyncHandler(db *gorm.DB) *AppSyncHandler {
	return &AppSyncHandler{db: db}
}

// appSyncStatusView mirrors the A07 binding state for one data source.
type appSyncStatusView struct {
	DataSourceID            string              `json:"datasource_id"`
	State                   string              `json:"state"`
	PauseReason             *string             `json:"pause_reason"`
	Binding                 *appSyncBindingView `json:"binding"`
	RequiresReauthorization bool                `json:"requires_reauthorization"`
}

type appSyncBindingView struct {
	InstallationID string `json:"installation_id"`
	ConnectionID   string `json:"connection_id"`
	AuthVersion    string `json:"auth_version"`
}

// GetSyncStatus GET /apps/datasources/:id/sync-status - A07 binding plus the
// live pause reason. The data source itself is looked up BY tenant (404
// otherwise); pause_reason only ever carries the IsValidPauseReason
// vocabulary (here: permission when reauthorization is required).
func (h *AppSyncHandler) GetSyncStatus(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	dsID := c.Param("id")
	var ds struct {
		ID     string `gorm:"column:id"`
		Status string `gorm:"column:status"`
	}
	if err := h.db.WithContext(c.Request.Context()).
		Table("data_sources").Select("id", "status").
		Where("id = ? AND tenant_id = ?", dsID, tenantID).Take(&ds).Error; err != nil {
		appFail(c, http.StatusNotFound, "DATASOURCE_NOT_FOUND", "data source not found")
		return
	}
	var bindingRow struct {
		InstallationID string `gorm:"column:installation_id"`
		ConnectionID   string `gorm:"column:connection_id"`
		AuthVersion    int64  `gorm:"column:auth_version"`
	}
	err := h.db.WithContext(c.Request.Context()).
		Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", tenantID, dsID).Take(&bindingRow).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			appFail(c, http.StatusInternalServerError, "SYNC_STATUS_FAILED", "failed to load sync binding")
			return
		}
		// No binding row: legacy data source - no migration is forced and no
		// credential ownership is silently rewritten.
		appOK(c, http.StatusOK, appSyncStatusView{DataSourceID: dsID, State: ds.Status, PauseReason: nil, Binding: nil, RequiresReauthorization: false})
		return
	}
	stored := &appconnector.StoredSyncBinding{
		TenantID: tenantID, DataSourceID: dsID,
		InstallationID: bindingRow.InstallationID, ConnectionID: bindingRow.ConnectionID,
		AuthVersion: bindingRow.AuthVersion,
	}
	var state *appconnector.BindingState
	var conn appconnectorrepo.ConnectionRow
	if connErr := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, bindingRow.ConnectionID).First(&conn).Error; connErr == nil {
		var inst appconnectorrepo.InstallationRow
		if instErr := h.db.WithContext(c.Request.Context()).
			Where("id = ? AND tenant_id = ?", conn.InstallationID, tenantID).First(&inst).Error; instErr == nil {
			state = &appconnector.BindingState{
				ConnectionState: conn.State, ConnectionKind: conn.Kind,
				InstallationState: inst.State, ConnectionAuthVer: conn.AuthVersion,
			}
		}
	}
	// state stays nil when the live rows are gone: ResolveSyncBinding then
	// reports the binding for bookkeeping with RequiresReauthorization=true.
	binding, err := appconnector.ResolveSyncBinding(c.Request.Context(), rowStore{row: stored}, tenantID, dsID, state)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "SYNC_STATUS_FAILED", "failed to resolve sync binding")
		return
	}
	view := appSyncStatusView{
		DataSourceID: dsID,
		State:        ds.Status,
		Binding: &appSyncBindingView{
			InstallationID: binding.InstallationID,
			ConnectionID:   binding.ConnectionID,
			AuthVersion:    strconv.FormatInt(binding.AuthVersion, 10),
		},
		RequiresReauthorization: binding.RequiresReauthorization,
	}
	if binding.RequiresReauthorization {
		reason := appconnector.PauseReasonPermission
		view.PauseReason = &reason
	}
	appOK(c, http.StatusOK, view)
}

// rowStore adapts an already-loaded binding row to the SyncBindingStore
// interface so ResolveSyncBinding re-reads nothing.
type rowStore struct {
	row *appconnector.StoredSyncBinding
}

func (s rowStore) FindSyncBinding(context.Context, uint64, string) (*appconnector.StoredSyncBinding, error) {
	return s.row, nil
}
