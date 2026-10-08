package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftVersionStore persists immutable Craft artifact versions (W01) in the
// migrated business database. One row per (workspace, run, manifest digest)
// carries the verification facts of that publish; the file manifest pins each
// file's storage ref and content digest. Publishing is idempotent by logical
// identity: the same content under the same run always answers the same
// version row, and rows are never updated after their publish — later
// workspace rounds publish new rows, so old version downloads keep resolving
// to the objects their manifest pinned.
type CraftVersionStore struct {
	db *gorm.DB
}

var _ craft.VersionStore = (*CraftVersionStore)(nil)

// The T07 evidence members ride on the same concrete store: a wired
// assembly type-asserts the version store to craft.VersionEvidenceStore and
// the promotion pins evidence in the same commit as the version.
var _ craft.VersionEvidenceStore = (*CraftVersionStore)(nil)
var _ craft.DraftFencedVersionStore = (*CraftVersionStore)(nil)

// NewCraftVersionStore constructs the craft.VersionStore implementation
// backed by the migrated business database.
func NewCraftVersionStore(db *gorm.DB) craft.VersionStore {
	return &CraftVersionStore{db: db}
}

type craftVersionRow struct {
	ID           string    `gorm:"column:id"`
	TenantID     uint64    `gorm:"column:tenant_id"`
	WorkspaceID  string    `gorm:"column:workspace_id"`
	RunID        string    `gorm:"column:run_id"`
	Kind         string    `gorm:"column:kind"`
	ManifestHash string    `gorm:"column:manifest_hash"`
	ChecksJSON   string    `gorm:"column:checks_json"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (craftVersionRow) TableName() string { return "craft_versions" }

type craftVersionFileRow struct {
	VersionID   string `gorm:"column:version_id"`
	Path        string `gorm:"column:path"`
	ResourceRef string `gorm:"column:resource_ref"`
	FileHash    string `gorm:"column:file_hash"`
	FileBytes   int64  `gorm:"column:file_bytes"`
	MIMEType    string `gorm:"column:mime"`
}

func (craftVersionFileRow) TableName() string { return "craft_version_files" }

// prepareCraftVersion validates the caller-independent invariants and
// derives the canonical identity: complete scope fields are checked by the
// caller, here the version must name its workspace, run and kind, the file
// manifest must be canonical, and a supplied id must equal the derived
// VersionID (an empty id is filled in).
func prepareCraftVersion(in craft.Version) (craft.Version, string, error) {
	if in.WorkspaceID == "" || in.RunID == "" || in.Kind == "" {
		return craft.Version{}, "", fmt.Errorf("%w: version requires workspace, run and kind", craft.ErrInvalidInput)
	}
	// Persist and return one canonical order so an identical retry compares
	// equal to the sorted rows reconstructed by loadCraftVersion.
	in.Files = append([]craft.File(nil), in.Files...)
	sort.Slice(in.Files, func(i, j int) bool { return in.Files[i].Path < in.Files[j].Path })
	digest, err := craft.ManifestDigest(in.Files)
	if err != nil {
		return craft.Version{}, "", err
	}
	if in.ID == "" {
		in.ID = craft.VersionID(in.WorkspaceID, in.RunID, digest)
	}
	if in.ID != craft.VersionID(in.WorkspaceID, in.RunID, digest) {
		return craft.Version{}, "", fmt.Errorf("%w: version id %q does not match its manifest identity", craft.ErrInvalidInput, in.ID)
	}
	if in.Checks == nil {
		in.Checks = []craft.Check{}
	}
	return in, digest, nil
}

// authorizeCraftVersion answers whether the scope may read one version row:
// a cross-tenant or cross-session workspace does not exist for the caller,
// another user's binding is visible but forbidden — the same ACL shape as
// the delegation store.
func authorizeCraftVersion(db *gorm.DB, row craftVersionRow, scope craft.Scope) error {
	if row.TenantID != scope.TenantID {
		return craft.ErrNotFound
	}
	var ws craftWorkspaceRow
	err := db.Where("id = ?", row.WorkspaceID).Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if ws.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if ws.OwnerID != scope.UserID {
		return fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	return nil
}

// loadCraftVersion reads one version with its file manifest, files ordered
// by path so the persisted shape is canonical.
func loadCraftVersion(db *gorm.DB, id string) (craft.Version, error) {
	var row craftVersionRow
	err := db.Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Version{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Version{}, err
	}
	var fileRows []craftVersionFileRow
	if err := db.Where("version_id = ?", id).Order("path ASC").Find(&fileRows).Error; err != nil {
		return craft.Version{}, err
	}
	files := make([]craft.File, 0, len(fileRows))
	for _, fr := range fileRows {
		files = append(files, craft.File{
			Path: fr.Path, Ref: fr.ResourceRef, SHA256: fr.FileHash,
			MIME: fr.MIMEType, Bytes: fr.FileBytes,
		})
	}
	checks := []craft.Check{}
	if row.ChecksJSON != "" {
		if err := json.Unmarshal([]byte(row.ChecksJSON), &checks); err != nil {
			return craft.Version{}, fmt.Errorf("craft: decode version %s checks: %w", id, err)
		}
	}
	return craft.Version{
		ID: row.ID, WorkspaceID: row.WorkspaceID, RunID: row.RunID,
		Kind: row.Kind, Files: files, Checks: checks,
	}, nil
}

// sameCraftVersion reports publish identity: same derived id and the same
// manifest, checks and kind. Only the identical replay may adopt a stored
// row; anything else is a conflict.
func sameCraftVersion(a, b craft.Version) bool {
	if a.ID != b.ID || a.WorkspaceID != b.WorkspaceID || a.RunID != b.RunID || a.Kind != b.Kind ||
		len(a.Files) != len(b.Files) || len(a.Checks) != len(b.Checks) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
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

// Publish records one immutable version. The workspace must belong to the
// publishing scope; file objects are expected to be durably uploaded before
// this call (the artifact service uploads first, then publishes in one
// transaction). A racing or replayed identical publish adopts the stored row
// and answers it with the same id; a different publish under the same
// identity is a conflict.
func (s *CraftVersionStore) Publish(ctx context.Context, scope craft.Scope, in craft.Version) (craft.Version, error) {
	return s.publish(ctx, scope, in, nil, nil)
}

func (s *CraftVersionStore) PublishWithDraftHead(ctx context.Context, scope craft.Scope, in craft.Version, expected craft.DraftHead, evidence *craft.VersionEvidence) (craft.Version, error) {
	return s.publish(ctx, scope, in, evidence, &expected)
}

// publish is the shared transaction core of Publish and
// PublishWithEvidence. evidence == nil pins no evidence member (the recorded
// legacy shape); otherwise the evidence row lands in the SAME transaction as
// the version row and its files, so a visible version always carries the
// evidence it was promoted with.
func (s *CraftVersionStore) publish(ctx context.Context, scope craft.Scope, in craft.Version, evidence *craft.VersionEvidence, expectedHead *craft.DraftHead) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete version scope", craft.ErrInvalidInput)
	}
	in, digest, err := prepareCraftVersion(in)
	if err != nil {
		return craft.Version{}, err
	}
	encoded, err := json.Marshal(in.Checks)
	if err != nil {
		return craft.Version{}, err
	}
	var evidenceRow craftVersionEvidenceRow
	if evidence != nil {
		if err := craft.ValidateVersionEvidence(*evidence); err != nil {
			return craft.Version{}, err
		}
		if evidence.VersionID != in.ID || evidence.RunID != in.RunID {
			return craft.Version{}, fmt.Errorf("%w: evidence binds version %s run %s, not the published version %s run %s",
				craft.ErrInvalidInput, evidence.VersionID, evidence.RunID, in.ID, in.RunID)
		}
		// The canonical encoding and its digest come from the one domain
		// function, so the stored bytes and the digest column can never
		// disagree about what was pinned. (evidenceDigest, not digest: the
		// outer digest is the version's manifest hash and stays in use
		// below.)
		raw, evidenceDigest, err := craft.EncodeVersionEvidence(*evidence)
		if err != nil {
			return craft.Version{}, err
		}
		evidenceRow = craftVersionEvidenceRow{
			VersionID: in.ID, TenantID: scope.TenantID,
			EvidenceJSON: string(raw), Digest: evidenceDigest,
			AcquiredAt: evidence.AcquiredAt, PinnedAt: evidence.PinnedAt,
		}
	}
	var out craft.Version
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ws craftWorkspaceRow
		e := tx.Where("id = ?", in.WorkspaceID).Take(&ws).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: workspace %s", craft.ErrNotFound, in.WorkspaceID)
		}
		if e != nil {
			return e
		}
		if ws.TenantID != scope.TenantID || ws.SessionID != scope.SessionID {
			return fmt.Errorf("%w: workspace %s is bound to another session", craft.ErrNotFound, in.WorkspaceID)
		}
		if ws.OwnerID != scope.UserID {
			return fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
		}
		if expectedHead != nil {
			if expectedHead.WorkspaceID != in.WorkspaceID || expectedHead.State != craft.DraftHeadSelected || expectedHead.SourceRunID != in.RunID {
				return fmt.Errorf("%w: invalid expected draft head for version %s", craft.ErrConflict, in.ID)
			}
			locked := tx.Model(&craftDraftHeadRow{}).
				Where("workspace_id = ? AND tenant_id = ? AND revision = ? AND state = ? AND source_run_id = ? AND manifest_digest = ?",
					in.WorkspaceID, scope.TenantID, expectedHead.Revision, string(craft.DraftHeadSelected), expectedHead.SourceRunID, expectedHead.ManifestDigest).
				UpdateColumn("updated_at", gorm.Expr("updated_at"))
			if locked.Error != nil {
				return locked.Error
			}
			if locked.RowsAffected != 1 {
				return fmt.Errorf("%w: workspace draft head changed before version publish", craft.ErrConflict)
			}
			var revision craftDraftRevisionRow
			if err := tx.Where("workspace_id = ? AND tenant_id = ? AND revision = ? AND source_run_id = ? AND manifest_digest = ?",
				in.WorkspaceID, scope.TenantID, expectedHead.Revision, expectedHead.SourceRunID, expectedHead.ManifestDigest).Take(&revision).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("%w: immutable draft revision no longer matches promotion", craft.ErrConflict)
				}
				return err
			}
			if digest != expectedHead.ManifestDigest {
				return fmt.Errorf("%w: published files do not match selected draft head manifest", craft.ErrConflict)
			}
		}

		// created_at is written by the store, not the database default: the
		// SQL CURRENT_TIMESTAMP defaults have whole-second precision, which
		// would order two same-second publishes arbitrarily. The Go clock's
		// sub-second precision keeps "newest first" deterministic.
		row := craftVersionRow{
			ID: in.ID, TenantID: scope.TenantID, WorkspaceID: in.WorkspaceID,
			RunID: in.RunID, Kind: in.Kind, ManifestHash: digest,
			ChecksJSON: string(encoded), CreatedAt: time.Now().UTC(),
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			fileRows := make([]craftVersionFileRow, 0, len(in.Files))
			for _, f := range in.Files {
				fileRows = append(fileRows, craftVersionFileRow{
					VersionID: in.ID, Path: f.Path, ResourceRef: f.Ref,
					FileHash: f.SHA256, FileBytes: f.Bytes, MIMEType: f.MIME,
				})
			}
			if len(fileRows) > 0 {
				if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fileRows).Error; e != nil {
					return e
				}
			}
			if evidence != nil {
				// This transaction created the version row above, so its
				// evidence member cannot pre-exist (the FK pins it to this
				// version): a plain insert is the whole write.
				if e := tx.Create(&evidenceRow).Error; e != nil {
					return e
				}
			}
			out = in
			return nil
		}

		// Lost the identity race (or this is a replay): only the identical
		// publish may adopt the stored version.
		stored, e := loadCraftVersion(tx, in.ID)
		if e != nil {
			return e
		}
		if !sameCraftVersion(stored, in) {
			return fmt.Errorf("%w: version %s already published with different content", craft.ErrConflict, in.ID)
		}
		if evidence != nil {
			// The version row pre-dates this call. It can only be without an
			// evidence member when it was published by an earlier,
			// evidence-less route (a pre-T07 deployment upgrade, or the
			// evidence-less Publish paths still serving other collections):
			// retro-pinning it would reconstruct history under a fresh
			// PinnedAt, so the store refuses exactly like different content —
			// only adoption of already-pinned identical evidence is allowed.
			if e := adoptCraftVersionEvidence(tx, evidenceRow); e != nil {
				return e
			}
		}
		out = stored
		return nil
	})
	if err != nil {
		return craft.Version{}, err
	}
	return out, nil
}

// craftVersionEvidenceRow is the T07 (#131) immutable evidence member of one
// version. Its production table is allocated by the central migration
// owner; the JSON plus its integrity digest are the source of truth, exactly
// like the Run's knowledge record row.
type craftVersionEvidenceRow struct {
	VersionID    string    `gorm:"column:version_id;primaryKey"`
	TenantID     uint64    `gorm:"column:tenant_id;not null"`
	EvidenceJSON string    `gorm:"column:evidence_json;type:text;not null"`
	Digest       string    `gorm:"column:digest;type:char(64);not null"`
	AcquiredAt   time.Time `gorm:"column:acquired_at;not null"`
	PinnedAt     time.Time `gorm:"column:pinned_at;not null"`
}

func (craftVersionEvidenceRow) TableName() string { return "craft_version_evidence" }

// sameCraftVersionEvidence reports evidence identity for replay adoption:
// every frozen fact must match. PinnedAt is deliberately excluded — it is
// the promotion's clock reading, not a fact of the observation, exactly as
// the version row's created_at is excluded from sameCraftVersion — so a
// replayed identical promotion adopts the stored evidence instead of
// conflicting with its own timestamp.
func sameCraftVersionEvidence(a, b craft.VersionEvidence) bool {
	if a.VersionID != b.VersionID || a.RunID != b.RunID ||
		a.RequestDigest != b.RequestDigest || a.PackageDigest != b.PackageDigest ||
		!a.AcquiredAt.Equal(b.AcquiredAt) || a.Empty != b.Empty || a.Truncated != b.Truncated ||
		len(a.Sources) != len(b.Sources) {
		// AcquiredAt compares with Equal (monotonic clock/location-safe): a
		// bare != on time.Time is Location-representation sensitive and one
		// future writer forgetting .UTC() would break idempotent adoption
		// with phantom conflicts.
		return false
	}
	for i := range a.Sources {
		if !sameKnowledgeSourceRecord(a.Sources[i], b.Sources[i]) {
			return false
		}
	}
	return true
}

// sameKnowledgeSourceRecord compares one recorded source with the SAME
// location-representation safety the top-level AcquiredAt uses: KnowledgeSourceRecord
// carries its own AcquiredAt time.Time, and a bare != compares clock
// representation — one side serialized in a different offset decodes
// "unequal" and turns an idempotent replay into a phantom conflict.
func sameKnowledgeSourceRecord(a, b craft.KnowledgeSourceRecord) bool {
	return a.ID == b.ID && a.Ref == b.Ref && a.Digest == b.Digest &&
		a.TenantID == b.TenantID && a.ExcerptBytes == b.ExcerptBytes &&
		a.AcquiredAt.Equal(b.AcquiredAt)
}

// adoptCraftVersionEvidence adopts the evidence already pinned to an
// existing version row. A stored version WITHOUT an evidence member was
// published by an earlier, evidence-less route (a pre-T07 deployment, or
// the evidence-less Publish paths): retro-pinning it would reconstruct
// history under a fresh PinnedAt, so it is refused like different content.
// Identical frozen facts (PinnedAt excluded, exactly like the replay
// adoption of the version row) adopt silently.
func adoptCraftVersionEvidence(tx *gorm.DB, row craftVersionEvidenceRow) error {
	var stored craftVersionEvidenceRow
	if err := tx.Where("version_id = ?", row.VersionID).Take(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: version %s already published without evidence", craft.ErrConflict, row.VersionID)
		}
		return err
	}
	// Integrity parity with the read path: verify the stored bytes against
	// their digest BEFORE adopting them, so a tampered row whose decoded
	// facts happen to match the pinned replay cannot be silently adopted.
	if sum := sha256.Sum256([]byte(stored.EvidenceJSON)); hex.EncodeToString(sum[:]) != stored.Digest {
		return fmt.Errorf("%w: stored evidence for version %s fails its integrity digest", craft.ErrCorruptEvidence, row.VersionID)
	}
	var pinned craft.VersionEvidence
	if err := json.Unmarshal([]byte(row.EvidenceJSON), &pinned); err != nil {
		return err
	}
	var storedEvidence craft.VersionEvidence
	if err := json.Unmarshal([]byte(stored.EvidenceJSON), &storedEvidence); err != nil {
		return fmt.Errorf("%w: corrupt stored evidence for version %s: %v", craft.ErrCorruptEvidence, row.VersionID, err)
	}
	if !sameCraftVersionEvidence(storedEvidence, pinned) {
		return fmt.Errorf("%w: version %s already pinned different evidence", craft.ErrConflict, row.VersionID)
	}
	return nil
}

// PublishWithEvidence is the T07 promotion write: one transaction publishes
// the immutable version and pins ev as its evidence member. The evidence
// must bind this exact version identity; the identical replay adopts the
// stored rows and a different evidence under the same version id is a
// conflict.
func (s *CraftVersionStore) PublishWithEvidence(ctx context.Context, scope craft.Scope, v craft.Version, ev craft.VersionEvidence) (craft.Version, error) {
	return s.publish(ctx, scope, v, &ev, nil)
}

// VersionEvidence returns the evidence pinned to one published version,
// read strictly from the stored snapshot keyed by Version ID. The read
// carries the same scope ACL as the version itself; a tampered row is
// refused by its integrity digest; a version promoted without evidence
// answers ErrNotFound — history is never reconstructed from the Workspace,
// the current knowledge base or the Run's live record.
func (s *CraftVersionStore) VersionEvidence(ctx context.Context, scope craft.Scope, versionID string) (craft.VersionEvidence, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || !craft.ValidVersionID(versionID) {
		return craft.VersionEvidence{}, fmt.Errorf("%w: incomplete version evidence request", craft.ErrInvalidInput)
	}
	var row craftVersionRow
	err := s.db.WithContext(ctx).Where("id = ?", versionID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.VersionEvidence{}, err
	}
	if e := authorizeCraftVersion(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.VersionEvidence{}, e
	}
	var evidenceRow craftVersionEvidenceRow
	err = s.db.WithContext(ctx).Where("version_id = ?", versionID).Take(&evidenceRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.VersionEvidence{}, err
	}
	// Integrity check: the digest column must be the SHA-256 of exactly the
	// stored bytes — the byte-for-byte contract craft.EncodeVersionEvidence
	// established at write time. Hashing the RAW stored bytes (never a
	// decode-then-re-encode) keeps byte-level tampering detectable.
	sum := sha256.Sum256([]byte(evidenceRow.EvidenceJSON))
	if hex.EncodeToString(sum[:]) != evidenceRow.Digest {
		return craft.VersionEvidence{}, fmt.Errorf("%w: version %s evidence digest mismatch", craft.ErrCorruptEvidence, versionID)
	}
	var ev craft.VersionEvidence
	if err := json.Unmarshal([]byte(evidenceRow.EvidenceJSON), &ev); err != nil {
		return craft.VersionEvidence{}, fmt.Errorf("%w: corrupt version %s evidence: %v", craft.ErrCorruptEvidence, versionID, err)
	}
	if ev.VersionID != versionID {
		return craft.VersionEvidence{}, fmt.Errorf("%w: version %s evidence binds %s", craft.ErrConflict, versionID, ev.VersionID)
	}
	if err := craft.ValidateVersionEvidence(ev); err != nil {
		return craft.VersionEvidence{}, fmt.Errorf("%w: corrupt version %s evidence: %v", craft.ErrCorruptEvidence, versionID, err)
	}
	return ev, nil
}

// List returns the scope's workspace versions, newest first. A session
// without a workspace binding has nothing to list.
func (s *CraftVersionStore) List(ctx context.Context, scope craft.Scope) ([]craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return nil, fmt.Errorf("%w: incomplete version scope", craft.ErrInvalidInput)
	}
	var ws craftWorkspaceRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).
		Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, craft.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ws.OwnerID != scope.UserID {
		return nil, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	var rows []craftVersionRow
	err = s.db.WithContext(ctx).Where("workspace_id = ?", ws.ID).
		Order("created_at DESC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	versions := make([]craft.Version, 0, len(rows))
	for _, row := range rows {
		v, e := loadCraftVersion(s.db.WithContext(ctx), row.ID)
		if e != nil {
			return nil, e
		}
		versions = append(versions, v)
	}
	return versions, nil
}

// Get returns one published version in the requesting scope.
func (s *CraftVersionStore) Get(ctx context.Context, scope craft.Scope, id string) (craft.Version, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || id == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete version request", craft.ErrInvalidInput)
	}
	var row craftVersionRow
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Version{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Version{}, err
	}
	if e := authorizeCraftVersion(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.Version{}, e
	}
	return loadCraftVersion(s.db.WithContext(ctx), id)
}
