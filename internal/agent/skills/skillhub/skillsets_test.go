package skillhub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/utils"
)

// skillsetPage renders one /api/v1/skillsets page document the way SkillHub
// does: {"skillSets": [...], "total": N}.
func skillsetPage(total int, entries ...map[string]any) string {
	doc := map[string]any{"skillSets": entries, "total": total}
	payload, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(payload)
}

func skillsetEntry(slug, name string, skillSlugs ...string) map[string]any {
	return map[string]any{
		"slug":        slug,
		"displayName": name,
		"summary":     name + " summary",
		"skillSlugs":  skillSlugs,
	}
}

// newSkillsetServerClient stands up an httptest upstream reachable through
// the SSRF-safe transport, mirroring newTestServerClient in client_test.go.
func newSkillsetServerClient(t *testing.T, h http.HandlerFunc) (*HTTPClient, *httptest.Server) {
	t.Helper()
	utils.SetSSRFWhitelistFromRaw("127.0.0.1,::1,localhost")
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client, err := New(srv.URL, 5*time.Second)
	require.NoError(t, err)
	return client, srv
}

func TestSkillsetIndexPaginatesUntilShortPage(t *testing.T) {
	full := make([]map[string]any, 0, skillsetPageSize)
	for i := 0; i < skillsetPageSize; i++ {
		full = append(full, skillsetEntry(fmt.Sprintf("set-%03d", i), fmt.Sprintf("Set %d", i)))
	}
	tail := []map[string]any{skillsetEntry("set-tail", "Tail")}

	var pages []string
	client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/skillsets", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, UserAgent, r.Header.Get("User-Agent"))
		require.Equal(t, strconv.Itoa(skillsetPageSize), r.URL.Query().Get("pageSize"))
		pages = append(pages, r.URL.Query().Get("page"))
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, skillsetPage(skillsetPageSize+1, full...))
		case "2":
			fmt.Fprint(w, skillsetPage(skillsetPageSize+1, tail...))
		default:
			t.Errorf("unexpected page %q requested", r.URL.Query().Get("page"))
		}
	})

	items, err := client.SkillsetIndex(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"1", "2"}, pages, "a full page followed by a short page stops the walk")
	require.Len(t, items, skillsetPageSize+1)
	require.Equal(t, "set-000", items[0].Slug)
	require.Equal(t, "set-tail", items[len(items)-1].Slug)
}

func TestSkillsetIndexStopsWhenTotalReached(t *testing.T) {
	var requests int32
	client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		// A full page whose deduped count already reaches the declared total.
		entries := make([]map[string]any, 0, skillsetPageSize)
		for i := 0; i < skillsetPageSize; i++ {
			entries = append(entries, skillsetEntry(fmt.Sprintf("dup-%02d", i%3), "D"))
		}
		fmt.Fprint(w, skillsetPage(3, entries...))
	})

	items, err := client.SkillsetIndex(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&requests), "total reached on page 1: no second page")
	require.Len(t, items, 3, "duplicate slugs across the page are deduped in insertion order")
	require.Equal(t, []string{"dup-00", "dup-01", "dup-02"}, []string{items[0].Slug, items[1].Slug, items[2].Slug})
}

func TestSkillsetIndexRejectsInvalidPayloads(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"wrong array key", `{"results": [], "total": 0}`},
		{"skillSets not an array", `{"skillSets": {"a": 1}}`},
		{"not an object", `["nope"]`},
		{"invalid json", `{"skillSets": [`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.body)
			})
			_, err := client.SkillsetIndex(context.Background())
			require.ErrorIs(t, err, ErrMarket)
		})
	}
}

func TestSkillsetIndexMapsDTOFields(t *testing.T) {
	// Hand-rolled body so the page can mix objects with junk entries the
	// way a real listing does.
	body := `{"total": 3, "skillSets": [
		{"slug": "pdf-tools", "displayName": "PDF 工具箱", "displayNameEn": "PDF Toolkit",
		 "summary": "一套 PDF 技能", "summaryEn": "PDF skills", "scene": "tech", "skillSlugs": ["pdf-extract", " pdf-merge ", ""]},
		{"slug": "legacy", "name": "Legacy Name", "description": "uses description"},
		{"slug": "no-name-at-all", "summary": "s"},
		"not-an-object",
		{"name": "Skipped Empty"}
	]}`
	client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})

	items, err := client.SkillsetIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 3)

	pdf := items[0]
	require.Equal(t, "pdf-tools", pdf.Slug)
	require.Equal(t, "PDF 工具箱", pdf.Name)
	require.Equal(t, "一套 PDF 技能", pdf.Description)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, pdf.SkillSlugs, "blank slugs drop, the rest trim")
	require.Equal(t, "tech", pdf.Raw["scene"], "Raw carries the original typed entry")
	require.Equal(t, "PDF Toolkit", pdf.Raw["displayNameEn"])

	require.Equal(t, "Legacy Name", items[1].Name, "name falls back displayName -> name")
	require.Equal(t, "uses description", items[1].Description, "summary falls back summary -> description")
	require.Equal(t, "no-name-at-all", items[2].Name, "a name-less entry falls back to its slug")
}

func TestSkillsetDetailHappyPath(t *testing.T) {
	var gotPath string
	client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `{"slug":"pdf-tools","displayName":"PDF 工具箱","displayNameEn":"PDF Toolkit",
			"summary":"一套 PDF 技能","skillSlugs":["pdf-extract","pdf-merge"],"scene":"tech"}`)
	})

	item, err := client.SkillsetDetail(context.Background(), "pdf-tools")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/skillsets/pdf-tools", gotPath)
	require.Equal(t, "pdf-tools", item.Slug)
	require.Equal(t, "PDF 工具箱", item.Name)
	require.Equal(t, "一套 PDF 技能", item.Description)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, item.SkillSlugs)
}

func TestSkillsetDetailValidatesSlugBeforeRequest(t *testing.T) {
	for _, slug := range []string{"", "  ", "a/b", "../escape", "a b", "a:b"} {
		client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("invalid slug %q must not reach the upstream", slug)
		})
		_, err := client.SkillsetDetail(context.Background(), slug)
		require.ErrorIs(t, err, ErrInvalidSkillsetSlug, "slug %q", slug)
	}
}

func TestSkillsetDetailNotFoundPayload(t *testing.T) {
	client, _ := newSkillsetServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"detail":"missing"}`)
	})
	_, err := client.SkillsetDetail(context.Background(), "ghost")
	require.ErrorIs(t, err, ErrSkillsetNotFound)
}

func TestMarketExpertID(t *testing.T) {
	require.Equal(t, "skillhub-skillset-pdf-tools", MarketExpertID("pdf-tools"))
	slug, ok := MarketExpertSlugFromID("skillhub-skillset-pdf-tools")
	require.True(t, ok)
	require.Equal(t, "pdf-tools", slug)
	_, ok = MarketExpertSlugFromID("stock-assistant")
	require.False(t, ok)
}

// scriptedSkillsetInner extends scriptedInner (cache_test.go) with the
// skillset operations so cache semantics can be table-tested.
type scriptedSkillsetInner struct {
	scriptedInner

	indexCalls int
	indexResp  []SkillsetSummary
	indexErr   error

	detailCalls int
	detailResp  SkillsetSummary
	detailErr   error
}

func (f *scriptedSkillsetInner) SkillsetIndex(ctx context.Context) ([]SkillsetSummary, error) {
	f.indexCalls++
	return f.indexResp, f.indexErr
}

func (f *scriptedSkillsetInner) SkillsetDetail(ctx context.Context, slug string) (SkillsetSummary, error) {
	f.detailCalls++
	f.detailResp.Slug = slug
	return f.detailResp, f.detailErr
}

func TestCachedSkillsetIndexFreshStaleUnreachable(t *testing.T) {
	inner := &scriptedSkillsetInner{indexResp: []SkillsetSummary{{Slug: "a"}}}
	clock := newFakeClock()
	c := newCached(inner, time.Minute, clock.Now)

	got, err := c.SkillsetIndex(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Fresh hit inside the TTL: no second upstream call.
	_, err = c.SkillsetIndex(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, inner.indexCalls)

	// Past the TTL with the upstream down: stale value + ErrStaleOnly.
	inner.indexErr = ErrMarket
	clock.advance(2 * time.Minute)
	got, err = c.SkillsetIndex(context.Background())
	require.ErrorIs(t, err, ErrStaleOnly)
	require.Len(t, got, 1, "stale-fallback serves the last good list")
}

func TestCachedSkillsetIndexUnreachableWithoutCache(t *testing.T) {
	inner := &scriptedSkillsetInner{indexErr: ErrMarket}
	c := newCached(inner, time.Minute, newFakeClock().Now)

	got, err := c.SkillsetIndex(context.Background())
	require.Nil(t, got)
	require.ErrorIs(t, err, ErrUnreachable)
}

func TestCachedSkillsetDetailKeyedPerSlug(t *testing.T) {
	inner := &scriptedSkillsetInner{detailResp: SkillsetSummary{Name: "n"}}
	c := newCached(inner, time.Hour, newFakeClock().Now)

	_, err := c.SkillsetDetail(context.Background(), "a")
	require.NoError(t, err)
	_, err = c.SkillsetDetail(context.Background(), "a")
	require.NoError(t, err)
	_, err = c.SkillsetDetail(context.Background(), "b")
	require.NoError(t, err)
	require.Equal(t, 2, inner.detailCalls, "same slug twice is one fetch; a different slug is its own key")
}

func TestCachedSkillsetsWithoutCapableInner(t *testing.T) {
	c := newCached(&scriptedInner{}, time.Minute, newFakeClock().Now)
	_, err := c.SkillsetIndex(context.Background())
	require.ErrorIs(t, err, ErrNoSkillsetClient)
	_, err = c.SkillsetDetail(context.Background(), "a")
	require.ErrorIs(t, err, ErrNoSkillsetClient)
}
