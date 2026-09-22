package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
)

// CraftPreviewCheckStore is W02's preview check update channel (独立 origin
// 受控预览). W01's Publish writes all three verification facts at collection
// time and treats any later difference as a conflict — by design, an immutable
// version must never be silently rewritten. The preview verdict, however, is
// only observable AFTER the version exists: the controlled page must really
// load before the preview check may pass. This store rewrites exactly the
// preview entry of the published checks_json in place, under the same scope
// ACL as reads, and touches nothing else — no new version row, no manifest
// change, no re-Publish. Combined with the preview evidence source feeding
// W01's collector, an idempotent re-collection of the same content still
// adopts the stored row instead of conflicting.
type CraftPreviewCheckStore struct {
	db *gorm.DB
}

var _ craft.PreviewCheckStore = (*CraftPreviewCheckStore)(nil)

// NewCraftPreviewCheckStore constructs the preview check update channel on
// the migrated business database. It composes with NewCraftVersionStore on
// the same *gorm.DB.
func NewCraftPreviewCheckStore(db *gorm.DB) craft.PreviewCheckStore {
	return &CraftPreviewCheckStore{db: db}
}

// UpdatePreviewCheck replaces (or appends, for pre-W01 rows) the preview
// check of one published version in the requesting scope. The version's
// build and entry checks, its identity and its file manifest are left
// byte-for-byte untouched.
func (s *CraftPreviewCheckStore) UpdatePreviewCheck(ctx context.Context, scope craft.Scope, versionID string, check craft.Check) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete preview check scope", craft.ErrInvalidInput)
	}
	if !craft.IsVersionID(versionID) {
		return craft.Version{}, fmt.Errorf("%w: version id %q", craft.ErrInvalidInput, versionID)
	}
	if check.Name != craft.CheckPreview {
		return craft.Version{}, fmt.Errorf("%w: channel updates only the %q check, got %q", craft.ErrInvalidInput, craft.CheckPreview, check.Name)
	}
	switch check.Status {
	case craft.CheckPassed, craft.CheckFailed, craft.CheckNotRun:
	default:
		return craft.Version{}, fmt.Errorf("%w: unknown check status %q", craft.ErrInvalidInput, check.Status)
	}

	var out craft.Version
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftVersionRow
		e := tx.Where("id = ?", versionID).Take(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: version %s", craft.ErrNotFound, versionID)
		}
		if e != nil {
			return e
		}
		// Same ACL as a version read: cross-tenant/cross-session does not
		// exist for the caller, a foreign owner is forbidden.
		if e := authorizeCraftVersion(tx, row, scope); e != nil {
			return e
		}

		checks := []craft.Check{}
		if row.ChecksJSON != "" {
			if err := json.Unmarshal([]byte(row.ChecksJSON), &checks); err != nil {
				return fmt.Errorf("craft: decode version %s checks: %w", versionID, err)
			}
		}
		replaced := false
		for i := range checks {
			if checks[i].Name == craft.CheckPreview {
				checks[i] = check
				replaced = true
			}
		}
		if !replaced {
			checks = append(checks, check)
		}
		encoded, err := json.Marshal(checks)
		if err != nil {
			return err
		}
		if e := tx.Model(&craftVersionRow{}).
			Where("id = ?", versionID).
			Update("checks_json", string(encoded)).Error; e != nil {
			return e
		}
		v, e := loadCraftVersion(tx, versionID)
		if e != nil {
			return e
		}
		out = v
		return nil
	})
	if err != nil {
		return craft.Version{}, err
	}
	return out, nil
}
