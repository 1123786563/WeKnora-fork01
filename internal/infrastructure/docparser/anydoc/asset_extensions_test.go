package anydoc

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// asset_links.rs::extension_for names the markdown links
// (`![alt](images/image-N<ext>)`) and extensionFor names the extracted
// assets; the image resolver joins the two by exact refMap lookup, so the
// tables must not drift apart (#3932). These tests hold the Go side of that
// contract without linking the Rust converter, hence no build tag.

func TestExtensionForFixedMapping(t *testing.T) {
	cases := []struct {
		mediaType string
		want      string
	}{
		// The mapped entries, in asset_links.rs order.
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"image/bmp", ".bmp"},
		{"image/tiff", ".tiff"},
		{"image/svg+xml", ".svg"},
		// Both sides lowercase the media type before the exact match.
		{"IMAGE/PNG", ".png"},
		{"Image/Jpg", ".jpg"},
		{"IMAGE/SVG+XML", ".svg"},
		// Everything else falls back to .bin, including the office drawing
		// formats behind #3932 and the vendor-prefixed variants the fixed
		// table deliberately does not know.
		{"image/emf", ".bin"},
		{"image/wmf", ".bin"},
		{"image/x-emf", ".bin"},
		{"image/x-wmf", ".bin"},
		{"image/x-ms-bmp", ".bin"},
		{"image/x-tiff", ".bin"},
		{"IMAGE/EMF", ".bin"},
		{"image/avif", ".bin"},
		{"video/mp4", ".bin"},
		{"", ".bin"},
		// Parameters are not stripped: the match is exact, as in Rust.
		{"image/png; charset=binary", ".bin"},
	}
	for _, tc := range cases {
		if got := extensionFor(tc.mediaType); got != tc.want {
			t.Errorf("extensionFor(%q) = %q, want %q", tc.mediaType, got, tc.want)
		}
	}

	// The exact failure of #3932: an EMF asset must be named image-1.bin so
	// it matches the Rust-written `![alt](images/image-1.bin)` link (the
	// reader stores assets under anydoc.ImageDir + Name).
	if name := fmt.Sprintf("image-1%s", extensionFor("image/emf")); name != "image-1.bin" {
		t.Errorf("EMF asset name = %q, want %q", name, "image-1.bin")
	}
}

// The mapping must be hermetic: whatever the MIME registry knows must not
// leak in, or asset names drift away from the Rust-written markdown links
// (#3932). text/html and application/pdf sit in Go's built-in table, so
// mime.ExtensionsByType resolves them on every machine with no help from
// /etc/mime.types, which makes them a deterministic witness that the
// registry is ignored. (On Debian with media-types installed the registry
// also resolves image/emf to .emf; the fixed table answers .bin regardless.)
func TestExtensionForNeverConsultsTheSystemMimeRegistry(t *testing.T) {
	for _, mediaType := range []string{"text/html", "application/pdf"} {
		if exts, err := mime.ExtensionsByType(mediaType); err != nil || len(exts) == 0 {
			t.Fatalf("precondition failed: mime.ExtensionsByType(%q) = %v, %v; "+
				"pick a type Go's built-in table resolves so the witness stays deterministic",
				mediaType, exts, err)
		}
		if got := extensionFor(mediaType); got != fallbackAssetExtension {
			t.Errorf("extensionFor(%q) = %q while the MIME registry offers extensions; "+
				"want %q — the fixed mapping must not consult the registry",
				mediaType, got, fallbackAssetExtension)
		}
	}
}

// Guardrail against single-sided edits: parse the match arms out of the
// vendored asset_links.rs and require them to agree with the Go table entry
// for entry, in both directions, fallback included. If this fails after a
// change on either side, mirror the change — never edit only one table.
func TestAssetExtensionsMirrorRustAssetLinks(t *testing.T) {
	rustExts, rustFallback := parseRustExtensionFor(t)

	if len(rustExts) == 0 {
		t.Fatal("parsed no match arms from asset_links.rs::extension_for; " +
			"if the Rust function was reshaped, update the parser in this test")
	}
	if rustFallback != fallbackAssetExtension {
		t.Errorf("asset_links.rs fallback = %q, want %q (the fallback must match too)",
			rustFallback, fallbackAssetExtension)
	}
	for mediaType, want := range assetExtensions {
		if got, ok := rustExts[mediaType]; !ok {
			t.Errorf("Go table maps %q to %s but asset_links.rs has no such arm — "+
				"remove the Go entry or add the Rust arm", mediaType, want)
		} else if got != want {
			t.Errorf("media type %q: Go table says %s, asset_links.rs says %s",
				mediaType, want, got)
		}
	}
	for mediaType, rustExt := range rustExts {
		if _, ok := assetExtensions[mediaType]; !ok {
			t.Errorf("asset_links.rs maps %q to %s but the Go table does not — "+
				"asset names will drift from the markdown links", mediaType, rustExt)
		}
	}
}

// parseRustExtensionFor extracts the `"media/type" => ".ext"` arms (and the
// `_` fallback) of extension_for from the vendored asset_links.rs. It reads
// the one-arm-per-line shape the file has used since it was vendored;
// anything it cannot understand fails the test rather than passing quietly.
func parseRustExtensionFor(t *testing.T) (extensions map[string]string, fallback string) {
	t.Helper()

	sourcePath := filepath.Join(repoRoot(t), "third_party", "anydoc-go", "src", "asset_links.rs")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v (if the vendored Rust moved, re-point this test "+
			"and re-sync asset_extensions.go by hand)", sourcePath, err)
	}

	extensions = map[string]string{}
	arm := regexp.MustCompile(`^\s*(.*?)\s*=>\s*("[^"]*")\s*,?\s*$`)
	quoted := regexp.MustCompile(`"([^"]*)"`)
	inside := false
	for _, line := range strings.Split(string(source), "\n") {
		switch {
		case strings.Contains(line, "fn extension_for("):
			inside = true
		case !inside, strings.HasPrefix(strings.TrimSpace(line), "//"):
			// Outside the function, or a comment line.
		case line == "}":
			// The function's closing brace at column zero.
			if fallback == "" {
				t.Fatal("asset_links.rs::extension_for has no `_ =>` fallback arm; " +
					"re-sync asset_extensions.go")
			}
			return extensions, fallback
		default:
			m := arm.FindStringSubmatch(line)
			if m == nil {
				continue // the `match` header or the match's closing brace
			}
			ext := quoted.FindStringSubmatch(m[2])[1]
			mediaTypes := quoted.FindAllStringSubmatch(m[1], -1)
			if len(mediaTypes) == 0 {
				if strings.TrimSpace(m[1]) != "_" {
					t.Fatalf("unrecognized arm in asset_links.rs: %q", line)
				}
				fallback = ext
				continue
			}
			for _, mediaType := range mediaTypes {
				extensions[mediaType[1]] = ext
			}
		}
	}
	t.Fatal("never found the end of extension_for in asset_links.rs")
	return nil, ""
}

// repoRoot walks up from this test file to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", thisFile)
		}
		dir = parent
	}
}
