// Tenant-internal skill market service (M4 Task 4). The semantic this file
// implements: tenant_skill_catalog rows are tenant-scoped and shared by
// every member of the workspace, so "publishing to the tenant market" is a
// VISIBILITY FLAG over a definition the installer already shares — not a
// content copy. The plan's cross-tenant chain (read publisher archive →
// RegisterCatalogFromArchive under the installer's tenant) collapses here:
// RegisterCatalogFromArchive upserts by (tenant, name) and returns the
// existing row for identical bytes without minting a second object, so
// re-registering the same archive into the SAME tenant is a no-op. Install
// therefore goes straight to InstallCatalogToConfigs, which resolves the
// shared catalog row and reads its own BundleRef through the bundle
// read-back chain — zero content duplication by construction.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TenantSkillMarketCatalogStore is the catalog-read slice the market needs:
// definition lookups scoped by tenant. repository.TenantSkillRepository
// satisfies it; tests fake it.
type TenantSkillMarketCatalogStore interface {
	GetCatalog(ctx context.Context, tenantID uint64, catalogID string) (*types.TenantSkillCatalogEntity, error)
	ListCatalogsByTenant(ctx context.Context, tenantID uint64) ([]*types.TenantSkillCatalogEntity, error)
	ListSkillsByTenant(ctx context.Context, tenantID uint64) ([]*types.TenantSkillEntity, error)
}

// TenantSkillMarketInstaller is the install half: the existing catalog
// install pipeline. *TenantSkillService satisfies it; tests fake it.
type TenantSkillMarketInstaller interface {
	InstallCatalogToConfigs(ctx context.Context, tenantID uint64, catalogID string, configIDs []string) (*CatalogInstallResult, error)
}

// TenantSkillMarketService implements interfaces.TenantSkillMarketService.
type TenantSkillMarketService struct {
	published repository.PublishedSkillRepository
	catalogs  TenantSkillMarketCatalogStore
	installs  TenantSkillMarketInstaller
	// users resolves publisher ids to display names. Nil (or a failed
	// lookup) degrades to showing the stored id.
	users interfaces.TenantSkillPublisherNames
	now   func() time.Time
}

var _ interfaces.TenantSkillMarketService = (*TenantSkillMarketService)(nil)

// NewTenantSkillMarketService wires the seams. catalogs is the tenant skill
// repository, installer the *TenantSkillService (InstallCatalogToConfigs);
// users may be nil (publisher names then fall back to the raw ids).
func NewTenantSkillMarketService(
	published repository.PublishedSkillRepository,
	catalogs TenantSkillMarketCatalogStore,
	installer TenantSkillMarketInstaller,
	users interfaces.TenantSkillPublisherNames,
) *TenantSkillMarketService {
	return &TenantSkillMarketService{
		published: published,
		catalogs:  catalogs,
		installs:  installer,
		users:     users,
		now:       time.Now,
	}
}

// withClock swaps the clock (tests observe the idempotent re-publish
// timestamp refresh without sleeping).
func (s *TenantSkillMarketService) withClock(now func() time.Time) *TenantSkillMarketService {
	s.now = now
	return s
}

// PublishSkill implements interfaces.TenantSkillMarketService.
func (s *TenantSkillMarketService) PublishSkill(
	ctx context.Context, tenantID uint64, catalogID, publishedBy string,
) (*interfaces.PublishedSkillView, error) {
	catalogID = strings.TrimSpace(catalogID)
	if catalogID == "" {
		return nil, apperrors.NewNotFoundError("skill not found")
	}
	// The catalog row must already live in THIS tenant: publishing is a flag
	// on a shared definition, so there is nothing to publish otherwise
	// (another tenant's catalog reads as absent — the wrong-tenant 404).
	catalog, err := s.catalogs.GetCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, apperrors.NewNotFoundError("skill not found")
	}
	now := s.now()
	if err := s.published.Upsert(ctx, &types.PublishedSkillEntity{
		ID: uuid.NewString(), TenantID: tenantID, CatalogID: catalogID,
		PublishedBy: strings.TrimSpace(publishedBy),
		CreatedAt:   now, UpdatedAt: now,
	}); err != nil {
		return nil, apperrors.NewInternalServerError("publish the skill: " + err.Error())
	}
	// Read back: the upsert kept the live row's ID and created_at on a
	// re-publish, so the stored row — not the candidate above — is the
	// answer.
	row, err := s.published.GetByTenantAndCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, apperrors.NewInternalServerError("the publish row vanished after publishing")
	}
	return &interfaces.PublishedSkillView{
		CatalogID: row.CatalogID, PublishedBy: row.PublishedBy,
		PublishedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// UnpublishSkill implements interfaces.TenantSkillMarketService.
func (s *TenantSkillMarketService) UnpublishSkill(
	ctx context.Context, tenantID uint64, catalogID string,
) error {
	catalogID = strings.TrimSpace(catalogID)
	row, err := s.published.GetByTenantAndCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return err
	}
	if row != nil {
		// Soft-delete the live row — including an orphaned one whose catalog
		// definition was deleted out from under it (DeleteCatalog knows
		// nothing about the market).
		if err := s.published.Delete(ctx, tenantID, catalogID); err != nil {
			return apperrors.NewInternalServerError("unpublish the skill: " + err.Error())
		}
		return nil
	}
	// Nothing live: idempotent as long as the tenant actually has the
	// catalog; a tenant that never had it gets the wrong-tenant 404.
	catalog, err := s.catalogs.GetCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return err
	}
	if catalog == nil {
		return apperrors.NewNotFoundError("skill not found")
	}
	return nil
}

// ListPublishedSkills implements interfaces.TenantSkillMarketService.
func (s *TenantSkillMarketService) ListPublishedSkills(
	ctx context.Context, tenantID uint64,
) (*interfaces.PublishedSkillIndex, error) {
	rows, err := s.published.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	catalogs, err := s.catalogs.ListCatalogsByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	catalogByID := make(map[string]*types.TenantSkillCatalogEntity, len(catalogs))
	for _, catalog := range catalogs {
		if catalog != nil {
			catalogByID[catalog.ID] = catalog
		}
	}
	// One install listing answers every "installed" flag: members share the
	// tenant's sandbox configs, so a skill installed anywhere in the
	// workspace is installed for the listing.
	installs, err := s.catalogs.ListSkillsByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	installedCatalogs := make(map[string]bool, len(installs))
	for _, row := range installs {
		if row != nil && strings.TrimSpace(row.CatalogID) != "" {
			installedCatalogs[row.CatalogID] = true
		}
	}
	publisherName := s.publisherNames(ctx, rows, catalogByID)

	index := &interfaces.PublishedSkillIndex{Skills: make([]interfaces.PublishedSkillEntry, 0, len(rows))}
	for _, row := range rows {
		catalog := catalogByID[row.CatalogID]
		if catalog == nil {
			// The definition was deleted; the orphan neither lists nor
			// installs — it is cleaned up by the next unpublish.
			continue
		}
		index.Skills = append(index.Skills, interfaces.PublishedSkillEntry{
			CatalogID:     catalog.ID,
			Name:          catalog.Name,
			Description:   catalog.Description,
			Version:       catalog.Version,
			PublisherName: publisherName[row.PublishedBy],
			Installed:     installedCatalogs[catalog.ID],
		})
	}
	return index, nil
}

// publisherNames resolves the distinct publisher ids the listing shows.
// A failed lookup degrades to the raw id: the listing stays servable and
// the degradation is logged (the install-flag precedent).
func (s *TenantSkillMarketService) publisherNames(
	ctx context.Context, rows []types.PublishedSkillEntity, catalogByID map[string]*types.TenantSkillCatalogEntity,
) map[string]string {
	out := make(map[string]string)
	ids := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.PublishedBy)
		if id == "" {
			continue
		}
		if _, ok := catalogByID[row.CatalogID]; !ok {
			continue // orphaned rows do not list
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
		logger.Warnf(ctx, "[tenant-skill-market] resolving publisher names failed: %v", err)
		return out
	}
	for id, user := range users {
		if user != nil && strings.TrimSpace(user.Username) != "" {
			out[id] = user.Username
		}
	}
	return out
}

// InstallPublishedSkill implements interfaces.TenantSkillMarketService.
// The publish row is the gate: only market-visible skills install through
// this surface (the plain catalog install remains the admin's direct path).
func (s *TenantSkillMarketService) InstallPublishedSkill(
	ctx context.Context, tenantID uint64, catalogID string, sandboxConfigIDs []string,
) (*interfaces.TenantSkillInstallResult, error) {
	catalogID = strings.TrimSpace(catalogID)
	row, err := s.published.GetByTenantAndCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		// Unknown, unpublished and other-tenant publish rows all read the
		// same way here — the wrong-tenant 404.
		return nil, apperrors.NewNotFoundError("skill is not published")
	}
	result, err := s.installs.InstallCatalogToConfigs(ctx, tenantID, catalogID, sandboxConfigIDs)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &interfaces.TenantSkillInstallResult{Installs: map[string]string{}}, nil
	}
	out := &interfaces.TenantSkillInstallResult{Installs: result.Installs}
	if out.Installs == nil {
		out.Installs = map[string]string{}
	}
	if len(result.Errors) > 0 {
		out.Errors = result.Errors
	}
	return out, nil
}
