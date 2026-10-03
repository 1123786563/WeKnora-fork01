package container

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
)

// CAREER-OCR H6: the production read handler must wire the task-grant
// fallback (WithGrantedRuns), or a Viewer/Collaborator grant holder's task
// detail/snapshot read degrades to owner-only 404 while the grant routes
// (task_grant_wiring_test.go) keep handing out grants — "grantable but
// unreadable". The grants/research/delivery providers already wire the same
// reader; this pins the read handler.
func TestNewWorkbenchReadHandlerWiresGrantedRuns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-mem db: %v", err)
	}
	h := NewWorkbenchReadHandler(
		repository.NewAgentRunStore(db),
		repository.NewAgentRunSnapshotRepository(db),
		repository.NewExecutionObservationStore(db),
		repository.NewWorkbenchListStore(db),
	)
	if h == nil {
		t.Fatal("NewWorkbenchReadHandler built as nil")
	}
	granted := reflect.ValueOf(h).Elem().FieldByName("granted")
	if !granted.IsValid() {
		t.Fatal("WorkbenchReadHandler has no granted field")
	}
	if granted.IsNil() {
		t.Error("granted reader is nil — task-grant holders would read owner-only in production assembly")
	}
	_ = any(h).(*session.WorkbenchReadHandler)
}
