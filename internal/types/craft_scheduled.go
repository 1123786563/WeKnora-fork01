package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// SP3 (C-23): Craft scheduled tasks — user-authored "cron + prompt" recipes
// that fire a craft session run on the 30s dispatcher. These are the durable
// row shapes behind migrations 000178 (PostgreSQL) / 000099 (SQLite); the
// scheduling semantics live in the spec's dispatch section, the cron
// expression is the single source of truth (the three editor modes compile
// into it at save time) and every fire leaves one append-only run row.

// CraftScheduledTaskStatus values: an active task is dispatchable (its
// next_run_at is the dispatcher's claim ticket), a paused task holds
// next_run_at NULL until resumed.
const (
	CraftScheduledTaskStatusActive = "active"
	CraftScheduledTaskStatusPaused = "paused"
)

// CraftScheduledRunStatus values — the six states one fire's run row moves
// through. queued/running are the in-flight pair (SKIP_IF_RUNNING overlap
// policy + the stuck sweeper's two windows key on them); succeeded, failed,
// skipped and awaiting_interaction are terminal.
const (
	CraftScheduledRunStatusQueued              = "queued"
	CraftScheduledRunStatusRunning             = "running"
	CraftScheduledRunStatusSucceeded           = "succeeded"
	CraftScheduledRunStatusFailed              = "failed"
	CraftScheduledRunStatusSkipped             = "skipped"
	CraftScheduledRunStatusAwaitingInteraction = "awaiting_interaction"
)

// CraftScheduledTriggerSource values: who fired this run — the dispatcher
// or the manual run-now entrance (which does not advance next_run_at).
const (
	CraftScheduledTriggerScheduled    = "scheduled"
	CraftScheduledTriggerManualRunNow = "manual_run_now"
)

// CraftScheduledSkipReason values: why a due fire was skipped instead of
// dispatched. A skipped fire still leaves a visible run row.
const (
	CraftScheduledSkipOwnerCraftDisabled = "owner_craft_disabled"
	CraftScheduledSkipPriorInFlight      = "prior_in_flight"
)

// CraftScheduledEditorMode values: which of the three UI form modes
// authored the task. A UI hint only — cron_expression is the truth.
const (
	CraftScheduledEditorModeInterval = "interval"
	CraftScheduledEditorModeDaily    = "daily"
	CraftScheduledEditorModeAdvanced = "advanced"
)

// CraftScheduledTask is one user-owned recipe row. Soft-deleted rows keep
// their run history readable; every owner-scoped read filters
// owner_id + deleted_at at the repository layer so a wrong-owner or deleted
// task is an indistinguishable not-found.
type CraftScheduledTask struct {
	ID             string         `json:"id" gorm:"column:id"`
	TenantID       uint64         `json:"tenant_id" gorm:"column:tenant_id"`
	OwnerID        string         `json:"owner_id" gorm:"column:owner_id"`
	Name           string         `json:"name" gorm:"column:name"`
	Prompt         string         `json:"prompt" gorm:"column:prompt"`
	CronExpression string         `json:"cron_expression" gorm:"column:cron_expression"`
	EditorMode     string         `json:"editor_mode" gorm:"column:editor_mode"`
	Status         string         `json:"status" gorm:"column:status"`
	NextRunAt      *time.Time     `json:"next_run_at,omitempty" gorm:"column:next_run_at"`
	LastRunAt      *time.Time     `json:"last_run_at,omitempty" gorm:"column:last_run_at"`
	CreatedAt      time.Time      `json:"created_at" gorm:"column:created_at"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"column:deleted_at"`
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (CraftScheduledTask) TableName() string { return "craft_scheduled_tasks" }

// CraftScheduledTaskRun is one fire's append-only ledger row — dispatched,
// manually triggered or skipped, one row per fire. Nullable columns are
// pointers: session_id is a logical link (the session row may be gone),
// skip/error fields exist only on their terminal shapes, and started_at is
// the fire time the row was inserted at (the run-history keyset anchor).
type CraftScheduledTaskRun struct {
	ID            string     `json:"id" gorm:"column:id"`
	TaskID        string     `json:"task_id" gorm:"column:task_id"`
	SessionID     *string    `json:"session_id,omitempty" gorm:"column:session_id"`
	Status        string     `json:"status" gorm:"column:status"`
	TriggerSource string     `json:"trigger_source" gorm:"column:trigger_source"`
	SkipReason    *string    `json:"skip_reason,omitempty" gorm:"column:skip_reason"`
	ErrorClass    *string    `json:"error_class,omitempty" gorm:"column:error_class"`
	ErrorDetail   *string    `json:"error_detail,omitempty" gorm:"column:error_detail"`
	StartedAt     *time.Time `json:"started_at,omitempty" gorm:"column:started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
	Summary       *string    `json:"summary,omitempty" gorm:"column:summary"`
	CreatedAt     time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (CraftScheduledTaskRun) TableName() string { return "craft_scheduled_task_runs" }

// craftCronParser accepts EXACTLY the canonical five-field cron expression
// (minute hour day-of-month month day-of-week). Descriptors stay off on
// purpose: robfig's @every would admit sub-minute periods (e.g. @every 30s)
// while the 30s dispatcher contract supports only minute-granularity
// schedules — the strict parser enforces that floor by construction.
var craftCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// ValidateCronExpression reports whether expr is a canonical five-field cron
// expression the dispatcher can schedule (spec §3: minute-level minimum
// granularity, the Onyx contract), INCLUDING the never-fires probe: a
// calendar-impossible pattern (Feb 30, Feb 31, the 31st of a 30-day month)
// parses as legal cron but holds no occurrence in any year — robfig's
// five-year search then answers a zero time, which would store a year-0001
// next_run_at ticket (permanently due; the claim CAS would rewrite the same
// zero ticket every sweep — a runaway fire loop). Callers validate at every
// write entrance so only fireable expressions can reach the durable claim
// path. TZ/CRON_TZ prefixes are rejected too: robfig would happily accept
// them, but the stored expression is the canonical form and next_run_at is
// always UTC — a per-expression timezone is a product decision, not a parse
// accident.
func ValidateCronExpression(expr string) error {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return fmt.Errorf("cron expression is empty")
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "TZ=") || strings.HasPrefix(upper, "CRON_TZ=") {
		return fmt.Errorf("invalid cron expression %q: timezone prefixes are not accepted (next_run_at is UTC)", trimmed)
	}
	schedule, err := craftCronParser.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", trimmed, err)
	}
	// Never-fires probe (SP3 Task 2 review Important-1): one Next() answer
	// separates "rare" (Feb 29 fires every leap year) from "never" (Feb 30
	// exists in no year). The probe time is irrelevant to the impossible
	// class; it only means patterns rarer than the five-year search window
	// are rejected too — an acceptable floor for a recurring-task product.
	if next := schedule.Next(time.Now().UTC()); next.IsZero() {
		return fmt.Errorf("invalid cron expression %q: schedule never fires (no occurrence in the calendar)", trimmed)
	}
	return nil
}

// NextCronFire returns the first fire of expr strictly after from, in from's
// location. The dispatcher uses it to advance next_run_at when claiming a
// due row; the API uses it for the next-fires preview. An invalid expression
// — unparseable, or parseable but with no occurrence after from (robfig
// signals the exhausted five-year search with a zero time) — is an error:
// returning the zero time would mint a year-0001 ticket or preview fire,
// and every write entrance validated first, so reaching here with garbage
// means stored state was corrupted.
func NextCronFire(expr string, from time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(expr)
	schedule, err := craftCronParser.Parse(trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", trimmed, err)
	}
	next := schedule.Next(from)
	// Zero-time guard (Important-1): a zero Next is robfig's "no occurrence
	// within five years", not a fireable instant. Error out — the claim
	// sweep's quarantine catches it and the preview degrades to empty.
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression %q never fires after %s", trimmed, from.Format(time.RFC3339))
	}
	return next, nil
}
