package database

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigrationVersionsUniquePerTrack pins B5-F53: golang-migrate's file
// source Initialize fails the whole track when two files claim the same
// version number, so a double-booked version blocks every deployment and
// every test that loads the tree. The SQLite half of the original clash
// (000119) was renumbered to 000123 in 8e9959402; this guard keeps both
// tracks honest going forward.
func TestMigrationVersionsUniquePerTrack(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))

	for _, track := range []string{"versioned", "sqlite"} {
		entries, err := os.ReadDir(filepath.Join(root, "migrations", track))
		require.NoError(t, err)

		versions := make([]int, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
				continue
			}
			versionText, _, found := strings.Cut(entry.Name(), "_")
			require.Truef(t, found, "%s migration must use a versioned filename: %s", track, entry.Name())
			version, err := strconv.Atoi(versionText)
			require.NoErrorf(t, err, "%s migration must start with a numeric version: %s", track, entry.Name())
			versions = append(versions, version)
		}
		require.NotEmptyf(t, versions, "%s track must contain migrations", track)

		sort.Ints(versions)
		for index := 1; index < len(versions); index++ {
			require.NotEqualf(t, versions[index-1], versions[index],
				"%s track has duplicate version %d — golang-migrate file source Initialize fails the entire track on duplicates", track, versions[index])
		}
	}
}
