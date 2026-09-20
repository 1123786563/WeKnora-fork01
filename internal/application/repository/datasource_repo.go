package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// DataSourceRepository provides data access for data sources
type DataSourceRepository struct {
	db *gorm.DB
}

// NewDataSourceRepository creates a new data source repository
func NewDataSourceRepository(db *gorm.DB) interfaces.DataSourceRepository {
	return &DataSourceRepository{db: db}
}

// Create inserts a new data source record
func (r *DataSourceRepository) Create(ctx context.Context, ds *types.DataSource) error {
	if ds == nil {
		return errors.New("data source is nil")
	}
	// GORM treats false as the zero value of bool. For a field tagged
	// default:true it replaces both the INSERT value and the in-memory field
	// with true, so a caller-selected false would be lost. Capture it, force
	// the column write, then restore the struct so Create's return value (and
	// the HTTP 201 body) match the database.
	syncDeletions := ds.SyncDeletions
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ds).Error; err != nil {
			return err
		}
		return tx.Model(&types.DataSource{}).
			Where("id = ?", ds.ID).
			UpdateColumn("sync_deletions", syncDeletions).Error
	})
	ds.SyncDeletions = syncDeletions
	return err
}

// FindByID retrieves a data source by ID
func (r *DataSourceRepository) FindByID(ctx context.Context, id string) (*types.DataSource, error) {
	if id == "" {
		return nil, errors.New("id is empty")
	}
	var ds types.DataSource
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(&ds).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("data source not found")
		}
		return nil, err
	}
	return &ds, nil
}

// FindByKnowledgeBase lists all data sources for a knowledge base
func (r *DataSourceRepository) FindByKnowledgeBase(ctx context.Context, kbID string) ([]*types.DataSource, error) {
	if kbID == "" {
		return nil, errors.New("knowledge base id is empty")
	}
	var dataSources []*types.DataSource
	if err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ?", kbID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Find(&dataSources).Error; err != nil {
		return nil, err
	}
	return dataSources, nil
}

// Update updates an existing data source
func (r *DataSourceRepository) Update(ctx context.Context, ds *types.DataSource) error {
	if ds == nil {
		return errors.New("data source is nil")
	}
	if ds.ID == "" {
		return errors.New("data source id is empty")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(ds).Updates(ds).Error; err != nil {
			return err
		}
		// GORM Updates(struct) deliberately skips zero values, which would make
		// a user-selected sync_deletions=false impossible to persist.
		return tx.Model(&types.DataSource{}).
			Where("id = ?", ds.ID).
			UpdateColumn("sync_deletions", ds.SyncDeletions).Error
	})
}

// UpdateSyncState updates only fields managed by sync execution. GORM's
// Updates(struct) skips zero values, so use a map here to persist cleared error
// messages without broadening the generic Update method.
func (r *DataSourceRepository) UpdateSyncState(ctx context.Context, ds *types.DataSource) error {
	if ds == nil {
		return errors.New("data source is nil")
	}
	if ds.ID == "" {
		return errors.New("data source id is empty")
	}
	if err := r.db.WithContext(ctx).
		Model(&types.DataSource{}).
		Where("id = ?", ds.ID).
		Updates(map[string]interface{}{
			"status":           ds.Status,
			"last_sync_at":     ds.LastSyncAt,
			"last_sync_cursor": ds.LastSyncCursor,
			"last_sync_result": ds.LastSyncResult,
			"error_message":    ds.ErrorMessage,
			"updated_at":       time.Now().UTC(),
		}).Error; err != nil {
		return err
	}
	return nil
}

// Delete performs a soft delete
func (r *DataSourceRepository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id is empty")
	}
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&types.DataSource{}).Error; err != nil {
		return err
	}
	return nil
}

// FindActive retrieves all active data sources (used for scheduling)
func (r *DataSourceRepository) FindActive(ctx context.Context) ([]*types.DataSource, error) {
	var dataSources []*types.DataSource
	if err := r.db.WithContext(ctx).
		Where("status = ?", types.DataSourceStatusActive).
		Where("deleted_at IS NULL").
		Where("sync_schedule != ''").
		Order("created_at DESC").
		Find(&dataSources).Error; err != nil {
		return nil, err
	}
	return dataSources, nil
}

// AppDataSourceBindingRow maps one app_datasource_bindings row (A07): the
// app installation and space connection a team-synced data source executes
// under, at the auth version its cursor was produced with.
type AppDataSourceBindingRow struct {
	ID                      string `gorm:"column:id"`
	TenantID                uint64 `gorm:"column:tenant_id"`
	DataSourceID            string `gorm:"column:datasource_id"`
	InstallationID          string `gorm:"column:installation_id"`
	ConnectionID            string `gorm:"column:connection_id"`
	AuthVersion             int64  `gorm:"column:auth_version"`
	RequiresReauthorization bool   `gorm:"column:requires_reauthorization"`
}

// TableName pins the relation created by migrations 000120 (PG) / 000040
// (SQLite). The row is deliberately NOT part of interfaces.DataSourceRepository:
// it backs the A07 scoped-sync path only.
func (AppDataSourceBindingRow) TableName() string { return "app_datasource_bindings" }

// FindAppDataSourceBinding loads the binding of one data source within a
// tenant. A missing row returns a nil row with a nil error so the caller can
// keep the legacy execution path.
func (r *DataSourceRepository) FindAppDataSourceBinding(
	ctx context.Context, tenantID uint64, dataSourceID string,
) (*AppDataSourceBindingRow, error) {
	if dataSourceID == "" {
		return nil, errors.New("data source id is empty")
	}
	var row AppDataSourceBindingRow
	err := r.db.WithContext(ctx).
		Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", tenantID, dataSourceID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// SaveAppDataSourceBinding upserts the (tenant_id, datasource_id) relation row.
// Duplicate runs must not create duplicate entries: the unique key is updated
// in place, never inserted twice.
func (r *DataSourceRepository) SaveAppDataSourceBinding(
	ctx context.Context, row *AppDataSourceBindingRow,
) error {
	if row == nil {
		return errors.New("binding is nil")
	}
	if row.DataSourceID == "" {
		return errors.New("data source id is empty")
	}
	updated := r.db.WithContext(ctx).
		Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", row.TenantID, row.DataSourceID).
		Updates(map[string]interface{}{
			"installation_id":          row.InstallationID,
			"connection_id":            row.ConnectionID,
			"auth_version":             row.AuthVersion,
			"requires_reauthorization": row.RequiresReauthorization,
			"updated_at":               time.Now().UTC(),
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Table("app_datasource_bindings").Create(row).Error
}

// DeleteAppDataSourceBindingsByDataSource removes every binding row of one
// data source (SP2-a Task 8 purge tail). Scoped to (tenant, data source) like
// Find/SaveAppDataSourceBinding; a missing row is a no-op so the purge worker
// can re-run its tail cleanup idempotently.
func (r *DataSourceRepository) DeleteAppDataSourceBindingsByDataSource(
	ctx context.Context, tenantID uint64, dataSourceID string,
) error {
	if dataSourceID == "" {
		return errors.New("data source id is empty")
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND datasource_id = ?", tenantID, dataSourceID).
		Delete(&AppDataSourceBindingRow{}).Error
}

// SyncLogRepository provides data access for sync logs
type SyncLogRepository struct {
	db *gorm.DB
}

// NewSyncLogRepository creates a new sync log repository
func NewSyncLogRepository(db *gorm.DB) interfaces.SyncLogRepository {
	return &SyncLogRepository{db: db}
}

// Create inserts a new sync log entry
func (r *SyncLogRepository) Create(ctx context.Context, log *types.SyncLog) error {
	if log == nil {
		return errors.New("sync log is nil")
	}
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return err
	}
	return nil
}

// FindByID retrieves a sync log by ID
func (r *SyncLogRepository) FindByID(ctx context.Context, id string) (*types.SyncLog, error) {
	if id == "" {
		return nil, errors.New("id is empty")
	}
	var log types.SyncLog
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("sync log not found")
		}
		return nil, err
	}
	return &log, nil
}

// FindByDataSource lists sync logs for a data source with pagination
func (r *SyncLogRepository) FindByDataSource(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error) {
	if dsID == "" {
		return nil, errors.New("data source id is empty")
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	var logs []*types.SyncLog
	if err := r.db.WithContext(ctx).
		Where("data_source_id = ?", dsID).
		Order("started_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// FindLatest retrieves the most recent sync log for a data source
func (r *SyncLogRepository) FindLatest(ctx context.Context, dsID string) (*types.SyncLog, error) {
	if dsID == "" {
		return nil, errors.New("data source id is empty")
	}
	var log types.SyncLog
	if err := r.db.WithContext(ctx).
		Where("data_source_id = ?", dsID).
		Order("started_at DESC").
		Limit(1).
		First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// HasRunningSync checks if a data source has any sync currently running, used
// to prevent overlapping sync executions. Liveness is judged by the latest
// heartbeat (heartbeat_at, falling back to started_at for runs that have not
// checkpointed yet): a "running" row with no liveness signal inside
// types.SyncStallWindow belongs to a dead run and does not count — otherwise
// one stalled row would block the data source's scheduled syncs forever.
func (r *SyncLogRepository) HasRunningSync(ctx context.Context, dsID string) (bool, error) {
	if dsID == "" {
		return false, errors.New("data source id is empty")
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("data_source_id = ?", dsID).
		Where("status = ?", types.SyncLogStatusRunning).
		Where("COALESCE(heartbeat_at, started_at) > ?", time.Now().UTC().Add(-types.SyncStallWindow)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update updates an existing sync log entry
func (r *SyncLogRepository) Update(ctx context.Context, log *types.SyncLog) error {
	if log == nil {
		return errors.New("sync log is nil")
	}
	if log.ID == "" {
		return errors.New("sync log id is empty")
	}
	if err := r.db.WithContext(ctx).
		Model(log).
		Updates(log).Error; err != nil {
		return err
	}
	return nil
}

// UpdateResult updates only fields produced by sync execution. Use an explicit
// map so empty error messages are written when a later sync succeeds.
func (r *SyncLogRepository) UpdateResult(ctx context.Context, log *types.SyncLog) error {
	if log == nil {
		return errors.New("sync log is nil")
	}
	if log.ID == "" {
		return errors.New("sync log id is empty")
	}
	if err := r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("id = ?", log.ID).
		Updates(map[string]interface{}{
			"status":        log.Status,
			"finished_at":   log.FinishedAt,
			"items_total":   log.ItemsTotal,
			"items_created": log.ItemsCreated,
			"items_updated": log.ItemsUpdated,
			"items_deleted": log.ItemsDeleted,
			"items_skipped": log.ItemsSkipped,
			"items_failed":  log.ItemsFailed,
			"error_message": log.ErrorMessage,
			"result":        log.Result,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
		return err
	}
	return nil
}

// UpdateHeartbeat records a liveness heartbeat for a sync run and refreshes
// updated_at so stall detection can distinguish live runs from stalled ones.
func (r *SyncLogRepository) UpdateHeartbeat(ctx context.Context, id string, at time.Time) error {
	if id == "" {
		return errors.New("sync log id is empty")
	}
	return r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("id = ?", id).
		Update("heartbeat_at", at).Error
}

// UpdateAsynqTaskID records the asynq task id backing a sync run so queued
// tasks can be traced back to their sync log for inspection or hard cancel.
func (r *SyncLogRepository) UpdateAsynqTaskID(ctx context.Context, id string, taskID string) error {
	if id == "" {
		return errors.New("sync log id is empty")
	}
	return r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("id = ?", id).
		Update("asynq_task_id", taskID).Error
}

// RequestCancel flags a running sync log for cooperative cancellation. Only
// rows currently in "running" status accept the flag; terminal or pending
// rows are a no-op (left untouched, nil returned).
func (r *SyncLogRepository) RequestCancel(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("sync log id is empty")
	}
	// Single-column Update keeps the write narrow: the flag is only ever set
	// to true here (never cleared through this method), and the status
	// predicate makes non-running rows a no-op — zero rows affected, no error.
	return r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("id = ?", id).
		Where("status = ?", types.SyncLogStatusRunning).
		Update("cancel_requested", true).Error
}

// CancelPendingByDataSource marks all non-terminal sync logs for a data source as canceled.
func (r *SyncLogRepository) CancelPendingByDataSource(ctx context.Context, dsID string) error {
	if dsID == "" {
		return errors.New("data source id is empty")
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&types.SyncLog{}).
		Where("data_source_id = ?", dsID).
		Where("status IN ?", []string{types.SyncLogStatusRunning, types.SyncLogStatusPending}).
		Updates(map[string]interface{}{
			"status":        types.SyncLogStatusCanceled,
			"finished_at":   &now,
			"error_message": "data source deleted",
		}).Error
}

// CleanupOldLogs deletes sync logs older than the retention period
func (r *SyncLogRepository) CleanupOldLogs(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	// Delete logs older than the retention period
	if err := r.db.WithContext(ctx).
		Where("started_at < NOW() - INTERVAL ? DAY", retentionDays).
		Delete(&types.SyncLog{}).Error; err != nil {
		return err
	}
	return nil
}
