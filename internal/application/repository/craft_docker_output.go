package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCraftDockerOutputInvalid       = errors.New("invalid Docker output identity or chunk")
	ErrCraftDockerOutputConflict      = fmt.Errorf("Docker output operation conflicts with durable identity: %w", craft.ErrConflict)
	ErrCraftDockerOutputNotFound      = errors.New("Docker output operation not found")
	ErrCraftDockerOutputSealed        = errors.New("Docker output operation is sealed")
	ErrCraftDockerOutputQuota         = errors.New("Docker output quota exceeded; output is partial")
	ErrCraftDockerOutputInvalidCursor = errors.New("invalid Docker output cursor")
	ErrCraftDockerOutputCursorAhead   = errors.New("Docker output cursor is ahead of committed output")
	ErrCraftDockerOutputCorrupt       = errors.New("durable Docker output chunk failed integrity check")
	ErrCraftDockerOutputUnavailable   = errors.New("durable Docker output is unavailable")
)

const MaxCraftDockerOutputBytes int64 = 8 << 20

// CraftDockerOutputScope binds output to the tenant's task, Run, activity and
// immutable Docker receipt. TaskID is the Run's persisted session/task ID.
type CraftDockerOutputScope struct {
	TenantID    uint64
	TaskID      string
	RunID       string
	ActivityKey string
	ContainerID string
	ExecID      string
}

type CraftDockerOutputChunk struct {
	Sequence int64
	Stream   string
	Bytes    []byte
	SHA256   string
}

type CraftDockerOutputSnapshot struct {
	NextSequence int64
	TotalBytes   int64
	Sealed       bool
	Truncated    bool
	Partial      bool
	Unavailable  bool
}

type CraftDockerOutputRepository struct {
	db       *gorm.DB
	maxBytes int64
}

func NewCraftDockerOutputRepository(db *gorm.DB, maxBytes int64) *CraftDockerOutputRepository {
	return &CraftDockerOutputRepository{db: db, maxBytes: maxBytes}
}

type craftDockerOutputOperationRow struct {
	TenantID     uint64     `gorm:"column:tenant_id;primaryKey"`
	TaskID       string     `gorm:"column:task_id"`
	RunID        string     `gorm:"column:run_id;primaryKey"`
	ActivityKey  string     `gorm:"column:activity_key;primaryKey"`
	ContainerID  string     `gorm:"column:container_id"`
	ExecID       string     `gorm:"column:exec_id"`
	MaxBytes     int64      `gorm:"column:max_bytes"`
	NextSequence int64      `gorm:"column:next_sequence"`
	TotalBytes   int64      `gorm:"column:total_bytes"`
	SealedAt     *time.Time `gorm:"column:sealed_at"`
	Truncated    bool       `gorm:"column:truncated"`
	LockVersion  int64      `gorm:"column:lock_version"`
}

func (craftDockerOutputOperationRow) TableName() string { return "craft_docker_output_operations" }

type craftDockerOutputChunkRow struct {
	TenantID       uint64    `gorm:"column:tenant_id;primaryKey"`
	RunID          string    `gorm:"column:run_id;primaryKey"`
	ActivityKey    string    `gorm:"column:activity_key;primaryKey"`
	Sequence       int64     `gorm:"column:sequence;primaryKey"`
	Stream         string    `gorm:"column:stream"`
	Bytes          []byte    `gorm:"column:bytes"`
	ByteCount      int64     `gorm:"column:byte_count"`
	SHA256         string    `gorm:"column:sha256"`
	InputByteCount int64     `gorm:"column:input_byte_count"`
	InputSHA256    string    `gorm:"column:input_sha256"`
	InputTruncated bool      `gorm:"column:input_truncated"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (craftDockerOutputChunkRow) TableName() string { return "craft_docker_output_chunks" }

// Open validates the current Run/task and the exact already-claimed Docker
// receipt before creating a durable writer operation. The operation's unique
// key is create-only so a restart cannot silently regain append authority.
func (r *CraftDockerOutputRepository) Open(ctx context.Context, scope CraftDockerOutputScope) error {
	if r == nil || r.db == nil || r.maxBytes <= 0 || r.maxBytes > MaxCraftDockerOutputBytes || !validCraftDockerOutputScope(scope) {
		return ErrCraftDockerOutputInvalid
	}
	var run struct{ SessionID string }
	if err := r.db.WithContext(ctx).Table("agent_runs").Select("session_id").Where("tenant_id = ? AND run_id = ?", scope.TenantID, scope.RunID).Take(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCraftDockerOutputConflict
		}
		return dockerOutputDBError(err)
	}
	if run.SessionID != scope.TaskID {
		return ErrCraftDockerOutputConflict
	}
	var count int64
	if err := r.db.WithContext(ctx).Table("craft_charge_start_journal").Where(
		"tenant_id = ? AND run_id = ? AND activity_key = ? AND protocol = 'docker_coordinator' AND provider = 'docker' AND container_id = ? AND exec_id = ? AND send_claimed_at IS NOT NULL",
		scope.TenantID, scope.RunID, scope.ActivityKey, scope.ContainerID, scope.ExecID).Count(&count).Error; err != nil {
		return dockerOutputDBError(err)
	}
	if count != 1 {
		return ErrCraftDockerOutputConflict
	}
	row := craftDockerOutputOperationRow{
		TenantID: scope.TenantID, TaskID: scope.TaskID, RunID: scope.RunID,
		ActivityKey: scope.ActivityKey, ContainerID: scope.ContainerID, ExecID: scope.ExecID,
		MaxBytes: r.maxBytes,
	}
	created := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return dockerOutputDBError(created.Error)
	}
	if created.RowsAffected != 1 {
		return ErrCraftDockerOutputConflict
	}
	return nil
}

// Append commits bytes and their assigned sequence in one transaction. The
// operation-row update serializes competing writers with Seal across DB
// connections; the transaction contains no caller callback.
func (r *CraftDockerOutputRepository) Append(ctx context.Context, scope CraftDockerOutputScope, stream string, chunk []byte) (int64, error) {
	return r.append(ctx, scope, nil, stream, chunk)
}

// AppendSequence is the retry-safe form for a producer with a stable cursor.
// Exact original input replay is idempotent, including quota-truncated input;
// gaps and divergent replays conflict.
func (r *CraftDockerOutputRepository) AppendSequence(ctx context.Context, scope CraftDockerOutputScope, sequence int64, stream string, chunk []byte) error {
	if sequence <= 0 {
		return ErrCraftDockerOutputInvalid
	}
	_, err := r.append(ctx, scope, &sequence, stream, chunk)
	return err
}

func (r *CraftDockerOutputRepository) append(ctx context.Context, scope CraftDockerOutputScope, expected *int64, stream string, chunk []byte) (sequence int64, returnErr error) {
	if r == nil || r.db == nil || !validCraftDockerOutputScope(scope) || (stream != "stdout" && stream != "stderr") || len(chunk) == 0 {
		return 0, ErrCraftDockerOutputInvalid
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var quota bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := tx.Model(&craftDockerOutputOperationRow{}).Where(outputScopeWhere(scope)+" AND sealed_at IS NULL", outputScopeArgs(scope)...).
			UpdateColumn("lock_version", gorm.Expr("lock_version + 1"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			if expected != nil {
				var op craftDockerOutputOperationRow
				if err := tx.Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Take(&op).Error; err == nil {
					if op.TaskID != scope.TaskID || op.ContainerID != scope.ContainerID || op.ExecID != scope.ExecID {
						return ErrCraftDockerOutputConflict
					}
					var replay craftDockerOutputChunkRow
					err := tx.Where(outputScopeWhere(scope)+" AND sequence = ?", append(outputScopeArgs(scope), *expected)...).Take(&replay).Error
					if err == nil {
						if outputReplayMatches(replay, stream, chunk) {
							sequence = replay.Sequence
							quota = replay.InputTruncated
							return nil
						}
						return ErrCraftDockerOutputConflict
					}
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return r.operationStateError(tx, ctx, scope)
		}
		var op craftDockerOutputOperationRow
		if err := tx.Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Take(&op).Error; err != nil {
			return err
		}
		if op.TaskID != scope.TaskID || op.ContainerID != scope.ContainerID || op.ExecID != scope.ExecID {
			return ErrCraftDockerOutputConflict
		}
		if expected != nil {
			var replay craftDockerOutputChunkRow
			err := tx.Where(outputScopeWhere(scope)+" AND sequence = ?", append(outputScopeArgs(scope), *expected)...).Take(&replay).Error
			if err == nil {
				if outputReplayMatches(replay, stream, chunk) {
					sequence = replay.Sequence
					quota = replay.InputTruncated
					return nil
				}
				return ErrCraftDockerOutputConflict
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if op.Truncated {
			return ErrCraftDockerOutputQuota
		}
		next := op.NextSequence + 1
		if expected != nil && *expected != next {
			return ErrCraftDockerOutputConflict
		}
		remaining := op.MaxBytes - op.TotalBytes
		if remaining < 0 {
			return ErrCraftDockerOutputCorrupt
		}
		stored := chunk
		if int64(len(stored)) > remaining {
			stored = stored[:remaining]
			quota = true
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(stored))
		storedBytes := make([]byte, len(stored))
		copy(storedBytes, stored)
		row := craftDockerOutputChunkRow{TenantID: scope.TenantID, RunID: scope.RunID, ActivityKey: scope.ActivityKey,
			Sequence: next, Stream: stream, Bytes: storedBytes, ByteCount: int64(len(stored)), SHA256: digest,
			InputByteCount: int64(len(chunk)), InputSHA256: fmt.Sprintf("%x", sha256.Sum256(chunk)), InputTruncated: quota}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		updates := map[string]any{"next_sequence": next, "total_bytes": op.TotalBytes + int64(len(stored)), "updated_at": time.Now().UTC()}
		if quota {
			updates["truncated"] = true
		}
		if err := tx.Model(&craftDockerOutputOperationRow{}).Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Updates(updates).Error; err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		sequence = next
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrCraftDockerOutputNotFound
		}
		return sequence, dockerOutputDBError(err)
	}
	if quota {
		return sequence, ErrCraftDockerOutputQuota
	}
	return sequence, nil
}

func (r *CraftDockerOutputRepository) Seal(ctx context.Context, scope CraftDockerOutputScope) error {
	if r == nil || r.db == nil || !validCraftDockerOutputScope(scope) {
		return ErrCraftDockerOutputInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := r.operation(ctx, scope); err != nil {
		return err
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&craftDockerOutputOperationRow{}).
		Where(outputScopeWhere(scope)+" AND sealed_at IS NULL", outputScopeArgs(scope)...).
		Updates(map[string]any{"sealed_at": now, "lock_version": gorm.Expr("lock_version + 1"), "updated_at": now})
	if result.Error != nil {
		return dockerOutputDBError(result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var op craftDockerOutputOperationRow
	if err := r.db.WithContext(ctx).Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Take(&op).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCraftDockerOutputNotFound
		}
		return dockerOutputDBError(err)
	}
	if op.SealedAt != nil {
		return nil
	}
	return ErrCraftDockerOutputUnavailable
}

func (r *CraftDockerOutputRepository) ReadAfter(ctx context.Context, scope CraftDockerOutputScope, cursor int64, limit int) ([]CraftDockerOutputChunk, CraftDockerOutputSnapshot, error) {
	if r == nil || r.db == nil || !validCraftDockerOutputScope(scope) {
		return nil, CraftDockerOutputSnapshot{Unavailable: true}, ErrCraftDockerOutputInvalid
	}
	if cursor < 0 || limit <= 0 {
		return nil, CraftDockerOutputSnapshot{}, ErrCraftDockerOutputInvalidCursor
	}
	if limit > 200 {
		limit = 200
	}
	var txOptions *sql.TxOptions
	if r.db.Name() == "postgres" {
		txOptions = &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	}
	var chunks []CraftDockerOutputChunk
	var state CraftDockerOutputSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var op craftDockerOutputOperationRow
		if err := tx.Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Take(&op).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCraftDockerOutputNotFound
			}
			return err
		}
		if op.TaskID != scope.TaskID || op.ContainerID != scope.ContainerID || op.ExecID != scope.ExecID {
			return ErrCraftDockerOutputConflict
		}
		state = outputSnapshot(op)
		if cursor > op.NextSequence {
			return ErrCraftDockerOutputCursorAhead
		}
		var rows []craftDockerOutputChunkRow
		if err := tx.Where(outputScopeWhere(scope)+" AND sequence > ?", append(outputScopeArgs(scope), cursor)...).
			Order("sequence ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		chunks = make([]CraftDockerOutputChunk, len(rows))
		expected := cursor + 1
		for i, row := range rows {
			if row.Sequence != expected || row.ByteCount != int64(len(row.Bytes)) || row.SHA256 != fmt.Sprintf("%x", sha256.Sum256(row.Bytes)) ||
				row.InputByteCount < row.ByteCount || !validOutputDigest(row.InputSHA256) ||
				(!row.InputTruncated && (row.InputByteCount != row.ByteCount || row.InputSHA256 != row.SHA256)) {
				return ErrCraftDockerOutputCorrupt
			}
			expected++
			chunks[i] = CraftDockerOutputChunk{Sequence: row.Sequence, Stream: row.Stream, Bytes: append([]byte(nil), row.Bytes...), SHA256: row.SHA256}
		}
		if int64(len(rows)) < int64(limit) && expected-1 != op.NextSequence {
			return ErrCraftDockerOutputCorrupt
		}
		return nil
	}, txOptions)
	if err != nil {
		if errors.Is(err, ErrCraftDockerOutputNotFound) || errors.Is(err, ErrCraftDockerOutputConflict) ||
			errors.Is(err, ErrCraftDockerOutputCursorAhead) || errors.Is(err, ErrCraftDockerOutputCorrupt) {
			return nil, state, err
		}
		return nil, CraftDockerOutputSnapshot{Unavailable: true}, dockerOutputDBError(err)
	}
	return chunks, state, nil
}

func outputReplayMatches(row craftDockerOutputChunkRow, stream string, input []byte) bool {
	return row.Stream == stream && row.InputByteCount == int64(len(input)) &&
		row.InputSHA256 == fmt.Sprintf("%x", sha256.Sum256(input))
}

func validOutputDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, r := range digest {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func (r *CraftDockerOutputRepository) operation(ctx context.Context, scope CraftDockerOutputScope) (craftDockerOutputOperationRow, error) {
	var op craftDockerOutputOperationRow
	err := r.db.WithContext(ctx).Where(outputScopeWhere(scope), outputScopeArgs(scope)...).Take(&op).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftDockerOutputOperationRow{}, ErrCraftDockerOutputNotFound
	}
	if err != nil {
		return craftDockerOutputOperationRow{}, dockerOutputDBError(err)
	}
	if op.TaskID != scope.TaskID || op.ContainerID != scope.ContainerID || op.ExecID != scope.ExecID {
		return craftDockerOutputOperationRow{}, ErrCraftDockerOutputConflict
	}
	return op, nil
}

func (r *CraftDockerOutputRepository) operationStateError(tx *gorm.DB, ctx context.Context, scope CraftDockerOutputScope) error {
	op, err := r.operation(ctx, scope)
	if err != nil {
		return err
	}
	if op.SealedAt != nil {
		return ErrCraftDockerOutputSealed
	}
	if op.Truncated {
		return ErrCraftDockerOutputQuota
	}
	// Unreachable under the current WHERE shape (sealed_at IS NULL rows are
	// handled above), but any future change to the locking predicate or the
	// state column must never silently swallow an unsealed, untruncated
	// zero-row update as success.
	if tx.Error == nil {
		return ErrCraftDockerOutputUnavailable
	}
	return tx.Error
}

func outputSnapshot(op craftDockerOutputOperationRow) CraftDockerOutputSnapshot {
	return CraftDockerOutputSnapshot{NextSequence: op.NextSequence, TotalBytes: op.TotalBytes,
		Sealed: op.SealedAt != nil, Truncated: op.Truncated,
		// The output store has no authority to attest stream completeness or
		// terminal process evidence. A sealed stream stays partial until a
		// separately reviewed coordinator joins those facts.
		Partial: op.Truncated || op.SealedAt != nil}
}

func validCraftDockerOutputScope(scope CraftDockerOutputScope) bool {
	return scope.TenantID > 0 && strings.TrimSpace(scope.TaskID) != "" && strings.TrimSpace(scope.RunID) != "" &&
		strings.TrimSpace(scope.ActivityKey) != "" && strings.TrimSpace(scope.ContainerID) != "" && strings.TrimSpace(scope.ExecID) != ""
}

func outputScopeWhere(CraftDockerOutputScope) string {
	return "tenant_id = ? AND run_id = ? AND activity_key = ?"
}
func outputScopeArgs(scope CraftDockerOutputScope) []any {
	return []any{scope.TenantID, scope.RunID, scope.ActivityKey}
}

func dockerOutputDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCraftDockerOutputConflict) || errors.Is(err, ErrCraftDockerOutputSealed) ||
		errors.Is(err, ErrCraftDockerOutputQuota) || errors.Is(err, ErrCraftDockerOutputNotFound) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrCraftDockerOutputUnavailable, err)
}

// CraftRunTaskID returns the persisted task/session for a Run. It is used by
// the application layer without allowing callers to invent task ownership.
func (r *CraftDockerOutputRepository) CraftRunTaskID(ctx context.Context, tenantID uint64, runID string) (string, error) {
	if r == nil || r.db == nil || tenantID == 0 || strings.TrimSpace(runID) == "" {
		return "", ErrCraftDockerOutputInvalid
	}
	var row struct{ SessionID string }
	err := r.db.WithContext(ctx).Table("agent_runs").Select("session_id").Where("tenant_id = ? AND run_id = ?", tenantID, runID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrCraftDockerOutputNotFound
	}
	if err != nil {
		return "", dockerOutputDBError(err)
	}
	return row.SessionID, nil
}
