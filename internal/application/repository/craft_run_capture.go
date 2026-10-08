package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftRunCapture is the durable post-terminal capture receipt. Files are
// populated only after the object uploads have completed and the manifest is
// sealed; retries then use these immutable refs rather than rereading output.
type CraftRunCapture struct {
	Scope                          craft.Scope
	WorkspaceID, RunID, Generation string
	PredecessorRevision            int64
	PredecessorState               craft.DraftHeadState
	PredecessorRunID               string
	PredecessorDigest              string
	State                          string
	ManifestDigest                 string
	DraftRevision                  *int64
	Files                          []craft.File
	LastError                      string
}

type craftRunCaptureRow struct {
	TenantID            uint64    `gorm:"column:tenant_id"`
	WorkspaceID         string    `gorm:"column:workspace_id"`
	RunID               string    `gorm:"column:run_id"`
	OwnerID             string    `gorm:"column:owner_id"`
	SessionID           string    `gorm:"column:session_id"`
	Generation          string    `gorm:"column:generation"`
	PredecessorRevision int64     `gorm:"column:predecessor_revision"`
	PredecessorState    string    `gorm:"column:predecessor_state"`
	PredecessorRunID    string    `gorm:"column:predecessor_run_id"`
	PredecessorDigest   string    `gorm:"column:predecessor_digest"`
	State               string    `gorm:"column:state"`
	ManifestDigest      string    `gorm:"column:manifest_digest"`
	DraftRevision       *int64    `gorm:"column:draft_revision"`
	LastError           string    `gorm:"column:last_error"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
}

func (craftRunCaptureRow) TableName() string { return "craft_run_captures" }

type craftRunCaptureFileRow struct {
	TenantID    uint64 `gorm:"column:tenant_id"`
	WorkspaceID string `gorm:"column:workspace_id"`
	RunID       string `gorm:"column:run_id"`
	Path        string `gorm:"column:path"`
	ObjectRef   string `gorm:"column:object_ref"`
	SHA256      string `gorm:"column:sha256"`
	Bytes       int64  `gorm:"column:bytes"`
	MIME        string `gorm:"column:mime"`
}

type craftCaptureSeed struct {
	WorkspaceID    string               `json:"workspace_id"`
	State          craft.DraftHeadState `json:"state"`
	DraftRevision  int64                `json:"draft_revision"`
	SourceRunID    string               `json:"source_run_id"`
	ManifestDigest string               `json:"manifest_digest"`
}

func decodeCaptureSeed(snapshot []byte) (craftCaptureSeed, error) {
	var envelope struct {
		Seed *craftCaptureSeed `json:"craft_workspace_seed"`
	}
	if err := json.Unmarshal(snapshot, &envelope); err != nil {
		return craftCaptureSeed{}, fmt.Errorf("%w: decode admission-frozen Workspace seed", craft.ErrInvalidInput)
	}
	if envelope.Seed == nil {
		return craftCaptureSeed{}, fmt.Errorf("%w: Run has no admission-frozen Workspace seed", craft.ErrConflict)
	}
	return *envelope.Seed, nil
}

func (s craftCaptureSeed) validate(workspaceID string) error {
	if s.WorkspaceID == "" || s.WorkspaceID != workspaceID || s.DraftRevision < 0 {
		return fmt.Errorf("%w: invalid admission-frozen Workspace seed", craft.ErrConflict)
	}
	switch s.State {
	case craft.DraftHeadEmpty:
		if s.DraftRevision != 0 || s.SourceRunID != "" || s.ManifestDigest != "" {
			return fmt.Errorf("%w: invalid empty admission-frozen seed", craft.ErrConflict)
		}
	case craft.DraftHeadSelected:
		if s.DraftRevision < 1 || s.SourceRunID == "" || !craft.ValidSHA256(s.ManifestDigest) {
			return fmt.Errorf("%w: invalid selected admission-frozen seed", craft.ErrConflict)
		}
	default:
		return fmt.Errorf("%w: unknown admission-frozen seed state", craft.ErrConflict)
	}
	return nil
}

func (craftRunCaptureFileRow) TableName() string { return "craft_run_capture_files" }

type CraftRunCaptureStore struct {
	db    *gorm.DB
	ticks uint64
}

func NewCraftRunCaptureStore(db *gorm.DB) *CraftRunCaptureStore { return &CraftRunCaptureStore{db: db} }

// craftCaptureDrainWindow is how long the global scan stays OFF a receipt
// after any write, so the immediate post-terminal drain owns it exclusively
// (each redo uploads fresh physical objects; the loser's uploads leak as
// unreferenced resource rows).
const craftCaptureDrainWindow = 30 * time.Second

// RecoverPendingTick advances one periodic-scan round: the expensive
// multi-table JOIN synthesis (full-history JSON predicates, unindexable)
// runs only every craftCaptureSynthesisEveryNth tick; every tick advances
// durable receipts with the indexed state query plus the freshness gate.
const craftCaptureSynthesisEveryNth = 20

func (s *CraftRunCaptureStore) RecoverPendingTick(ctx context.Context, limit int) ([]CraftRunCapture, error) {
	s.ticks++
	return s.recoverPending(ctx, limit, s.ticks%craftCaptureSynthesisEveryNth == 1)
}

// EnsurePending creates the frozen receipt for one terminal Run, or returns
// the existing receipt. The complete server-owned identity and quiescent SQL
// fences are checked in the same transaction as insertion.
func (s *CraftRunCaptureStore) EnsurePending(ctx context.Context, scope craft.Scope, workspaceID, runID, generation string) (CraftRunCapture, error) {
	if s == nil || s.db == nil || scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" || runID == "" || generation == "" {
		return CraftRunCapture{}, fmt.Errorf("%w: incomplete Run capture identity", craft.ErrInvalidInput)
	}
	var out CraftRunCapture
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		var authority struct {
			OwnerID     string  `gorm:"column:owner_id"`
			SessionID   string  `gorm:"column:session_id"`
			Status      string  `gorm:"column:status"`
			ActiveRunID *string `gorm:"column:active_agent_run_id"`
			Generation  string  `gorm:"column:generation"`
			ViewState   string  `gorm:"column:view_state"`
			Snapshot    []byte  `gorm:"column:snapshot"`
		}
		err = tx.Raw(`SELECT r.owner_id,r.session_id,r.status,s.active_agent_run_id,
			r.snapshot,v.generation,v.state AS view_state
			FROM agent_runs r JOIN sessions s ON s.tenant_id=r.tenant_id AND s.id=r.session_id
			JOIN craft_run_views v ON v.tenant_id=r.tenant_id AND v.run_id=r.run_id
			JOIN craft_workspaces w ON w.tenant_id=r.tenant_id AND w.session_id=r.session_id AND w.owner_id=r.owner_id AND w.id=?
			WHERE r.tenant_id=? AND r.run_id=?`, workspaceID, scope.TenantID, runID).Take(&authority).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		if err != nil {
			return err
		}
		if authority.OwnerID != scope.UserID {
			return craft.ErrForbidden
		}
		if authority.SessionID != scope.SessionID {
			return craft.ErrNotFound
		}
		if authority.Status != "succeeded" && authority.Status != "failed" && authority.Status != "canceled" {
			return fmt.Errorf("%w: Run is not terminal", craft.ErrConflict)
		}
		if authority.ActiveRunID != nil && *authority.ActiveRunID != "" {
			return fmt.Errorf("%w: Run writer slot is not empty", craft.ErrConflict)
		}
		if authority.ViewState != string(craft.RunViewStateBound) || authority.Generation != generation {
			return fmt.Errorf("%w: Run generation is not bound", craft.ErrConflict)
		}
		var pending int64
		if err := tx.Table("agent_tool_calls").Where("tenant_id=? AND run_id=? AND status NOT IN ('succeeded','failed')", scope.TenantID, runID).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return fmt.Errorf("%w: pending tool call", craft.ErrConflict)
		}
		if err := tx.Table("craft_delegations").Where("tenant_id=? AND run_id=? AND status NOT IN ('succeeded','failed','canceled')", scope.TenantID, runID).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return fmt.Errorf("%w: pending delegation", craft.ErrConflict)
		}
		seed, err := decodeCaptureSeed(authority.Snapshot)
		if err != nil {
			return err
		}
		if err := seed.validate(workspaceID); err != nil {
			return err
		}
		var existing craftRunCaptureRow
		err = tx.Where("tenant_id=? AND workspace_id=? AND run_id=?", scope.TenantID, workspaceID, runID).Take(&existing).Error
		if err == nil {
			if existing.OwnerID != scope.UserID || existing.SessionID != scope.SessionID {
				return craft.ErrForbidden
			}
			if existing.Generation != generation || existing.PredecessorRevision != seed.DraftRevision || existing.PredecessorState != string(seed.State) || existing.PredecessorRunID != seed.SourceRunID || existing.PredecessorDigest != seed.ManifestDigest {
				return fmt.Errorf("%w: Run capture generation or frozen predecessor changed", craft.ErrConflict)
			}
			out = captureValue(existing, nil)
			if existing.State == "sealed" || existing.State == "advanced" {
				out, err = loadSealedCaptureFiles(ctx, tx, existing)
				if err != nil {
					return err
				}
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current struct {
			Revision    int64   `gorm:"column:revision"`
			State       string  `gorm:"column:state"`
			SourceRunID *string `gorm:"column:source_run_id"`
			Digest      *string `gorm:"column:manifest_digest"`
		}
		if err := tx.Table("craft_workspace_draft_heads").Where("tenant_id=? AND workspace_id=?", scope.TenantID, workspaceID).Take(&current).Error; err != nil {
			return err
		}
		currentRun, currentDigest := "", ""
		if current.SourceRunID != nil {
			currentRun = *current.SourceRunID
		}
		if current.Digest != nil {
			currentDigest = *current.Digest
		}
		if current.Revision != seed.DraftRevision || craft.DraftHeadState(current.State) != seed.State || currentRun != seed.SourceRunID || currentDigest != seed.ManifestDigest {
			return fmt.Errorf("%w: current draft differs from the admission-frozen predecessor", craft.ErrConflict)
		}
		row := craftRunCaptureRow{TenantID: scope.TenantID, WorkspaceID: workspaceID, RunID: runID, OwnerID: scope.UserID, SessionID: scope.SessionID, Generation: generation, PredecessorRevision: seed.DraftRevision, PredecessorState: string(seed.State), PredecessorRunID: seed.SourceRunID, PredecessorDigest: seed.ManifestDigest, State: "pending"}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		var stored craftRunCaptureRow
		if err := tx.Where("tenant_id=? AND workspace_id=? AND run_id=?", scope.TenantID, workspaceID, runID).Take(&stored).Error; err != nil {
			return err
		}
		if stored.OwnerID != scope.UserID || stored.SessionID != scope.SessionID || stored.Generation != generation || stored.PredecessorRevision != seed.DraftRevision || stored.PredecessorState != string(seed.State) || stored.PredecessorRunID != seed.SourceRunID || stored.PredecessorDigest != seed.ManifestDigest {
			return fmt.Errorf("%w: Run capture predecessor or generation changed", craft.ErrConflict)
		}
		out = captureValue(stored, nil)
		return nil
	})
	return out, err
}

// BeginCapture durably records the exact path/hash/size manifest identity
// before uploading objects. A retry after upload-before-seal may reread only
// the same quiescent bytes; a changed manifest for the same Run conflicts.
// ClaimForDrain atomically moves a pending receipt into the capturing
// state (refreshing updated_at) WITHOUT pinning any manifest digest: the
// freshness gate then excludes it from the periodic ticker for the whole
// quiescence/staging phase, while the later BeginCapture with the real
// digest stays idempotent.
func (s *CraftRunCaptureStore) ClaimForDrain(ctx context.Context, receipt CraftRunCapture) (CraftRunCapture, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&craftRunCaptureRow{}).
			Where("tenant_id=? AND workspace_id=? AND run_id=? AND state = 'pending'", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).
			Updates(map[string]any{"state": "capturing", "last_error": "", "updated_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// The CAS matched nothing: another path already owns the
			// receipt (or it moved on). Reporting success here would let
			// the caller walk the tree anyway — the exact double-capture
			// (two full sha256 walks, two uploads, one orphaned resource
			// row) the freshness gate exists to prevent.
			return fmt.Errorf("%w: capture receipt %s already claimed", craft.ErrConflict, receipt.RunID)
		}
		return nil
	})
	if err != nil {
		return receipt, err
	}
	receipt.State = "capturing"
	return receipt, nil
}

func (s *CraftRunCaptureStore) BeginCapture(ctx context.Context, receipt CraftRunCapture, digest string) (CraftRunCapture, error) {
	if !craft.ValidSHA256(digest) {
		return CraftRunCapture{}, fmt.Errorf("%w: empty capture attempt digest", craft.ErrInvalidInput)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftRunCaptureRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&row).Error; err != nil {
			return err
		}
		if row.Generation != receipt.Generation || row.PredecessorRevision != receipt.PredecessorRevision || row.PredecessorState != string(receipt.PredecessorState) || row.PredecessorRunID != receipt.PredecessorRunID || row.PredecessorDigest != receipt.PredecessorDigest {
			return craft.ErrConflict
		}
		if row.State == "sealed" || row.State == "advanced" {
			if row.ManifestDigest != digest {
				return craft.ErrConflict
			}
			return nil
		}
		if row.ManifestDigest != "" && row.ManifestDigest != digest {
			return fmt.Errorf("%w: Run output changed after capture began", craft.ErrConflict)
		}
		return tx.Model(&craftRunCaptureRow{}).Where("tenant_id=? AND workspace_id=? AND run_id=? AND state IN ('pending','capturing','blocked')", row.TenantID, row.WorkspaceID, row.RunID).
			Updates(map[string]any{"state": "capturing", "manifest_digest": digest, "last_error": "", "updated_at": time.Now()}).Error
	})
	if err != nil {
		return CraftRunCapture{}, err
	}
	var row craftRunCaptureRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&row).Error; err != nil {
		return CraftRunCapture{}, err
	}
	var files []craftRunCaptureFileRow
	if row.State == "sealed" || row.State == "advanced" {
		if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Order("path ASC").Find(&files).Error; err != nil {
			return CraftRunCapture{}, err
		}
	}
	return captureValue(row, files), nil
}

// RecoverPending returns durable receipts and synthesizes receipts for terminal
// Runs missed by a worker after its terminal transaction. Only a still-current
// admission-frozen predecessor is synthesized; mismatches stay fenced.
func (s *CraftRunCaptureStore) RecoverPending(ctx context.Context, limit int) ([]CraftRunCapture, error) {
	return s.recoverPending(ctx, limit, true)
}

func (s *CraftRunCaptureStore) recoverPending(ctx context.Context, limit int, synthesize bool) ([]CraftRunCapture, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []craftRunCaptureRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if synthesize {
			// Recovery accepts only the predecessor frozen into the immutable
			// admission snapshot and still current in durable draft history.
			if err := tx.Exec(craftCaptureRecoveryInsertSQL(tx.Dialector.Name())).Error; err != nil {
				return err
			}
		}
		// Freshness gate: a CAPTURING receipt touched within the drain
		// window is being processed by the immediate post-terminal drain
		// right now — the ticker must not redo it concurrently (each redo
		// uploads fresh physical objects; the loser's uploads leak). Sealed
		// rows (crash after seal before draft CAS) and pending rows stay
		// immediately recoverable.
		return tx.Where("state IN ('pending','capturing','sealed','blocked')").
			Where("state != 'capturing' OR updated_at IS NULL OR updated_at < ?", time.Now().Add(-craftCaptureDrainWindow)).
			Order("created_at ASC").Limit(limit).Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]CraftRunCapture, 0, len(rows))
	for _, row := range rows {
		var files []craftRunCaptureFileRow
		if row.State == "sealed" {
			if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Order("path ASC").Find(&files).Error; err != nil {
				return nil, err
			}
		}
		out = append(out, captureValue(row, files))
	}
	return out, nil
}

func craftCaptureRecoveryInsertSQL(dialect string) string {
	if dialect == "postgres" {
		return `INSERT INTO craft_run_captures (tenant_id,workspace_id,run_id,owner_id,session_id,generation,predecessor_revision,predecessor_state,predecessor_run_id,predecessor_digest,state)
		SELECT r.tenant_id,w.id,r.run_id,r.owner_id,r.session_id,v.generation,
		CAST(r.snapshot->'craft_workspace_seed'->>'draft_revision' AS BIGINT),r.snapshot->'craft_workspace_seed'->>'state',
		COALESCE(r.snapshot->'craft_workspace_seed'->>'source_run_id',''),COALESCE(r.snapshot->'craft_workspace_seed'->>'manifest_digest',''),'pending'
		FROM agent_runs r JOIN craft_run_views v ON v.tenant_id=r.tenant_id AND v.run_id=r.run_id AND v.state='bound'
		JOIN craft_workspaces w ON w.tenant_id=r.tenant_id AND w.session_id=r.session_id AND w.owner_id=r.owner_id AND w.id=(r.snapshot->'craft_workspace_seed'->>'workspace_id')
		JOIN craft_workspace_draft_heads h ON h.tenant_id=w.tenant_id AND h.workspace_id=w.id
		WHERE r.status IN ('succeeded','failed','canceled') AND jsonb_typeof(r.snapshot->'craft_workspace_seed')='object'
		AND h.revision=CAST(r.snapshot->'craft_workspace_seed'->>'draft_revision' AS BIGINT)
		AND h.state=(r.snapshot->'craft_workspace_seed'->>'state')
		AND COALESCE(h.source_run_id,'')=COALESCE(r.snapshot->'craft_workspace_seed'->>'source_run_id','')
		AND COALESCE(h.manifest_digest,'')=COALESCE(r.snapshot->'craft_workspace_seed'->>'manifest_digest','')
		ON CONFLICT (tenant_id,workspace_id,run_id) DO NOTHING`
	}
	return `INSERT INTO craft_run_captures (tenant_id,workspace_id,run_id,owner_id,session_id,generation,predecessor_revision,predecessor_state,predecessor_run_id,predecessor_digest,state)
	SELECT r.tenant_id,w.id,r.run_id,r.owner_id,r.session_id,v.generation,
	CAST(json_extract(r.snapshot,'$.craft_workspace_seed.draft_revision') AS INTEGER),json_extract(r.snapshot,'$.craft_workspace_seed.state'),
	COALESCE(json_extract(r.snapshot,'$.craft_workspace_seed.source_run_id'),''),COALESCE(json_extract(r.snapshot,'$.craft_workspace_seed.manifest_digest'),''),'pending'
	FROM agent_runs r JOIN craft_run_views v ON v.tenant_id=r.tenant_id AND v.run_id=r.run_id AND v.state='bound'
	JOIN craft_workspaces w ON w.tenant_id=r.tenant_id AND w.session_id=r.session_id AND w.owner_id=r.owner_id AND w.id=json_extract(r.snapshot,'$.craft_workspace_seed.workspace_id')
	JOIN craft_workspace_draft_heads h ON h.tenant_id=w.tenant_id AND h.workspace_id=w.id
	WHERE r.status IN ('succeeded','failed','canceled') AND json_type(r.snapshot,'$.craft_workspace_seed')='object'
	AND h.revision=CAST(json_extract(r.snapshot,'$.craft_workspace_seed.draft_revision') AS INTEGER)
	AND h.state=json_extract(r.snapshot,'$.craft_workspace_seed.state')
	AND COALESCE(h.source_run_id,'')=COALESCE(json_extract(r.snapshot,'$.craft_workspace_seed.source_run_id'),'')
	AND COALESCE(h.manifest_digest,'')=COALESCE(json_extract(r.snapshot,'$.craft_workspace_seed.manifest_digest'),'')
	ON CONFLICT(tenant_id,workspace_id,run_id) DO NOTHING`
}

// RecoverPendingForRun returns the still-unadvanced receipts of one Run. It
// deliberately performs no global missing-receipt synthesis: the per-run
// drain is a cheap targeted post-terminal path, and the periodic scan owns
// synthesis for receipts stranded between the terminal commit and enqueue.
func (s *CraftRunCaptureStore) RecoverPendingForRun(ctx context.Context, tenantID uint64, runID string) ([]CraftRunCapture, error) {
	if tenantID == 0 || runID == "" {
		return nil, fmt.Errorf("%w: per-run capture recovery requires the Run identity", craft.ErrInvalidInput)
	}
	var rows []craftRunCaptureRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ? AND state IN ('pending','capturing','sealed','blocked')", tenantID, runID).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CraftRunCapture, 0, len(rows))
	for _, row := range rows {
		var files []craftRunCaptureFileRow
		if row.State == "sealed" {
			if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Order("path ASC").Find(&files).Error; err != nil {
				return nil, err
			}
		}
		out = append(out, captureValue(row, files))
	}
	return out, nil
}

func (s *CraftRunCaptureStore) Seal(ctx context.Context, receipt CraftRunCapture, files []craft.File, digest string) (CraftRunCapture, error) {
	if digest == "" || len(files) == 0 {
		return CraftRunCapture{}, fmt.Errorf("%w: empty capture manifest", craft.ErrInvalidInput)
	}
	actualDigest, err := craft.ManifestDigest(files)
	if err != nil {
		return CraftRunCapture{}, err
	}
	if actualDigest != digest {
		return CraftRunCapture{}, craft.ErrConflict
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftRunCaptureRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&row).Error; err != nil {
			return err
		}
		if row.Generation != receipt.Generation || row.PredecessorRevision != receipt.PredecessorRevision || row.PredecessorState != string(receipt.PredecessorState) || row.PredecessorRunID != receipt.PredecessorRunID || row.PredecessorDigest != receipt.PredecessorDigest {
			return craft.ErrConflict
		}
		if row.State == "sealed" || row.State == "advanced" {
			var old []craftRunCaptureFileRow
			if err := tx.Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Order("path ASC").Find(&old).Error; err != nil {
				return err
			}
			stored := captureValue(row, old)
			if stored.ManifestDigest != digest || !sameCaptureFiles(stored.Files, files) {
				return craft.ErrConflict
			}
			return nil
		}
		if row.State != "capturing" || row.ManifestDigest != digest {
			return fmt.Errorf("%w: sealed refs differ from capture attempt", craft.ErrConflict)
		}
		fr := make([]craftRunCaptureFileRow, 0, len(files))
		for _, f := range files {
			fr = append(fr, craftRunCaptureFileRow{TenantID: row.TenantID, WorkspaceID: row.WorkspaceID, RunID: row.RunID, Path: f.Path, ObjectRef: f.Ref, SHA256: f.SHA256, Bytes: f.Bytes, MIME: f.MIME})
		}
		if err := tx.Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Delete(&craftRunCaptureFileRow{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&fr).Error; err != nil {
			return err
		}
		return tx.Model(&craftRunCaptureRow{}).Where("tenant_id=? AND workspace_id=? AND run_id=? AND state IN ('pending','capturing','blocked')", row.TenantID, row.WorkspaceID, row.RunID).Updates(map[string]any{"state": "sealed", "manifest_digest": digest, "last_error": "", "updated_at": time.Now()}).Error
	})
	if err != nil {
		return CraftRunCapture{}, err
	}
	var stored craftRunCaptureRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return CraftRunCapture{}, craft.ErrNotFound
		}
		return CraftRunCapture{}, err
	}
	if stored.OwnerID != receipt.Scope.UserID || stored.SessionID != receipt.Scope.SessionID || stored.Generation != receipt.Generation || stored.PredecessorRevision != receipt.PredecessorRevision || stored.PredecessorState != string(receipt.PredecessorState) || stored.PredecessorRunID != receipt.PredecessorRunID || stored.PredecessorDigest != receipt.PredecessorDigest || stored.ManifestDigest != digest || (stored.State != "sealed" && stored.State != "advanced") {
		return CraftRunCapture{}, craft.ErrConflict
	}
	return loadSealedCaptureFiles(ctx, s.db.WithContext(ctx), stored)
}

// VerifySealedRefs verifies the current scoped receipt, immutable file rows,
// and resource-catalog ownership before the service opens object bytes. Craft
// capture uploads always use persistent resource:// handles; raw provider
// paths have no authoritative tenant metadata and are rejected.
func (s *CraftRunCaptureStore) VerifySealedRefs(ctx context.Context, receipt CraftRunCapture) error {
	if s == nil || s.db == nil || receipt.Scope.TenantID == 0 || receipt.Scope.UserID == "" || receipt.Scope.SessionID == "" || receipt.WorkspaceID == "" || receipt.RunID == "" {
		return fmt.Errorf("%w: incomplete sealed capture identity", craft.ErrInvalidInput)
	}
	var row craftRunCaptureRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		return err
	}
	if row.OwnerID != receipt.Scope.UserID || row.SessionID != receipt.Scope.SessionID {
		return craft.ErrForbidden
	}
	if row.Generation != receipt.Generation || row.PredecessorRevision != receipt.PredecessorRevision || row.PredecessorState != string(receipt.PredecessorState) || row.PredecessorRunID != receipt.PredecessorRunID || row.PredecessorDigest != receipt.PredecessorDigest || row.ManifestDigest != receipt.ManifestDigest || row.State != "sealed" {
		return craft.ErrConflict
	}
	loaded, err := loadSealedCaptureFiles(ctx, s.db.WithContext(ctx), row)
	if err != nil {
		return err
	}
	if !sameCaptureFiles(loaded.Files, receipt.Files) {
		return craft.ErrConflict
	}
	for _, file := range loaded.Files {
		handle, ok := types.ParseResourcePath(file.Ref)
		if !ok {
			return fmt.Errorf("%w: capture object has no tenant-scoped resource identity", craft.ErrForbidden)
		}
		var resource struct {
			TenantID    uint64 `gorm:"column:tenant_id"`
			Size        int64  `gorm:"column:size"`
			ContentHash string `gorm:"column:content_hash"`
			State       string `gorm:"column:state"`
		}
		if err := s.db.WithContext(ctx).Table("resources").Select("tenant_id,size,content_hash,state").Where("handle=?", handle).Take(&resource).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return craft.ErrNotFound
			}
			return err
		}
		if resource.TenantID != receipt.Scope.TenantID {
			return craft.ErrForbidden
		}
		if resource.State != types.ResourceStateActive || resource.Size != file.Bytes || resource.ContentHash != file.SHA256 {
			return craft.ErrConflict
		}
	}
	return nil
}

func loadSealedCaptureFiles(ctx context.Context, db *gorm.DB, row craftRunCaptureRow) (CraftRunCapture, error) {
	var files []craftRunCaptureFileRow
	if err := db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", row.TenantID, row.WorkspaceID, row.RunID).Order("path ASC").Find(&files).Error; err != nil {
		return CraftRunCapture{}, err
	}
	if len(files) == 0 {
		return CraftRunCapture{}, fmt.Errorf("%w: sealed capture has no files", craft.ErrConflict)
	}
	loaded := captureValue(row, files)
	digest, err := craft.ManifestDigest(loaded.Files)
	if err != nil {
		return CraftRunCapture{}, err
	}
	if digest != row.ManifestDigest {
		return CraftRunCapture{}, fmt.Errorf("%w: sealed capture file rows do not match the manifest", craft.ErrConflict)
	}
	return loaded, nil
}

func (s *CraftRunCaptureStore) MarkAdvanced(ctx context.Context, receipt CraftRunCapture, revision int64) error {
	if revision <= receipt.PredecessorRevision || receipt.ManifestDigest == "" {
		return craft.ErrConflict
	}
	result := s.db.WithContext(ctx).Model(&craftRunCaptureRow{}).
		Where("tenant_id=? AND workspace_id=? AND run_id=? AND generation=? AND predecessor_revision=? AND predecessor_state=? AND predecessor_run_id=? AND predecessor_digest=? AND manifest_digest=? AND state='sealed'", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID, receipt.Generation, receipt.PredecessorRevision, receipt.PredecessorState, receipt.PredecessorRunID, receipt.PredecessorDigest, receipt.ManifestDigest).
		Updates(map[string]any{"state": "advanced", "draft_revision": revision, "last_error": "", "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var stored craftRunCaptureRow
	err := s.db.WithContext(ctx).Where("tenant_id=? AND workspace_id=? AND run_id=?", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Take(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if stored.State == "advanced" && stored.Generation == receipt.Generation && stored.PredecessorRevision == receipt.PredecessorRevision && stored.PredecessorState == string(receipt.PredecessorState) && stored.PredecessorRunID == receipt.PredecessorRunID && stored.PredecessorDigest == receipt.PredecessorDigest && stored.ManifestDigest == receipt.ManifestDigest && stored.DraftRevision != nil && *stored.DraftRevision == revision {
		return nil
	}
	return craft.ErrConflict
}

func (s *CraftRunCaptureStore) MarkPendingError(ctx context.Context, receipt CraftRunCapture, err error) error {
	message := "capture is pending"
	if err != nil {
		message = err.Error()
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	return s.db.WithContext(ctx).Model(&craftRunCaptureRow{}).Where("tenant_id=? AND workspace_id=? AND run_id=? AND state<>'advanced'", receipt.Scope.TenantID, receipt.WorkspaceID, receipt.RunID).Updates(map[string]any{"state": gorm.Expr("CASE WHEN state IN ('sealed','capturing') THEN state ELSE 'pending' END"), "last_error": message, "updated_at": time.Now()}).Error
}

func captureValue(row craftRunCaptureRow, fileRows []craftRunCaptureFileRow) CraftRunCapture {
	out := CraftRunCapture{Scope: craft.Scope{TenantID: row.TenantID, UserID: row.OwnerID, SessionID: row.SessionID}, WorkspaceID: row.WorkspaceID, RunID: row.RunID, Generation: row.Generation, PredecessorRevision: row.PredecessorRevision, PredecessorState: craft.DraftHeadState(row.PredecessorState), PredecessorRunID: row.PredecessorRunID, PredecessorDigest: row.PredecessorDigest, State: row.State, ManifestDigest: row.ManifestDigest, DraftRevision: row.DraftRevision, LastError: row.LastError, Files: make([]craft.File, 0, len(fileRows))}
	for _, f := range fileRows {
		out.Files = append(out.Files, craft.File{Path: f.Path, Ref: f.ObjectRef, SHA256: f.SHA256, Bytes: f.Bytes, MIME: f.MIME})
	}
	return out
}
func sameCaptureFiles(a, b []craft.File) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
