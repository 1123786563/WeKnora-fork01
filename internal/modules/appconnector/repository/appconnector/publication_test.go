package appconnector

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPublicationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func plannedRow(actionID string) PublicationRow {
	return PublicationRow{
		TenantID: 7, ActionID: actionID, ConnectionID: "conn-notion", Provider: "notion",
		Mode: "create", Destination: "parent-1", ExpectedVersion: "2026-09-24T08:00:00.000Z",
		ArtifactVersionID: "ver-1", ArtifactDigest: "d1", State: PublicationPlanned,
	}
}

func TestPublicationCreateFindRoundTrip(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	row := plannedRow("act-1")
	if err := store.CreatePublication(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindByAction(context.Background(), 7, "act-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "notion" || got.Mode != "create" || got.Destination != "parent-1" ||
		got.ExpectedVersion != "2026-09-24T08:00:00.000Z" || got.ArtifactVersionID != "ver-1" {
		t.Fatalf("round trip drift: %+v", got)
	}
	// Duplicate plan for the same action is a conflict, never a silent
	// overwrite of the approval binding.
	if err := store.CreatePublication(context.Background(), row); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("duplicate create must conflict, got %v", err)
	}
	if _, err := store.FindByAction(context.Background(), 8, "act-1"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("cross-tenant lookup must be not-found, got %v", err)
	}
}

func TestPublicationSettleTransitions(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	if err := store.CreatePublication(ctx, plannedRow("act-1")); err != nil {
		t.Fatal(err)
	}
	// planned -> unknown (parked for provider query) -> published.
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationUnknown, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationPublished, "page-9", "2026-09-24T12:00:00.000Z", `{"object":"page"}`); err != nil {
		t.Fatal(err)
	}
	got, _ := store.FindByAction(ctx, 7, "act-1")
	if got.State != PublicationPublished || got.ExternalID != "page-9" ||
		got.ExternalVersion != "2026-09-24T12:00:00.000Z" || got.ReceiptJSON == "" {
		t.Fatalf("settle must record the external version + receipt: %+v", got)
	}
	// Terminal states never move again — a late settle cannot rewrite the
	// recorded receipt.
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationFailed, "", "", ""); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("published is terminal, got %v", err)
	}
	// planned -> failed is legal.
	if err := store.CreatePublication(ctx, plannedRow("act-2")); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-2", PublicationFailed, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-2", PublicationPublished, "x", "y", ""); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("failed is terminal, got %v", err)
	}
}

func TestPublicationSaveProgressIdempotent(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	if err := store.CreatePublication(ctx, plannedRow("act-1")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProgress(ctx, 7, "act-1", `{"page_id":"page-9","blocks_done":2}`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProgress(ctx, 7, "act-1", `{"page_id":"page-9","blocks_done":4}`); err != nil {
		t.Fatal(err)
	}
	got, _ := store.FindByAction(ctx, 7, "act-1")
	if got.ProgressJSON != `{"page_id":"page-9","blocks_done":4}` {
		t.Fatalf("progress must be last-write-wins durable checkpoint: %q", got.ProgressJSON)
	}
	// Progress writes never touch the state columns.
	if got.State != PublicationPlanned {
		t.Fatalf("progress must not settle: %s", got.State)
	}
}

func TestPublicationLatestPublishedByDestination(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	row := plannedRow("act-1")
	row.Mode = "update"
	row.Destination = "page-9"
	if err := store.CreatePublication(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationPublished, "page-9", "v1", "{}"); err != nil {
		t.Fatal(err)
	}
	got, err := store.LatestPublishedByDestination(ctx, 7, "conn-notion", "page-9")
	if err != nil || got.ActionID != "act-1" {
		t.Fatalf("published receipt must be discoverable: %+v %v", got, err)
	}
	if _, err := store.LatestPublishedByDestination(ctx, 7, "conn-notion", "page-other"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("unpublished destination must be not-found, got %v", err)
	}
	if _, err := store.LatestPublishedByDestination(ctx, 8, "conn-notion", "page-9"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("cross-tenant destination must be not-found, got %v", err)
	}
	_ = time.Now
}
