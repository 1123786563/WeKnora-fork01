package notion

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestPaginatedResponseRequestStatusParsing (#3836): the response wrapper must
// parse the request_status envelope, and truncated() must fire only on
// type == "incomplete" — absent or complete statuses behave exactly as before.
func TestPaginatedResponseRequestStatusParsing(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		truncated  bool
		reason     string
		hasStatus  bool
		statusType string
	}{
		{
			name:      "absent request_status is not truncated",
			payload:   `{"object":"list","results":[],"has_more":false}`,
			truncated: false,
		},
		{
			name:       "complete is not truncated",
			payload:    `{"object":"list","results":[],"has_more":false,"request_status":{"type":"complete"}}`,
			truncated:  false,
			hasStatus:  true,
			statusType: "complete",
		},
		{
			name: "incomplete with reason is truncated",
			payload: `{"object":"list","results":[],"has_more":false,` +
				`"request_status":{"type":"incomplete","incomplete_reason":"query_result_limit_reached"}}`,
			truncated:  true,
			hasStatus:  true,
			statusType: "incomplete",
			reason:     "query_result_limit_reached",
		},
		{
			name: "incomplete without reason is truncated",
			payload: `{"object":"list","results":[],"has_more":true,"next_cursor":"p2",` +
				`"request_status":{"type":"incomplete"}}`,
			truncated:  true,
			hasStatus:  true,
			statusType: "incomplete",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp paginatedResponse
			if err := json.Unmarshal([]byte(tt.payload), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := resp.truncated(); got != tt.truncated {
				t.Errorf("truncated() = %v, want %v", got, tt.truncated)
			}
			if !tt.hasStatus {
				if resp.RequestStatus != nil {
					t.Errorf("RequestStatus = %+v, want nil", resp.RequestStatus)
				}
				return
			}
			if resp.RequestStatus == nil {
				t.Fatal("RequestStatus = nil, want parsed")
			}
			if resp.RequestStatus.Type != tt.statusType {
				t.Errorf("Type = %q, want %q", resp.RequestStatus.Type, tt.statusType)
			}
			if resp.RequestStatus.IncompleteReason != tt.reason {
				t.Errorf("IncompleteReason = %q, want %q", resp.RequestStatus.IncompleteReason, tt.reason)
			}
		})
	}
}

// truncationQueryRecord builds a database record page object for query results.
func truncationQueryRecord(id, edited string) map[string]interface{} {
	return map[string]interface{}{
		"id":               id,
		"object":           "page",
		"url":              "https://notion.so/" + id,
		"last_edited_time": edited,
		"in_trash":         false,
		"parent":           map[string]interface{}{"type": "data_source_id", "data_source_id": "ds-1"},
		"properties": map[string]interface{}{
			"Name": map[string]interface{}{
				"type":  "title",
				"title": []interface{}{map[string]interface{}{"plain_text": id}},
			},
		},
	}
}

// TestQueryDatabaseAllTruncatedErrors (#3836): when the vendor hits its query
// result limit, has_more=false and the response still carries rows — the exact
// shape of a complete read. QueryDatabaseAll must return a recognizable error
// instead of silently returning the partial row set, and must not fall back to
// database-container resolution as if the ID kind were wrong.
func TestQueryDatabaseAllTruncatedErrors(t *testing.T) {
	allowNotionTestServer(t)
	var requests int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/data_sources/ds-1/query", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"results": []interface{}{
				truncationQueryRecord("r1", "2026-06-01T00:00:00Z"),
				truncationQueryRecord("r2", "2026-06-01T00:00:00Z"),
			},
			"has_more": false, // same shape as a complete read…
			"request_status": map[string]interface{}{ // …but the vendor dropped rows
				"type":              "incomplete",
				"incomplete_reason": "query_result_limit_reached",
			},
		})
	})
	// Resolution endpoints must never be reached: the query hit a real data
	// source, so a truncation error must not be misread as a wrong-ID-kind 404.
	mux.HandleFunc("/v1/databases/ds-1", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		http.NotFound(w, r)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := mustTestClient(t, "test-token", ts.URL)
	records, err := client.QueryDatabaseAll(context.Background(), "ds-1")
	if err == nil {
		t.Fatalf("expected a truncation error, got silent success with %d records", len(records))
	}
	if !errors.Is(err, ErrQueryTruncated) {
		t.Errorf("error must match ErrQueryTruncated, got: %v", err)
	}
	if !strings.Contains(err.Error(), "query_result_limit_reached") {
		t.Errorf("error should name the incomplete_reason, got: %v", err)
	}
	if !strings.Contains(err.Error(), "narrow the data source") {
		t.Errorf("error should guide the user to narrow the data source or add filters, got: %v", err)
	}
	if records != nil {
		t.Errorf("no partial record set may be returned on truncation, got %d records", len(records))
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("expected exactly 1 request (query only, no ID-kind fallback), got %d", got)
	}
}

// TestQueryTruncationSignaledBeforeLastPage (#3836): Notion may attach
// request_status to any page, not just the last one — the signal must abort
// pagination immediately without following the next cursor.
func TestQueryTruncationSignaledBeforeLastPage(t *testing.T) {
	allowNotionTestServer(t)
	var queries int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/data_sources/ds-1/query", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&queries, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object":   "list",
			"results":  []interface{}{truncationQueryRecord("r1", "2026-06-01T00:00:00Z")},
			"has_more": true, "next_cursor": "page-2",
			"request_status": map[string]interface{}{
				"type":              "incomplete",
				"incomplete_reason": "query_result_limit_reached",
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := mustTestClient(t, "test-token", ts.URL)
	records, err := client.QueryDatabaseAll(context.Background(), "ds-1")
	if !errors.Is(err, ErrQueryTruncated) {
		t.Fatalf("expected ErrQueryTruncated, got err=%v records=%d", err, len(records))
	}
	if got := atomic.LoadInt32(&queries); got != 1 {
		t.Errorf("truncation on page 1 must abort before following the cursor, got %d query requests", got)
	}
}

// TestQueryDatabaseAllCompleteRequestStatus (#3836): an explicit
// request_status.type == "complete" is a normal full read — success, all rows.
func TestQueryDatabaseAllCompleteRequestStatus(t *testing.T) {
	allowNotionTestServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/data_sources/ds-1/query", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"results": []interface{}{
				truncationQueryRecord("r1", "2026-06-01T00:00:00Z"),
				truncationQueryRecord("r2", "2026-06-01T00:00:00Z"),
			},
			"has_more": false,
			"request_status": map[string]interface{}{
				"type": "complete",
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := mustTestClient(t, "test-token", ts.URL)
	records, err := client.QueryDatabaseAll(context.Background(), "ds-1")
	if err != nil {
		t.Fatalf("complete request_status must not error: %v", err)
	}
	if errors.Is(err, ErrQueryTruncated) {
		t.Error("complete read must never be flagged as truncated")
	}
	if len(records) != 2 {
		t.Fatalf("expected both records, got %d", len(records))
	}
}

// truncatedIncrementalServer emulates a workspace whose database query can be
// toggled between a truncated subset (r1/r2 + request_status incomplete) and a
// complete read (r1..r4), to exercise incremental round semantics end to end.
func truncatedIncrementalServer(t *testing.T, truncate *atomic.Bool) *types.DataSourceConfig {
	t.Helper()
	allowNotionTestServer(t)
	record := func(id, edited string) map[string]interface{} {
		rec := truncationQueryRecord(id, edited)
		rec["parent"] = map[string]interface{}{"type": "data_source_id", "data_source_id": "db-1"}
		return rec
	}
	writeList := func(w http.ResponseWriter, results []interface{}, hasMore bool, requestStatus map[string]interface{}) {
		resp := map[string]interface{}{"object": "list", "results": results, "has_more": hasMore}
		if hasMore {
			resp["next_cursor"] = "unused"
		}
		if requestStatus != nil {
			resp["request_status"] = requestStatus
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, _ *http.Request) {
		writeList(w, []interface{}{map[string]interface{}{
			"id":               "db-1",
			"object":           "data_source",
			"url":              "https://notion.so/db-1",
			"last_edited_time": "2026-06-01T00:00:00Z", // changed vs cursor
			"in_trash":         false,
			"parent":           map[string]interface{}{"type": "workspace", "workspace": true},
			"title":            []interface{}{map[string]interface{}{"plain_text": "Tasks"}},
		}}, false, nil)
	})
	mux.HandleFunc("/v1/data_sources/db-1", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":               "db-1",
			"object":           "data_source",
			"last_edited_time": "2026-06-01T00:00:00Z",
			"title":            []interface{}{map[string]interface{}{"plain_text": "Tasks"}},
		})
	})
	mux.HandleFunc("/v1/data_sources/db-1/query", func(w http.ResponseWriter, _ *http.Request) {
		if truncate.Load() {
			// Vendor dropped r3/r4; has_more=false makes it look complete.
			writeList(w, []interface{}{
				record("r1", "2026-01-01T00:00:00Z"),
				record("r2", "2026-01-01T00:00:00Z"),
			}, false, map[string]interface{}{
				"type":              "incomplete",
				"incomplete_reason": "query_result_limit_reached",
			})
			return
		}
		// Complete read: r1/r2 unchanged, r3/r4 edited since the last cursor.
		writeList(w, []interface{}{
			record("r1", "2026-01-01T00:00:00Z"),
			record("r2", "2026-01-01T00:00:00Z"),
			record("r3", "2026-06-02T00:00:00Z"),
			record("r4", "2026-06-02T00:00:00Z"),
		}, false, nil)
	})
	// Record block content for buildDatabaseItem (empty pages).
	for _, rid := range []string{"r1", "r2", "r3", "r4"} {
		mux.HandleFunc("/v1/blocks/"+rid+"/children", func(w http.ResponseWriter, _ *http.Request) {
			writeList(w, []interface{}{}, false, nil)
		})
	}
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return makeNotionConfig(&Config{APIKey: "test-token"}, ts.URL, []string{"db-1"})
}

// TestFetchIncrementalTruncatedQueryFailsRound (#3836, end to end): the cursor
// already knows records r1..r4; this round's query only returns r1/r2 because
// the vendor truncated it. The truncated row set must not become the deletion
// baseline — the whole round fails (no items, nil cursor so the previous one is
// kept), and once the source stops truncating, replaying the same old cursor
// produces a normal round with no spurious deletions.
func TestFetchIncrementalTruncatedQueryFailsRound(t *testing.T) {
	truncate := &atomic.Bool{}
	config := truncatedIncrementalServer(t, truncate)
	prev := buildCursor(map[string]time.Time{
		"db-1": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"r1":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"r2":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"r3":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"r4":   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	connector := NewConnector()

	// Truncated round: only r1/r2 come back, r3/r4 were dropped by the vendor.
	truncate.Store(true)
	items, next, err := connector.FetchIncremental(context.Background(), config, prev)
	if err == nil {
		t.Fatal("truncated query must fail the incremental round, not return partial results")
	}
	if !errors.Is(err, ErrQueryTruncated) {
		t.Errorf("round error must match ErrQueryTruncated, got: %v", err)
	}
	if !strings.Contains(err.Error(), "narrow the data source") {
		t.Errorf("round error should guide the user to narrow the source or add filters, got: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("failed round must emit no items (deletion baseline is polluted), got %+v", items)
	}
	for _, item := range items {
		if item.IsDeleted {
			t.Errorf("failed round must not emit deletions, got IsDeleted for %s", item.ExternalID)
		}
	}
	if next != nil {
		t.Error("failed round must return a nil cursor so the previous one is kept and the round is retried")
	}

	// Recovered round with the SAME old cursor: a complete read succeeds,
	// rebuilds the database item for the changed rows, and deletes nothing.
	truncate.Store(false)
	items, next, err = connector.FetchIncremental(context.Background(), config, prev)
	if err != nil {
		t.Fatalf("FetchIncremental() after recovery: %v", err)
	}
	if len(items) != 1 || items[0].ExternalID != "db-1" {
		t.Fatalf("expected a single rebuilt database item for db-1, got %+v", items)
	}
	if items[0].IsDeleted {
		t.Error("recovered round must not report the database as deleted")
	}
	for _, item := range items {
		if item.IsDeleted {
			t.Errorf("no deletions expected once all records are visible again, got IsDeleted for %s", item.ExternalID)
		}
	}
	if next == nil {
		t.Fatal("successful round must return a cursor")
	}
	// The new cursor must carry the full record set (r1..r4), not the truncated one.
	cursorBytes, _ := json.Marshal(next.ConnectorCursor)
	var newCursor notionCursor
	if err := json.Unmarshal(cursorBytes, &newCursor); err != nil {
		t.Fatalf("unmarshal new cursor: %v", err)
	}
	for _, rid := range []string{"db-1", "r1", "r2", "r3", "r4"} {
		if _, ok := newCursor.PageEditTimes[rid]; !ok {
			t.Errorf("new cursor is missing %s after the complete read", rid)
		}
	}
}
