package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftCandidateStore keeps Run output private until a later promotion path
// explicitly consumes it. It is intentionally separate from VersionStore.
type CraftCandidateStore struct {
	db *gorm.DB
}

var _ craft.CandidateStore = (*CraftCandidateStore)(nil)

func NewCraftCandidateStore(db *gorm.DB) craft.CandidateStore {
	return &CraftCandidateStore{db: db}
}

type craftCandidateRow struct {
	ID             string    `gorm:"column:id"`
	TenantID       uint64    `gorm:"column:tenant_id"`
	OwnerID        string    `gorm:"column:owner_id"`
	SessionID      string    `gorm:"column:session_id"`
	WorkspaceID    string    `gorm:"column:workspace_id"`
	RunID          string    `gorm:"column:run_id"`
	Generation     string    `gorm:"column:generation"`
	Kind           string    `gorm:"column:kind"`
	ManifestDigest string    `gorm:"column:manifest_digest"`
	ChecksJSON     string    `gorm:"column:checks_json"`
	EvidenceJSON   string    `gorm:"column:evidence_json"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (craftCandidateRow) TableName() string { return "craft_candidates" }

type craftCandidateFileRow struct {
	CandidateID string `gorm:"column:candidate_id"`
	Path        string `gorm:"column:path"`
	ResourceRef string `gorm:"column:resource_ref"`
	FileHash    string `gorm:"column:file_hash"`
	FileBytes   int64  `gorm:"column:file_bytes"`
	MIMEType    string `gorm:"column:mime"`
}

func (craftCandidateFileRow) TableName() string { return "craft_candidate_files" }

func (r craftCandidateRow) candidate(db *gorm.DB) (craft.Candidate, error) {
	var fileRows []craftCandidateFileRow
	if err := db.Where("candidate_id = ?", r.ID).Order("path ASC").Find(&fileRows).Error; err != nil {
		return craft.Candidate{}, err
	}
	files := make([]craft.File, 0, len(fileRows))
	for _, file := range fileRows {
		files = append(files, craft.File{
			Path: file.Path, Ref: file.ResourceRef, SHA256: file.FileHash,
			Bytes: file.FileBytes, MIME: file.MIMEType,
		})
	}
	checks := []craft.Check{}
	if err := json.Unmarshal([]byte(r.ChecksJSON), &checks); err != nil {
		return craft.Candidate{}, fmt.Errorf("craft: decode candidate %s checks: %w", r.ID, err)
	}
	var evidence craft.ArtifactEvidence
	if err := json.Unmarshal([]byte(r.EvidenceJSON), &evidence); err != nil {
		return craft.Candidate{}, fmt.Errorf("craft: decode candidate %s evidence: %w", r.ID, err)
	}
	candidate := craft.Candidate{
		ID: r.ID, Scope: craft.Scope{TenantID: r.TenantID, UserID: r.OwnerID, SessionID: r.SessionID},
		WorkspaceID: r.WorkspaceID, RunID: r.RunID, Generation: r.Generation, Kind: r.Kind,
		ManifestDigest: r.ManifestDigest, Files: files, Checks: checks, Evidence: evidence,
	}
	if err := candidate.Validate(candidate.Scope); err != nil {
		return craft.Candidate{}, fmt.Errorf("craft: stored candidate %s is invalid: %w", r.ID, err)
	}
	return candidate, nil
}

func (s *CraftCandidateStore) PutCandidate(ctx context.Context, scope craft.Scope, candidate craft.Candidate) (craft.Candidate, error) {
	if s == nil || s.db == nil {
		return craft.Candidate{}, fmt.Errorf("%w: candidate store is not assembled", craft.ErrInvalidInput)
	}
	if err := candidate.Validate(scope); err != nil {
		return craft.Candidate{}, err
	}
	checksJSON, err := json.Marshal(candidate.Checks)
	if err != nil {
		return craft.Candidate{}, fmt.Errorf("craft: encode candidate checks: %w", err)
	}
	evidenceJSON, err := json.Marshal(candidate.Evidence)
	if err != nil {
		return craft.Candidate{}, fmt.Errorf("craft: encode candidate evidence: %w", err)
	}
	var out craft.Candidate
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateCandidateScope(tx, scope, candidate.WorkspaceID, candidate.RunID); err != nil {
			return err
		}
		row := craftCandidateRow{
			ID: candidate.ID, TenantID: scope.TenantID, OwnerID: scope.UserID,
			SessionID: scope.SessionID, WorkspaceID: candidate.WorkspaceID, RunID: candidate.RunID,
			Generation: candidate.Generation, Kind: candidate.Kind, ManifestDigest: candidate.ManifestDigest,
			ChecksJSON: string(checksJSON), EvidenceJSON: string(evidenceJSON),
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var stored craftCandidateRow
			if err := tx.Where("tenant_id = ? AND workspace_id = ? AND run_id = ?", scope.TenantID, candidate.WorkspaceID, candidate.RunID).Take(&stored).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return craft.ErrConflict
				}
				return err
			}
			if err := authorizeCandidateRow(tx, stored, scope); err != nil {
				return err
			}
			prior, err := stored.candidate(tx)
			if err != nil {
				return err
			}
			if !sameCraftCandidate(prior, candidate) {
				return fmt.Errorf("%w: Run already has a different candidate", craft.ErrConflict)
			}
			out = prior
			return nil
		}

		fileRows := make([]craftCandidateFileRow, 0, len(candidate.Files))
		for _, file := range candidate.Files {
			fileRows = append(fileRows, craftCandidateFileRow{
				CandidateID: candidate.ID, Path: file.Path, ResourceRef: file.Ref,
				FileHash: file.SHA256, FileBytes: file.Bytes, MIMEType: file.MIME,
			})
		}
		if len(fileRows) > 0 {
			if err := tx.Create(&fileRows).Error; err != nil {
				return err
			}
		}
		out = candidate
		return nil
	})
	if err != nil {
		return craft.Candidate{}, err
	}
	return out, nil
}

func (s *CraftCandidateStore) GetCandidate(ctx context.Context, scope craft.Scope, id string) (craft.Candidate, error) {
	if s == nil || s.db == nil || scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || id == "" {
		return craft.Candidate{}, craft.ErrNotFound
	}
	var out craft.Candidate
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftCandidateRow
		err := tx.Where("tenant_id = ? AND id = ?", scope.TenantID, id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := authorizeCandidateRow(tx, row, scope); err != nil {
			return err
		}
		out, err = row.candidate(tx)
		return err
	})
	if err != nil {
		return craft.Candidate{}, err
	}
	return out, nil
}

// validateCandidateScope binds candidate persistence to both the workspace
// owner/session and the durable Run row, not only caller-supplied scope fields.
func validateCandidateScope(tx *gorm.DB, scope craft.Scope, workspaceID, runID string) error {
	var workspace craftWorkspaceRow
	err := tx.Where("tenant_id = ? AND id = ?", scope.TenantID, workspaceID).Take(&workspace).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if workspace.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if workspace.OwnerID != scope.UserID {
		return craft.ErrForbidden
	}
	var run agentRunRow
	err = tx.Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if run.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if run.OwnerID != scope.UserID {
		return craft.ErrForbidden
	}
	return nil
}

func authorizeCandidateRow(tx *gorm.DB, row craftCandidateRow, scope craft.Scope) error {
	if row.TenantID != scope.TenantID || row.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if row.OwnerID != scope.UserID {
		return craft.ErrForbidden
	}
	var workspace craftWorkspaceRow
	err := tx.Where("tenant_id = ? AND id = ?", row.TenantID, row.WorkspaceID).Take(&workspace).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if workspace.SessionID != row.SessionID {
		return craft.ErrConflict
	}
	if workspace.OwnerID != row.OwnerID {
		return craft.ErrConflict
	}
	return validateCandidateScope(tx, scope, row.WorkspaceID, row.RunID)
}

func sameCraftCandidate(a, b craft.Candidate) bool {
	if a.ID != b.ID || a.Scope != b.Scope || a.WorkspaceID != b.WorkspaceID || a.RunID != b.RunID ||
		a.Generation != b.Generation || a.Kind != b.Kind || a.ManifestDigest != b.ManifestDigest ||
		a.Evidence != b.Evidence || len(a.Files) != len(b.Files) || len(a.Checks) != len(b.Checks) {
		return false
	}
	for i := range a.Files {
		// Object stores commonly issue a fresh opaque reference for every
		// upload. Candidate replay is content identity; the previously sealed
		// reference wins and the redundant upload remains an O03 orphan.
		if a.Files[i].Path != b.Files[i].Path || a.Files[i].SHA256 != b.Files[i].SHA256 ||
			a.Files[i].Bytes != b.Files[i].Bytes || a.Files[i].MIME != b.Files[i].MIME {
			return false
		}
	}
	for i := range a.Checks {
		if a.Checks[i] != b.Checks[i] {
			return false
		}
	}
	return true
}
