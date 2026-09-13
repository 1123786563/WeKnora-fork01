package craft

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// VersionIDPrefix marks every persisted version identity. The remainder of the
// id is the VersionKey hex digest, so a well-formed version id is exactly
// "ver_" + 64 lowercase hex characters.
const VersionIDPrefix = "ver_"

// VersionKey derives the immutable version identity from the workspace, the
// producing run and the manifest digest (W01: 不可变产物与验证事实). It is a
// pure function of its inputs: changed content changes the key, and a publish
// retry after a crash derives the exact same key again.
func VersionKey(workspaceID, runID, digest string) string {
	raw, _ := json.Marshal([]string{workspaceID, runID, digest})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// VersionID is the durable row identity of one version: the version key in
// its persisted form. Publish derives it server-side; a caller-supplied id
// must equal it exactly.
func VersionID(workspaceID, runID, digest string) string {
	return VersionIDPrefix + VersionKey(workspaceID, runID, digest)
}

// VersionStore persists immutable artifact versions. Publishing is
// idempotent by logical identity (workspace, run, manifest digest): the same
// content under the same run always answers the same version, and a version
// that was once published never changes its files or checks afterwards.
// Later modifications of the workspace produce new versions; old downloads
// keep resolving to the objects their manifest pinned.
type VersionStore interface {
	// Publish records one immutable version for the scope's workspace after
	// every file object it references was already durably uploaded. Repeating
	// the identical publish returns the stored version with the same id.
	Publish(ctx context.Context, scope Scope, v Version) (Version, error)
	// List returns the scope's workspace versions, newest first.
	List(ctx context.Context, scope Scope) ([]Version, error)
	// Get returns one published version in the requesting scope.
	Get(ctx context.Context, scope Scope, id string) (Version, error)
}

// Verification check names recorded on a version. Each name is one
// independently reported fact; consumers (W02 preview, D01-D03 delivery)
// match on these exact strings.
const (
	// CheckBuild reports the sub-execution's build step exit code.
	CheckBuild = "build"
	// CheckEntry reports whether the kind's entry file exists in the version.
	CheckEntry = "entry"
	// CheckPreview reports the controlled page preview verdict (W02).
	CheckPreview = "preview"
)

// Check statuses. A check that was not executed for this version is not_run —
// it is never silently reported as passed.
const (
	CheckPassed = "passed"
	CheckFailed = "failed"
	CheckNotRun = "not_run"
)

// KindWeb is the first supported artwork kind; its deliverable entry is the
// generated index.html. The D01-D03 kinds declare their entries alongside
// their generators: the entry is always the kind's DELIVERABLE (the file a
// user downloads), never an intermediate — the document's deliverable is
// the DOCX export (report.md is the editable source riding in the same
// version), the spreadsheet's is the recalculated workbook and the deck's
// is the PPTX.
const KindWeb = "web"

// EntryPath returns the version-relative file the kind's deliverable must
// contain for the entry check to pass. The path doubles as the controlled
// preview's ticket entry (W02 Issue).
func EntryPath(kind string) (string, bool) {
	switch kind {
	case KindWeb:
		return "index.html", true
	case KindDocument:
		return DocumentDOCXPath, true
	case KindSpreadsheet:
		return SpreadsheetXLSXPath, true
	case KindSlides:
		return SlidesPPTXPath, true
	default:
		return "", false
	}
}

// MaxArtifactPathBytes bounds one version-relative artifact path, matching
// the craft_version_files.path column.
const MaxArtifactPathBytes = 512

// credentialArtifactNames are exact basenames that never enter a version:
// account or deployment credentials have no place in a user-downloadable
// artifact bundle.
var credentialArtifactNames = map[string]struct{}{
	".env": {}, ".netrc": {}, ".npmrc": {}, ".htpasswd": {},
	"id_rsa": {}, "id_dsa": {}, "id_ecdsa": {}, "id_ed25519": {},
}

// credentialArtifactSuffixes mark key material regardless of basename.
var credentialArtifactSuffixes = []string{".pem", ".key", ".pfx", ".p12"}

// isCredentialArtifact reports whether the version-relative path names (or
// contains a path element naming) credential material. The whole path is
// examined so "assets/.env" is refused just like ".env".
func isCredentialArtifact(rel string) bool {
	for _, elem := range strings.Split(strings.ToLower(rel), "/") {
		if _, ok := credentialArtifactNames[elem]; ok {
			return true
		}
		// .env.local / .env.production / .env.development.local … carry the
		// same real secrets as .env; only the explicitly share-safe
		// .env.example template is exempt. (W01 review hardening, landed in
		// W02.)
		if strings.HasPrefix(elem, ".env.") && elem != ".env.example" {
			return true
		}
		for _, suffix := range credentialArtifactSuffixes {
			if strings.HasSuffix(elem, suffix) {
				return true
			}
		}
		if strings.HasPrefix(elem, "credentials") || strings.HasPrefix(elem, "secrets") {
			return true
		}
	}
	return false
}

// ValidateArtifactPath accepts exactly canonical, output-relative artifact
// paths: non-empty, forward-slash separated, no absolute prefix, no dot or
// dot-dot element (traversal), no backslash or NUL, within the length bound,
// and not naming credential material. Everything that could escape the
// version's own directory tree is rejected before any byte is read.
func ValidateArtifactPath(rel string) error {
	if rel == "" || len(rel) > MaxArtifactPathBytes {
		return fmt.Errorf("%w: artifact path %q", ErrInvalidInput, rel)
	}
	if strings.ContainsAny(rel, "\\\x00") {
		return fmt.Errorf("%w: artifact path %q must use forward slashes only", ErrInvalidInput, rel)
	}
	if strings.HasPrefix(rel, "/") {
		return fmt.Errorf("%w: artifact path %q must be output-relative", ErrInvalidInput, rel)
	}
	for _, elem := range strings.Split(rel, "/") {
		if elem == "" || elem == "." || elem == ".." {
			return fmt.Errorf("%w: artifact path %q is not canonical", ErrInvalidInput, rel)
		}
	}
	if isCredentialArtifact(rel) {
		return fmt.Errorf("%w: artifact path %q looks like a credentials file", ErrInvalidInput, rel)
	}
	return nil
}

// ArtifactRelativePath maps one listed sandbox path onto its version-relative
// form. The listed path must already be canonical and must sit strictly
// inside the output directory — a traversal, an absolute escape or the output
// directory itself is an error, never a silently dropped file.
func ArtifactRelativePath(outputDir, listed string) (string, error) {
	dir := path.Clean(strings.TrimSpace(outputDir))
	raw := strings.TrimSpace(listed)
	if dir == "" || dir == "." || dir == "/" {
		return "", fmt.Errorf("%w: artifact output dir %q", ErrInvalidInput, outputDir)
	}
	if raw == "" {
		return "", fmt.Errorf("%w: empty artifact path", ErrInvalidInput)
	}
	if p := path.Clean(raw); p != raw {
		return "", fmt.Errorf("%w: artifact path %q is not canonical", ErrInvalidInput, listed)
	}
	prefix := dir + "/"
	if !strings.HasPrefix(raw, prefix) {
		return "", fmt.Errorf("%w: artifact path %q escapes output dir %q", ErrInvalidInput, listed, outputDir)
	}
	rel := strings.TrimPrefix(raw, prefix)
	if rel == "" {
		return "", fmt.Errorf("%w: artifact path %q is the output dir itself", ErrInvalidInput, listed)
	}
	if err := ValidateArtifactPath(rel); err != nil {
		return "", err
	}
	return rel, nil
}

// manifestEntry is the canonical per-file record hashed into a manifest
// digest. MIME and storage ref are excluded: they are presentation of the
// same bytes, not identity.
type manifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// ManifestDigest derives the content digest of one version's file manifest:
// entries sorted by path so listing order cannot change identity, each entry
// contributing path, content digest and size. Modification times are
// deliberately absent — identity follows content, so a file rewritten under
// an unchanged path and mtime still yields a different digest and can never
// hide inside an already published version.
func ManifestDigest(files []File) (string, error) {
	entries := make([]manifestEntry, 0, len(files))
	for _, f := range files {
		if err := ValidateArtifactPath(f.Path); err != nil {
			return "", err
		}
		if !ValidSHA256(f.SHA256) {
			return "", fmt.Errorf("%w: artifact %q has malformed digest %q", ErrInvalidInput, f.Path, f.SHA256)
		}
		if f.Bytes < 0 {
			return "", fmt.Errorf("%w: artifact %q has negative size", ErrInvalidInput, f.Path)
		}
		entries = append(entries, manifestEntry{Path: f.Path, SHA256: f.SHA256, Bytes: f.Bytes})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	for i := 1; i < len(entries); i++ {
		if entries[i].Path == entries[i-1].Path {
			return "", fmt.Errorf("%w: duplicate artifact path %q", ErrInvalidInput, entries[i].Path)
		}
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ArtifactEvidence carries the externally observed verification facts for one
// collection round. The collector verifies the entry file against its own
// manifest, but the build exit code comes from the sub-execution's build
// step and the page preview verdict belongs to W02; a fact that was not
// observed stays unreported and maps to not_run — it is never fabricated as
// passed.
type ArtifactEvidence struct {
	// ClaimedSuccess reports that the sub-execution's own output claimed the
	// deliverable was produced.
	ClaimedSuccess bool
	// BuildRan reports that a build step was observed; BuildExitCode is its
	// exit code.
	BuildRan      bool
	BuildExitCode int
	// PreviewRan reports that W02's controlled preview verification ran for
	// this version; PreviewPassed is its verdict.
	PreviewRan    bool
	PreviewPassed bool
}

// BuildChecks reports the three verification facts of one version as
// independent checks: build exit code, entry existence and page preview.
// Unobserved facts are not_run; a claimed success without the entry file is
// a failed entry check, not a missing one; the preview check can only pass
// when the preview verification actually ran (W02) — until then it stays
// not_run.
func BuildChecks(kind string, files []File, evidence ArtifactEvidence) []Check {
	checks := make([]Check, 0, 3)

	switch {
	case !evidence.BuildRan:
		checks = append(checks, Check{Name: CheckBuild, Status: CheckNotRun,
			Detail: "no build step was observed for this run"})
	case evidence.BuildExitCode == 0:
		checks = append(checks, Check{Name: CheckBuild, Status: CheckPassed,
			Detail: "build exited 0"})
	default:
		checks = append(checks, Check{Name: CheckBuild, Status: CheckFailed,
			Detail: fmt.Sprintf("build exited %d", evidence.BuildExitCode)})
	}

	entry, defined := EntryPath(kind)
	switch {
	case !defined:
		checks = append(checks, Check{Name: CheckEntry, Status: CheckNotRun,
			Detail: fmt.Sprintf("kind %q has no defined entry file yet", kind)})
	case hasArtifactFile(files, entry):
		checks = append(checks, Check{Name: CheckEntry, Status: CheckPassed,
			Detail: fmt.Sprintf("entry %s present", entry)})
	case evidence.ClaimedSuccess:
		checks = append(checks, Check{Name: CheckEntry, Status: CheckFailed,
			Detail: fmt.Sprintf("sub-execution claimed success but entry %s is missing", entry)})
	default:
		checks = append(checks, Check{Name: CheckEntry, Status: CheckFailed,
			Detail: fmt.Sprintf("entry %s was not produced", entry)})
	}

	switch {
	case !evidence.PreviewRan:
		checks = append(checks, Check{Name: CheckPreview, Status: CheckNotRun,
			Detail: "page preview not verified for this version"})
	case evidence.PreviewPassed:
		checks = append(checks, Check{Name: CheckPreview, Status: CheckPassed,
			Detail: "controlled preview served this version's files"})
	default:
		checks = append(checks, Check{Name: CheckPreview, Status: CheckFailed,
			Detail: "controlled preview verification failed"})
	}
	return checks
}

// hasArtifactFile reports whether the exact version-relative path is part of
// the manifest.
func hasArtifactFile(files []File, rel string) bool {
	for _, f := range files {
		if f.Path == rel {
			return true
		}
	}
	return false
}
