package confluence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFetchByExternalID_ServerPage(t *testing.T) {
	api := &streamAPI{pages: []streamPage{
		{id: "p1", title: "Page One", version: 1},
		{id: "p2", title: "Page Two", version: 3},
	}}
	connector := newStreamConnector(api)

	item, err := connector.FetchByExternalID(context.Background(), streamConfig(), "p2")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "p2" {
		t.Errorf("ExternalID = %q, want p2", item.ExternalID)
	}
	// Title must be the real listing title, not a placeholder (Ruling P-3).
	if item.Title != "Page Two" {
		t.Errorf("Title = %q, want Page Two", item.Title)
	}
	if !strings.Contains(string(item.Content), "Page Two") {
		t.Errorf("Content missing rendered body; got: %q", string(item.Content))
	}
	if item.SourceResourceID != "1" {
		t.Errorf("SourceResourceID = %q, want the selected space id 1", item.SourceResourceID)
	}
	if item.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", item.ContentType)
	}
	if item.Metadata["channel"] != types.ChannelConfluence {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelConfluence)
	}
	if item.Metadata["page_id"] != "p2" || item.Metadata["space_key"] != "ENG" {
		t.Errorf("metadata page_id/space_key = %q/%q, want p2/ENG",
			item.Metadata["page_id"], item.Metadata["space_key"])
	}
	if item.Metadata["creator"] != "Ada" {
		t.Errorf("creator = %q, want Ada (from the single-page body)", item.Metadata["creator"])
	}
	// SubtreeKeep semantics: the Confluence page path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the Confluence page path, want false")
	}
	// The targeted refetch must not re-read pages it did not match.
	if api.bodyCalls != 1 {
		t.Errorf("bodyCalls = %d, want 1 (only the matched page)", api.bodyCalls)
	}
}

func TestFetchByExternalID_CloudPage(t *testing.T) {
	api := &streamAPI{cloud: true, pages: []streamPage{{id: "c1", title: "Cloud Page", version: 2}}}
	connector := newStreamConnector(api)

	item, err := connector.FetchByExternalID(context.Background(), cloudStreamConfig(), "c1")
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.ExternalID != "c1" || item.Title != "Cloud Page" {
		t.Errorf("item = %q/%q, want c1/Cloud Page", item.ExternalID, item.Title)
	}
	if item.SourceResourceID != "1" || item.Metadata["space_key"] != "ENG" {
		t.Errorf("resource/space metadata = %q/%q, want 1/ENG",
			item.SourceResourceID, item.Metadata["space_key"])
	}
}

func TestFetchByExternalID_NotFound(t *testing.T) {
	api := &streamAPI{pages: []streamPage{{id: "p1", title: "Page", version: 1}}}
	connector := newStreamConnector(api)

	_, err := connector.FetchByExternalID(context.Background(), streamConfig(), "p-missing")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("unknown page must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "p-missing") {
		t.Errorf("error should name the missing page, got: %v", err)
	}
}

func TestFetchByExternalID_BodyDeletedAfterListing(t *testing.T) {
	api := &streamAPI{
		pages:      []streamPage{{id: "p1", title: "Page", version: 1}},
		bodyStatus: map[string]int{"p1": 404},
	}
	connector := newStreamConnector(api)

	_, err := connector.FetchByExternalID(context.Background(), streamConfig(), "p1")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("page deleted between listing and read must wrap ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	api := &streamAPI{pages: []streamPage{{id: "p1", title: "Page", version: 1}}}

	_, err := newStreamConnector(api).FetchByExternalID(context.Background(), streamConfig(), "")
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_RequiresSelectedSpace(t *testing.T) {
	api := &streamAPI{pages: []streamPage{{id: "p1", title: "Page", version: 1}}}
	cfg := &types.DataSourceConfig{Credentials: streamConfig().Credentials}

	_, err := newStreamConnector(api).FetchByExternalID(context.Background(), cfg, "p1")
	if err == nil || !strings.Contains(err.Error(), "at least one selected space") {
		t.Fatalf("config without selected spaces must fail upfront, got: %v", err)
	}
}
