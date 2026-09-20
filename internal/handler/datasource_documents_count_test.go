package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// ──────────────────────────────────────────────────────────────────────
// SP2-a Task 9: GET /datasource/:id/documents-count and the
// purge_documents flag on DELETE /datasource/:id (spec §4.1).
// ──────────────────────────────────────────────────────────────────────

// newOwnedDSStub returns a data-source service stub whose GetDataSource
// always resolves to one data source owned by tenant 1 in "kb1"; tests
// override the specific call they assert on.
func newOwnedDSStub() *stubDataSourceService {
	return &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, TenantID: 1, KnowledgeBaseID: "kb1"}, nil
		},
	}
}

func newOwnedKBStub() *stubKBServiceForDS {
	return &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
}

// The happy path: tenant + ds id reach the service and the response body is
// {"count": N} — the N the delete-source confirmation dialog displays.
func TestDataSource_CountDocuments_ReturnsCount(t *testing.T) {
	var gotTenantID uint64
	var gotDSID string
	dsSvc := newOwnedDSStub()
	dsSvc.countDataSourceDocuments = func(_ context.Context, tenantID uint64, dsID string) (int64, error) {
		gotTenantID, gotDSID = tenantID, dsID
		return 3, nil
	}
	h := NewDataSourceHandler(dsSvc, newOwnedKBStub())

	w := httptest.NewRecorder()
	req := withDSCtx(httptest.NewRequest(http.MethodGet, "/datasource/ds1/documents-count", nil), 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["count"] != 3 {
		t.Fatalf("expected count=3, got %d", resp["count"])
	}
	if gotTenantID != 1 || gotDSID != "ds1" {
		t.Fatalf("service called with tenant=%d ds=%q", gotTenantID, gotDSID)
	}
}

// A missing (or cross-tenant) data source is a 404 and the count service is
// never reached — ownership is proven before anything is counted.
func TestDataSource_CountDocuments_NotFound(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, _ string) (*types.DataSource, error) {
			return nil, errors.New("data source not found")
		},
		countDataSourceDocuments: func(_ context.Context, _ uint64, _ string) (int64, error) {
			t.Fatal("count service must not be called for a data source that fails ownership lookup")
			return 0, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, newOwnedKBStub())

	w := httptest.NewRecorder()
	req := withDSCtx(httptest.NewRequest(http.MethodGet, "/datasource/ds-x/documents-count", nil), 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Fatal("expected an error message in the 404 body")
	}
}

// A cross-tenant knowledge base behind the data source is a 403 — the same
// getOwnedDataSource precedence every other ds endpoint uses.
func TestDataSource_CountDocuments_ForbiddenCrossTenantKB(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, TenantID: 42, KnowledgeBaseID: "kb1"}, nil
		},
		countDataSourceDocuments: func(_ context.Context, _ uint64, _ string) (int64, error) {
			t.Fatal("count service must not be called when the KB belongs to another tenant")
			return 0, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 42}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := withDSCtx(httptest.NewRequest(http.MethodGet, "/datasource/ds1/documents-count", nil), 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// A count failure maps to 500, not to a bogus zero count.
func TestDataSource_CountDocuments_ServiceErrorIs500(t *testing.T) {
	dsSvc := newOwnedDSStub()
	dsSvc.countDataSourceDocuments = func(_ context.Context, _ uint64, _ string) (int64, error) {
		return 0, errors.New("db down")
	}
	h := NewDataSourceHandler(dsSvc, newOwnedKBStub())

	w := httptest.NewRecorder()
	req := withDSCtx(httptest.NewRequest(http.MethodGet, "/datasource/ds1/documents-count", nil), 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

// The purge_documents matrix: only the exact string "true" switches the
// delete to the cascade path; absent, "false", and lookalike values keep the
// legacy keep-documents behavior — fully backward compatible (spec §4.1).
func TestDataSource_Delete_PurgeDocumentsParamMatrix(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantFit bool
	}{
		{"absent keeps documents", "", false},
		{"explicit false keeps documents", "purge_documents=false", false},
		{"non-canonical value is not true", "purge_documents=1", false},
		{"true purges documents", "purge_documents=true", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotFlag bool
			var gotID string
			called := 0
			dsSvc := newOwnedDSStub()
			dsSvc.deleteDataSource = func(_ context.Context, id string, purgeDocuments bool) error {
				called++
				gotID, gotFlag = id, purgeDocuments
				return nil
			}
			h := NewDataSourceHandler(dsSvc, newOwnedKBStub())

			target := "/datasource/ds1"
			if tc.query != "" {
				target += "?" + tc.query
			}
			w := httptest.NewRecorder()
			req := withDSCtx(httptest.NewRequest(http.MethodDelete, target, nil), 1)
			newDataSourceTestRouter(h).ServeHTTP(w, req)

			if w.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
			}
			if called != 1 {
				t.Fatalf("expected exactly one delete call, got %d", called)
			}
			if gotID != "ds1" {
				t.Fatalf("service called with ds=%q", gotID)
			}
			if gotFlag != tc.wantFit {
				t.Fatalf("query %q: expected purge_documents=%t forwarded, got %t", tc.query, tc.wantFit, gotFlag)
			}
		})
	}
}

// A delete against a data source that fails ownership lookup never reaches
// the service, whatever the query string says.
func TestDataSource_Delete_PurgeDocumentsNotOwnedIs404(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, _ string) (*types.DataSource, error) {
			return nil, errors.New("data source not found")
		},
		deleteDataSource: func(_ context.Context, _ string, _ bool) error {
			t.Fatal("delete service must not be called for a non-owned data source")
			return nil
		},
	}
	h := NewDataSourceHandler(dsSvc, newOwnedKBStub())

	w := httptest.NewRecorder()
	req := withDSCtx(httptest.NewRequest(http.MethodDelete, "/datasource/ds-x?purge_documents=true", nil), 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}
