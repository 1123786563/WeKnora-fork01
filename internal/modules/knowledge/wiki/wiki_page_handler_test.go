package wiki

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWikiLinkTitlesTestHandler(t *testing.T) (*WikiPageHandler, interfaces.WikiPageRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageRevision{}))

	repo := NewWikiPageRepository(db)
	svc := NewWikiPageService(repo, nil, nil, nil, nil, Seams{}, nil)
	return NewWikiPageHandler(svc, nil, nil, nil, nil, nil), repo
}

// TestWikiPageDetailResponseResolvesLinkTitles covers the #3922 assembly:
// the page-detail payload must carry a slug→title map covering its
// in_links / out_links entries so the reader footer can render the target
// page's own (Chinese) title instead of the pinyin kebab-case slug.
// Targets of every page type resolve; slugs with no matching page (or an
// empty title) stay absent from the map so the client falls back to the
// slug without erroring.
func TestWikiPageDetailResponseResolvesLinkTitles(t *testing.T) {
	handler, repo := newWikiLinkTitlesTestHandler(t)
	ctx := context.Background()
	now := time.Now()

	mk := func(id, slug, title, pageType string) {
		require.NoError(t, repo.Create(ctx, &types.WikiPage{
			ID: id, TenantID: 1, KnowledgeBaseID: "kb-links", Slug: slug, Title: title,
			PageType: pageType, Status: types.WikiPageStatusPublished,
			Version: 1, CreatedAt: now, UpdatedAt: now,
		}))
	}
	mk("p-concept", "concept/zhu-jie-gu-dong-wei-bei-zhixing-ren", "朱解古董为被知行人", types.WikiPageTypeConcept)
	mk("p-entity", "entity/mou-gong-si", "某公司", types.WikiPageTypeEntity)
	mk("p-summary", "summary/mou-wen-dang", "某文档 - Summary", types.WikiPageTypeSummary)
	mk("p-empty-title", "concept/kong-biao-ti", "", types.WikiPageTypeConcept)

	page := &types.WikiPage{
		TenantID: 1, KnowledgeBaseID: "kb-links", Slug: "entity/mou-ye-mian",
		Title: "某页面", PageType: types.WikiPageTypeEntity,
		InLinks: types.StringArray{
			"concept/zhu-jie-gu-dong-wei-bei-zhixing-ren",
			"entity/mou-gong-si",
			"summary/mou-wen-dang",
			"concept/bu-cun-zai",
			"concept/kong-biao-ti",
		},
		OutLinks: types.StringArray{"summary/mou-wen-dang"},
	}

	resp := handler.wikiPageDetailResponse(ctx, "kb-links", page)
	require.Equal(t, "朱解古董为被知行人", resp.LinkTitles["concept/zhu-jie-gu-dong-wei-bei-zhixing-ren"])
	require.Equal(t, "某公司", resp.LinkTitles["entity/mou-gong-si"])
	require.Equal(t, "某文档 - Summary", resp.LinkTitles["summary/mou-wen-dang"])
	// Out-link targets resolve through the same batched map.
	require.Contains(t, resp.LinkTitles, "summary/mou-wen-dang")
	// Missing / empty-title targets must fall back to the slug client-side:
	// absent from the map, never an error and never a blank label.
	require.NotContains(t, resp.LinkTitles, "concept/bu-cun-zai")
	require.NotContains(t, resp.LinkTitles, "concept/kong-biao-ti")
}

// TestWikiPageDetailResponseNoLinksOmitsMap pins the cheap path: a page
// without links ships no link_titles payload at all.
func TestWikiPageDetailResponseNoLinksOmitsMap(t *testing.T) {
	handler, _ := newWikiLinkTitlesTestHandler(t)
	page := &types.WikiPage{
		TenantID: 1, KnowledgeBaseID: "kb-links", Slug: "entity/wu-lian-jie",
		Title: "无链接页面", PageType: types.WikiPageTypeEntity,
	}
	resp := handler.wikiPageDetailResponse(context.Background(), "kb-links", page)
	require.Empty(t, resp.InLinks)
	require.Empty(t, resp.OutLinks)
	require.Nil(t, resp.LinkTitles)
}

// TestWikiPageDetailResponseLookupFailureDegradesToSlug guards the
// degradation contract: a failed title lookup must not fail the page read
// (no panic, no error surfaced) — the map stays empty and the client falls
// back to slugs.
func TestWikiPageDetailResponseLookupFailureDegradesToSlug(t *testing.T) {
	// No AutoMigrate of wiki_pages: every ListBySlugs query errors.
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}))
	repo := NewWikiPageRepository(db)
	svc := NewWikiPageService(repo, nil, nil, nil, nil, Seams{}, nil)
	handler := NewWikiPageHandler(svc, nil, nil, nil, nil, nil)

	page := &types.WikiPage{
		TenantID: 1, KnowledgeBaseID: "kb-broken", Slug: "entity/mou-ye-mian",
		InLinks: types.StringArray{"concept/bu-cun-zai"},
	}
	resp := handler.wikiPageDetailResponse(context.Background(), "kb-broken", page)
	require.Equal(t, page.Slug, resp.Slug)
	require.Nil(t, resp.LinkTitles)
}
