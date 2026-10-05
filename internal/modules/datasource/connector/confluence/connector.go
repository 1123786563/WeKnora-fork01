// Package confluence imports Confluence Server/Data Center and Cloud spaces as Markdown.
package confluence

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

var (
	_ datasource.StreamingConnector     = (*Connector)(nil)
	_ datasource.FullStreamingConnector = (*Connector)(nil)
)

// markdownConverter is the shared HTML→Markdown converter for page bodies. It
// keeps the baseline of the package-level htmltomd.ConvertString helper (base +
// commonmark) and additionally enables the table and strikethrough plugins the
// baseline lacks: without them Confluence tables were flattened to run-on text
// and <del>/<s>/<strike> lost their meaning. A Converter is safe for concurrent
// use, so it is built once and reused for every page.
var markdownConverter = converter.NewConverter(
	converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
		strikethrough.NewStrikethroughPlugin(),
	),
)

// Connector implements datasource.StreamingConnector for Confluence.
type Connector struct {
	newClient  func(config) (*client, error)
	spaceCache spaceCache
}

const pickerSpaceCacheTTL = time.Minute

type spaceCache struct {
	mu      sync.Mutex
	entries map[string]cachedSpaces
}

type cachedSpaces struct {
	values  []space
	expires time.Time
}

// NewConnector creates a Confluence connector.
func NewConnector() *Connector { return &Connector{newClient: newClient} }

func (c *Connector) pickerSpaces(ctx context.Context, client *client, cfg config) ([]space, error) {
	key := pickerSpaceCacheKey(cfg)
	now := time.Now()
	c.spaceCache.mu.Lock()
	if cached, ok := c.spaceCache.entries[key]; ok && now.Before(cached.expires) {
		values := append([]space(nil), cached.values...)
		c.spaceCache.mu.Unlock()
		return values, nil
	}
	c.spaceCache.mu.Unlock()
	spaces, err := client.spaces(ctx)
	if err != nil {
		return nil, err
	}
	c.spaceCache.mu.Lock()
	if c.spaceCache.entries == nil {
		c.spaceCache.entries = make(map[string]cachedSpaces)
	}
	for key, cached := range c.spaceCache.entries {
		if !now.Before(cached.expires) {
			delete(c.spaceCache.entries, key)
		}
	}
	c.spaceCache.entries[key] = cachedSpaces{
		values:  append([]space(nil), spaces...),
		expires: now.Add(pickerSpaceCacheTTL),
	}
	c.spaceCache.mu.Unlock()
	return spaces, nil
}

// pickerSpaceCacheKey partitions cached space visibility by every credential
// component that can affect permissions. The secret is fingerprinted so it
// never becomes a map key retained in process memory or appears in diagnostics.
func pickerSpaceCacheKey(cfg config) string {
	fingerprint := sha256.Sum256([]byte(cfg.secret))
	return cfg.edition + "\x00" + cfg.baseURL + "\x00" + cfg.username + "\x00" + fmt.Sprintf("%x", fingerprint)
}

// Type returns the connector type identifier.
func (*Connector) Type() string { return types.ConnectorTypeConfluence }

func (c *Connector) configured(ds *types.DataSourceConfig) (*client, config, error) {
	cfg, err := parseConfig(ds)
	if err != nil {
		return nil, config{}, err
	}
	factory := c.newClient
	if factory == nil {
		factory = newClient
	}
	client, err := factory(cfg)
	return client, cfg, err
}

// Validate pings Confluence with the supplied credentials.
func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	client, _, err := c.configured(ds)
	if err != nil {
		return err
	}
	return client.ping(ctx)
}

// ListResources exposes a lazy Space → Page tree. It never scans a whole
// space merely to render the picker.
func (c *Connector) ListResources(
	ctx context.Context, ds *types.DataSourceConfig, parentID string,
) ([]types.Resource, error) {
	client, cfg, err := c.configured(ds)
	if err != nil {
		return nil, err
	}
	if parentID == "" {
		spaces, err := c.pickerSpaces(ctx, client, cfg)
		if err != nil {
			return nil, err
		}
		out := make([]types.Resource, 0, len(spaces))
		for _, s := range spaces {
			metadata := map[string]interface{}{"space_key": s.Key}
			if cfg.cloud() {
				// Cloud cannot enumerate a space's top-level folders, so pages under
				// them are unselectable in the picker; the frontend surfaces this
				// limitation when the space is expanded.
				metadata["hierarchy_limitation"] = "cloud_top_level_containers"
			}
			out = append(out, types.Resource{
				ExternalID: s.ID, Name: s.Name, Type: "space",
				URL: client.resourceURL(s.Links.WebUI), HasChildren: true,
				Metadata: metadata,
			})
		}
		return out, nil
	}
	ref, err := parseResourceID(parentID)
	if err != nil {
		return nil, err
	}
	spaces, err := c.pickerSpaces(ctx, client, cfg)
	if err != nil {
		return nil, err
	}
	var selected space
	found := false
	for _, s := range spaces {
		if s.ID == ref.SpaceID {
			selected, found = s, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("confluence space %s is unavailable", ref.SpaceID)
	}
	var pages []page
	if ref.Kind == resourceSpace {
		pages, err = client.topLevelPages(ctx, selected)
	} else {
		if ref.Kind == resourcePage {
			if err := client.validatePageOwnership(ctx, selected, ref.PageID); err != nil {
				return nil, fmt.Errorf("validate Confluence page %s: %w", ref.PageID, err)
			}
		}
		pages, err = client.directChildPages(ctx, selected, ref.PageID)
	}
	if err != nil {
		return nil, fmt.Errorf("list Confluence resources under %s: %w", parentID, err)
	}
	out := make([]types.Resource, 0, len(pages))
	for _, p := range pages {
		out = append(out, pageResource(client, selected, parentID, p))
	}
	return out, nil
}

func pageResource(client *client, s space, parentID string, p page) types.Resource {
	return types.Resource{
		ExternalID: makePageResourceID(s.ID, p.ID), ParentID: parentID, Name: p.Title, Type: "page",
		URL: client.resourceURL(p.Links.WebUI), HasChildren: true,
		Metadata: map[string]interface{}{"space_id": s.ID, "page_id": p.ID},
	}
}

// ResolveResourceAncestors expands only the O(depth) path required to reveal
// persisted page selections in the lazy picker. Resolution is best effort:
// a deleted page, an unavailable space, or a malformed id skips just its own
// reveal path instead of failing every saved selection; the sync path keeps
// its strict validation in buildSyncPlan.
func (c *Connector) ResolveResourceAncestors(
	ctx context.Context,
	ds *types.DataSourceConfig,
	resourceIDs []string,
) ([]string, error) {
	client, _, err := c.configured(ds)
	if err != nil {
		return nil, err
	}
	spaces, err := client.spaces(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]space, len(spaces))
	for _, s := range spaces {
		byID[s.ID] = s
	}
	seen, out := map[string]bool{}, []string{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range resourceIDs {
		ref, err := parseResourceID(id)
		if err != nil {
			logger.Warnf(ctx, "skip malformed Confluence resource %q during ancestor resolution: %v", id, err)
			continue
		}
		if ref.Kind == resourceSpace {
			continue
		}
		s, ok := byID[ref.SpaceID]
		if !ok {
			logger.Warnf(ctx, "skip Confluence resource %s: space %s is unavailable", id, ref.SpaceID)
			continue
		}
		ancestors, err := client.pageAncestors(ctx, s, ref.PageID)
		if err != nil {
			logger.Warnf(ctx, "skip Confluence resource %s during ancestor resolution: %v", id, err)
			continue
		}
		add(s.ID)
		for _, ancestor := range ancestors {
			if ancestor.Kind != "" && ancestor.Kind != "page" {
				continue
			}
			add(makePageResourceID(s.ID, ancestor.ID))
		}
	}
	return out, nil
}

// FetchAll syncs every selected space. Deletion reconciliation needs FetchStream.
func (c *Connector) FetchAll(
	ctx context.Context, ds *types.DataSourceConfig, resourceIDs []string,
) ([]types.FetchedItem, error) {
	dsCopy := *ds
	dsCopy.ResourceIDs = resourceIDs
	return c.collect(ctx, &dsCopy, nil)
}

// FetchIncremental syncs pages whose versions changed since the last cursor.
func (c *Connector) FetchIncremental(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	items, next, err := c.collectWithCursor(ctx, ds, old)
	return items, next, err
}

type collector struct{ items []types.FetchedItem }

func (h *collector) Emit(_ context.Context, item types.FetchedItem) error {
	h.items = append(h.items, item)
	return nil
}
func (*collector) Checkpoint(context.Context, *types.SyncCursor) error { return nil }

func (c *Connector) collect(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, error) {
	items, _, err := c.collectWithCursor(ctx, ds, old)
	return items, err
}

func (c *Connector) collectWithCursor(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	h := &collector{}
	next, err := c.FetchStream(ctx, ds, old, h)
	return h.items, next, err
}

// FetchStream is the one sync engine. A version enters its cursor only after
// its Markdown has been fetched and Emit has succeeded, making checkpoints safe.
func (c *Connector) FetchStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	return c.fetchStream(ctx, ds, old, h, false)
}

// FetchFullStream forces every page to be re-fetched while retaining old solely
// as a complete baseline for deletion reconciliation. Mid-run checkpoints keep
// that baseline plus the pages already re-fetched so Asynq retries resume
// instead of starting over.
func (c *Connector) FetchFullStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	return c.fetchStream(ctx, ds, old, h, true)
}

func (c *Connector) fetchStream(
	ctx context.Context,
	ds *types.DataSourceConfig,
	old *types.SyncCursor,
	h datasource.StreamHandler,
	forceFull bool,
) (*types.SyncCursor, error) {
	if ds == nil || len(ds.ResourceIDs) == 0 {
		return nil, fmt.Errorf("confluence requires at least one selected space")
	}
	client, _, err := c.configured(ds)
	if err != nil {
		return nil, err
	}
	plan, err := c.buildSyncPlan(ctx, client, ds.ResourceIDs)
	if err != nil {
		return nil, err
	}
	currentRoots := make(map[string]bool, len(plan.Roots))
	for _, root := range plan.Roots {
		currentRoots[root.ResourceID] = true
	}
	baseline, next := prepareSyncCursors(old, forceFull)
	// A prior successful body is reusable when ownership moves to a different
	// selected root and this run cannot fetch the new body. Keep that version in
	// the new root rather than dropping it during ownership reconciliation.
	priorVersionByPage := make(map[string]string)
	for _, pages := range baseline.SpacePages {
		for id, version := range pages {
			if _, exists := priorVersionByPage[id]; !exists {
				priorVersionByPage[id] = version
			}
		}
	}
	// A page may be returned by multiple overlapping roots (for example during
	// a configuration change). Fetch it once, while retaining a cursor entry
	// per scope root so existing cursor JSON remains compatible.
	seenPages := make(map[string]struct{})
	incompleteRoots := make(map[string]bool)
	incompleteSpaces := make([]space, 0)
	for _, root := range plan.Roots {
		resourceID, s := root.ResourceID, root.Space
		var pages []page
		complete := true
		if root.Kind == syncWholeSpace {
			pages, complete, err = client.pages(ctx, s)
		} else {
			pages, err = client.pageSubtree(ctx, s, root.PageID)
		}
		if err != nil {
			return nil, fmt.Errorf("list pages in Confluence scope %s: %w", resourceID, err)
		}
		priorPages := baseline.SpacePages[resourceID]
		if !complete {
			logger.Warnf(
				ctx,
				"[Confluence] space %s listing was cut short; checking unlisted pages before deleting",
				s.Key,
			)
			incompleteRoots[resourceID] = true
			incompleteSpaces = append(incompleteSpaces, s)
		}
		if next.SpacePages[resourceID] == nil {
			next.SpacePages[resourceID] = map[string]string{}
		}
		if complete && len(pages) == 0 && len(priorPages) > 0 {
			return nil, fmt.Errorf(
				"refusing Confluence mirror deletion in %s: listing returned 0 pages against a %d-page baseline",
				s.Key, len(priorPages),
			)
		}
		for _, summary := range pages {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if _, duplicate := seenPages[summary.ID]; duplicate {
				// The first scope already owns the emitted document; preserve this
				// root's version record so a later selection change stays incremental.
				if !forceFull {
					if version, ok := priorPages[summary.ID]; ok {
						next.SpacePages[resourceID][summary.ID] = version
					}
				}
				continue
			}
			seenPages[summary.ID] = struct{}{}
			version, versionKnown := pageVersion(summary)
			// An unknown version cannot prove a page unchanged, so it must
			// always refresh the body instead of skipping on a stable "t:".
			if versionKnown {
				if forceFull {
					if next.SpacePages[resourceID][summary.ID] == version {
						continue
					}
				} else if priorPages[summary.ID] == version {
					continue
				}
			}
			full, err := client.body(ctx, summary.ID)
			if err != nil {
				if ctx.Err() != nil {
					return nil, err
				}
				if emitErr := h.Emit(ctx, failedPageItem(resourceID, summary, err)); emitErr != nil {
					return nil, emitErr
				}
				if !forceFull {
					if prior, ok := priorVersionByPage[summary.ID]; ok {
						next.SpacePages[resourceID][summary.ID] = prior
					}
				}
				continue
			}
			item, err := markdownItem(ctx, client, resourceID, summary, full)
			if err != nil {
				if emitErr := h.Emit(ctx, failedPageItem(resourceID, summary, err)); emitErr != nil {
					return nil, emitErr
				}
				if !forceFull {
					if prior, ok := priorVersionByPage[summary.ID]; ok {
						next.SpacePages[resourceID][summary.ID] = prior
					}
				}
				continue
			}
			if err := h.Emit(ctx, item); err != nil {
				return nil, err
			}
			next.SpacePages[resourceID][summary.ID] = version
			if err := h.Checkpoint(ctx, next.syncCursor()); err != nil {
				return nil, err
			}
		}
	}
	// Reconcile previous owners only after every current scope has been enumerated.
	// A page may move between roots, so a missing page must be checked against every
	// scope whose listing was cut short before a tombstone can be emitted.
	previousPages := make(map[string]string)
	for owner, pages := range baseline.SpacePages {
		for id := range pages {
			if _, exists := previousPages[id]; !exists {
				previousPages[id] = owner
			}
		}
	}
	missing := make([]string, 0)
	probeCount := 0
	for id := range previousPages {
		if _, present := seenPages[id]; present {
			continue
		}
		owner := previousPages[id]
		for _, s := range incompleteSpaces {
			if probeCount >= maxDeletionProbes {
				return nil, fmt.Errorf(
					"Confluence deletion reconciliation deferred: more than %d pages need verification",
					maxDeletionProbes,
				)
			}
			probeCount++
			present, err := client.pageInSpace(ctx, id, s.Key)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, fmt.Errorf(
					"Confluence deletion reconciliation deferred: could not verify page %s in space %s: %w",
					id, s.Key, err,
				)
			}
			if present {
				version := baseline.SpacePages[owner][id]
				if next.SpacePages[s.ID] == nil {
					next.SpacePages[s.ID] = make(map[string]string)
				}
				next.SpacePages[s.ID][id] = version
				seenPages[id] = struct{}{}
				break
			}
		}
		if _, present := seenPages[id]; present {
			continue
		}
		missing = append(missing, id)
	}
	// Protect still-selected roots from a suspiciously large shrink. Removed
	// roots are an explicit scope change, so their out-of-scope pages can be
	// tombstoned once every current scope has been enumerated safely.
	owners := make([]string, 0, len(baseline.SpacePages))
	for owner := range baseline.SpacePages {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		// A deliberate scope change may remove most of an old root. The guard is
		// only for unexpected shrinkage of a still-selected, completely listed root.
		if !currentRoots[owner] || incompleteRoots[owner] {
			continue
		}
		pages := baseline.SpacePages[owner]
		missingInRoot := 0
		for id := range pages {
			if _, present := seenPages[id]; !present {
				missingInRoot++
			}
		}
		if len(pages) > 0 && missingInRoot >= 20 && missingInRoot*100 >= len(pages)*80 {
			return nil, fmt.Errorf(
				"refusing Confluence mirror deletion: %d/%d previously synced pages would be removed from scope %s",
				missingInRoot, len(pages), owner,
			)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		deleted := types.FetchedItem{
			ExternalID: id, IsDeleted: true, SourceResourceID: previousPages[id],
			Metadata: map[string]string{"channel": types.ChannelConfluence},
		}
		if err := h.Emit(ctx, deleted); err != nil {
			return nil, err
		}
		// Drop the tombstoned page before checkpointing so the cursor records
		// the deletion itself; an interrupted run resumes with the remaining
		// candidates instead of re-emitting completed tombstones.
		removePageOwnership(&next, id)
		if err := h.Checkpoint(ctx, next.syncCursor()); err != nil {
			return nil, err
		}
	}
	// Commit exactly the current ownership roots only after reconciliation. This
	// removes cancelled scopes while keeping the on-disk cursor field compatible.
	committedRoots := make(map[string]map[string]string, len(plan.Roots))
	for _, root := range plan.Roots {
		if pages := next.SpacePages[root.ResourceID]; pages != nil {
			committedRoots[root.ResourceID] = pages
		} else {
			committedRoots[root.ResourceID] = map[string]string{}
		}
	}
	next.SpacePages = committedRoots
	next.FullSync = false
	next.FullSyncBaseline = nil
	return next.syncCursor(), nil
}

func failedPageItem(resourceID string, summary page, err error) types.FetchedItem {
	code, reason := classifyConfluenceError(err)
	return types.FetchedItem{
		ExternalID:       summary.ID,
		Title:            summary.Title,
		SourceResourceID: resourceID,
		Metadata: map[string]string{
			"channel":           types.ChannelConfluence,
			"error":             err.Error(),
			"error_reason_code": code,
			"error_reason":      reason,
			"page_id":           summary.ID,
		},
	}
}

func classifyConfluenceError(err error) (code, reason string) {
	var api *apiError
	if errors.As(err, &api) {
		switch api.status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return "confluence_auth_or_permission",
				"Authentication or permission error; check credentials and space permissions"
		case http.StatusNotFound:
			return "confluence_not_found", "Confluence page was not found; will retry on the next sync"
		case http.StatusTooManyRequests:
			return "confluence_rate_limited", "Confluence API rate limited; will retry on the next sync"
		default:
			if api.status >= 500 {
				return "confluence_server_unavailable",
					"Confluence service temporarily unavailable; will retry on the next sync"
			}
			return "confluence_api_error", "Confluence API error; see server logs"
		}
	}
	return "confluence_sync_failed", "Confluence page could not be synced; see server logs"
}

func markdownItem(
	ctx context.Context,
	client *client,
	resourceID string,
	summary page,
	full pageBody,
) (types.FetchedItem, error) {
	html := full.Body.View.Value
	// Inline same-origin private Confluence images as data URIs so the generic
	// docparser image pipeline can persist them and rewrite to resource:// URLs.
	// Best-effort: a failed image keeps its original src and never fails the page.
	html = newAssetResolver(client).Resolve(ctx, html)
	// Images the resolver could not inline keep their original relative src, and
	// Confluence emits child-page links relative to the base URL; both would be
	// dead references outside the Confluence origin, so resolve them absolutely.
	html = client.absolutizeRefs(html)
	// Confluence renders draw.io and mermaid macros as inline <svg>, which the
	// converter silently drops; keep a visible placeholder instead.
	html = replaceInlineSVGs(html)
	markdown, err := markdownConverter.ConvertString(html)
	if err != nil {
		return types.FetchedItem{}, err
	}
	if strings.TrimSpace(markdown) == "" {
		markdown = "# " + summary.Title + "\n"
	}
	if full.Title != "" {
		summary.Title = full.Title
	}
	if full.Version.Number > 0 || full.Version.When != "" {
		summary.Version = full.Version
	}
	if full.Space.Key != "" {
		summary.Space = full.Space
	}
	if webui := strings.TrimSpace(summary.Links.WebUI); webui == "" && full.Links.WebUI != "" {
		summary.Links.WebUI = full.Links.WebUI
	}
	metadata := map[string]string{
		"channel": types.ChannelConfluence, "space_key": summary.Space.Key,
		"space_name": summary.Space.Name, "page_id": summary.ID,
	}
	if creator := strings.TrimSpace(summary.Version.By.DisplayName); creator != "" {
		metadata["creator"] = creator
	}
	return types.FetchedItem{
		ExternalID:       summary.ID,
		Title:            summary.Title,
		Content:          []byte(markdown),
		ContentType:      "text/markdown",
		FileName:         pageFileName(summary.Title, summary.ID),
		URL:              client.resourceURL(summary.Links.WebUI),
		UpdatedAt:        pageUpdatedAt(summary),
		SourceResourceID: resourceID,
		Metadata:         metadata,
	}, nil
}

// aTagRe matches a full <a ...> start tag, tolerating '>' inside quoted
// attribute values, mirroring imgTagRe in asset_resolver.go.
var aTagRe = regexp.MustCompile(`(?i)<a\b(?:[^>"']|"[^"]*"|'[^']*')*>`)

// hrefAttrRe captures the href attribute value of a start tag, supporting
// double-quoted, single-quoted, or bare values. Group 1 is the " href=" prefix,
// group 2 the value, matching srcAttrRe in asset_resolver.go.
var hrefAttrRe = regexp.MustCompile(`(?i)(\shref\s*=\s*)("[^"]*"|'[^']*'|[^\s"'>]+)`)

// absolutizeRefs resolves relative href (<a>) and src (<img>) values in a page
// body against the configured Confluence base URL, reusing the client's
// context-path aware endpoint resolution. Only rewritable relative references
// change; everything else is left byte-for-byte untouched so absolute links,
// in-page anchors, data URIs, and mailto targets survive conversion as-is.
func (c *client) absolutizeRefs(content string) string {
	content = aTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		return rewriteTagAttr(tag, hrefAttrRe, c.absoluteRef)
	})
	content = imgTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		return rewriteTagAttr(tag, srcAttrRe, c.absoluteRef)
	})
	return content
}

// rewriteTagAttr rewrites the attribute captured by attrRe inside a single
// start tag through resolve, preserving the rest of the tag byte-for-byte. A
// missing attribute or a value resolve returns unchanged leaves the tag as-is.
func rewriteTagAttr(tag string, attrRe *regexp.Regexp, resolve func(string) string) string {
	loc := attrRe.FindStringSubmatchIndex(tag)
	if loc == nil {
		return tag
	}
	raw := tag[loc[4]:loc[5]]
	if len(raw) >= 2 {
		if q := raw[0]; (q == '"' || q == '\'') && raw[len(raw)-1] == q {
			raw = raw[1 : len(raw)-1]
		}
	}
	value := strings.TrimSpace(html.UnescapeString(raw))
	if value == "" {
		return tag
	}
	resolved := resolve(value)
	if resolved == value {
		return tag
	}
	return tag[:loc[4]] + `"` + html.EscapeString(resolved) + `"` + tag[loc[5]:]
}

// absoluteRef resolves a relative http(s) reference against the configured
// Confluence base URL. Absolute URLs, protocol-relative references, in-page
// anchors, and non-http schemes are returned unchanged, as are references that
// fail resolution (for example they would leave the configured context path);
// a reference that cannot be absolutized keeps pointing where it pointed
// instead of failing the page.
func (c *client) absoluteRef(ref string) string {
	if strings.HasPrefix(ref, "#") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.Scheme != "" || u.Host != "" {
		return ref
	}
	resolved, err := c.resolveEndpoint(ref)
	if err != nil {
		return ref
	}
	// resolveEndpoint joins scheme, host, context path, and query, but drops
	// the fragment of a relative reference; re-attach it in original form.
	if frag := u.EscapedFragment(); frag != "" {
		resolved += "#" + frag
	}
	return resolved
}

// inlineSVGRe matches an inline <svg>...</svg> block. Confluence renders
// draw.io and mermaid macros this way, and the Markdown converter drops them
// silently, which used to lose the diagram without a trace.
var inlineSVGRe = regexp.MustCompile(`(?is)<svg\b.*?</svg>`)

var (
	// svgLabelAttrRe captures aria-label from an inline <svg> block.
	svgLabelAttrRe = regexp.MustCompile(`(?i)\saria-label\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	// svgNameAttrRe captures data-name from the start tag wrapping an <svg>.
	svgNameAttrRe = regexp.MustCompile(`(?i)\sdata-name\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

// svgParentLookback bounds how far before an <svg> start the immediately
// preceding tag is searched for a wrapping container's data-name.
const svgParentLookback = 256

// replaceInlineSVGs replaces every inline <svg> block with a textual
// "[diagram: name]" placeholder instead of silently losing the diagram. The
// label comes from the svg's aria-label, else the wrapping start tag's
// data-name, else the 1-based position of the diagram within the page.
func replaceInlineSVGs(content string) string {
	matches := inlineSVGRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var b strings.Builder
	b.Grow(len(content))
	last := 0
	for i, loc := range matches {
		b.WriteString(content[last:loc[0]])
		b.WriteString("[diagram: " + html.EscapeString(svgDiagramName(content, loc, i+1)) + "]")
		last = loc[1]
	}
	b.WriteString(content[last:])
	return b.String()
}

// svgDiagramName picks the placeholder label for the <svg> at loc: aria-label
// on the svg itself, else data-name on the immediately preceding start tag
// (Confluence diagram macros wrap the svg in a labeled container), else the
// diagram's 1-based index in the page.
func svgDiagramName(content string, loc []int, index int) string {
	if name := attrValue(svgLabelAttrRe.FindStringSubmatch(content[loc[0]:loc[1]])); name != "" {
		return name
	}
	windowStart := loc[0] - svgParentLookback
	if windowStart < 0 {
		windowStart = 0
	}
	window := content[windowStart:loc[0]]
	if tagStart := strings.LastIndexByte(window, '<'); tagStart >= 0 {
		if name := attrValue(svgNameAttrRe.FindStringSubmatch(window[tagStart:])); name != "" {
			return name
		}
	}
	return strconv.Itoa(index)
}

// attrValue returns the trimmed, HTML-unescaped first capture of an attribute
// regexp match, stripping optional surrounding quotes; "" when there is none.
func attrValue(m []string) string {
	if len(m) < 2 {
		return ""
	}
	value := m[1]
	if len(value) >= 2 {
		if q := value[0]; (q == '"' || q == '\'') && value[len(value)-1] == q {
			value = value[1 : len(value)-1]
		}
	}
	return strings.TrimSpace(html.UnescapeString(value))
}
