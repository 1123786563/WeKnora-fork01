package rss

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFetchByExternalID_Item(t *testing.T) {
	feed := newFakeFeed(t)
	feedURL := feed.feedURL()

	// The sync path mints "<feedURL>:<GUID>" for these fixture items.
	externalID := feedURL + ":guid-1"
	item, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feedURL, ""), externalID,
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != externalID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, externalID)
	}
	if item.Title != "Article One" {
		t.Errorf("Title = %q, want Article One", item.Title)
	}
	if !strings.Contains(string(item.Content), "first paragraph") {
		t.Errorf("Content missing full-text article body; got: %q", string(item.Content))
	}
	if item.SourceResourceID != feedURL {
		t.Errorf("SourceResourceID = %q, want the feed URL", item.SourceResourceID)
	}
	if item.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", item.ContentType)
	}
	if item.Metadata["channel"] != types.ChannelRSS {
		t.Errorf("channel = %q, want %q", item.Metadata["channel"], types.ChannelRSS)
	}
	if item.Metadata["guid"] != "guid-1" {
		t.Errorf("guid = %q, want guid-1", item.Metadata["guid"])
	}
	// SubtreeKeep semantics: the RSS item path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the RSS item path, want false")
	}
}

func TestFetchByExternalID_LinkFallbackMatch(t *testing.T) {
	feed := newFakeFeed(t)
	feedURL := feed.feedURL()

	// An id minted from the item's link (e.g. a feed that stopped exposing
	// GUIDs) must still resolve the same entry. The fixture serves article
	// links under the server root, not under /feed.xml.
	externalID := feedURL + ":" + feed.server.URL + "/article/a2"
	item, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feedURL, ""), externalID,
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}
	if item.Title != "Article Two" {
		t.Errorf("Title = %q, want Article Two", item.Title)
	}
	if item.ExternalID != externalID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, externalID)
	}
}

func TestFetchByExternalID_ItemDroppedFromFeed(t *testing.T) {
	feed := newFakeFeed(t)
	feedURL := feed.feedURL()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feedURL, ""), feedURL+":guid-gone",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("dropped item must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "guid-gone") {
		t.Errorf("error should name the dropped item, got: %v", err)
	}
}

func TestFetchByExternalID_ForeignFeed(t *testing.T) {
	feed := newFakeFeed(t)
	feedURL := feed.feedURL()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feedURL, ""),
		"http://elsewhere.test/feed.xml:some-item",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("external id from an unconfigured feed must wrap datasource.ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_UnfetchableFeedSurfacesError(t *testing.T) {
	feed := newFakeFeed(t)
	feedURL := feed.feedURL()
	feed.failFeed.Store(true)

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feedURL, ""), feedURL+":guid-1",
	)
	if err == nil {
		t.Fatal("expected an error when the owning feed cannot be fetched")
	}
	if errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("transient feed failure must not be reported as ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fetch feed") {
		t.Errorf("error should name the feed fetch failure, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	feed := newFakeFeed(t)

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeConfig(feed.feedURL(), ""), "",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}
