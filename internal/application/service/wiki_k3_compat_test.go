// Pass B (23-knowledge-wikifaq) K3.1 宿主侧真实 seam 特征化测试。
//
// TestRepairContentLinks 原位于 service/wiki_page_test.go:176-228（package
// service，同包直引 resolveDeadSlug）。K3.1 迁移后该用例断言的模糊匹配
// 行为仍由宿主 K2 符号 resolveDeadSlug（slug_fuzzy.go，ib2 前未迁出）提供，
// 故按 20 计划 §6.1 R2 的生产接线形态在宿主侧以真实 seam 驱动 wiki 包
// 实现——与 W2 构造兼容层同一函数目标，断言逐字保持不变。
package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepairContentLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageRevision{}))

	ctx := context.Background()
	repo := wiki.NewWikiPageRepository(db)
	svc := wiki.NewWikiPageService(repo, nil, nil, nil, nil, wikiK3Seams(), nil)
	const kbID = "kb-repair"
	now := time.Now()

	seed := func(slug, title, pageType string) {
		require.NoError(t, repo.Create(ctx, &types.WikiPage{
			ID: uuid.New().String(), TenantID: 1, KnowledgeBaseID: kbID,
			Slug: slug, Title: title, PageType: pageType,
			Status: types.WikiPageStatusPublished, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		}))
	}

	// Real summary page (clean UUID slug) + a distractor summary + an entity.
	realSummary := "summary/07a20bb1-a662-47cf-9929-06fb5d5b5b5e"
	seed(realSummary, "Weknora 试错记录.md - Summary", types.WikiPageTypeSummary)
	seed("summary/fcadcaab-e094-4037-b71c-9edfdcdba058", "Another Doc - Summary", types.WikiPageTypeSummary)
	seed("entity/mongodb", "MongoDB", types.WikiPageTypeEntity)

	t.Run("mangled uuid summary link repaired via bigram", func(t *testing.T) {
		// One hex digit inserted into the last group — 404s on exact lookup.
		content := "See [[summary/07a20bb1-a662-47cf-9929-06fb14d5b14b14e|Weknora 试错记录.md - Summary]] for context."
		got, changed, err := svc.RepairContentLinks(ctx, kbID, "synthesis/notes", content)
		require.NoError(t, err)
		require.True(t, changed, "expected the mangled summary link to be rewritten")
		require.Contains(t, got, "[["+realSummary+"|Weknora 试错记录.md - Summary]]")
		require.NotContains(t, got, "06fb14d5b14b14e")
	})

	t.Run("live links untouched", func(t *testing.T) {
		content := "Fine: [[entity/mongodb]] and [[" + realSummary + "]]."
		got, changed, err := svc.RepairContentLinks(ctx, kbID, "synthesis/notes", content)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, content, got)
	})

	t.Run("unresolvable dead link left as-is, never stripped", func(t *testing.T) {
		// A future entity page that doesn't exist and isn't similar to anything.
		content := "Pending: [[entity/some-brand-new-topic|New Topic]]."
		got, changed, err := svc.RepairContentLinks(ctx, kbID, "synthesis/notes", content)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, content, got, "unresolvable links must be preserved verbatim, not stripped")
	})
}
