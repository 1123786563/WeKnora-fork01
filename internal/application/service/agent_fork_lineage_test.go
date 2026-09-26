package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable pins the
// migration↔projection alignment for T32 (#62): the lineage columns exist
// on both marketplace tables and the license registry table is present
// after the full migration stream (same pattern as the workbench
// notifications alignment test from #34).
func TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	var count int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_release_submissions WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_releases WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Exec(`SELECT 1 FROM agent_licenses`).Error)
}
