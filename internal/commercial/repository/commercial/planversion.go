package commercial

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	// ErrPublicationNotFound: no publication recorded for the plan version.
	ErrPublicationNotFound = errors.New("publication_not_found")
	// ErrPublicationConflict: a publication already exists for the command
	// key or the (plan_key, version) with DIFFERENT content — never a silent
	// overwrite of the immutable publish projection.
	ErrPublicationConflict = errors.New("publication_conflict")
	// ErrPlanVersionExists: the (plan_key, version) slot is taken; a
	// concurrent CreateDraft lost the race and the caller retries with a
	// fresh NextVersion.
	ErrPlanVersionExists = errors.New("plan_version_exists")
)

// PublicationRow is the append-only publish projection: the immutable map
// from a plan version to its seam command identity and receipt. PlanCode is
// the external plan code — seam-internal by design; it NEVER crosses the
// admin API (the admin surface addresses versions by (plan_key, version)).
type PublicationRow struct {
	CommandKey  string    `gorm:"primaryKey;column:command_key"` // publish_plan_version:<key>:<version>
	PlanKey     string    `gorm:"column:plan_key;uniqueIndex:uq_plan_publication_version"`
	Version     int64     `gorm:"column:version;uniqueIndex:uq_plan_publication_version"`
	PlanCode    string    `gorm:"column:plan_code;uniqueIndex"`
	ReceiptJSON string    `gorm:"column:receipt_json;not null"`
	PublishedBy string    `gorm:"column:published_by;not null default ''"`
	PublishedAt time.Time `gorm:"column:published_at;not null"`
}

func (PublicationRow) TableName() string { return "commercial_plan_publications" }

// VersionView is the joined read model behind the admin list/get endpoints:
// the catalog row plus receipt presence (empty ReceiptJSON / nil
// PublishedAt on an unpublished version).
type VersionView struct {
	PlanKey        string
	Version        int64
	State          string
	DefinitionJSON string
	ReceiptJSON    string
	PublishedAt    *time.Time
}

// PlanVersionStore persists the draft→publishing→published lifecycle and
// the append-only publication projection. Immutability of a published
// definition is enforced at EVERY layer: this store's guards, the legacy
// CatalogStore guard, and the DB trigger installed by EnsureSchema /
// migrations 000179 (PostgreSQL) and 000100 (SQLite).
type PlanVersionStore struct{ db *gorm.DB }

func NewPlanVersionStore(db *gorm.DB) *PlanVersionStore { return &PlanVersionStore{db: db} }

// EnsureSchema creates the publications table and the immutability trigger
// with portable DDL (CREATE TABLE IF NOT EXISTS per dialect — the
// NewCommercialHandler precedent), so tests and dev deployments are safe
// without running the migration. The catalog table is created too (same
// shape as the 000111/000031 migrations) because the trigger hangs on it.
func (s *PlanVersionStore) EnsureSchema(ctx context.Context) error {
	if err := s.db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS commercial_plan_catalog (
		plan_key TEXT NOT NULL,
		version INTEGER NOT NULL,
		definition_json TEXT NOT NULL,
		external_id TEXT NOT NULL UNIQUE,
		state TEXT NOT NULL,
		PRIMARY KEY (plan_key, version)
	)`).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS commercial_plan_publications (
		command_key TEXT NOT NULL,
		plan_key TEXT NOT NULL,
		version INTEGER NOT NULL,
		plan_code TEXT NOT NULL UNIQUE,
		receipt_json TEXT NOT NULL,
		published_by TEXT NOT NULL DEFAULT '',
		published_at DATETIME NOT NULL,
		PRIMARY KEY (command_key),
		UNIQUE (plan_key, version)
	)`).Error; err != nil {
		return err
	}
	return s.ensureImmutabilityTrigger(ctx)
}

// ensureImmutabilityTrigger installs the published-row immutability trigger
// in the active dialect. Any UPDATE changing definition_json/external_id of
// a row whose OLD state is 'published' aborts with the closed token
// 'published_plan_immutable'; state-only transitions (publishing→published,
// →archived) never trip it.
func (s *PlanVersionStore) ensureImmutabilityTrigger(ctx context.Context) error {
	switch s.db.Dialector.Name() {
	case "sqlite":
		return s.db.WithContext(ctx).Exec(`CREATE TRIGGER IF NOT EXISTS trg_commercial_plan_catalog_published_immutable
			BEFORE UPDATE ON commercial_plan_catalog
			FOR EACH ROW
			WHEN OLD.state = 'published'
				AND (NEW.definition_json <> OLD.definition_json OR NEW.external_id <> OLD.external_id)
			BEGIN
				SELECT RAISE(ABORT, 'published_plan_immutable');
			END`).Error
	default: // PostgreSQL (versioned migrations carry the canonical copy)
		if err := s.db.WithContext(ctx).Exec(`CREATE OR REPLACE FUNCTION commercial_plan_catalog_published_immutable() RETURNS trigger AS $$
			BEGIN
				IF OLD.state = 'published' AND (NEW.definition_json IS DISTINCT FROM OLD.definition_json OR NEW.external_id IS DISTINCT FROM OLD.external_id) THEN
					RAISE EXCEPTION 'published_plan_immutable';
				END IF;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql`).Error; err != nil {
			return err
		}
		return s.db.WithContext(ctx).Exec(`DROP TRIGGER IF EXISTS trg_commercial_plan_catalog_published_immutable ON commercial_plan_catalog`).Error
		// The CREATE TRIGGER itself is owned by migration 000179; EnsureSchema
		// only guarantees the function shape for dev/test bootstrapping.
	}
}

// NextVersion returns max(version)+1 for the plan key (1 for a fresh key).
// Races between NextVersion and CreateDraft are resolved by the catalog
// primary key plus a caller retry (ErrPlanVersionExists).
func (s *PlanVersionStore) NextVersion(ctx context.Context, planKey string) (int64, error) {
	var max int64
	err := s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(MAX(version), 0) FROM commercial_plan_catalog WHERE plan_key = ?`, planKey).
		Scan(&max).Error
	return max + 1, err
}

// CreateDraft inserts a brand-new draft row. Only the draft state is
// accepted; an existing (plan_key, version) slot is a version-exists error
// the caller resolves by retrying with a fresh NextVersion.
func (s *PlanVersionStore) CreateDraft(ctx context.Context, row PlanRow) error {
	if row.PlanKey == "" || row.Version <= 0 || row.DefinitionJSON == "" || row.ExternalID == "" {
		return ErrInvalidPlanRow
	}
	if row.State != domain.PlanStateDraft {
		return domain.ErrInvalidPlanState
	}
	err := s.db.WithContext(ctx).Create(&row).Error
	if err != nil && isUniqueViolation(err) {
		return ErrPlanVersionExists
	}
	return err
}

// UpdateDraft rewrites the definition of a DRAFT version only. A published
// (or publishing) version is immutable: any change is a new
// (plan_key, version) row, never an in-place edit.
func (s *PlanVersionStore) UpdateDraft(ctx context.Context, planKey string, version int64, definitionJSON string) error {
	if definitionJSON == "" {
		return ErrInvalidPlanRow
	}
	res := s.db.WithContext(ctx).Model(&PlanRow{}).
		Where("plan_key = ? AND version = ? AND state = ?", planKey, version, domain.PlanStateDraft).
		Update("definition_json", definitionJSON)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var current PlanRow
		err := s.db.WithContext(ctx).Where("plan_key = ? AND version = ?", planKey, version).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPlanNotFound
		}
		if err != nil {
			return err
		}
		if current.State == domain.PlanStatePublished || current.State == domain.PlanStatePublishing {
			return ErrPublishedPlanImmutable
		}
		return domain.ErrInvalidPlanState
	}
	return nil
}

// SetPublishing moves draft|publishing → publishing. The transition is
// retry-safe: a row already publishing stays publishing (a lost response is
// re-driven with the SAME command key, never a new identity).
func (s *PlanVersionStore) SetPublishing(ctx context.Context, planKey string, version int64) error {
	res := s.db.WithContext(ctx).Model(&PlanRow{}).
		Where("plan_key = ? AND version = ? AND state IN ?", planKey, version,
			[]string{domain.PlanStateDraft, domain.PlanStatePublishing}).
		Update("state", domain.PlanStatePublishing)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var current PlanRow
		err := s.db.WithContext(ctx).Where("plan_key = ? AND version = ?", planKey, version).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPlanNotFound
		}
		return err
	}
	return nil
}

// RecordPublication inserts the publication and flips the catalog row
// publishing → published in ONE transaction. Replaying the exact same
// CommandKey row verifies-equals and returns (idempotent); the same key —
// or the same (plan_key, version) — with DIFFERENT content is a conflict.
func (s *PlanVersionStore) RecordPublication(ctx context.Context, row PublicationRow, planKey string, version int64) error {
	if row.CommandKey == "" || row.PlanKey == "" || row.Version <= 0 || row.PlanCode == "" || row.ReceiptJSON == "" {
		return ErrInvalidPlanRow
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Create(&row).Error
		if err != nil {
			if !isUniqueViolation(err) {
				return err
			}
			var existing PublicationRow
			byKey := tx.Where("command_key = ?", row.CommandKey).First(&existing).Error
			switch {
			case byKey == nil && existing.PlanKey == row.PlanKey && existing.Version == row.Version &&
				existing.PlanCode == row.PlanCode && existing.ReceiptJSON == row.ReceiptJSON:
				// Byte-equal replay of the recorded publication: fall through
				// to the (idempotent) state flip.
			case byKey == nil:
				return ErrPublicationConflict
			default:
				// The unique violation came from the (plan_key, version)
				// index instead: same version recorded under another key.
				if q := tx.Where("plan_key = ? AND version = ?", planKey, version).First(&existing).Error; q == nil {
					return ErrPublicationConflict
				}
				return err
			}
		}
		res := tx.Model(&PlanRow{}).
			Where("plan_key = ? AND version = ? AND state = ?", planKey, version, domain.PlanStatePublishing).
			Update("state", domain.PlanStatePublished)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Already published (idempotent replay) or an unexpected state;
			// verify before declaring success.
			var current PlanRow
			if err := tx.Where("plan_key = ? AND version = ?", planKey, version).First(&current).Error; err != nil {
				return err
			}
			if current.State != domain.PlanStatePublished {
				return domain.ErrInvalidPlanState
			}
		}
		return nil
	})
}

// GetPublication returns the recorded publication of one plan version.
func (s *PlanVersionStore) GetPublication(ctx context.Context, planKey string, version int64) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).Where("plan_key = ? AND version = ?", planKey, version).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}

// FindPublicationByCode resolves an external plan code back to its
// published (plan_key, version) — the benefits projection's reverse
// mapping (the authority snapshot answers plan codes; the local
// publications table is the only map back to product vocabulary).
func (s *PlanVersionStore) FindPublicationByCode(ctx context.Context, planCode string) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).Where("plan_code = ?", planCode).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}

// ListPublications returns every publication of one plan key, ordered by
// version (seam-internal rows; the admin API never projects them).
func (s *PlanVersionStore) ListPublications(ctx context.Context, planKey string) ([]PublicationRow, error) {
	var rows []PublicationRow
	err := s.db.WithContext(ctx).Where("plan_key = ?", planKey).Order("version").Find(&rows).Error
	return rows, err
}

const versionViewSelect = `SELECT c.plan_key, c.version, c.state, c.definition_json,
	COALESCE(p.receipt_json, '') AS receipt_json, p.published_at
	FROM commercial_plan_catalog c
	LEFT JOIN commercial_plan_publications p
		ON p.plan_key = c.plan_key AND p.version = c.version`

// ListVersions returns every catalog row LEFT JOINed with its publication,
// ordered by (plan_key, version).
func (s *PlanVersionStore) ListVersions(ctx context.Context) ([]VersionView, error) {
	var views []VersionView
	err := s.db.WithContext(ctx).
		Raw(versionViewSelect + ` ORDER BY c.plan_key, c.version`).Scan(&views).Error
	return views, err
}

// GetVersion returns one version row joined with its publication.
func (s *PlanVersionStore) GetVersion(ctx context.Context, planKey string, version int64) (VersionView, error) {
	var view VersionView
	err := s.db.WithContext(ctx).
		Raw(versionViewSelect+` WHERE c.plan_key = ? AND c.version = ?`, planKey, version).Scan(&view).Error
	if err != nil {
		return VersionView{}, err
	}
	if view.PlanKey == "" {
		return VersionView{}, ErrPlanNotFound
	}
	return view, nil
}

// isUniqueViolation reports a unique/PK constraint failure across the
// SQLite and PostgreSQL drivers gorm fronts here.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{"UNIQUE constraint failed", "duplicate key value", "PRIMARY KEY"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
