package recoverytest

import (
	"database/sql"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/lib/pq"
)

// bootstrapMatrixDatabase creates a throwaway database for one matrix run and
// returns a DSN pointing at it; the database is dropped on cleanup.
func bootstrapMatrixDatabase(t *testing.T, dsn string) string {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres admin: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	if _, err := db.Exec("DROP DATABASE IF EXISTS " + name); err != nil {
		t.Fatalf("drop stale matrix database: %v", err)
	}
	if _, err := db.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create matrix database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
		_, dropErr := db.Exec("DROP DATABASE IF EXISTS " + name)
		if dropErr != nil {
			t.Logf("drop matrix database %s: %v", name, dropErr)
		}
	})
	return strings.Replace(dsn, "/trpc_test", "/"+name, 1)
}

// TestCrashMatrixPostgreSQL runs the same SIGKILL matrix against an isolated
// PostgreSQL schema when TRPC_RECOVERY_PG_DSN is provided. The provider
// applies the versioned migrations into a per-case schema, so the durable
// semantics (fenced leases, checkpoint round-trip, journal reuse, decision
// resume) are exercised on the second supported dialect with real process
// kills - the same acceptance the SQLite matrix provides.
func TestCrashMatrixPostgreSQL(t *testing.T) {
	pgDSN := os.Getenv("TRPC_RECOVERY_PG_DSN")
	if pgDSN == "" {
		t.Skip("TRPC_RECOVERY_PG_DSN unset: PostgreSQL SIGKILL matrix NOT VERIFIED")
	}
	// A throwaway database per run keeps the matrix hermetic: extensions,
	// schemas and leftovers from earlier runs cannot leak between cases.
	freshDSN := bootstrapMatrixDatabase(t, pgDSN)
	bin := buildRecoveryProvider(t)
	t.Setenv("TRPC_RECOVERY_GRAPH_PROVIDER", bin)
	t.Setenv("TRPC_RECOVERY_USE_PG", "1")
	t.Setenv("TRPC_RECOVERY_PG_DSN", freshDSN)

	for _, name := range []string{
		"after_admission",
		"after_plan_before_dispatch",
		"after_result_before_checkpoint",
		"after_finalize",
		"after_side_effect_before_result",
		"unknown_result_user_retry",
		"idempotent_redelivery",
	} {
		t.Run(name, func(t *testing.T) {
			counter := newCounterServer()
			server := httptest.NewServer(counter.handler())
			defer server.Close()
			t.Setenv("TRPC_RECOVERY_COUNTER_URL", server.URL)

			report := runCrashCase(t, name)
			wantStatus := "succeeded"
			if name == "after_side_effect_before_result" {
				wantStatus = "waiting_user"
			}
			if report.FinalStatus != wantStatus {
				t.Fatalf("case %s status=%s want=%s report=%#v", name, report.FinalStatus, wantStatus, report)
			}
			wantCalls := 1
			if name == "unknown_result_user_retry" {
				wantCalls = 2
			}
			if report.ExternalCalls != wantCalls {
				t.Fatalf("case %s calls=%d want=%d", name, report.ExternalCalls, wantCalls)
			}
			if report.LostEvents != 0 {
				t.Fatalf("case %s lost events=%d", name, report.LostEvents)
			}
		})
	}
}
