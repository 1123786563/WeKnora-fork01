package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkbenchRequestRepositoryCAS(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:request-cas?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&workbenchRequestRow{}); err != nil {
		t.Fatal(err)
	}
	r := NewWorkbenchRequestRepository(db)
	ctx := context.Background()
	if err := r.CreatePending(ctx, WorkbenchRequest{TenantID: 1, ActorID: "u", RequestID: "r", RequestHash: "h", State: "pending"}); err != nil {
		t.Fatal(err)
	}
	if err := r.CreatePending(ctx, WorkbenchRequest{TenantID: 1, ActorID: "u", RequestID: "r", RequestHash: "h", State: "pending"}); err == nil {
		t.Fatal("duplicate request must be rejected by the unique key")
	}
	got, err := r.Get(ctx, 1, "u", "r")
	if err != nil || got.State != "pending" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
