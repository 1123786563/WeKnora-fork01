package craft

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strings"
	"time"
)

// Hard resource ceilings for expanding one archive input. The archive itself
// is already bounded by the T01 input caps (it entered as one opaque input of
// at most MaxInputBytes); these ceilings bound what it may become. Every
// ceiling is enforced on actually-read bytes, never on declared header
// sizes, which may lie.
const (
	// MaxArchiveEntries caps the regular-file members one archive may add,
	// aligned with the per-round input count so expansion cannot smuggle a
	// wider round than an upload.
	MaxArchiveEntries = MaxInputsPerRound
	// MaxArchiveMemberBytes caps one expanded member, aligned with the
	// per-file input cap.
	MaxArchiveMemberBytes = MaxInputBytes
	// MaxArchiveExpandedBytes caps the cumulative expanded bytes of one
	// archive, aligned with the 100 MiB per-round total. Because members are
	// buffered until the whole archive passed every check, this is also the
	// extraction's memory ceiling.
	MaxArchiveExpandedBytes = MaxTotalInputBytes
	// MaxArchiveDepth caps the directory nesting of one member path.
	MaxArchiveDepth = 8
	// MaxArchiveCompressionRatio caps expanded bytes per compressed byte;
	// anything above it is treated as a compression bomb.
	MaxArchiveCompressionRatio = 100
	// MaxArchiveExtractDuration bounds the wall clock of one expansion as
	// enforced by the SERVICE layer's context around the database/file IO;
	// the pure-CPU decompression loop itself is bounded indirectly by the
	// byte and entry ceilings, not by this constant directly.
	MaxArchiveExtractDuration = 30 * time.Second
)

// ArchiveFormat identifies one supported archive container.
type ArchiveFormat string

// The supported archive containers of the first release.
const (
	ArchiveZip   ArchiveFormat = "zip"
	ArchiveTarGz ArchiveFormat = "tar+gzip"
	ArchiveTar   ArchiveFormat = "tar"
)

// DetectArchiveFormat reports which supported container the bytes use. It
// sniffs content magic only — a file extension never proves the bytes were
// understood.
func DetectArchiveFormat(data []byte) (ArchiveFormat, bool) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06")) {
		return ArchiveZip, true
	}
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return ArchiveTarGz, true
	}
	if len(data) >= 262 && string(data[257:262]) == "ustar" {
		return ArchiveTar, true
	}
	return "", false
}

// ArchiveMember is one extracted regular-file member with its canonical
// archive-relative path.
type ArchiveMember struct {
	Path    string
	Content []byte
}

// ValidateArchiveEntryPath canonicalizes one archive entry name and refuses
// everything that could escape the archive root or smuggle a non-canonical
// duplicate: empty names, NUL and backslash separators, absolute paths,
// Windows drive letters, parent traversal, over-deep nesting and over-long
// segments. The returned path is the clean, forward-slash, relative form
// used for duplicate detection and member identities. The archive root
// itself ("." / "./", the common first entry of `tar -czf x.tgz .`) is a
// validated no-op: it names nothing below the root, so the caller skips it
// via IsArchiveRootEntry instead of reserving a canonical path.
func ValidateArchiveEntryPath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%w: empty archive entry name", ErrInvalidInput)
	}
	if strings.ContainsAny(raw, "\\\x00") {
		return "", fmt.Errorf("%w: archive entry %q uses a non-canonical separator", ErrInvalidInput, raw)
	}
	if strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("%w: archive entry %q is an absolute path", ErrInvalidInput, raw)
	}
	cleaned := path.Clean(raw)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("%w: archive entry %q escapes the archive root", ErrInvalidInput, raw)
	}
	segments := strings.Split(cleaned, "/")
	if len(segments) > MaxArchiveDepth {
		return "", fmt.Errorf("%w: archive entry %q nests %d levels over the cap %d",
			ErrInvalidInput, raw, len(segments), MaxArchiveDepth)
	}
	for i, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: archive entry %q is not canonical", ErrInvalidInput, raw)
		}
		if len(segment) > 255 {
			return "", fmt.Errorf("%w: archive entry %q has an over-long segment", ErrInvalidInput, raw)
		}
		if i == 0 && len(segment) == 2 && segment[1] == ':' && isDriveLetter(segment[0]) {
			return "", fmt.Errorf("%w: archive entry %q is a drive-absolute path", ErrInvalidInput, raw)
		}
	}
	return cleaned, nil
}

// IsArchiveRootEntry reports whether an entry names the archive root itself
// ("." / "./" — the common first entry of GNU `tar -czf x.tgz .`). Root
// directory entries carry no material and no path below the root, so the
// extractors treat them as a no-op instead of an escape.
func IsArchiveRootEntry(raw string) bool {
	return path.Clean(raw) == "."
}

func isDriveLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ExtractArchive validates and extracts every regular-file member of one
// supported archive within the hard ceilings. It fails deterministically —
// never partially — on hostile paths, normalized duplicates, non-regular
// members (symlinks, hard links, devices, FIFOs, special files), nested
// archives, compression bombs and any ceiling breach, returning a typed
// craft error. The returned members are sorted by canonical path.
func ExtractArchive(archive []byte) ([]ArchiveMember, error) {
	format, ok := DetectArchiveFormat(archive)
	if !ok {
		// Deterministic rejection of the caller's own bytes: this is request
		// content, not a server dependency, so it maps to 400 (not the 503
		// that ErrUnsupported would produce).
		return nil, fmt.Errorf("%w: bytes are not a supported archive", ErrInvalidInput)
	}
	switch format {
	case ArchiveZip:
		return extractZipArchive(archive)
	case ArchiveTarGz:
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, fmt.Errorf("%w: malformed gzip stream: %v", ErrInvalidInput, err)
		}
		defer gz.Close()
		return extractTarArchive(gz, int64(len(archive)))
	case ArchiveTar:
		// A raw tar stores its members uncompressed, so the ratio ceiling
		// has nothing to catch; the byte caps govern it.
		return extractTarArchive(bytes.NewReader(archive), MaxArchiveMemberBytes)
	default:
		return nil, fmt.Errorf("%w: archive container %q", ErrUnsupported, format)
	}
}

// archiveBudget carries the cumulative accounting shared by every container.
type archiveBudget struct {
	compressed int64
	total      int64
	entries    int
	seen       map[string]struct{}
	members    []ArchiveMember
}

func newArchiveBudget(compressed int64) *archiveBudget {
	return &archiveBudget{compressed: compressed, seen: map[string]struct{}{}}
}

func archiveRatioExceeded(expanded, compressed int64) bool {
	if expanded <= 0 {
		return false
	}
	if compressed <= 0 {
		return true
	}
	if compressed > math.MaxInt64/MaxArchiveCompressionRatio {
		return false
	}
	return expanded > compressed*MaxArchiveCompressionRatio
}

// addExpandedBytes accounts for bytes already expanded by a container
// reader. Check before addition so cumulative accounting cannot overflow.
func (b *archiveBudget) addExpandedBytes(size int64) error {
	if size <= 0 {
		return nil
	}
	if b.total < 0 || size > math.MaxInt64-b.total {
		return fmt.Errorf("%w: archive expanded-byte accounting overflow", ErrInvalidInput)
	}
	total := b.total + size
	if total > MaxArchiveExpandedBytes {
		return fmt.Errorf("%w: archive expands over the %d byte cap", ErrInvalidInput, MaxArchiveExpandedBytes)
	}
	if archiveRatioExceeded(total, b.compressed) {
		return fmt.Errorf("%w: archive expands %d bytes from %d compressed bytes over the %d:1 ratio cap",
			ErrInvalidInput, total, b.compressed, MaxArchiveCompressionRatio)
	}
	b.total = total
	return nil
}

// archiveStreamReader accounts for the complete decompressed tar stream
// underneath archive/tar. PAX metadata is parsed inside tar.Reader.Next, so
// charging after Next returns would allow allocation before the limit.
type archiveStreamReader struct {
	reader       io.Reader
	budget       *archiveBudget
	limit        int64
	ratioLimited bool
}

func newArchiveStreamReader(reader io.Reader, budget *archiveBudget) *archiveStreamReader {
	ratioLimit := int64(math.MaxInt64)
	if budget.compressed <= 0 {
		ratioLimit = 0
	} else if budget.compressed <= math.MaxInt64/MaxArchiveCompressionRatio {
		ratioLimit = budget.compressed * MaxArchiveCompressionRatio
	}
	limit := int64(MaxArchiveExpandedBytes)
	if ratioLimit < limit {
		limit = ratioLimit
	}
	return &archiveStreamReader{
		reader: reader, budget: budget, limit: limit,
		ratioLimited: ratioLimit < int64(MaxArchiveExpandedBytes),
	}
}

func (r *archiveStreamReader) limitError() error {
	if r.ratioLimited {
		return fmt.Errorf("%w: archive expands %d bytes from %d compressed bytes over the %d:1 ratio cap",
			ErrInvalidInput, r.budget.total+1, r.budget.compressed, MaxArchiveCompressionRatio)
	}
	return fmt.Errorf("%w: archive expands over the %d byte cap", ErrInvalidInput, MaxArchiveExpandedBytes)
}

func (r *archiveStreamReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := r.limit - r.budget.total
	if remaining <= 0 {
		var probe [1]byte
		n, err := io.ReadFull(r.reader, probe[:])
		if n > 0 {
			return 0, r.limitError()
		}
		return 0, err
	}
	if int64(len(p)) > remaining {
		p = p[:int(remaining)]
	}
	n, err := r.reader.Read(p)
	if n > 0 {
		if addErr := r.budget.addExpandedBytes(int64(n)); addErr != nil {
			return 0, addErr
		}
	}
	return n, err
}

// reserveEntry canonicalizes one entry name for the seen set and rejects
// normalized duplicates across files and directories.
func (b *archiveBudget) reserveEntry(name string) (string, error) {
	canonical, err := ValidateArchiveEntryPath(name)
	if err != nil {
		return "", err
	}
	// The seen set is bounded SEPARATELY from the regular-file budget:
	// ordinary archives carry directory entries alongside their files
	// (Finder zip -r, Windows "send to compressed folder"), so the total
	// ceiling is a wide multiple of MaxArchiveEntries while regular-file
	// accounting stays capped by readMember. A malicious central directory
	// still cannot name hundreds of thousands of entries.
	if len(b.seen) >= 10*MaxArchiveEntries {
		return "", fmt.Errorf("%w: archive exceeds the maximum entry count %d", ErrInvalidInput, 10*MaxArchiveEntries)
	}
	if _, duplicate := b.seen[canonical]; duplicate {
		return "", fmt.Errorf("%w: archive entries normalize to the duplicate path %q", ErrConflict, canonical)
	}
	b.seen[canonical] = struct{}{}
	return canonical, nil
}

// readMember reads one regular member inside every ceiling. The read itself
// is bounded, so a lying header size cannot defer the rejection.
func (b *archiveBudget) readMember(r io.Reader, canonical string) error {
	return b.readMemberWithAccounting(r, canonical, true)
}

// readTarMember relies on archiveStreamReader, which has already charged the
// member's bytes together with headers, padding and PAX metadata.
func (b *archiveBudget) readTarMember(r io.Reader, canonical string) error {
	return b.readMemberWithAccounting(r, canonical, false)
}

func (b *archiveBudget) readMemberWithAccounting(r io.Reader, canonical string, countBytes bool) error {
	if b.entries >= MaxArchiveEntries {
		return fmt.Errorf("%w: archive holds more than %d members", ErrInvalidInput, MaxArchiveEntries)
	}
	content, err := io.ReadAll(io.LimitReader(r, MaxArchiveMemberBytes+1))
	if err != nil {
		return fmt.Errorf("%w: archive stream failed mid-extraction at %s: %v", ErrInvalidInput, canonical, err)
	}
	if int64(len(content)) > MaxArchiveMemberBytes {
		return fmt.Errorf("%w: archive member %s expands over the %d byte cap",
			ErrInvalidInput, canonical, MaxArchiveMemberBytes)
	}
	if countBytes {
		if err := b.addExpandedBytes(int64(len(content))); err != nil {
			return err
		}
	}
	if nestedArchiveMember(canonical, content) {
		// Deterministic content rejection: retrying the same archive can
		// never succeed, so this is a 4xx request error, not 503.
		return fmt.Errorf("%w: archive member %s is itself an archive; recursive expansion is refused",
			ErrInvalidInput, canonical)
	}
	b.entries++
	b.members = append(b.members, ArchiveMember{Path: canonical, Content: content})
	return nil
}

// finish sorts the members and refuses archives that carry no material.
func (b *archiveBudget) finish() ([]ArchiveMember, error) {
	if len(b.members) == 0 {
		return nil, fmt.Errorf("%w: archive holds no usable members", ErrInvalidInput)
	}
	sort.Slice(b.members, func(i, j int) bool { return b.members[i].Path < b.members[j].Path })
	return b.members, nil
}

// nestedArchiveMember reports whether a member would recurse: either the
// member's own name claims an archive extension, or its bytes carry a
// supported container's magic. Refusing both closes the rename bypass.
func nestedArchiveMember(canonical string, content []byte) bool {
	lower := strings.ToLower(canonical)
	for _, ext := range []string{".zip", ".tar", ".tgz", ".gz"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	if _, ok := DetectArchiveFormat(content); ok {
		return true
	}
	return false
}

func extractZipArchive(data []byte) ([]ArchiveMember, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: malformed zip container: %v", ErrInvalidInput, err)
	}
	// The central-directory count check bounds the reader.File materialized
	// allocation (see the round-2 note); the true fix is streaming EOCD
	// parsing, tracked in the lane backlog. The cap keeps the worst-case
	// transient at a documented multiple instead of the input's 4-5x.
	budget := newArchiveBudget(int64(len(data)))
	for _, file := range reader.File {
		mode := file.FileInfo().Mode()
		switch {
		case mode.IsDir():
			// Directory entries carry no material but still validate and
			// occupy their canonical path, so a later file cannot shadow
			// them and duplicates still conflict. The archive root itself
			// ("./") is the one no-op: it names nothing below the root.
			if IsArchiveRootEntry(file.Name) {
				continue
			}
			if _, err := budget.reserveEntry(file.Name); err != nil {
				return nil, err
			}
		case mode.IsRegular():
			canonical, err := budget.reserveEntry(file.Name)
			if err != nil {
				return nil, err
			}
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("%w: zip entry %s failed mid-extraction: %v", ErrInvalidInput, canonical, err)
			}
			err = budget.readMember(rc, canonical)
			rc.Close()
			if err != nil {
				return nil, err
			}
		default:
			// Symlinks, hard links, devices, FIFOs, sockets and any other
			// special member are refused.
			return nil, fmt.Errorf("%w: archive entry %q is not a regular file", ErrInvalidInput, file.Name)
		}
	}
	return budget.finish()
}

func extractTarArchive(r io.Reader, compressed int64) ([]ArchiveMember, error) {
	budget := newArchiveBudget(compressed)
	stream := newArchiveStreamReader(r, budget)
	reader := tar.NewReader(stream)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: tar stream failed mid-extraction: %v", ErrInvalidInput, err)
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			canonical, err := budget.reserveEntry(header.Name)
			if err != nil {
				return nil, err
			}
			if err := budget.readTarMember(reader, canonical); err != nil {
				return nil, err
			}
		case tar.TypeDir:
			// The archive root itself ("./", the common first entry of
			// `tar -czf x.tgz .`) is a no-op; other directories validate
			// and occupy their canonical path.
			if IsArchiveRootEntry(header.Name) {
				continue
			}
			if _, err := budget.reserveEntry(header.Name); err != nil {
				return nil, err
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			// tar.Reader normally consumes these internally in Next; if one
			// is exposed, the underlying stream reader already charged it.
			if _, err := io.Copy(io.Discard, reader); err != nil {
				return nil, fmt.Errorf("%w: archive pax header unreadable: %v", ErrInvalidInput, err)
			}
		default:
			// Hard links, symlinks, char/block devices, FIFOs, contiguous
			// files and vendor extensions are all refused.
			return nil, fmt.Errorf("%w: archive entry %q has unsupported type %q",
				ErrInvalidInput, header.Name, string(header.Typeflag))
		}
	}
	// tar.Reader stops at the first pair of zero blocks, but the enclosing
	// gzip stream may contain more expanded data or additional gzip members.
	// Drain through the same accounting reader so trailing bytes stay within
	// the archive budget and gzip checksums are validated before accepting it.
	if _, err := io.Copy(io.Discard, stream); err != nil {
		return nil, fmt.Errorf("%w: tar stream failed after archive terminator: %v", ErrInvalidInput, err)
	}
	return budget.finish()
}
