package rss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/mmcdole/gofeed"
)

// TestMain whitelists loopback for SSRF so the httptest servers (127.0.0.1)
// are reachable. Production keeps the default strict SSRF policy.
func TestMain(m *testing.M) {
	_ = os.Setenv("SSRF_WHITELIST", "127.0.0.1,::1")
	utils.ResetSSRFWhitelistForTest()
	code := m.Run()
	os.Exit(code)
}

const longArticleBody = `<p>This is the first paragraph of a reasonably long article that the ` +
	`readability extractor should detect as the main content of the page. It contains ` +
	`enough words to clear the minimum content threshold used by the algorithm.</p>` +
	`<p>The second paragraph continues the discussion with more sentences so that the ` +
	`scoring heuristics confidently select this block over navigation and footer noise.</p>` +
	`<p>A third paragraph adds further substance, ensuring the article is unmistakably ` +
	`the dominant readable region of the document under test.</p>`

// fakeFeed spins up an httptest server serving an RSS feed and article pages.
type fakeFeed struct {
	server             *httptest.Server
	feedTitle          string
	itemContent        string // optional <description>/content for items
	item1Title         string // optional override of item 1's <title>
	item1Path          string // optional override of item 1's <link> path (under server root)
	item1Desc          string // optional override of item 1's <description>
	item1Article       string // optional override of item 1's article page body (path /article/a1 only)
	articleAuthHeaders []string
	articleFetches     atomic.Int32
	failFeed           atomic.Bool
}

func newFakeFeed(t *testing.T) *fakeFeed {
	t.Helper()
	f := &fakeFeed{feedTitle: "Test Feed"}
	mux := http.NewServeMux()

	mux.HandleFunc("/article/", func(w http.ResponseWriter, r *http.Request) {
		f.articleFetches.Add(1)
		for k, vals := range r.Header {
			if strings.EqualFold(k, "X-Test-Auth") && len(vals) > 0 && vals[0] != "" {
				f.articleAuthHeaders = append(f.articleAuthHeaders, vals[0])
			}
		}
		body := longArticleBody
		if r.URL.Path == "/article/a1" && f.item1Article != "" {
			body = f.item1Article
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s</title></head>`+
			`<body><nav>menu</nav><article><h1>Heading</h1>%s</article><footer>foot</footer></body></html>`,
			"Full "+r.URL.Path, body)
	})

	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		if f.failFeed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if auth := r.Header.Get("X-Test-Auth"); f.itemContent == "needs-auth" && auth != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		base := "http://" + r.Host
		desc := "summary fallback"
		title1 := f.item1Title
		if title1 == "" {
			title1 = "Article One"
		}
		path1 := f.item1Path
		if path1 == "" {
			path1 = "/article/a1"
		}
		desc1 := f.item1Desc
		if desc1 == "" {
			desc1 = desc
		}
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0"><channel>
<title>%s</title>
<link>%s</link>
<description>A test feed</description>
<item>
  <title>%s</title>
  <link>%s%s</link>
  <guid>guid-1</guid>
  <pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
  <description>%s</description>
</item>
<item>
  <title>Article Two</title>
  <link>%s/article/a2</link>
  <guid>guid-2</guid>
  <pubDate>Tue, 03 Jan 2006 15:04:05 GMT</pubDate>
  <description>%s</description>
</item>
</channel></rss>`, f.feedTitle, base, title1, base, path1, desc1, base, desc)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeFeed) feedURL() string { return f.server.URL + "/feed.xml" }

func makeConfig(feedURLs string, headers string) *types.DataSourceConfig {
	cfg := &types.DataSourceConfig{
		Type: types.ConnectorTypeRSS,
		Settings: map[string]interface{}{
			"feed_urls": feedURLs,
		},
		Credentials: map[string]interface{}{},
	}
	if headers != "" {
		cfg.Credentials["auth_headers"] = headers
	}
	return cfg
}

func TestConnector_Type(t *testing.T) {
	if NewConnector().Type() != types.ConnectorTypeRSS {
		t.Fatalf("Type() = %q, want %q", NewConnector().Type(), types.ConnectorTypeRSS)
	}
}

func TestParseConfig_RequiresFeedURLs(t *testing.T) {
	if _, err := parseConfig(makeConfig("   ", "")); err == nil {
		t.Fatal("expected error when feed_urls is blank")
	}
}

func TestParseConfig_FeedURLsFromSettings(t *testing.T) {
	cfg, err := parseConfig(makeConfig("https://example.com/feed.xml", ""))
	if err != nil {
		t.Fatalf("parseConfig error: %v", err)
	}
	got := cfg.feedURLList()
	if len(got) != 1 || got[0] != "https://example.com/feed.xml" {
		t.Fatalf("feedURLList = %v", got)
	}
}

func TestParseConfig_LegacyFeedURLsInCredentials(t *testing.T) {
	legacy := &types.DataSourceConfig{
		Type: types.ConnectorTypeRSS,
		Credentials: map[string]interface{}{
			"feed_urls": "https://legacy.example/feed.xml",
		},
	}
	cfg, err := parseConfig(legacy)
	if err != nil {
		t.Fatalf("parseConfig error: %v", err)
	}
	if got := cfg.feedURLList(); len(got) != 1 || got[0] != "https://legacy.example/feed.xml" {
		t.Fatalf("feedURLList = %v", got)
	}
}

func TestConfig_FeedURLList_SplitsAndDedupes(t *testing.T) {
	cfg := &Config{FeedURLs: "https://a.com/f, https://b.com/f\nhttps://a.com/f\n"}
	got := cfg.feedURLList()
	want := []string{"https://a.com/f", "https://b.com/f"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("feedURLList = %v, want %v", got, want)
	}
}

func TestConfig_ParseHeaders(t *testing.T) {
	cfg := &Config{AuthHeaders: "Authorization: Bearer x\nX-Foo:  bar \nbroken-line\n: noname"}
	got := cfg.parseHeaders()
	if got["Authorization"] != "Bearer x" {
		t.Errorf("Authorization = %q", got["Authorization"])
	}
	if got["X-Foo"] != "bar" {
		t.Errorf("X-Foo = %q", got["X-Foo"])
	}
	if len(got) != 2 {
		t.Errorf("expected 2 headers, got %d: %v", len(got), got)
	}
}

func TestFeedSignalFingerprint(t *testing.T) {
	item := &gofeed.Item{GUID: "g1", Link: "https://example.com/a", Title: "t"}
	sig1 := feedSignalFingerprint(item, "body")
	sig2 := feedSignalFingerprint(item, "body")
	if sig1 == "" || sig1 != sig2 {
		t.Fatalf("feed signal unstable: %q vs %q", sig1, sig2)
	}
	sig3 := feedSignalFingerprint(item, "changed")
	if sig1 == sig3 {
		t.Fatal("expected different signal when feed content changes")
	}
}

func TestItemFingerprint(t *testing.T) {
	base := itemFingerprint("Title", "https://example.com/a", "body")
	if base == "" || base != itemFingerprint("Title", "https://example.com/a", "body") {
		t.Fatalf("item fingerprint unstable: %q", base)
	}
	if base == itemFingerprint("New Title", "https://example.com/a", "body") {
		t.Fatal("expected different fingerprint on title-only change")
	}
	if base == itemFingerprint("Title", "https://example.com/b", "body") {
		t.Fatal("expected different fingerprint on link-only change")
	}
	if base == itemFingerprint("Title", "https://example.com/a", "changed") {
		t.Fatal("expected different fingerprint on body change")
	}
}

func TestItemExternalID_ScopesByFeed(t *testing.T) {
	got := itemExternalID("https://a.com/feed", "guid-1")
	want := "https://a.com/feed:guid-1"
	if got != want {
		t.Fatalf("itemExternalID = %q, want %q", got, want)
	}
}

func TestConnector_Validate_Success(t *testing.T) {
	f := newFakeFeed(t)
	if err := NewConnector().Validate(context.Background(), makeConfig(f.feedURL(), "")); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}

func TestConnector_Validate_PrivateFeedWithHeader(t *testing.T) {
	f := newFakeFeed(t)
	f.itemContent = "needs-auth"

	// Without the header → 401.
	if err := NewConnector().Validate(context.Background(), makeConfig(f.feedURL(), "")); err == nil {
		t.Fatal("expected error without auth header")
	}
	// With the header → success.
	if err := NewConnector().Validate(
		context.Background(), makeConfig(f.feedURL(), "X-Test-Auth: secret"),
	); err != nil {
		t.Fatalf("Validate with header error: %v", err)
	}
}

func TestConnector_FetchAll_DoesNotSendAuthHeadersToArticles(t *testing.T) {
	f := newFakeFeed(t)
	f.itemContent = "needs-auth"
	_, err := NewConnector().FetchAll(
		context.Background(), makeConfig(f.feedURL(), "X-Test-Auth: secret"), nil,
	)
	if err != nil {
		t.Fatalf("FetchAll error: %v", err)
	}
	if len(f.articleAuthHeaders) != 0 {
		t.Fatalf("article requests must not carry feed auth headers, got %v", f.articleAuthHeaders)
	}
}

func TestConnector_ListResources(t *testing.T) {
	f := newFakeFeed(t)
	res, err := NewConnector().ListResources(context.Background(), makeConfig(f.feedURL(), ""), "")
	if err != nil {
		t.Fatalf("ListResources error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(res))
	}
	if res[0].ExternalID != f.feedURL() {
		t.Errorf("ExternalID = %q, want %q", res[0].ExternalID, f.feedURL())
	}
	if res[0].Name != "Test Feed" {
		t.Errorf("Name = %q, want %q", res[0].Name, "Test Feed")
	}

	// Non-empty parentID → empty (feeds are flat).
	children, err := NewConnector().ListResources(context.Background(), makeConfig(f.feedURL(), ""), "feed-x")
	if err != nil {
		t.Fatalf("ListResources(parent) error: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("expected no children, got %d", len(children))
	}
}

func TestConnector_FetchAll_FullTextMarkdown(t *testing.T) {
	f := newFakeFeed(t)
	items, err := NewConnector().FetchAll(context.Background(), makeConfig(f.feedURL(), ""), nil)
	if err != nil {
		t.Fatalf("FetchAll error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	it := items[0]
	wantExternalID := itemExternalID(f.feedURL(), "guid-1")
	if it.ExternalID != wantExternalID {
		t.Errorf("ExternalID = %q, want %q", it.ExternalID, wantExternalID)
	}
	if it.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", it.ContentType)
	}
	if it.Metadata["channel"] != types.ChannelRSS {
		t.Errorf("channel = %q, want %q", it.Metadata["channel"], types.ChannelRSS)
	}
	// Full text from the article page should be present (not the short summary).
	if !strings.Contains(string(it.Content), "first paragraph") {
		t.Errorf("expected full article text in content, got: %q", string(it.Content))
	}
	if !strings.HasSuffix(it.FileName, ".md") {
		t.Errorf("FileName = %q, want .md suffix", it.FileName)
	}
}

func TestConnector_FetchIncremental_SkipsWithoutArticleFetch(t *testing.T) {
	f := newFakeFeed(t)
	cfg := makeConfig(f.feedURL(), "")
	cfg.ResourceIDs = []string{f.feedURL()}

	items, cursor, err := NewConnector().FetchIncremental(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first FetchIncremental error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first sync, got %d", len(items))
	}
	if got := f.articleFetches.Load(); got != 2 {
		t.Fatalf("first sync article fetches = %d, want 2", got)
	}

	f.articleFetches.Store(0)
	items2, _, err := NewConnector().FetchIncremental(context.Background(), cfg, cursor)
	if err != nil {
		t.Fatalf("second FetchIncremental error: %v", err)
	}
	if len(items2) != 0 {
		t.Fatalf("expected 0 items on unchanged second sync, got %d", len(items2))
	}
	if got := f.articleFetches.Load(); got != 0 {
		t.Fatalf("unchanged incremental sync must not refetch articles, got %d fetches", got)
	}
}

func TestConnector_FetchIncremental_SkipsUnchanged(t *testing.T) {
	f := newFakeFeed(t)
	cfg := makeConfig(f.feedURL(), "")
	cfg.ResourceIDs = []string{f.feedURL()}

	// First sync: everything is new.
	items, cursor, err := NewConnector().FetchIncremental(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first FetchIncremental error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first sync, got %d", len(items))
	}
	if cursor == nil {
		t.Fatal("expected non-nil cursor")
	}

	// Second sync with the returned cursor: nothing changed → no items.
	items2, _, err := NewConnector().FetchIncremental(context.Background(), cfg, cursor)
	if err != nil {
		t.Fatalf("second FetchIncremental error: %v", err)
	}
	if len(items2) != 0 {
		t.Fatalf("expected 0 items on unchanged second sync, got %d", len(items2))
	}
}

// syncIncremental runs one incremental sync round against feedURL.
func syncIncremental(t *testing.T, feedURL string, cursor *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor) {
	t.Helper()
	cfg := makeConfig(feedURL, "")
	cfg.ResourceIDs = []string{feedURL}
	items, next, err := NewConnector().FetchIncremental(context.Background(), cfg, cursor)
	if err != nil {
		t.Fatalf("FetchIncremental error: %v", err)
	}
	if next == nil {
		t.Fatal("expected non-nil cursor")
	}
	return items, next
}

// findByGUID picks the fetched item carrying the given feed GUID.
func findByGUID(items []types.FetchedItem, guid string) types.FetchedItem {
	for _, it := range items {
		if it.Metadata["guid"] == guid {
			return it
		}
	}
	return types.FetchedItem{}
}

// TestConnector_FetchIncremental_TitleOnlyChangeYieldsUpdate guards #3823: an
// entry whose <title> changed but whose body did not must surface exactly one
// update carrying the new title, with the Markdown body byte-identical so the
// downstream file hash and chunking stay untouched. The next unchanged sync
// must go back to zero updates (the new feed signal settles).
func TestConnector_FetchIncremental_TitleOnlyChangeYieldsUpdate(t *testing.T) {
	f := newFakeFeed(t)

	items, cursor := syncIncremental(t, f.feedURL(), nil)
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first sync, got %d", len(items))
	}
	before := findByGUID(items, "guid-1")
	if before.Title != "Article One" {
		t.Fatalf("fixture title = %q, want Article One", before.Title)
	}

	f.item1Title = "Article One (retitled)"
	items2, cursor2 := syncIncremental(t, f.feedURL(), cursor)
	if len(items2) != 1 {
		t.Fatalf("title-only change must yield exactly 1 update, got %d", len(items2))
	}
	got := items2[0]
	if got.ExternalID != before.ExternalID {
		t.Fatalf("update ExternalID = %q, want %q", got.ExternalID, before.ExternalID)
	}
	if got.Title != "Article One (retitled)" {
		t.Fatalf("update Title = %q, want the renamed title", got.Title)
	}
	if got.URL != before.URL || got.Metadata["link"] != before.Metadata["link"] {
		t.Fatalf("title-only change must not move the link: URL=%q metadata.link=%q",
			got.URL, got.Metadata["link"])
	}
	if string(got.Content) != string(before.Content) {
		t.Fatal("title-only change must keep the Markdown body byte-identical")
	}

	items3, _ := syncIncremental(t, f.feedURL(), cursor2)
	if len(items3) != 0 {
		t.Fatalf("unchanged sync after title change must yield 0 updates, got %d", len(items3))
	}
}

// TestConnector_FetchIncremental_LinkOnlyChangeYieldsUpdate guards #3823: an
// entry whose <link> moved while GUID, title and body stayed the same must
// surface exactly one update carrying the new URL and metadata.link.
func TestConnector_FetchIncremental_LinkOnlyChangeYieldsUpdate(t *testing.T) {
	f := newFakeFeed(t)

	items, cursor := syncIncremental(t, f.feedURL(), nil)
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first sync, got %d", len(items))
	}
	before := findByGUID(items, "guid-1")

	f.item1Path = "/article/a1-moved"
	wantURL := f.server.URL + "/article/a1-moved"
	items2, cursor2 := syncIncremental(t, f.feedURL(), cursor)
	if len(items2) != 1 {
		t.Fatalf("link-only change must yield exactly 1 update, got %d", len(items2))
	}
	got := items2[0]
	if got.ExternalID != before.ExternalID {
		t.Fatalf("update ExternalID = %q, want %q (GUID must stay stable)",
			got.ExternalID, before.ExternalID)
	}
	if got.URL != wantURL {
		t.Fatalf("update URL = %q, want %q", got.URL, wantURL)
	}
	if got.Metadata["link"] != wantURL {
		t.Fatalf("update metadata.link = %q, want %q", got.Metadata["link"], wantURL)
	}
	if got.Title != before.Title {
		t.Fatalf("link-only change must keep the title, got %q", got.Title)
	}
	// The moved page serves the same article, so the body must stay identical.
	if string(got.Content) != string(before.Content) {
		t.Fatal("link-only change must keep the Markdown body byte-identical")
	}

	items3, _ := syncIncremental(t, f.feedURL(), cursor2)
	if len(items3) != 0 {
		t.Fatalf("unchanged sync after link change must yield 0 updates, got %d", len(items3))
	}
}

// TestConnector_FetchIncremental_BodyChangeStillYieldsUpdate keeps the classic
// behavior: a feed-visible body edit (summary + article page both revised)
// still yields exactly one update with the new Markdown.
func TestConnector_FetchIncremental_BodyChangeStillYieldsUpdate(t *testing.T) {
	f := newFakeFeed(t)

	items, cursor := syncIncremental(t, f.feedURL(), nil)
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first sync, got %d", len(items))
	}
	before := findByGUID(items, "guid-1")

	f.item1Desc = "revised summary fallback"
	f.item1Article = strings.Replace(longArticleBody, "first paragraph", "revised paragraph", 1)
	items2, cursor2 := syncIncremental(t, f.feedURL(), cursor)
	if len(items2) != 1 {
		t.Fatalf("body change must yield exactly 1 update, got %d", len(items2))
	}
	if string(items2[0].Content) == string(before.Content) {
		t.Fatal("expected the updated article body to be ingested")
	}
	if !strings.Contains(string(items2[0].Content), "revised paragraph") {
		t.Fatalf("update body missing the revised text, got: %q", string(items2[0].Content))
	}

	items3, _ := syncIncremental(t, f.feedURL(), cursor2)
	if len(items3) != 0 {
		t.Fatalf("unchanged sync after body change must yield 0 updates, got %d", len(items3))
	}
}

// TestConnector_FetchIncremental_SummaryOnlyChangeStillSkips keeps dedup tight:
// a feed entry whose summary text changed while title, link and the ingested
// body stayed the same must NOT be re-emitted (nothing persisted moved).
func TestConnector_FetchIncremental_SummaryOnlyChangeStillSkips(t *testing.T) {
	f := newFakeFeed(t)

	_, cursor := syncIncremental(t, f.feedURL(), nil)

	f.item1Desc = "retouched summary that never reaches the document"
	items2, cursor2 := syncIncremental(t, f.feedURL(), cursor)
	if len(items2) != 0 {
		t.Fatalf("summary-only change must yield 0 updates, got %d", len(items2))
	}

	items3, _ := syncIncremental(t, f.feedURL(), cursor2)
	if len(items3) != 0 {
		t.Fatalf("unchanged sync must yield 0 updates, got %d", len(items3))
	}
}

func TestConnector_Walk_PartialFeedFailure(t *testing.T) {
	f := newFakeFeed(t)
	cfg := makeConfig(f.feedURL()+", https://invalid.invalid/feed.xml", "")
	items, err := NewConnector().FetchAll(context.Background(), cfg, nil)
	var partial *datasource.PartialFetchError
	if !errors.As(err, &partial) {
		t.Fatalf("expected PartialFetchError, got %v", err)
	}
	if len(partial.Details) != 1 {
		t.Fatalf("expected 1 feed error, got %v", partial.Details)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items from healthy feed, got %d", len(items))
	}
}

func TestConnector_Walk_PreservesCursorOnFeedFailure(t *testing.T) {
	f := newFakeFeed(t)
	cfg := makeConfig(f.feedURL(), "")
	cfg.ResourceIDs = []string{f.feedURL()}

	_, cursor, err := NewConnector().FetchIncremental(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first sync error: %v", err)
	}

	f.failFeed.Store(true)
	_, newCursor, err := NewConnector().FetchIncremental(context.Background(), cfg, cursor)
	if err == nil {
		t.Fatal("expected error when sole feed is unavailable")
	}
	if newCursor == nil || newCursor.ConnectorCursor == nil {
		t.Fatal("expected cursor to be preserved on feed failure")
	}

	var restored rssCursor
	b, err := json.Marshal(newCursor.ConnectorCursor)
	if err != nil {
		t.Fatalf("marshal cursor: %v", err)
	}
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatalf("unmarshal cursor: %v", err)
	}
	items := restored.FeedItems[f.feedURL()]
	if len(items) != 2 {
		t.Fatalf("expected preserved fingerprints for 2 items, got %d", len(items))
	}
	if items["guid-1"] == "" || items["guid-2"] == "" {
		t.Fatalf("expected non-empty preserved fingerprints, got %+v", items)
	}
}

func TestConnector_ResolveResourceAncestors_Empty(t *testing.T) {
	got, err := NewConnector().ResolveResourceAncestors(
		context.Background(), makeConfig("https://a.com/f", ""), []string{"x"},
	)
	if err != nil {
		t.Fatalf("ResolveResourceAncestors error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty ancestors, got %v", got)
	}
}

func TestResolveItem_FileNamePolicy(t *testing.T) {
	for _, tt := range []struct{ name, title, want string }{
		{"empty", "", "untitled.md"},
		{"blank", " \n\t\u2003", "untitled.md"},
		{"line breaks and punctuation", "\u2003 A\nB\rC\tD/E \u2003", "A B C D_E.md"},
		{"existing extension", "Title.md", "Title.md.md"},
		{"long title", strings.Repeat("测", 100), strings.Repeat("测", 66) + ".md"},
		{"space at truncation boundary", strings.Repeat("a", 199) + "\nmore", strings.Repeat("a", 199) + " .md"},
		{"other controls retained", "a\x01b", "a\x01b.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := NewConnector().resolveItem(
				context.Background(), nil, &gofeed.Feed{}, &gofeed.Item{Title: tt.title},
				"https://example.com/feed", "id", "<p>content</p>",
			)
			if result.item.FileName != tt.want {
				t.Fatalf("FileName = %q, want %q", result.item.FileName, tt.want)
			}
			if strings.TrimSpace(tt.title) != "" && result.item.Title != tt.title {
				t.Fatalf("source title changed: %q", result.item.Title)
			}
		})
	}
}
