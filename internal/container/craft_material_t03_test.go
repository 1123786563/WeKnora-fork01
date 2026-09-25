package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestCraftT03InputPublicationLeavesNoExecutableBits verifies the sandbox
// material boundary from the host side: an uploaded script published into a
// RunView inputs digest directory lands as a regular file with mode 0644 —
// read for everyone, writable by no one but the owner, and carrying no
// execute bits for any principal, so the read-only input tree holds data
// without executable capability.
func TestCraftT03InputPublicationLeavesNoExecutableBits(t *testing.T) {
	dir := t.TempDir()
	dirFD, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	require.NoError(t, err)
	defer unix.Close(dirFD)

	published, err := writeRunViewInputAtomic(dirFD, "analyze.py", []byte("import csv\nprint('data')\n"))
	require.NoError(t, err)
	require.True(t, published)

	info, err := os.Lstat(filepath.Join(dir, "analyze.py"))
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "published input must stay mode 0644")
	require.Zero(t, info.Mode()&0o111, "published input must carry no execute bits for any principal")
}
