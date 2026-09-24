package craft

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// loadArchiveFixture reads one committed fixture from testdata/archive.
func loadArchiveFixture(t *testing.T, name string) []byte {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixture := filepath.Join(filepath.Dir(filename), "testdata", "archive", name)
	data, err := os.ReadFile(fixture)
	require.NoError(t, err)
	return data
}

type testZipEntry struct {
	name    string
	content []byte
	mode    os.FileMode
}

// buildTestZip assembles an in-memory zip with regular and special entries.
func buildTestZip(t *testing.T, entries []testZipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		h.SetMode(mode)
		w, err := zw.CreateHeader(h)
		require.NoError(t, err)
		if e.mode&os.ModeSymlink == 0 && !e.mode.IsDir() {
			_, err = w.Write(e.content)
			require.NoError(t, err)
		} else if e.mode&os.ModeSymlink != 0 {
			_, err = w.Write(e.content)
			require.NoError(t, err)
		}
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

type testTarEntry struct {
	name     string
	content  []byte
	typeflag byte
	link     string
}

// buildTestTarGz assembles an in-memory gzipped tar with regular and special
// entries (symlinks, hard links, devices, FIFOs).
func buildTestTarGz(t *testing.T, entries []testTarEntry) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, e := range entries {
		typeflag := e.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.content)), Typeflag: typeflag}
		if typeflag == tar.TypeSymlink || typeflag == tar.TypeLink {
			hdr.Size = 0
			hdr.Linkname = e.link
		}
		if typeflag == tar.TypeDir {
			hdr.Size = 0
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Size > 0 {
			_, err := tw.Write(e.content)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	_, err := zw.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return gzBuf.Bytes()
}

func TestArchiveDetectsSupportedFormats(t *testing.T) {
	zipData := loadArchiveFixture(t, "valid.zip")
	format, ok := DetectArchiveFormat(zipData)
	require.True(t, ok)
	require.Equal(t, ArchiveZip, format)

	tarGz := buildTestTarGz(t, []testTarEntry{{name: "a.txt", content: []byte("x")}})
	format, ok = DetectArchiveFormat(tarGz)
	require.True(t, ok)
	require.Equal(t, ArchiveTarGz, format)

	// Raw (uncompressed) tar carries the ustar magic at offset 257.
	var rawTar bytes.Buffer
	tw := tar.NewWriter(&rawTar)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 1}))
	_, err := tw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	format, ok = DetectArchiveFormat(rawTar.Bytes())
	require.True(t, ok)
	require.Equal(t, ArchiveTar, format)

	_, ok = DetectArchiveFormat([]byte("plain text, not an archive"))
	require.False(t, ok)
	_, ok = DetectArchiveFormat(nil)
	require.False(t, ok)
}

func TestArchiveExtractsZipWithinLimits(t *testing.T) {
	members, err := ExtractArchive(loadArchiveFixture(t, "valid.zip"))
	require.NoError(t, err)
	require.Len(t, members, 3)
	// Deterministic, canonically-pathed, sorted output.
	require.Equal(t, "data/meta.json", members[0].Path)
	require.Equal(t, `{"region":"north","sales":120}`, string(members[0].Content))
	require.Equal(t, "data/sales.csv", members[1].Path)
	require.Equal(t, "region,sales\nnorth,120\nsouth,80\n", string(members[1].Content))
	require.Equal(t, "docs/readme.md", members[2].Path)
	require.Equal(t, "# Region sales guide\n", string(members[2].Content))
}

func TestArchiveExtractsTarGzWithinLimits(t *testing.T) {
	data := buildTestTarGz(t, []testTarEntry{
		{name: "docs/", typeflag: tar.TypeDir},
		{name: "docs/guide.md", content: []byte("guide")},
		{name: "top.txt", content: []byte("top")},
	})
	members, err := ExtractArchive(data)
	require.NoError(t, err)
	require.Len(t, members, 2, "directory entries produce no material")
	require.Equal(t, "docs/guide.md", members[0].Path)
	require.Equal(t, "top.txt", members[1].Path)
}

func TestArchiveRejectsUnsupportedBytes(t *testing.T) {
	_, err := ExtractArchive([]byte("definitely not an archive"))
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestArchiveValidateEntryPathRules(t *testing.T) {
	canonical, err := ValidateArchiveEntryPath("./docs/data.csv")
	require.NoError(t, err)
	require.Equal(t, "docs/data.csv", canonical)

	canonical, err = ValidateArchiveEntryPath("docs//data.csv")
	require.NoError(t, err)
	require.Equal(t, "docs/data.csv", canonical)

	canonical, err = ValidateArchiveEntryPath("docs/./data.csv")
	require.NoError(t, err)
	require.Equal(t, "docs/data.csv", canonical)

	deep := "a/b/c/d/e/f/g/file.txt"
	canonical, err = ValidateArchiveEntryPath(deep)
	require.NoError(t, err)
	require.Equal(t, deep, canonical)

	for _, hostile := range []string{
		"",
		".",
		"..",
		"/absolute.txt",
		"../escape.txt",
		"a/../../escape.txt",
		`a\backslash.txt`,
		"a\x00nul.txt",
		"a/b/c/d/e/f/g/h/i/too-deep.txt",
	} {
		_, err := ValidateArchiveEntryPath(hostile)
		require.ErrorIs(t, err, ErrInvalidInput, "path %q must be rejected", hostile)
	}
}

func TestArchiveRejectsHostileEntryPaths(t *testing.T) {
	for _, hostile := range []string{
		"/absolute.txt",
		"../escape.txt",
		"a/../../escape.txt",
		`a\backslash.txt`,
		"a\x00nul.txt",
	} {
		data := buildTestZip(t, []testZipEntry{
			{name: "ok.txt", content: []byte("fine")},
			{name: hostile, content: []byte("evil")},
		})
		_, err := ExtractArchive(data)
		require.ErrorIs(t, err, ErrInvalidInput, "entry %q must be rejected", hostile)
	}
}

func TestArchiveRejectsNormalizedDuplicatePaths(t *testing.T) {
	data := buildTestZip(t, []testZipEntry{
		{name: "b.txt", content: []byte("first")},
		{name: "./b.txt", content: []byte("second")},
	})
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrConflict, "two entries normalizing to one canonical path must conflict")

	data = buildTestZip(t, []testZipEntry{
		{name: "b.txt", content: []byte("first")},
		{name: "docs/../b.txt", content: []byte("second")},
	})
	_, err = ExtractArchive(data)
	require.ErrorIs(t, err, ErrConflict, "entries normalizing across directories to one path must conflict")
}

func TestArchiveRejectsNonRegularZipMembers(t *testing.T) {
	data := buildTestZip(t, []testZipEntry{
		{name: "real.txt", content: []byte("fine")},
		{name: "link.txt", content: []byte("target"), mode: os.ModeSymlink | 0o777},
	})
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "symlink members must be rejected")
}

func TestArchiveRejectsNonRegularTarMembers(t *testing.T) {
	special := []testTarEntry{
		{name: "link.txt", typeflag: tar.TypeSymlink, link: "/etc/passwd"},
		{name: "hard.txt", typeflag: tar.TypeLink, link: "ok.txt"},
		{name: "char.dev", typeflag: tar.TypeChar},
		{name: "block.dev", typeflag: tar.TypeBlock},
		{name: "fifo.pipe", typeflag: tar.TypeFifo},
	}
	for _, entry := range special {
		data := buildTestTarGz(t, []testTarEntry{
			{name: "ok.txt", content: []byte("fine")},
			entry,
		})
		_, err := ExtractArchive(data)
		require.ErrorIs(t, err, ErrInvalidInput, "entry %q (%c) must be rejected", entry.name, entry.typeflag)
	}
}

func TestArchiveRejectsNestedArchiveMembers(t *testing.T) {
	inner := buildTestZip(t, []testZipEntry{{name: "inner.txt", content: []byte("inner")}})
	// A real inner archive by content.
	data := buildTestZip(t, []testZipEntry{
		{name: "outer.txt", content: []byte("outer")},
		{name: "payload.bin", content: inner},
	})
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrUnsupported, "recursive archive expansion must fail deterministically")

	// A member merely claiming an archive extension is also refused, even
	// when its bytes are not an archive, so renaming cannot smuggle one.
	data = buildTestZip(t, []testZipEntry{
		{name: "inner.zip", content: []byte("not really a zip")},
	})
	_, err = ExtractArchive(data)
	require.ErrorIs(t, err, ErrUnsupported)

	gz := buildTestTarGz(t, []testTarEntry{{name: "a.txt", content: []byte("x")}})
	data = buildTestZip(t, []testZipEntry{
		{name: "inner.tgz", content: gz},
	})
	_, err = ExtractArchive(data)
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestArchiveRejectsCompressionBomb(t *testing.T) {
	// The committed bomb fixture expands 4 MiB of zeros from a ~4 KiB archive,
	// far above the bounded compression ratio.
	_, err := ExtractArchive(loadArchiveFixture(t, "bomb.zip"))
	require.ErrorIs(t, err, ErrInvalidInput, "compression bombs must fail deterministically")
}

func TestArchiveEnforcesEntryCountCap(t *testing.T) {
	entries := make([]testZipEntry, 0, MaxArchiveEntries+1)
	for i := 0; i < MaxArchiveEntries+1; i++ {
		entries = append(entries, testZipEntry{name: "f" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".txt", content: []byte("x")})
	}
	data := buildTestZip(t, entries)
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "archives above the entry cap must be rejected")

	entries = entries[:MaxArchiveEntries]
	data = buildTestZip(t, entries)
	members, err := ExtractArchive(data)
	require.NoError(t, err)
	require.Len(t, members, MaxArchiveEntries)
}

func TestArchiveEnforcesMemberSizeCap(t *testing.T) {
	// A member over the per-input cap is refused even when the archive
	// itself is small: the read is bounded, so the declared size is
	// irrelevant.
	data := buildTestZip(t, []testZipEntry{
		{name: "huge.bin", content: bytes.Repeat([]byte{0}, MaxInputBytes+1)},
	})
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "members above the per-input byte cap must be rejected")
}

func TestArchiveEnforcesCumulativeCap(t *testing.T) {
	// Six 18 MiB members stay under every per-file cap but together exceed
	// the cumulative expanded-byte ceiling; highly compressible content
	// keeps the fixture itself tiny.
	entries := make([]testZipEntry, 0, 6)
	for i := 0; i < 6; i++ {
		entries = append(entries, testZipEntry{
			name:    "part" + string(rune('a'+i)) + ".bin",
			content: bytes.Repeat([]byte{0}, 18<<20),
		})
	}
	data := buildTestZip(t, entries)
	require.Less(t, int64(len(data)), int64(MaxArchiveExpandedBytes), "fixture must stay compressed")
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "cumulative expanded bytes cannot bypass the round cap")
}

func TestArchiveRejectsTruncatedStreamMidExtraction(t *testing.T) {
	full := buildTestTarGz(t, []testTarEntry{
		{name: "first.txt", content: bytes.Repeat([]byte("A"), 4096)},
		{name: "second.txt", content: bytes.Repeat([]byte("B"), 65536)},
	})
	// Highly compressible members compress to a few hundred bytes; cut the
	// stream in half so the damage always lands mid-member.
	require.Greater(t, len(full), 128)
	truncated := full[:len(full)/2]
	_, err := ExtractArchive(truncated)
	require.Error(t, err, "a stream truncated mid-member must fail")
	require.False(t, errors.Is(err, ErrUnsupported))
}

func TestArchiveRejectsEmptyAndDirOnlyArchives(t *testing.T) {
	data := buildTestZip(t, nil)
	_, err := ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "an archive with no members must be refused")

	data = buildTestZip(t, []testZipEntry{{name: "docs/", mode: os.ModeDir | 0o755}})
	_, err = ExtractArchive(data)
	require.ErrorIs(t, err, ErrInvalidInput, "directory-only archives carry no material")
}
