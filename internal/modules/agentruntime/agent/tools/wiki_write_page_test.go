package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type sourceRefWikiService struct {
	interfaces.WikiPageService
	page      *types.WikiPage
	createdKB string
}

func (s *sourceRefWikiService) GetPageBySlug(context.Context, string, string) (*types.WikiPage, error) {
	return s.page, nil
}

func (s *sourceRefWikiService) RepairContentLinks(_ context.Context, _, _, content string) (string, bool, error) {
	return content, false, nil
}

func (s *sourceRefWikiService) UpdatePage(_ context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	s.page = page
	return page, nil
}

func (s *sourceRefWikiService) CreatePage(_ context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	s.page = page
	s.createdKB = page.KnowledgeBaseID
	return page, nil
}

func (s *sourceRefWikiService) InjectCrossLinks(context.Context, string, []string) {}
func (s *sourceRefWikiService) RebuildIndexPage(context.Context, string) error     { return nil }

func TestNormalizeAndValidateWikiSlug(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "already valid", in: "entity/acme-corp", want: "entity/acme-corp"},
		{name: "lowercased and spaces", in: "Entity/Acme Corp", want: "entity/acme-corp"},
		{name: "trimmed", in: "  concept/rag  ", want: "concept/rag"},
		{name: "cjk kept", in: "entity/上海中心大厦", want: "entity/上海中心大厦"},
		{name: "uuid summary", in: "summary/07a20bb1-a662-47cf-9929-06fb5d5b5b5e", want: "summary/07a20bb1-a662-47cf-9929-06fb5d5b5b5e"},
		{name: "empty", in: "   ", wantErr: true},
		{name: "leading slash", in: "/entity/x", wantErr: true},
		{name: "trailing slash", in: "entity/x/", wantErr: true},
		{name: "double slash", in: "entity//x", wantErr: true},
		{name: "invalid char", in: "entity/x!y", wantErr: true},
		{name: "invalid space-only becomes empty", in: "  ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAndValidateWikiSlug(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got slug %q", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeAndValidateWikiSlug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsSummaryNamespace(t *testing.T) {
	if !isSummaryNamespace("summary/abc") {
		t.Fatal("summary/abc must be in the summary namespace")
	}
	if isSummaryNamespace("summary") {
		t.Fatal("bare 'summary' (no slash) must not count as the summary namespace")
	}
	if isSummaryNamespace("entity/summary-of-x") {
		t.Fatal("entity/summary-of-x must not count as the summary namespace")
	}
}

func TestWikiWritePageDistinguishesOmittedAndExplicitEmptySourceRefs(t *testing.T) {
	for _, test := range []struct {
		name string
		args string
		want int
	}{
		{
			name: "omitted preserves provenance",
			args: `{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept"}`,
			want: 1,
		},
		{
			name: "empty clears provenance",
			args: `{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept","source_refs":[]}`,
			want: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &sourceRefWikiService{page: &types.WikiPage{
				KnowledgeBaseID: "kb-1",
				Slug:            "concept/a",
				SourceRefs:      types.StringArray{"doc-real|Document"},
			}}
			tool := NewWikiWritePageTool(service, []string{"kb-1"}, nil, NewWikiRouteResolver())
			result, err := tool.Execute(context.Background(), json.RawMessage(test.args))
			if err != nil || result == nil || !result.Success {
				t.Fatalf("write failed: result=%+v err=%v", result, err)
			}
			if got := len(service.page.SourceRefs); got != test.want {
				t.Fatalf("source_refs length = %d, want %d", got, test.want)
			}
		})
	}
}

func TestWikiWritePageDistinguishesOmittedAndExplicitEmptyAliases(t *testing.T) {
	for _, test := range []struct {
		name string
		args string
		want []string
	}{
		{
			name: "omitted preserves stored aliases",
			args: `{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept"}`,
			want: []string{"kept"},
		},
		{
			name: "empty clears stored aliases",
			args: `{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept","aliases":[]}`,
			want: nil,
		},
		{
			name: "explicit list replaces stored aliases",
			args: `{"slug":"concept/a","title":"A","summary":"S","content":"C","page_type":"concept","aliases":["fresh"]}`,
			want: []string{"fresh"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &sourceRefWikiService{page: &types.WikiPage{
				KnowledgeBaseID: "kb-1",
				Slug:            "concept/a",
				Aliases:         types.StringArray{"kept"},
			}}
			tool := NewWikiWritePageTool(service, []string{"kb-1"}, nil, NewWikiRouteResolver())
			result, err := tool.Execute(context.Background(), json.RawMessage(test.args))
			if err != nil || result == nil || !result.Success {
				t.Fatalf("write failed: result=%+v err=%v", result, err)
			}
			if len(service.page.Aliases) != len(test.want) {
				t.Fatalf("aliases = %v, want %v", service.page.Aliases, test.want)
			}
			for i := range test.want {
				if service.page.Aliases[i] != test.want[i] {
					t.Fatalf("aliases = %v, want %v", service.page.Aliases, test.want)
				}
			}
		})
	}
}

func TestWikiWritePageRoutesNewPageFromAuthorizedSourceRefs(t *testing.T) {
	service := &sourceRefWikiService{}
	knowledgeService := &scopeKnowledgeService{knowledge: &types.Knowledge{
		ID: "doc-2", KnowledgeBaseID: "kb-2", Title: "Document 2",
	}}
	searchTargets := types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledge, KnowledgeBaseID: "kb-2", KnowledgeIDs: []string{"doc-2"},
	}}
	tool := NewWikiWritePageTool(
		service, []string{"kb-1", "kb-2"}, knowledgeService, NewWikiRouteResolver(),
	).WithSearchTargets(searchTargets)

	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"slug":"concept/new","title":"New","summary":"Summary","content":"Content","page_type":"concept","source_refs":["doc-2"]}`,
	))
	if err != nil || result == nil || !result.Success {
		t.Fatalf("write failed: result=%+v err=%v", result, err)
	}
	if service.createdKB != "kb-2" {
		t.Fatalf("new page routed to %q, want source-owned kb-2", service.createdKB)
	}
}

// rowDropGuardProbe records the context UpdatePage was called under and can
// simulate the wiki service's row-drop guard (#3792) rejecting the write.
// The real guard lives in the wiki package, which this package cannot import
// (wiki -> agent -> tools would cycle), so the probe replays its sentinel
// message verbatim.
type rowDropGuardProbe struct {
	interfaces.WikiPageService
	stored    *types.WikiPage
	updateCtx context.Context
	updateErr error
	called    bool
}

func (p *rowDropGuardProbe) GetPageBySlug(_ context.Context, _, _ string) (*types.WikiPage, error) {
	// The real service returns a fresh copy from the DB on every read; hand
	// out a copy too, so the tool's in-place mutation of the fetched page
	// cannot reach the "stored" state on a rejected write.
	cp := *p.stored
	return &cp, nil
}

func (p *rowDropGuardProbe) RepairContentLinks(_ context.Context, _, _, content string) (string, bool, error) {
	return content, false, nil
}

func (p *rowDropGuardProbe) UpdatePage(ctx context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	p.called = true
	p.updateCtx = ctx
	if p.updateErr != nil {
		return nil, p.updateErr
	}
	p.stored = page
	return page, nil
}

func (p *rowDropGuardProbe) CreatePage(_ context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	p.stored = page
	return page, nil
}

func (p *rowDropGuardProbe) InjectCrossLinks(context.Context, string, []string) {}
func (p *rowDropGuardProbe) RebuildIndexPage(context.Context, string) error     { return nil }

// TestWikiWritePageTagsAgentEditSourceOnUpdate proves the tool hands
// UpdatePage a context attributed to WikiEditSourceAgent — the identity the
// exit-level row-drop guard keys on.
func TestWikiWritePageTagsAgentEditSourceOnUpdate(t *testing.T) {
	probe := &rowDropGuardProbe{stored: &types.WikiPage{
		KnowledgeBaseID: "kb-1", Slug: "entity/cert-ledger", Title: "台账",
		Content: "| 证书 |\n| --- |\n| A |\n| B |\n", Version: 1,
	}}
	tool := NewWikiWritePageTool(probe, []string{"kb-1"}, nil, NewWikiRouteResolver())

	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"slug":"entity/cert-ledger","title":"台账","summary":"s","content":"| 证书 |\n| --- |\n| A |\n| B |\n","page_type":"entity"}`,
	))
	if err != nil || result == nil || !result.Success {
		t.Fatalf("write failed: result=%+v err=%v", result, err)
	}
	if !probe.called {
		t.Fatal("UpdatePage was not called")
	}
	if got := types.WikiEditSourceFromContext(probe.updateCtx); got != types.WikiEditSourceAgent {
		t.Fatalf("UpdatePage edit source = %q, want %q", got, types.WikiEditSourceAgent)
	}
}

// TestWikiWritePageSurfacesRowDropRejection proves a guard rejection reaches
// the model as a failed tool result and the stored page keeps its previous
// content — the agent learns the rewrite lost rows instead of a silent
// truncation (#3792).
func TestWikiWritePageSurfacesRowDropRejection(t *testing.T) {
	full := "| 证书 |\n| --- |\n| A |\n| B |\n| C |\n| D |\n"
	probe := &rowDropGuardProbe{
		stored: &types.WikiPage{
			KnowledgeBaseID: "kb-1", Slug: "entity/cert-ledger", Title: "台账",
			Content: full, Version: 1,
		},
		// Verbatim shape of the wiki service's ErrWikiRowDropRejected wrap.
		updateErr: errors.New("wiki page update rejected: rewrite would drop existing table rows: slug=entity/cert-ledger rows=6->3 dropped=3 content_len=90->40"),
	}
	tool := NewWikiWritePageTool(probe, []string{"kb-1"}, nil, NewWikiRouteResolver())

	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"slug":"entity/cert-ledger","title":"台账","summary":"s","content":"| 证书 |\n| --- |\n| A |\n","page_type":"entity"}`,
	))
	if err != nil {
		t.Fatalf("tool returned transport error: %v", err)
	}
	if result == nil || result.Success {
		t.Fatalf("rejection must surface as failed tool result, got %+v", result)
	}
	if !strings.Contains(result.Error, "drop existing table rows") ||
		!strings.Contains(result.Error, "rows=6->3") {
		t.Fatalf("rejection detail lost: %q", result.Error)
	}
	if probe.stored.Content != full {
		t.Fatalf("stored page must keep previous content on rejection, got %q", probe.stored.Content)
	}
}
