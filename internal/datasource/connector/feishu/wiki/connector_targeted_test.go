package wiki

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/datasource/connector/feishu/core"
	"github.com/Tencent/WeKnora/internal/types"
)

// fakeWikiTargeted serves exactly what FetchByExternalID needs: the auth
// endpoint, the single-node get_node API, and the export/download endpoints
// used for content retrieval. Mirrors the fakeFeishu* servers in
// connector_test.go; kept separate so the targeted tests do not depend on the
// list endpoints.
func fakeWikiTargeted(t *testing.T, nodes []core.WikiNode) *core.Config {
	t.Helper()
	mux := http.NewServeMux()
	nodeByToken := make(map[string]core.WikiNode, len(nodes))
	for _, n := range nodes {
		n.SpaceID = "space1"
		nodeByToken[n.NodeToken] = n
	}

	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, core.TokenResponse{
			ApiResponse:       core.ApiResponse{Code: 0},
			TenantAccessToken: "fake-token",
			Expire:            7200,
		})
	})

	mux.HandleFunc("/open-apis/wiki/v2/spaces/get_node", func(w http.ResponseWriter, r *http.Request) {
		node, ok := nodeByToken[r.URL.Query().Get("token")]
		if !ok {
			writeJSON(w, core.WikiNodeInfoResponse{
				ApiResponse: core.ApiResponse{Code: 1663001, Msg: "node not found"},
			})
			return
		}
		writeJSON(w, core.WikiNodeInfoResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data:        core.WikiNodeInfoData{Node: node},
		})
	})

	// Export trio (default docx parse mode is export).
	mux.HandleFunc("/open-apis/drive/v1/export_tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writeJSON(w, core.ExportTaskCreateResponse{
				ApiResponse: core.ApiResponse{Code: 0},
				Data:        core.ExportTaskCreateData{Ticket: "ticket-tgt"},
			})
			return
		}
		writeJSON(w, core.ExportTaskStatusResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data: core.ExportTaskStatusData{Result: core.ExportTaskResult{
				FileToken: "ft-tgt", FileSize: 64, JobStatus: 0, FileName: "targeted.docx",
			}},
		})
	})
	mux.HandleFunc("/open-apis/drive/v1/export_tasks/ticket-tgt", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, core.ExportTaskStatusResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data: core.ExportTaskStatusData{Result: core.ExportTaskResult{
				FileToken: "ft-tgt", FileSize: 64, JobStatus: 0, FileName: "targeted.docx",
			}},
		})
	})
	mux.HandleFunc("/open-apis/drive/v1/export_tasks/file/ft-tgt/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("targeted-export-bytes"))
	})

	// Drive download for "file" type nodes.
	mux.HandleFunc("/open-apis/drive/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("targeted-file-bytes"))
			return
		}
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &core.Config{AppID: "test-app-id", AppSecret: "test-app-secret", BaseURL: ts.URL}
}

func TestFetchByExternalID_DocxExportPath(t *testing.T) {
	cfg := fakeWikiTargeted(t, []core.WikiNode{{
		NodeToken:     "nt-tgt",
		ObjToken:      "obj-tgt",
		ObjType:       "docx",
		Title:         "Targeted Doc",
		ObjCreateTime: "1700000000",
		ObjEditTime:   "1711000000",
	}})

	conn := NewConnector(core.RegionFeishu)
	item, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, []string{"space1"}), "nt-tgt")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "nt-tgt" {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, "nt-tgt")
	}
	if item.Title != "Targeted Doc" {
		t.Errorf("Title = %q, want %q", item.Title, "Targeted Doc")
	}
	if string(item.Content) != "targeted-export-bytes" {
		t.Errorf("Content = %q, want %q", string(item.Content), "targeted-export-bytes")
	}
	if item.SourceResourceID != "space1" {
		t.Errorf("SourceResourceID = %q, want %q", item.SourceResourceID, "space1")
	}
	if item.Metadata["obj_type"] != "docx" || item.Metadata["space_id"] != "space1" {
		t.Errorf("metadata wrong: %+v", item.Metadata)
	}
	if item.Metadata["channel"] != types.ChannelFeishu {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelFeishu)
	}
	if got := item.UpdatedAt.Unix(); got != 1711000000 {
		t.Errorf("UpdatedAt = %d, want obj_edit_time 1711000000", got)
	}
	// SubtreeKeep semantics: the export path (default FEISHU_DOCX_PARSE_MODE)
	// never sets ReplacesSubtree — a targeted re-ingest must not sweep prior
	// attachment children it cannot re-list (feishu/core/shared.go precedent).
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the export path, want false")
	}
}

func TestFetchByExternalID_FileNode(t *testing.T) {
	cfg := fakeWikiTargeted(t, []core.WikiNode{{
		NodeToken:    "nt-file",
		ObjToken:     "obj-file",
		ObjType:      "file",
		Title:        "manual.pdf",
		NodeEditTime: "1711468800",
	}})

	conn := NewConnector(core.RegionFeishu)
	item, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, []string{"space1"}), "nt-file")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.ExternalID != "nt-file" || item.Title != "manual.pdf" {
		t.Errorf("item identity wrong: %+v", item)
	}
	if string(item.Content) != "targeted-file-bytes" {
		t.Errorf("Content = %q, want %q", string(item.Content), "targeted-file-bytes")
	}
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree must stay false on the file download path")
	}
}

func TestFetchByExternalID_NotFoundInAnySpace(t *testing.T) {
	cfg := fakeWikiTargeted(t, nil)

	conn := NewConnector(core.RegionFeishu)
	_, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, []string{"space1"}), "nt-missing")
	if err == nil {
		t.Fatal("expected an error for a node that exists in no configured space")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nt-missing") {
		t.Errorf("error should name the missing token, got: %v", err)
	}
}

func TestFetchByExternalID_NoResourceIDs(t *testing.T) {
	cfg := fakeWikiTargeted(t, []core.WikiNode{{NodeToken: "nt-tgt", ObjToken: "obj-tgt", ObjType: "docx", Title: "T"}})

	conn := NewConnector(core.RegionFeishu)
	_, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, nil), "nt-tgt")
	if err == nil {
		t.Fatal("expected an error when no wiki spaces are configured to probe")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	cfg := fakeWikiTargeted(t, nil)
	_, err := NewConnector(core.RegionFeishu).FetchByExternalID(context.Background(), makeConfig(cfg, []string{"space1"}), "")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}

// TestFetchByExternalID_SubtreeChildID verifies that a subtree child id
// ("<node>#<kind>#<token>", e.g. a failed attachment from a prior sync's error
// sample) resolves through its parent node and returns the matching sub-item
// from the docx blocks fan-out.
func TestFetchByExternalID_SubtreeChildID(t *testing.T) {
	t.Setenv("FEISHU_DOCX_PARSE_MODE", "blocks")
	const (
		nodeToken = "nt-docx"
		objToken  = "obj-docx"
		attToken  = "ft-att-1"
		attName   = "report.pdf"
	)
	// Attachment content must exceed core.MinAttachmentBytes to be promoted.
	attContent := bytes.Repeat([]byte("x"), core.MinAttachmentBytes+1)

	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, core.TokenResponse{ApiResponse: core.ApiResponse{Code: 0}, TenantAccessToken: "fake-token", Expire: 7200})
	})
	mux.HandleFunc("/open-apis/wiki/v2/spaces/get_node", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != nodeToken {
			writeJSON(w, core.WikiNodeInfoResponse{ApiResponse: core.ApiResponse{Code: 1663001, Msg: "node not found"}})
			return
		}
		writeJSON(w, core.WikiNodeInfoResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data:        core.WikiNodeInfoData{Node: core.WikiNode{SpaceID: "space1", NodeToken: nodeToken, ObjToken: objToken, ObjType: "docx", Title: "Blocks Doc", NodeEditTime: "1711468800"}},
		})
	})
	mux.HandleFunc("/open-apis/docx/v1/documents/"+objToken+"/blocks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, core.DocxBlocksResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data: core.DocxBlocksData{Items: []core.DocxBlock{
				{BlockID: "b1", BlockType: core.BlockTypePage},
				{BlockID: "b2", BlockType: core.BlockTypeText, Text: &core.BlockText{
					Elements: []core.TextElement{{TextRun: &core.TextRun{Content: "Hello blocks"}}},
				}},
				{BlockID: "b3", BlockType: core.BlockTypeFile, File: &core.BlockFileRef{Token: attToken, Name: attName}},
			}},
		})
	})
	mux.HandleFunc("/open-apis/drive/v1/medias/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(attContent)
			return
		}
		http.NotFound(w, r)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	cfg := &core.Config{AppID: "test-app-id", AppSecret: "test-app-secret", BaseURL: ts.URL}

	childID := nodeToken + "#file#" + attToken
	item, err := NewConnector(core.RegionFeishu).FetchByExternalID(
		context.Background(), makeConfig(cfg, []string{"space1"}), childID,
	)
	if err != nil {
		t.Fatalf("FetchByExternalID(child id) error: %v", err)
	}
	if item.ExternalID != childID {
		t.Errorf("ExternalID = %q, want the child id %q", item.ExternalID, childID)
	}
	if item.Title != attName {
		t.Errorf("Title = %q, want %q", item.Title, attName)
	}
	if len(item.Content) != len(attContent) {
		t.Errorf("Content = %d bytes, want %d", len(item.Content), len(attContent))
	}
	// Only the parent document item carries ReplacesSubtree; a child item never
	// does, and a targeted child re-ingest must not sweep anything.
	if item.ReplacesSubtree {
		t.Error("a sub-item must never set ReplacesSubtree")
	}
}

// TestFetchByExternalID_UnsupportedDocTypeNotFound pins the failure semantics
// for a node that resolves but has no fetchable content (mindnote): the scoped
// reindex path gets a recognizable error, not a nil item.
func TestFetchByExternalID_UnsupportedDocTypeNotFound(t *testing.T) {
	cfg := fakeWikiTargeted(t, []core.WikiNode{{
		NodeToken: "nt-mn", ObjToken: "obj-mn", ObjType: "mindnote", Title: "Mind",
	}})

	_, err := NewConnector(core.RegionFeishu).FetchByExternalID(context.Background(), makeConfig(cfg, []string{"space1"}), "nt-mn")
	if err == nil {
		t.Fatal("expected an error for a node with no fetchable content")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}
