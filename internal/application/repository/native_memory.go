package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNativeMemoryScope         = errors.New("native memory scope is unavailable")
	ErrNativeMemoryWriteRejected = errors.New("native memory write rejected")
)

const (
	NativeMemoryJobQueued    = "queued"
	NativeMemoryJobRunning   = "running"
	NativeMemoryJobSucceeded = "succeeded"
	NativeMemoryJobDiscarded = "discarded"
	NativeMemoryJobFailed    = "failed"
)

// NativeMemoryState is the durable generation fence for one tenant subject.
type NativeMemoryState struct {
	Enabled, Tombstoned                             bool
	Generation, PolicyRevision, TombstoneGeneration int64
}

// NativeMemoryEntry is a native-memory projection. Tombstoned rows remain as
// a deletion receipt so delayed extractions cannot recreate their old value.
type NativeMemoryEntry struct {
	ID, Content string
	Generation  int64
	Tombstoned  bool
	Metadata    map[string]any
}

type nativeMemoryScopeRow struct {
	TenantID            uint64 `gorm:"column:tenant_id"`
	UserID              string `gorm:"column:user_id"`
	Generation          int64  `gorm:"column:generation"`
	TombstoneGeneration int64  `gorm:"column:tombstone_generation"`
	Enabled             bool   `gorm:"column:enabled"`
	PolicyRevision      int64  `gorm:"column:policy_revision"`
}

func (nativeMemoryScopeRow) TableName() string { return "native_agent_memory_scopes" }

type nativeMemoryEntryRow struct {
	TenantID, Generation                uint64
	UserID, MemoryID, Content, Metadata string
	Tombstoned                          bool
}

func (nativeMemoryEntryRow) TableName() string { return "native_agent_memory_entries" }

type nativeMemoryJobRow struct {
	TenantID                                 uint64 `gorm:"column:tenant_id"`
	SubjectID, JobID, ThroughEventID, Status string
	Generation, PolicyRevision               int64
}

func (nativeMemoryJobRow) TableName() string { return "native_memory_jobs" }

// NativeMemoryRepository makes the native tables the authorization and
// generation authority. SDK memory implementations have no conditional-write
// input, so they cannot be used as this linearization point.
type NativeMemoryRepository struct{ db *gorm.DB }

func NewNativeMemoryRepository(db *gorm.DB) *NativeMemoryRepository {
	return &NativeMemoryRepository{db: db}
}

func nativeMemorySubject(scope nativecontract.Scope) (string, error) {
	key, err := nativecontract.MemoryKey(scope)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNativeMemoryScope, err)
	}
	return key.UserID, nil
}

func (r *NativeMemoryRepository) EnsureScope(ctx context.Context, scope nativecontract.Scope) error {
	subject, err := nativeMemorySubject(scope)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenantCount int64
		if err := tx.Table("native_agent_tenants").Where("tenant_id=?", scope.TenantID).Count(&tenantCount).Error; err != nil {
			return err
		}
		if tenantCount != 1 {
			return ErrNativeMemoryScope
		}
		row := nativeMemoryScopeRow{TenantID: scope.TenantID, UserID: subject, Enabled: true, PolicyRevision: scope.PolicyRevision}
		result := tx.Where("tenant_id=? AND user_id=?", scope.TenantID, subject).FirstOrCreate(&row)
		if result.Error != nil || result.RowsAffected != 0 || row.PolicyRevision == scope.PolicyRevision {
			return result.Error
		}
		result = tx.Model(&nativeMemoryScopeRow{}).Where("tenant_id=? AND user_id=? AND generation=? AND policy_revision=?", scope.TenantID, subject, row.Generation, row.PolicyRevision).Updates(map[string]any{"generation": row.Generation + 1, "policy_revision": scope.PolicyRevision, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNativeMemoryWriteRejected
		}
		return tx.Model(&nativeMemoryJobRow{}).Where("tenant_id=? AND subject_id=? AND generation<? AND status IN ?", scope.TenantID, subject, row.Generation+1, []string{NativeMemoryJobQueued, NativeMemoryJobRunning}).Update("status", NativeMemoryJobDiscarded).Error
	})
}

func (r *NativeMemoryRepository) state(tx *gorm.DB, scope nativecontract.Scope) (nativeMemoryScopeRow, string, error) {
	subject, err := nativeMemorySubject(scope)
	if err != nil {
		return nativeMemoryScopeRow{}, "", err
	}
	var row nativeMemoryScopeRow
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", scope.TenantID, subject).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nativeMemoryScopeRow{}, subject, ErrNativeMemoryScope
	}
	return row, subject, err
}

func memoryState(row nativeMemoryScopeRow) NativeMemoryState {
	return NativeMemoryState{Enabled: row.Enabled, Generation: row.Generation, TombstoneGeneration: row.TombstoneGeneration, PolicyRevision: row.PolicyRevision}
}

func (r *NativeMemoryRepository) State(ctx context.Context, scope nativecontract.Scope) (NativeMemoryState, error) {
	row, _, err := r.state(r.db.WithContext(ctx), scope)
	return memoryState(row), err
}

// SetEnabled changes policy and advances the generation in the same transaction.
func (r *NativeMemoryRepository) SetEnabled(ctx context.Context, scope nativecontract.Scope, enabled bool) error {
	return r.mutateScope(ctx, scope, enabled, false)
}
func (r *NativeMemoryRepository) Clear(ctx context.Context, scope nativecontract.Scope) error {
	return r.mutateScope(ctx, scope, false, true)
}
func (r *NativeMemoryRepository) mutateScope(ctx context.Context, scope nativecontract.Scope, enabled, clear bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, subject, err := r.state(tx, scope)
		if err != nil {
			return err
		}
		if clear {
			enabled = row.Enabled
		}
		next := row.Generation + 1
		updates := map[string]any{"enabled": enabled, "generation": next, "policy_revision": scope.PolicyRevision, "updated_at": time.Now().UTC()}
		if clear {
			updates["tombstone_generation"] = next
		}
		result := tx.Model(&nativeMemoryScopeRow{}).Where("tenant_id=? AND user_id=? AND generation=?", scope.TenantID, subject, row.Generation).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNativeMemoryWriteRejected
		}
		if clear {
			if err := tx.Model(&nativeMemoryEntryRow{}).Where("tenant_id=? AND user_id=?", scope.TenantID, subject).Update("tombstoned", true).Error; err != nil {
				return err
			}
		}
		return tx.Model(&nativeMemoryJobRow{}).Where("tenant_id=? AND subject_id=? AND generation<? AND status IN ?", scope.TenantID, subject, next, []string{NativeMemoryJobQueued, NativeMemoryJobRunning}).Update("status", NativeMemoryJobDiscarded).Error
	})
}

func (r *NativeMemoryRepository) Delete(ctx context.Context, scope nativecontract.Scope, id string) error {
	if id == "" {
		return ErrNativeMemoryWriteRejected
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, subject, err := r.state(tx, scope)
		if err != nil {
			return err
		}
		next := row.Generation + 1
		result := tx.Model(&nativeMemoryScopeRow{}).Where("tenant_id=? AND user_id=? AND generation=?", scope.TenantID, subject, row.Generation).Updates(map[string]any{"generation": next, "tombstone_generation": next, "policy_revision": scope.PolicyRevision, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNativeMemoryWriteRejected
		}
		entry := nativeMemoryEntryRow{TenantID: scope.TenantID, UserID: subject, MemoryID: id, Generation: uint64(next), Tombstoned: true, Content: "", Metadata: "{}"}
		if err := tx.Where("tenant_id=? AND user_id=? AND memory_id=?", scope.TenantID, subject, id).Assign(map[string]any{"generation": next, "tombstoned": true}).FirstOrCreate(&entry).Error; err != nil {
			return err
		}
		return tx.Model(&nativeMemoryJobRow{}).Where("tenant_id=? AND subject_id=? AND generation<? AND status IN ?", scope.TenantID, subject, next, []string{NativeMemoryJobQueued, NativeMemoryJobRunning}).Update("status", NativeMemoryJobDiscarded).Error
	})
}

func (r *NativeMemoryRepository) Enqueue(ctx context.Context, job nativecontract.MemoryJob) error {
	if job.ID == "" || job.ThroughEventID == "" {
		return ErrNativeMemoryWriteRejected
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, subject, err := r.state(tx, job.Scope)
		if err != nil {
			return err
		}
		if !AcceptNativeMemoryWrite(row.Enabled, row.Generation, row.PolicyRevision, job) {
			return ErrNativeMemoryWriteRejected
		}
		candidate := nativeMemoryJobRow{TenantID: job.Scope.TenantID, SubjectID: subject, JobID: job.ID, Generation: job.Generation, PolicyRevision: job.PolicyRevision, ThroughEventID: job.ThroughEventID, Status: NativeMemoryJobQueued}
		var existing nativeMemoryJobRow
		err = tx.Where("tenant_id=? AND subject_id=? AND job_id=?", candidate.TenantID, subject, job.ID).Take(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&candidate).Error
		}
		if err != nil {
			return err
		}
		if existing.Generation != candidate.Generation || existing.PolicyRevision != candidate.PolicyRevision || existing.ThroughEventID != candidate.ThroughEventID || existing.Status == NativeMemoryJobDiscarded || existing.Status == NativeMemoryJobFailed {
			return ErrNativeMemoryWriteRejected
		}
		return nil
	})
}

func AcceptNativeMemoryWrite(enabled bool, currentGeneration, currentPolicy int64, job nativecontract.MemoryJob) bool {
	return enabled && currentGeneration == job.Generation && currentPolicy == job.PolicyRevision
}

// Write is used for foreground writes. It checks the generation inside the
// transaction; a caller cannot reuse a generation observed before a clear.
func (r *NativeMemoryRepository) Write(ctx context.Context, scope nativecontract.Scope, generation int64, id, content string, metadata map[string]any) error {
	written, err := r.commit(ctx, nativecontract.MemoryJob{Scope: scope, Generation: generation, PolicyRevision: scope.PolicyRevision}, []NativeMemoryEntry{{ID: id, Content: content, Metadata: metadata}}, false)
	if err != nil {
		return err
	}
	if !written {
		return ErrNativeMemoryWriteRejected
	}
	return nil
}

func (r *NativeMemoryRepository) Commit(ctx context.Context, job nativecontract.MemoryJob, id, content string, metadata map[string]any) (bool, error) {
	return r.commit(ctx, job, []NativeMemoryEntry{{ID: id, Content: content, Metadata: metadata}}, true)
}
func (r *NativeMemoryRepository) CommitWrites(ctx context.Context, job nativecontract.MemoryJob, entries []NativeMemoryEntry) (bool, error) {
	return r.commit(ctx, job, entries, true)
}

// Replace tombstones the old SDK key and writes the rotated key in one
// transaction. A failed replacement therefore never forgets the old value.
func (r *NativeMemoryRepository) Replace(ctx context.Context, scope nativecontract.Scope, generation int64, oldID string, entry NativeMemoryEntry) error {
	if oldID == "" || entry.ID == "" || entry.Content == "" {
		return ErrNativeMemoryWriteRejected
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, subject, err := r.state(tx, scope)
		if err != nil {
			return err
		}
		if !row.Enabled || row.Generation != generation {
			return ErrNativeMemoryWriteRejected
		}
		metadata, err := json.Marshal(entry.Metadata)
		if err != nil {
			return err
		}
		candidate := nativeMemoryEntryRow{TenantID: scope.TenantID, UserID: subject, MemoryID: entry.ID, Generation: uint64(generation), Content: entry.Content, Metadata: string(metadata)}
		if err := tx.Where("tenant_id=? AND user_id=? AND memory_id=?", scope.TenantID, subject, entry.ID).Assign(map[string]any{"generation": generation, "tombstoned": false, "content": entry.Content, "metadata": string(metadata)}).FirstOrCreate(&candidate).Error; err != nil {
			return err
		}
		result := tx.Model(&nativeMemoryEntryRow{}).Where("tenant_id=? AND user_id=? AND memory_id=? AND generation=? AND tombstoned=?", scope.TenantID, subject, oldID, generation, false).Update("tombstoned", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNativeMemoryWriteRejected
		}
		return nil
	})
}

type NativeMemoryEntryWrite = NativeMemoryEntry

func (r *NativeMemoryRepository) commit(ctx context.Context, job nativecontract.MemoryJob, entries []NativeMemoryEntry, jobWrite bool) (bool, error) {
	if len(entries) == 0 {
		return false, ErrNativeMemoryWriteRejected
	}
	returnValue := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, subject, err := r.state(tx, job.Scope)
		if err != nil {
			return err
		}
		if jobWrite {
			var persisted nativeMemoryJobRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND subject_id=? AND job_id=? AND generation=? AND policy_revision=? AND through_event_id=?", job.Scope.TenantID, subject, job.ID, job.Generation, job.PolicyRevision, job.ThroughEventID).Take(&persisted).Error; err != nil {
				return err
			}
			if persisted.Status != NativeMemoryJobQueued && persisted.Status != NativeMemoryJobRunning {
				return nil
			}
		}
		if !AcceptNativeMemoryWrite(row.Enabled, row.Generation, row.PolicyRevision, job) {
			if jobWrite {
				if err := tx.Model(&nativeMemoryJobRow{}).Where("tenant_id=? AND subject_id=? AND job_id=?", job.Scope.TenantID, subject, job.ID).Update("status", NativeMemoryJobDiscarded).Error; err != nil {
					return err
				}
			}
			return nil
		}
		for _, entry := range entries {
			if entry.ID == "" || entry.Content == "" {
				return ErrNativeMemoryWriteRejected
			}
			metadata, err := json.Marshal(entry.Metadata)
			if err != nil {
				return err
			}
			candidate := nativeMemoryEntryRow{TenantID: job.Scope.TenantID, UserID: subject, MemoryID: entry.ID, Generation: uint64(job.Generation), Content: entry.Content, Metadata: string(metadata)}
			if err := tx.Where("tenant_id=? AND user_id=? AND memory_id=?", job.Scope.TenantID, subject, entry.ID).Assign(map[string]any{"generation": job.Generation, "tombstoned": false, "content": entry.Content, "metadata": string(metadata)}).FirstOrCreate(&candidate).Error; err != nil {
				return err
			}
		}
		if jobWrite {
			result := tx.Model(&nativeMemoryJobRow{}).Where("tenant_id=? AND subject_id=? AND job_id=? AND generation=? AND policy_revision=? AND through_event_id=? AND status IN ?", job.Scope.TenantID, subject, job.ID, job.Generation, job.PolicyRevision, job.ThroughEventID, []string{NativeMemoryJobQueued, NativeMemoryJobRunning}).Update("status", NativeMemoryJobSucceeded)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrNativeMemoryWriteRejected
			}
		}
		returnValue = true
		return nil
	})
	return returnValue, err
}

func (r *NativeMemoryRepository) Read(ctx context.Context, scope nativecontract.Scope, limit int) ([]NativeMemoryEntry, error) {
	row, subject, err := r.state(r.db.WithContext(ctx), scope)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return []NativeMemoryEntry{}, nil
	}
	if limit <= 0 {
		return []NativeMemoryEntry{}, nil
	}
	var rows []nativeMemoryEntryRow
	if err := r.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND generation=? AND tombstoned=?", scope.TenantID, subject, row.Generation, false).Order("memory_id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]NativeMemoryEntry, 0, len(rows))
	for _, row := range rows {
		var metadata map[string]any
		_ = json.Unmarshal([]byte(row.Metadata), &metadata)
		out = append(out, NativeMemoryEntry{ID: row.MemoryID, Content: row.Content, Generation: int64(row.Generation), Tombstoned: row.Tombstoned, Metadata: metadata})
	}
	return out, nil
}
func (r *NativeMemoryRepository) Entry(ctx context.Context, scope nativecontract.Scope, id string) (NativeMemoryEntry, error) {
	_, subject, err := r.state(r.db.WithContext(ctx), scope)
	if err != nil {
		return NativeMemoryEntry{}, err
	}
	var row nativeMemoryEntryRow
	if err := r.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND memory_id=?", scope.TenantID, subject, id).Take(&row).Error; err != nil {
		return NativeMemoryEntry{}, err
	}
	var metadata map[string]any
	_ = json.Unmarshal([]byte(row.Metadata), &metadata)
	return NativeMemoryEntry{ID: row.MemoryID, Content: row.Content, Generation: int64(row.Generation), Tombstoned: row.Tombstoned, Metadata: metadata}, nil
}
func (r *NativeMemoryRepository) JobStatus(ctx context.Context, job nativecontract.MemoryJob) (string, error) {
	_, subject, err := r.state(r.db.WithContext(ctx), job.Scope)
	if err != nil {
		return "", err
	}
	var row nativeMemoryJobRow
	if err := r.db.WithContext(ctx).Where("tenant_id=? AND subject_id=? AND job_id=?", job.Scope.TenantID, subject, job.ID).Take(&row).Error; err != nil {
		return "", err
	}
	return row.Status, nil
}

// Discard makes an authorization or generation-invalid job terminal. Such a
// job must not be retried: retrying it after re-enable would resurrect data.
func (r *NativeMemoryRepository) Discard(ctx context.Context, job nativecontract.MemoryJob) error {
	_, subject, err := r.state(r.db.WithContext(ctx), job.Scope)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&nativeMemoryJobRow{}).
		Where("tenant_id=? AND subject_id=? AND job_id=?", job.Scope.TenantID, subject, job.ID).
		Update("status", NativeMemoryJobDiscarded).Error
}

// Fail records a terminal extraction/storage failure. Generation and
// authorization invalidation use Discard instead, because those jobs are
// forbidden from retrying after a later re-enable.
func (r *NativeMemoryRepository) Fail(ctx context.Context, job nativecontract.MemoryJob) error {
	_, subject, err := r.state(r.db.WithContext(ctx), job.Scope)
	if err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Model(&nativeMemoryJobRow{}).
		Where("tenant_id=? AND subject_id=? AND job_id=? AND generation=? AND policy_revision=? AND through_event_id=? AND status IN ?", job.Scope.TenantID, subject, job.ID, job.Generation, job.PolicyRevision, job.ThroughEventID, []string{NativeMemoryJobQueued, NativeMemoryJobRunning}).
		Update("status", NativeMemoryJobFailed)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return nil
}
