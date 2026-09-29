package database

import (
	"fmt"
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

	issue30SQLite := []string{
		"task_grants", "agent_adoption_variants", "public_agent_marketplace",
		"app_publications", "task_compliance", "code_deliveries",
		"mobile_device_app", "app_action_plans", "agent_upgrade_proposals",
		"task_research", "agent_fork_lineage", "space_connection_grants",
	}
	issue30Versioned := []string{
		"task_grants", "agent_adoption_variants", "public_agent_marketplace",
		"app_publications", "task_compliance", "code_deliveries",
		"mobile_device_app", "app_action_plans", "space_connection_grants",
		"agent_upgrade_proposals", "task_research", "agent_fork_lineage",
	}
	issue140 := []string{
		"career_profile", "artifact_version_revocation", "career_source_revisions",
		"career_opportunities", "career_evaluations", "workbench_application_tasks",
		"career_applications", "source_import_observations", "career_searches",
		"career_materials", "career_progress_events", "career_material_exports",
		"career_search_rules", "career_submissions", "career_exports_deletions",
		"career_preparations", "career_reminders", "career_usage", "career_reconciliations",
	}
	for _, spec := range []struct {
		track        string
		issue30Base  int
		issue140Base int
		issue30      []string
	}{
		{track: "sqlite", issue30Base: 112, issue140Base: 124, issue30: issue30SQLite},
		{track: "versioned", issue30Base: 191, issue140Base: 203, issue30: issue30Versioned},
	} {
		trackDir := filepath.Join(root, "migrations", spec.track)
		for i, name := range spec.issue30 {
			version := spec.issue30Base + i
			for _, direction := range []string{"up", "down"} {
				path := filepath.Join(trackDir, fmt.Sprintf("%06d_%s.%s.sql", version, name, direction))
				require.FileExistsf(t, path, "issue30 migration filename/version changed: %s", path)
			}
		}
		for i, name := range issue140 {
			version := spec.issue140Base + i
			for _, direction := range []string{"up", "down"} {
				path := filepath.Join(trackDir, fmt.Sprintf("%06d_%s.%s.sql", version, name, direction))
				require.FileExistsf(t, path, "#140 migration must preserve order at version %d: %s", version, path)
			}
		}
	}
}
