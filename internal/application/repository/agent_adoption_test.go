package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// seedAdoptionRelease drives the REAL marketplace repository to publish one
// immutable Release whose Manifest declares capability requirements, so the
// adoption rows under test bind to genuine FK-respecting marketplace data.
func seedAdoptionRelease(t *testing.T, db *gorm.DB, tenantID uint64, agentID, semanticVersion string) (listingID, releaseID string) {
	t.Helper()
	versionID := agentID + "-v1"
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, ?, ?, 1, '{}', 'sha', 'author')`,
		versionID, tenantID, agentID,
	).Error)
	manifest := `{"semantic_version":"` + semanticVersion + `","display_name":"Listing ` + agentID + `","summary":"s","supported_languages":["en"],"use_cases":["u"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"` + versionID + `","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"quick-answer","system_prompt":"p"},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	repo := NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: tenantID, SourceAgentID: agentID, DisplayName: "Listing " + agentID, Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: tenantID, AgentVersionID: versionID, SourceAgentID: agentID, AuthorID: "author",
			SemanticVersion: semanticVersion, BundleDigest: "digest-" + agentID,
			ManifestJSON: manifest, DependencyLockJSON: `{"dependencies":[]}`,
			Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), tenantID, "", submission.ID, "digest-"+agentID, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID
}

func TestAgentAdoptionRepositoryLifecycle(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-a", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()

	adopted, created, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, adopted.ID)
	again, createdAgain, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	require.False(t, createdAgain, "re-adopting the same listing is idempotent")
	require.Equal(t, adopted.ID, again.ID)

	// Marketplace read proxies are tenant-scoped: another tenant reads absence.
	listing, err := repo.GetMarketplaceListing(ctx, 1, listingID)
	require.NoError(t, err)
	require.NotNil(t, listing)
	listing, err = repo.GetMarketplaceListing(ctx, 2, listingID)
	require.NoError(t, err)
	require.Nil(t, listing)
	release, err := repo.GetRelease(ctx, 2, releaseID)
	require.NoError(t, err)
	require.Nil(t, release)

	variant, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Sales", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	secondVariant, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Legal", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	require.NotEqual(t, variant.ID, secondVariant.ID, "one adoption owns multiple independent variants")
	variants, err := repo.ListVariantsByAdoption(ctx, 1, adopted.ID)
	require.NoError(t, err)
	require.Len(t, variants, 2)
	stored, err := repo.GetVariant(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", stored.State)
	foreignVariant, err := repo.GetVariant(ctx, 2, variant.ID)
	require.NoError(t, err)
	require.Nil(t, foreignVariant)

	require.NoError(t, repo.ReplaceCapabilityMappings(ctx, 1, variant.ID, []types.AgentVariantCapabilityMappingEntity{
		{TenantID: 1, VariantID: variant.ID, Capability: "model", ModelID: "gpt-x", KnowledgeBaseIDs: "[]", ConnectionIDs: "[]", UpdatedBy: "admin"},
	}, "draft"))
	require.NoError(t, repo.ReplaceCapabilityMappings(ctx, 1, variant.ID, []types.AgentVariantCapabilityMappingEntity{
		{TenantID: 1, VariantID: variant.ID, Capability: "model", ModelID: "gpt-x", KnowledgeBaseIDs: "[]", ConnectionIDs: "[]", UpdatedBy: "admin"},
		{TenantID: 1, VariantID: variant.ID, Capability: "knowledge", KnowledgeBaseIDs: `["kb-1"]`, ConnectionIDs: "[]", UpdatedBy: "admin"},
	}, "mapped"))
	rows, err := repo.ListCapabilityMappings(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "replacement must not accumulate stale rows")
	stored, err = repo.GetVariant(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Equal(t, "mapped", stored.State)
	err = repo.ReplaceCapabilityMappings(ctx, 1, "missing-variant", nil, "draft")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound, "replacing mappings of an unknown variant must fail as not-found")

	tested, err := repo.UpdateVariantState(ctx, 1, variant.ID, []string{"mapped"}, "tested", map[string]any{"tested_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "tested", tested.State)
	require.Equal(t, "admin", tested.TestedBy)
	require.NotNil(t, tested.TestedAt)
	_, err = repo.UpdateVariantState(ctx, 1, variant.ID, []string{"mapped"}, "tested", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantTransition, "a stale from-state must refuse the transition")
	_, err = repo.UpdateVariantState(ctx, 1, "missing-variant", []string{"mapped"}, "tested", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
	published, err := repo.UpdateVariantState(ctx, 1, variant.ID, []string{"tested"}, "published", map[string]any{"local_agent_id": "published-agent", "local_agent_version_id": "published-version", "published_by": "admin"})
	require.NoError(t, err)
	_, err = repo.UpdateVariantState(ctx, 1, published.ID, []string{"published"}, "published", map[string]any{"local_agent_version_id": "rewritten-version"})
	require.ErrorIs(t, err, ErrAgentAdoptionVariantIdentityImmutable)

	adoptions, err := repo.ListAdoptions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, adoptions, 1)
	foreignAdoption, err := repo.GetAdoption(ctx, 2, adopted.ID)
	require.NoError(t, err)
	require.Nil(t, foreignAdoption)
}

func TestAgentAdoptionRepositoryReAdoptCannotReviveEndedAdoption(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-ended", "1.0.0")
	secondReleaseID := "agent-ended-release-2"
	require.NoError(t, db.Exec(
		`INSERT INTO agent_releases (id, tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, published_by)
		 SELECT ?, tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number + 1, '2.0.0', bundle_digest || '-r2', manifest_json, dependency_lock_json, bundle, published_by
		 FROM agent_releases WHERE tenant_id = ? AND id = ?`,
		secondReleaseID, 1, releaseID,
	).Error)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	adoption, created, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)
	require.True(t, created)
	_, err = repo.EndAdoption(ctx, 1, adoption.ID, "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)

	for _, acceptedReleaseID := range []string{releaseID, secondReleaseID} {
		_, _, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
			TenantID: 1, ListingID: listingID, AcceptedReleaseID: acceptedReleaseID, State: "active", CreatedBy: "admin",
		})
		require.ErrorIs(t, err, ErrAgentAdoptionTransition, "ended Adoption must reject same- and different-release re-adoption")
	}
	stored, err := repo.GetAdoption(ctx, 1, adoption.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "ended", stored.State)
	require.Equal(t, releaseID, stored.AcceptedReleaseID, "re-adoption must not mutate the archived accepted pointer")
}

func TestAgentAdoptionRepositoryConflictLoserRejectsEndedWinner(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-ended-winner", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	var injected bool
	var injectErr error
	callbackName := "test:inject_ended_adoption_winner:" + t.Name()
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != "agent_adoptions" {
			return
		}
		injected = true
		_, injectErr = tx.Statement.ConnPool.ExecContext(tx.Statement.Context,
			`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by) VALUES ('ended-winner', 1, ?, ?, 'ended', 'prior-admin')`,
			listingID, releaseID,
		)
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })

	_, created, err := repo.AdoptListing(context.Background(), &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "new-admin",
	})
	require.True(t, injected)
	require.NoError(t, injectErr)
	require.ErrorIs(t, err, ErrAgentAdoptionTransition, "the unique-index loser must not report an ended winner as idempotent success")
	require.False(t, created)
	var count int64
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ? AND listing_id = ?", 1, listingID).Count(&count).Error)
	require.Zero(t, count, "the simulated winner and losing request share a transaction; refusal rolls both back")
}

func TestSQLiteScopeNoOpAdoptionUpdateReportsMatchedRow(t *testing.T) {
	db := openLifecycleRaceDB(t)
	listingID, _ := seedLifecycleRaceAdoption(t, db)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	result := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND listing_id = ?", 1, listingID).
		UpdateColumn("updated_at", gorm.Expr("updated_at"))
	require.NoError(t, result.Error)
	require.EqualValues(t, 1, result.RowsAffected, "SQLite must report the matched scope row for the lock-first no-op UPDATE")
}

func TestAdoptListingLockSerializesAgainstEndAdoption(t *testing.T) {
	db := openLifecycleRaceDB(t)
	listingID, releaseID := seedLifecycleRaceAdoption(t, db)
	repo := NewAgentAdoptionRepository(db)
	adoptBarrier := registerAdoptionScopeGuardBarrier(t, db, true)
	endBarrier := registerLifecycleGuardBarrier(t, db, "end", false, false)
	adoptDone := make(chan error, 1)
	go func() {
		_, _, err := repo.AdoptListing(context.WithValue(context.Background(), lifecycleLockTestContextKey{}, "adopt"), &types.AgentAdoptionEntity{
			TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin",
		})
		adoptDone <- err
	}()
	waitForLifecycleBarrier(t, adoptBarrier, adoptDone, "AdoptListing")
	endDone := make(chan error, 1)
	go func() {
		_, err := repo.EndAdoption(context.WithValue(context.Background(), lifecycleLockTestContextKey{}, "end"), 1, "race-adoption", "active", "ended", map[string]any{"ended_by": "admin"})
		endDone <- err
	}()
	waitForLifecycleBarrier(t, endBarrier, endDone, "EndAdoption")

	// Adopt's actual scope UPDATE has executed while its transaction is held;
	// End has reached its guarded UPDATE callback. Releasing Adopt fixes the
	// intended order without sleeps. The outcome must retain active adoption
	// semantics first and then permit the terminal End transition.
	adoptBarrier.unblock()
	require.NoError(t, awaitLifecycleOperation(t, adoptDone, "AdoptListing"))
	endBarrier.unblock()
	require.NoError(t, awaitLifecycleOperation(t, endDone, "EndAdoption"))
	stored, err := repo.GetAdoption(context.Background(), 1, "race-adoption")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "ended", stored.State)
	require.Equal(t, releaseID, stored.AcceptedReleaseID)
}

func registerAdoptionScopeGuardBarrier(t *testing.T, db *gorm.DB, hold bool) *lifecycleCallbackBarrier {
	t.Helper()
	barrier := newLifecycleCallbackBarrier()
	name := "test:adoption-scope-guard:" + fmt.Sprintf("%p", barrier)
	err := db.Callback().Update().After("gorm:update").Before("gorm:after_update").Register(name, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "agent_adoptions" || tx.Statement.Context.Value(lifecycleLockTestContextKey{}) != "adopt" {
			return
		}
		barrier.arrive(hold)
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Callback().Update().Remove(name) })
	t.Cleanup(barrier.unblock)
	return barrier
}

func TestAgentAdoptionPublishedAvailableAgentsJoinsLocalAgents(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-b", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	adopted, _, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	published, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Sales", State: "published", LocalAgentID: "agent-sales", CreatedBy: "admin"})
	require.NoError(t, err)

	rows, err := repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, rows, "a published variant whose local agent row is absent yields no available agent")

	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-sales", TenantID: 1, Name: "Sales Assistant", CreatedBy: "admin", Config: types.CustomAgentConfig{AgentMode: "quick-answer"}}).Error)
	rows, err = repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, published.ID, rows[0].Variant.ID)
	require.NotNil(t, rows[0].Agent)
	require.Equal(t, "agent-sales", rows[0].Agent.ID)

	require.NoError(t, db.Delete(&types.CustomAgent{}, "tenant_id = ? AND id = ?", 1, "agent-sales").Error)
	rows, err = repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, rows, "a soft-deleted local agent disappears from the read model")

	rows, err = repo.PublishedAvailableAgents(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, rows, "another tenant never sees tenant-1 availability")
}

// TestAgentAdoptionRepositoryAdoptListingLostRaceConverges covers the unique
// conflict path in AdoptListing. A one-shot query callback inserts a winner
// on the current transaction after the scope miss-read and before the
// attempted insert. Injecting on the transaction connection keeps this case
// deterministic under the lock-first SQLite transaction.
func TestAgentAdoptionRepositoryAdoptListingLostRaceConverges(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-race", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()

	var injected bool
	var injectErr error
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:inject_racing_adoption", func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != "agent_adoptions" {
			return
		}
		injected = true
		_, injectErr = tx.Statement.ConnPool.ExecContext(tx.Statement.Context,
			`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by) VALUES ('winner', 1, ?, ?, 'active', 'other-admin')`,
			listingID, releaseID,
		)
	}))
	defer db.Callback().Query().Remove("test:inject_racing_adoption")

	adoptive := &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"}
	adopted, created, err := repo.AdoptListing(ctx, adoptive)
	require.NoError(t, injectErr)
	require.True(t, injected, "the test must have injected the racing winner between miss-read and INSERT")
	require.NoError(t, err, "the loser of a concurrent first adopt must converge, not surface the unique-index error")
	require.False(t, created, "the racing loser did not create the row")
	require.Equal(t, "winner", adopted.ID, "the loser reuses the winner's row")
	require.Equal(t, releaseID, adopted.AcceptedReleaseID)
	require.Equal(t, "active", adopted.State)

	// The converged row keeps sequential semantics afterwards.
	again, createdAgain, err := repo.AdoptListing(ctx, adoptive)
	require.NoError(t, err)
	require.False(t, createdAgain)
	require.Equal(t, "winner", again.ID)
}

// TestAgentAdoptionRepositoryAdoptListingLostRaceDifferentRelease verifies
// the conflict loser accepting a different release still advances the accepted
// pointer on the active winner's row — the same last-write-wins rule as a sequential
// re-adopt (spec §8 step 3 "建立或更新 Adoption").
func TestAgentAdoptionRepositoryAdoptListingLostRaceDifferentRelease(t *testing.T) {
	db := openRunTestDB(t)
	listingID, winnerRelease := seedAdoptionRelease(t, db, 1, "agent-race-b", "1.0.0")
	// A second, later release on the same listing for the loser to accept,
	// cloned from the winner release to keep every FK linkage valid. The
	// digest and semantic version change to satisfy the (listing, digest) and
	// (listing, semantic_version) unique indexes.
	require.NoError(t, db.Exec(
		`INSERT INTO agent_releases (id, tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number, semantic_version, bundle_digest, manifest_json, dependency_lock_json, bundle, published_by)
		 SELECT 'loser-release', tenant_id, listing_id, submission_id, agent_version_id, source_agent_id, release_number + 1, '2.0.0', bundle_digest || '-r2', manifest_json, dependency_lock_json, bundle, published_by
		 FROM agent_releases WHERE tenant_id = 1 AND id = ?`,
		winnerRelease,
	).Error)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()

	var injected bool
	var injectErr error
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:inject_racing_adoption_b", func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != "agent_adoptions" {
			return
		}
		injected = true
		_, injectErr = tx.Statement.ConnPool.ExecContext(tx.Statement.Context,
			`INSERT INTO agent_adoptions (id, tenant_id, listing_id, accepted_release_id, state, created_by) VALUES ('winner', 1, ?, ?, 'active', 'other-admin')`,
			listingID, winnerRelease,
		)
	}))
	defer db.Callback().Query().Remove("test:inject_racing_adoption_b")

	adopted, created, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: "loser-release", State: "active", CreatedBy: "admin"})
	require.NoError(t, injectErr)
	require.True(t, injected)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, "winner", adopted.ID)
	require.Equal(t, "loser-release", adopted.AcceptedReleaseID, "the racing loser advances the accepted pointer on the winner's row")

	stored, err := repo.GetAdoption(ctx, 1, "winner")
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, "loser-release", stored.AcceptedReleaseID, "the pointer advance is persisted")
}

// openAdoptionVariantDB builds only the tables ReplaceCapabilityMappings
// touches (gorm AutoMigrate of the entities). The B3-F87 CAS guard under
// test is store-level SQL semantics, so the case keeps its direct-DDL setup.
// It originally sidestepped the full migration track because this table held
// both migrations/sqlite 000112 and migrations/versioned 000191 (duplicate
// numbers against task_grants); that conflict is fixed by renumbering
// agent_adoption_variants to 000113/000192, and the direct-DDL approach
// stays unchanged.
func openAdoptionVariantDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.AgentAdoptionEntity{}, &types.AgentAdoptionVariantEntity{}, &types.AgentVariantCapabilityMappingEntity{}))
	// AutoMigrate 不创建 uq_agent_adoptions_scope（实体无 uniqueIndex tag）；
	// 显式补建使 adoptListingTx 的 OnConflict 竞态分支由真实唯一索引驱动，
	// 与迁移 000113/000114 的生产 DDL 一致。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id)").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestReplaceCapabilityMappingsRefusesToDowngradeConcurrentState(t *testing.T) {
	db := openAdoptionVariantDB(t)
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	// 夹具：variant 处于 published（模拟并发 PublishVariant 的 CAS 恰好落地）。
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		TenantID: 1, ID: "v1", AdoptionID: "a1", ReleaseID: "r1", State: "published", Name: "V",
	}).Error)

	err := repo.ReplaceCapabilityMappings(ctx, 1, "v1",
		[]types.AgentVariantCapabilityMappingEntity{{TenantID: 1, VariantID: "v1", Capability: "model", ModelID: "m1"}}, "mapped")

	require.ErrorIs(t, err, ErrAgentAdoptionRemapStateConflict, "并发 CAS 落地后不得被静默打回（B3-F87）")
	after, err := repo.GetVariant(ctx, 1, "v1")
	require.NoError(t, err)
	require.NotNil(t, after)
	require.Equal(t, "published", after.State, "published 状态必须保持")
}

func TestAgentAdoptionRepositoryResolvesIntroducedListingAndRelease(t *testing.T) {
	db := openAdoptionVariantDB(t)
	require.NoError(t, db.AutoMigrate(&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseEntity{}, &types.TenantIntroducedReleaseEntity{}))
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	bundle := []byte(`{"payload":{"system_prompt":"portable"}}`)

	// 引入台账：tenant 2 引入了 public listing 的一个 release
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "introduced-r1", TenantID: 2, PublicListingID: "pub-listing-1", PublicReleaseID: "pub-release-1",
		DisplayName: "Public helper", Summary: "s", SemanticVersion: "1.0.0", BundleDigest: "d1",
		ManifestJSON: `{"capability_requirements":["knowledge"]}`, DependencyLockJSON: `{"dependencies":[]}`,
		Bundle: bundle, IntroducedBy: "admin-2",
	}).Error)

	listing, err := repo.GetMarketplaceListing(ctx, 2, "pub-listing-1")
	require.NoError(t, err)
	require.NotNil(t, listing, "引入台账应合成为可采用的 listing")
	require.Equal(t, "listed", listing.State)
	require.NotNil(t, listing.CurrentReleaseID)
	require.Equal(t, "introduced-r1", *listing.CurrentReleaseID)

	release, err := repo.GetRelease(ctx, 2, "introduced-r1")
	require.NoError(t, err)
	require.NotNil(t, release, "引入台账应合成为可读的 release")
	require.Equal(t, "pub-listing-1", release.ListingID)
	require.Equal(t, bundle, release.Bundle)
	require.Equal(t, `{"capability_requirements":["knowledge"]}`, release.ManifestJSON)

	// 本地真实行优先：同 id 存在本地 release 时不得走回退
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "introduced-r1", TenantID: 2, ListingID: "local-listing", SubmissionID: "s1",
		AgentVersionID: "v1", SourceAgentID: "a1", ReleaseNumber: 1, SemanticVersion: "9.9.9",
		BundleDigest: "d1", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("local"),
	}).Error)
	local, err := repo.GetRelease(ctx, 2, "introduced-r1")
	require.NoError(t, err)
	require.Equal(t, "local-listing", local.ListingID)

	// 租户隔离：tenant 1 看不到 tenant 2 的引入
	other, err := repo.GetMarketplaceListing(ctx, 1, "pub-listing-1")
	require.NoError(t, err)
	require.Nil(t, other)
	otherRelease, err := repo.GetRelease(ctx, 1, "introduced-r1")
	require.NoError(t, err)
	require.Nil(t, otherRelease)
}
