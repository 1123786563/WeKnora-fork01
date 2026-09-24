package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

	adoptions, err := repo.ListAdoptions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, adoptions, 1)
	foreignAdoption, err := repo.GetAdoption(ctx, 2, adopted.ID)
	require.NoError(t, err)
	require.Nil(t, foreignAdoption)
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
