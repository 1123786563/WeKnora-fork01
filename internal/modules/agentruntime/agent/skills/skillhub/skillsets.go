// Skillset index client (M4 Task 2): the SkillHub skillset listing/detail
// endpoints, ported from Octop's
// octop/infra/agents/experts/skillhub_market.py (_load_all_skillsets_uncached,
// fetch_skillset, _skillset_from_raw). A skillset is the expert-like asset
// SkillHub exposes — a curated list of skill slugs plus zh/en display
// metadata — and is the unit the expert market installs.
package skillhub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Skillset wire constants, ports of the Octop module constants.
const (
	skillsetsListEndpoint = "/api/v1/skillsets"
	// skillsetPageSize is the page size the listing walk requests (Octop:
	// _SKILLSET_PAGE_SIZE; the endpoint ignores limit= but honors page+pageSize).
	skillsetPageSize = 100
	// maxSkillsetPages bounds the pagination walk (Octop: _MAX_SKILLSET_PAGES).
	maxSkillsetPages = 20
)

// MarketExpertIDPrefix is prepended to a skillset slug to form the expert
// manifest ID of its materialized expert (Octop: MARKET_EXPERT_PREFIX). The
// prefix keeps market experts from ever colliding with builtin IDs.
const MarketExpertIDPrefix = "skillhub-skillset-"

// skillsetSlugPattern is Octop's _SLUG_RE: slugs are strictly
// [A-Za-z0-9_.-]+, which is what lets a slug be used both as a path segment
// against the registry and as a directory name in tenant storage.
var skillsetSlugPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// ErrInvalidSkillsetSlug: a skillset slug failed the strict slug pattern.
var ErrInvalidSkillsetSlug = fmt.Errorf("%w: invalid skillset slug", ErrMarket)

// ErrSkillsetNotFound: the registry answered a detail lookup without the
// requested skillset (missing payload or a slug-less document).
var ErrSkillsetNotFound = fmt.Errorf("%w: skillset not found", ErrMarket)

// ErrNoSkillsetClient: the wrapped client does not implement the skillset
// operations (CachedClient only proxies them when its inner does).
var ErrNoSkillsetClient = fmt.Errorf("%w: inner client does not support skillset operations", ErrMarket)

// MarketExpertID returns the expert manifest ID for one skillset slug.
func MarketExpertID(slug string) string {
	return MarketExpertIDPrefix + slug
}

// MarketExpertSlugFromID splits a market expert ID back into its skillset
// slug; ok is false for every other ID (builtins included).
func MarketExpertSlugFromID(id string) (string, bool) {
	if strings.HasPrefix(id, MarketExpertIDPrefix) {
		return strings.TrimPrefix(id, MarketExpertIDPrefix), true
	}
	return "", false
}

// ValidateSkillsetSlug trims and validates one skillset slug (Octop:
// validate_skillset_slug). A slug outside [A-Za-z0-9_.-]+ is rejected before
// it can reach a URL path or the filesystem.
func ValidateSkillsetSlug(slug string) (string, error) {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" || !skillsetSlugPattern.MatchString(trimmed) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSkillsetSlug, slug)
	}
	return trimmed, nil
}

// SkillsetSummary is one skillset listing, the skillset counterpart of
// SkillSummary. Name maps displayName -> name -> slug and Description maps
// summary -> description exactly like the skill listing (T1's chains); the
// zh/en originals stay available in Raw (displayNameEn, summaryEn, scene,
// ...) for consumers that need per-locale text — expert materialization in
// internal/agent/experts is the one today. Raw carries the original JSON
// values, stringified the way jsonValueString does.
type SkillsetSummary struct {
	Slug        string
	Name        string
	Description string
	SkillSlugs  []string
	Raw         map[string]any
}

// SkillsetClient is the skillset index surface: the full paginated listing
// and one-slug detail. HTTPClient and CachedClient implement it alongside
// the T1 Client interface.
type SkillsetClient interface {
	// SkillsetIndex walks every page of the registry listing, deduplicating
	// by slug, until a short page, the declared total or the page cap ends
	// the walk.
	SkillsetIndex(ctx context.Context) ([]SkillsetSummary, error)
	// SkillsetDetail fetches one skillset document; the slug is validated
	// before any request is made.
	SkillsetDetail(ctx context.Context, slug string) (SkillsetSummary, error)
}

// SkillsetIndex implements SkillsetClient on the direct HTTP client. The
// walk mirrors _load_all_skillsets_uncached: the endpoint defaults to 20 rows
// and ignores limit=, so page+pageSize is the supported shape; entries with
// a blank or already-seen slug are skipped; the walk stops on an empty page,
// a page whose deduped count reached the declared total, or a page shorter
// than the requested size. At most maxSkillsetPages pages are requested.
func (c *HTTPClient) SkillsetIndex(ctx context.Context) ([]SkillsetSummary, error) {
	var items []SkillsetSummary
	seen := make(map[string]bool)
	var total int64 = -1

	for page := 1; page <= maxSkillsetPages; page++ {
		payload, err := c.get(ctx, skillsetsListEndpoint,
			url.Values{
				"page":     {strconv.Itoa(page)},
				"pageSize": {strconv.Itoa(skillsetPageSize)},
			},
			"application/json", maxJSONBytes)
		if err != nil {
			return nil, err
		}
		rawItems, declared, err := parseSkillsetsPage(payload)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			total = declared
		}

		for _, raw := range rawItems {
			summary := skillsetFromRaw(raw)
			if summary.Slug == "" || seen[summary.Slug] {
				continue
			}
			seen[summary.Slug] = true
			items = append(items, summary)
		}

		if len(rawItems) == 0 {
			break
		}
		if total > 0 && int64(len(items)) >= total {
			break
		}
		if len(rawItems) < skillsetPageSize {
			break
		}
	}
	return items, nil
}

// SkillsetDetail implements SkillsetClient on the direct HTTP client
// (Octop: fetch_skillset). A document that is not an object, or that carries
// no slug, is a not-found answer, matching Octop's NOT_FOUND mapping.
func (c *HTTPClient) SkillsetDetail(ctx context.Context, slug string) (SkillsetSummary, error) {
	safeSlug, err := ValidateSkillsetSlug(slug)
	if err != nil {
		return SkillsetSummary{}, err
	}
	payload, err := c.get(ctx, skillsetsListEndpoint+"/"+url.PathEscape(safeSlug), nil,
		"application/json", maxJSONBytes)
	if err != nil {
		return SkillsetSummary{}, err
	}
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return SkillsetSummary{}, fmt.Errorf("%w: invalid JSON from SkillHub skillset detail: %v", ErrMarket, err)
	}
	entry, ok := document.(map[string]any)
	if !ok {
		return SkillsetSummary{}, fmt.Errorf("%w: %q", ErrSkillsetNotFound, safeSlug)
	}
	summary := skillsetFromRaw(entry)
	if summary.Slug == "" {
		return SkillsetSummary{}, fmt.Errorf("%w: %q", ErrSkillsetNotFound, safeSlug)
	}
	return summary, nil
}

// parseSkillsetsPage decodes one listing page: a JSON object whose "skillSets"
// value is an array (Octop: data.get("skillSets")). The declared "total" is
// returned when present and positive, -1 otherwise.
func parseSkillsetsPage(payload []byte) ([]map[string]any, int64, error) {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, 0, fmt.Errorf("%w: invalid JSON from SkillHub skillsets: %v", ErrMarket, err)
	}
	object, ok := document.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("%w: SkillHub skillsets response must contain a skillSets array", ErrMarket)
	}
	rawResults, ok := object["skillSets"].([]any)
	if !ok {
		return nil, 0, fmt.Errorf("%w: SkillHub skillsets response must contain a skillSets array", ErrMarket)
	}

	entries := make([]map[string]any, 0, len(rawResults))
	for _, raw := range rawResults {
		if entry, ok := raw.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}

	declared := int64(-1)
	if total, ok := object["total"]; ok && isTruthyJSON(total) {
		if n, err := jsonNumber(total); err == nil && n > 0 {
			declared = n
		}
	}
	return entries, declared, nil
}

// jsonNumber coerces a JSON scalar to a positive-capable int64 (Octop:
// _coerce_positive_int — int(value) semantics, truthiness already checked).
func jsonNumber(value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("not a number: %T", value)
	}
}

// skillsetFromRaw normalizes one raw skillset entry (Octop:
// _skillset_from_raw). The name/description chains follow the T1 skill
// listing mapping (displayName -> name -> slug; summary -> description) so
// both listings behave identically; SkillSlugs are trimmed with blanks
// dropped, in entry order.
func skillsetFromRaw(entry map[string]any) SkillsetSummary {
	slug := strings.TrimSpace(firstTruthyString(entry, "slug"))

	name := firstTruthyString(entry, "displayName")
	if name == "" {
		name = firstTruthyString(entry, "name")
	}
	if name == "" {
		name = slug
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = slug
	}

	var skillSlugs []string
	if raw, ok := entry["skillSlugs"].([]any); ok {
		for _, item := range raw {
			if trimmed := strings.TrimSpace(jsonValueString(item)); trimmed != "" {
				skillSlugs = append(skillSlugs, trimmed)
			}
		}
	}

	return SkillsetSummary{
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(firstTruthyString(entry, "summary", "description")),
		SkillSlugs:  skillSlugs,
		Raw:         entry,
	}
}

// SkillsetIndex implements SkillsetClient on the cached wrapper.
func (c *CachedClient) SkillsetIndex(ctx context.Context) ([]SkillsetSummary, error) {
	if c.skillsets == nil {
		return nil, ErrNoSkillsetClient
	}
	value, err := c.do(ctx, cacheKey("skillset-index"), func() (any, error) {
		return c.skillsets.SkillsetIndex(ctx)
	})
	if value == nil {
		return nil, err
	}
	results, _ := value.([]SkillsetSummary)
	return results, err
}

// SkillsetDetail implements SkillsetClient on the cached wrapper. The slug
// is validated before the cache so an invalid slug never populates an entry.
func (c *CachedClient) SkillsetDetail(ctx context.Context, slug string) (SkillsetSummary, error) {
	if c.skillsets == nil {
		return SkillsetSummary{}, ErrNoSkillsetClient
	}
	if _, err := ValidateSkillsetSlug(slug); err != nil {
		return SkillsetSummary{}, err
	}
	value, err := c.do(ctx, cacheKey("skillset-detail", strings.TrimSpace(slug)), func() (any, error) {
		return c.skillsets.SkillsetDetail(ctx, slug)
	})
	if value == nil {
		return SkillsetSummary{}, err
	}
	summary, _ := value.(SkillsetSummary)
	return summary, err
}
