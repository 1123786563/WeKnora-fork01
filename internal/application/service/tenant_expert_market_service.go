// Tenant-internal expert market service (M4 Task 5). The semantic this file
// implements: publishing an agent EXPORTS it — the M2 §5 whitelist
// (persona/system prompt, skill references, subagents, starters) is
// materialized into an immutable expert snapshot in the SAME ScanExperts
// layout the builtin library and the skillhub materializer use, written
// atomically into a tenant-scoped published-experts root that the experts
// catalog never scans. Installing copies the digest-verified snapshot into
// the tenant's installed-experts root (the tree the M2 ExpertSource reads),
// records an expert_installs row on the manifest slug — the linkage that
// answers the listing's installed flag — and instantiates through the M2
// ExpertService, so skill resolution (installed → selected, otherwise
// pending) and agent creation behave exactly like every other expert.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TenantExpertAgentSource is the agent-read slice the publish flow needs:
// one tenant-scoped agent lookup. interfaces.CustomAgentService satisfies
// it; tests fake it.
type TenantExpertAgentSource interface {
	GetAgentByIDAndTenant(ctx context.Context, id string, tenantID uint64) (*types.CustomAgent, error)
}

// TenantExpertMarketService implements interfaces.TenantExpertMarketService.
type TenantExpertMarketService struct {
	agents       TenantExpertAgentSource
	published    repository.PublishedExpertRepository
	installs     repository.ExpertInstallRepository
	instantiator MarketExpertInstantiator // the M2 ExpertService Instantiate seam
	// users resolves publisher ids to display names. Nil (or a failed
	// lookup) degrades to showing the stored id.
	users interfaces.TenantSkillPublisherNames
	// publishedRoot is where immutable publish snapshots live
	// (experts.PublishedDataRoot in production; a temp dir in tests) —
	// outside the installed-experts root the catalog scans.
	publishedRoot string
	// marketRoot is the installed-experts root installs seed into
	// (experts.MarketDataRoot in production).
	marketRoot string
	// locks serializes installs per (tenant, published expert) — the Octop
	// _install_lock_for precedent: two concurrent installs would race the
	// atomic snapshot copy and the ledger upsert.
	locks *keyedMutex
	now   func() time.Time
}

var _ interfaces.TenantExpertMarketService = (*TenantExpertMarketService)(nil)

// NewTenantExpertMarketService wires the seams. agents is the custom agent
// service, published/installs the market repositories, instantiator the
// interfaces.ExpertService (Instantiate), users may be nil, and the roots
// are experts.PublishedDataRoot()/experts.MarketDataRoot() in production.
func NewTenantExpertMarketService(
	agents TenantExpertAgentSource,
	published repository.PublishedExpertRepository,
	installs repository.ExpertInstallRepository,
	instantiator MarketExpertInstantiator,
	users interfaces.TenantSkillPublisherNames,
	publishedRoot, marketRoot string,
) *TenantExpertMarketService {
	return &TenantExpertMarketService{
		agents:        agents,
		published:     published,
		installs:      installs,
		instantiator:  instantiator,
		users:         users,
		publishedRoot: publishedRoot,
		marketRoot:    marketRoot,
		locks:         newKeyedMutex(),
		now:           time.Now,
	}
}

// withTenantExpertInstallLock runs fn while holding the per-(tenant,
// published expert) install lock.
func (s *TenantExpertMarketService) withTenantExpertInstallLock(
	ctx context.Context, tenantID uint64, publishedID string, fn func(context.Context) error,
) error {
	key := fmt.Sprintf("tenant-expert:%d:%s", tenantID, publishedID)
	release, err := s.locks.lock(ctx, key)
	if err != nil {
		return err
	}
	defer release()
	return fn(ctx)
}

// PublishAgentExpert implements interfaces.TenantExpertMarketService.
func (s *TenantExpertMarketService) PublishAgentExpert(
	ctx context.Context, tenantID uint64, agentID, publishedBy string, req interfaces.PublishAgentExpertRequest,
) (*interfaces.PublishedExpertView, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, apperrors.NewNotFoundError("agent not found")
	}
	// The source agent must live in THIS tenant: the export reads the
	// publisher's agent, and another tenant's agent reads as absent (the
	// wrong-tenant 404).
	agent, err := s.agents.GetAgentByIDAndTenant(ctx, agentID, tenantID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return nil, apperrors.NewNotFoundError("agent not found")
		}
		return nil, apperrors.NewInternalServerError("load the agent to publish: " + err.Error())
	}
	if agent == nil {
		return nil, apperrors.NewNotFoundError("agent not found")
	}

	materialized, err := experts.MaterializePublishedAgent(agent, experts.PublishedAgentExport{
		Name:        req.Name,
		Description: req.Description,
		Locale:      types.LanguageFromContextOrDefault(ctx),
	})
	if err != nil {
		return nil, apperrors.NewBadRequestError("cannot export the agent as an expert: " + err.Error())
	}

	// The immutable snapshot: written atomically under the tenant's
	// published-experts root (staging outside every scan root — the
	// WriteMaterializedExpert contract). A re-publish atomically replaces
	// the previous snapshot in full.
	snapshotDir := experts.PublishedSnapshotDir(s.publishedRoot, tenantID, materialized.Manifest.ID)
	if err := experts.WriteMaterializedExpert(snapshotDir, materialized); err != nil {
		return nil, apperrors.NewInternalServerError("write the expert snapshot: " + err.Error())
	}

	now := s.now()
	name := strings.TrimSpace(materialized.Manifest.Label["zh"])
	if name == "" {
		name = materialized.Manifest.Label["en"]
	}
	if err := s.published.Upsert(ctx, &types.PublishedExpertEntity{
		ID: uuid.NewString(), TenantID: tenantID, AgentID: agentID,
		Name: name, Description: materialized.Manifest.Description["zh"],
		SnapshotRef:    snapshotDir,
		SnapshotSHA256: materialized.SnapshotSHA256(),
		PublishedBy:    strings.TrimSpace(publishedBy),
		CreatedAt:      now, UpdatedAt: now,
	}); err != nil {
		return nil, apperrors.NewInternalServerError("publish the expert: " + err.Error())
	}
	// Read back: the upsert kept the live row's ID and created_at on a
	// re-publish, so the stored row — not the candidate above — is the
	// answer.
	row, err := s.published.GetByTenantAndAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, apperrors.NewInternalServerError("the publish row vanished after publishing")
	}
	logger.Infof(ctx, "Published agent %s of tenant %d as expert %s (snapshot sha %s)",
		agentID, tenantID, materialized.Manifest.ID, row.SnapshotSHA256)
	return &interfaces.PublishedExpertView{
		ID:             row.ID,
		AgentID:        row.AgentID,
		Name:           row.Name,
		Description:    row.Description,
		SnapshotSHA256: row.SnapshotSHA256,
		PublishedBy:    row.PublishedBy,
		PublishedAt:    row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

// UnpublishExpert implements interfaces.TenantExpertMarketService. The
// snapshot stays on disk: it is immutable history and any installed copies
// reference their own trees, not this directory.
func (s *TenantExpertMarketService) UnpublishExpert(
	ctx context.Context, tenantID uint64, publishedID string,
) error {
	row, err := s.published.GetByTenantAndID(ctx, tenantID, strings.TrimSpace(publishedID))
	if err != nil {
		return err
	}
	if row == nil {
		return apperrors.NewNotFoundError("expert is not published")
	}
	if err := s.published.Delete(ctx, tenantID, row.ID); err != nil {
		return apperrors.NewInternalServerError("unpublish the expert: " + err.Error())
	}
	return nil
}

// ListPublishedExperts implements interfaces.TenantExpertMarketService.
func (s *TenantExpertMarketService) ListPublishedExperts(
	ctx context.Context, tenantID uint64,
) (*interfaces.PublishedExpertIndex, error) {
	rows, err := s.published.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	// One ledger listing answers every "installed" flag: the linkage is the
	// expert_installs row whose slug IS the published expert's manifest ID
	// (tenant-expert-<agentID>), the same deterministic key installs write.
	installed := make(map[string]bool)
	if s.installs != nil {
		ledger, err := s.installs.ListByTenant(ctx, tenantID)
		if err != nil {
			// Degrade to "not installed" — the listing stays servable.
			logger.Warnf(ctx, "[tenant-expert-market] listing the expert install ledger failed: %v", err)
		} else {
			for _, row := range ledger {
				installed[row.Slug] = true
			}
		}
	}
	names := s.publisherNames(ctx, rows)

	index := &interfaces.PublishedExpertIndex{Experts: make([]interfaces.PublishedExpertEntry, 0, len(rows))}
	for _, row := range rows {
		index.Experts = append(index.Experts, interfaces.PublishedExpertEntry{
			ID:            row.ID,
			Name:          row.Name,
			Description:   row.Description,
			PublisherName: names[row.PublishedBy],
			Installed:     installed[experts.TenantExpertID(row.AgentID)],
			CreatedAt:     row.CreatedAt,
		})
	}
	return index, nil
}

// publisherNames resolves the distinct publisher ids the listing shows. A
// failed lookup degrades to the raw id (the tenant skill market precedent).
func (s *TenantExpertMarketService) publisherNames(
	ctx context.Context, rows []types.PublishedExpertEntity,
) map[string]string {
	out := make(map[string]string)
	ids := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.PublishedBy)
		if id == "" {
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		out[id] = id // fallback until a user row says otherwise
	}
	if len(ids) == 0 || s.users == nil {
		return out
	}
	users, err := s.users.GetUsersByIDs(ctx, ids)
	if err != nil {
		logger.Warnf(ctx, "[tenant-expert-market] resolving publisher names failed: %v", err)
		return out
	}
	for id, user := range users {
		if user != nil && strings.TrimSpace(user.Username) != "" {
			out[id] = user.Username
		}
	}
	return out
}

// InstallPublishedExpert implements interfaces.TenantExpertMarketService.
// Flow: gate on the publish row → lock → load the snapshot → verify its
// digest against the row → copy atomically into the tenant's
// installed-experts root → upsert the ledger row → M2 Instantiate. The
// seeds precede Instantiate because its catalog lookup scans the installed
// tree per call; an Instantiate failure therefore leaves the expert
// installed (and listed) for a retry — the skillset-install precedent.
func (s *TenantExpertMarketService) InstallPublishedExpert(
	ctx context.Context, tenantID uint64, publishedID string, req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	publishedID = strings.TrimSpace(publishedID)
	row, err := s.published.GetByTenantAndID(ctx, tenantID, publishedID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		// Unknown, unpublished and other-tenant publish rows all read the
		// same way here — the wrong-tenant 404.
		return nil, apperrors.NewNotFoundError("expert is not published")
	}

	var result *interfaces.InstantiateResult
	err = s.withTenantExpertInstallLock(ctx, tenantID, publishedID, func(ctx context.Context) error {
		expertID := experts.TenantExpertID(row.AgentID)
		snapshot, err := experts.LoadExpertDir(experts.PublishedSnapshotDir(s.publishedRoot, tenantID, expertID))
		if err != nil {
			return apperrors.NewInternalServerError("read the published expert snapshot: " + err.Error())
		}
		// The immutable-snapshot contract: the tree on disk must still hash
		// to the digest the publish row recorded, else the install refuses
		// to seed tampered content.
		seed := &experts.MaterializedExpert{Manifest: snapshot.Manifest, PersonaFiles: snapshot.PersonaFiles}
		if digest := seed.SnapshotSHA256(); digest == "" || digest != row.SnapshotSHA256 {
			return apperrors.NewInternalServerError(
				"the published expert snapshot does not match its recorded digest; re-publish the expert")
		}

		installDir := experts.MarketInstallDir(s.marketRoot, tenantID, expertID)
		if err := experts.WriteMaterializedExpert(installDir, seed); err != nil {
			return apperrors.NewInternalServerError("seed the installed expert: " + err.Error())
		}

		// The ledger row ties the install back to the published expert:
		// slug = the manifest ID (tenant-expert-<agentID>), the same key
		// the listing's installed flag resolves.
		createdBy, _ := types.UserIDFromContext(ctx)
		if err := s.installs.Upsert(ctx, &types.ExpertInstallEntity{
			ID: uuid.NewString(), TenantID: tenantID, Slug: expertID,
			StorageRef: installDir, SnapshotSHA256: row.SnapshotSHA256, CreatedBy: createdBy,
		}); err != nil {
			return apperrors.NewInternalServerError("record the expert install: " + err.Error())
		}

		instantiated, err := s.instantiator.Instantiate(ctx, tenantID, expertID, req)
		if err != nil {
			return marketInstantiateError(err)
		}
		result = instantiated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
