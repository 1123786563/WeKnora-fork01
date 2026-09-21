package drive

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/modules/datasource/connector/feishu/core"
	"github.com/Tencent/WeKnora/internal/types"
)

// fakeDriveTargeted serves the Drive list API (per folder_token), the file
// download endpoint, and the export trio — exactly what FetchByExternalID's
// list-and-filter path (Ruling P-2) needs. filesByFolder emulates a folder
// tree: a folder token maps to its direct children.
func fakeDriveTargeted(t *testing.T, filesByFolder map[string][]core.DriveFile) *core.Config {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, core.TokenResponse{ApiResponse: core.ApiResponse{Code: 0}, TenantAccessToken: "fake-token", Expire: 7200})
	})

	mux.HandleFunc("/open-apis/drive/v1/files", func(w http.ResponseWriter, r *http.Request) {
		folderToken := r.URL.Query().Get("folder_token")
		writeJSON(w, core.DriveFileListResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data:        core.DriveFileListData{Files: filesByFolder[folderToken]},
		})
	})

	// Drive file download for "file" type entries.
	mux.HandleFunc("/open-apis/drive/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("drive-targeted-file-bytes"))
			return
		}
		http.NotFound(w, r)
	})

	// Export trio for docx/sheet/bitable entries.
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
				FileToken: "ft-export-tgt", FileSize: 64, JobStatus: 0, FileName: "targeted.xlsx",
			}},
		})
	})
	mux.HandleFunc("/open-apis/drive/v1/export_tasks/ticket-tgt", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, core.ExportTaskStatusResponse{
			ApiResponse: core.ApiResponse{Code: 0},
			Data: core.ExportTaskStatusData{Result: core.ExportTaskResult{
				FileToken: "ft-export-tgt", FileSize: 64, JobStatus: 0, FileName: "targeted.xlsx",
			}},
		})
	})
	mux.HandleFunc("/open-apis/drive/v1/export_tasks/file/ft-export-tgt/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("drive-targeted-export-bytes"))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return &core.Config{AppID: "test-app-id", AppSecret: "test-app-secret", BaseURL: ts.URL}
}

func TestFetchByExternalID_FileToken(t *testing.T) {
	cfg := fakeDriveTargeted(t, map[string][]core.DriveFile{
		"fld-root": {
			{Token: "ft-tgt", Name: "report.pdf", Type: "file", ParentToken: "fld-root",
				URL: "https://example.test/f", CreatedTime: "1700000000", ModifiedTime: "1711000000"},
			{Token: "ft-other", Name: "other.pdf", Type: "file", ParentToken: "fld-root",
				URL: "https://example.test/o", ModifiedTime: "1711000001"},
		},
	})

	conn := NewDriveConnector(core.RegionFeishuDrive)
	item, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, []string{"fld-root"}), "ft-tgt")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "ft-tgt" {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, "ft-tgt")
	}
	if item.Title != "report.pdf" {
		t.Errorf("Title = %q, want %q", item.Title, "report.pdf")
	}
	if string(item.Content) != "drive-targeted-file-bytes" {
		t.Errorf("Content = %q, want %q", string(item.Content), "drive-targeted-file-bytes")
	}
	if item.SourceResourceID != "fld-root" {
		t.Errorf("SourceResourceID = %q, want %q", item.SourceResourceID, "fld-root")
	}
	if item.Metadata["channel"] != types.ChannelFeishuDrive {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelFeishuDrive)
	}
	if item.Metadata["file_token"] != "ft-tgt" || item.Metadata["obj_type"] != "file" {
		t.Errorf("metadata wrong: %+v", item.Metadata)
	}
	// SubtreeKeep semantics: the raw-file download path never sets
	// ReplacesSubtree, and a targeted re-ingest must keep it that way.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the file download path, want false")
	}
}

// TestFetchByExternalID_NestedFolderAndLarkChannel verifies the subtree walk
// (list → recurse into folders → filter by token, Ruling P-2) and the Lark
// Drive channel variant — one implementation, two connector types.
func TestFetchByExternalID_NestedFolderAndLarkChannel(t *testing.T) {
	cfg := fakeDriveTargeted(t, map[string][]core.DriveFile{
		"fld-root": {
			{Token: "fld-sub", Name: "Subfolder", Type: "folder", ParentToken: "fld-root"},
		},
		"fld-sub": {
			{Token: "ft-nested", Name: "Nested Doc", Type: "docx", ParentToken: "fld-sub",
				URL: "https://example.test/n", CreatedTime: "1700000000", ModifiedTime: "1711000000"},
		},
	})

	conn := NewDriveConnector(core.RegionLarkDrive)
	item, err := conn.FetchByExternalID(context.Background(), makeConfig(cfg, []string{"fld-root"}), "ft-nested")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.ExternalID != "ft-nested" {
		t.Errorf("ExternalID = %q, want ft-nested (matched in a nested folder)", item.ExternalID)
	}
	if item.Metadata["channel"] != types.ChannelLarkDrive {
		t.Errorf("channel = %q, want %q (lark_drive region)", item.Metadata["channel"], types.ChannelLarkDrive)
	}
	if string(item.Content) != "drive-targeted-export-bytes" {
		t.Errorf("Content = %q, want the export fallback bytes", string(item.Content))
	}
	// Default parse mode is export, which never sets ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the docx export path, want false")
	}
}

func TestFetchByExternalID_NotFound(t *testing.T) {
	cfg := fakeDriveTargeted(t, map[string][]core.DriveFile{
		"fld-root": {
			{Token: "ft-other", Name: "other.pdf", Type: "file", ParentToken: "fld-root", ModifiedTime: "1711000001"},
		},
	})

	_, err := NewDriveConnector(core.RegionFeishuDrive).FetchByExternalID(
		context.Background(), makeConfig(cfg, []string{"fld-root"}), "ft-missing",
	)
	if err == nil {
		t.Fatal("expected an error for a token not present under the configured folder")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ft-missing") {
		t.Errorf("error should name the missing token, got: %v", err)
	}
}

func TestFetchByExternalID_NoResourceIDs(t *testing.T) {
	cfg := fakeDriveTargeted(t, nil)

	_, err := NewDriveConnector(core.RegionFeishuDrive).FetchByExternalID(
		context.Background(), makeConfig(cfg, nil), "ft-tgt",
	)
	if err == nil {
		t.Fatal("expected an error when no Drive folders are configured to search")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	cfg := fakeDriveTargeted(t, nil)
	_, err := NewDriveConnector(core.RegionFeishuDrive).FetchByExternalID(
		context.Background(), makeConfig(cfg, []string{"fld-root"}), "",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}
