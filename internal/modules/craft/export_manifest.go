package craft

// T12 (#132): the version-bound source bundle's export manifest and
// citation/source manifest.
//
// Both projections are PURE FUNCTIONS OF IMMUTABLE FACTS ONLY — the
// version's pinned file manifest, its recorded checks and the version's
// pinned evidence (T07). The Workspace, the current knowledge base and the
// Run's live record are never inputs, so a historical version's bundle
// digest stays stable after any later edit. Restricted originals are
// excluded by construction: the manifest lists generated artifact members
// only, and every cited original appears as a durable authenticated
// reference (craftkb://) that resolves solely through the viewer's own
// fresh authorization (T10) — the manifest itself grants no access.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Fixed documents every bundle carries beside the version's own members.
// The names are part of the bundle contract and are RESERVED:
// ValidateExportBundleMembers refuses any member path that collides with
// one (these three are flat reserved names the packager owns), so a bundle
// never carries duplicate entries whose unpack order would be undefined.
const (
	// BundleManifestPath is the machine-readable export manifest inside the
	// bundle: member digests plus knowledge origins and its own canonical
	// digest. T13's owner consent binds exactly this digest.
	BundleManifestPath = "export-manifest.json"
	// BundleSourcesPath is the human-readable citation/source manifest:
	// stable citation identity, title, digest and acquisition time, and the
	// authenticated source reference for every source the Run used.
	BundleSourcesPath = "sources.json"
	// BundleBuildPath is the build metadata: the version's kind and its
	// independently recorded checks.
	BundleBuildPath = "build.json"
)

// BundleSourceSchema is the only accepted citation/source manifest schema.
const BundleSourceSchema = 1

// BundleSource is one entry of the bundle's citation/source manifest. Every
// fact except Title is pinned verbatim from the version's evidence; Title is
// a best-effort display name resolved from the current library rows and is
// deliberately NOT part of the export manifest's identity.
type BundleSource struct {
	// CitationID is the stable citation identity (kc_ + 24 hex).
	CitationID string `json:"citation_id"`
	// Ref is the durable authenticated source reference (craftkb://…).
	// Opening it always re-authorizes against the CURRENT viewer's grant.
	Ref string `json:"ref"`
	// Digest is the source's content digest observed at acquisition time.
	Digest string `json:"digest"`
	// AcquiredAt is when the producing Run observed this source.
	AcquiredAt time.Time `json:"acquired_at"`
	// Title is a display-only name; empty when the current library rows no
	// longer resolve (deleted or revoked originals still cite honestly).
	Title string `json:"title,omitempty"`
}

// BundleCitationManifest is the citation/source manifest document riding
// every bundle (sources.json). It carries ONLY stable identities, digests
// and times plus display titles — never excerpts, never provider URLs, and
// never any authority: downloading it opens nothing.
type BundleCitationManifest struct {
	Schema    int            `json:"schema"`
	VersionID string         `json:"version_id"`
	RunID     string         `json:"run_id"`
	PinnedAt  time.Time      `json:"pinned_at"`
	Empty     bool           `json:"empty"`
	Sources   []BundleSource `json:"sources"`
}

// BuildBundleSources derives the citation/source manifest's entries from the
// pinned evidence, ordered by citation id so identical evidence always
// serializes identically. titles maps knowledge coordinates (or any caller
// key) to display names; missing titles degrade to empty, never to guesses.
func BuildBundleSources(evidence VersionEvidence, titles func(*KnowledgeSourceRecord) string) ([]BundleSource, error) {
	if err := ValidateVersionEvidence(evidence); err != nil {
		return nil, err
	}
	sources := make([]BundleSource, 0, len(evidence.Sources))
	for i := range evidence.Sources {
		source := evidence.Sources[i]
		entry := BundleSource{
			CitationID: source.ID, Ref: source.Ref,
			Digest: source.Digest, AcquiredAt: source.AcquiredAt,
		}
		if titles != nil {
			entry.Title = titles(&source)
		}
		sources = append(sources, entry)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].CitationID < sources[j].CitationID })
	return sources, nil
}

// BuildBundleCitationManifest assembles the bundle's sources.json document
// from immutable facts plus best-effort display titles.
func BuildBundleCitationManifest(version Version, evidence VersionEvidence, titles map[string]string) (BundleCitationManifest, error) {
	if version.ID == "" {
		return BundleCitationManifest{}, fmt.Errorf("%w: bundle citation manifest requires the version id", ErrInvalidInput)
	}
	if evidence.VersionID != version.ID {
		return BundleCitationManifest{}, fmt.Errorf("%w: evidence %s does not bind version %s", ErrInvalidInput, evidence.VersionID, version.ID)
	}
	lookup := func(source *KnowledgeSourceRecord) string { return titles[source.ID] }
	sources, err := BuildBundleSources(evidence, lookup)
	if err != nil {
		return BundleCitationManifest{}, err
	}
	return BundleCitationManifest{
		Schema: BundleSourceSchema, VersionID: version.ID, RunID: evidence.RunID,
		PinnedAt: evidence.PinnedAt, Empty: evidence.Empty, Sources: sources,
	}, nil
}

// BuildExportManifest derives one version's export manifest (the T00 frozen
// DTO) from immutable facts only:
//
//   - Files lists exactly the version's own artifact members — generated
//     deliverables, never original knowledge material — each carrying the
//     knowledge origins the producing Run actually used (the pinned
//     evidence), with the cross-tenant sources marked restricted;
//   - ManifestDigest is the canonical digest of exactly this projection
//     (ExportManifestDigest): it covers member paths, digests and origins,
//     so a manifest differing only in origins hashes differently, and a
//     later Workspace edit cannot move it at all.
//
// tenantID is the OWNING task's tenant: sources recorded from another
// tenant (organization-shared libraries) are the restricted originals the
// first release excludes and T13's consent governs.
func BuildExportManifest(version Version, evidence VersionEvidence, tenantID uint64) (ExportManifest, error) {
	if err := ValidateVersionEvidence(evidence); err != nil {
		return ExportManifest{}, err
	}
	if evidence.VersionID != version.ID {
		return ExportManifest{}, fmt.Errorf("%w: evidence %s does not bind version %s", ErrInvalidInput, evidence.VersionID, version.ID)
	}
	if tenantID == 0 {
		return ExportManifest{}, fmt.Errorf("%w: export manifest requires the owning tenant", ErrInvalidInput)
	}
	origins := make([]ExportOriginRef, 0, len(evidence.Sources))
	for _, source := range evidence.Sources {
		origins = append(origins, ExportOriginRef{
			Kind: ExportOriginKnowledge, Ref: source.Ref, SHA256: source.Digest,
			Restricted: source.TenantID != tenantID,
		})
	}
	sort.Slice(origins, func(i, j int) bool {
		if origins[i].Ref != origins[j].Ref {
			return origins[i].Ref < origins[j].Ref
		}
		return origins[i].SHA256 < origins[j].SHA256
	})
	files := make([]ExportFile, 0, len(version.Files))
	for _, member := range version.Files {
		files = append(files, ExportFile{
			Path: member.Path, SHA256: member.SHA256,
			// Generated members are not restricted originals: restricted
			// ORIGINALS are excluded by construction (they never enter a
			// version manifest), and restricted DERIVED exports are T13's
			// consent-governed surface.
			Restricted: false,
			// Non-nil even when the pinned evidence is empty (a
			// first-class validated state): the T00 frozen consumer
			// contract rejects a nil Origins slice, so every member carries
			// an empty — never nil — origins vector.
			Origins: append([]ExportOriginRef{}, origins...),
		})
	}
	manifest := ExportManifest{VersionID: version.ID, Files: files}
	digest, err := ExportManifestDigest(manifest)
	if err != nil {
		return ExportManifest{}, err
	}
	manifest.ManifestDigest = digest
	return manifest, nil
}

// exportManifestEntry is the canonical per-file record hashed into an export
// manifest digest: path, content digest and origins (kind, ref, digest,
// restricted flag). Origins are part of identity — two manifests differing
// only in origins are different manifests.
type exportManifestEntry struct {
	Path    string            `json:"path"`
	SHA256  string            `json:"sha256"`
	Origins []ExportOriginRef `json:"origins"`
}

// exportManifestIdentity is the canonical form hashed by
// ExportManifestDigest. The manifest's own ManifestDigest field is excluded
// (it is derived from these bytes); files are ordered by path and each
// file's origins by (ref, digest), so listing order never changes identity.
type exportManifestIdentity struct {
	VersionID string                `json:"version_id"`
	Files     []exportManifestEntry `json:"files"`
}

// ExportManifestDigest derives the canonical digest of one export manifest:
// the SHA-256 of its canonical JSON encoding (files sorted by path, origins
// sorted, self-referential digest field excluded). Identical immutable
// inputs always derive the identical digest; any changed member content,
// member set or origin set changes it.
//
// The input check is deliberately NOT ExportManifest.Validate: that frozen
// consumer-side validation requires the digest to already be filled in,
// while THIS function is the one deriving it. The structural facts that
// matter for identity — well-formed paths, digests and origins — are
// validated directly here.
func ExportManifestDigest(m ExportManifest) (string, error) {
	if m.VersionID == "" {
		return "", fmt.Errorf("%w: export manifest requires its version", ErrInvalidInput)
	}
	entries := make([]exportManifestEntry, 0, len(m.Files))
	for _, f := range m.Files {
		if f.Path == "" || f.SHA256 == "" {
			return "", fmt.Errorf("%w: export member %q lacks its path or digest", ErrInvalidInput, f.Path)
		}
		if !ValidSHA256(f.SHA256) {
			return "", fmt.Errorf("%w: export member %q has a malformed digest", ErrInvalidInput, f.Path)
		}
		for _, origin := range f.Origins {
			if err := origin.Validate(); err != nil {
				return "", err
			}
		}
		origins := append([]ExportOriginRef(nil), f.Origins...)
		sort.Slice(origins, func(i, j int) bool {
			if origins[i].Ref != origins[j].Ref {
				return origins[i].Ref < origins[j].Ref
			}
			return origins[i].SHA256 < origins[j].SHA256
		})
		entries = append(entries, exportManifestEntry{Path: f.Path, SHA256: f.SHA256, Origins: origins})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	for i := 1; i < len(entries); i++ {
		if entries[i].Path == entries[i-1].Path {
			return "", fmt.Errorf("%w: duplicate export member %q", ErrInvalidInput, entries[i].Path)
		}
	}
	raw, err := json.Marshal(exportManifestIdentity{VersionID: m.VersionID, Files: entries})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateExportBundleMembers re-validates every member path a bundle is
// about to package. The collection gate already validated these paths at
// publish time; this defensive pass refuses a corrupted store row (a
// traversal, an absolute path, a credential-shaped member) BEFORE any byte
// is packaged, and a single hostile member poisons the whole bundle.
// The three flat bundle documents are RESERVED names: a member colliding
// with one would duplicate a zip entry (unpackers disagree about the
// winner), so the manifest-to-package correspondence stays deterministic
// only if the collision is refused outright.
// reservedFold reports whether path equals any reserved name
// case-insensitively after Win32 trailing-dot/space normalization.
func reservedFold(path string, reserved map[string]bool) bool {
	normalized := strings.ToLower(strings.TrimRight(path, " ."))
	for name := range reserved {
		if strings.ToLower(name) == normalized {
			return true
		}
	}
	return false
}

func ValidateExportBundleMembers(files []File) error {
	reserved := map[string]bool{
		BundleManifestPath: true,
		BundleSourcesPath:  true,
		BundleBuildPath:    true,
	}
	for _, member := range files {
		if err := ValidateArtifactPath(member.Path); err != nil {
			return err
		}
		if first := strings.SplitN(member.Path, "/", 2)[0]; len(first) >= 2 && first[1] == ':' &&
			((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z')) {
			// Windows drive-letter path (both cases, any suffix — c:evil is
			// a drive-RELATIVE escape on Windows too): a zip-slip variant
			// refused BEFORE any byte is packaged.
			return fmt.Errorf("%w: bundle member %q uses a drive-letter path", ErrInvalidInput, member.Path)
		}
		// Per-SEGMENT drive check: a colon anywhere in any segment (foo/c:evil
		// puts the drive spec mid-path; c:evil is drive-relative) is refused —
		// Windows resolves both as drive paths or NTFS ADS streams.
		for _, segment := range strings.Split(member.Path, "/") {
			// ANY colon in ANY segment (foo/c:evil mid-path, c:evil
			// drive-relative, report:v2 NTFS ADS) is refused — the comment's
			// invariant, now the code's: Windows resolves each shape as a
			// drive path or an alternate-data-stream smuggle.
			if strings.Contains(segment, ":") {
				return fmt.Errorf("%w: bundle member %q segment %q contains a colon", ErrInvalidInput, member.Path, segment)
			}
		}
		// Reserved-name collision compares case-insensitively and after the
		// Win32 normalization (trailing dots/spaces stripped): on
		// case-insensitive filesystems a later member would otherwise
		// overwrite the authoritative fixed document on extraction.
		normalized := strings.ToLower(strings.TrimRight(member.Path, " ."))
		if reserved[normalized] || reservedFold(member.Path, reserved) {
			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
		}
		if reserved[member.Path] {
			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
		}
		if !ValidSHA256(member.SHA256) {
			return fmt.Errorf("%w: bundle member %q has a malformed digest", ErrInvalidInput, member.Path)
		}
	}
	return nil
}
