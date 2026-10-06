package anydoc

import "strings"

// assetExtensions is the fixed media-type → file-extension table for embedded
// image assets.
//
// It must stay character-for-character identical to
// third_party/anydoc-go/src/asset_links.rs::extension_for: the Rust side
// writes the markdown link `![alt](images/image-N<ext>)` while this side
// names the extracted asset `image-N<ext>` (collectAssets), and the image
// resolver joins the two by exact refMap lookup (markdown_image_scanner.go
// feeds image_resolver.go) — a one-character drift between the tables and the
// image bytes are silently dropped (#3932). The table is therefore hermetic:
// mime.ExtensionsByType drifts with the host's /etc/mime.types (Debian's
// media-types package maps image/emf to .emf while Rust says .bin), so it
// must never be consulted here. Every type the table does not know —
// EMF/WMF drawings included — falls back to fallbackAssetExtension on both
// sides.
var assetExtensions = map[string]string{
	"image/jpeg":    ".jpg",
	"image/jpg":     ".jpg",
	"image/png":     ".png",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/bmp":     ".bmp",
	"image/tiff":    ".tiff",
	"image/svg+xml": ".svg",
}

// fallbackAssetExtension is what both this table and
// asset_links.rs::extension_for return for every media type they do not map.
const fallbackAssetExtension = ".bin"

// extensionFor returns the extension asset names are built from, mirroring
// asset_links.rs::extension_for entry for entry: the media type is lowercased
// (Rust: to_ascii_lowercase) before the exact-match lookup, and unmapped
// types get .bin. The system MIME registry is never consulted.
func extensionFor(mediaType string) string {
	if ext, ok := assetExtensions[strings.ToLower(mediaType)]; ok {
		return ext
	}
	return fallbackAssetExtension
}
