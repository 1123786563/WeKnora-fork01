package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// T10 (#127): the authenticated source-open resolver for the CURRENT viewer.
//
// Contract (Spec stories 25/33, parent #107):
//
//   - every open re-runs BOTH doors: the current Task grant (T08 role via
//     RequireTaskAccess, so a revoked Task member is denied before any
//     resource lookup) and the caller's own current knowledge permission
//     through the shared-aware document ACL port (the same entrance the Run's
//     build went through);
//   - Task/artifact access NEVER implies original-source access — the ref is
//     returned only when the viewer's own permission covers the exact
//     recorded document, library and owning tenant;
//   - the result is exactly the durable recorded ref (craftkb://…). No
//     provider/storage URL is produced, exposed or cached, and nothing about
//     a previous open decision is remembered: a grant revoked between two
//     clicks fails the second click;
//   - missing citations, missing Runs and tampered (non-craftkb) recorded
//     refs all deny stably and non-leaking (ErrNotFound); a denial carries
//     no reason. The historical citation row itself stays visible with its
//     digest through Sources — a denial never erases the citation.
//
// A caller must resolve the returned ref through the existing source
// services; this resolver never follows a model-provided or tampered URL.

// AuthorizeSourceOpen returns only a recorded stable ref after fresh Task and
// underlying knowledge checks. A caller must resolve the ref through the
// existing source service; this method never follows a model-provided URL.
func (s *CraftKnowledgeService) AuthorizeSourceOpen(ctx context.Context, scope craft.Scope, runID, citationID string) (string, error) {
	if s == nil || s.records == nil || s.access == nil {
		return "", craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskOpenSource); err != nil {
		return "", err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return "", craft.ErrForbidden
	}
	record, err := s.records.Load(ctx, scope, runID)
	if err != nil {
		return "", err
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
		return "", craft.ErrForbidden
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return "", craft.ErrConflict
	}
	for _, source := range record.Sources {
		if source.ID != citationID {
			continue
		}
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		kbID := craftKnowledgeBaseOfRef(source.Ref)
		if knowledgeID == "" || kbID == "" {
			return "", craft.ErrNotFound
		}
		rows, err := s.access(ctx, scope.TenantID, []string{knowledgeID})
		if err != nil {
			return "", err
		}
		for _, row := range rows {
			if row != nil && row.ID == knowledgeID && row.KnowledgeBaseID == kbID && row.TenantID == source.TenantID {
				return source.Ref, nil
			}
		}
		return "", craft.ErrForbidden
	}
	return "", craft.ErrNotFound
}
