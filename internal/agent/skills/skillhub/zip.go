package skillhub

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ZipFiles is the parsed package: workspace-relative member paths with their
// contents. It is a type alias so callers can build the literal slice type
// directly when constructing fixtures or expected values.
type ZipFiles = []struct {
	Name    string
	Content []byte
}

// Safety limits, ports of the Octop constants.
const (
	maxZipEntries            = 2000
	maxZipUncompressedBytes  = 64 << 20 // 64 MiB across all members
	maxZipCompressionRatio   = 100
	ratioCheckThresholdBytes = 1024 * 1024 // ratio only enforced above 1 MiB
)

// Unix file-type bits from the zip external attributes (mode is stored in
// the high 16 bits), used exactly the way Python's stat module uses them.
const (
	zipSIFMT  uint32 = 0xF000
	zipSIFREG uint32 = 0x8000
	zipSIFDIR uint32 = 0x4000
	zipSIFLNK uint32 = 0xA000
)

// zipEncryptedFlag is the general-purpose bit 0: traditional PKWARE encryption.
const zipEncryptedFlag uint16 = 0x1

// ParsePackage validates a SkillHub package zip and returns its file
// payloads with workspace-relative names. It is the Go port of Octop's
// parse_skillhub_package and enforces, in order:
//
//  1. the payload is a readable zip archive,
//  2. at most 2000 entries,
//  3. per entry: a safe path (no absolute/`..`/NUL/backslash-traversal/
//     drive-letter/dot-only names, duplicates after normalization),
//     a regular file, directory or mode-less entry (no symlinks or other
//     special types), no encrypted entries,
//  4. total declared uncompressed size at most 64 MiB and per-entry
//     compression ratio at most 100 for entries above 1 MiB,
//  5. per entry, on read: streamed size never exceeds the declared size
//     nor the global cap, and the final size matches the declaration,
//  6. every member is valid UTF-8 (WeKnora deviation from Octop, which
//     replaces invalid sequences in non-manifest text files — see the
//     package port notes),
//  7. a single redundant top-level wrapper directory is stripped when the
//     root SKILL.md only exists under it,
//  8. a root SKILL.md is required.
func ParsePackage(payload []byte) (ZipFiles, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("%w: SkillHub package is not a valid ZIP archive: %v", ErrPackage, err)
	}

	validated, err := validateZip(reader)
	if err != nil {
		return nil, err
	}

	var files ZipFiles
	for _, member := range validated {
		// Directory members contribute no payload (Python: member.is_dir(),
		// i.e. a trailing slash in the raw name).
		if strings.HasSuffix(member.file.Name, "/") {
			continue
		}
		content, err := readZipMember(member)
		if err != nil {
			return nil, err
		}
		// Deviation from Octop: instead of coercing invalid UTF-8 in text
		// members (coerce_utf8_text_bytes), every member must already be
		// valid UTF-8. Stricter, simpler, and the error names the member.
		if !utf8.Valid(content) {
			return nil, fmt.Errorf("%w: package entry %q is not valid UTF-8", ErrPackage, member.clean)
		}
		files = append(files, struct {
			Name    string
			Content []byte
		}{Name: member.clean, Content: content})
	}

	files = stripWrapperDirectory(files)

	if len(files) == 0 {
		return nil, fmt.Errorf("%w: SkillHub package does not contain a root SKILL.md", ErrPackage)
	}
	hasRoot := false
	for _, f := range files {
		if f.Name == "SKILL.md" {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		return nil, fmt.Errorf("%w: SkillHub package does not contain a root SKILL.md", ErrPackage)
	}
	return files, nil
}

// validatedMember pairs a zip entry with its normalized clean path.
type validatedMember struct {
	file  *zip.File
	clean string
}

// validateZip is the port of _validate_zip: every header-level rule fires
// before a single member is decompressed.
func validateZip(reader *zip.Reader) ([]validatedMember, error) {
	if len(reader.File) > maxZipEntries {
		return nil, fmt.Errorf("%w: SkillHub package has too many entries", ErrPackageTooLarge)
	}

	var totalUncompressed uint64
	seen := make(map[string]bool, len(reader.File))
	validated := make([]validatedMember, 0, len(reader.File))
	for _, file := range reader.File {
		header := &file.FileHeader

		clean, err := safeZipPath(header.Name)
		if err != nil {
			return nil, err
		}
		if seen[clean] {
			return nil, fmt.Errorf("%w: duplicate zip path entry: %s", ErrPackage, header.Name)
		}
		seen[clean] = true

		// File type from the unix mode bits in the high half of
		// external_attr. Allowed: none (0), regular, directory. Symlinks
		// and every other special type are rejected outright.
		mode := header.ExternalAttrs >> 16
		fileType := mode & zipSIFMT
		if fileType == zipSIFLNK || (fileType != 0 && fileType != zipSIFREG && fileType != zipSIFDIR) {
			return nil, fmt.Errorf("%w: unsupported zip entry type: %s", ErrPackage, header.Name)
		}
		if header.Flags&zipEncryptedFlag != 0 {
			return nil, fmt.Errorf("%w: encrypted zip entry is not supported: %s", ErrPackage, header.Name)
		}

		totalUncompressed += header.UncompressedSize64
		if totalUncompressed > maxZipUncompressedBytes {
			return nil, fmt.Errorf("%w: SkillHub package uncompressed size exceeds %d MB",
				ErrPackageTooLarge, maxZipUncompressedBytes>>20)
		}
		// Zip-bomb guard: a member above 1 MiB whose declared uncompressed
		// size dwarfs its compressed size is rejected up front.
		if compressed := header.CompressedSize64; compressed > 0 &&
			header.UncompressedSize64 > ratioCheckThresholdBytes &&
			float64(header.UncompressedSize64)/float64(compressed) > maxZipCompressionRatio {
			return nil, fmt.Errorf("%w: zip compression ratio is too high: %s", ErrPackageTooLarge, header.Name)
		}

		validated = append(validated, validatedMember{file: file, clean: clean})
	}
	return validated, nil
}

// safeZipPath is the port of _safe_zip_path. Backslashes are normalized to
// forward slashes first (so "a\..\b" is traversal, not a filename), then the
// path must be relative, free of ".." segments and NUL bytes, and must not
// start with a Windows drive-letter segment. Dot and empty segments are
// collapsed the way PurePosixPath does, so "a//b" and "a/b" collide in the
// duplicate check.
func safeZipPath(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")

	unsafe := func() (string, error) {
		return "", fmt.Errorf("%w: unsafe zip path entry: %q", ErrPackage, name)
	}

	if normalized == "" || strings.ContainsRune(normalized, '\x00') || strings.HasPrefix(normalized, "/") {
		return unsafe()
	}

	var parts []string
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		// Everything collapsed: "", ".", "./" ...
		return "", fmt.Errorf("%w: invalid zip path entry: %q", ErrPackage, name)
	}
	for _, part := range parts {
		if part == ".." {
			return unsafe()
		}
	}
	if strings.HasSuffix(parts[0], ":") {
		// Drive-letter first segment ("C:/x") after backslash normalization.
		return unsafe()
	}

	return strings.Join(parts, "/"), nil
}

// readZipMember decompresses one member with a streamed cap: the running
// total may never exceed the declared size or the global uncompressed cap,
// and the final total must equal the declaration.
func readZipMember(member validatedMember) ([]byte, error) {
	reader, err := member.file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: SkillHub package is not a valid ZIP archive: %v", ErrPackage, err)
	}
	defer reader.Close()

	declared := member.file.UncompressedSize64
	var chunks [][]byte
	var total uint64
	buf := make([]byte, readChunk)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			total += uint64(n)
			if total > declared || total > maxZipUncompressedBytes {
				return nil, fmt.Errorf("%w: zip entry is too large: %s", ErrPackageTooLarge, member.file.Name)
			}
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, fmt.Errorf("%w: SkillHub package is not a valid ZIP archive: %v", ErrPackage, readErr)
		}
	}
	if total != declared {
		return nil, fmt.Errorf("%w: zip entry size mismatch: %s", ErrPackage, member.file.Name)
	}

	out := make([]byte, 0, total)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	return out, nil
}

// stripWrapperDirectory removes a single redundant top-level directory when
// the package has no root SKILL.md but exactly one top-level segment that
// contains SKILL.md directly under it ("pkg/SKILL.md" -> "SKILL.md").
// Unlike the Python original, which slices the prefix unconditionally and
// can produce empty names for a root-level file named like the wrapper,
// this port keeps members that do not carry the prefix verbatim.
func stripWrapperDirectory(files ZipFiles) ZipFiles {
	hasRoot := false
	topLevels := make(map[string]bool)
	for _, f := range files {
		if f.Name == "SKILL.md" {
			hasRoot = true
			break
		}
		topLevels[strings.SplitN(f.Name, "/", 2)[0]] = true
	}
	if hasRoot || len(topLevels) != 1 {
		return files
	}

	var wrapper string
	for level := range topLevels {
		wrapper = level
	}
	prefix := wrapper + "/"
	hasWrappedManifest := false
	for _, f := range files {
		if f.Name == prefix+"SKILL.md" {
			hasWrappedManifest = true
			break
		}
	}
	if !hasWrappedManifest {
		return files
	}

	stripped := make(ZipFiles, 0, len(files))
	for _, f := range files {
		if strings.HasPrefix(f.Name, prefix) {
			stripped = append(stripped, struct {
				Name    string
				Content []byte
			}{Name: f.Name[len(prefix):], Content: f.Content})
		} else {
			stripped = append(stripped, f)
		}
	}
	return stripped
}
