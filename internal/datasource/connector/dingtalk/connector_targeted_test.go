package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

// targetedAPI serves one workspace whose root holds a folder with a nested
// document, mirroring the surfaces FetchByExternalID walks.
func targetedAPI() *fakeAPI {
	return &fakeAPI{
		workspaces: []workspace{{ID: "ws", RootNodeID: "root", Name: "Workspace"}},
		nodes: map[string][]node{
			"root": {
				{ID: "folder", Type: "FOLDER"},
				{ID: "other", Type: "FILE", Category: "ALIDOC", Extension: "adoc",
					Name: "Unrelated", WorkspaceID: "ws"},
			},
			"folder": {
				{
					ID: "doc-1", Type: "FILE", Category: "ALIDOC", Extension: "adoc",
					Name: "Roadmap", WorkspaceID: "ws", ModifiedTime: "2026-07-25T08:00:00Z",
				},
			},
		},
		blocks: map[string][]json.RawMessage{
			"doc-1": {rawJSON(`{
				"blockType":"paragraph",
				"children":[{"elementType":"text","text":"Q3 goals","bold":true}]
			}`)},
			"other": {rawJSON(`{
				"blockType":"paragraph",
				"children":[{"elementType":"text","text":"unrelated"}]
			}`)},
		},
		nodeErrors:  make(map[string]error),
		blockErrors: make(map[string]error),
	}
}

func TestFetchByExternalID_Document(t *testing.T) {
	api := targetedAPI()
	connector := testConnector(api)

	item, err := connector.FetchByExternalID(context.Background(), testConfig("ws"), "doc-1")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "doc-1" {
		t.Errorf("ExternalID = %q, want doc-1", item.ExternalID)
	}
	// Title and timestamps must be the real listing metadata recovered by the
	// node-id filter, not placeholders.
	if item.Title != "Roadmap" {
		t.Errorf("Title = %q, want Roadmap", item.Title)
	}
	if item.Content == nil || string(item.Content) != "# Roadmap\n\n**Q3 goals**\n" {
		t.Errorf("Content = %q, want the rendered document", string(item.Content))
	}
	if item.SourceResourceID != "ws" {
		t.Errorf("SourceResourceID = %q, want ws", item.SourceResourceID)
	}
	if item.Metadata["channel"] != types.ChannelDingtalk {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelDingtalk)
	}
	if item.Metadata["node_id"] != "doc-1" || item.Metadata["workspace_id"] != "ws" {
		t.Errorf("node/workspace metadata = %q/%q, want doc-1/ws",
			item.Metadata["node_id"], item.Metadata["workspace_id"])
	}
	if item.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero, want the listing modifiedTime")
	}
	// SubtreeKeep semantics: the document path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the DingTalk document path, want false")
	}
	// Only the matched document is read: the sibling must not be fetched.
	if api.blockCalls["other"] != 0 {
		t.Errorf("documentBlocks called for unrelated node %d times, want 0", api.blockCalls["other"])
	}
}

// TestFetchByExternalID_SingleDocumentSelection verifies the scope that
// selects one document directly (the encoded resource reference) refetches
// that document through its Document shortcut.
func TestFetchByExternalID_SingleDocumentSelection(t *testing.T) {
	api := targetedAPI()
	connector := testConnector(api)

	// doc-1 lives under "folder", so the saved selection records the ancestor.
	resourceID, err := encodeResourceReference(resourceReference{
		WorkspaceID: "ws", NodeID: "doc-1", Ancestors: []string{"folder"},
	})
	if err != nil {
		t.Fatalf("encodeResourceReference() error: %v", err)
	}
	item, err := connector.FetchByExternalID(context.Background(), testConfig(resourceID), "doc-1")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.ExternalID != "doc-1" || item.Title != "Roadmap" {
		t.Errorf("item = %q/%q, want doc-1/Roadmap", item.ExternalID, item.Title)
	}
	if item.SourceResourceID != resourceID {
		t.Errorf("SourceResourceID = %q, want %q", item.SourceResourceID, resourceID)
	}
}

func TestFetchByExternalID_NotFound(t *testing.T) {
	connector := testConnector(targetedAPI())

	_, err := connector.FetchByExternalID(context.Background(), testConfig("ws"), "doc-missing")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("unknown document must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	connector := testConnector(targetedAPI())

	_, err := connector.FetchByExternalID(context.Background(), testConfig("ws"), "")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_RequiresSelectedResources(t *testing.T) {
	connector := testConnector(targetedAPI())

	_, err := connector.FetchByExternalID(context.Background(), testConfig(), "doc-1")
	if err == nil || err.Error() != "no DingTalk resources selected" {
		t.Fatalf("config without resources must fail upfront, got: %v", err)
	}
}

// TestFetchByExternalID_ScanFailureSurfaced pins that a scope that cannot be
// listed is reported as a scan failure rather than a per-item not-found.
func TestFetchByExternalID_ScanFailureSurfaced(t *testing.T) {
	api := targetedAPI()
	api.nodeErrors["folder"] = errors.New("listing boom")
	connector := testConnector(api)

	_, err := connector.FetchByExternalID(context.Background(), testConfig("ws"), "doc-1")
	if err == nil {
		t.Fatal("expected the scan failure to surface")
	}
	if errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("scan failure must not be reported as ErrItemNotFound, got: %v", err)
	}
	if !errors.Is(err, api.nodeErrors["folder"]) {
		t.Errorf("error should wrap the listing failure, got: %v", err)
	}
}
