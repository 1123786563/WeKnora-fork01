package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentAdoptionNotFound          = errors.New("agent adoption resource not found")
	ErrAgentAdoptionVariantTransition = errors.New("agent adoption variant state transition failed")
	// ErrAgentAdoptionRemapStateConflict marks a ReplaceCapabilityMappings
	// whose guarded state UPDATE lost to a concurrent transition (e.g.
	// publish landing between the service's pre-check and this write): the
	// rewrite fails loudly instead of silently downgrading the state (B3-F87).
	ErrAgentAdoptionRemapStateConflict = errors.New("agent adoption variant state changed during capability rewrite")
)

// AgentAdoptionPublishedRow pairs one published Variant with its live local
// agent row. Agent is nil when the local agent is gone (soft-deleted): the
// available-agent read model then simply omits the row.
type AgentAdoptionPublishedRow struct {
	Variant types.AgentAdoptionVariantEntity
	Agent   *types.CustomAgent
}

// AgentAdoptionRepository owns the Adoption/Variant/mapping SQL. All reads
// and writes are tenant-scoped and parameter-bound; the two marketplace
// getters are read-only proxies so the adoption service never re-implements
// marketplace queries.
type AgentAdoptionRepository interface {
	AdoptListing(context.Context, *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error)
	GetAdoption(context.Context, uint64, string) (*types.AgentAdoptionEntity, error)
	ListAdoptions(context.Context, uint64) ([]types.AgentAdoptionEntity, error)
	CreateVariant(context.Context, *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error)
	GetVariant(context.Context, uint64, string) (*types.AgentAdoptionVariantEntity, error)
	ListVariantsByAdoption(context.Context, uint64, string) ([]types.AgentAdoptionVariantEntity, error)
	ReplaceCapabilityMappings(context.Context, uint64, string, []types.AgentVariantCapabilityMappingEntity, string) error
	ListCapabilityMappings(context.Context, uint64, string) ([]types.AgentVariantCapabilityMappingEntity, error)
	UpdateVariantState(context.Context, uint64, string, []string, string, map[string]any) (*types.AgentAdoptionVariantEntity, error)
	PublishedAvailableAgents(context.Context, uint64) ([]AgentAdoptionPublishedRow, error)
	GetMarketplaceListing(context.Context, uint64, string) (*types.AgentMarketplaceListingEntity, error)
	GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error)
	TransitionAdoption(context.Context, uint64, string, string, string, map[string]any) (*types.AgentAdoptionEntity, error)
	RetiredVariantAgentExists(context.Context, uint64, string) (bool, error)
	RetireVariant(context.Context, uint64, string, string, string) (*types.AgentAdoptionVariantEntity, error)
	EndAdoption(context.Context, uint64, string, string, string) (*types.AgentAdoptionEntity, error)
	IsRetiredMarketplaceAgent(context.Context, uint64, string) (bool, error)
}

type agentAdoptionRepository struct{ db *gorm.DB }

func NewAgentAdoptionRepository(db *gorm.DB) AgentAdoptionRepository {
	return &agentAdoptionRepository{db: db}
}

// AdoptListing inserts the (tenant, listing)-unique Adoption or returns the
// existing row; re-adopting with a different release advances the accepted
// pointer (spec §8 step 3 "建立或更新 Adoption"). Concurrent first adopts
// converge: when two requests race past the missing-row check, the insert
// that loses to uq_agent_adoptions_scope falls back to the existing row and
// reconciles the accepted pointer, so both callers get an idempotent result
// instead of a unique-index error.
func (r *agentAdoptionRepository) AdoptListing(ctx context.Context, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error) {
	var result *types.AgentAdoptionEntity
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, created, err = adoptListingTx(tx, adoption)
		return err
	})
	return result, created, err
}

// adoptListingTx is the transaction-bound adopt upsert, shared with the
// public-marketplace IntroduceRelease transaction (T30 #60). Semantics are
// identical to the pre-refactor AdoptListing: an existing (tenant,
// listing) row reconciles its accepted pointer; a first insert races on
// uq_agent_adoptions_scope and converges to the winner.
func adoptListingTx(tx *gorm.DB, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error) {
	if err := checkReleaseAdmissionTx(tx, adoption.TenantID, adoption.AcceptedReleaseID); err != nil {
		return nil, false, err
	}
	// Serialize eligibility checks with UnlistListing/DeprecateRelease before
	// either inserting or reconciling an Adoption. A service-side precheck is
	// useful for errors, but cannot authorize this write by itself.
	if err := requireListedListing(tx, adoption.TenantID, adoption.ListingID); err != nil {
		return nil, false, err
	}
	if err := requireActiveRelease(tx, adoption.TenantID, adoption.AcceptedReleaseID, adoption.ListingID); err != nil {
		return nil, false, err
	}
	var existing types.AgentAdoptionEntity
	err := tx.Where("tenant_id = ? AND listing_id = ?", adoption.TenantID, adoption.ListingID).First(&existing).Error
	if err == nil {
		return reconcileAdoptionTx(tx, &existing, adoption.AcceptedReleaseID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	created := *adoption
	created.ID = uuid.NewString()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if created.State == "" {
		created.State = "active"
	}
	inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if inserted.Error != nil {
		return nil, false, inserted.Error
	}
	if inserted.RowsAffected == 1 {
		return &created, true, nil
	}
	// Lost the race to uq_agent_adoptions_scope: re-read the winner's row
	// and reconcile, exactly like a sequential re-adopt.
	var winner types.AgentAdoptionEntity
	if err := tx.Where("tenant_id = ? AND listing_id = ?", adoption.TenantID, adoption.ListingID).First(&winner).Error; err != nil {
		return nil, false, err
	}
	if winner.State != "active" {
		return nil, false, ErrAgentAdoptionTransition
	}
	return reconcileAdoptionTx(tx, &winner, adoption.AcceptedReleaseID)
}

// reconcileAdoption is the shared existing-row path: an Adoption accepted at
// the same Release returns as-is; a different Release advances the accepted
// pointer (last write wins, matching sequential adopt semantics).
func reconcileAdoptionTx(tx *gorm.DB, existing *types.AgentAdoptionEntity, acceptedReleaseID string) (*types.AgentAdoptionEntity, bool, error) {
	guard := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", existing.TenantID, existing.ID).
		UpdateColumn("id", gorm.Expr("id"))
	if guard.Error != nil {
		return nil, false, guard.Error
	}
	if guard.RowsAffected != 1 {
		return nil, false, ErrAgentAdoptionNotFound
	}
	// Refresh after acquiring the row guard so concurrent lifecycle changes
	// cannot make the reconciliation decision from a stale Adoption snapshot.
	if err := tx.Where("tenant_id = ? AND id = ?", existing.TenantID, existing.ID).First(existing).Error; err != nil {
		return nil, false, err
	}
	if existing.State != "active" {
		return nil, false, ErrAgentAdoptionTransition
	}
	if existing.AcceptedReleaseID == acceptedReleaseID {
		return existing, false, nil
	}
	existing.AcceptedReleaseID = acceptedReleaseID
	existing.UpdatedAt = time.Now().UTC()
	if err := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", existing.TenantID, existing.ID).
		Updates(map[string]any{"accepted_release_id": existing.AcceptedReleaseID, "updated_at": existing.UpdatedAt}).Error; err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (r *agentAdoptionRepository) GetAdoption(ctx context.Context, tenantID uint64, adoptionID string) (*types.AgentAdoptionEntity, error) {
	var row types.AgentAdoptionEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(adoptionID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentAdoptionRepository) ListAdoptions(ctx context.Context, tenantID uint64) ([]types.AgentAdoptionEntity, error) {
	rows := []types.AgentAdoptionEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *agentAdoptionRepository) CreateVariant(ctx context.Context, variant *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error) {
	if variant == nil || variant.TenantID == 0 || strings.TrimSpace(variant.AdoptionID) == "" || strings.TrimSpace(variant.ReleaseID) == "" || strings.TrimSpace(variant.Name) == "" {
		return nil, fmt.Errorf("invalid agent adoption variant")
	}
	created := *variant
	created.ID = uuid.NewString()
	created.Name = strings.TrimSpace(created.Name)
	if created.State == "" {
		created.State = "draft"
	}
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock rows in the shared order listing -> release -> adoption. This
		// keeps new variant writes synchronized with unlist/deprecate and with
		// adoption lifecycle transitions.
		var adoption types.AgentAdoptionEntity
		if err := tx.Where("tenant_id = ? AND id = ?", created.TenantID, created.AdoptionID).First(&adoption).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentAdoptionNotFound
			}
			return err
		}
		if err := requireListedListing(tx, created.TenantID, adoption.ListingID); err != nil {
			return err
		}
		if err := requireActiveRelease(tx, created.TenantID, created.ReleaseID, adoption.ListingID); err != nil {
			return err
		}
		if err := lockAdoptionState(tx, created.TenantID, created.AdoptionID, "active"); err != nil {
			return err
		}
		return tx.Create(&created).Error
	}); err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *agentAdoptionRepository) GetVariant(ctx context.Context, tenantID uint64, variantID string) (*types.AgentAdoptionVariantEntity, error) {
	var row types.AgentAdoptionVariantEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentAdoptionRepository) ListVariantsByAdoption(ctx context.Context, tenantID uint64, adoptionID string) ([]types.AgentAdoptionVariantEntity, error) {
	rows := []types.AgentAdoptionVariantEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND adoption_id = ?", tenantID, strings.TrimSpace(adoptionID)).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

// allowedRemapStates mirrors the service-side pre-check (draft/mapped/
// tested are re-mappable; published is terminal). The UPDATE carries it as a
// guard so a concurrent CAS landing between the service's read and this
// write cannot be silently downgraded (B3-F87).
var allowedRemapStates = []string{"draft", "mapped", "tested"}

// ReplaceCapabilityMappings atomically swaps the Variant's mapping rows and
// records the recomputed state (draft while incomplete, mapped once every
// required capability is bound). One transaction, all parameters bound.
func (r *agentAdoptionRepository) ReplaceCapabilityMappings(ctx context.Context, tenantID uint64, variantID string, mappings []types.AgentVariantCapabilityMappingEntity, nextState string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND variant_id = ?", tenantID, strings.TrimSpace(variantID)).Delete(&types.AgentVariantCapabilityMappingEntity{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for i := range mappings {
			mappings[i].ID = uuid.NewString()
			mappings[i].TenantID = tenantID
			mappings[i].VariantID = strings.TrimSpace(variantID)
			mappings[i].CreatedAt = now
			mappings[i].UpdatedAt = now
			if err := tx.Create(&mappings[i]).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&types.AgentAdoptionVariantEntity{}).
			Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(variantID), allowedRemapStates).
			Updates(map[string]any{"state": nextState, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			// The guarded UPDATE missed: the variant is gone OR its state moved
			// past the re-mappable set concurrently. Distinguish the two so the
			// caller surfaces a state conflict (409) instead of a plain 404.
			var current types.AgentAdoptionVariantEntity
			if err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).
				Take(&current).Error; err == nil {
				return fmt.Errorf("%w: state is %q", ErrAgentAdoptionRemapStateConflict, current.State)
			}
			return ErrAgentAdoptionNotFound
		}
		return nil
	})
}

func (r *agentAdoptionRepository) ListCapabilityMappings(ctx context.Context, tenantID uint64, variantID string) ([]types.AgentVariantCapabilityMappingEntity, error) {
	rows := []types.AgentVariantCapabilityMappingEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND variant_id = ?", tenantID, strings.TrimSpace(variantID)).Order("capability ASC").Find(&rows).Error
	return rows, err
}

// UpdateVariantState is the compare-and-set transition: the update applies
// only when the current state is one of expectedFrom, so a concurrent or
// repeated transition fails loudly instead of double-applying.
func (r *agentAdoptionRepository) UpdateVariantState(ctx context.Context, tenantID uint64, variantID string, expectedFrom []string, nextState string, updates map[string]any) (*types.AgentAdoptionVariantEntity, error) {
	set := map[string]any{"state": nextState, "updated_at": time.Now().UTC()}
	for key, value := range updates {
		set[key] = value
	}
	// Paired stamps: naming an actor (*_by) also records when it happened
	// (tested_by/tested_at, published_by/published_at), unless the caller
	// supplied the instant explicitly.
	for key := range updates {
		if !strings.HasSuffix(key, "_by") {
			continue
		}
		at := key[:len(key)-len("_by")] + "_at"
		if _, ok := set[at]; !ok {
			set[at] = time.Now().UTC()
		}
	}
	result := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(variantID), expectedFrom).
		Updates(set)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var current types.AgentAdoptionVariantEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentAdoptionNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: variant %s is %q, expected one of %v", ErrAgentAdoptionVariantTransition, variantID, current.State, expectedFrom)
	}
	return r.GetVariant(ctx, tenantID, variantID)
}

// PublishedAvailableAgents returns the adoption-aware available-agent read
// model: published Variants of active Adoptions, joined with their live
// local agent rows (soft-deleted agents drop out via gorm's DeletedAt
// default scope).
func (r *agentAdoptionRepository) PublishedAvailableAgents(ctx context.Context, tenantID uint64) ([]AgentAdoptionPublishedRow, error) {
	adoptions := []types.AgentAdoptionEntity{}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND state = ?", tenantID, "active").Find(&adoptions).Error; err != nil {
		return nil, err
	}
	active := make(map[string]bool, len(adoptions))
	for _, adoption := range adoptions {
		active[adoption.ID] = true
	}
	variants := []types.AgentAdoptionVariantEntity{}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND state = ?", tenantID, "published").Order("created_at ASC, id ASC").Find(&variants).Error; err != nil {
		return nil, err
	}
	agentIDs := make([]string, 0, len(variants))
	for _, variant := range variants {
		if active[variant.AdoptionID] && variant.LocalAgentID != "" {
			agentIDs = append(agentIDs, variant.LocalAgentID)
		}
	}
	agents := map[string]*types.CustomAgent{}
	if len(agentIDs) > 0 {
		rows := []types.CustomAgent{}
		if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, agentIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			agents[rows[i].ID] = &rows[i]
		}
	}
	out := []AgentAdoptionPublishedRow{}
	for _, variant := range variants {
		if !active[variant.AdoptionID] {
			continue
		}
		agent := agents[variant.LocalAgentID]
		if agent == nil {
			// The local agent row is gone (soft-deleted or never created):
			// the available-agent read model simply omits the row.
			continue
		}
		out = append(out, AgentAdoptionPublishedRow{Variant: variant, Agent: agent})
	}
	return out, nil
}

func (r *agentAdoptionRepository) GetMarketplaceListing(ctx context.Context, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	var row types.AgentMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(listingID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return introducedListing(r.db.WithContext(ctx), tenantID, listingID)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// introducedListing synthesizes the adoptable Listing view of a public
// listing this tenant introduced (T30 #60): local rows win, the fallback
// resolves the LATEST introduced release of that public listing as the
// current release. Tenant-scoped like every other read.
func introducedListing(tx *gorm.DB, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	var latest types.TenantIntroducedReleaseEntity
	err := tx.Where("tenant_id = ? AND public_listing_id = ?", tenantID, strings.TrimSpace(listingID)).
		Order("introduced_at DESC, id DESC").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	current := latest.ID
	return &types.AgentMarketplaceListingEntity{
		ID: latest.PublicListingID, TenantID: tenantID, SourceAgentID: latest.PublicListingID,
		DisplayName: latest.DisplayName, Summary: latest.Summary, State: "listed",
		CurrentReleaseID: &current,
	}, nil
}

func (r *agentAdoptionRepository) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return introducedRelease(r.db.WithContext(ctx), tenantID, releaseID)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// introducedRelease synthesizes the Release view of one introduced public
// release: the #59 chain (CreateVariant gating, manifest reads, publish
// digest verification and payload decode) consumes it unchanged. The
// synthesized row deliberately keeps the portable content ONLY — there is
// no local submission/agent-version lineage, which is exactly the point of
// the introduction ledger.
func introducedRelease(tx *gorm.DB, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.TenantIntroducedReleaseEntity
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &types.AgentReleaseEntity{
		ID: row.ID, TenantID: tenantID, ListingID: row.PublicListingID,
		SemanticVersion: row.SemanticVersion, BundleDigest: row.BundleDigest,
		ManifestJSON: row.ManifestJSON, DependencyLockJSON: row.DependencyLockJSON,
		Bundle: row.Bundle, PublishedBy: row.IntroducedBy, CreatedAt: row.IntroducedAt,
	}, nil
}
func (r *agentAdoptionRepository) RetireVariant(ctx context.Context, tenantID uint64, variantID, actorID, reason string) (*types.AgentAdoptionVariantEntity, error) {
	variantID = strings.TrimSpace(variantID)
	if tenantID == 0 || variantID == "" || strings.TrimSpace(actorID) == "" || strings.TrimSpace(reason) == "" {
		return nil, ErrAgentAdoptionVariantTransition
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, variantID, []string{"draft", "mapped", "tested", "published"}).
		Updates(map[string]any{"state": "retired", "retired_by": strings.TrimSpace(actorID), "retired_at": now, "retirement_reason": strings.TrimSpace(reason), "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).Where("tenant_id = ? AND id = ?", tenantID, variantID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, ErrAgentAdoptionNotFound
		}
		return nil, ErrAgentAdoptionVariantTransition
	}
	return r.GetVariant(ctx, tenantID, variantID)
}

// EndAdoption is a one-way CAS that also enforces the aggregate invariant:
// every Variant must already be retired before the Adoption can end.
func (r *agentAdoptionRepository) EndAdoption(ctx context.Context, tenantID uint64, adoptionID, actorID, reason string) (*types.AgentAdoptionEntity, error) {
	adoptionID = strings.TrimSpace(adoptionID)
	if tenantID == 0 || adoptionID == "" || strings.TrimSpace(actorID) == "" || strings.TrimSpace(reason) == "" {
		return nil, ErrAgentAdoptionTransition
	}
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// This conditional no-op UPDATE is the same parent-row gate used by
		// CreateVariant. Keep the child check in a later command so PostgreSQL
		// READ COMMITTED takes a fresh snapshot after any gate wait.
		gate := tx.Model(&types.AgentAdoptionEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, "active").
			UpdateColumn("updated_at", gorm.Expr("updated_at"))
		if gate.Error != nil {
			return gate.Error
		}
		if gate.RowsAffected != 1 {
			var count int64
			if err := tx.Model(&types.AgentAdoptionEntity{}).Where("tenant_id = ? AND id = ?", tenantID, adoptionID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrAgentAdoptionNotFound
			}
			return ErrAgentAdoptionTransition
		}
		var variants int64
		if err := tx.Model(&types.AgentAdoptionVariantEntity{}).
			Where("tenant_id = ? AND adoption_id = ? AND state <> ?", tenantID, adoptionID, "retired").Count(&variants).Error; err != nil {
			return err
		}
		if variants != 0 {
			return ErrAgentAdoptionTransition
		}
		ended := tx.Model(&types.AgentAdoptionEntity{}).
			Where("tenant_id = ? AND id = ? AND state = ?", tenantID, adoptionID, "active").
			Updates(map[string]any{"state": "ended", "ended_by": strings.TrimSpace(actorID), "ended_at": now, "end_reason": strings.TrimSpace(reason), "updated_at": now})
		if ended.Error != nil {
			return ended.Error
		}
		if ended.RowsAffected != 1 {
			return ErrAgentAdoptionTransition
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetAdoption(ctx, tenantID, adoptionID)
}

// IsRetiredMarketplaceAgent implements the Task admission lookup. An Agent
// with no marketplace Variant is ordinary local content and is admitted.
func (r *agentAdoptionRepository) IsRetiredMarketplaceAgent(ctx context.Context, tenantID uint64, localAgentID string) (bool, error) {
	if tenantID == 0 || strings.TrimSpace(localAgentID) == "" {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND local_agent_id = ? AND state = ?", tenantID, strings.TrimSpace(localAgentID), "retired").Count(&count).Error
	return count > 0, err
}
