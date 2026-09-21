// Package service - Craft immutable artifact versions (W01).
//
// CraftArtifactService turns one finished delegation's workspace output into
// an immutable craft.Version: files are read from the sandbox output the
// session is bound to, validated against the collection rules, uploaded
// through the same resource storage the tenant already uses, and only then
// published through craft.VersionStore in a single transaction. Identity
// follows content (path + SHA-256 + size, sorted by path), never path+mtime,
// so a rewrite under an unchanged mtime still produces a new version and old
// versions keep serving their pinned bytes forever.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"path"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Collection caps. Defaults mirror the artifact collector's per-file bound;
// the total bound keeps one runaway round from exhausting process memory.
const (
	defaultCraftMaxArtifactFileBytes  int64 = 50 << 20
	defaultCraftMaxTotalArtifactBytes int64 = 200 << 20
)

// CraftArtifactConfig bounds one collection round. Every field has a safe
// zero-value default applied by withDefaults.
type CraftArtifactConfig struct {
	// Kind is the artwork kind this service collects for; it selects the
	// entry-check rule. Defaults to craft.KindWeb until further kinds are
	// gated in (O05).
	Kind string

	// OutputDir is the sandbox directory scanned for artifact files. Empty
	// falls back to skills.ArtifactOutputDir() — the same directory the
	// runtime tells skill scripts to write to.
	OutputDir string

	// MaxFileBytes caps one artifact file, checked against both the listed
	// size and the actually-read bytes.
	MaxFileBytes int64

	// MaxTotalBytes caps the summed bytes of one round.
	MaxTotalBytes int64
}

func (c CraftArtifactConfig) withDefaults() CraftArtifactConfig {
	if strings.TrimSpace(c.Kind) == "" {
		c.Kind = craft.KindWeb
	}
	if strings.TrimSpace(c.OutputDir) == "" {
		c.OutputDir = skills.ArtifactOutputDir()
	}
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = defaultCraftMaxArtifactFileBytes
	}
	if c.MaxTotalBytes <= 0 {
		c.MaxTotalBytes = defaultCraftMaxTotalArtifactBytes
	}
	return c
}

// ArtifactEvidenceSource supplies the externally observed verification facts
// for one delegation: the build step's exit code and, later, W02's preview
// verdict. It may be nil — the collector then reports those checks as
// not_run instead of guessing. Whatever it returns is recorded verbatim;
// facts that never happened are never fabricated here.
type ArtifactEvidenceSource func(ctx context.Context, task craft.Task) craft.ArtifactEvidence

// CraftArtifactService collects one delegation's workspace output into an
// immutable, run-linked version.
type CraftArtifactService struct {
	source   SandboxArtifactSource
	files    interfaces.FileService
	versions craft.VersionStore
	evidence ArtifactEvidenceSource
	config   CraftArtifactConfig
}

// NewCraftArtifactService assembles the artifact service from the
// session-bound sandbox source, the tenant resource storage and the version
// store. evidence may be nil.
func NewCraftArtifactService(
	source SandboxArtifactSource,
	files interfaces.FileService,
	versions craft.VersionStore,
	evidence ArtifactEvidenceSource,
	config CraftArtifactConfig,
) *CraftArtifactService {
	if source == nil || files == nil || versions == nil {
		panic("craft: NewCraftArtifactService requires a source, file service and version store")
	}
	return &CraftArtifactService{
		source:   source,
		files:    files,
		versions: versions,
		evidence: evidence,
		config:   config.withDefaults(),
	}
}

// stagedArtifact is one accepted output file with its bytes in memory,
// already validated against the collection rules.
type stagedArtifact struct {
	rel  string
	data []byte
}

// Collect reads the delegation's workspace output and publishes it as one
// immutable version, using the service's configured kind. Delegations whose
// session kind is known at call time use CollectForKind instead.
//
// Ordering is the W01 contract:
//
//  1. every listed entry is validated (regular file only — symlinks abort,
//     no traversal, no credentials, per-file and total caps) and read;
//  2. every file is uploaded through the resource storage BEFORE any
//     version row exists, so a visible version always has all its objects;
//     objects stranded by a later failure are left for O03's deferred
//     reclamation;
//  3. the version — identity derived from the content manifest — publishes
//     through the store in one transaction. Only a published version is ever
//     associated with tool results, via its RunID.
//
// A failed collection returns an error and leaves no version visible.
func (s *CraftArtifactService) Collect(ctx context.Context, task craft.Task) (craft.Version, error) {
	return s.CollectForKind(ctx, task, s.config.Kind)
}

// CollectForKind is the kind-aware collection entrance (D01 wiring): the
// version is stamped with the SESSION's kind, the entry check judges that
// kind's own deliverable, and — for the kinds whose skill writes a
// manifest.json — the manifest is decoded and validated server-side BEFORE
// anything publishes. A skill cannot fake its way past the gate by writing
// checks: "passed" inside a structurally invalid manifest publishes nothing.
func (s *CraftArtifactService) CollectForKind(ctx context.Context, task craft.Task, kind string) (craft.Version, error) {
	if s == nil || s.source == nil || s.files == nil || s.versions == nil {
		return craft.Version{}, fmt.Errorf("%w: artifact service is not assembled", craft.ErrInvalidInput)
	}
	if !craft.KnownKind(kind) {
		return craft.Version{}, fmt.Errorf("%w: unknown artifact kind %q", craft.ErrInvalidInput, kind)
	}
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.WorkspaceID == "" || task.Fence.RunID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete delegation request", craft.ErrInvalidInput)
	}

	entries, err := s.source.ListSessionFiles(ctx, task.Scope.SessionID, s.config.OutputDir)
	if err != nil {
		return craft.Version{}, fmt.Errorf("craft: list workspace output: %w", err)
	}

	staged := make([]stagedArtifact, 0, len(entries))
	var total int64
	for _, entry := range entries {
		switch entry.Type {
		case sandbox.RemoteEntryDir:
			continue // directory structure, not content
		case sandbox.RemoteEntryFile:
		default:
			// Symlinks and devices: the collector cannot verify where they
			// point, so the round fails closed instead of following them.
			return craft.Version{}, fmt.Errorf("%w: output entry %q is not a regular file",
				craft.ErrInvalidInput, entry.Path)
		}
		rel, err := craft.ArtifactRelativePath(s.config.OutputDir, entry.Path)
		if err != nil {
			return craft.Version{}, err
		}
		if entry.Size > s.config.MaxFileBytes {
			return craft.Version{}, fmt.Errorf("%w: artifact %q lists %d bytes over the %d cap",
				craft.ErrInvalidInput, rel, entry.Size, s.config.MaxFileBytes)
		}
		data, err := s.source.ReadSessionFile(ctx, task.Scope.SessionID, entry.Path)
		if err != nil {
			return craft.Version{}, fmt.Errorf("craft: read artifact %q: %w", rel, err)
		}
		if int64(len(data)) > s.config.MaxFileBytes {
			return craft.Version{}, fmt.Errorf("%w: artifact %q read %d bytes over the %d cap",
				craft.ErrInvalidInput, rel, len(data), s.config.MaxFileBytes)
		}
		total += int64(len(data))
		if total > s.config.MaxTotalBytes {
			return craft.Version{}, fmt.Errorf("%w: round read %d bytes over the %d total cap",
				craft.ErrInvalidInput, total, s.config.MaxTotalBytes)
		}
		staged = append(staged, stagedArtifact{rel: rel, data: data})
	}
	sort.Slice(staged, func(i, j int) bool { return staged[i].rel < staged[j].rel })

	// Server-side manifest admission (D01/D02/D03 wiring): the kinds whose
	// skill contract is a manifest.json must carry one, and it must validate
	// against the kind's own rules — recalc/preview/parse for spreadsheets,
	// render/pages/sources for slides, generate/modify/preview/export for
	// documents. The gate fires BEFORE any upload or publish, so a faked or
	// inconsistent manifest leaves no version visible at all.
	if err := craftValidateStagedManifest(kind, staged); err != nil {
		logger.Warnf(ctx, "[CraftArtifact] manifest admission rejected the round: %v", err)
		return craft.Version{}, err
	}

	// Upload the complete object set first; the manifest is all-or-nothing.
	files := make([]craft.File, 0, len(staged))
	for _, a := range staged {
		sum := sha256.Sum256(a.data)
		digest := hex.EncodeToString(sum[:])
		// Content-addressed storage name: republishing the same bytes lands
		// on the same object, so retries never duplicate blobs.
		storageName := "craft_" + digest + "_" + safeFileName(path.Base(a.rel))
		ref, err := s.files.SaveBytes(ctx, a.data, task.Scope.TenantID, storageName, false)
		if err != nil {
			logger.Warnf(ctx, "[CraftArtifact] upload failed for %s: %v (already-uploaded objects stay for O03 deferred reclamation)", a.rel, err)
			return craft.Version{}, fmt.Errorf("craft: upload artifact %q: %w", a.rel, err)
		}
		files = append(files, craft.File{
			Path: a.rel, Ref: ref, SHA256: digest,
			MIME: craftArtifactMIME(a.rel), Bytes: int64(len(a.data)),
		})
	}

	digest, err := craft.ManifestDigest(files)
	if err != nil {
		return craft.Version{}, err
	}
	evidence := craft.ArtifactEvidence{}
	if s.evidence != nil {
		evidence = s.evidence(ctx, task)
	}
	version := craft.Version{
		ID:          craft.VersionID(task.WorkspaceID, task.Fence.RunID, digest),
		WorkspaceID: task.WorkspaceID,
		RunID:       task.Fence.RunID,
		Kind:        kind,
		Files:       files,
		Checks:      craft.BuildChecks(kind, files, evidence),
	}
	published, err := s.versions.Publish(ctx, task.Scope, version)
	if err != nil {
		logger.Warnf(ctx, "[CraftArtifact] publish failed for run %s: %v (uploaded objects stay for O03 deferred reclamation)", task.Fence.RunID, err)
		return craft.Version{}, err
	}
	logger.Infof(ctx, "[CraftArtifact] published version %s workspace %s run %s files %d",
		published.ID, task.WorkspaceID, task.Fence.RunID, len(published.Files))
	return published, nil
}

// craftArtifactMIME reports the MIME type served for one artifact path; the
// fallback keeps unknown extensions downloadable as opaque bytes.
func craftArtifactMIME(rel string) string {
	if t := mime.TypeByExtension(path.Ext(rel)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// craftManifestGateKinds lists the kinds whose skill contract is a
// manifest.json the server must admit before publishing. Web keeps the
// entry-check-only gate (its skill ships no manifest).
func craftManifestGateKind(kind string) bool {
	return kind == craft.KindDocument || kind == craft.KindSpreadsheet || kind == craft.KindSlides
}

// craftValidateStagedManifest decodes the round's output/manifest.json and
// runs the kind's own validator against it. A gated kind without a
// manifest, a manifest that is not JSON, or one that fails its kind's rules
// (wrong deliverable paths, unready artifacts, failed or missing checks,
// invented citations…) refuses the whole collection — fail closed.
func craftValidateStagedManifest(kind string, staged []stagedArtifact) error {
	if !craftManifestGateKind(kind) {
		return nil
	}
	for _, artifact := range staged {
		if artifact.rel != "manifest.json" {
			continue
		}
		switch kind {
		case craft.KindDocument:
			var m craft.DocumentManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: document manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateDocumentManifest(m); err != nil {
				return err
			}
		case craft.KindSpreadsheet:
			var m craft.SpreadsheetManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: spreadsheet manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateSpreadsheetManifest(m); err != nil {
				return err
			}
		case craft.KindSlides:
			var m craft.SlideManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: slides manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateSlidesManifest(m); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("%w: kind %q requires output/manifest.json (gate checks) for admission; none was collected", craft.ErrInvalidInput, kind)
}
