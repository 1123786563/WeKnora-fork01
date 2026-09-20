package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type stubDataSourceService struct {
	interfaces.DataSourceService
	getSyncLogs              func(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error)
	getDataSource            func(ctx context.Context, id string) (*types.DataSource, error)
	cancelSyncLog            func(ctx context.Context, tenantID uint64, dsID, logID string) error
	deleteDataSource         func(ctx context.Context, id string, purgeDocuments bool) error
	countDataSourceDocuments func(ctx context.Context, tenantID uint64, dsID string) (int64, error)
	manualSync               func(ctx context.Context, dsID string, forceFull bool) (*types.SyncLog, error)
}

func (s *stubDataSourceService) GetSyncLogs(ctx context.Context, dsID string, limit int, offset int) ([]*types.SyncLog, error) {
	if s.getSyncLogs != nil {
		return s.getSyncLogs(ctx, dsID, limit, offset)
	}
	return nil, nil
}

func (s *stubDataSourceService) GetDataSource(ctx context.Context, id string) (*types.DataSource, error) {
	if s.getDataSource != nil {
		return s.getDataSource(ctx, id)
	}
	return nil, nil
}

func (s *stubDataSourceService) CancelSyncLog(ctx context.Context, tenantID uint64, dsID, logID string) error {
	if s.cancelSyncLog != nil {
		return s.cancelSyncLog(ctx, tenantID, dsID, logID)
	}
	return nil
}

func (s *stubDataSourceService) DeleteDataSource(ctx context.Context, id string, purgeDocuments bool) error {
	if s.deleteDataSource != nil {
		return s.deleteDataSource(ctx, id, purgeDocuments)
	}
	return nil
}

func (s *stubDataSourceService) CountDataSourceDocuments(ctx context.Context, tenantID uint64, dsID string) (int64, error) {
	if s.countDataSourceDocuments != nil {
		return s.countDataSourceDocuments(ctx, tenantID, dsID)
	}
	return 0, nil
}

func (s *stubDataSourceService) ManualSync(ctx context.Context, dsID string, forceFull bool) (*types.SyncLog, error) {
	if s.manualSync != nil {
		return s.manualSync(ctx, dsID, forceFull)
	}
	return nil, nil
}

type stubKBServiceForDS struct {
	interfaces.KnowledgeBaseService
	getByID func(ctx context.Context, id string) (*types.KnowledgeBase, error)
}

func (s *stubKBServiceForDS) GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, nil
}

func newDataSourceTestRouter(h *DataSourceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.Use(func(c *gin.Context) {
		if tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64); ok {
			c.Set(types.TenantIDContextKey.String(), tenantID)
		}
		c.Next()
	})
	r.GET("/datasource/:id/logs", h.GetSyncLogs)
	r.POST("/datasource/:id/logs/:log_id/cancel", h.CancelSyncLog)
	r.GET("/datasource/:id/documents-count", h.CountDocuments)
	r.DELETE("/datasource/:id", h.DeleteDataSource)
	r.POST("/datasource/:id/sync", h.ManualSync)
	return r
}

func withDSCtx(req *http.Request, tenantID uint64) *http.Request {
	ctx := req.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	return req.WithContext(ctx)
}

func TestDataSource_GetSyncLogs_ValidLimitWithinBounds(t *testing.T) {
	var capturedLimit, capturedOffset int
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, limit int, offset int) ([]*types.SyncLog, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []*types.SyncLog{
				{ID: "log1", DataSourceID: "ds1"},
			}, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=50&offset=25", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedLimit != 50 {
		t.Fatalf("expected limit=50, got %d", capturedLimit)
	}
	if capturedOffset != 25 {
		t.Fatalf("expected offset=25, got %d", capturedOffset)
	}
}

func TestDataSource_GetSyncLogs_LimitExceedingMaximum(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called when limit exceeds maximum")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=999", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit > 100, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errMsg, ok := resp["error"].(string)
	if !ok || errMsg == "" {
		t.Fatalf("expected error message in response")
	}
	if errMsg != "limit must be between 1 and 100" {
		t.Fatalf("expected specific error message, got %q", errMsg)
	}
}

func TestDataSource_GetSyncLogs_MissingLimitDefaultsCorrectly(t *testing.T) {
	var capturedLimit, capturedOffset int
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, limit int, offset int) ([]*types.SyncLog, error) {
			capturedLimit = limit
			capturedOffset = offset
			return []*types.SyncLog{}, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedLimit != 10 {
		t.Fatalf("expected default limit=10, got %d", capturedLimit)
	}
	if capturedOffset != 0 {
		t.Fatalf("expected default offset=0 (page 1), got %d", capturedOffset)
	}
}

func TestDataSource_GetSyncLogs_NonNumericLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with non-numeric limit")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=abc", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric limit, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDataSource_GetSyncLogs_ZeroLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with limit=0")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=0", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDataSource_GetSyncLogs_NegativeLimitRejected(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		getSyncLogs: func(_ context.Context, _ string, _ int, _ int) ([]*types.SyncLog, error) {
			t.Fatalf("service must not be called with negative limit")
			return nil, nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/datasource/ds1/logs?limit=-5", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative limit, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestDataSource_CancelSyncLog_Accepted covers the happy path end to end at the
// HTTP layer: path params and tenant reach the service, and the response is
// 202 {"status":"cancel_requested"} — the cancel is only a request; the sync
// loop observes it at its next checkpoint/batch boundary.
func TestDataSource_CancelSyncLog_Accepted(t *testing.T) {
	var gotTenantID uint64
	var gotDSID, gotLogID string
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		cancelSyncLog: func(_ context.Context, tenantID uint64, dsID, logID string) error {
			gotTenantID, gotDSID, gotLogID = tenantID, dsID, logID
			return nil
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/logs/log-9/cancel", nil)
	req = withDSCtx(req, 1)
	newDataSourceTestRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "cancel_requested" {
		t.Fatalf("expected status=cancel_requested, got %q", resp["status"])
	}
	if gotTenantID != 1 || gotDSID != "ds1" || gotLogID != "log-9" {
		t.Fatalf("service called with tenant=%d ds=%q log=%q", gotTenantID, gotDSID, gotLogID)
	}
}

// TestDataSource_CancelSyncLog_NotFound verifies a service-level ownership or
// state rejection (wrong ds/tenant, missing id, non-running log) maps to 404.
func TestDataSource_CancelSyncLog_NotFound(t *testing.T) {
	dsSvc := &stubDataSourceService{
		getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
			return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
		},
		cancelSyncLog: func(_ context.Context, _ uint64, _, _ string) error {
			return service.ErrSyncLogNotFound
		},
	}
	kbSvc := &stubKBServiceForDS{
		getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
			return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
		},
	}
	h := NewDataSourceHandler(dsSvc, kbSvc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/logs/log-9/cancel", nil)
	req = withDSCtx(req, 1)
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

// ManualSync's optional body {"force_full": bool} must default to false for an
// absent/empty body while forwarding an explicit true, so an old client that
// POSTs with no body keeps the previous incremental behaviour.
func TestDataSource_ManualSync_ForceFullBodyMatrix(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"explicit true", `{"force_full":true}`, true},
		{"explicit false", `{"force_full":false}`, false},
		{"empty object", `{}`, false},
		{"empty body", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotForceFull *bool
			dsSvc := &stubDataSourceService{
				getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
					return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
				},
				manualSync: func(_ context.Context, _ string, forceFull bool) (*types.SyncLog, error) {
					gotForceFull = &forceFull
					return &types.SyncLog{ID: "log1"}, nil
				},
			}
			kbSvc := &stubKBServiceForDS{
				getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
					return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
				},
			}
			h := NewDataSourceHandler(dsSvc, kbSvc)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/sync", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = withDSCtx(req, 1)
			newDataSourceTestRouter(h).ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
			}
			if gotForceFull == nil {
				t.Fatal("service.ManualSync was never called")
			}
			if *gotForceFull != tc.want {
				t.Fatalf("force_full = %v, want %v", *gotForceFull, tc.want)
			}
		})
	}

	// No body at all (nil) is also legal.
	t.Run("nil body", func(t *testing.T) {
		var gotForceFull *bool
		dsSvc := &stubDataSourceService{
			getDataSource: func(_ context.Context, id string) (*types.DataSource, error) {
				return &types.DataSource{ID: id, KnowledgeBaseID: "kb1"}, nil
			},
			manualSync: func(_ context.Context, _ string, forceFull bool) (*types.SyncLog, error) {
				gotForceFull = &forceFull
				return &types.SyncLog{ID: "log1"}, nil
			},
		}
		kbSvc := &stubKBServiceForDS{
			getByID: func(_ context.Context, _ string) (*types.KnowledgeBase, error) {
				return &types.KnowledgeBase{ID: "kb1", TenantID: 1}, nil
			},
		}
		h := NewDataSourceHandler(dsSvc, kbSvc)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/datasource/ds1/sync", nil)
		req = withDSCtx(req, 1)
		newDataSourceTestRouter(h).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		if gotForceFull == nil || *gotForceFull {
			t.Fatalf("nil body must default force_full to false, got %v", gotForceFull)
		}
	})
}
