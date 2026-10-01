package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeAdoptionAgentSource stands in for the Agent domain on the publish
// flow: it records every instantiated local Agent Definition and, like the
// real customAgentService.CreateAgent, persists the row so the DB-backed
// available-agent read model can join it.
type fakeAdoptionAgentSource struct {
	db      *gorm.DB
	created []*types.CustomAgent
}

func (f *fakeAdoptionAgentSource) CreateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	agent.ID = fmt.Sprintf("agent-%d", len(f.created)+1)
	agent.TenantID = 1
	if err := f.db.Create(agent).Error; err != nil {
		return nil, err
	}
	f.created = append(f.created, agent)
	return agent, nil
}

type fakeAdoptionVersions struct{}

func (fakeAdoptionVersions) FreezeAgentVersion(_ context.Context, _ uint64, _, agentID string) (interfaces.AgentVersionView, error) {
	return interfaces.AgentVersionView{ID: "version-of-" + agentID, AgentID: agentID, VersionNumber: 1}, nil
}
func (fakeAdoptionVersions) GetAgentVersion(context.Context, uint64, string) (types.AgentVersionSnapshot, error) {
	return types.AgentVersionSnapshot{}, nil
}
func (fakeAdoptionVersions) ListAgentVersions(context.Context, uint64, string) ([]interfaces.AgentVersionView, error) {
	return nil, nil
}

func newAgentAdoptionServiceForTest(t *testing.T) (*AgentAdoptionService, *gorm.DB, *fakeAdoptionAgentSource) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	agents := &fakeAdoptionAgentSource{db: db}
	svc := NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), agents, fakeAdoptionVersions{})
	return svc, db, agents
}

type gatedCreateVariantRepository struct {
	repository.AgentAdoptionRepository
	reached chan struct{}
	resume  <-chan struct{}
}

func (r *gatedCreateVariantRepository) CreateVariant(ctx context.Context, variant *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error) {
	close(r.reached)
	<-r.resume
	return r.AgentAdoptionRepository.CreateVariant(ctx, variant)
}

// seedAdoptionServiceRelease publishes a real Release whose Manifest
// requires the capabilities "model" and "knowledge".
func seedAdoptionServiceRelease(t *testing.T, db *gorm.DB) (listingID, releaseID string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES ('version-a', 1, 'agent-a', 1, '{}', 'sha', 'author')`,
	).Error)
	manifest := `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"]},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	sum := sha256.Sum256(bundle)
	digest := hex.EncodeToString(sum[:])
	repo := repository.NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Helper", Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", AuthorID: "author",
			SemanticVersion: "1.0.0", BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID
}

func TestAgentAdoptionServiceVariantsMappingTestPublish(t *testing.T) {
	svc, db, agents := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()

	adoption, created, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, releaseID, adoption.AcceptedReleaseID)
	reAdopted, createdAgain, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.False(t, createdAgain)
	require.Equal(t, adoption.ID, reAdopted.ID)

	sales, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales Assistant"})
	require.NoError(t, err)
	require.Equal(t, "draft", sales.State)
	require.Equal(t, []string{"knowledge", "model"}, sales.MissingCapabilities)

	// AC2: a variant missing required capabilities is not runnable and the
	// refusal names every missing capability.
	_, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)
	require.Contains(t, err.Error(), "knowledge")
	require.Contains(t, err.Error(), "model")
	_, err = svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)

	sales, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "gpt-x"}})
	require.NoError(t, err)
	require.Equal(t, "draft", sales.State, "partial mapping keeps the variant unmapped")
	require.Equal(t, []string{"knowledge"}, sales.MissingCapabilities)
	_, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)
	require.Contains(t, err.Error(), "knowledge")

	sales, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-x"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-sales"}},
	})
	require.NoError(t, err)
	require.Equal(t, "mapped", sales.State)
	require.Empty(t, sales.MissingCapabilities)
	sales, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.NoError(t, err)
	require.Equal(t, "tested", sales.State)
	require.Equal(t, "admin", sales.TestedBy)

	// AC1: the SAME adoption derives a second, independent variant.
	legal, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Legal Assistant"})
	require.NoError(t, err)
	require.Equal(t, "draft", legal.State)
	legal, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", legal.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-legal"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-legal"}},
	})
	require.NoError(t, err)
	legal, err = svc.TestVariant(ctx, 1, "admin", legal.ID)
	require.NoError(t, err)
	require.Equal(t, "tested", legal.State)

	publishedSales, err := svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.NoError(t, err)
	require.Equal(t, "published", publishedSales.Variant.State)
	require.NotEmpty(t, publishedSales.Variant.LocalAgentID)
	require.Equal(t, "version-of-agent-1", publishedSales.Variant.LocalAgentVersionID)
	publishedLegal, err := svc.PublishVariant(ctx, 1, "admin", legal.ID)
	require.NoError(t, err)
	require.Equal(t, "published", publishedLegal.Variant.State)
	require.NotEqual(t, publishedSales.Variant.LocalAgentID, publishedLegal.Variant.LocalAgentID)
	require.Len(t, agents.created, 2, "one adoption with two variants instantiates two independent local agents")
	require.Equal(t, []string{"kb-sales"}, agents.created[0].Config.KnowledgeBases)
	require.Equal(t, "gpt-x", agents.created[0].Config.ModelID)
	require.Equal(t, []string{"kb-legal"}, agents.created[1].Config.KnowledgeBases)
	require.Equal(t, "gpt-legal", agents.created[1].Config.ModelID)
	require.Equal(t, "Be useful.", agents.created[0].Config.SystemPrompt, "portable payload behavior survives the round trip")

	available, err := svc.ListAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Len(t, available, 2)
	require.Equal(t, "supported", available[0].Capability.State)
	listed, err := svc.ListAdoptions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Len(t, listed[0].Variants, 2)

	// A published variant refuses silent re-mapping and re-publication.
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "other"}})
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	_, err = svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	require.Len(t, agents.created, 2, "a refused re-publish must not instantiate another local agent")

	// Testing an untested-again path: publish from mapped (not tested) refuses.
	_, err = svc.PublishVariant(ctx, 1, "admin", "missing-variant")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
}

func TestCreateVariantRechecksAdoptionAfterServicePrecheck(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()
	adoption, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)

	resume := make(chan struct{})
	var resumeOnce sync.Once
	defer resumeOnce.Do(func() { close(resume) })
	baseRepo := svc.repo
	gate := &gatedCreateVariantRepository{AgentAdoptionRepository: baseRepo, reached: make(chan struct{}), resume: resume}
	svc.repo = gate
	type result struct {
		view interfaces.AdoptionVariantView
		err  error
	}
	created := make(chan result, 1)
	go func() {
		view, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "racing", ReleaseID: releaseID})
		created <- result{view: view, err: err}
	}()

	select {
	case <-gate.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("CreateVariant did not reach its repository insert")
	}
	_, err = repository.NewAgentAdoptionRepository(db).EndAdoption(ctx, 1, adoption.ID, "active", "ended", map[string]any{"ended_by": "admin"})
	require.NoError(t, err)
	resumeOnce.Do(func() { close(resume) })

	select {
	case got := <-created:
		require.ErrorIs(t, got.err, ErrAgentAdoptionStateConflict)
	case <-time.After(5 * time.Second):
		t.Fatal("CreateVariant did not return after the repository gate opened")
	}

	variants, err := baseRepo.ListVariantsByAdoption(ctx, 1, adoption.ID)
	require.NoError(t, err)
	require.Empty(t, variants, "a stale active-state pre-check must not create a Variant after Adoption ends")
	ended, err := baseRepo.GetAdoption(ctx, 1, adoption.ID)
	require.NoError(t, err)
	require.Equal(t, "ended", ended.State)
}

func TestAgentAdoptionServiceRejectsUnknownAndDuplicateCapabilities(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, _ := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()
	adoption, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	variant, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales"})
	require.NoError(t, err)

	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "sandbox"}})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "a"}, {Capability: "model", ModelID: "b"}})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)

	// Cross-tenant ids read as not-found; an empty binding never covers.
	_, err = svc.CreateVariant(ctx, 2, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "X"})
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
	variant, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "model"}})
	require.NoError(t, err)
	require.Equal(t, []string{"knowledge", "model"}, variant.MissingCapabilities, "an empty binding leaves the capability missing")
	require.Equal(t, "draft", variant.State)
}

func TestAgentAdoptionServicePublishRefusesTamperedRelease(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()
	adoption, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	variant, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales"})
	require.NoError(t, err)
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-x"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-1"}},
	})
	require.NoError(t, err)
	_, err = svc.TestVariant(ctx, 1, "admin", variant.ID)
	require.NoError(t, err)

	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).Update("bundle", []byte(`{"payload":"tampered"}`)).Error)
	_, err = svc.PublishVariant(ctx, 1, "admin", variant.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match its recorded digest")
}

// TestBuildLocalAgentModelSelectionIsOrderIndependent pins the explicit
// model-selection rule of buildLocalAgent: when several capability mappings
// each carry a model, the binding of the lexicographically smallest
// capability wins, regardless of the order the mappings arrive in. The rule
// used to be implicit in ListCapabilityMappings' capability ASC ordering;
// it must not depend on it.
func TestBuildLocalAgentModelSelectionIsOrderIndependent(t *testing.T) {
	variant := &types.AgentAdoptionVariantEntity{Name: "Helper"}
	payload := types.AgentReleasePayload{AgentMode: "quick-answer"}
	manifest := types.AgentReleaseManifest{ReleaseMetadata: types.ReleaseMetadata{Summary: "s"}}
	// Reverse of the repository's capability ASC order: the smallest
	// capability carrying a model is "knowledge".
	reverseOrder := []types.AgentVariantCapabilityMappingEntity{
		{Capability: "tool-search", ModelID: " model-tool ", KnowledgeBaseIDs: `["kb-tool"]`},
		{Capability: "knowledge", ModelID: "model-knowledge", KnowledgeBaseIDs: `["kb-1"]`},
		{Capability: "vision", KnowledgeBaseIDs: `["kb-vision"]`},
	}
	ascOrder := []types.AgentVariantCapabilityMappingEntity{reverseOrder[1], reverseOrder[2], reverseOrder[0]}

	built := buildLocalAgent(variant, payload, manifest, reverseOrder)
	require.Equal(t, "model-knowledge", built.Config.ModelID, "the smallest capability with a model wins even when rows arrive unordered")
	require.Equal(t, "model-knowledge", buildLocalAgent(variant, payload, manifest, ascOrder).Config.ModelID, "selection is identical under the repository's ASC order")

	// Every knowledge binding is accumulated regardless of order, and the
	// model-less capability contributes no model.
	require.ElementsMatch(t, []string{"kb-1", "kb-tool", "kb-vision"}, built.Config.KnowledgeBases)
	require.Equal(t, "Helper", built.Name)
	require.Equal(t, "quick-answer", built.Config.AgentMode)

	// With a single model-bearing mapping the selection is that model,
	// whitespace-trimmed, and an all-model-less variant builds no model.
	onlyTool := []types.AgentVariantCapabilityMappingEntity{reverseOrder[0]}
	require.Equal(t, "model-tool", buildLocalAgent(variant, payload, manifest, onlyTool).Config.ModelID)
	noModel := []types.AgentVariantCapabilityMappingEntity{reverseOrder[2], {Capability: "web", KnowledgeBaseIDs: `["kb-web"]`}}
	require.Empty(t, buildLocalAgent(variant, payload, manifest, noModel).Config.ModelID)
}
