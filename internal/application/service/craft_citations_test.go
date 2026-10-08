package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t06ActionChecker denies selected task actions to exercise the
// task-authority seam of citation resolution.
type t06ActionChecker struct {
	denied map[craft.TaskAction]bool
}

func (c *t06ActionChecker) CheckTaskAccess(_ context.Context, _ craft.Scope, action craft.TaskAction) error {
	if c.denied[action] {
		return craft.ErrForbidden
	}
	return nil
}

// t06StubAuthorizer stands in for the T05 knowledge service on the
// fresh-authorization seam, returning injected outcomes per citation id.
type t06StubAuthorizer struct {
	calls   []string
	outcome map[string]error
	refs    map[string]string
}

func (a *t06StubAuthorizer) AuthorizeSourceOpen(_ context.Context, _ craft.Scope, _, citationID string) (string, error) {
	a.calls = append(a.calls, citationID)
	if err, ok := a.outcome[citationID]; ok {
		return "", err
	}
	return a.refs[citationID], nil
}

// t06PublishedRun drives the real T05 journey (select → publish) so the
// citation lane consumes a genuine recorded source, then returns the record.
func t06PublishedRun(t *testing.T, checker craft.TaskAccessChecker, records *t05Records, runID string) craft.KnowledgeRecord {
	t.Helper()
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "sales excerpt")
	base := f.service(t, nil)
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: publisher,
		Now: func() time.Time { return time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC) },
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	_, err = svc.BuildForRun(craftKnowledgeCtx(scope), scope, runID, "sales", []string{"k-a"})
	require.NoError(t, err)
	rec, ok := records.byRun[runID]
	require.True(t, ok, "the T05 journey must have persisted the run's actual-source record")
	require.Equal(t, craft.KnowledgePublicationPublished, rec.PublicationState)
	require.Len(t, rec.Sources, 1)
	return rec
}

func t06StagedOutput(t *testing.T, manifest *craft.WebCitationManifest) map[string][]byte {
	t.Helper()
	staged := map[string][]byte{
		"index.html":           []byte("<!doctype html><html><body><main>page</main></body></html>"),
		"assets/craft-web.css": []byte("body{}"),
		"assets/craft-web.js":  []byte("/*local*/"),
		"build-log.json":       []byte(`{"schema":1}`),
	}
	if manifest != nil {
		raw, err := json.Marshal(manifest)
		require.NoError(t, err)
		view, err := craft.RenderWebCitationView(*manifest)
		require.NoError(t, err)
		staged[craft.WebCitationsPath] = raw
		staged["index.html"] = []byte("<!doctype html><html><body><main>page</main>" + view + "</body></html>")
	}
	return staged
}

func TestCraftT06Journey(t *testing.T) {
	checker := &t05Checker{allowed: true}
	records := &t05Records{}
	rec := t06PublishedRun(t, checker, records, "run-t06")
	realID := rec.Sources[0].ID
	fabricated := craft.KnowledgeCitationID("kb-a", "k-a", "c-never-staged")

	// The citation service is joined to the SAME record store the workbench
	// sources projection reads, and to the real knowledge service for opens.
	// (The authorizer seam is stubbed here only where the journey needs a
	// deterministic revocation; the real *CraftKnowledgeService implements it.)
	knowledge := &t06StubAuthorizer{refs: map[string]string{realID: rec.Sources[0].Ref}}
	citationSvc, err := NewCraftCitationService(CraftCitationConfig{
		Records:    records,
		Authorizer: knowledge,
		TaskAccess: checker,
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	ctx := craftKnowledgeCtx(scope)

	// 1) Success: facts bind to recorded sources; inference carries an
	//    explicit distinct marker and never a citation id.
	manifest := craft.WebCitationManifest{Schema: craft.WebCitationSchema, Lang: "zh-CN", Entries: []craft.WebCitationEntry{
		{Kind: craft.WebCitationFact, CitationID: realID, Claim: "华东区 2025 Q3 销售额来自该来源"},
		{Kind: craft.WebCitationInference, Claim: "预计 Q4 延续增长"},
	}}
	admitted, err := citationSvc.AdmitStagedWebCitations(ctx, scope, "run-t06", t06StagedOutput(t, &manifest))
	require.NoError(t, err)
	require.Len(t, admitted.Entries, 2)
	require.Equal(t, craft.WebCitationFact, admitted.Entries[0].Kind)
	require.Equal(t, realID, admitted.Entries[0].CitationID)
	require.Equal(t, craft.WebCitationInference, admitted.Entries[1].Kind)
	require.Empty(t, admitted.Entries[1].CitationID, "inference must never present a source citation")

	// 2) Web citations and Workbench evidence agree: every admitted fact id
	//    is one of the recorded sources the sources projection serves.
	workbench, err := citationSvc.WorkbenchAgreement(ctx, scope, "run-t06", admitted)
	require.NoError(t, err)
	require.Equal(t, []string{realID}, workbench.FactCitationIDs)
	require.Len(t, workbench.RecordedCitationIDs, 1)
	require.Equal(t, realID, workbench.RecordedCitationIDs[0])
	require.Empty(t, workbench.UnboundCitationIDs)

	// 3) Opening a cited source resolves through fresh authorization and
	//    returns the durable ref.
	open, err := citationSvc.ResolveCitation(ctx, scope, "run-t06", realID)
	require.NoError(t, err)
	require.Equal(t, rec.Sources[0].Ref, open.Ref)
	require.Nil(t, open.Placeholder)

	// 4) Revoked source keeps a non-leaking placeholder instead of erroring
	//    with detail (recovery path).
	knowledge.outcome = map[string]error{realID: craft.ErrForbidden}
	open, err = citationSvc.ResolveCitation(ctx, scope, "run-t06", realID)
	require.NoError(t, err)
	require.Empty(t, open.Ref)
	require.NotNil(t, open.Placeholder)
	require.Equal(t, realID, open.Placeholder.CitationID)
	require.Equal(t, craft.WebCitationStatusUnavailable, open.Placeholder.Status)
	raw, err := json.Marshal(open)
	require.NoError(t, err)
	require.NotContains(t, string(raw), rec.Sources[0].Ref, "the placeholder must not leak the durable ref")
	require.NotContains(t, string(raw), rec.Sources[0].Digest, "the placeholder must not leak the excerpt digest")

	// 5) Fabricated citation ids cannot silently resolve.
	_, err = citationSvc.ResolveCitation(ctx, scope, "run-t06", fabricated)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// 6) Admission rejects a manifest whose fact cites an unrecorded id.
	fabricatedManifest := craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: []craft.WebCitationEntry{
		{Kind: craft.WebCitationFact, CitationID: fabricated, Claim: "made up"},
	}}
	_, err = citationSvc.AdmitStagedWebCitations(ctx, scope, "run-t06", t06StagedOutput(t, &fabricatedManifest))
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Contains(t, err.Error(), "not among the Run")

	// 7) A page that displays a citation marker without a manifest is refused.
	ghost := map[string][]byte{"index.html": []byte("<p " + craft.WebCitationFactMarkerAttr + "=\"" + realID + "\">ghost</p>")}
	_, err = citationSvc.AdmitStagedWebCitations(ctx, scope, "run-t06", ghost)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Contains(t, err.Error(), "not declared")

	// 8) A citation-free page without a manifest stays admissible.
	plain, err := citationSvc.AdmitStagedWebCitations(ctx, scope, "run-t06", t06StagedOutput(t, nil))
	require.NoError(t, err)
	require.Empty(t, plain.Entries)
}

func TestCraftT06AdmissionFailsClosed(t *testing.T) {
	checker := &t05Checker{allowed: true}
	records := &t05Records{}
	rec := t06PublishedRun(t, checker, records, "run-t06-fail")
	realID := rec.Sources[0].ID
	manifest := craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: []craft.WebCitationEntry{
		{Kind: craft.WebCitationFact, CitationID: realID, Claim: "claim"},
	}}
	staged := t06StagedOutput(t, &manifest)

	svc, err := NewCraftCitationService(CraftCitationConfig{Records: records, Authorizer: &t06StubAuthorizer{}, TaskAccess: checker})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	ctx := craftKnowledgeCtx(scope)

	// Cross-tenant/cross-task scope cannot read another task's record.
	foreign := scope
	foreign.SessionID = "s-other"
	_, err = svc.AdmitStagedWebCitations(ctx, foreign, "run-t06-fail", staged)
	require.ErrorIs(t, err, craft.ErrForbidden)
	// A run id with no record of its own is not this task's evidence.
	_, err = svc.AdmitStagedWebCitations(ctx, scope, "run-other", staged)
	require.ErrorIs(t, err, craft.ErrNotFound)
	// An unpublished record is not delivered evidence.
	unpublished := rec
	unpublished.RunID = "run-unpub"
	unpublished.PublicationState = craft.KnowledgePublicationPrepared
	unpublishedRecords := &t05Records{byRun: map[string]craft.KnowledgeRecord{"run-unpub": unpublished}}
	unpubSvc, err := NewCraftCitationService(CraftCitationConfig{Records: unpublishedRecords, Authorizer: &t06StubAuthorizer{}, TaskAccess: checker})
	require.NoError(t, err)
	_, err = unpubSvc.AdmitStagedWebCitations(ctx, scope, "run-unpub", staged)
	require.ErrorIs(t, err, craft.ErrConflict)
	// The staged output must carry the web entry so the view is verifiable.
	noEntry := map[string][]byte{craft.WebCitationsPath: staged[craft.WebCitationsPath]}
	_, err = svc.AdmitStagedWebCitations(ctx, scope, "run-t06-fail", noEntry)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	// Without the record port the gate fails closed.
	bare, err := NewCraftCitationService(CraftCitationConfig{Authorizer: &t06StubAuthorizer{}, TaskAccess: checker})
	require.Error(t, err)
	require.Nil(t, bare)
}

func TestCraftT06ResolveCitationAuthorityMatrix(t *testing.T) {
	checker := &t05Checker{allowed: true}
	records := &t05Records{}
	rec := t06PublishedRun(t, checker, records, "run-t06-open")
	realID := rec.Sources[0].ID
	fabricated := craft.KnowledgeCitationID("kb-a", "k-a", "c-ghost")
	authorizer := &t06StubAuthorizer{refs: map[string]string{realID: rec.Sources[0].Ref}}
	svc, err := NewCraftCitationService(CraftCitationConfig{Records: records, Authorizer: authorizer, TaskAccess: checker})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	ctx := craftKnowledgeCtx(scope)

	// Task-level denial is an authorization failure, not a placeholder.
	openChecker := &t06ActionChecker{denied: map[craft.TaskAction]bool{craft.TaskOpenSource: true}}
	deniedSvc, err := NewCraftCitationService(CraftCitationConfig{Records: records, Authorizer: authorizer, TaskAccess: openChecker})
	require.NoError(t, err)
	_, err = deniedSvc.ResolveCitation(ctx, scope, "run-t06-open", realID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, authorizer.calls, "the fresh ACL seam must not even be consulted without task authority")

	// Source-level outcomes: revoked → placeholder; missing → placeholder;
	// unpublished package → conflict; unknown internal error propagates.
	authorizer.outcome = map[string]error{
		realID:     craft.ErrForbidden,
		fabricated: nil,
	}
	open, err := svc.ResolveCitation(ctx, scope, "run-t06-open", realID)
	require.NoError(t, err)
	require.NotNil(t, open.Placeholder)
	require.Equal(t, craft.WebCitationStatusUnavailable, open.Placeholder.Status)

	// A missing (never recorded) id is NotFound, never a placeholder.
	_, err = svc.ResolveCitation(ctx, scope, "run-t06-open", fabricated)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// An unpublished record conflicts rather than half-opening.
	unpublished := rec
	unpublished.RunID = "run-unpub2"
	unpublished.PublicationState = craft.KnowledgePublicationPrepared
	unpubSvc, err := NewCraftCitationService(CraftCitationConfig{
		Records:    &t05Records{byRun: map[string]craft.KnowledgeRecord{"run-unpub2": unpublished}},
		Authorizer: authorizer, TaskAccess: checker,
	})
	require.NoError(t, err)
	_, err = unpubSvc.ResolveCitation(ctx, scope, "run-unpub2", realID)
	require.ErrorIs(t, err, craft.ErrConflict)

	// Unexpected authorizer errors propagate unchanged.
	authorizer.outcome = map[string]error{realID: errors.New("storage failed")}
	_, err = svc.ResolveCitation(ctx, scope, "run-t06-open", realID)
	require.ErrorContains(t, err, "storage failed")
}
