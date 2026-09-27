package container

// T20 (#139) post-terminal promotion orchestration. The durable capture
// receipts that reached a terminal state (sealed/advanced) name a Workspace
// draft revision; when that head is the SELECTED product of the receipt's
// own Run, the four-check promotion gate runs through the SAME artifact
// service the capture coordinator assembled. Promotion stays fail-closed on
// every incomplete fact: a missing page probe leaves both page facts not_run
// and the gate refuses (the prior default version keeps the seat); unsealed
// or foreign heads never trigger an attempt. The page-load probe itself is
// T14's live browser boundary — register it through
// RegisterCraftWebPageLoadProbe when that lane's implementation integrates;
// until then the trigger fires, refuses honestly and logs.

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
)

// craftPromotionScanLimit bounds one promotion pass.
const craftPromotionScanLimit = 32

var registeredCraftWebPageLoadProbe service.WebPageLoadProbe

// RegisterCraftWebPageLoadProbe installs the T14 externally observed page
// probe both artifact assemblies (runtime + capture coordinator) consume.
// nil (the default) keeps both page facts not_run and every promotion
// fails closed.
func RegisterCraftWebPageLoadProbe(probe service.WebPageLoadProbe) {
	registeredCraftWebPageLoadProbe = probe
}

// RegisteredCraftWebPageLoadProbe returns the registered probe (nil until
// the T14 implementation lands).
func RegisteredCraftWebPageLoadProbe() service.WebPageLoadProbe {
	return registeredCraftWebPageLoadProbe
}

// craftPostTerminalPromoter is the promotion half of the post-terminal
// orchestration: after the capture coordinator sealed a Run's output into
// the Workspace draft head, this pass attempts the four-check promotion of
// exactly that sealed head.
type craftPostTerminalPromoter struct {
	db       *gorm.DB
	promote  *service.CraftArtifactService
	drafts   craft.DraftHeadStore
	versions craft.VersionStore
}

// newCraftPostTerminalPromoter validates the wiring fail-closed: every store
// is mandatory because the pass runs unattended after terminal Runs.
func newCraftPostTerminalPromoter(
	db *gorm.DB,
	promote *service.CraftArtifactService,
	drafts craft.DraftHeadStore,
	versions craft.VersionStore,
) *craftPostTerminalPromoter {
	if db == nil || promote == nil || drafts == nil || versions == nil {
		logger.Warnf(context.Background(),
			"[CraftPromotion] post-terminal promotion NOT assembled: the trigger stays inert (captures seal, nothing publishes)")
		return nil
	}
	return &craftPostTerminalPromoter{db: db, promote: promote, drafts: drafts, versions: versions}
}

// craftPromotionReceiptRow is the terminal-receipt scan shape (parameter-
// bound query; scope reconstructs exactly the way the repository maps rows).
type craftPromotionReceiptRow struct {
	TenantID      uint64
	WorkspaceID   string
	RunID         string
	OwnerID       string
	SessionID     string
	State         string
	DraftRevision *int64
}

// promoteTerminalReceipts scans the terminal capture receipts and attempts
// the four-check promotion for every SEALED head that is the selected
// product of the receipt's own Run. Idempotent: a replayed identical
// promotion adopts the already published version, so repeated passes (per
// Run completion and every periodic scan) are safe and cheap — the version
// existence check precedes any probe observation.
func (p *craftPostTerminalPromoter) promoteTerminalReceipts(ctx context.Context, limit int) {
	if p == nil {
		return
	}
	if limit <= 0 {
		limit = craftPromotionScanLimit
	}
	var rows []craftPromotionReceiptRow
	if err := p.db.WithContext(ctx).
		Table("craft_run_captures").
		Select("tenant_id, workspace_id, run_id, owner_id, session_id, state, draft_revision").
		Where("state IN ? AND draft_revision IS NOT NULL", []string{"sealed", "advanced"}).
		Order("updated_at ASC").Limit(limit).Scan(&rows).Error; err != nil {
		logger.Warnf(ctx, "[CraftPromotion] terminal receipt scan failed: %v", err)
		return
	}
	for _, row := range rows {
		if row.DraftRevision == nil {
			continue
		}
		scope := craft.Scope{TenantID: row.TenantID, UserID: row.OwnerID, SessionID: row.SessionID}
		head, err := p.drafts.ReadRevision(ctx, scope, row.WorkspaceID, *row.DraftRevision)
		if err != nil {
			// An unreadable revision (concurrently advanced workspace) is not a
			// promotion candidate; the next pass re-evaluates the new head.
			logger.Debugf(ctx, "[CraftPromotion] head %s@%d unreadable for run %s: %v", row.WorkspaceID, *row.DraftRevision, row.RunID, err)
			continue
		}
		if head.State != craft.DraftHeadSelected {
			// A failed, stopped or superseded Run never promotes: the head is
			// not its sealed product.
			continue
		}
		if head.SourceRunID != row.RunID {
			logger.Warnf(ctx, "[CraftPromotion] head %s@%d sealed from run %s but receipt names run %s; refusing",
				row.WorkspaceID, head.Revision, head.SourceRunID, row.RunID)
			continue
		}
		versionID := craft.VersionID(head.WorkspaceID, head.SourceRunID, head.ManifestDigest)
		if _, err := p.versions.Get(ctx, scope, versionID); err == nil {
			// Already published: an identical replay adopts the stored version.
			continue
		} else if !errors.Is(err, craft.ErrNotFound) {
			logger.Warnf(ctx, "[CraftPromotion] version lookup %s failed: %v", versionID, err)
			continue
		}
		version, err := p.promote.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
			WorkspaceID: head.WorkspaceID, RunID: head.SourceRunID,
			CandidateID: craft.CandidateID(head.WorkspaceID, head.SourceRunID, head.ManifestDigest),
			Revision:    head.Revision,
		})
		if err != nil {
			// Fail-closed by contract (e.g. the page probe is not assembled):
			// the candidate stays a private Workspace draft and the prior
			// default version keeps the seat. The refusal is operator-visible.
			logger.Warnf(ctx, "[CraftPromotion] run %s stayed a private draft (promotion refused): %v", head.SourceRunID, err)
			continue
		}
		logger.Infof(ctx, "[CraftPromotion] run %s promoted to version %s at workspace revision %d",
			head.SourceRunID, version.ID, head.Revision)
	}
}
