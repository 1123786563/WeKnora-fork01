package craft

// T12 (#132) module seam: the version-bound source bundle's export manifest
// and citation/source manifest are pure functions of IMMUTABLE facts only —
// the version's pinned file manifest, its build checks and the version's
// pinned evidence (T07). These tests pin:
//
//   - the export manifest lists exactly the version's own artifact members
//     with knowledge origins derived from the pinned evidence, and the
//     cross-tenant source is marked restricted while own-tenant is not;
//   - the canonical manifest digest is order-independent, covers origins
//     (a manifest differing only in origins hashes differently) and stays
//     byte-stable for identical immutable inputs;
//   - display titles ride only the citation/source manifest: a title change
//     never moves the export manifest digest (history stays stable even
//     when the current library rows are edited);
//   - bundle member paths are re-validated defensively: traversal, absolute
//     and credential-shaped members are refused before any byte is packaged.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func t12VersionFixture() Version {
	return Version{
		ID: "ver_" + strings.Repeat("1", 64), WorkspaceID: "ws-t12", RunID: "run-1", Kind: KindWeb,
		Files: []File{
			{Path: "index.html", Ref: "obj-index", SHA256: t12SHA("index"), MIME: "text/html", Bytes: 42},
			{Path: "citations.json", Ref: "obj-cit", SHA256: t12SHA("citations"), MIME: "application/json", Bytes: 7},
		},
		Checks: []Check{
			{Name: CheckBuild, Status: CheckPassed, Detail: "build exited 0"},
			{Name: CheckEntry, Status: CheckPassed, Detail: "entry index.html present"},
			{Name: CheckPreview, Status: CheckPassed, Detail: "controlled preview served this version's files"},
		},
	}
}

func t12EvidenceFixture() VersionEvidence {
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	return VersionEvidence{
		VersionID:     "ver_" + strings.Repeat("1", 64),
		RunID:         "run-1",
		RequestDigest: t12SHA("req"),
		PackageDigest: t12SHA("pkg"),
		AcquiredAt:    acquired,
		PinnedAt:      acquired.Add(time.Hour),
		Sources: []KnowledgeSourceRecord{
			{ID: "kc_" + strings.Repeat("a", 24), Ref: "craftkb://kb/kb-own/knowledge/k-own/chunk/c-own",
				Digest: t12SHA("own"), TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 32},
			{ID: "kc_" + strings.Repeat("b", 24), Ref: "craftkb://kb/kb-shared/knowledge/k-shared/chunk/c-shared",
				Digest: t12SHA("shared"), TenantID: 7, AcquiredAt: acquired.Add(time.Minute), ExcerptBytes: 32},
		},
	}
}

func TestCraftT12ExportManifestDerivation(t *testing.T) {
	version, evidence := t12VersionFixture(), t12EvidenceFixture()

	manifest, err := BuildExportManifest(version, evidence, 1)
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	if manifest.VersionID != version.ID {
		t.Fatalf("manifest binds %q, want the version %q", manifest.VersionID, version.ID)
	}
	if len(manifest.Files) != len(version.Files) {
		t.Fatalf("manifest lists %d files, want exactly the %d version members", len(manifest.Files), len(version.Files))
	}
	byPath := map[string]ExportFile{}
	for _, f := range manifest.Files {
		byPath[f.Path] = f
	}
	for _, member := range version.Files {
		entry, ok := byPath[member.Path]
		if !ok {
			t.Fatalf("version member %q missing from the export manifest", member.Path)
		}
		if entry.SHA256 != member.SHA256 {
			t.Fatalf("member %q carries digest %q, want the pinned %q", member.Path, entry.SHA256, member.SHA256)
		}
		if entry.Restricted {
			t.Fatalf("member %q is a generated artifact, not a restricted original; the bundle excludes restricted originals by default", member.Path)
		}
		if len(entry.Origins) != 2 {
			t.Fatalf("member %q carries %d origins, want the pinned evidence's 2 knowledge sources", member.Path, len(entry.Origins))
		}
		restrictedSeen := false
		for _, origin := range entry.Origins {
			if origin.Kind != ExportOriginKnowledge {
				t.Fatalf("origin %q has kind %q, want knowledge", origin.Ref, origin.Kind)
			}
			if !strings.HasPrefix(origin.Ref, "craftkb://") {
				t.Fatalf("origin ref %q is not a durable knowledge reference", origin.Ref)
			}
			restrictedSeen = restrictedSeen || origin.Restricted
		}
		if !restrictedSeen {
			t.Fatal("the cross-tenant (organization-shared) source must be marked restricted in its origins")
		}
	}
	digest, err := ExportManifestDigest(manifest)
	if err != nil {
		t.Fatalf("ExportManifestDigest: %v", err)
	}
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		t.Fatalf("manifest digest %q is not a lowercase sha-256 hex", digest)
	}
	if manifest.ManifestDigest != digest {
		t.Fatalf("manifest carries digest %q, want the canonical %q", manifest.ManifestDigest, digest)
	}
}

func TestCraftT12ExportManifestDigestCanonical(t *testing.T) {
	version, evidence := t12VersionFixture(), t12EvidenceFixture()
	manifest, err := BuildExportManifest(version, evidence, 1)
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	base, err := ExportManifestDigest(manifest)
	if err != nil {
		t.Fatalf("ExportManifestDigest: %v", err)
	}

	// Order independence: the same members listed in reverse hash identically.
	reordered := manifest
	reordered.Files = []ExportFile{manifest.Files[1], manifest.Files[0]}
	if other, err := ExportManifestDigest(reordered); err != nil || other != base {
		t.Fatalf("digest must be order-independent: %q vs %q (err %v)", other, base, err)
	}

	// Origins are hashed: dropping one origin changes the digest.
	trimmed := manifest
	trimmed.Files = append([]ExportFile(nil), manifest.Files...)
	trimmed.Files[0].Origins = trimmed.Files[0].Origins[:1]
	if other, err := ExportManifestDigest(trimmed); err != nil || other == base {
		t.Fatal("a manifest differing only in origins must hash differently")
	}

	// Immutable stability: identical inputs derive the identical digest.
	again, err := BuildExportManifest(version, evidence, 1)
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	if other, err := ExportManifestDigest(again); err != nil || other != base {
		t.Fatal("identical immutable inputs must derive the identical digest")
	}

	// Content change: a rewritten member changes the digest.
	edited := version
	edited.Files = append([]File(nil), version.Files...)
	edited.Files[0].SHA256 = t12SHA("index-rewritten")
	editedManifest, err := BuildExportManifest(edited, evidence, 1)
	if err != nil {
		t.Fatalf("BuildExportManifest: %v", err)
	}
	if other, err := ExportManifestDigest(editedManifest); err != nil || other == base {
		t.Fatal("changed member content must change the manifest digest")
	}
}

func TestCraftT12BundleCitationManifest(t *testing.T) {
	version, evidence := t12VersionFixture(), t12EvidenceFixture()
	titles := map[string]string{
		evidence.Sources[0].ID: "Own Region Sales",
		evidence.Sources[1].ID: "Shared Region Sales",
	}

	citation, err := BuildBundleCitationManifest(version, evidence, titles)
	if err != nil {
		t.Fatalf("BuildBundleCitationManifest: %v", err)
	}
	if citation.Schema != BundleSourceSchema {
		t.Fatalf("citation manifest schema %d, want %d", citation.Schema, BundleSourceSchema)
	}
	if citation.VersionID != version.ID || citation.RunID != evidence.RunID {
		t.Fatal("the citation manifest must bind the version and its producing run")
	}
	if citation.PinnedAt != evidence.PinnedAt {
		t.Fatal("the citation manifest carries the evidence's pin time")
	}
	if len(citation.Sources) != 2 {
		t.Fatalf("citation manifest lists %d sources, want the 2 pinned ones", len(citation.Sources))
	}
	for i, source := range citation.Sources {
		pinned := evidence.Sources[i]
		if source.CitationID != pinned.ID || source.Ref != pinned.Ref ||
			source.Digest != pinned.Digest || !source.AcquiredAt.Equal(pinned.AcquiredAt) {
			t.Fatalf("source %d does not carry the pinned facts verbatim: %+v", i, source)
		}
		if !strings.HasPrefix(source.Ref, "craftkb://") {
			t.Fatalf("source ref %q must stay the durable authenticated reference, never a URL", source.Ref)
		}
		if source.Title == "" {
			t.Fatalf("source %q carries no title", source.CitationID)
		}
	}

	// The authenticated reference carries no original material: encoding the
	// citation manifest must never embed excerpts or provider URLs.
	raw, err := json.Marshal(citation)
	if err != nil {
		t.Fatalf("marshal citation manifest: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "excerpt") || strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("citation manifest leaks original material or URLs: %s", body)
	}

	// Titles are display-only: they ride the citation manifest alone, and
	// BuildExportManifest does not even receive them — a later library edit
	// can never move the export manifest digest.
	retitled := map[string]string{evidence.Sources[0].ID: "Renamed Later", evidence.Sources[1].ID: ""}
	if _, err := BuildBundleCitationManifest(version, evidence, retitled); err != nil {
		t.Fatalf("BuildBundleCitationManifest with new titles: %v", err)
	}

	// An empty evidence pins an empty, honestly-empty citation manifest.
	empty := evidence
	empty.Sources = nil
	empty.Empty = true
	empty.AcquiredAt = time.Time{}
	emptyCitation, err := BuildBundleCitationManifest(version, empty, nil)
	if err != nil {
		t.Fatalf("BuildBundleCitationManifest(empty): %v", err)
	}
	if !emptyCitation.Empty || len(emptyCitation.Sources) != 0 {
		t.Fatal("empty evidence must pin an honestly empty citation manifest")
	}
}

func TestCraftT12BundleMemberGuards(t *testing.T) {
	good := []File{{Path: "index.html", SHA256: t12SHA("index"), Bytes: 1}}
	if err := ValidateExportBundleMembers(good); err != nil {
		t.Fatalf("canonical members must pass: %v", err)
	}
	hostile := []string{
		"../escape.txt", "a/../../b.txt", "/absolute.txt", "a\\b.txt",
		".env", "assets/.env", "secrets.txt", "",
	}
	for _, path := range hostile {
		if err := ValidateExportBundleMembers([]File{{Path: path, SHA256: t12SHA(path)}}); err == nil {
			t.Fatalf("hostile member path %q must be refused", path)
		}
	}
	// A single hostile member poisons the whole bundle.
	mixed := append([]File(nil), good...)
	mixed = append(mixed, File{Path: "../escape.txt", SHA256: t12SHA("x")})
	if err := ValidateExportBundleMembers(mixed); err == nil {
		t.Fatal("a bundle with one hostile member must be refused wholesale")
	}
}

// TestCraftT12ReservedBundleDocumentsAreRefused pins the OCR fix: the three
// flat bundle documents own their names — a member colliding with one would
// duplicate a zip entry whose unpack order is undefined, so validation
// refuses the collision outright.
func TestCraftT12ReservedBundleDocumentsAreRefused(t *testing.T) {
	for _, reserved := range []string{BundleManifestPath, BundleSourcesPath, BundleBuildPath} {
		members := []File{
			{Path: "index.html", SHA256: t12SHA("index"), Bytes: 1},
			{Path: reserved, SHA256: t12SHA(reserved), Bytes: 1},
		}
		if err := ValidateExportBundleMembers(members); err == nil {
			t.Fatalf("member colliding with the reserved bundle document %q must be refused", reserved)
		}
	}
}

// TestCraftT12EmptyEvidenceKeepsOriginsNonNil pins the OCR fix: EMPTY
// pinned evidence is a first-class validated state, and every member's
// Origins must stay a non-nil (empty) slice — the T00 frozen consumer
// contract rejects nil Origins, and the JSON must read [] not null.
func TestCraftT12EmptyEvidenceKeepsOriginsNonNil(t *testing.T) {
	pinned := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	evidence := VersionEvidence{
		VersionID: "ver_" + strings.Repeat("1", 64), RunID: "run-empty",
		RequestDigest: t12SHA("req"), PackageDigest: t12SHA("pkg"),
		Empty: true, PinnedAt: pinned,
	}
	version := Version{
		ID: evidence.VersionID, RunID: "run-empty", Kind: KindWeb,
		Files: []File{{Path: "index.html", SHA256: t12SHA("index"), Bytes: 1}},
	}
	manifest, err := BuildExportManifest(version, evidence, 1)
	if err != nil {
		t.Fatalf("empty evidence must build a manifest: %v", err)
	}
	for _, file := range manifest.Files {
		if file.Origins == nil {
			t.Fatalf("member %q carries a nil origins vector: the frozen consumer contract rejects it", file.Path)
		}
		if len(file.Origins) != 0 {
			t.Fatalf("member %q must carry zero origins for empty evidence", file.Path)
		}
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("an empty-evidence manifest must satisfy the frozen contract: %v", err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"origins":null`)) {
		t.Fatal("the wire manifest must serialize empty origins as [], never null")
	}
}

// t12SHA derives a stable lowercase sha-256 hex fixture.
func t12SHA(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

// Wrap-up OCR F27: a colon in ANY segment of a bundle member is refused —
// not only the drive-letter prefix shape. report:v2 smuggles an NTFS ADS
// entry through zip extraction on Windows exactly like c:evil does.
func TestCraftT12ExportBundleRefusesAnyColonSegment(t *testing.T) {
	for _, path := range []string{"report:v2", "docs/report:v2", "c:evil", "foo/c:evil"} {
		err := ValidateExportBundleMembers([]File{{Path: path, SHA256: t12SHA("x"), MIME: "text/html", Bytes: 4}})
		if err == nil {
			t.Fatalf("bundle member %q with a colon segment must be refused", path)
		}
	}
	if err := ValidateExportBundleMembers([]File{{Path: "clean/report-v2.html", SHA256: t12SHA("x"), MIME: "text/html", Bytes: 4}}); err != nil {
		t.Fatalf("a colon-free member stays acceptable: %v", err)
	}
}
