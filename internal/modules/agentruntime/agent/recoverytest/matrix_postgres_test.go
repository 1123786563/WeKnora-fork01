package recoverytest

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/lib/pq"
)

func TestPostgresDSNForDatabaseReplacesAnySourceDatabase(t *testing.T) {
	got, err := postgresDSNForDatabase("postgres://postgres:secret@127.0.0.1:5432/WeKnora?sslmode=disable", "matrix_case")
	if err != nil {
		t.Fatalf("postgresDSNForDatabase() error = %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if parsed.Path != "/matrix_case" {
		t.Fatalf("database path = %q, want %q", parsed.Path, "/matrix_case")
	}
	if parsed.Query().Get("sslmode") != "disable" {
		t.Fatalf("sslmode = %q, want disable", parsed.Query().Get("sslmode"))
	}
}

func TestPostgresMatrixDatabaseNameIsUniqueAndSafe(t *testing.T) {
	if got := postgresMatrixDatabaseName("TestCrashMatrixPostgreSQL/after_admission", 123); got != "testcrashmatrixpostgresql_after_admission_123" {
		t.Fatalf("database name = %q", got)
	}
}

// bootstrapMatrixDatabase creates a throwaway database for one matrix run and
// returns a DSN pointing at it; the database is dropped on cleanup.
func bootstrapMatrixDatabase(t *testing.T, dsn string) string {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres admin: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := postgresMatrixDatabaseName(t.Name(), time.Now().UnixNano())
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
	freshDSN, err := postgresDSNForDatabase(dsn, name)
	if err != nil {
		t.Fatalf("replace matrix database in DSN: %v", err)
	}
	return freshDSN
}

func postgresDSNForDatabase(dsn, name string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" || name == "" {
		return "", fmt.Errorf("invalid PostgreSQL database DSN")
	}
	parsed.Path = "/" + name
	return parsed.String(), nil
}

func postgresMatrixDatabaseName(testName string, nonce int64) string {
	return fmt.Sprintf("%s_%d", strings.ToLower(strings.ReplaceAll(testName, "/", "_")), nonce)
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
		"oauth_park",
		"mcp_set_drift",
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
			if name == "mcp_set_drift" {
				wantStatus = "failed"
			}
			if report.FinalStatus != wantStatus {
				t.Fatalf("case %s status=%s want=%s report=%#v", name, report.FinalStatus, wantStatus, report)
			}
			wantCalls := 1
			if name == "unknown_result_user_retry" {
				wantCalls = 2
			}
			if name == "mcp_set_drift" {
				wantCalls = 0
			}
			if report.ExternalCalls != wantCalls {
				t.Fatalf("case %s calls=%d want=%d counter=%d report=%#v", name, report.ExternalCalls, wantCalls, counter.value(), report)
			}
			if report.LostEvents != 0 {
				t.Fatalf("case %s lost events=%d", name, report.LostEvents)
			}
		})
	}
}
