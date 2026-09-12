package craft

import (
	"strings"
	"testing"
)

// TestPreviewRejectsMainOrigin is the W02 brief's Step 1 acceptance test,
// verbatim: the controlled preview may never share the app's origin, never
// accept a non-https/unparseable preview origin, and must accept a distinct
// isolated https origin.
func TestPreviewRejectsMainOrigin(t *testing.T) {
	if PreviewOriginAllowed("https://app.test", "https://app.test") {
		t.Fatal("same origin")
	}
	if PreviewOriginAllowed("https://app.test", "javascript:alert(1)") {
		t.Fatal("unsafe URL")
	}
	if !PreviewOriginAllowed("https://app.test", "https://preview.test") {
		t.Fatal("isolated origin rejected")
	}
}

// TestPreviewOriginAllowedHostileOrigins pins the rest of the origin rule:
// userinfo, plain http, an origin carrying a path/query/fragment, and missing
// hosts are all rejected; a port-distinct origin is a distinct origin.
func TestPreviewOriginAllowedHostileOrigins(t *testing.T) {
	reject := [][2]string{
		{"", ""},
		{"https://app.test", ""},
		{"", "https://preview.test"},
		{"https://app.test", "http://preview.test"},
		{"https://app.test", "https://user:pw@preview.test"},
		{"https://app.test", "https://preview.test/"},
		{"https://app.test", "https://preview.test/x"},
		{"https://app.test", "https://preview.test?q=1"},
		{"https://app.test", "https://preview.test#f"},
		{"https://app.test", "ftp://preview.test"},
	}
	for _, pair := range reject {
		if PreviewOriginAllowed(pair[0], pair[1]) {
			t.Fatalf("PreviewOriginAllowed(%q, %q) = true, want false", pair[0], pair[1])
		}
	}
	// A different port on the same host IS a different origin and is allowed.
	if !PreviewOriginAllowed("https://app.test:8443", "https://app.test") {
		t.Fatal("port-distinct origin rejected")
	}
}

// TestPreviewTokenMintAndDigest pins the token scheme: 256 bits of entropy,
// URL-safe encoding, and a digest that is not the token itself.
func TestPreviewTokenMintAndDigest(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		token, digest, err := NewPreviewToken()
		if err != nil {
			t.Fatalf("NewPreviewToken: %v", err)
		}
		if len(token) != 43 { // 32 bytes base64.RawURLEncoding
			t.Fatalf("token length = %d, want 43", len(token))
		}
		if strings.ContainsAny(token, "+/=") {
			t.Fatalf("token %q is not raw-url-safe", token)
		}
		if seen[token] {
			t.Fatalf("token repeated after %d draws", i)
		}
		seen[token] = true
		if digest == token || len(digest) != 64 {
			t.Fatalf("digest %q is not a hex digest of the token", digest)
		}
		if again := PreviewTokenDigest(token); again != digest {
			t.Fatalf("digest not stable: %q vs %q", again, digest)
		}
	}
}

// TestIsVersionID pins the version id shape accepted by preview issuance.
func TestIsVersionID(t *testing.T) {
	good := VersionID("ws", "run", strings.Repeat("a", 64))
	if !IsVersionID(good) {
		t.Fatalf("IsVersionID(%q) = false, want true", good)
	}
	bad := []string{"", "ver_", "ver_abc", strings.Repeat("a", 64), "ver_" + strings.Repeat("A", 64), "vx_" + strings.Repeat("a", 64)}
	for _, id := range bad {
		if IsVersionID(id) {
			t.Fatalf("IsVersionID(%q) = true, want false", id)
		}
	}
}

// TestPreviewableKind pins the closed loop: only static web artifacts are
// previewable; anything needing a backing application is refused.
func TestPreviewableKind(t *testing.T) {
	if !PreviewableKind(KindWeb) {
		t.Fatal("web kind must be previewable")
	}
	for _, kind := range []string{"", "app", "api", "webapp "} {
		if PreviewableKind(kind) {
			t.Fatalf("PreviewableKind(%q) = true, want false", kind)
		}
	}
}

// TestValidatePreviewRequestPath pins the double-decode and traversal
// defenses on the serving side.
func TestValidatePreviewRequestPath(t *testing.T) {
	for _, ok := range []string{"index.html", "/index.html", "css/app.css", "assets/img/logo.png", "a/b/c.js"} {
		if got, err := ValidatePreviewRequestPath(ok); err != nil || got != strings.TrimPrefix(ok, "/") {
			t.Fatalf("ValidatePreviewRequestPath(%q) = %q, %v; want accepted", ok, got, err)
		}
	}
	for _, bad := range []string{
		"", "/", "..", "../secret", "a/../../etc/passwd", "//etc/passwd", // absolute-path smuggle past the wildcard
		"a//b", "a/./b", "a\\b", "a%2e%2e%2fb", "%2e%2e", "a%00b", // second-decode smuggles
		".env", "cfg/.env.production", "keys/server.pem",
		strings.Repeat("x", MaxArtifactPathBytes+1),
	} {
		if _, err := ValidatePreviewRequestPath(bad); err == nil {
			t.Fatalf("ValidatePreviewRequestPath(%q) accepted, want rejected", bad)
		}
	}
}

// TestValidateArtifactPathRejectsEnvVariants pins the W01-review hardening:
// Vite/Next style .env.<env> files carry real secrets and never enter a
// version; only the share-safe .env.example template is exempt.
func TestValidateArtifactPathRejectsEnvVariants(t *testing.T) {
	for _, rel := range []string{".env.local", ".env.production", ".env.development", ".env.development.local", "config/.env.production"} {
		if err := ValidateArtifactPath(rel); err == nil {
			t.Fatalf("ValidateArtifactPath(%q) accepted, want rejected", rel)
		}
	}
	if err := ValidateArtifactPath(".env.example"); err != nil {
		t.Fatalf("ValidateArtifactPath(.env.example) = %v, want accepted", err)
	}
}

// TestPreviewCheckFromEvidenceMatchesBuildChecks pins the consistency the
// update channel depends on: the preview check written after verification is
// byte-identical to the one W01's collector derives from the same evidence,
// so an idempotent re-collect still adopts the stored version row.
func TestPreviewCheckFromEvidenceMatchesBuildChecks(t *testing.T) {
	files := []File{{Path: "index.html", SHA256: strings.Repeat("0", 64), Bytes: 3}}
	for _, tc := range []struct {
		ran, passed bool
	}{
		{false, false},
		{true, false},
		{true, true},
		{false, true}, // nonsense input: must still render as not-run, never pass
	} {
		want := BuildChecks(KindWeb, files, ArtifactEvidence{PreviewRan: tc.ran, PreviewPassed: tc.passed})
		var expected Check
		for _, c := range want {
			if c.Name == CheckPreview {
				expected = c
			}
		}
		got := PreviewCheckFromEvidence(tc.ran, tc.passed)
		if got != expected {
			t.Fatalf("PreviewCheckFromEvidence(%v, %v) = %+v, want %+v", tc.ran, tc.passed, got, expected)
		}
	}
}
