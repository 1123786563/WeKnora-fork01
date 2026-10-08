package repository

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestSemanticScopeEpochFanout(t *testing.T) {
	db := newSemanticSQLiteTestDB(t)
	for _, id := range []string{"owned", "shared", "unrelated", "deleted"} {
		require.NoError(t, db.Exec("INSERT INTO knowledge_bases(id,name,tenant_id,embedding_model_id,summary_model_id) VALUES(?,?,1,'','')", id, id).Error)
	}
	require.NoError(t, db.Exec("UPDATE knowledge_bases SET deleted_at=CURRENT_TIMESTAMP WHERE id='deleted'").Error)
	for _, id := range []string{"org-1", "org-2"} {
		require.NoError(t, db.Exec("INSERT INTO organizations(id,name,owner_id) VALUES(?,?,'owner')", id, id).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO organization_tenant_members(id,organization_id,tenant_id) VALUES('m','org-1',2)").Error)
	for _, row := range [][3]string{{"s1", "shared", "org-1"}, {"s2", "unrelated", "org-2"}, {"s3", "deleted", "org-1"}} {
		require.NoError(t, db.Exec("INSERT INTO kb_shares(id,knowledge_base_id,organization_id,shared_by_user_id,source_tenant_id) VALUES(?,?,?,'owner',1)", row[0], row[1], row[2]).Error)
	}
	r := NewSemanticControlRepository(db)
	ctx := context.Background()
	require.NoError(t, r.InvalidateTenant(ctx, 2))
	assertSemanticEpoch(t, db, 1, "shared", 1)
	assertRowCount(t, db, "semantic_access_epochs", 1)
	require.NoError(t, r.InvalidateOrganization(ctx, "org-1"))
	assertSemanticEpoch(t, db, 1, "shared", 2)
	require.NoError(t, r.InvalidateTenant(ctx, 1))
	assertSemanticEpoch(t, db, 1, "owned", 1)
	assertRowCount(t, db, "semantic_access_epochs", 3)
	require.NoError(t, r.BumpSemanticEpochs(ctx, []types.SemanticScopeKey{{TenantID: 1, KBID: "shared"}, {TenantID: 1, KBID: "owned"}, {TenantID: 1, KBID: "shared"}}))
	assertSemanticEpoch(t, db, 1, "shared", 4)
	assertSemanticEpoch(t, db, 1, "owned", 2)
}

func TestSemanticScopeEpochAtomicFailures(t *testing.T) {
	db := newSemanticSQLiteTestDB(t)
	r := NewSemanticControlRepository(db)
	ctx := context.Background()
	require.NoError(t, r.BumpSemanticEpochs(ctx, nil))
	require.Error(t, r.BumpSemanticEpochs(ctx, []types.SemanticScopeKey{{TenantID: 1, KBID: "a"}, {KBID: "b"}}))
	assertRowCount(t, db, "semantic_access_epochs", 0)
	require.NoError(t, db.Exec("INSERT INTO semantic_access_epochs(tenant_id,kb_id,epoch) VALUES('1','z',?)", strconv.FormatUint(math.MaxUint64, 10)).Error)
	require.ErrorIs(t, r.BumpSemanticEpochs(ctx, []types.SemanticScopeKey{{TenantID: 1, KBID: "a"}, {TenantID: 1, KBID: "z"}}), ErrSemanticEpochOverflow)
	assertRowCount(t, db, "semantic_access_epochs", 1)
	require.NoError(t, db.Exec("DROP TABLE semantic_access_epochs").Error)
	require.Error(t, r.InvalidateKB(ctx, 1, "a"))
}

func TestSemanticScopeEpochUserUnion(t *testing.T) {
	db := newSemanticSQLiteTestDB(t)
	ctx := context.Background()
	r := NewSemanticControlRepository(db)
	require.NoError(t, db.Exec("INSERT INTO knowledge_bases(id,name,tenant_id,embedding_model_id,summary_model_id) VALUES('owned','owned',1,'',''),('shared','shared',1,'',''),('other','other',3,'','')").Error)
	require.NoError(t, db.Exec("INSERT INTO tenant_members(user_id,tenant_id,role,status) VALUES('user',1,'viewer','active'),('user',2,'viewer','active'),('user',3,'viewer','suspended')").Error)
	require.NoError(t, db.Exec("INSERT INTO organizations(id,name,owner_id) VALUES('org','org','owner')").Error)
	require.NoError(t, db.Exec("INSERT INTO organization_tenant_members(id,organization_id,tenant_id) VALUES('m','org',2)").Error)
	require.NoError(t, db.Exec("INSERT INTO kb_shares(id,knowledge_base_id,organization_id,shared_by_user_id,source_tenant_id) VALUES('s','shared','org','owner',1)").Error)
	require.NoError(t, r.InvalidateUser(ctx, "user"))
	assertSemanticEpoch(t, db, 1, "owned", 1)
	assertSemanticEpoch(t, db, 1, "shared", 1)
	assertRowCount(t, db, "semantic_access_epochs", 2)
	require.Error(t, r.InvalidateUser(ctx, ""))
}
