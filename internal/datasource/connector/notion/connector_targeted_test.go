package notion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFetchByExternalID_Page(t *testing.T) {
	allowNotionTestServer(t)
	ts, cfg := fakeNotion()
	defer ts.Close()

	item, err := NewConnector().FetchByExternalID(
		context.Background(), makeNotionConfig(cfg, ts.URL, []string{"page-1"}), "page-1",
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "page-1" {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, "page-1")
	}
	if item.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", item.Title, "Test Page")
	}
	if item.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", item.ContentType)
	}
	if !strings.Contains(string(item.Content), "Hello world") {
		t.Errorf("Content missing expected text; got: %q", string(item.Content))
	}
	if item.Metadata["channel"] != types.ChannelNotion {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelNotion)
	}
	if item.Metadata["object_type"] != objectTypePage {
		t.Errorf("object_type = %q, want %q", item.Metadata["object_type"], objectTypePage)
	}
	// SubtreeKeep semantics: the Notion page path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the Notion page path, want false")
	}
}

// TestFetchByExternalID_DatabaseRecord verifies that a database-row external id
// routes through fetchPage's record branch (buildRecordItem) and returns the
// row's item, not the whole database table.
func TestFetchByExternalID_DatabaseRecord(t *testing.T) {
	allowNotionTestServer(t)
	ts, cfg := fakeNotion()
	defer ts.Close()

	item, err := NewConnector().FetchByExternalID(
		context.Background(), makeNotionConfig(cfg, ts.URL, []string{"record-1"}), "record-1",
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.ExternalID != "record-1" {
		t.Errorf("ExternalID = %q, want record-1", item.ExternalID)
	}
	if item.Title != "Record One" {
		t.Errorf("Title = %q, want Record One", item.Title)
	}
	if !strings.Contains(string(item.Content), "Record One") {
		t.Errorf("Content missing record title; got: %q", string(item.Content))
	}
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree must stay false for a database record item")
	}
}

func TestFetchByExternalID_NotFound(t *testing.T) {
	allowNotionTestServer(t)
	ts, cfg := fakeNotion()
	defer ts.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeNotionConfig(cfg, ts.URL, []string{"page-1"}), "page-missing",
	)
	if err == nil {
		t.Fatal("expected an error for a page id the fake does not serve")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "page-missing") {
		t.Errorf("error should name the missing page, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	allowNotionTestServer(t)
	ts, cfg := fakeNotion()
	defer ts.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeNotionConfig(cfg, ts.URL, nil), "",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}
