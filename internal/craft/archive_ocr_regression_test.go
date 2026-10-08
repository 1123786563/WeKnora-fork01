package craft

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildTarGzLikeGnuTarDot builds the very common `tar -czf x.tgz .` layout:
// the first entry is the archive root directory "./" followed by regular
// members inside it.
func buildTarGzLikeGnuTarDot(t *testing.T) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "./index.html", Mode: 0o644, Size: int64(len("<html>T02</html>"))}))
	_, err := tw.Write([]byte("<html>T02</html>"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	_, err = zw.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return gzBuf.Bytes()
}

// TestExtractArchiveSkipsArchiveRootDirectoryEntry is the OCR medium-finding
// regression: a GNU tar archive rooted at "./" is a legal common layout and
// must extract, not report the root directory entry as an escape.
func TestExtractArchiveSkipsArchiveRootDirectoryEntry(t *testing.T) {
	members, err := ExtractArchive(buildTarGzLikeGnuTarDot(t))
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "index.html", members[0].Path)
	require.Equal(t, "<html>T02</html>", string(members[0].Content))

	// The same no-op holds for a zip that carries a "./" directory entry.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	dir, err := zw.Create("./")
	require.NoError(t, err)
	_, err = dir.Write(nil)
	require.NoError(t, err)
	f, err := zw.Create("notes.txt")
	require.NoError(t, err)
	_, err = f.Write([]byte("opaque"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	members, err = ExtractArchive(zipBuf.Bytes())
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "notes.txt", members[0].Path)
}

// TestExtractArchiveContentRejectionsAreRequestErrors is the OCR low-finding
// regression: non-archive bytes and nested archive members are deterministic
// rejections of the caller's content and must surface as ErrInvalidInput
// (HTTP 400), never ErrUnsupported (HTTP 503, which would promise that a
// retry of the same bytes could succeed and pollute 5xx monitoring).
func TestExtractArchiveContentRejectionsAreRequestErrors(t *testing.T) {
	_, err := ExtractArchive([]byte("certainly not an archive"))
	require.ErrorIs(t, err, ErrInvalidInput)
	require.NotErrorIs(t, err, ErrUnsupported)

	// A member that is itself an archive refuses recursion the same way.
	inner := loadArchiveFixture(t, "valid.zip")
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	f, ferr := zw.Create("nested.zip")
	require.NoError(t, ferr)
	_, ferr = f.Write(inner)
	require.NoError(t, ferr)
	require.NoError(t, zw.Close())
	_, err = ExtractArchive(zipBuf.Bytes())
	require.ErrorIs(t, err, ErrInvalidInput)
	require.NotErrorIs(t, err, ErrUnsupported)
}
