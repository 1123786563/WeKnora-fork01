package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// T34 (#64) Task 1: 迁移↔投影对齐——两张撤回台账表必须由生产迁移轨道
// （migrations/sqlite 全量 Up，经 openRunTestDB）创建，列集与
// types.AgentReleaseRevocationEntity / AgentDependencyRevocationEntity 对齐。
func TestAgentSecurityRevocationTablesExistAfterMigrations(t *testing.T) {
	db := openRunTestDB(t)
	require.True(t, db.Migrator().HasTable("agent_release_revocations"),
		"agent_release_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "listing_id", "release_id", "reason",
		"replacement_release_id", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_release_revocations", column),
			"agent_release_revocations.%s 必须存在（与实体列对齐）", column)
	}
	require.True(t, db.Migrator().HasTable("agent_dependency_revocations"),
		"agent_dependency_revocations 必须由生产迁移创建")
	for _, column := range []string{"id", "tenant_id", "dep_type", "dep_id", "dep_version", "dep_digest",
		"reason", "replacement_version", "in_flight_disposition", "canceled_run_count", "revoked_by", "revoked_at", "created_at"} {
		require.Truef(t, db.Migrator().HasColumn("agent_dependency_revocations", column),
			"agent_dependency_revocations.%s 必须存在（与实体列对齐）", column)
	}
}
