package commercial

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testPlanVersionStore mirrors the order_test.go SQLite harness: one pooled
// connection serializes goroutines at the driver level (SQLite is
// single-writer anyway). EnsureSchema bootstraps the catalog + publications
// tables and the immutability trigger, so the store is testable without the
// migration files.
func testPlanVersionStore(t *testing.T) (*PlanVersionStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	s := NewPlanVersionStore(db)
	if err := s.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, db
}

func draftRow(planKey string, version int64, externalID string) PlanRow {
	return PlanRow{
		PlanKey:        planKey,
		Version:        version,
		DefinitionJSON: `{"Key":"` + planKey + `","Version":` + strconv.FormatInt(version, 10) + `,"Price":9900,"Monthly":9900000}`,
		ExternalID:     externalID,
		State:          domain.PlanStateDraft,
	}
}

func publication(planKey string, version int64, commandKey, planCode string) PublicationRow {
	return PublicationRow{
		CommandKey:  commandKey,
		PlanKey:     planKey,
		Version:     version,
		PlanCode:    planCode,
		ReceiptJSON: `{"key":"` + commandKey + `","external_id":"` + planCode + `","recorded_at":"2026-09-21T12:00:00Z"}`,
		PublishedBy: "operator-1",
		PublishedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}
}

// TestNextVersionMonotonic: max(version)+1; a fresh plan key starts at 1.
func TestNextVersionMonotonic(t *testing.T) {
	s, _ := testPlanVersionStore(t)
	ctx := context.Background()
	for v := int64(1); v <= 3; v++ {
		if err := s.CreateDraft(ctx, draftRow("pro", v, domain.DeterministicPlanCode("pro", v))); err != nil {
			t.Fatalf("seed v%d: %v", v, err)
		}
	}
	next, err := s.NextVersion(ctx, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if next != 4 {
		t.Fatalf("next version after v3 = %d, want 4", next)
	}
	fresh, err := s.NextVersion(ctx, "brand-new")
	if err != nil {
		t.Fatal(err)
	}
	if fresh != 1 {
		t.Fatalf("fresh plan key starts at %d, want 1", fresh)
	}
}

// TestDraftLifecycle: create draft → mutate draft → SetPublishing →
// RecordPublication flips the catalog row to published AND inserts the
// publication in ONE transaction (asserted by one read-back).
func TestDraftLifecycle(t *testing.T) {
	s, _ := testPlanVersionStore(t)
	ctx := context.Background()
	if err := s.CreateDraft(ctx, draftRow("pro", 1, domain.DeterministicPlanCode("pro", 1))); err != nil {
		t.Fatal(err)
	}
	updated := `{"Key":"pro","Version":1,"Price":9900,"Monthly":9900000,"Currency":"CNY"}`
	if err := s.UpdateDraft(ctx, "pro", 1, updated); err != nil {
		t.Fatalf("update draft: %v", err)
	}
	var row PlanRow
	if err := rawCatalog(t, s, "pro", 1).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.DefinitionJSON != updated || row.State != domain.PlanStateDraft {
		t.Fatalf("draft update must mutate the definition and keep state=draft, got %+v", row)
	}
	if err := s.SetPublishing(ctx, "pro", 1); err != nil {
		t.Fatalf("set publishing: %v", err)
	}
	pub := publication("pro", 1, domain.PublishCommandKey("pro", 1), domain.DeterministicPlanCode("pro", 1))
	if err := s.RecordPublication(ctx, pub, "pro", 1); err != nil {
		t.Fatalf("record publication: %v", err)
	}
	// One read-back asserts BOTH effects of the atomic transaction.
	if err := rawCatalog(t, s, "pro", 1).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != domain.PlanStatePublished {
		t.Fatalf("catalog row state = %q, want published", row.State)
	}
	got, err := s.GetPublication(ctx, "pro", 1)
	if err != nil {
		t.Fatalf("get publication: %v", err)
	}
	if got.CommandKey != pub.CommandKey || got.PlanCode != pub.PlanCode || got.PublishedBy != pub.PublishedBy {
		t.Fatalf("publication mismatch: %+v", got)
	}
}

func rawCatalog(t *testing.T, s *PlanVersionStore, planKey string, version int64) *gorm.DB {
	t.Helper()
	return s.db.Raw(`SELECT plan_key, version, definition_json, external_id, state FROM commercial_plan_catalog WHERE plan_key = ? AND version = ?`, planKey, version)
}

// TestPublishedRowImmutableEveryLayer: after RecordPublication the row is
// immutable at EVERY layer — the store guard, the DB trigger (direct SQL
// UPDATE must fail), and the legacy SaveDefinition path.
func TestPublishedRowImmutableEveryLayer(t *testing.T) {
	s, db := testPlanVersionStore(t)
	ctx := context.Background()
	if err := s.CreateDraft(ctx, draftRow("pro", 1, domain.DeterministicPlanCode("pro", 1))); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPublishing(ctx, "pro", 1); err != nil {
		t.Fatal(err)
	}
	pub := publication("pro", 1, domain.PublishCommandKey("pro", 1), domain.DeterministicPlanCode("pro", 1))
	if err := s.RecordPublication(ctx, pub, "pro", 1); err != nil {
		t.Fatal(err)
	}

	// (a) Store guard.
	err := s.UpdateDraft(ctx, "pro", 1, `{"tampered":true}`)
	if !errors.Is(err, ErrPublishedPlanImmutable) {
		t.Fatalf("UpdateDraft on published must fail ErrPublishedPlanImmutable, got %v", err)
	}
	// (b) The DB trigger: a DIRECT SQL UPDATE must fail — the constraint is
	// in the database, not only in the service.
	err = db.Exec(`UPDATE commercial_plan_catalog SET definition_json = ? WHERE plan_key = ? AND version = ?`,
		`{"tampered":true}`, "pro", 1).Error
	if err == nil {
		t.Fatal("direct SQL UPDATE of a published definition_json must fail under the trigger")
	}
	if !strings.Contains(err.Error(), "published_plan_immutable") {
		t.Fatalf("trigger error must carry the closed token, got %v", err)
	}
	err = db.Exec(`UPDATE commercial_plan_catalog SET external_id = ? WHERE plan_key = ? AND version = ?`,
		"weknora-pro-v99", "pro", 1).Error
	if err == nil || !strings.Contains(err.Error(), "published_plan_immutable") {
		t.Fatalf("direct SQL UPDATE of a published external_id must fail under the trigger, got %v", err)
	}
	// A state-only UPDATE on a publishing row stays legal (the publish flip
	// itself must never trip the trigger).
	if err := s.CreateDraft(ctx, draftRow("pro", 2, domain.DeterministicPlanCode("pro", 2))); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE commercial_plan_catalog SET state = ? WHERE plan_key = ? AND version = ?`,
		domain.PlanStatePublishing, "pro", 2).Error; err != nil {
		t.Fatalf("state flip on a draft row must not trip the trigger: %v", err)
	}
	// (c) Legacy path.
	catalog := NewCatalogStore(db)
	err = catalog.SaveDefinition(ctx, draftRow("pro", 1, domain.DeterministicPlanCode("pro", 1)))
	if !errors.Is(err, ErrPublishedPlanImmutable) {
		t.Fatalf("legacy SaveDefinition on published must fail ErrPublishedPlanImmutable, got %v", err)
	}
}

// TestRecordPublicationIdempotent: replaying the same CommandKey
// verifies-equals and returns without error or a second row; a DIFFERENT
// stored publication under the same key is a conflict.
func TestRecordPublicationIdempotent(t *testing.T) {
	s, db := testPlanVersionStore(t)
	ctx := context.Background()
	if err := s.CreateDraft(ctx, draftRow("pro", 1, domain.DeterministicPlanCode("pro", 1))); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPublishing(ctx, "pro", 1); err != nil {
		t.Fatal(err)
	}
	pub := publication("pro", 1, domain.PublishCommandKey("pro", 1), domain.DeterministicPlanCode("pro", 1))
	if err := s.RecordPublication(ctx, pub, "pro", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPublication(ctx, pub, "pro", 1); err != nil {
		t.Fatalf("idempotent replay of the same publication must succeed, got %v", err)
	}
	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM commercial_plan_publications`).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("replay must not insert a second row, got %d", n)
	}

	conflicting := pub
	conflicting.ReceiptJSON = `{"key":"other"}`
	if err := s.RecordPublication(ctx, conflicting, "pro", 1); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("same key with different content must conflict, got %v", err)
	}
}

// TestListAndGetVersionsJoinReceipts: the version views LEFT JOIN the
// publications — a published version carries the receipt and published_at,
// a draft carries neither.
func TestListAndGetVersionsJoinReceipts(t *testing.T) {
	s, _ := testPlanVersionStore(t)
	ctx := context.Background()
	if err := s.CreateDraft(ctx, draftRow("pro", 1, domain.DeterministicPlanCode("pro", 1))); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDraft(ctx, draftRow("lite", 1, domain.DeterministicPlanCode("lite", 1))); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPublishing(ctx, "pro", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordPublication(ctx,
		publication("pro", 1, domain.PublishCommandKey("pro", 1), domain.DeterministicPlanCode("pro", 1)), "pro", 1); err != nil {
		t.Fatal(err)
	}

	list, err := s.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 version rows, got %d", len(list))
	}
	var published, draft *VersionView
	for i := range list {
		switch {
		case list[i].PlanKey == "pro":
			published = &list[i]
		case list[i].PlanKey == "lite":
			draft = &list[i]
		}
	}
	if published == nil || draft == nil {
		t.Fatalf("want one pro and one lite row, got %+v", list)
	}
	if published.State != domain.PlanStatePublished || published.ReceiptJSON == "" || published.PublishedAt == nil {
		t.Fatalf("published version must carry receipt presence + published_at, got %+v", published)
	}
	if draft.State != domain.PlanStateDraft || draft.ReceiptJSON != "" || draft.PublishedAt != nil {
		t.Fatalf("draft version must carry neither receipt nor published_at, got %+v", draft)
	}

	got, err := s.GetVersion(ctx, "pro", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.PlanStatePublished || got.ReceiptJSON == "" {
		t.Fatalf("GetVersion(pro,1) must join the receipt, got %+v", got)
	}
	if _, err := s.GetVersion(ctx, "pro", 99); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("missing version must answer ErrPlanNotFound, got %v", err)
	}
}
