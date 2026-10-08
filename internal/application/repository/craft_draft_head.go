package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CraftDraftHeadStore struct{ db *gorm.DB }

var _ craft.DraftHeadStore = (*CraftDraftHeadStore)(nil)

func NewCraftDraftHeadStore(db *gorm.DB) *CraftDraftHeadStore {
	return &CraftDraftHeadStore{db: db}
}

type craftDraftHeadRow struct {
	WorkspaceID    string    `gorm:"column:workspace_id;primaryKey"`
	TenantID       uint64    `gorm:"column:tenant_id"`
	Revision       int64     `gorm:"column:revision"`
	State          string    `gorm:"column:state"`
	SourceRunID    *string   `gorm:"column:source_run_id"`
	ManifestDigest *string   `gorm:"column:manifest_digest"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (craftDraftHeadRow) TableName() string { return "craft_workspace_draft_heads" }

type craftDraftOriginRow struct {
	WorkspaceID    string `gorm:"column:workspace_id;primaryKey"`
	TenantID       uint64 `gorm:"column:tenant_id"`
	OriginRevision int64  `gorm:"column:origin_revision"`
	OriginState    string `gorm:"column:origin_state"`
}

func (craftDraftOriginRow) TableName() string { return "craft_workspace_draft_origins" }

type craftDraftRevisionRow struct {
	WorkspaceID    string    `gorm:"column:workspace_id"`
	Revision       int64     `gorm:"column:revision"`
	TenantID       uint64    `gorm:"column:tenant_id"`
	SourceRunID    string    `gorm:"column:source_run_id"`
	ManifestDigest string    `gorm:"column:manifest_digest"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (craftDraftRevisionRow) TableName() string { return "craft_workspace_draft_revisions" }

type craftDraftFileRow struct {
	WorkspaceID string `gorm:"column:workspace_id"`
	Revision    int64  `gorm:"column:revision"`
	Path        string `gorm:"column:path"`
	ObjectRef   string `gorm:"column:object_ref"`
	SHA256      string `gorm:"column:sha256"`
	Bytes       int64  `gorm:"column:bytes"`
	MIME        string `gorm:"column:mime"`
}

func (craftDraftFileRow) TableName() string { return "craft_workspace_draft_files" }

func (s *CraftDraftHeadStore) Read(ctx context.Context, scope craft.Scope, workspaceID string) (craft.DraftHead, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" {
		return craft.DraftHead{}, fmt.Errorf("%w: incomplete draft head scope", craft.ErrInvalidInput)
	}
	// One joined SELECT binds authorization and projects the head, its
	// immutable revision row and files from one statement snapshot. Separate
	// reads would permit an owner transfer or CAS head change between queries.
	type resultRow struct {
		WorkspaceID   string  `gorm:"column:workspace_id"`
		HeadID        *string `gorm:"column:head_id"`
		TenantID      *uint64 `gorm:"column:head_tenant_id"`
		Revision      *int64  `gorm:"column:head_revision"`
		State         *string `gorm:"column:head_state"`
		SourceRunID   *string `gorm:"column:head_source_run_id"`
		Manifest      *string `gorm:"column:head_manifest_digest"`
		RevisionRunID *string `gorm:"column:revision_source_run_id"`
		RevisionHash  *string `gorm:"column:revision_manifest_digest"`
		Path          *string `gorm:"column:file_path"`
		ObjectRef     *string `gorm:"column:object_ref"`
		SHA256        *string `gorm:"column:sha256"`
		Bytes         *int64  `gorm:"column:file_bytes"`
		MIME          *string `gorm:"column:mime"`
	}
	var rows []resultRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT w.id AS workspace_id,
		       h.workspace_id AS head_id, h.tenant_id AS head_tenant_id,
		       h.revision AS head_revision, h.state AS head_state,
		       h.source_run_id AS head_source_run_id, h.manifest_digest AS head_manifest_digest,
		       r.source_run_id AS revision_source_run_id, r.manifest_digest AS revision_manifest_digest,
		       f.path AS file_path, f.object_ref, f.sha256, f.bytes AS file_bytes, f.mime
		FROM craft_workspaces w
		LEFT JOIN craft_workspace_draft_heads h
		  ON h.workspace_id = w.id AND h.tenant_id = w.tenant_id
		LEFT JOIN craft_workspace_draft_revisions r
		  ON r.workspace_id = h.workspace_id AND r.tenant_id = h.tenant_id
		 AND r.revision = h.revision AND h.state = 'selected'
		LEFT JOIN craft_workspace_draft_files f
		  ON f.workspace_id = r.workspace_id AND f.revision = r.revision
		WHERE w.id = ? AND w.tenant_id = ? AND w.session_id = ? AND w.owner_id = ?
		ORDER BY f.path`, workspaceID, scope.TenantID, scope.SessionID, scope.UserID).Scan(&rows).Error
	if err != nil {
		return craft.DraftHead{}, err
	}
	if len(rows) == 0 {
		var exists int64
		if err := s.db.WithContext(ctx).Model(&craftWorkspaceRow{}).Where("id = ? AND tenant_id = ? AND session_id = ?", workspaceID, scope.TenantID, scope.SessionID).Count(&exists).Error; err != nil {
			return craft.DraftHead{}, err
		}
		if exists > 0 {
			return craft.DraftHead{}, craft.ErrForbidden
		}
		return craft.DraftHead{}, craft.ErrNotFound
	}
	first := rows[0]
	if first.HeadID == nil {
		return craft.DraftHead{}, craft.ErrDraftHeadUnresolved
	}
	if first.Revision == nil || first.State == nil || first.TenantID == nil || *first.TenantID != scope.TenantID {
		return craft.DraftHead{}, fmt.Errorf("%w: corrupt persisted draft head identity", craft.ErrInvalidInput)
	}
	head := craft.DraftHead{WorkspaceID: *first.HeadID, Revision: *first.Revision, State: craft.DraftHeadState(*first.State)}
	if first.SourceRunID != nil {
		head.SourceRunID = *first.SourceRunID
	}
	if first.Manifest != nil {
		head.ManifestDigest = *first.Manifest
	}
	if head.State == craft.DraftHeadSelected {
		if first.RevisionRunID == nil || first.RevisionHash == nil || *first.RevisionRunID != head.SourceRunID || *first.RevisionHash != head.ManifestDigest {
			return craft.DraftHead{}, fmt.Errorf("%w: selected head does not match immutable revision", craft.ErrInvalidInput)
		}
		head.Files = make([]craft.File, 0, len(rows))
		for _, row := range rows {
			if row.Path == nil || row.ObjectRef == nil || row.SHA256 == nil || row.Bytes == nil || row.MIME == nil {
				return craft.DraftHead{}, fmt.Errorf("%w: incomplete persisted draft file", craft.ErrInvalidInput)
			}
			head.Files = append(head.Files, craft.File{Path: *row.Path, Ref: *row.ObjectRef, SHA256: *row.SHA256, Bytes: *row.Bytes, MIME: *row.MIME})
		}
	}
	if err := head.Validate(); err != nil {
		return craft.DraftHead{}, fmt.Errorf("%w: corrupt persisted draft head", craft.ErrInvalidInput)
	}
	return head.Clone(), nil
}

// ReadRevision loads one immutable draft revision by the identity frozen in an
// admitted Run. Authorization, revision metadata, source Run and files share
// one SELECT snapshot so a concurrent Workspace owner change cannot authorize
// files selected under another owner. Revision zero is the explicit empty
// predecessor; it has no revision/file rows by schema design.
func (s *CraftDraftHeadStore) ReadRevision(ctx context.Context, scope craft.Scope, workspaceID string, revision int64) (craft.DraftHead, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" || revision < 0 {
		return craft.DraftHead{}, fmt.Errorf("%w: incomplete draft revision scope", craft.ErrInvalidInput)
	}
	type resultRow struct {
		WorkspaceID   string  `gorm:"column:workspace_id"`
		HeadID        *string `gorm:"column:head_id"`
		HeadTenantID  *uint64 `gorm:"column:head_tenant_id"`
		HeadRevision  *int64  `gorm:"column:head_revision"`
		HeadState     *string `gorm:"column:head_state"`
		HeadSourceRun *string `gorm:"column:head_source_run_id"`
		HeadManifest  *string `gorm:"column:head_manifest_digest"`
		OriginID      *string `gorm:"column:origin_workspace_id"`
		OriginTenant  *uint64 `gorm:"column:origin_tenant_id"`
		OriginVersion *int64  `gorm:"column:origin_revision"`
		OriginState   *string `gorm:"column:origin_state"`
		RevisionID    *int64  `gorm:"column:revision_id"`
		RevisionRunID *string `gorm:"column:revision_source_run_id"`
		RevisionHash  *string `gorm:"column:revision_manifest_digest"`
		RunOwnerID    *string `gorm:"column:run_owner_id"`
		RunSessionID  *string `gorm:"column:run_session_id"`
		RunStatus     *string `gorm:"column:run_status"`
		Path          *string `gorm:"column:file_path"`
		ObjectRef     *string `gorm:"column:object_ref"`
		SHA256        *string `gorm:"column:sha256"`
		Bytes         *int64  `gorm:"column:file_bytes"`
		MIME          *string `gorm:"column:mime"`
	}
	var rows []resultRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT w.id AS workspace_id,
		       h.workspace_id AS head_id, h.tenant_id AS head_tenant_id,
		       h.revision AS head_revision, h.state AS head_state,
		       h.source_run_id AS head_source_run_id, h.manifest_digest AS head_manifest_digest,
		       o.workspace_id AS origin_workspace_id, o.tenant_id AS origin_tenant_id,
		       o.origin_revision, o.origin_state,
		       r.revision AS revision_id, r.source_run_id AS revision_source_run_id,
		       r.manifest_digest AS revision_manifest_digest,
	       ar.owner_id AS run_owner_id, ar.session_id AS run_session_id, ar.status AS run_status,
	       f.path AS file_path, f.object_ref, f.sha256, f.bytes AS file_bytes, f.mime
		FROM craft_workspaces w
		LEFT JOIN craft_workspace_draft_heads h
		  ON h.workspace_id = w.id AND h.tenant_id = w.tenant_id
		LEFT JOIN craft_workspace_draft_origins o
		  ON o.workspace_id = w.id AND o.tenant_id = w.tenant_id
		LEFT JOIN craft_workspace_draft_revisions r
		  ON r.workspace_id = w.id AND r.tenant_id = w.tenant_id AND r.revision = ?
		LEFT JOIN agent_runs ar
		  ON ar.tenant_id = r.tenant_id AND ar.run_id = r.source_run_id
		LEFT JOIN craft_workspace_draft_files f
		  ON f.workspace_id = r.workspace_id AND f.revision = r.revision
		WHERE w.id = ? AND w.tenant_id = ? AND w.session_id = ? AND w.owner_id = ?
		ORDER BY f.path`, revision, workspaceID, scope.TenantID, scope.SessionID, scope.UserID).Scan(&rows).Error
	if err != nil {
		return craft.DraftHead{}, err
	}
	if len(rows) == 0 {
		var exists int64
		if err := s.db.WithContext(ctx).Model(&craftWorkspaceRow{}).Where("id = ? AND tenant_id = ? AND session_id = ?", workspaceID, scope.TenantID, scope.SessionID).Count(&exists).Error; err != nil {
			return craft.DraftHead{}, err
		}
		if exists > 0 {
			return craft.DraftHead{}, craft.ErrForbidden
		}
		return craft.DraftHead{}, craft.ErrNotFound
	}
	first := rows[0]
	if first.HeadID == nil || first.HeadTenantID == nil || *first.HeadTenantID != scope.TenantID || first.HeadRevision == nil || first.HeadState == nil {
		return craft.DraftHead{}, craft.ErrDraftHeadUnresolved
	}
	// Validate the extant head's basic state, but do not use its revision or
	// manifest to select historical content.
	switch craft.DraftHeadState(*first.HeadState) {
	case craft.DraftHeadEmpty:
		if *first.HeadRevision != 0 || first.HeadSourceRun != nil || first.HeadManifest != nil {
			return craft.DraftHead{}, fmt.Errorf("%w: corrupt empty draft head", craft.ErrInvalidInput)
		}
	case craft.DraftHeadSelected:
		if *first.HeadRevision < 1 || first.HeadSourceRun == nil || *first.HeadSourceRun == "" || first.HeadManifest == nil || *first.HeadManifest == "" {
			return craft.DraftHead{}, fmt.Errorf("%w: corrupt selected draft head", craft.ErrInvalidInput)
		}
	default:
		return craft.DraftHead{}, fmt.Errorf("%w: unknown draft head state %q", craft.ErrInvalidInput, *first.HeadState)
	}
	if revision == 0 {
		if first.OriginID == nil || first.OriginTenant == nil || *first.OriginTenant != scope.TenantID ||
			first.OriginVersion == nil || *first.OriginVersion != 0 || first.OriginState == nil || *first.OriginState != string(craft.DraftHeadEmpty) {
			return craft.DraftHead{}, craft.ErrDraftHeadUnresolved
		}
		return craft.DraftHead{WorkspaceID: workspaceID, Revision: 0, State: craft.DraftHeadEmpty}, nil
	}
	if revision > *first.HeadRevision {
		return craft.DraftHead{}, craft.ErrNotFound
	}
	if first.RevisionID == nil {
		return craft.DraftHead{}, fmt.Errorf("%w: immutable draft revision %d is missing below current head", craft.ErrInvalidInput, revision)
	}
	if *first.RevisionID != revision || first.RevisionRunID == nil || first.RevisionHash == nil ||
		first.RunOwnerID == nil || *first.RunOwnerID != scope.UserID || first.RunSessionID == nil || *first.RunSessionID != scope.SessionID || first.RunStatus == nil {
		return craft.DraftHead{}, fmt.Errorf("%w: immutable draft revision identity is corrupt or foreign", craft.ErrInvalidInput)
	}
	if revision == *first.HeadRevision && (*first.HeadSourceRun == "" || *first.HeadSourceRun != *first.RevisionRunID ||
		*first.HeadManifest == "" || *first.HeadManifest != *first.RevisionHash) {
		return craft.DraftHead{}, fmt.Errorf("%w: current draft head disagrees with its immutable revision", craft.ErrInvalidInput)
	}
	switch *first.RunStatus {
	case "succeeded", "failed", "canceled":
	default:
		return craft.DraftHead{}, fmt.Errorf("%w: immutable draft source Run is not terminal", craft.ErrInvalidInput)
	}
	head := craft.DraftHead{
		WorkspaceID: workspaceID, Revision: revision, State: craft.DraftHeadSelected,
		SourceRunID: *first.RevisionRunID, ManifestDigest: *first.RevisionHash,
		Files: make([]craft.File, 0, len(rows)),
	}
	for _, row := range rows {
		if row.Path == nil || row.ObjectRef == nil || row.SHA256 == nil || row.Bytes == nil || row.MIME == nil {
			return craft.DraftHead{}, fmt.Errorf("%w: incomplete immutable draft file", craft.ErrInvalidInput)
		}
		head.Files = append(head.Files, craft.File{Path: *row.Path, Ref: *row.ObjectRef, SHA256: *row.SHA256, Bytes: *row.Bytes, MIME: *row.MIME})
	}
	if err := head.Validate(); err != nil {
		return craft.DraftHead{}, fmt.Errorf("%w: corrupt immutable draft revision", craft.ErrInvalidInput)
	}
	return head.Clone(), nil
}

// Advance records a sealed capture from a caller that already verified the
// RunView is stopped and idle while holding the Workspace writer fence. SQL
// independently requires a terminal durable Run, matching identity, an empty
// active slot and no unfinished tool/delegation writers; those checks do not
// themselves prove filesystem quiescence.
func (s *CraftDraftHeadStore) Advance(ctx context.Context, scope craft.Scope, workspaceID string, expectedRevision int64, sourceRunID string, files []craft.File) (craft.DraftHead, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" || sourceRunID == "" || expectedRevision < 0 || expectedRevision == int64(^uint64(0)>>1) {
		return craft.DraftHead{}, fmt.Errorf("%w: incomplete draft advancement identity", craft.ErrInvalidInput)
	}
	canonicalFiles := append([]craft.File(nil), files...)
	sort.Slice(canonicalFiles, func(i, j int) bool { return canonicalFiles[i].Path < canonicalFiles[j].Path })
	digest, err := craft.ManifestDigest(canonicalFiles)
	if err != nil {
		return craft.DraftHead{}, err
	}
	proposed := craft.DraftHead{
		WorkspaceID: workspaceID, Revision: expectedRevision + 1, State: craft.DraftHeadSelected,
		SourceRunID: sourceRunID, ManifestDigest: digest, Files: canonicalFiles,
	}
	if err := proposed.Validate(); err != nil {
		return craft.DraftHead{}, err
	}
	var out craft.DraftHead
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// This no-op CAS is the first operation so PostgreSQL locks the head row
		// and SQLite acquires its writer slot before any transaction reads. The
		// second, mutating CAS below cannot race a peer that observed the same rev.
		locked := tx.Model(&craftDraftHeadRow{}).
			Where("workspace_id = ? AND tenant_id = ? AND revision = ?", workspaceID, scope.TenantID, expectedRevision).
			UpdateColumn("updated_at", gorm.Expr("updated_at"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			var workspace craftWorkspaceRow
			lookup := tx.Where("id = ? AND tenant_id = ? AND session_id = ?", workspaceID, scope.TenantID, scope.SessionID).Take(&workspace)
			if lookup.Error == gorm.ErrRecordNotFound {
				return craft.ErrNotFound
			}
			if lookup.Error != nil {
				return lookup.Error
			}
			if workspace.OwnerID != scope.UserID {
				return craft.ErrForbidden
			}
			var head craftDraftHeadRow
			headErr := tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&head).Error
			if headErr == gorm.ErrRecordNotFound {
				return craft.ErrDraftHeadUnresolved
			}
			if headErr != nil {
				return headErr
			}
			return craft.ErrConflict
		}

		var workspace craftWorkspaceRow
		lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND session_id = ?", workspaceID, scope.TenantID, scope.SessionID).
			Take(&workspace)
		if lookup.Error == gorm.ErrRecordNotFound {
			return craft.ErrNotFound
		}
		if lookup.Error != nil {
			return lookup.Error
		}
		if workspace.OwnerID != scope.UserID {
			return craft.ErrForbidden
		}

		var current craftDraftHeadRow
		if e := tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&current).Error; e != nil {
			if e == gorm.ErrRecordNotFound {
				return craft.ErrDraftHeadUnresolved
			}
			return e
		}
		if current.Revision != expectedRevision {
			return craft.ErrConflict
		}
		if current.State == string(craft.DraftHeadEmpty) {
			if current.Revision != 0 || current.SourceRunID != nil || current.ManifestDigest != nil {
				return fmt.Errorf("%w: corrupt empty draft head", craft.ErrInvalidInput)
			}
		} else if current.State != string(craft.DraftHeadSelected) || current.Revision < 1 || current.SourceRunID == nil || *current.SourceRunID == "" || current.ManifestDigest == nil || *current.ManifestDigest == "" {
			return fmt.Errorf("%w: corrupt current draft head", craft.ErrInvalidInput)
		} else {
			var previous craftDraftRevisionRow
			if e := tx.Where("workspace_id = ? AND revision = ? AND tenant_id = ?", workspaceID, current.Revision, scope.TenantID).Take(&previous).Error; e != nil {
				return fmt.Errorf("%w: current immutable revision is missing", craft.ErrInvalidInput)
			}
			if previous.SourceRunID != *current.SourceRunID || previous.ManifestDigest != *current.ManifestDigest {
				return fmt.Errorf("%w: current head disagrees with immutable revision", craft.ErrInvalidInput)
			}
			var previousFiles []craftDraftFileRow
			if e := tx.Where("workspace_id = ? AND revision = ?", workspaceID, current.Revision).Order("path").Find(&previousFiles).Error; e != nil {
				return e
			}
			validated := craft.DraftHead{
				WorkspaceID: workspaceID, Revision: current.Revision, State: craft.DraftHeadSelected,
				SourceRunID: previous.SourceRunID, ManifestDigest: previous.ManifestDigest,
				Files: make([]craft.File, 0, len(previousFiles)),
			}
			for _, file := range previousFiles {
				validated.Files = append(validated.Files, craft.File{Path: file.Path, Ref: file.ObjectRef, SHA256: file.SHA256, Bytes: file.Bytes, MIME: file.MIME})
			}
			if e := validated.Validate(); e != nil {
				return fmt.Errorf("%w: current manifest is corrupt", craft.ErrInvalidInput)
			}
		}

		runKey := agentruntime.RunKey{TenantID: scope.TenantID, RunID: sourceRunID}
		if e := lockRunTransitionRow(tx, runKey); e != nil {
			if errors.Is(e, agentruntime.ErrNotFound) {
				return craft.ErrNotFound
			}
			return e
		}
		var run agentRunRow
		if e := runScope(tx, runKey).Take(&run).Error; e != nil {
			return e
		}
		if run.OwnerID != scope.UserID || run.SessionID != scope.SessionID {
			return craft.ErrForbidden
		}
		switch run.Status {
		case "succeeded", "failed", "canceled":
		default:
			return fmt.Errorf("%w: source Run status %q is not confirmed terminal", craft.ErrBusy, run.Status)
		}

		// Serialize against Run admission while checking the session writer slot.
		slotUpdate := tx.Table("sessions").Where("tenant_id = ? AND id = ?", scope.TenantID, scope.SessionID).
			UpdateColumn("active_agent_run_id", gorm.Expr("active_agent_run_id"))
		if slotUpdate.Error != nil {
			return slotUpdate.Error
		}
		if slotUpdate.RowsAffected != 1 {
			return craft.ErrNotFound
		}
		var session struct {
			ActiveRunID *string `gorm:"column:active_agent_run_id"`
		}
		if e := tx.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", scope.TenantID, scope.SessionID).Take(&session).Error; e != nil {
			return e
		}
		if session.ActiveRunID != nil && *session.ActiveRunID != "" {
			return fmt.Errorf("%w: Workspace has an active Run writer", craft.ErrBusy)
		}

		var pendingTools, pendingDelegations int64
		if e := tx.Table("agent_tool_calls").Where("tenant_id = ? AND run_id = ? AND status NOT IN ('succeeded','failed')", scope.TenantID, sourceRunID).Count(&pendingTools).Error; e != nil {
			return e
		}
		if e := tx.Model(&craftDelegationRow{}).Where("tenant_id = ? AND run_id = ? AND status NOT IN ('succeeded','failed','canceled')", scope.TenantID, sourceRunID).Count(&pendingDelegations).Error; e != nil {
			return e
		}
		if pendingTools != 0 || pendingDelegations != 0 {
			return fmt.Errorf("%w: source Run has unfinished tool or delegation writers", craft.ErrBusy)
		}
		var reused int64
		if e := tx.Table("craft_workspace_draft_revisions").Where("tenant_id = ? AND workspace_id = ? AND source_run_id = ?", scope.TenantID, workspaceID, sourceRunID).Count(&reused).Error; e != nil {
			return e
		}
		if reused != 0 {
			return fmt.Errorf("%w: source Run already produced a draft revision", craft.ErrConflict)
		}

		updated := tx.Model(&craftDraftHeadRow{}).
			Where("workspace_id = ? AND tenant_id = ? AND revision = ?", workspaceID, scope.TenantID, expectedRevision).
			Updates(map[string]any{
				"revision": proposed.Revision, "state": string(craft.DraftHeadSelected),
				"source_run_id": sourceRunID, "manifest_digest": digest,
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return craft.ErrConflict
		}
		revision := craftDraftRevisionRow{WorkspaceID: workspaceID, Revision: proposed.Revision, TenantID: scope.TenantID, SourceRunID: sourceRunID, ManifestDigest: digest}
		if e := tx.Create(&revision).Error; e != nil {
			return e
		}
		fileRows := make([]craftDraftFileRow, 0, len(canonicalFiles))
		for _, file := range canonicalFiles {
			fileRows = append(fileRows, craftDraftFileRow{
				WorkspaceID: workspaceID, Revision: proposed.Revision, Path: file.Path, ObjectRef: file.Ref,
				SHA256: file.SHA256, Bytes: file.Bytes, MIME: file.MIME,
			})
		}
		if e := tx.Create(&fileRows).Error; e != nil {
			return e
		}
		// Build the return value from the transaction's persisted rows, not the
		// caller's slice. This binds the result to exactly the revision that was
		// committed even if a later revision advances before this call returns.
		var storedHead craftDraftHeadRow
		if e := tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&storedHead).Error; e != nil {
			return e
		}
		var storedRevision craftDraftRevisionRow
		if e := tx.Where("workspace_id = ? AND revision = ? AND tenant_id = ?", workspaceID, proposed.Revision, scope.TenantID).Take(&storedRevision).Error; e != nil {
			return e
		}
		var storedFiles []craftDraftFileRow
		if e := tx.Where("workspace_id = ? AND revision = ?", workspaceID, proposed.Revision).Order("path").Find(&storedFiles).Error; e != nil {
			return e
		}
		out = craft.DraftHead{
			WorkspaceID: storedHead.WorkspaceID, Revision: storedHead.Revision, State: craft.DraftHeadState(storedHead.State),
			SourceRunID: storedRevision.SourceRunID, ManifestDigest: storedRevision.ManifestDigest,
			Files: make([]craft.File, 0, len(storedFiles)),
		}
		for _, file := range storedFiles {
			out.Files = append(out.Files, craft.File{Path: file.Path, Ref: file.ObjectRef, SHA256: file.SHA256, Bytes: file.Bytes, MIME: file.MIME})
		}
		if storedHead.Revision != proposed.Revision || storedHead.State != string(craft.DraftHeadSelected) || storedHead.SourceRunID == nil || *storedHead.SourceRunID != out.SourceRunID || storedHead.ManifestDigest == nil || *storedHead.ManifestDigest != out.ManifestDigest {
			return fmt.Errorf("%w: committed draft head differs from inserted revision", craft.ErrInvalidInput)
		}
		if e := out.Validate(); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return craft.DraftHead{}, err
	}
	return out.Clone(), nil
}
