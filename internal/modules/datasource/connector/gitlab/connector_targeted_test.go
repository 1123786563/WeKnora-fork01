package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

// newGitLabTargetedServer serves one project (id 42) with a single readable
// file README.md@main, mirroring the API surface FetchByExternalID touches.
func newGitLabTargetedServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 42,
			"name": "repo",
			"path_with_namespace": "group/repo",
			"web_url": "https://gitlab.test/group/repo",
			"default_branch": "main"
		}`))
	})
	mux.HandleFunc("/api/v4/projects/42/repository/files/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/raw") || !strings.Contains(r.URL.Path, "README") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("ref") != "main" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("readme body\n"))
	})
	mux.HandleFunc("/api/v4/projects/999", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func gitLabTargetedConfig(server *httptest.Server) *types.DataSourceConfig {
	return &types.DataSourceConfig{
		Type: types.ConnectorTypeGitLab,
		Credentials: map[string]interface{}{
			"base_url":     server.URL,
			"access_token": "test-token-1",
		},
	}
}

func gitLabExternalID(server *httptest.Server, projectID, ref, file string) string {
	// Mirror item()'s minting: canonicalBase is the normalized base URL with
	// the /api/v4 suffix the client appends.
	return fmt.Sprintf("gitlab:%s/api/v4:%s:%s:%s", server.URL, projectID, ref, file)
}

func TestFetchByExternalID_File(t *testing.T) {
	allowLocalGitLabServer(t)
	server := newGitLabTargetedServer(t)
	defer server.Close()

	externalID := gitLabExternalID(server, "42", "main", "README.md")
	item, err := NewConnector().FetchByExternalID(
		context.Background(), gitLabTargetedConfig(server), externalID,
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != externalID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, externalID)
	}
	if item.Title != "group/repo/README.md" {
		t.Errorf("Title = %q, want group/repo/README.md", item.Title)
	}
	if string(item.Content) != "readme body\n" {
		t.Errorf("Content = %q, want readme body\\n", string(item.Content))
	}
	if item.SourceResourceID != "42" {
		t.Errorf("SourceResourceID = %q, want 42", item.SourceResourceID)
	}
	if item.Metadata["channel"] != types.ConnectorTypeGitLab {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ConnectorTypeGitLab)
	}
	if item.Metadata["gitlab_ref"] != "main" || item.Metadata["gitlab_path"] != "README.md" {
		t.Errorf("metadata ref/path = %q/%q, want main/README.md",
			item.Metadata["gitlab_ref"], item.Metadata["gitlab_path"])
	}
	// SubtreeKeep semantics: the git file path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the git file path, want false")
	}
}

func TestFetchByExternalID_FileNotFound(t *testing.T) {
	allowLocalGitLabServer(t)
	server := newGitLabTargetedServer(t)
	defer server.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), gitLabTargetedConfig(server),
		gitLabExternalID(server, "42", "main", "GONE.md"),
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("missing file must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "GONE.md") {
		t.Errorf("error should name the missing file, got: %v", err)
	}
}

func TestFetchByExternalID_ProjectNotFound(t *testing.T) {
	allowLocalGitLabServer(t)
	server := newGitLabTargetedServer(t)
	defer server.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), gitLabTargetedConfig(server),
		gitLabExternalID(server, "999", "main", "README.md"),
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("missing project must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "999") {
		t.Errorf("error should name the missing project, got: %v", err)
	}
}

func TestFetchByExternalID_ForeignBase(t *testing.T) {
	allowLocalGitLabServer(t)
	server := newGitLabTargetedServer(t)
	defer server.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), gitLabTargetedConfig(server),
		"gitlab:https://gitlab.elsewhere.test/api/v4:42:main:README.md",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("external id from another instance must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_MalformedAndEmpty(t *testing.T) {
	allowLocalGitLabServer(t)
	server := newGitLabTargetedServer(t)
	defer server.Close()
	cfg := gitLabTargetedConfig(server)

	for _, externalID := range []string{"", "gitlab:only-base", fmt.Sprintf("gitlab:%s/api/v4:42", server.URL)} {
		_, err := NewConnector().FetchByExternalID(context.Background(), cfg, externalID)
		if !errors.Is(err, datasource.ErrItemNotFound) {
			t.Fatalf("externalID %q must be rejected with ErrItemNotFound, got: %v", externalID, err)
		}
	}
}
