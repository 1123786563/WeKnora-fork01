package yuque

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

func targetedDocDetail() v2DocDetail {
	return v2DocDetail{
		ID:               42,
		Type:             "Doc",
		Slug:             "hello",
		Title:            "Hello Doc",
		BookID:           7,
		Format:           "markdown",
		Body:             "# Hello",
		Status:           "1",
		ContentUpdatedAt: "2026-01-15T10:00:00Z",
		WordCount:        123,
		Book:             v2Repo{ID: 7, Namespace: "team/kb"},
	}
}

func TestFetchByExternalID_Doc(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()
	f.handleJSON("/api/v2/repos/docs/42", 200, v2DocDetailResponse{Data: targetedDocDetail()})

	item, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "42",
	)
	if err != nil {
		t.Fatalf("FetchByExternalID() error: %v", err)
	}

	if item.ExternalID != "42" {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, "42")
	}
	if item.Title != "Hello Doc" {
		t.Errorf("Title = %q, want %q", item.Title, "Hello Doc")
	}
	if string(item.Content) != "# Hello" {
		t.Errorf("Content = %q, want %q", string(item.Content), "# Hello")
	}
	if item.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", item.ContentType)
	}
	if item.FileName != "Hello Doc.md" {
		t.Errorf("FileName = %q, want %q", item.FileName, "Hello Doc.md")
	}
	if item.URL != f.server.URL+"/team/kb/hello" {
		t.Errorf("URL = %q, want %q", item.URL, f.server.URL+"/team/kb/hello")
	}
	if item.SourceResourceID != "7" {
		t.Errorf("SourceResourceID = %q, want %q (book id from the detail response)", item.SourceResourceID, "7")
	}
	if want := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC); !item.UpdatedAt.Equal(want) {
		t.Errorf("UpdatedAt = %v, want %v", item.UpdatedAt, want)
	}
	for k, want := range map[string]string{
		"doc_id":  "42",
		"book_id": "7",
		"slug":    "hello",
		"channel": types.ChannelYuque,
	} {
		if item.Metadata[k] != want {
			t.Errorf("metadata[%s] = %q, want %q", k, item.Metadata[k], want)
		}
	}
	// SubtreeKeep semantics: the Yuque doc path never fans out into a
	// reconciled subtree, so a targeted re-ingest must not set ReplacesSubtree.
	if item.ReplacesSubtree {
		t.Error("ReplacesSubtree = true on the Yuque doc path, want false")
	}
}

func TestFetchByExternalID_NotFound(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()
	f.handleJSON("/api/v2/repos/docs/99", 404, apiErrorBody{Message: "Not Found"})

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "99",
	)
	if err == nil {
		t.Fatal("expected an error for a doc id the fake reports as 404")
	}
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("error must wrap datasource.ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error should name the missing doc, got: %v", err)
	}
}

func TestFetchByExternalID_NonNumericID(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "not-a-number",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("a non-numeric doc id must be rejected with ErrItemNotFound, got: %v", err)
	}
}

func TestFetchByExternalID_EmptyExternalID(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "",
	)
	if !errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("empty external id must be rejected with ErrItemNotFound, got: %v", err)
	}
}

// TestFetchByExternalID_DraftRejected pins the semantics for a doc that exists
// but is not syncable (draft status): a plain, descriptive error — NOT
// ErrItemNotFound, because the doc exists and the message must not mislead the
// user into thinking it was deleted. Mirrors walk()'s skip filters.
func TestFetchByExternalID_DraftRejected(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()
	detail := targetedDocDetail()
	detail.ID = 43
	detail.Status = "0" // draft
	f.handleJSON("/api/v2/repos/docs/43", 200, v2DocDetailResponse{Data: detail})

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "43",
	)
	if err == nil {
		t.Fatal("expected an error for a draft doc")
	}
	if errors.Is(err, datasource.ErrItemNotFound) {
		t.Fatalf("a draft exists at the source; the error must not be ErrItemNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("error should mention the doc status, got: %v", err)
	}
}

// TestFetchByExternalID_UnsupportedFormatRejected mirrors walk()'s format
// guard: an html-format doc body is not trustworthy Markdown, so the targeted
// fetch must refuse it with a descriptive error.
func TestFetchByExternalID_UnsupportedFormatRejected(t *testing.T) {
	f := newFakeYuque()
	defer f.Close()
	detail := targetedDocDetail()
	detail.ID = 44
	detail.Format = "html"
	f.handleJSON("/api/v2/repos/docs/44", 200, v2DocDetailResponse{Data: detail})

	_, err := NewConnector().FetchByExternalID(
		context.Background(), makeDSConfig(f, []string{"7"}), "44",
	)
	if err == nil {
		t.Fatal("expected an error for an unsupported doc format")
	}
	if !strings.Contains(err.Error(), "format") {
		t.Errorf("error should mention the doc format, got: %v", err)
	}
}
