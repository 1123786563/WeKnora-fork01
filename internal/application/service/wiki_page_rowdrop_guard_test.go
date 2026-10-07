package service

import (
	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// certLedgerPage builds a five-row certificate ledger page, the #3792 shape:
// a long machine-maintained table a rewrite model loves to truncate.
func certLedgerPage() *types.WikiPage {
	return &types.WikiPage{
		TenantID:        1,
		KnowledgeBaseID: "kb-rowdrop",
		Slug:            "entity/cert-ledger",
		Title:           "证书台账",
		Summary:         "证书台账",
		PageType:        types.WikiPageTypeEntity,
		Content: strings.Join([]string{
			"# 证书台账",
			"",
			"| 证书 | 颁发日期 | 金额 |",
			"| --- | --- | --- |",
			"| SSL证书A | 2024-01-01 | 1000 |",
			"| SSL证书B | 2024-02-01 | 2000 |",
			"| SSL证书C | 2024-03-01 | 3000 |",
			"| SSL证书D | 2024-04-01 | 4000 |",
			"| SSL证书E | 2024-05-01 | 5000 |",
			"",
			"备注：每年复核一次。",
		}, "\n"),
	}
}

// rewriteKeepingRows is a full-page rewrite that keeps every row identity but
// reorders them, bolds identity cells, and edits the date/amount columns —
// the legitimate rewrite shape the guard must not block (#3617 criterion).
func rewriteKeepingRows() string {
	return strings.Join([]string{
		"# 证书台账（更新）",
		"",
		"| **证书** | 颁发日期 | 金额 |",
		"| --- | --- | --- |",
		"| **SSL证书E** | 2025-05-01 | 5500 |",
		"| SSL证书A | 2025-01-01 | 1100 |",
		"| **SSL证书C** | 2025-03-01 | 3300 |",
		"| SSL证书B | 2025-02-01 | 2200 |",
		"| SSL证书D | 2025-04-01 | 4400 |",
		"",
		"备注：每半年复核一次。",
	}, "\n")
}

// rewriteDroppingRows is the model-truncation shape: header intact, only the
// first three rows re-emitted, D and E silently gone.
func rewriteDroppingRows() string {
	return strings.Join([]string{
		"# 证书台账",
		"",
		"| 证书 | 颁发日期 | 金额 |",
		"| --- | --- | --- |",
		"| SSL证书A | 2024-01-01 | 1000 |",
		"| SSL证书B | 2024-02-01 | 2000 |",
		"| SSL证书C | 2024-03-01 | 3000 |",
		"",
		"备注：每年复核一次。",
	}, "\n")
}

func newRowDropTestService(t *testing.T) (interfaces.WikiPageService, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageRevision{}))
	return NewWikiPageService(wiki.NewWikiPageRepository(db), nil, nil, nil, nil, Seams{}, nil), db, context.Background()
}

// countRevisions counts the stored snapshots of a page, to prove a rejected
// write leaves no revision row behind.
func countRevisions(t *testing.T, db *gorm.DB, page *types.WikiPage) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&types.WikiPageRevision{}).Where("page_id = ?", page.ID).Count(&n).Error)
	return n
}

func TestWikiTableRowIdentities(t *testing.T) {
	ids := wikiTableRowIdentities(certLedgerPage().Content)
	// Header counts as a row identity; the separator row does not.
	require.Equal(t, []string{"证书", "SSL证书A", "SSL证书B", "SSL证书C", "SSL证书D", "SSL证书E"}, ids)
}

func TestDroppedWikiTableRows(t *testing.T) {
	tests := []struct {
		name      string
		old, new  string
		wantDrops []string
	}{
		{"identical content passes", certLedgerPage().Content, certLedgerPage().Content, nil},
		{"reorder + bold + edited columns passes", certLedgerPage().Content, rewriteKeepingRows(), nil},
		{"dropped rows detected", certLedgerPage().Content, rewriteDroppingRows(), []string{"SSL证书D", "SSL证书E"}},
		{"wipe to empty content drops everything", certLedgerPage().Content, "", []string{"证书", "SSL证书A", "SSL证书B", "SSL证书C", "SSL证书D", "SSL证书E"}},
		{"mid-line pipe is not a table row", "plain prose\nnot a table | just a pipe in text", "shorter", nil},
		{"content without any table rows", "# Title\n\nbody only", "# Other\n\nbody", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantDrops, droppedWikiTableRows(tc.old, tc.new))
		})
	}
}

func TestDroppedWikiTableRowsSeparatorAndEmphasis(t *testing.T) {
	// A separator line is not a row identity...
	require.Empty(t, wikiTableRowIdentities("| --- | --- |\n| --- | :-: |"))
	// ...and emphasis/code wrappers normalize away so bolded rows keep identity.
	id, ok := wikiTableRowIdentity("| **证书A** | x |")
	require.True(t, ok)
	require.Equal(t, "证书A", id)
	id, ok = wikiTableRowIdentity("| `证书A` | x |")
	require.True(t, ok)
	require.Equal(t, "证书A", id)
}

func TestUpdatePageRejectsAgentRewriteDroppingTableRows(t *testing.T) {
	svc, db, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	// The agent tool tags its context with WikiEditSourceAgent before calling
	// UpdatePage (see wiki_write_page.go) — reproduce that exact call shape.
	agentCtx := types.WithWikiEditSource(ctx, types.WikiEditSourceAgent)
	page := *created
	page.Content = rewriteDroppingRows()
	_, err = svc.UpdatePage(agentCtx, &page)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrWikiRowDropRejected)

	// Rejection detail must name slug, row-count change and dropped count.
	msg := err.Error()
	require.Contains(t, msg, "slug=entity/cert-ledger")
	require.Contains(t, msg, "rows=6->4")
	require.Contains(t, msg, "dropped=2")

	// The stored page is untouched: content kept, version kept, no snapshot.
	stored, err := svc.GetPageBySlug(ctx, "kb-rowdrop", "entity/cert-ledger")
	require.NoError(t, err)
	require.Equal(t, certLedgerPage().Content, stored.Content)
	require.Equal(t, 1, stored.Version)
	require.Zero(t, countRevisions(t, db, stored))
}

func TestUpdatePageRejectsAgentRewriteLogsGuardFields(t *testing.T) {
	svc, _, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	var buf strings.Builder
	logger.SetOutput(&buf)
	t.Cleanup(func() { logger.SetOutput(os.Stdout) })

	page := *created
	page.Content = rewriteDroppingRows()
	_, err = svc.UpdatePage(types.WithWikiEditSource(ctx, types.WikiEditSourceAgent), &page)
	require.ErrorIs(t, err, ErrWikiRowDropRejected)

	log := buf.String()
	require.Contains(t, log, "entity/cert-ledger")               // slug
	require.Contains(t, log, "dropping 2 table row(s)")          // row-count change
	require.Contains(t, log, "rows 6->4")                        // row-count change
	require.Contains(t, log, fmt.Sprintf("content %d->%d bytes", // content lengths
		len(certLedgerPage().Content), len(rewriteDroppingRows())))
	require.Contains(t, log, "agent") // edit source
}

func TestUpdatePageAllowsUserRowDeletion(t *testing.T) {
	svc, _, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	userCtx := types.WithWikiEditSource(ctx, types.WikiEditSourceUser)
	page := *created
	page.Content = rewriteDroppingRows()
	updated, err := svc.UpdatePage(userCtx, &page)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)

	stored, err := svc.GetPageBySlug(ctx, "kb-rowdrop", "entity/cert-ledger")
	require.NoError(t, err)
	require.Equal(t, rewriteDroppingRows(), stored.Content)
	require.Equal(t, types.WikiEditSourceUser, stored.LastEditSource)
}

func TestUpdatePageAllowsRevertToShorterVersion(t *testing.T) {
	svc, _, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	// A human first grows the ledger to a longer version...
	userCtx := types.WithWikiEditSource(ctx, types.WikiEditSourceUser)
	page := *created
	page.Content = strings.Replace(created.Content,
		"| SSL证书E | 2024-05-01 | 5000 |",
		"| SSL证书E | 2024-05-01 | 5000 |\n| SSL证书F | 2024-06-01 | 6000 |", 1)
	_, err = svc.UpdatePage(userCtx, &page)
	require.NoError(t, err)

	// ...then reverts to the shorter version 1. The revert path writes
	// through UpdatePage tagged WikiEditSourceRevert and must not be blocked.
	reverted, err := svc.RevertPageToVersion(ctx, "kb-rowdrop", "entity/cert-ledger", 1)
	require.NoError(t, err)
	require.Equal(t, certLedgerPage().Content, reverted.Content)
	require.Equal(t, types.WikiEditSourceRevert, reverted.LastEditSource)
}

func TestUpdatePageAllowsLegitimateRewriteShapes(t *testing.T) {
	svc, _, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	page := *created
	page.Content = rewriteKeepingRows()
	updated, err := svc.UpdatePage(types.WithWikiEditSource(ctx, types.WikiEditSourceAgent), &page)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)

	// Content-identical pipeline writes (e.g. lint archiving an empty page)
	// stay untouched by the guard: status changes bump the version (status is
	// a user-visible field) but no table row can be "dropped".
	stored, err := svc.GetPageBySlug(ctx, "kb-rowdrop", "entity/cert-ledger")
	require.NoError(t, err)
	stored.Status = types.WikiPageStatusArchived
	metaUpdated, err := svc.UpdatePage(ctx, stored)
	require.NoError(t, err)
	require.Equal(t, rewriteKeepingRows(), metaUpdated.Content)
	require.Equal(t, 3, metaUpdated.Version) // status counts as a visible change
}

func TestUpdatePagePipelineShrinkMarkerExemptsRetract(t *testing.T) {
	svc, _, ctx := newRowDropTestService(t)
	created, err := svc.CreatePage(ctx, certLedgerPage())
	require.NoError(t, err)

	// Default ctx (no edit source) attributes to pipeline: unmarked drops are
	// rejected — this is exactly what the retract caller marks.
	page := *created
	page.Content = rewriteDroppingRows()
	_, err = svc.UpdatePage(ctx, &page)
	require.ErrorIs(t, err, ErrWikiRowDropRejected)

	// The retract caller marks the write as an intentional shrink
	// (see reduceSlugUpdates): the same rewrite now passes.
	page2 := *created
	page2.Content = rewriteDroppingRows()
	updated, err := svc.UpdatePage(withWikiShrinkAllowed(ctx), &page2)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
}
