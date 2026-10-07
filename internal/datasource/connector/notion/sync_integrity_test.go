package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// captureNotionSyncLogs redirects the global logger for the duration of one
// test so warnings emitted by the connector can be asserted on. It is the
// fork-side twin of captureNotionLogs (query_windowing_test.go), which the
// upstream #3845 tests own.
func captureNotionSyncLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)
	t.Cleanup(func() { logger.SetOutput(os.Stdout) })
	return buf
}

// paginatedBlocksServer serves /v1/blocks/{pageID}/children as `pages` cursor
// pages of `perPage` flat paragraph blocks each.
func paginatedBlocksServer(t *testing.T, pageID string, pages, perPage int) *httptest.Server {
	t.Helper()
	allowNotionTestServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/blocks/"+pageID+"/children", func(w http.ResponseWriter, r *http.Request) {
		index := 0
		if cursor := r.URL.Query().Get("start_cursor"); cursor != "" {
			var err error
			if index, err = strconv.Atoi(cursor); err != nil {
				http.Error(w, "bad cursor", http.StatusBadRequest)
				return
			}
		}
		results := make([]map[string]interface{}, perPage)
		for i := range results {
			results[i] = map[string]interface{}{
				"id":           fmt.Sprintf("blk-%d-%d", index, i),
				"type":         "paragraph",
				"has_children": false,
				"paragraph":    map[string]interface{}{"rich_text": []interface{}{}},
			}
		}
		resp := map[string]interface{}{"object": "list", "results": results}
		if index+1 < pages {
			resp["has_more"] = true
			resp["next_cursor"] = strconv.Itoa(index + 1)
		} else {
			resp["has_more"] = false
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// TestBlocksTruncated covers the truncation predicate directly: only a cap hit
// with a real next page means content is dropped.
func TestBlocksTruncated(t *testing.T) {
	tests := []struct {
		name  string
		total int
		resp  paginatedResponse
		want  bool
	}{
		{"below cap with next page", maxBlocksPerPage - 1, paginatedResponse{HasMore: true, NextCursor: "next"}, false},
		{"exact cap no more", maxBlocksPerPage, paginatedResponse{HasMore: false}, false},
		{"exact cap with empty cursor", maxBlocksPerPage, paginatedResponse{HasMore: true}, false},
		{"exact cap with next page", maxBlocksPerPage, paginatedResponse{HasMore: true, NextCursor: "next"}, true},
		{"over cap with next page", maxBlocksPerPage + 200, paginatedResponse{HasMore: true, NextCursor: "next"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blocksTruncated(tt.total, tt.resp); got != tt.want {
				t.Errorf("blocksTruncated(%d, %+v) = %v, want %v", tt.total, tt.resp, got, tt.want)
			}
		})
	}
}

// TestGetBlockChildrenAllWarnsOnTruncation (#3778): when more than
// maxBlocksPerPage blocks exist, the client keeps the status-quo behavior
// (stop at the cap, no error) but no longer truncates silently — a warning
// naming the page is emitted.
func TestGetBlockChildrenAllWarnsOnTruncation(t *testing.T) {
	ts := paginatedBlocksServer(t, "big-page", 11, 100) // 1100 blocks available
	logs := captureNotionSyncLogs(t)

	client := mustTestClient(t, "test-token", ts.URL)
	blocks, err := client.GetBlockChildrenAll(context.Background(), "big-page")
	if err != nil {
		t.Fatalf("GetBlockChildrenAll() error: %v", err)
	}
	if len(blocks) != maxBlocksPerPage {
		t.Fatalf("expected the status-quo cap of %d blocks, got %d", maxBlocksPerPage, len(blocks))
	}
	if msg := logs.String(); !strings.Contains(msg, "exceeded 1000 blocks") || !strings.Contains(msg, "big-page") {
		t.Errorf("expected a truncation warning naming the page, got: %s", msg)
	}
}

// TestGetBlockChildrenAllExactCapDoesNotWarn: exactly maxBlocksPerPage blocks
// with no next page is a complete fetch, not a truncation.
func TestGetBlockChildrenAllExactCapDoesNotWarn(t *testing.T) {
	ts := paginatedBlocksServer(t, "big-page", 10, 100) // exactly 1000 blocks
	logs := captureNotionSyncLogs(t)

	client := mustTestClient(t, "test-token", ts.URL)
	blocks, err := client.GetBlockChildrenAll(context.Background(), "big-page")
	if err != nil {
		t.Fatalf("GetBlockChildrenAll() error: %v", err)
	}
	if len(blocks) != maxBlocksPerPage {
		t.Fatalf("expected all %d blocks, got %d", maxBlocksPerPage, len(blocks))
	}
	if msg := logs.String(); strings.Contains(msg, "truncating") {
		t.Errorf("no truncation warning expected for a complete fetch, got: %s", msg)
	}
}

// twoPageIncrementalServer emulates a workspace with two changed pages whose
// block reads can be toggled to fail, to exercise incremental round semantics.
func twoPageIncrementalServer(t *testing.T, failPageB *atomic.Bool) *types.DataSourceConfig {
	t.Helper()
	allowNotionTestServer(t)
	page := func(id, title string) string {
		return `{"id":"` + id + `","object":"page","url":"https://notion.so/` + id + `",` +
			`"last_edited_time":"2026-02-01T10:00:00Z","parent":{"type":"workspace","workspace":true},` +
			`"properties":{"title":{"type":"title","title":[{"plain_text":"` + title + `"}]}}}`
	}
	blocks := func(text string) string {
		return `{"object":"list","results":[{"id":"b-1","type":"paragraph","has_children":false,` +
			`"paragraph":{"rich_text":[{"type":"text","plain_text":"` + text + `","text":{"content":"` + text + `"}}]}}],` +
			`"has_more":false}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","results":[` + page("page-a", "Page A") + `,` + page("page-b", "Page B") + `],"has_more":false}`))
	})
	mux.HandleFunc("/v1/blocks/page-a/children", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(blocks("Alpha content")))
	})
	mux.HandleFunc("/v1/blocks/page-b/children", func(w http.ResponseWriter, r *http.Request) {
		if failPageB.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(blocks("Beta content")))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return makeNotionConfig(&Config{APIKey: "test-token"}, ts.URL, []string{"page-a", "page-b"})
}

// TestFetchIncrementalBlocksFailureKeepsCursor (#3692): when one of two changed
// pages fails its block reads, the whole incremental round must fail without
// emitting items or advancing the cursor; once the failure clears, a retry with
// the same old cursor must fetch both pages.
func TestFetchIncrementalBlocksFailureKeepsCursor(t *testing.T) {
	failPageB := &atomic.Bool{}
	config := twoPageIncrementalServer(t, failPageB)
	prev := buildCursor(map[string]time.Time{
		"page-a": time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		"page-b": time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	})

	// Failed round: page-b's blocks are unreadable.
	failPageB.Store(true)
	connector := NewConnector()
	items, next, err := connector.FetchIncremental(context.Background(), config, prev)
	if err == nil {
		t.Fatal("expected the incremental round to fail when a page's blocks cannot be read")
	}
	if !strings.Contains(err.Error(), "page-b") {
		t.Errorf("error should name the failed page, got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("failed round must not emit items, got %d", len(items))
	}
	if next != nil {
		t.Error("failed round must not return a cursor; the old cursor must be kept so the unread page is retried")
	}

	// Recovered round: the same old cursor must pick up both changed pages.
	failPageB.Store(false)
	items, next, err = connector.FetchIncremental(context.Background(), config, prev)
	if err != nil {
		t.Fatalf("FetchIncremental() after recovery: %v", err)
	}
	got := make(map[string]string, len(items))
	for _, item := range items {
		got[item.ExternalID] = string(item.Content)
	}
	if len(got) != 2 {
		t.Fatalf("expected both changed pages after recovery, got %d items: %v", len(items), got)
	}
	if !strings.Contains(got["page-a"], "Alpha content") {
		t.Errorf("page-a content missing after recovery: %q", got["page-a"])
	}
	if !strings.Contains(got["page-b"], "Beta content") {
		t.Errorf("page-b content missing after recovery: %q", got["page-b"])
	}
	if next == nil {
		t.Fatal("successful round must return a cursor")
	}

	// Steady state: replaying the returned cursor yields no changes and no error.
	items, _, err = connector.FetchIncremental(context.Background(), config, next)
	if err != nil {
		t.Fatalf("steady-state FetchIncremental(): %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected no changes in steady state, got %d items", len(items))
	}
}
