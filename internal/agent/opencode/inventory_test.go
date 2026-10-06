package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	inventoryDirectory = "/craft/run/tenant-1/run-1"
	inventoryProject   = "project-craft-run-1"
)

func inventoryClient(t *testing.T, handler http.HandlerFunc) *Inventory {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, server.Client())
	require.NoError(t, err)
	inventory, err := NewInventory(client, inventoryDirectory, inventoryProject)
	require.NoError(t, err)
	return inventory
}

func inventoryID(n int) string {
	return fmt.Sprintf("ses_%012x%014d", n, n)
}

func inventorySession(id string) SessionInfo {
	return SessionInfo{
		ID: id, ProjectID: inventoryProject, Title: "Run session",
		Location: SessionLocation{Directory: inventoryDirectory, WorkspaceID: "workspace-1"},
		Time:     SessionTime{Created: 100, Updated: 101},
	}
}

func writeInventoryList(w http.ResponseWriter, data []SessionInfo, next string) {
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Data   []SessionInfo `json:"data"`
		Cursor *struct {
			Next string `json:"next,omitempty"`
		} `json:"cursor"`
	}{Data: data, Cursor: &struct {
		Next string `json:"next,omitempty"`
	}{}}
	if next != "" {
		response.Cursor.Next = next
	}
	_ = json.NewEncoder(w).Encode(response)
}

func TestInventoryListSessionsPaginatesAndBindsDirectoryAndProject(t *testing.T) {
	pages := 0
	inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/session", r.URL.Path)
		require.Equal(t, inventoryDirectory, r.Header.Get(directoryHeader))
		require.Equal(t, inventoryProject, r.URL.Query().Get("project"))
		require.Equal(t, "100", r.URL.Query().Get("limit"))
		pages++
		switch r.URL.Query().Get("cursor") {
		case "":
			writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "next-page")
		case "next-page":
			writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(2))}, "")
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	})

	got, err := inventory.ListSessions(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{inventoryID(1), inventoryID(2)}, []string{got[0].ID, got[1].ID})
	require.Len(t, got, 2)
	require.Equal(t, inventoryDirectory, got[1].Location.Directory)
	require.Equal(t, inventoryProject, got[1].ProjectID)
	require.Equal(t, 2, pages)
}

func TestInventoryListSessionsRejectsCursorLoopAndTruncatedPage(t *testing.T) {
	t.Run("cursor loop", func(t *testing.T) {
		requests := 0
		inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
			requests++
			writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(requests))}, "repeat")
		})
		_, err := inventory.ListSessions(context.Background())
		require.ErrorContains(t, err, "cursor")
	})

	t.Run("full page without continuation is ambiguous", func(t *testing.T) {
		inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
			limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
			require.NoError(t, err)
			rows := make([]SessionInfo, limit)
			for i := range rows {
				rows[i] = inventorySession(inventoryID(i))
			}
			writeInventoryList(w, rows, "")
		})
		_, err := inventory.ListSessions(context.Background())
		require.ErrorContains(t, err, "truncated")
	})
}

func TestInventoryListSessionsRequiresCursorEnvelopeOnEveryPage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		final string
	}{
		{name: "missing cursor", final: `{"data":[]}`},
		{name: "null cursor", final: `{"data":[],"cursor":null}`},
	} {
		t.Run(tc.name+" first page", func(t *testing.T) {
			inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.final))
			})
			_, err := inventory.ListSessions(context.Background())
			require.ErrorContains(t, err, "cursor envelope")
		})

		t.Run(tc.name+" later page", func(t *testing.T) {
			inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("cursor") == "next-page" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tc.final))
					return
				}
				writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "next-page")
			})
			_, err := inventory.ListSessions(context.Background())
			require.ErrorContains(t, err, "cursor envelope")
		})
	}

	t.Run("empty next in a present envelope is a valid final page", func(t *testing.T) {
		inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
			writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "")
		})
		got, err := inventory.ListSessions(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 1)
	})
}

func TestInventoryRedirectRejectsChangedProjectLimitOrCursor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		location string
	}{
		{name: "dropped project", location: "/api/session?limit=100"},
		{name: "changed project", location: "/api/session?limit=100&project=another-project"},
		{name: "dropped limit", location: "/api/session?project=" + inventoryProject},
		{name: "changed limit", location: "/api/session?limit=99&project=" + inventoryProject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
				requests++
				if requests == 1 {
					w.Header().Set("Location", tc.location)
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				}
				writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "")
			})
			_, err := inventory.ListSessions(context.Background())
			require.ErrorContains(t, err, "query scope")
			require.Equal(t, 1, requests, "the redirected request must not be sent")
		})
	}
}

func TestInventoryLaterPageRedirectCannotDropOrChangeCursor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		location string
	}{
		{name: "dropped cursor", location: "/api/session?limit=100&project=" + inventoryProject},
		{name: "changed cursor", location: "/api/session?cursor=other&limit=100&project=" + inventoryProject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "next-page")
					return
				}
				if r.URL.Query().Get("cursor") == "next-page" {
					w.Header().Set("Location", tc.location)
					w.WriteHeader(http.StatusTemporaryRedirect)
					return
				}
				writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(2))}, "")
			})
			_, err := inventory.ListSessions(context.Background())
			require.ErrorContains(t, err, "query scope")
			require.Equal(t, 2, requests, "a changed later-page cursor must not be followed")
		})
	}
}

func TestInventoryListSessionsRejectsMalformedDuplicateAndForeignMetadata(t *testing.T) {
	cases := []struct {
		name string
		rows []SessionInfo
	}{
		{name: "empty ID", rows: []SessionInfo{{ProjectID: inventoryProject, Location: SessionLocation{Directory: inventoryDirectory}}}},
		{name: "unsafe ID", rows: []SessionInfo{inventorySession("../other")}},
		{name: "duplicate ID", rows: []SessionInfo{inventorySession(inventoryID(3)), inventorySession(inventoryID(3))}},
		{name: "foreign directory", rows: []SessionInfo{func() SessionInfo {
			s := inventorySession(inventoryID(4))
			s.Location.Directory = "/craft/other"
			return s
		}()}},
		{name: "foreign project", rows: []SessionInfo{func() SessionInfo {
			s := inventorySession(inventoryID(5))
			s.ProjectID = "other-project"
			return s
		}()}},
		{name: "missing location", rows: []SessionInfo{{ID: inventoryID(6), ProjectID: inventoryProject}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) { writeInventoryList(w, tc.rows, "") })
			_, err := inventory.ListSessions(context.Background())
			require.Error(t, err)
		})
	}
}

func TestInventoryGetSessionByIDRequiresExactTypedMetadata(t *testing.T) {
	t.Run("exact metadata", func(t *testing.T) {
		inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/session/"+inventoryID(1), r.URL.Path)
			require.Empty(t, r.URL.RawQuery, "the pinned GET-by-ID route has no project query schema")
			require.Equal(t, inventoryDirectory, r.Header.Get(directoryHeader))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]SessionInfo{"data": inventorySession(inventoryID(1))})
		})
		got, err := inventory.GetSession(context.Background(), inventoryID(1))
		require.NoError(t, err)
		require.Equal(t, inventorySession(inventoryID(1)), got)
	})

	for _, tc := range []struct {
		name string
		row  SessionInfo
	}{
		{name: "mismatched ID", row: inventorySession(inventoryID(7))},
		{name: "foreign directory", row: func() SessionInfo { s := inventorySession(inventoryID(1)); s.Location.Directory = "/other"; return s }()},
		{name: "foreign project", row: func() SessionInfo { s := inventorySession(inventoryID(1)); s.ProjectID = "other"; return s }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]SessionInfo{"data": tc.row})
			})
			_, err := inventory.GetSession(context.Background(), inventoryID(1))
			require.Error(t, err)
		})
	}
}

func TestInventoryRejectsInvalidScopeAndMalformedLookupIDBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeInventoryList(w, []SessionInfo{}, "")
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	require.NoError(t, err)
	_, err = NewInventory(client, "/craft/run/../other", inventoryProject)
	require.Error(t, err)
	bound, err := client.WithDirectory("/craft/run/tenant-1/other-run")
	require.NoError(t, err)
	_, err = NewInventory(bound, inventoryDirectory, inventoryProject)
	require.Error(t, err)

	inventory, err := NewInventory(client, inventoryDirectory, inventoryProject)
	require.NoError(t, err)
	_, err = inventory.GetSession(context.Background(), "../other")
	require.Error(t, err)
	require.Zero(t, requests, "invalid scope and lookup IDs must fail before any HTTP request")
}

func TestInventoryRejectsLegacyRoute404AndCrossOriginRedirect(t *testing.T) {
	t.Run("legacy API mismatch", func(t *testing.T) {
		inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
		_, err := inventory.ListSessions(context.Background())
		require.Error(t, err)
		require.ErrorContains(t, err, "404")
	})
	t.Run("GET-by-ID legacy mismatch", func(t *testing.T) {
		inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
		_, err := inventory.GetSession(context.Background(), inventoryID(1))
		require.Error(t, err)
		require.ErrorContains(t, err, "404")
	})

	t.Run("cross origin redirect", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("cross-origin redirect must not be followed") }))
		defer target.Close()
		inventory := inventoryClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", target.URL)
			w.WriteHeader(http.StatusTemporaryRedirect)
		})
		_, err := inventory.ListSessions(context.Background())
		require.Error(t, err)
		require.ErrorContains(t, err, "origin")
	})
}

func TestInventoryFollowsSameOriginRedirectWithoutLosingDirectory(t *testing.T) {
	requests := 0
	inventory := inventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, inventoryDirectory, r.Header.Get(directoryHeader))
		require.Equal(t, inventoryProject, r.URL.Query().Get("project"))
		require.Equal(t, "100", r.URL.Query().Get("limit"))
		if requests == 1 {
			writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(1))}, "next-page")
			return
		}
		require.Equal(t, "next-page", r.URL.Query().Get("cursor"))
		if requests == 2 {
			w.Header().Set("Location", "/api/session?"+r.URL.RawQuery)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		require.Equal(t, "/api/session", r.URL.Path)
		writeInventoryList(w, []SessionInfo{inventorySession(inventoryID(2))}, "")
	})
	got, err := inventory.ListSessions(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{inventoryID(1), inventoryID(2)}, []string{got[0].ID, got[1].ID})
	require.Equal(t, 3, requests)
}
