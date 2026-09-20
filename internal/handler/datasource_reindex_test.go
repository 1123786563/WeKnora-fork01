package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

func newReindexTestHandler(t *testing.T, dsSvc *stubDataSourceService) *DataSourceHandler {
	t.Helper()
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	return NewDataSourceHandler(dsSvc, kbSvc)
}

// The happy path: ownership passes, the body's external_ids and request_id
// reach the service verbatim, and the response is 202 {"sync_log_id": ...} —
// accepted-for-processing, since the run completes asynchronously.
func TestDataSource_ReindexItems_Accepted(t *testing.T) {
	var gotDSID, gotRequestID string
	var gotIDs []string
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		reindexItems: func(_ context.Context, dsID string, externalIDs []string, requestID string) (string, error) {
			gotDSID, gotIDs, gotRequestID = dsID, externalIDs, requestID
			return "sync-log-9", nil
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/reindex",
		strings.NewReader(`{"external_ids":["doc-a","doc-b"],"request_id":"req-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(newReindexTestHandler(t, dsSvc)).ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["sync_log_id"] != "sync-log-9" {
		t.Fatalf("sync_log_id = %q, want %q", resp["sync_log_id"], "sync-log-9")
	}
	if gotDSID != "ds1" || gotRequestID != "req-1" {
		t.Fatalf("service called with ds=%q requestID=%q", gotDSID, gotRequestID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != "doc-a" || gotIDs[1] != "doc-b" {
		t.Fatalf("external_ids = %v", gotIDs)
	}
}

// An empty (or missing) external_ids list is a 400 before the service is
// reached: a scoped run with nothing to do is a client mistake, not a no-op.
func TestDataSource_ReindexItems_EmptyIDsRejected(t *testing.T) {
	for _, body := range []string{`{"external_ids":[]}`, `{}`, ``} {
		dsSvc := &stubDataSourceService{
			getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
				return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
			},
			reindexItems: func(_ context.Context, _ string, _ []string, _ string) (string, error) {
				t.Fatal("service must not be called for an empty external_ids list")
				return "", nil
			},
		}

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/reindex", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = withDSCtx(req, 1)
		newDataSourceTestRouter(newReindexTestHandler(t, dsSvc)).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %q: expected 400, got %d", body, w.Code)
		}
	}
}

// A data source the caller does not own (or that does not exist) answers 404
// via the same ownership check as every other mutation endpoint.
func TestDataSource_ReindexItems_NotOwnedIs404(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, _ string) (*types.DataSource, error) {
			return nil, errors.New("not found")
		},
		reindexItems: func(_ context.Context, _ string, _ []string, _ string) (string, error) {
			t.Fatal("service must not be called when ownership fails")
			return "", nil
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/reindex",
		strings.NewReader(`{"external_ids":["doc-a"]}`))
	req.Header.Set("Content-Type", "application/json")
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(newReindexTestHandler(t, dsSvc)).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// A duplicate request_id (double click) surfaces as 409 with the idempotency
// explanation, so the client can show "already retrying" instead of an error.
func TestDataSource_ReindexItems_DuplicateRequestIs409(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		reindexItems: func(_ context.Context, _ string, _ []string, _ string) (string, error) {
			return "", service.ErrReindexDuplicateRequest
		},
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/reindex",
		strings.NewReader(`{"external_ids":["doc-a"],"request_id":"req-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(newReindexTestHandler(t, dsSvc)).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Fatal("expected an error message in the 409 body")
	}
}
