package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentAdoptionInvalidInput       = errors.New("invalid agent adoption request")
	ErrAgentAdoptionNotFound           = errors.New("agent adoption resource not found")
	ErrAgentAdoptionVariantNotRunnable = errors.New("agent adoption variant is not runnable")
	ErrAgentAdoptionStateConflict      = errors.New("agent adoption variant state conflict")
)

// Variant lifecycle states (CONTEXT.md「Agent 变体」「可用 Agent」; the spec's
// "mapping-required" is state draft with a non-empty missing_capabilities).
// retire belongs to #63 and is deliberately absent.
const (
	AgentVariantStateDraft     = "draft"
	AgentVariantStateMapped    = "mapped"
	AgentVariantStateTested    = "tested"
	AgentVariantStatePublished = "published"
)

// AgentAdoptionStateActive is the only Adoption state in this ticket;
// end_adoption belongs to #63.
const AgentAdoptionStateActive = "active"

// AdoptionAgentSource is the Agent-domain seam the publish flow needs:
// instantiating the Variant's local Agent Definition.
// interfaces.CustomAgentService satisfies it structurally.
type AdoptionAgentSource interface {
	CreateAgent(ctx context.Context, agent *types.CustomAgent) (*types.CustomAgent, error)
}

type AgentAdoptionService struct {
	repo                repository.AgentAdoptionRepository
	agents              AdoptionAgentSource
	versions            interfaces.AgentVersionService
	now                 func() time.Time
	releaseSecurityGate ReleaseSecurityGate
}

// SetReleaseSecurityGate installs the optional security admission check used
// before a release is newly adopted or materialized as a local variant.
func (s *AgentAdoptionService) SetReleaseSecurityGate(gate ReleaseSecurityGate) {
	s.releaseSecurityGate = gate
}

var _ interfaces.AgentAdoptionService = (*AgentAdoptionService)(nil)

func NewAgentAdoptionService(repo repository.AgentAdoptionRepository, agents AdoptionAgentSource, versions interfaces.AgentVersionService) *AgentAdoptionService {
	return &AgentAdoptionService{repo: repo, agents: agents, versions: versions, now: time.Now}
}

func (s *AgentAdoptionService) Adopt(ctx context.Context, tenantID uint64, actorID string, input interfaces.AdoptInput) (interfaces.AdoptionView, bool, error) {
	input.ListingID, input.ReleaseID = strings.TrimSpace(input.ListingID), strings.TrimSpace(input.ReleaseID)
	actorID = strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || input.ListingID == "" {
		return interfaces.AdoptionView{}, false, ErrAgentAdoptionInvalidInput
	}
	listing, err := s.repo.GetMarketplaceListing(ctx, tenantID, input.ListingID)
	if err != nil {
		return interfaces.AdoptionView{}, false, err
	}
	if listing == nil {
		return interfaces.AdoptionView{}, false, ErrAgentAdoptionNotFound
	}
	if listing.State != "listed" || listing.CurrentReleaseID == nil {
		return interfaces.AdoptionView{}, false, fmt.Errorf("%w: listing %s has no adoptable release", ErrAgentAdoptionInvalidInput, listing.ID)
	}
	releaseID := input.ReleaseID
	if releaseID == "" {
		releaseID = *listing.CurrentReleaseID
	}
	release, err := s.repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return interfaces.AdoptionView{}, false, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.AdoptionView{}, false, fmt.Errorf("%w: release does not belong to listing", ErrAgentAdoptionInvalidInput)
	}
	if s.releaseSecurityGate != nil {
		if err := s.releaseSecurityGate.ReleaseAdmission(ctx, tenantID, releaseID); err != nil {
			return interfaces.AdoptionView{}, false, err
		}
	}
	if release.DeprecatedAt != nil {
		return interfaces.AdoptionView{}, false, fmt.Errorf("%w: %w: release %s is deprecated; successor: %s", ErrAgentReleaseDeprecated, ErrAgentAdoptionStateConflict, releaseID, successorHint(release.SuccessorReleaseID))
	}
	row, created, err := s.repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
		TenantID: tenantID, ListingID: listing.ID, AcceptedReleaseID: releaseID,
		State: AgentAdoptionStateActive, CreatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrAgentMarketplaceListingUnavailable) {
			return interfaces.AdoptionView{}, false, fmt.Errorf("%w: listing is no longer available for adoption", ErrAgentAdoptionStateConflict)
		}
		if errors.Is(err, repository.ErrAgentMarketplaceReleaseDeprecated) {
			return interfaces.AdoptionView{}, false, fmt.Errorf("%w: %w: release is no longer eligible", ErrAgentReleaseDeprecated, ErrAgentAdoptionStateConflict)
		}
		return interfaces.AdoptionView{}, false, err
	}
	view, err := s.adoptionView(ctx, tenantID, row)
	return view, created, err
}

func (s *AgentAdoptionService) ListAdoptions(ctx context.Context, tenantID uint64) ([]interfaces.AdoptionView, error) {
	rows, err := s.repo.ListAdoptions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AdoptionView, 0, len(rows))
	for i := range rows {
		view, err := s.adoptionView(ctx, tenantID, &rows[i])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AgentAdoptionService) CreateVariant(ctx context.Context, tenantID uint64, actorID, adoptionID string, input interfaces.VariantDraftInput) (interfaces.AdoptionVariantView, error) {
	adoptionID, input.Name, input.ReleaseID = strings.TrimSpace(adoptionID), strings.TrimSpace(input.Name), strings.TrimSpace(input.ReleaseID)
	actorID = strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || adoptionID == "" || input.Name == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	adoption, err := s.repo.GetAdoption(ctx, tenantID, adoptionID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if adoption == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	if adoption.State != AgentAdoptionStateActive {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: adoption state is %q", ErrAgentAdoptionStateConflict, adoption.State)
	}
	releaseID := input.ReleaseID
	if releaseID == "" {
		releaseID = adoption.AcceptedReleaseID
	}
	release, err := s.repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if release == nil || release.ListingID != adoption.ListingID {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: release does not belong to the adopted listing", ErrAgentAdoptionInvalidInput)
	}
	if s.releaseSecurityGate != nil {
		if err := s.releaseSecurityGate.ReleaseAdmission(ctx, tenantID, releaseID); err != nil {
			return interfaces.AdoptionVariantView{}, err
		}
	}
	if release.DeprecatedAt != nil {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: %w: release %s is deprecated; successor: %s", ErrAgentReleaseDeprecated, ErrAgentAdoptionStateConflict, releaseID, successorHint(release.SuccessorReleaseID))
	}
	created, err := s.repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: tenantID, AdoptionID: adoption.ID, ReleaseID: releaseID,
		Name: input.Name, State: AgentVariantStateDraft, CreatedBy: actorID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrAgentMarketplaceListingUnavailable) {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: listing is no longer available for new variants", ErrAgentAdoptionStateConflict)
		}
		if errors.Is(err, repository.ErrAgentMarketplaceReleaseDeprecated) {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: %w: release is no longer eligible", ErrAgentReleaseDeprecated, ErrAgentAdoptionStateConflict)
		}
		if errors.Is(err, repository.ErrAgentAdoptionTransition) {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: %w", ErrAgentAdoptionStateConflict, err)
		}
		if errors.Is(err, repository.ErrAgentAdoptionNotFound) {
			return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
		}
		return interfaces.AdoptionVariantView{}, err
	}
	return s.variantView(ctx, tenantID, created)
}

func (s *AgentAdoptionService) UpdateCapabilityMapping(ctx context.Context, tenantID uint64, actorID, variantID string, mappings []interfaces.CapabilityMapping) (interfaces.AdoptionVariantView, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	variant, err := s.repo.GetVariant(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if variant == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	switch variant.State {
	case AgentVariantStateDraft, AgentVariantStateMapped, AgentVariantStateTested:
	default:
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: state %q cannot be re-mapped", ErrAgentAdoptionStateConflict, variant.State)
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	required := make(map[string]bool, len(manifest.CapabilityRequirements))
	for _, capability := range manifest.CapabilityRequirements {
		required[capability] = true
	}
	rows := make([]types.AgentVariantCapabilityMappingEntity, 0, len(mappings))
	seen := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		capability := strings.TrimSpace(mapping.Capability)
		if capability == "" || !required[capability] {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: capability %q is not required by release %s", ErrAgentAdoptionInvalidInput, mapping.Capability, variant.ReleaseID)
		}
		if seen[capability] {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: duplicate mapping for capability %q", ErrAgentAdoptionInvalidInput, capability)
		}
		seen[capability] = true
		rows = append(rows, types.AgentVariantCapabilityMappingEntity{
			TenantID: tenantID, VariantID: variant.ID, Capability: capability,
			ModelID:          strings.TrimSpace(mapping.ModelID),
			KnowledgeBaseIDs: encodeIDList(mapping.KnowledgeBaseIDs),
			ConnectionIDs:    encodeIDList(mapping.ConnectionIDs),
			UpdatedBy:        actorID,
		})
	}
	nextState := AgentVariantStateDraft
	if len(missingCapabilities(manifest.CapabilityRequirements, rows)) == 0 {
		nextState = AgentVariantStateMapped
	}
	if err := s.repo.ReplaceCapabilityMappings(ctx, tenantID, variant.ID, rows, nextState); err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	updated, err := s.repo.GetVariant(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if updated == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	return s.variantView(ctx, tenantID, updated)
}

func (s *AgentAdoptionService) TestVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.AdoptionVariantView, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	variant, missing, err := s.variantWithMissing(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if variant == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	if len(missing) > 0 {
		return interfaces.AdoptionVariantView{}, notRunnable(missing)
	}
	if variant.State != AgentVariantStateMapped {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: state is %q; complete capability mapping first", ErrAgentAdoptionStateConflict, variant.State)
	}
	testedAt := s.now().UTC()
	updated, err := s.repo.UpdateVariantState(ctx, tenantID, variant.ID, []string{AgentVariantStateMapped}, AgentVariantStateTested, map[string]any{
		"tested_by": actorID, "tested_at": testedAt,
	})
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return s.variantView(ctx, tenantID, updated)
}

func (s *AgentAdoptionService) PublishVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.PublishVariantResult, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionInvalidInput
	}
	variant, missing, err := s.variantWithMissing(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	if variant == nil {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionNotFound
	}
	if s.releaseSecurityGate != nil {
		if err := s.releaseSecurityGate.ReleaseAdmission(ctx, tenantID, variant.ReleaseID); err != nil {
			return interfaces.PublishVariantResult{}, err
		}
	}
	if len(missing) > 0 {
		return interfaces.PublishVariantResult{}, notRunnable(missing)
	}
	if variant.State != AgentVariantStateTested {
		return interfaces.PublishVariantResult{}, fmt.Errorf("%w: state is %q; complete capability mapping and test first", ErrAgentAdoptionStateConflict, variant.State)
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	release, err := s.repo.GetRelease(ctx, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	if release == nil {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionNotFound
	}
	sum := sha256.Sum256(release.Bundle)
	if hex.EncodeToString(sum[:]) != release.BundleDigest {
		return interfaces.PublishVariantResult{}, fmt.Errorf("release bundle does not match its recorded digest")
	}
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(release.Bundle, &envelope); err != nil {
		return interfaces.PublishVariantResult{}, fmt.Errorf("decode release bundle payload: %w", err)
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	created, err := s.agents.CreateAgent(ctx, buildLocalAgent(variant, envelope.Payload, *manifest, mappings))
	if err != nil {
		return interfaces.PublishVariantResult{}, fmt.Errorf("create the variant local agent: %w", err)
	}
	view, err := s.versions.FreezeAgentVersion(ctx, tenantID, actorID, created.ID)
	if err != nil {
		// Known architectural limitation (parked, controller ruling): the
		// local agent above is already instantiated; this failure leaves it
		// orphaned while the variant stays tested. Log the id so a later
		// reconciliation pass can find it.
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"variant_id": variant.ID, "tenant_id": tenantID,
			"local_agent_id": created.ID,
			"stage":          "freeze_variant_local_agent_version",
		})
		return interfaces.PublishVariantResult{}, fmt.Errorf("freeze the variant local agent version: %w", err)
	}
	publishedAt := s.now().UTC()
	updated, err := s.repo.UpdateVariantState(ctx, tenantID, variant.ID, []string{AgentVariantStateTested}, AgentVariantStatePublished, map[string]any{
		"local_agent_id": created.ID, "local_agent_version_id": view.ID,
		"published_by": actorID, "published_at": publishedAt,
	})
	if err != nil {
		// Known architectural limitation (parked, controller ruling): the
		// CAS to published failed after the local agent and its frozen
		// version exist; retrying would instantiate another agent. Log the
		// ids so a later reconciliation pass can find the orphan.
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"variant_id": variant.ID, "tenant_id": tenantID,
			"local_agent_id": created.ID, "local_agent_version_id": view.ID,
			"stage": "cas_variant_to_published",
		})
		return interfaces.PublishVariantResult{}, err
	}
	variantView, err := s.variantView(ctx, tenantID, updated)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	return interfaces.PublishVariantResult{Variant: variantView}, nil
}

func (s *AgentAdoptionService) ListAvailableAgents(ctx context.Context, tenantID uint64) ([]interfaces.AvailableAgentView, error) {
	if tenantID == 0 {
		return nil, ErrAgentAdoptionInvalidInput
	}
	rows, err := s.repo.PublishedAvailableAgents(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AvailableAgentView, 0, len(rows))
	for _, row := range rows {
		if row.Agent == nil {
			continue // the local agent row is gone: the variant simply is not available
		}
		views = append(views, interfaces.AvailableAgentView{
			AgentID:     row.Agent.ID,
			VariantID:   row.Variant.ID,
			AdoptionID:  row.Variant.AdoptionID,
			ReleaseID:   row.Variant.ReleaseID,
			Name:        row.Agent.Name,
			Description: row.Agent.Description,
			IsBuiltin:   row.Agent.IsBuiltin,
			Capability:  interfaces.AgentCapabilityVerdict{State: "supported", Reason: ""},
		})
	}
	return views, nil
}

// notRunnable builds the explicit refusal: every missing capability is
// named, sorted (missingCapabilities already sorts).
func notRunnable(missing []string) error {
	return fmt.Errorf("%w: missing required capabilities: %s", ErrAgentAdoptionVariantNotRunnable, strings.Join(missing, ", "))
}

func successorHint(releaseID string) string {
	if releaseID = strings.TrimSpace(releaseID); releaseID != "" {
		return releaseID
	}
	return "none declared"
}

func (s *AgentAdoptionService) adoptionView(ctx context.Context, tenantID uint64, row *types.AgentAdoptionEntity) (interfaces.AdoptionView, error) {
	view := interfaces.AdoptionView{AgentAdoptionEntity: *row, Variants: []interfaces.AdoptionVariantView{}}
	variants, err := s.repo.ListVariantsByAdoption(ctx, tenantID, row.ID)
	if err != nil {
		return interfaces.AdoptionView{}, err
	}
	for i := range variants {
		variant, err := s.variantView(ctx, tenantID, &variants[i])
		if err != nil {
			return interfaces.AdoptionView{}, err
		}
		view.Variants = append(view.Variants, variant)
	}
	return view, nil
}

func (s *AgentAdoptionService) variantView(ctx context.Context, tenantID uint64, variant *types.AgentAdoptionVariantEntity) (interfaces.AdoptionVariantView, error) {
	missing, err := s.missingCapabilitiesOf(ctx, tenantID, variant)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return interfaces.AdoptionVariantView{AgentAdoptionVariantEntity: *variant, MissingCapabilities: missing}, nil
}

func (s *AgentAdoptionService) variantWithMissing(ctx context.Context, tenantID uint64, variantID string) (*types.AgentAdoptionVariantEntity, []string, error) {
	variant, err := s.repo.GetVariant(ctx, tenantID, variantID)
	if err != nil || variant == nil {
		return nil, nil, err
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, nil, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return nil, nil, err
	}
	return variant, missingCapabilities(manifest.CapabilityRequirements, mappings), nil
}

func (s *AgentAdoptionService) missingCapabilitiesOf(ctx context.Context, tenantID uint64, variant *types.AgentAdoptionVariantEntity) ([]string, error) {
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return nil, err
	}
	return missingCapabilities(manifest.CapabilityRequirements, mappings), nil
}

type adoptionReleaseReader interface {
	GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error)
}

func releaseManifest(ctx context.Context, repo adoptionReleaseReader, tenantID uint64, releaseID string) (*types.AgentReleaseManifest, error) {
	release, err := repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, ErrAgentAdoptionNotFound
	}
	var manifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &manifest); err != nil {
		return nil, fmt.Errorf("decode release manifest: %w", err)
	}
	return &manifest, nil
}

// missingCapabilities computes the Manifest capability requirements that no
// stored mapping binds to a local resource. Sorting keeps the refusal
// message deterministic.
func missingCapabilities(requirements []string, mappings []types.AgentVariantCapabilityMappingEntity) []string {
	bound := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		if capabilityBound(mapping) {
			bound[mapping.Capability] = true
		}
	}
	missing := make([]string, 0, len(requirements))
	for _, capability := range requirements {
		if !bound[capability] {
			missing = append(missing, capability)
		}
	}
	sort.Strings(missing)
	return missing
}

func capabilityBound(mapping types.AgentVariantCapabilityMappingEntity) bool {
	if strings.TrimSpace(mapping.ModelID) != "" {
		return true
	}
	return len(decodeIDList(mapping.KnowledgeBaseIDs)) > 0 || len(decodeIDList(mapping.ConnectionIDs)) > 0
}

func decodeIDList(encoded string) []string {
	ids := []string{}
	// A malformed payload degrades to the empty binding, but the failure is
	// observable in logs instead of being silently swallowed (B3-F73): rows
	// are produced by encodeIDList, so corruption means a storage defect.
	if err := json.Unmarshal([]byte(encoded), &ids); err != nil {
		logger.Errorf(context.Background(), "agent adoption: malformed id list payload %q: %v", encoded, err)
	}
	return ids
}

func encodeIDList(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// buildLocalAgent projects the Release's portable payload plus the
// Variant's local capability mapping into a local Agent Definition draft.
// The payload never carries KB/model bindings (T28 allow-list boundary);
// every local binding here comes from the mapping rows.
//
// Model selection is an explicit, order-independent rule: when several
// mappings each carry a model, the binding of the lexicographically
// smallest capability name wins. (ListCapabilityMappings already returns
// rows ordered capability ASC, so this reproduces the historical "first
// non-empty ModelID" outcome while staying deterministic even for
// unordered input.)
func buildLocalAgent(variant *types.AgentAdoptionVariantEntity, payload types.AgentReleasePayload, manifest types.AgentReleaseManifest, mappings []types.AgentVariantCapabilityMappingEntity) *types.CustomAgent {
	knowledgeBases := []string{}
	modelID := ""
	modelCapability := ""
	for _, mapping := range mappings {
		knowledgeBases = append(knowledgeBases, decodeIDList(mapping.KnowledgeBaseIDs)...)
		trimmed := strings.TrimSpace(mapping.ModelID)
		if trimmed == "" {
			continue
		}
		if modelID == "" || mapping.Capability < modelCapability {
			modelID = trimmed
			modelCapability = mapping.Capability
		}
	}
	config := types.CustomAgentConfig{
		AgentMode:      payload.AgentMode,
		SystemPrompt:   payload.SystemPrompt,
		PersonaMBTI:    payload.PersonaMBTI,
		PersonaStyle:   payload.PersonaStyle,
		AllowedTools:   payload.AllowedTools,
		Subagents:      payload.Subagents,
		KnowledgeBases: knowledgeBases,
		ModelID:        modelID,
	}
	if len(payload.Skills) > 0 {
		config.SkillsSelectionMode = "selected"
		config.SelectedSkills = payload.Skills
	}
	return &types.CustomAgent{
		Name:        variant.Name,
		Description: manifest.Summary,
		Config:      config,
	}
}
