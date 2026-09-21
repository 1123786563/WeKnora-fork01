package skillhub

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// zipEntry describes one member written through the real archive/zip writer.
type zipEntry struct {
	name  string
	data  []byte
	mode  fs.FileMode // zero -> plain 0644 file
	flags uint16      // preserved into the entry's general-purpose flags
}

// buildZip writes entries with the standard library writer so headers are
// self-consistent. Use forgeZip below when a test needs headers that lie.
func buildZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		fh := &zip.FileHeader{
			Name:   e.name,
			Method: zip.Deflate,
			Flags:  e.flags,
		}
		switch {
		case strings.HasSuffix(e.name, "/"):
			fh.SetMode(fs.ModeDir | 0o755)
		case e.mode != 0:
			fh.SetMode(e.mode)
		default:
			fh.SetMode(0o644)
		}
		w, err := zw.CreateHeader(fh)
		require.NoError(t, err)
		if len(e.data) > 0 {
			_, err = w.Write(e.data)
			require.NoError(t, err)
		}
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// forgedEntry is one zip member whose declared central-directory sizes may
// differ from the bytes actually stored, mirroring hostile archives.
type forgedEntry struct {
	name                 string
	data                 []byte
	method               uint16
	declaredUncompressed uint64
	declaredCompressed   uint64
}

// forgeZip assembles a minimal stored-method zip by hand. Local and central
// headers both carry the declared (possibly lying) sizes.
func forgeZip(entries ...forgedEntry) []byte {
	var local, central bytes.Buffer
	offsets := make([]uint64, len(entries))
	crc := make([]uint32, len(entries))

	writeLocal := func(e forgedEntry, sum uint32) {
		var h [30]byte
		binary.LittleEndian.PutUint32(h[0:], 0x04034b50)
		binary.LittleEndian.PutUint16(h[4:], 20)
		binary.LittleEndian.PutUint16(h[6:], 0)
		binary.LittleEndian.PutUint16(h[8:], e.method)
		binary.LittleEndian.PutUint32(h[14:], sum)
		binary.LittleEndian.PutUint32(h[18:], uint32(e.declaredCompressed))
		binary.LittleEndian.PutUint32(h[22:], uint32(e.declaredUncompressed))
		binary.LittleEndian.PutUint16(h[26:], uint16(len(e.name)))
		local.Write(h[:])
		local.WriteString(e.name)
		local.Write(e.data)
	}

	for i, e := range entries {
		offsets[i] = uint64(local.Len())
		crc[i] = crc32.ChecksumIEEE(e.data)
		writeLocal(e, crc[i])
	}

	for i, e := range entries {
		var h [46]byte
		binary.LittleEndian.PutUint32(h[0:], 0x02014b50)
		binary.LittleEndian.PutUint16(h[4:], 20)
		binary.LittleEndian.PutUint16(h[6:], 20)
		binary.LittleEndian.PutUint16(h[10:], e.method)
		binary.LittleEndian.PutUint32(h[16:], crc[i])
		binary.LittleEndian.PutUint32(h[20:], uint32(e.declaredCompressed))
		binary.LittleEndian.PutUint32(h[24:], uint32(e.declaredUncompressed))
		binary.LittleEndian.PutUint16(h[28:], uint16(len(e.name)))
		binary.LittleEndian.PutUint32(h[38:], 0o644<<16)
		binary.LittleEndian.PutUint32(h[42:], uint32(offsets[i]))
		central.Write(h[:])
		central.WriteString(e.name)
	}

	var eocd [22]byte
	binary.LittleEndian.PutUint32(eocd[0:], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[8:], uint16(len(entries)))
	binary.LittleEndian.PutUint16(eocd[10:], uint16(len(entries)))
	binary.LittleEndian.PutUint32(eocd[12:], uint32(central.Len()))
	binary.LittleEndian.PutUint32(eocd[16:], uint32(local.Len()))

	var out bytes.Buffer
	out.Write(local.Bytes())
	out.Write(central.Bytes())
	out.Write(eocd[:])
	return out.Bytes()
}

func namesOf(files ZipFiles) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}

func TestParsePackageHappyPath(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("---\nname: demo\n---\nbody")},
		zipEntry{name: "reference.md", data: []byte("docs")},
		zipEntry{name: "scripts/run.sh", data: []byte("#!/bin/sh\n")},
		zipEntry{name: "assets/"},
	)
	files, err := ParsePackage(payload)
	require.NoError(t, err)
	require.Equal(t, []string{"SKILL.md", "reference.md", "scripts/run.sh"}, namesOf(files))
	require.Equal(t, []byte("---\nname: demo\n---\nbody"), files[0].Content)
}

func TestParsePackageNotAZip(t *testing.T) {
	_, err := ParsePackage([]byte("this is not a zip archive"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageWrapperDirStripped(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "pkg/", data: nil},
		zipEntry{name: "pkg/SKILL.md", data: []byte("manifest")},
		zipEntry{name: "pkg/lib/tool.py", data: []byte("print(1)")},
	)
	files, err := ParsePackage(payload)
	require.NoError(t, err)
	require.Equal(t, []string{"SKILL.md", "lib/tool.py"}, namesOf(files))
	require.Equal(t, []byte("manifest"), files[0].Content)
}

func TestParsePackageRootKeptWhenRootSkillMDPresent(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("root manifest")},
		zipEntry{name: "pkg/extra.txt", data: []byte("keep me")},
	)
	files, err := ParsePackage(payload)
	require.NoError(t, err)
	require.Equal(t, []string{"SKILL.md", "pkg/extra.txt"}, namesOf(files))
}

func TestParsePackageMissingRootSkillMD(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "a.txt", data: []byte("a")},
		zipEntry{name: "b/c.txt", data: []byte("c")},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "root SKILL.md")
}

func TestParsePackageMultipleTopLevelsWithoutRootSkillMD(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "one/SKILL.md", data: []byte("one")},
		zipEntry{name: "two/readme.txt", data: []byte("two")},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageOnlyDirectories(t *testing.T) {
	payload := buildZip(t, zipEntry{name: "pkg/", data: nil})
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageTooManyEntries(t *testing.T) {
	entries := []zipEntry{{name: "SKILL.md", data: []byte("m")}}
	for i := 0; i < maxZipEntries; i++ {
		entries = append(entries, zipEntry{name: "f" + itoa(i), data: []byte("x")})
	}
	_, err := ParsePackage(buildZip(t, entries...))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "too many entries")
}

func TestParsePackageDuplicateNames(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("first")},
		zipEntry{name: "SKILL.md", data: []byte("second")},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "uplicate")
}

func TestParsePackageDuplicateAfterNormalization(t *testing.T) {
	// "a//b" and "a/b" normalize to the same path and must collide.
	payload := buildZip(t,
		zipEntry{name: "a//b.txt", data: []byte("x")},
		zipEntry{name: "a/b.txt", data: []byte("y")},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageDirectoryShadowsFileName(t *testing.T) {
	// A directory entry "SKILL.md/" cleans to "SKILL.md" and duplicates the file.
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("file")},
		zipEntry{name: "SKILL.md/", data: nil},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageUnsafePaths(t *testing.T) {
	cases := []string{
		"../evil.txt",
		"/etc/passwd",
		"a/../../evil.txt",
		"..",
		".",
		"",
		"C:/autoexec.bat",
		"a\\..\\b.txt", // backslash traversal
		"a/b\x00c.txt", // NUL byte
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			entries := []zipEntry{{name: "SKILL.md", data: []byte("m")}, {name: name, data: []byte("x")}}
			_, err := ParsePackage(buildZip(t, entries...))
			require.Error(t, err, "path %q must be rejected", name)
			require.ErrorIs(t, err, ErrPackage)
		})
	}
}

func TestParsePackageDotSegmentNormalized(t *testing.T) {
	// "./SKILL.md" cleans to "SKILL.md": accepted, not treated as missing root.
	payload := buildZip(t, zipEntry{name: "./SKILL.md", data: []byte("m")})
	files, err := ParsePackage(payload)
	require.NoError(t, err)
	require.Equal(t, []string{"SKILL.md"}, namesOf(files))
}

func TestParsePackageSymlinkEntry(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "link.txt", data: []byte("/etc/passwd"), mode: fs.ModeSymlink | 0o777},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "nsupported zip entry type")
}

func TestParsePackageEncryptedEntry(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "secret.txt", data: []byte("x"), flags: 0x1},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "ncrypted")
}

func TestParsePackageCompressionRatioBomb(t *testing.T) {
	// 2 MiB of zeros deflates to a couple of KB: ratio far above 100.
	bomb := bytes.Repeat([]byte{0}, 2*1024*1024)
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "bomb.bin", data: bomb},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
	require.Contains(t, err.Error(), "compression ratio")
}

func TestParsePackageDeclaredUncompressedOverCap(t *testing.T) {
	// Central directory declares 64 MiB + 1 for a stored entry that really
	// holds one byte. Validation must trust nothing and reject on the
	// declared total alone, before any read is attempted.
	payload := forgeZip(
		forgedEntry{name: "SKILL.md", data: []byte("m"), declaredUncompressed: 1, declaredCompressed: 1},
		forgedEntry{name: "huge.bin", data: []byte("x"), declaredUncompressed: uint64(maxZipUncompressedBytes + 1), declaredCompressed: 1},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestParsePackageRatioOnForgedHeaders(t *testing.T) {
	// Declared 2 MiB uncompressed vs 8 KiB compressed: ratio 256 > 100.
	payload := forgeZip(
		forgedEntry{name: "SKILL.md", data: []byte("m"), declaredUncompressed: 1, declaredCompressed: 1},
		forgedEntry{name: "r.bin", data: []byte("x"), declaredUncompressed: 2 * 1024 * 1024, declaredCompressed: 8 * 1024},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
	require.Contains(t, err.Error(), "compression ratio")
}

func TestParsePackageReadExceedsDeclaredSize(t *testing.T) {
	// Stored entry declaring 1 byte of uncompressed data over 5 actual
	// bytes: the streamed read must trip the per-entry cap.
	payload := forgeZip(
		forgedEntry{name: "SKILL.md", data: []byte("m"), declaredUncompressed: 1, declaredCompressed: 1},
		forgedEntry{name: "grow.bin", data: []byte("xxxxx"), declaredUncompressed: 1, declaredCompressed: 5},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageReadShortOfDeclaredSize(t *testing.T) {
	// Declared uncompressed 8 bytes but only 3 stored: size mismatch.
	payload := forgeZip(
		forgedEntry{name: "SKILL.md", data: []byte("m"), declaredUncompressed: 1, declaredCompressed: 1},
		forgedEntry{name: "shrink.bin", data: []byte("abc"), declaredUncompressed: 8, declaredCompressed: 3},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestParsePackageInvalidUTF8Member(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "bad.txt", data: []byte{0xff, 0xfe, 'x'}},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "bad.txt")
}

// SkillHub once shipped a published SKILL.md where ≤ (U+2264, e2 89 a4) was
// corrupted to e2 6a 24; Octop carries a targeted repair table for exactly
// that corruption and this port must too.
func TestParsePackageManifestKnownUTF8Repair(t *testing.T) {
	broken := []byte("---\nname: demo\n---\nkeep it \xe2j$ 5 items\n")
	payload := buildZip(t, zipEntry{name: "SKILL.md", data: broken})
	files, err := ParsePackage(payload)
	require.NoError(t, err, "known \u2264 corruption in the manifest must be repaired, not rejected")
	require.Equal(t, "---\nname: demo\n---\nkeep it \u2264 5 items\n", string(files[0].Content))
}

func TestParsePackageTextMemberKnownUTF8Repair(t *testing.T) {
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "notes.txt", data: []byte("count \xe2j$ done")},
	)
	files, err := ParsePackage(payload)
	require.NoError(t, err)
	require.Equal(t, "count \u2264 done", string(files[1].Content))
}

func TestParsePackageBinaryMemberPassthrough(t *testing.T) {
	// Invalid-UTF-8 bytes in a binary-looking member are payload, not text.
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0xff, 0xfe, 0x00, 0xc3}
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "assets/logo.png", data: png},
		zipEntry{name: "fonts/icon.woff2", data: []byte{0xd0, 0x17, 0xff}},
	)
	files, err := ParsePackage(payload)
	require.NoError(t, err, "binary members must pass through untouched")
	require.Len(t, files, 3)
	require.Equal(t, png, files[1].Content)
	require.Equal(t, []byte{0xd0, 0x17, 0xff}, files[2].Content)
}

func TestParsePackageTextMemberUnrepairableUTF8StillErrors(t *testing.T) {
	// No known repair matches 0xff 0xfe: text members stay strictly invalid.
	payload := buildZip(t,
		zipEntry{name: "SKILL.md", data: []byte("m")},
		zipEntry{name: "readme.md", data: []byte{0xff, 0xfe, 'x'}},
	)
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "readme.md")
}

func TestParsePackageManifestUnrepairableUTF8(t *testing.T) {
	payload := buildZip(t, zipEntry{name: "SKILL.md", data: []byte("bad \xff\xfe manifest")})
	_, err := ParsePackage(payload)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
	require.Contains(t, err.Error(), "SKILL.md")
}

func TestParsePackageExtensionlessTextNames(t *testing.T) {
	// LICENSE/README are text by name (no suffix), like Octop's name set.
	for _, name := range []string{"LICENSE", "README.md"} {
		payload := buildZip(t,
			zipEntry{name: "SKILL.md", data: []byte("m")},
			zipEntry{name: name, data: []byte{0xff}},
		)
		_, err := ParsePackage(payload)
		require.Error(t, err, "%s is a text member and must fail UTF-8 validation", name)
		require.ErrorIs(t, err, ErrPackage)
	}
}

func TestParsePackageEmptyZip(t *testing.T) {
	// A zip with no members at all is a valid archive but not a skill.
	_, err := ParsePackage(buildZip(t))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackage)
}

func TestZipFilesAliasShape(t *testing.T) {
	// ZipFiles is an alias: callers can build the literal type directly.
	files := ZipFiles{{Name: "SKILL.md", Content: []byte("x")}}
	_, err := ParsePackage(buildZip(t, zipEntry{name: "SKILL.md", data: []byte("x")}))
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// Guard: the error taxonomy must stay chained so errors.Is walks it.
func TestErrorTaxonomy(t *testing.T) {
	require.ErrorIs(t, ErrMarketTimeout, ErrMarket)
	require.ErrorIs(t, ErrPackageTooLarge, ErrPackage)
}
