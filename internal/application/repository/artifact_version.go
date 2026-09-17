package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Artifact version scan/import lifecycle. A version row is created pending,
// flips to uploaded once the object bytes land at the derived key, then to
// clean or quarantined by the scanner, and only a clean version may be
// published (ready). Quarantine is terminal for that version identity: the
// producer must import a new version ID.
const (
	ArtifactScanPending     = "pending"
	ArtifactScanUploaded    = "uploaded"
	ArtifactScanClean       = "clean"
	ArtifactScanQuarantined = "quarantined"
	ArtifactScanReady       = "ready"
)

// MaxArtifactVersionBytes caps the object size a version row may describe.
// The paseo-adapter reads exports with the same ceiling so a runaway producer
// cannot stream unbounded bytes through the bridge.
const MaxArtifactVersionBytes = 8 << 30 // 8 GiB

var (
	// ErrArtifactVersionConflict reports an attempt to rewrite the immutable
	// content of an existing artifact version.
	ErrArtifactVersionConflict = errors.New("artifact version conflict")
	// ErrArtifactVersionNotFound reports a missing, non-readable, or
	// not-scoped artifact version. All three are deliberately
	// indistinguishable so callers cannot probe other tenants' versions.
	ErrArtifactVersionNotFound = errors.New("artifact version not found")
	// ErrArtifactVersionState reports a lifecycle transition that is out of
	// order or forbidden (for example publishing a quarantined version).
	ErrArtifactVersionState = errors.New("artifact version state conflict")
	// ErrArtifactMetadataInvalid reports metadata that fails validation
	// before any durable write.
	ErrArtifactMetadataInvalid = errors.New("invalid artifact metadata")
)

// ArtifactVersion is the immutable durable record of one imported execution
// output object. Content fields (RunID, SessionID, Digest, ObjectKey, MIME,
// Size) are frozen at insert; only ScanState advances through server-owned
// transitions.
type ArtifactVersion struct {
	TenantID  uint64
	ID        string
	RunID     string
	SessionID string
	Digest    string
	ObjectKey string
	MIME      string
	ScanState string
	Size      int64
}

// ArtifactVersionStore owns the artifact_versions table. Inserts never
// upsert: replaying identical content is idempotent, and any difference
// against the stored version fails with ErrArtifactVersionConflict.
type ArtifactVersionStore struct {
	db *gorm.DB
}

// NewArtifactVersionStore constructs a store backed by the migrated business
// database.
func NewArtifactVersionStore(db *gorm.DB) *ArtifactVersionStore {
	return &ArtifactVersionStore{db: db}
}

type artifactVersionRow struct {
	TenantID  uint64 `gorm:"column:tenant_id"`
	ID        string `gorm:"column:id"`
	RunID     string `gorm:"column:run_id"`
	SessionID string `gorm:"column:session_id"`
	Digest    string `gorm:"column:digest"`
	ObjectKey string `gorm:"column:object_key"`
	MIME      string `gorm:"column:mime"`
	ScanState string `gorm:"column:scan_state"`
	Size      int64  `gorm:"column:size"`
}

func (artifactVersionRow) TableName() string { return "artifact_versions" }

func (r artifactVersionRow) view() ArtifactVersion {
	return ArtifactVersion{
		TenantID:  r.TenantID,
		ID:        r.ID,
		RunID:     r.RunID,
		SessionID: r.SessionID,
		Digest:    r.Digest,
		ObjectKey: r.ObjectKey,
		MIME:      r.MIME,
		ScanState: r.ScanState,
		Size:      r.Size,
	}
}

func (r artifactVersionRow) sameContent(other artifactVersionRow) bool {
	return r.RunID == other.RunID &&
		r.SessionID == other.SessionID &&
		r.Digest == other.Digest &&
		r.ObjectKey == other.ObjectKey &&
		r.MIME == other.MIME &&
		r.Size == other.Size
}

// validateArtifactMetadata checks the size/digest pair before any durable
// write. Digests are sha256 hex (64 lowercase hex characters); uppercase is
// rejected so digests compare byte-identically across producers.
func validateArtifactMetadata(size int64, digest string) error {
	if size <= 0 {
		return fmt.Errorf("%w: size must be positive", ErrArtifactMetadataInvalid)
	}
	if size > MaxArtifactVersionBytes {
		return fmt.Errorf("%w: size exceeds MaxArtifactVersionBytes", ErrArtifactMetadataInvalid)
	}
	if len(digest) != 64 {
		return fmt.Errorf("%w: digest must be 64 hex characters", ErrArtifactMetadataInvalid)
	}
	for i := 0; i < len(digest); i++ {
		c := digest[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return fmt.Errorf("%w: digest must be lowercase hex", ErrArtifactMetadataInvalid)
	}
	return nil
}

func validArtifactScanState(state string) bool {
	switch state {
	case ArtifactScanPending, ArtifactScanUploaded, ArtifactScanClean, ArtifactScanQuarantined, ArtifactScanReady:
		return true
	}
	return false
}

// DeriveArtifactObjectKey computes the server-side object key for a version.
// The key is deterministic and content-addressed (tenant, run, digest), so an
// upload that lands before its metadata transaction commits can always be
// reconciled by replaying the import. Clients never supply object keys.
func DeriveArtifactObjectKey(tenantID uint64, runID, digest string) string {
	return fmt.Sprintf("artifact-versions/%d/%s/%s", tenantID, runID, digest)
}

// requireArtifactRun fails closed unless the exact (tenant, run, session)
// triple exists in agent_runs, so a version can never attach to a fabricated
// or cross-scoped run.
func requireArtifactRun(ctx context.Context, db *gorm.DB, row artifactVersionRow) error {
	var exists int64
	err := db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND run_id = ? AND session_id = ?", row.TenantID, row.RunID, row.SessionID).
		Count(&exists).Error
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrArtifactVersionNotFound
	}
	return nil
}

func insertArtifactVersionRow(ctx context.Context, tx *gorm.DB, row artifactVersionRow) error {
	var existing artifactVersionRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", row.TenantID, row.ID).Take(&existing).Error
	if err == nil {
		// Immutable: identical content replays as a no-op, any difference
		// conflicts. ScanState is excluded because it advances through
		// server-owned transitions after the content is frozen.
		if existing.sameContent(row) {
			return nil
		}
		return ErrArtifactVersionConflict
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&row).Error
}

func (s *ArtifactVersionStore) validateInsert(ctx context.Context, row artifactVersionRow) error {
	if row.TenantID == 0 || row.ID == "" || row.RunID == "" || row.SessionID == "" {
		return fmt.Errorf("%w: tenant, id, run and session are required", ErrArtifactMetadataInvalid)
	}
	if strings.TrimSpace(row.MIME) == "" {
		return fmt.Errorf("%w: mime is required", ErrArtifactMetadataInvalid)
	}
	if row.ObjectKey == "" {
		return fmt.Errorf("%w: object key is required", ErrArtifactMetadataInvalid)
	}
	if err := validateArtifactMetadata(row.Size, row.Digest); err != nil {
		return err
	}
	if !validArtifactScanState(row.ScanState) {
		return fmt.Errorf("%w: unknown scan state %q", ErrArtifactMetadataInvalid, row.ScanState)
	}
	return requireArtifactRun(ctx, s.db, row)
}

// Insert persists a version row without upserting. The provided ObjectKey is
// stored as given; production entry points must use Import, which derives the
// key server-side.
func (s *ArtifactVersionStore) Insert(ctx context.Context, version ArtifactVersion) error {
	row := artifactVersionRow(version)
	if err := s.validateInsert(ctx, row); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return insertArtifactVersionRow(ctx, tx, row)
	})
}

// Import is the production entry point. It ignores any client-supplied
// ObjectKey and ScanState: the key is derived from the immutable identity
// and new versions always start pending. Replaying an import is idempotent
// and returns the persisted row (including its current scan state), which is
// what reconciliation after a lost metadata transaction replays.
func (s *ArtifactVersionStore) Import(ctx context.Context, version ArtifactVersion) (ArtifactVersion, error) {
	row := artifactVersionRow(version)
	row.ObjectKey = DeriveArtifactObjectKey(row.TenantID, row.RunID, row.Digest)
	row.ScanState = ArtifactScanPending
	if err := s.validateInsert(ctx, row); err != nil {
		return ArtifactVersion{}, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return insertArtifactVersionRow(ctx, tx, row)
	})
	if err != nil {
		return ArtifactVersion{}, err
	}
	return s.Get(ctx, row.TenantID, row.ID)
}

// Get loads a version in any scan state (internal/ownership tooling).
func (s *ArtifactVersionStore) Get(ctx context.Context, tenantID uint64, id string) (ArtifactVersion, error) {
	if tenantID == 0 || id == "" {
		return ArtifactVersion{}, ErrArtifactVersionNotFound
	}
	var row artifactVersionRow
	err := s.db.WithContext(ctx).Table("artifact_versions").
		Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ArtifactVersion{}, ErrArtifactVersionNotFound
	}
	if err != nil {
		return ArtifactVersion{}, err
	}
	return row.view(), nil
}

// ReadableArtifactVersion loads a version only when it is published (ready)
// and scoped to the caller's tenant and session. Anything else reads as
// not-found so unreachable versions cannot be probed.
func (s *ArtifactVersionStore) ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, id string) (ArtifactVersion, error) {
	if tenantID == 0 || sessionID == "" || id == "" {
		return ArtifactVersion{}, ErrArtifactVersionNotFound
	}
	var row artifactVersionRow
	err := s.db.WithContext(ctx).Table("artifact_versions").
		Where("tenant_id = ? AND session_id = ? AND id = ? AND scan_state = ?", tenantID, sessionID, id, ArtifactScanReady).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ArtifactVersion{}, ErrArtifactVersionNotFound
	}
	if err != nil {
		return ArtifactVersion{}, err
	}
	return row.view(), nil
}

// ListByRun returns every version of a run inside one tenant, in creation
// order, regardless of scan state.
func (s *ArtifactVersionStore) ListByRun(ctx context.Context, tenantID uint64, runID string) ([]ArtifactVersion, error) {
	if tenantID == 0 || runID == "" {
		return nil, ErrArtifactVersionNotFound
	}
	var rows []artifactVersionRow
	err := s.db.WithContext(ctx).Table("artifact_versions").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
		Order("created_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	versions := make([]ArtifactVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, row.view())
	}
	return versions, nil
}

func (s *ArtifactVersionStore) transitionScanState(ctx context.Context, tenantID uint64, id, from, to string) error {
	updated := s.db.WithContext(ctx).Table("artifact_versions").
		Where("tenant_id = ? AND id = ? AND scan_state = ?", tenantID, id, from).
		Updates(map[string]any{"scan_state": to, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected == 1 {
		return nil
	}
	current, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: version %s is %s, cannot become %s", ErrArtifactVersionState, id, current.ScanState, to)
}

// MarkUploaded records that the object bytes landed at the derived key.
// Re-observing the upload after later states is a no-op; a quarantined
// version can no longer advance.
func (s *ArtifactVersionStore) MarkUploaded(ctx context.Context, tenantID uint64, id string) error {
	err := s.transitionScanState(ctx, tenantID, id, ArtifactScanPending, ArtifactScanUploaded)
	if err == nil || !errors.Is(err, ErrArtifactVersionState) {
		return err
	}
	current, getErr := s.Get(ctx, tenantID, id)
	if getErr != nil {
		return getErr
	}
	switch current.ScanState {
	case ArtifactScanUploaded, ArtifactScanClean, ArtifactScanReady:
		return nil
	}
	return err
}

// MarkScanned records the scanner verdict. A failed scan quarantines the
// version permanently: no URL may be published for it and the producer must
// import a fresh version ID.
func (s *ArtifactVersionStore) MarkScanned(ctx context.Context, tenantID uint64, id string, clean bool) error {
	outcome := ArtifactScanQuarantined
	if clean {
		outcome = ArtifactScanClean
	}
	err := s.transitionScanState(ctx, tenantID, id, ArtifactScanUploaded, outcome)
	if err == nil || !errors.Is(err, ErrArtifactVersionState) {
		return err
	}
	current, getErr := s.Get(ctx, tenantID, id)
	if getErr != nil {
		return getErr
	}
	if clean && (current.ScanState == ArtifactScanClean || current.ScanState == ArtifactScanReady) {
		return nil
	}
	if !clean && current.ScanState == ArtifactScanQuarantined {
		return nil
	}
	return err
}

// MarkReady publishes a clean version. Pending, uploaded, and quarantined
// versions can never be published.
func (s *ArtifactVersionStore) MarkReady(ctx context.Context, tenantID uint64, id string) error {
	err := s.transitionScanState(ctx, tenantID, id, ArtifactScanClean, ArtifactScanReady)
	if err == nil || !errors.Is(err, ErrArtifactVersionState) {
		return err
	}
	if current, getErr := s.Get(ctx, tenantID, id); getErr == nil && current.ScanState == ArtifactScanReady {
		return nil
	}
	return err
}
