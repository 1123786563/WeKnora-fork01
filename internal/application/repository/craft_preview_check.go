package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// NewCraftPreviewCheckStoreConcrete is the same store at its concrete type
// for consumers that need the T15 (#130) UpdateWebProbeCheck fact channel
// beyond the frozen craft.PreviewCheckStore interface (dig.As binding).
func NewCraftPreviewCheckStoreConcrete(db *gorm.DB) *CraftPreviewCheckStore {
	return &CraftPreviewCheckStore{db: db}
}

// UpdateWebProbeCheck records one externally observed T15 web-gate probe fact
// — preview reachability or actual page load — on a published version. One
// call records exactly one fact: the other web fact, the build/entry checks,
// the version's identity and its file manifest stay untouched, so the two
// probe facts are recorded independently by construction.
//
// Substitution and immutability rules:
//   - page load may only pass after reachability passed — the load fact can
//     never be recorded on the strength of a reachable origin alone;
//   - reachability cannot be withdrawn while a passed page load exists;
//   - a fact already recorded with a different status is a conflict
//     (recorded facts are immutable); an identical rewrite is an idempotent
//     no-op, which is what duplicate probe callbacks are.
func (s *CraftPreviewCheckStore) UpdateWebProbeCheck(ctx context.Context, scope craft.Scope, versionID, name string, outcome craft.CheckOutcome) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete web probe scope", craft.ErrInvalidInput)
	}
	if !craft.IsVersionID(versionID) {
		return craft.Version{}, fmt.Errorf("%w: version id %q", craft.ErrInvalidInput, versionID)
	}
	switch name {
	case craft.CheckPreviewReachable, craft.CheckPageLoad:
	default:
		return craft.Version{}, fmt.Errorf("%w: channel updates only the %q/%q checks, got %q", craft.ErrInvalidInput, craft.CheckPreviewReachable, craft.CheckPageLoad, name)
	}
	switch outcome {
	case craft.WebCheckPassed, craft.WebCheckFailed:
	case craft.WebCheckNotRun:
		// Writing an explicit not_run row is a contract trap: the row is
		// immutable once written, so the check name could never again record
		// a REAL observation, while a missing row already means not_run in
		// WebEvidenceFromChecks. Producers that could not observe simply do
		// not write.
		return craft.Version{}, fmt.Errorf("%w: the probe channel records observations only; not_run is the absent-row state, got %q", craft.ErrInvalidInput, string(outcome))
	default:
		return craft.Version{}, fmt.Errorf("%w: unknown web check outcome %q", craft.ErrInvalidInput, string(outcome))
	}

	var out craft.Version
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftVersionRow
		// Row lock (SELECT ... FOR UPDATE, the repo's established pattern):
		// check writers arrive from independent observation callbacks, so
		// concurrent updates must serialize on the version row. Without the
		// lock both read the same old checks_json snapshot and the last
		// unconditional write silently drops the other's fact — or worse,
		// rewrites a recorded outcome and breaks fact immutability.
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", versionID).Take(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: version %s", craft.ErrNotFound, versionID)
		}
		if e != nil {
			return e
		}
		if e := authorizeCraftVersion(tx, row, scope); e != nil {
			return e
		}

		checks := []craft.Check{}
		if row.ChecksJSON != "" {
			if err := json.Unmarshal([]byte(row.ChecksJSON), &checks); err != nil {
				return fmt.Errorf("craft: decode version %s checks: %w", versionID, err)
			}
		}
		current := craft.WebEvidenceFromChecks(checks)

		// Substitution fence: the load fact needs a reachable origin behind it.
		if name == craft.CheckPageLoad && outcome == craft.WebCheckPassed && current.PreviewReachable != craft.WebCheckPassed {
			return fmt.Errorf("%w: page load cannot pass before the preview origin is reachable", craft.ErrInvalidInput)
		}
		// A passed load proves the origin was reachable; withdrawing
		// reachability afterwards contradicts a recorded fact.
		if name == craft.CheckPreviewReachable && outcome != craft.WebCheckPassed && current.PageLoaded == craft.WebCheckPassed {
			return fmt.Errorf("%w: reachability cannot be withdrawn while a page load passed", craft.ErrConflict)
		}

		detail := "controlled preview origin reachability observed"
		if name == craft.CheckPageLoad {
			detail = "actual browser page load observed"
		}
		check := craft.Check{Name: name, Status: string(outcome), Detail: detail}
		for _, existing := range checks {
			if existing.Name != name {
				continue
			}
			if existing.Status != check.Status {
				return fmt.Errorf("%w: check %q already recorded %q for version %s", craft.ErrConflict, name, existing.Status, versionID)
			}
			out, e = loadCraftVersion(tx, versionID)
			return e // identical rewrite: idempotent no-op
		}
		checks = append(checks, check)
		encoded, err := json.Marshal(checks)
		if err != nil {
			return err
		}
		if e := tx.Model(&craftVersionRow{}).
			Where("id = ?", versionID).
			Update("checks_json", string(encoded)).Error; e != nil {
			return e
		}
		out, e = loadCraftVersion(tx, versionID)
		return e
	})
	if err != nil {
		return craft.Version{}, err
	}
	return out, nil
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
		// Row lock (SELECT ... FOR UPDATE, the repo's established pattern):
		// check writers arrive from independent observation callbacks, so
		// concurrent updates must serialize on the version row. Without the
		// lock both read the same old checks_json snapshot and the last
		// unconditional write silently drops the other's fact — or worse,
		// rewrites a recorded outcome and breaks fact immutability.
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", versionID).Take(&row).Error
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
