package craft

import "testing"

// TestVersionKeyChangesWithContent pins the immutability contract of the
// version identity: the key tracks the manifest digest (so changed content can
// never reuse an old version) and is a pure function of its inputs (so a
// publish retry after a crash derives the exact same key again).
func TestVersionKeyChangesWithContent(t *testing.T) {
	a := VersionKey("w", "r", "sha1")
	if a == VersionKey("w", "r", "sha2") {
		t.Fatal("mutable version")
	}
	if a != VersionKey("w", "r", "sha1") {
		t.Fatal("publish retry changed key")
	}
}

// TestVersionKeyBindsWorkspaceAndRun pins the second half of the identity:
// the same content published for a different workspace or a different run is
// a different version, never a shared row.
func TestVersionKeyBindsWorkspaceAndRun(t *testing.T) {
	base := VersionKey("w1", "r1", "d")
	if base == VersionKey("w2", "r1", "d") {
		t.Fatal("workspace not bound into key")
	}
	if base == VersionKey("w1", "r2", "d") {
		t.Fatal("run not bound into key")
	}
	if len(base) != 64 {
		t.Fatalf("key must be 64 hex chars, got %d", len(base))
	}
	if id := VersionID("w", "r", "d"); id[:4] != VersionIDPrefix || len(id) != 4+64 {
		t.Fatalf("version id malformed: %q", id)
	}
}

// sha256 fixtures below are irrelevant to the rule under test; any distinct
// well-formed digests do.
const (
	digestA = "1111111111111111111111111111111111111111111111111111111111111111"
	digestB = "2222222222222222222222222222222222222222222222222222222222222222"
)

// TestManifestDigestTracksContentNotOrder pins the anti path+mtime rule: the
// manifest digest follows content, so listing order cannot change it and a
// file rewritten under an unchanged path (mtime never being an input at all)
// must produce a different digest.
func TestManifestDigestTracksContentNotOrder(t *testing.T) {
	a := File{Path: "index.html", SHA256: digestA, Bytes: 3}
	b := File{Path: "assets/app.js", SHA256: digestB, Bytes: 5}

	first, err := ManifestDigest([]File{a, b})
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := ManifestDigest([]File{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if first != reordered {
		t.Fatal("listing order changed the manifest digest")
	}

	rewritten := a
	rewritten.SHA256 = digestB
	second, err := ManifestDigest([]File{rewritten, b})
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("rewritten content kept the old version identity")
	}

	// Path participates too: same bytes under a new path is a new version.
	moved := File{Path: "moved.html", SHA256: digestA, Bytes: 3}
	movedDigest, err := ManifestDigest([]File{moved, b})
	if err != nil {
		t.Fatal(err)
	}
	if movedDigest == first {
		t.Fatal("path move kept the old version identity")
	}
}

// TestManifestDigestRejectsBrokenManifests pins the manifest invariants:
// duplicate paths, traversal paths and malformed digests never reach a
// version identity.
func TestManifestDigestRejectsBrokenManifests(t *testing.T) {
	good := File{Path: "index.html", SHA256: digestA, Bytes: 3}
	if _, err := ManifestDigest([]File{good, good}); err == nil {
		t.Fatal("duplicate path accepted")
	}
	bad := good
	bad.Path = "../escape.html"
	if _, err := ManifestDigest([]File{bad}); err == nil {
		t.Fatal("traversal path accepted")
	}
	bad = good
	bad.SHA256 = "nothex"
	if _, err := ManifestDigest([]File{bad}); err == nil {
		t.Fatal("malformed digest accepted")
	}
	if _, err := ManifestDigest(nil); err != nil {
		t.Fatalf("empty manifest must digest: %v", err)
	}
}

// TestArtifactRelativePathCannotEscape pins the output-dir containment rule:
// only canonical paths strictly inside the output directory map onto
// version-relative paths.
func TestArtifactRelativePathCannotEscape(t *testing.T) {
	const dir = "/workspace/output"
	rel, err := ArtifactRelativePath(dir, dir+"/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if rel != "index.html" {
		t.Fatalf("got %q", rel)
	}
	if rel, err = ArtifactRelativePath(dir, dir+"/assets/app.js"); err != nil || rel != "assets/app.js" {
		t.Fatalf("nested path: %q %v", rel, err)
	}
	for _, listed := range []string{
		"/workspace/output/../secret",
		"/workspace/secret",
		"/etc/passwd",
		dir,
		dir + "//index.html",
		"index.html",
		"",
	} {
		if _, err := ArtifactRelativePath(dir, listed); err == nil {
			t.Fatalf("accepted %q", listed)
		}
	}
	if _, err := ArtifactRelativePath("", dir+"/index.html"); err == nil {
		t.Fatal("accepted empty output dir")
	}
}

// TestValidateArtifactPathRejectsHostileNames pins the per-path rules:
// canonical relative paths pass; traversal, absolute paths, backslashes and
// credential-looking names are refused before any byte is read.
func TestValidateArtifactPathRejectsHostileNames(t *testing.T) {
	for _, ok := range []string{"index.html", "assets/app.js", "styles/main.css", "data/report.csv"} {
		if err := ValidateArtifactPath(ok); err != nil {
			t.Fatalf("rejected canonical %q: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "/abs.html", "../secret", "a/../b", "a//b", "a\\b",
		".env", "assets/.env", "server.pem", "tls/key.key", "id_rsa", "id_ed25519",
		"credentials.json", "config/secrets.yaml", "cert.pfx", "bundle.p12",
	} {
		if err := ValidateArtifactPath(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

// TestBuildChecksNotRunByDefault pins the verification-fact rules: unobserved
// facts are not_run, a claimed success without the entry file fails the entry
// check, and the preview check can only pass when a preview actually ran —
// W02 owns that fact, so W01 collections never record preview passed.
func TestBuildChecksNotRunByDefault(t *testing.T) {
	files := []File{{Path: "index.html", SHA256: digestA, Bytes: 3}}

	checks := BuildChecks(KindWeb, files, ArtifactEvidence{})
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}
	byName := map[string]Check{}
	for _, c := range checks {
		byName[c.Name] = c
	}
	if byName[CheckBuild].Status != CheckNotRun {
		t.Fatalf("build without evidence must be not_run, got %q", byName[CheckBuild].Status)
	}
	if byName[CheckEntry].Status != CheckPassed {
		t.Fatalf("present entry must pass, got %q", byName[CheckEntry].Status)
	}
	if byName[CheckPreview].Status != CheckNotRun {
		t.Fatalf("unverified preview must be not_run, got %q", byName[CheckPreview].Status)
	}

	// Claimed success but the entry file is missing: a failed fact, never a
	// missing one.
	checks = BuildChecks(KindWeb, nil, ArtifactEvidence{ClaimedSuccess: true})
	byName = map[string]Check{}
	for _, c := range checks {
		byName[c.Name] = c
	}
	if byName[CheckEntry].Status != CheckFailed {
		t.Fatalf("claimed success without files must fail entry, got %q", byName[CheckEntry].Status)
	}

	// Build exit codes report exactly what was observed: a build that ran and
	// exited 0 passes, a non-zero exit fails, and only an unobserved build is
	// not_run.
	checks = BuildChecks(KindWeb, files, ArtifactEvidence{BuildRan: true})
	if checks[0].Status != CheckPassed {
		t.Fatalf("observed zero build exit must pass, got %q", checks[0].Status)
	}
	checks = BuildChecks(KindWeb, files, ArtifactEvidence{BuildRan: true, BuildExitCode: 2})
	if checks[0].Status != CheckFailed {
		t.Fatalf("explicit non-zero exit must fail, got %q", checks[0].Status)
	}
	checks = BuildChecks(KindWeb, files, ArtifactEvidence{BuildRan: true, BuildExitCode: 0})
	if checks[0].Status != CheckPassed {
		t.Fatalf("zero exit must pass, got %q", checks[0].Status)
	}

	// Preview passes only with observed preview evidence.
	checks = BuildChecks(KindWeb, files, ArtifactEvidence{PreviewRan: true, PreviewPassed: true})
	byName = map[string]Check{}
	for _, c := range checks {
		byName[c.Name] = c
	}
	if byName[CheckPreview].Status != CheckPassed {
		t.Fatalf("observed preview pass must pass, got %q", byName[CheckPreview].Status)
	}
	checks = BuildChecks(KindWeb, files, ArtifactEvidence{PreviewRan: true})
	if checks[2].Status != CheckFailed {
		t.Fatalf("observed preview failure must fail, got %q", checks[2].Status)
	}

	// Kinds without a defined entry cannot have their entry judged yet.
	// (D01 wiring note: "document" gained its entry in report.docx, so the
	// undefined-kind probe uses a kind outside the closed set.)
	checks = BuildChecks("diagram", files, ArtifactEvidence{})
	if checks[1].Status != CheckNotRun {
		t.Fatalf("undefined kind entry must be not_run, got %q", checks[1].Status)
	}
}
