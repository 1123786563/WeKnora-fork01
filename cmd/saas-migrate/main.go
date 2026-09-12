// Command saas-migrate migrates legacy spaces into the commercial model
// (O02). It is deliberately conservative: it plans from evidence only
// (existing F02 Customer mappings, operator-provided Customer IDs, proven
// connection ownership), defaults to a zero-write dry-run, requires an
// explicit --apply for any write, and re-binds through the idempotent F02
// AccountStore.Bind so a repeated apply never creates a second Customer.
//
// Usage:
//
//	saas-migrate -tenants 1,2,3 -db-driver sqlite -dsn file:migrate.db
//	saas-migrate -tenants 1,2 -customer-map 2=cus_op_42 -apply -dsn file:migrate.db
//
// It only uses existing stores (commercial_accounts via AccountStore); it
// never writes tables owned by other tasks.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domcom "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// tenantReport is one per-tenant line of the operator report.
type tenantReport struct {
	TenantID               uint64
	Plan                   string
	IssueCredits           bool
	ConnectionState        string
	ConnectionOwnerProven  bool
	CustomerID             string
	ReuseExistingCustomer  bool
	BindRequired           bool
	Applied                bool
	SkipReason             string
	LegacyBalanceConverted bool
	DataSourcesPreserved   bool
	Error                  string
}

func main() {
	var (
		tenantsFlag  = flag.String("tenants", "", "explicit comma-separated tenant ID list (required unless -tenants-file is set)")
		tenantsFile  = flag.String("tenants-file", "", "file with one tenant ID per line (alternative to -tenants)")
		customerFlag = flag.String("customer-map", "", "comma-separated tenantID=customerID pairs verified by the operator, e.g. 7=cus_x,8=cus_y")
		provenFlag   = flag.String("proven-owners", "", "comma-separated tenant IDs with proven connection-ownership evidence (A07 binding); every other tenant stays requires_reauthorization")
		dryRun       = flag.Bool("dry-run", true, "report only, zero writes (default)")
		apply        = flag.Bool("apply", false, "explicitly apply the plan; without this flag the command never writes")
		dbDriver     = flag.String("db-driver", "sqlite", "database driver: sqlite or postgres")
		dsn          = flag.String("dsn", "", "database DSN (required for --apply; enables reading existing mappings in dry-run)")
	)
	flag.Parse()

	if err := run(runParams{
		tenantsFlag:  *tenantsFlag,
		tenantsFile:  *tenantsFile,
		customerFlag: *customerFlag,
		provenFlag:   *provenFlag,
		dryRun:       *dryRun,
		apply:        *apply,
		dbDriver:     *dbDriver,
		dsn:          *dsn,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "saas-migrate:", err)
		os.Exit(1)
	}
}

type runParams struct {
	tenantsFlag  string
	tenantsFile  string
	customerFlag string
	provenFlag   string
	dryRun       bool
	apply        bool
	dbDriver     string
	dsn          string
}

func run(p runParams) error {
	tenantIDs, err := parseTenantList(p.tenantsFlag, p.tenantsFile)
	if err != nil {
		return err
	}
	customerMap, err := parseCustomerMap(p.customerFlag)
	if err != nil {
		return err
	}
	provenOwners, err := parseIDSet(p.provenFlag)
	if err != nil {
		return err
	}

	// Any write requires the explicit --apply flag; the default is dry-run
	// with zero writes. ApplyAllowed is the domain rule the CLI defers to.
	applyAllowed := p.apply && domcom.ApplyAllowed(p.apply)

	var store *commercial.AccountStore
	if p.dsn != "" {
		db, err := openDB(p.dbDriver, p.dsn)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		store = commercial.NewAccountStore(db)
	} else if applyAllowed {
		return fmt.Errorf("-dsn is required for --apply (AccountStore.Bind needs the database)")
	}

	ctx := context.Background()
	reports := make([]tenantReport, 0, len(tenantIDs))
	failures := 0
	for _, id := range tenantIDs {
		report := migrateOne(ctx, store, id, customerMap[id], provenOwners[id], applyAllowed)
		if report.Error != "" {
			failures++
		}
		reports = append(reports, report)
	}

	printReports(reports, applyAllowed)
	if failures > 0 {
		return fmt.Errorf("%d/%d tenant(s) failed; see report above", failures, len(tenantIDs))
	}
	return nil
}

// migrateOne plans one tenant from evidence and, only under an explicit
// apply, binds the mapping through the idempotent F02 AccountStore.Bind.
// A repeated apply re-binds the same mapping and must not create a second
// Customer. Tenants without an existing mapping and without an operator
// provided Customer ID are skipped — credentials and customers are never
// shared or invented.
func migrateOne(ctx context.Context, store *commercial.AccountStore, tenantID uint64, operatorCustomerID string, provenOwner, applyAllowed bool) tenantReport {
	input := domcom.TenantMigrationInput{
		TenantID:              tenantID,
		OperatorCustomerID:    operatorCustomerID,
		ProvenConnectionOwner: provenOwner,
	}
	// Existing-mapping evidence is read-only: reuse, never duplicate.
	if store != nil {
		if acct, err := store.Get(ctx, tenantID); err == nil {
			input.ExistingCustomerID = acct.CustomerID
		}
	}
	plan := domcom.PlanTenantMigration(input)
	report := tenantReport{
		TenantID:               tenantID,
		Plan:                   plan.Decision.Plan,
		IssueCredits:           plan.Decision.IssueCredits,
		ConnectionState:        plan.Decision.ConnectionState,
		ConnectionOwnerProven:  provenOwner,
		CustomerID:             plan.CustomerID,
		ReuseExistingCustomer:  plan.ReuseExistingCustomer,
		BindRequired:           plan.BindRequired,
		SkipReason:             plan.SkipReason,
		LegacyBalanceConverted: plan.LegacyBalanceConverted,
		DataSourcesPreserved:   plan.DataSourcesPreserved,
	}
	if !applyAllowed || !plan.BindRequired {
		return report
	}
	if err := store.Bind(ctx, commercial.Account{TenantID: tenantID, CustomerID: plan.CustomerID}); err != nil {
		report.Error = err.Error()
		return report
	}
	report.Applied = true
	return report
}

func printReports(reports []tenantReport, applied bool) {
	mode := "DRY-RUN (zero writes)"
	if applied {
		mode = "APPLY"
	}
	fmt.Printf("saas-migrate report %s — mode=%s, tenants=%d\n", time.Now().UTC().Format(time.RFC3339), mode, len(reports))
	for _, r := range reports {
		status := "skip"
		switch {
		case r.Error != "":
			status = "FAIL"
		case r.Applied:
			status = "applied"
		case r.BindRequired:
			status = "would-apply"
		}
		line := fmt.Sprintf("tenant=%d status=%s plan=%s issue_credits=%v connection=%s owner_proven=%v customer_id=%q reuse=%v skip_reason=%q legacy_balance_converted=%v data_sources_preserved=%v",
			r.TenantID, status, r.Plan, r.IssueCredits, r.ConnectionState, r.ConnectionOwnerProven, r.CustomerID, r.ReuseExistingCustomer, r.SkipReason, r.LegacyBalanceConverted, r.DataSourcesPreserved)
		if r.Error != "" {
			line += fmt.Sprintf(" error=%q", r.Error)
		}
		fmt.Println(line)
	}
}

func openDB(driver, dsn string) (*gorm.DB, error) {
	switch driver {
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	case "postgres":
		return gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	default:
		return nil, fmt.Errorf("unsupported db driver %q (sqlite or postgres)", driver)
	}
}

func parseTenantList(flagList, file string) ([]uint64, error) {
	// The migration NEVER runs against an implicit fleet: an explicit tenant
	// list (flag or file) is required.
	var raw []string
	if flagList != "" {
		raw = strings.Split(flagList, ",")
	} else if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read tenants file: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				raw = append(raw, line)
			}
		}
	} else {
		return nil, fmt.Errorf("an explicit tenant list is required: -tenants or -tenants-file")
	}
	ids := make([]uint64, 0, len(raw))
	seen := map[uint64]bool{}
	for _, s := range raw {
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("invalid tenant ID %q", s)
		}
		if seen[id] {
			continue // duplicate entries are idempotent
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("tenant list is empty")
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func parseCustomerMap(flagList string) (map[uint64]string, error) {
	out := map[uint64]string{}
	if flagList == "" {
		return out, nil
	}
	for _, pair := range strings.Split(flagList, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid customer mapping %q (want tenantID=customerID)", pair)
		}
		id, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid tenant ID in mapping %q", pair)
		}
		customer := strings.TrimSpace(parts[1])
		if customer == "" {
			return nil, fmt.Errorf("empty customer ID in mapping %q", pair)
		}
		out[id] = customer
	}
	return out, nil
}

func parseIDSet(flagList string) (map[uint64]bool, error) {
	out := map[uint64]bool{}
	if flagList == "" {
		return out, nil
	}
	for _, s := range strings.Split(flagList, ",") {
		id, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid tenant ID %q in proven-owners", s)
		}
		out[id] = true
	}
	return out, nil
}
