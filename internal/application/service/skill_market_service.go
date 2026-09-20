// Skill market service (M4 Task 3): the remote-consumption surface over the
// T1 skillhub client (search / rankings / download, cached wrapper with
// Download bypass) and the T2 skillset index + materializer. Skill installs
// repack a validated package into the archive layout the tenant catalog
// accepts and reuse the existing register/install pipeline; skillset installs
// materialize the expert tree, record the install ledger row and compose with
// the M2 ExpertService.Instantiate (the catalog merge makes the fresh tree
// visible to GetExpert immediately — MarketExpertSource scans per call).
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/agent/skills/skillhub"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// SkillMarketCatalog is the tenant-catalog half of a market skill install.
// *TenantSkillService satisfies it; tests fake it.
type SkillMarketCatalog interface {
	RegisterCatalogFromArchive(ctx context.Context, tenantID uint64, archive []byte) (*types.TenantSkillCatalogEntity, error)
	InstallCatalogToConfigs(ctx context.Context, tenantID uint64, catalogID string, configIDs []string) (*CatalogInstallResult, error)
}

// MarketExpertInstantiator is the slice of interfaces.ExpertService the
// skillset install composes with, as its own seam so tests can fake the M2
// boundary without dragging the whole service graph in.
type MarketExpertInstantiator interface {
	Instantiate(ctx context.Context, tenantID uint64, expertID string, req interfaces.InstantiateRequest) (*interfaces.InstantiateResult, error)
}

// SkillMarketService implements interfaces.SkillMarketService.
type SkillMarketService struct {
	market   skillhub.Client          // cached wrapper: Search/Rankings/Download (Download bypasses the cache)
	skillset skillhub.SkillsetClient  // cached wrapper: SkillsetIndex/SkillsetDetail
	catalog  SkillMarketCatalog       // tenant catalog register/install pipeline
	experts  MarketExpertInstantiator // M2 Instantiate
	installs repository.ExpertInstallRepository
	// dataRoot is the expert-market root materialized trees live under
	// (experts.MarketDataRoot in production; t.TempDir in tests).
	dataRoot string
	// locks serializes installs per (tenant, kind, slug) — the Octop
	// _install_lock_for precedent. Two concurrent installs of the same slug
	// would double-download, race the catalog upsert, and on the skillset
	// side race the staging-swap write.
	locks *keyedMutex
}

var _ interfaces.SkillMarketService = (*SkillMarketService)(nil)

// NewSkillMarketService wires the seams. market and skillset usually are the
// SAME cached client (it implements both interfaces); catalog is the
// *TenantSkillService; expertInstantiate the interfaces.ExpertService.
func NewSkillMarketService(
	market skillhub.Client,
	skillset skillhub.SkillsetClient,
	catalog SkillMarketCatalog,
	expertInstantiate MarketExpertInstantiator,
	installs repository.ExpertInstallRepository,
	dataRoot string,
) *SkillMarketService {
	return &SkillMarketService{
		market:   market,
		skillset: skillset,
		catalog:  catalog,
		experts:  expertInstantiate,
		installs: installs,
		dataRoot: dataRoot,
		locks:    newKeyedMutex(),
	}
}

// NewSkillMarketServiceFromConfig builds the production service: one HTTP
// client from the skillhub_market config section, wrapped in the TTL cache
// (stale-fallback for listings; Download bypasses it by construction).
func NewSkillMarketServiceFromConfig(
	cfg *config.Config,
	catalog SkillMarketCatalog,
	expertInstantiate MarketExpertInstantiator,
	installs repository.ExpertInstallRepository,
) (*SkillMarketService, error) {
	client, err := skillhub.New(cfg.SkillHubMarketHost(), cfg.SkillHubMarketTimeout())
	if err != nil {
		return nil, fmt.Errorf("build the skill market client: %w", err)
	}
	cached := skillhub.NewCached(client, skillhub.DefaultCacheTTL)
	return NewSkillMarketService(cached, cached, catalog, expertInstantiate, installs, experts.MarketDataRoot()), nil
}

// withMarketInstallLock runs fn while holding the per-(tenant, kind, slug)
// install lock.
func (s *SkillMarketService) withMarketInstallLock(
	ctx context.Context, tenantID uint64, kind, slug string, fn func(context.Context) error,
) error {
	key := fmt.Sprintf("skillhub-%s:%d:%s", kind, tenantID, slug)
	release, err := s.locks.lock(ctx, key)
	if err != nil {
		return err
	}
	defer release()
	return fn(ctx)
}

// ---------------------------------------------------------------------------
// listings
// ---------------------------------------------------------------------------

// SearchMarketSkills implements interfaces.SkillMarketService.
func (s *SkillMarketService) SearchMarketSkills(ctx context.Context, query string, limit int) (*interfaces.SkillMarketListing, error) {
	results, err := s.market.Search(ctx, query, limit)
	if isHardMarketFailure(err) {
		return nil, skillMarketListingError("search", err)
	}
	return marketListing(results, err), nil
}

// MarketSkillRankings implements interfaces.SkillMarketService.
func (s *SkillMarketService) MarketSkillRankings(ctx context.Context, kind string) (*interfaces.SkillMarketListing, error) {
	results, err := s.market.Rankings(ctx, kind)
	if isHardMarketFailure(err) {
		return nil, skillMarketListingError("rankings", err)
	}
	return marketListing(results, err), nil
}

// isHardMarketFailure separates the two error families the cached wrapper
// returns: ErrStaleOnly rides a servable value (the last good answer, even an
// empty one) and must become a 200 with stale=true; every other failure has
// no value worth serving.
func isHardMarketFailure(err error) bool {
	return err != nil && !errors.Is(err, skillhub.ErrStaleOnly)
}

// marketListing projects the client summaries and stamps the stale flag the
// cached wrapper reports alongside a served value.
func marketListing(results []skillhub.SkillSummary, err error) *interfaces.SkillMarketListing {
	out := &interfaces.SkillMarketListing{
		Results: make([]interfaces.SkillMarketResult, 0, len(results)),
		Stale:   err != nil && errors.Is(err, skillhub.ErrStaleOnly),
	}
	for _, r := range results {
		raw := r.Raw
		if raw == nil {
			raw = map[string]string{}
		}
		out.Results = append(out.Results, interfaces.SkillMarketResult{
			Slug: r.Slug, Name: r.Name, Description: r.Description, Version: r.Version, Raw: raw,
		})
	}
	return out
}

// skillMarketListingError maps a failed listing fetch onto the HTTP taxonomy:
// an unreachable upstream with no cached value is a 503 with the underlying
// cause in the message — never a silent empty list.
func skillMarketListingError(operation string, err error) error {
	if errors.Is(err, skillhub.ErrUnreachable) {
		return apperrors.NewServiceUnavailableError(
			"the skill market is unreachable, please retry later (" + operation + ": " + err.Error() + ")")
	}
	if errors.Is(err, skillhub.ErrUnsupportedRanking) {
		return apperrors.NewBadRequestError(err.Error())
	}
	if errors.Is(err, skillhub.ErrInvalidSkillsetSlug) {
		return apperrors.NewBadRequestError(err.Error())
	}
	return apperrors.NewServiceUnavailableError(
		"the skill market request failed (" + operation + ": " + err.Error() + ")")
}

// ---------------------------------------------------------------------------
// skill install
// ---------------------------------------------------------------------------

// InstallMarketSkill implements interfaces.SkillMarketService. Flow (the
// binding M4 contract): Download → ParsePackage → repack to the skill-root
// archive the catalog accepts → RegisterCatalogFromArchive →
// InstallCatalogToConfigs, all under the per-slug install lock.
func (s *SkillMarketService) InstallMarketSkill(
	ctx context.Context, tenantID uint64, slug string, sandboxConfigIDs []string,
) (*interfaces.MarketSkillInstallResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, apperrors.NewBadRequestError("slug is required")
	}
	configIDs := uniqueNonEmptyStrings(sandboxConfigIDs)
	if len(configIDs) == 0 {
		return nil, apperrors.NewBadRequestError("at least one sandbox is required")
	}

	var result *interfaces.MarketSkillInstallResult
	err := s.withMarketInstallLock(ctx, tenantID, "skill", slug, func(ctx context.Context) error {
		archive, err := s.downloadPackage(ctx, slug)
		if err != nil {
			return err
		}
		repacked, err := repackMarketSkill(slug, archive)
		if err != nil {
			return err
		}
		catalog, err := s.catalog.RegisterCatalogFromArchive(ctx, tenantID, repacked)
		if err != nil {
			return err
		}
		installed, err := s.catalog.InstallCatalogToConfigs(ctx, tenantID, catalog.ID, configIDs)
		if err != nil {
			return err
		}
		result = marketSkillInstallResult(catalog.ID, configIDs, installed)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// downloadPackage fetches one package zip. The cached wrapper's Download
// bypasses the cache by construction, so this is always a fresh fetch; every
// transport failure (including an upstream 404, which the registry does not
// distinguish) surfaces as a 503 carrying the underlying cause.
func (s *SkillMarketService) downloadPackage(ctx context.Context, slug string) ([]byte, error) {
	archive, err := s.market.Download(ctx, slug)
	if err != nil {
		return nil, skillMarketDownloadError(slug, err)
	}
	return archive, nil
}

// repackMarketSkill validates the downloaded package and re-zips its members
// into the flat skill-root layout RegisterCatalogFromArchive's parser
// (ParseSkillBundle) accepts. ParsePackage re-roots the tree at the SKILL.md
// directory, so the deterministic zipSkillFiles output — root SKILL.md plus
// the payload members, no wrapper directory — parses as a plain uploaded
// bundle. Reusing zipSkillFiles (rather than forwarding the raw archive)
// also guarantees the stored bundle carries exactly the validated members:
// nothing ParsePackage rejected can survive into tenant storage.
func repackMarketSkill(slug string, archive []byte) ([]byte, error) {
	files, err := skillhub.ParsePackage(archive)
	if err != nil {
		return nil, apperrors.NewBadRequestError(
			"the skill market package for " + slug + " is invalid: " + err.Error())
	}
	byName := make(map[string][]byte, len(files))
	for _, f := range files {
		byName[f.Name] = f.Content
	}
	repacked, err := zipSkillFiles(byName)
	if err != nil {
		return nil, apperrors.NewInternalServerError(
			"repack the skill market package for " + slug + ": " + err.Error())
	}
	return repacked, nil
}

// marketSkillInstallResult flattens the catalog install outcome into the
// market response shape: install IDs in the requested config order.
func marketSkillInstallResult(catalogID string, configIDs []string, installed *CatalogInstallResult) *interfaces.MarketSkillInstallResult {
	out := &interfaces.MarketSkillInstallResult{
		CatalogID:  catalogID,
		InstallIDs: make([]string, 0, len(configIDs)),
	}
	if installed == nil {
		return out
	}
	for _, configID := range configIDs {
		if skillID, ok := installed.Installs[configID]; ok {
			out.InstallIDs = append(out.InstallIDs, skillID)
		}
	}
	if len(installed.Errors) > 0 {
		out.Errors = installed.Errors
	}
	return out
}

// skillMarketDownloadError maps a package-download failure onto the HTTP
// taxonomy. ErrPackage-family failures are 400s (the registry served
// something validation rejects); everything else — including an upstream
// 404, indistinguishable from an outage on this wire — is a 503 whose
// message carries the underlying cause.
func skillMarketDownloadError(slug string, err error) error {
	if errors.Is(err, skillhub.ErrPackage) {
		return apperrors.NewBadRequestError(
			"the skill market package for " + slug + " failed validation: " + err.Error())
	}
	return apperrors.NewServiceUnavailableError(
		"cannot download the skill market package for " + slug + ": " + err.Error())
}

// ---------------------------------------------------------------------------
// skillset index / detail
// ---------------------------------------------------------------------------

// ListMarketSkillsets implements interfaces.SkillMarketService.
func (s *SkillMarketService) ListMarketSkillsets(ctx context.Context, tenantID uint64) (*interfaces.MarketSkillsetIndex, error) {
	summaries, err := s.skillset.SkillsetIndex(ctx)
	if isHardMarketFailure(err) {
		return nil, skillMarketListingError("skillsets", err)
	}
	installed := s.installedSkillsetSlugs(ctx, tenantID)
	index := &interfaces.MarketSkillsetIndex{
		Skillsets: make([]interfaces.MarketSkillset, 0, len(summaries)),
		Stale:     err != nil && errors.Is(err, skillhub.ErrStaleOnly),
	}
	for _, ss := range summaries {
		index.Skillsets = append(index.Skillsets, interfaces.MarketSkillset{
			Slug:        ss.Slug,
			Name:        ss.Name,
			Description: ss.Description,
			SkillSlugs:  nonNilSkillSlugs(ss.SkillSlugs),
			Installed:   installed[ss.Slug],
		})
	}
	return index, nil
}

// nonNilSkillSlugs keeps the JSON shape an array (never null) and drops
// blank entries.
func nonNilSkillSlugs(slugs []string) []string {
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if trimmed := strings.TrimSpace(slug); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// GetMarketSkillset implements interfaces.SkillMarketService.
func (s *SkillMarketService) GetMarketSkillset(ctx context.Context, tenantID uint64, slug string) (*interfaces.MarketSkillsetDetail, error) {
	summary, err := s.skillset.SkillsetDetail(ctx, slug)
	if isHardMarketFailure(err) {
		return nil, skillsetDetailError(slug, err)
	}
	installed := s.installedSkillsetSlugs(ctx, tenantID)
	return &interfaces.MarketSkillsetDetail{
		Slug:          summary.Slug,
		Name:          summary.Name,
		Description:   summary.Description,
		NameEn:        rawLocalized(summary.Raw, "displayNameEn"),
		DescriptionEn: rawLocalized(summary.Raw, "summaryEn"),
		SkillSlugs:    nonNilSkillSlugs(summary.SkillSlugs),
		Installed:     installed[summary.Slug],
		Stale:         err != nil && errors.Is(err, skillhub.ErrStaleOnly),
	}, nil
}

// installedSkillsetSlugs answers the tenant's installed skillset slugs from
// the ledger. A ledger read failure degrades to "no flags": the listing is
// the registry's answer and stays servable, the degradation is logged, and
// the install flows still consult the authoritative directory scan.
func (s *SkillMarketService) installedSkillsetSlugs(ctx context.Context, tenantID uint64) map[string]bool {
	out := make(map[string]bool)
	if s.installs == nil {
		return out
	}
	rows, err := s.installs.ListByTenant(ctx, tenantID)
	if err != nil {
		logger.Warnf(ctx, "[skill-market] listing the expert install ledger failed: %v", err)
		return out
	}
	for _, row := range rows {
		out[row.Slug] = true
	}
	return out
}

// skillsetDetailError maps a skillset detail failure: not-found and invalid
// slug keep their dedicated statuses, everything else is an unreachable 503.
func skillsetDetailError(slug string, err error) error {
	if errors.Is(err, skillhub.ErrSkillsetNotFound) {
		return apperrors.NewNotFoundError("skillset not found: " + slug)
	}
	if errors.Is(err, skillhub.ErrInvalidSkillsetSlug) {
		return apperrors.NewBadRequestError(err.Error())
	}
	return skillMarketListingError("skillset "+slug, err)
}

// rawLocalized picks one truthy localized string out of the raw registry
// entry (the same fields MaterializeSkillset consumes).
func rawLocalized(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case bool:
		if v {
			return "true"
		}
	case float64:
		return fmt.Sprintf("%g", v)
	}
	return ""
}

// ---------------------------------------------------------------------------
// skillset (expert) install
// ---------------------------------------------------------------------------

// InstallMarketSkillset implements interfaces.SkillMarketService. Flow (the
// binding M4 contract): SkillsetDetail → per-skill Download+ParsePackage →
// MaterializeSkillset → WriteMaterializedExpert → ledger upsert →
// ExpertService.Instantiate, all under the per-slug install lock. The write
// precedes Instantiate because Instantiate's catalog lookup scans the
// directory (per-call, no cache), so the tree must exist first; an Instantiate
// failure therefore leaves the skillset installed (and listed) for a retry —
// the materialized expert is tenant data, not a throwaway.
func (s *SkillMarketService) InstallMarketSkillset(
	ctx context.Context, tenantID uint64, slug string, req interfaces.InstantiateRequest,
) (*interfaces.MarketSkillsetInstallResult, error) {
	summary, err := s.skillset.SkillsetDetail(ctx, slug)
	if isHardMarketFailure(err) {
		return nil, skillsetDetailError(slug, err)
	}

	var result *interfaces.MarketSkillsetInstallResult
	err = s.withMarketInstallLock(ctx, tenantID, "skillset", summary.Slug, func(ctx context.Context) error {
		materialized, err := experts.MaterializeSkillset(summary, func(skillSlug string) (skillhub.ZipFiles, error) {
			archive, err := s.downloadPackage(ctx, skillSlug)
			if err != nil {
				return nil, err
			}
			files, err := skillhub.ParsePackage(archive)
			if err != nil {
				return nil, apperrors.NewBadRequestError(
					"the skill market package for " + skillSlug + " is invalid: " + err.Error())
			}
			return files, nil
		})
		if err != nil {
			return marketMaterializeError(err)
		}

		installDir := experts.MarketInstallDir(s.dataRoot, tenantID, summary.Slug)
		if err := experts.WriteMaterializedExpert(installDir, materialized); err != nil {
			return apperrors.NewInternalServerError("install the skillset expert: " + err.Error())
		}
		snapshot := materialized.SnapshotSHA256()

		createdBy, _ := types.UserIDFromContext(ctx)
		if err := s.installs.Upsert(ctx, &types.ExpertInstallEntity{
			ID:             uuid.NewString(),
			TenantID:       tenantID,
			Slug:           summary.Slug,
			StorageRef:     installDir,
			SnapshotSHA256: snapshot,
			CreatedBy:      createdBy,
		}); err != nil {
			return apperrors.NewInternalServerError("record the skillset install: " + err.Error())
		}

		expertID := skillhub.MarketExpertID(summary.Slug)
		instantiated, err := s.experts.Instantiate(ctx, tenantID, expertID, req)
		if err != nil {
			return marketInstantiateError(err)
		}
		result = &interfaces.MarketSkillsetInstallResult{
			InstantiateResult: *instantiated,
			ExpertID:          expertID,
			SnapshotSHA256:    snapshot,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// marketMaterializeError maps the materializer's wrapped failures: download
// and validation problems already carry app errors (or skillhub sentinels
// routed through the download mapping); the rest are bad requests naming the
// skillset.
func marketMaterializeError(err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, skillhub.ErrMarket) || errors.Is(err, skillhub.ErrPackage) {
		// A fetch failure that escaped the apperror mapping above (the
		// materializer wraps it with skillset/skill context).
		if errors.Is(err, skillhub.ErrPackage) {
			return apperrors.NewBadRequestError(err.Error())
		}
		return apperrors.NewServiceUnavailableError("the skill market is unreachable: " + err.Error())
	}
	return apperrors.NewBadRequestError(err.Error())
}

// marketInstantiateError keeps Instantiate's own rejections (expert-not-found
// surfaced as 404, CreateAgent validation as its app status) intact and wraps
// everything else as an internal error with context.
func marketInstantiateError(err error) error {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, ErrExpertNotFound) {
		return apperrors.NewInternalServerError(
			"the materialized expert vanished before instantiation: " + err.Error())
	}
	return apperrors.NewInternalServerError("instantiate the skillset expert: " + err.Error())
}
