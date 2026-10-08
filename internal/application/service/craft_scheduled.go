package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// SP3 (C-23): the scheduled-task application service. The editor's three
// scheduling modes compile into ONE canonical five-field cron expression at
// every write entrance — cron_expression is the stored scheduling truth,
// editor_mode stays only as the UI hint that authored it. CRUD owns the
// dispatcher's claim ticket (next_run_at): an active task always carries its
// next fire, a paused task parks the ticket NULL, and run-now appends a
// manual queued ledger row without touching the ticket. Enqueueing those
// rows is the Task 4 dispatcher's wiring; this layer's contract ends at the
// durable row.

// craftScheduledNameMax mirrors the name column width (VARCHAR(128)) so an
// oversized name is a 400 here rather than a driver-level insert failure on
// PostgreSQL.
const craftScheduledNameMax = 128

// CompileCronSpec compiles one editor payload into the canonical five-field
// cron expression the store keeps (spec §3: the backend compiles; the cron,
// never the payload, is the scheduling truth).
//
//	interval  {"every_minutes": 30}   -> */30 * * * *  (every_minutes >= 1)
//	daily     {"at": "09:30"}          -> 30 9 * * *   (24h HH:MM, H:MM too)
//	advanced  {"cron": "*/5 * * * *"}  -> passthrough, validated
//
// Every malformed shape answers an error wrapping craft.ErrInvalidInput
// (400 semantics); callers classify with errors.Is.
func CompileCronSpec(mode string, payload json.RawMessage) (string, error) {
	switch mode {
	case types.CraftScheduledEditorModeInterval:
		var spec struct {
			EveryMinutes int64 `json:"every_minutes"`
		}
		if err := json.Unmarshal(payload, &spec); err != nil {
			return "", fmt.Errorf("%w: interval payload must be {\"every_minutes\": <minutes>}: %v",
				craft.ErrInvalidInput, err)
		}
		if spec.EveryMinutes < 1 {
			return "", fmt.Errorf("%w: interval every_minutes must be >= 1, got %d",
				craft.ErrInvalidInput, spec.EveryMinutes)
		}
		return compileScheduledCron(fmt.Sprintf("*/%d * * * *", spec.EveryMinutes))
	case types.CraftScheduledEditorModeDaily:
		var spec struct {
			At string `json:"at"`
		}
		if err := json.Unmarshal(payload, &spec); err != nil {
			return "", fmt.Errorf("%w: daily payload must be {\"at\": \"HH:MM\"}: %v",
				craft.ErrInvalidInput, err)
		}
		hour, minute, err := parseDailyAt(spec.At)
		if err != nil {
			return "", err
		}
		return compileScheduledCron(fmt.Sprintf("%d %d * * *", minute, hour))
	case types.CraftScheduledEditorModeAdvanced:
		var spec struct {
			Cron string `json:"cron"`
		}
		if err := json.Unmarshal(payload, &spec); err != nil {
			return "", fmt.Errorf("%w: advanced payload must be {\"cron\": \"...\"}: %v",
				craft.ErrInvalidInput, err)
		}
		return compileScheduledCron(spec.Cron)
	default:
		return "", fmt.Errorf("%w: unknown editor mode %q (interval|daily|advanced)",
			craft.ErrInvalidInput, mode)
	}
}

// compileScheduledCron validates the compiled expression with the strict
// five-field parser (minute-granularity floor: no descriptors, no timezone
// prefixes) and returns its trimmed canonical form.
func compileScheduledCron(expr string) (string, error) {
	trimmed := strings.TrimSpace(expr)
	if err := types.ValidateCronExpression(trimmed); err != nil {
		return "", fmt.Errorf("%w: %v", craft.ErrInvalidInput, err)
	}
	return trimmed, nil
}

// parseDailyAt parses the daily editor's "at" value — a 24h clock time of
// day, "HH:MM" or "H:MM" — into its hour and minute.
func parseDailyAt(at string) (hour, minute int, err error) {
	parts := strings.Split(strings.TrimSpace(at), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: daily at must be \"HH:MM\", got %q", craft.ErrInvalidInput, at)
	}
	hour, herr := strconv.Atoi(parts[0])
	minute, merr := strconv.Atoi(parts[1])
	if herr != nil || merr != nil {
		return 0, 0, fmt.Errorf("%w: daily at must be \"HH:MM\" with numeric fields, got %q",
			craft.ErrInvalidInput, at)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("%w: daily at %q is outside 00:00-23:59", craft.ErrInvalidInput, at)
	}
	return hour, minute, nil
}

// NextRunsAfter returns the first n fires of expr strictly after the given
// instant, in after's location — the API's next-fires preview. n <= 0
// answers nil. An expression that no longer parses (corrupted stored state —
// every write entrance validates) also answers nil: the preview degrades to
// empty rather than failing the whole read.
func NextRunsAfter(expr string, after time.Time, n int) []time.Time {
	if n <= 0 {
		return nil
	}
	fires := make([]time.Time, 0, n)
	from := after
	for len(fires) < n {
		fire, err := types.NextCronFire(expr, from)
		if err != nil {
			return nil
		}
		fires = append(fires, fire)
		from = fire
	}
	return fires
}

// CraftScheduledStore is the persistence port this service consumes: the
// owner-scoped recipe CRUD plus the run-ledger append and page. The GORM
// implementation is repository.CraftScheduledTaskRepository, which also
// carries the due-claim and run-status methods owned by the Task 4/5
// dispatcher and executor services.
type CraftScheduledStore interface {
	Create(ctx context.Context, task *types.CraftScheduledTask) error
	ListByOwner(ctx context.Context, tenantID uint64, ownerID string) ([]types.CraftScheduledTask, error)
	GetByID(ctx context.Context, tenantID uint64, ownerID, id string) (*types.CraftScheduledTask, error)
	Update(ctx context.Context, task *types.CraftScheduledTask) error
	SoftDelete(ctx context.Context, tenantID uint64, ownerID, id string) error
	InsertRun(ctx context.Context, run *types.CraftScheduledTaskRun) error
	// ListRunsByTask pages one task's run history newest-first by started_at
	// (the before cursor is exclusive; the limit is clamped 1..100, default
	// 50). NOT owner-scoped: the Task 4 dispatcher reads it internally; the
	// owner-facing entrance below resolves the scope first.
	ListRunsByTask(ctx context.Context, taskID string, before *time.Time, limit int) ([]types.CraftScheduledTaskRun, error)
}

// CraftScheduledService is the /craft/scheduled-tasks application surface:
// compile-validate-write CRUD over the recipe store, the claim-ticket
// lifecycle and the manual run-now entrance.
type CraftScheduledService struct {
	store CraftScheduledStore
	now   func() time.Time
}

// NewCraftScheduledService assembles the service over its store.
func NewCraftScheduledService(store CraftScheduledStore) (*CraftScheduledService, error) {
	if store == nil {
		return nil, errors.New("craft: scheduled task service requires a store")
	}
	return &CraftScheduledService{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}, nil
}

// WithClock swaps the clock (exported so the HTTP-layer tests in another
// package can pin ticket computation and the fires preview instead of
// sleeping).
func (s *CraftScheduledService) WithClock(now func() time.Time) *CraftScheduledService {
	if now != nil {
		s.now = now
	}
	return s
}

// CraftScheduledTaskCreate carries the POST semantics (spec §3): the
// editor's mode+payload pair compiles to the stored cron, status defaults to
// active, and run_immediately optionally appends one manual queued run.
type CraftScheduledTaskCreate struct {
	Name           string
	Prompt         string
	EditorMode     string
	Payload        json.RawMessage
	Status         string
	RunImmediately bool
}

// CreateScheduledTask validates and stores one recipe, arms its first claim
// ticket and, when asked, appends the manual queued run. The returned run id
// is empty unless run_immediately produced a run. If the run append fails
// the recipe is still stored — the created task returns together with the
// error.
func (s *CraftScheduledService) CreateScheduledTask(
	ctx context.Context, tenantID uint64, ownerID string, in CraftScheduledTaskCreate,
) (*types.CraftScheduledTask, string, error) {
	if tenantID == 0 || strings.TrimSpace(ownerID) == "" {
		return nil, "", fmt.Errorf("%w: scheduled task requires tenant and owner", craft.ErrInvalidInput)
	}
	if err := validateScheduledName(in.Name); err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return nil, "", fmt.Errorf("%w: prompt is required", craft.ErrInvalidInput)
	}
	if !validScheduledEditorMode(in.EditorMode) {
		return nil, "", fmt.Errorf("%w: unknown editor mode %q (interval|daily|advanced)",
			craft.ErrInvalidInput, in.EditorMode)
	}
	status := in.Status
	if status == "" {
		status = types.CraftScheduledTaskStatusActive
	}
	if !validScheduledStatus(status) {
		return nil, "", fmt.Errorf("%w: unknown status %q (active|paused)", craft.ErrInvalidInput, status)
	}
	cron, err := CompileCronSpec(in.EditorMode, in.Payload)
	if err != nil {
		return nil, "", err
	}
	task := &types.CraftScheduledTask{
		TenantID:       tenantID,
		OwnerID:        ownerID,
		Name:           strings.TrimSpace(in.Name),
		Prompt:         strings.TrimSpace(in.Prompt),
		CronExpression: cron,
		EditorMode:     in.EditorMode,
		Status:         status,
	}
	// The claim ticket: an active task is immediately dispatchable, a
	// paused task parks NULL until resumed (spec §2). CompileCronSpec just
	// validated the expression, so a parse failure here is unreachable.
	if status == types.CraftScheduledTaskStatusActive {
		next, ferr := types.NextCronFire(cron, s.now())
		if ferr != nil {
			return nil, "", fmt.Errorf("compute first fire: %w", ferr)
		}
		task.NextRunAt = &next
	}
	if err := s.store.Create(ctx, task); err != nil {
		return nil, "", err
	}
	runID := ""
	if in.RunImmediately {
		id, rerr := s.insertManualRun(ctx, task)
		if rerr != nil {
			return task, "", rerr
		}
		runID = id
	}
	return task, runID, nil
}

// CraftScheduledTaskUpdate carries the PATCH semantics: every field is
// optional (nil = untouched); editor mode and payload MUST arrive as a pair
// because together they recompile the stored cron; status moves the recipe
// between active and paused.
type CraftScheduledTaskUpdate struct {
	Name       *string
	Prompt     *string
	EditorMode *string
	Payload    json.RawMessage
	Status     *string
}

// UpdateScheduledTask merges a partial update into the owner's live recipe
// and recomputes the claim ticket: paused parks next_run_at NULL, active —
// whether untouched, rescheduled or resumed — carries the next fire after
// now (an unchanged schedule recomputes to the same fire).
func (s *CraftScheduledService) UpdateScheduledTask(
	ctx context.Context, tenantID uint64, ownerID, id string, in CraftScheduledTaskUpdate,
) (*types.CraftScheduledTask, error) {
	hasMode := in.EditorMode != nil
	hasPayload := len(in.Payload) > 0
	if hasMode != hasPayload {
		return nil, fmt.Errorf("%w: editor_mode and payload must be updated as a pair", craft.ErrInvalidInput)
	}
	if in.Name != nil {
		if err := validateScheduledName(*in.Name); err != nil {
			return nil, err
		}
	}
	if in.Prompt != nil && strings.TrimSpace(*in.Prompt) == "" {
		return nil, fmt.Errorf("%w: prompt cannot be empty", craft.ErrInvalidInput)
	}
	if in.EditorMode != nil && !validScheduledEditorMode(*in.EditorMode) {
		return nil, fmt.Errorf("%w: unknown editor mode %q (interval|daily|advanced)",
			craft.ErrInvalidInput, *in.EditorMode)
	}
	if in.Status != nil && !validScheduledStatus(*in.Status) {
		return nil, fmt.Errorf("%w: unknown status %q (active|paused)", craft.ErrInvalidInput, *in.Status)
	}

	task, err := s.store.GetByID(ctx, tenantID, ownerID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		task.Name = strings.TrimSpace(*in.Name)
	}
	if in.Prompt != nil {
		task.Prompt = strings.TrimSpace(*in.Prompt)
	}
	if in.EditorMode != nil {
		cron, cerr := CompileCronSpec(*in.EditorMode, in.Payload)
		if cerr != nil {
			return nil, cerr
		}
		task.EditorMode = *in.EditorMode
		task.CronExpression = cron
	}
	if in.Status != nil {
		task.Status = *in.Status
	}
	if task.Status == types.CraftScheduledTaskStatusPaused {
		task.NextRunAt = nil
	} else {
		// The stored expression passed both write entrances, so a parse
		// failure here is corrupted state — surfaced, not muted into a 400.
		next, nerr := types.NextCronFire(task.CronExpression, s.now())
		if nerr != nil {
			return nil, fmt.Errorf("recompute next fire of task %s: %w", task.ID, nerr)
		}
		task.NextRunAt = &next
	}
	if err := s.store.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// CraftScheduledTaskDetail is the GET shape: the recipe plus, while active,
// the next three fires previewed from now (spec §3). Paused recipes preview
// nothing — nothing is scheduled until resumed.
type CraftScheduledTaskDetail struct {
	Task      types.CraftScheduledTask
	NextFires []time.Time
}

// GetScheduledTask returns the owner's recipe with its next-3-fires preview.
func (s *CraftScheduledService) GetScheduledTask(
	ctx context.Context, tenantID uint64, ownerID, id string,
) (*CraftScheduledTaskDetail, error) {
	task, err := s.store.GetByID(ctx, tenantID, ownerID, id)
	if err != nil {
		return nil, err
	}
	detail := &CraftScheduledTaskDetail{Task: *task}
	if task.Status == types.CraftScheduledTaskStatusActive {
		detail.NextFires = NextRunsAfter(task.CronExpression, s.now(), 3)
	}
	return detail, nil
}

// ListScheduledTasks returns the owner's live recipes newest-first (V1
// carries no pagination — the Onyx shape).
func (s *CraftScheduledService) ListScheduledTasks(
	ctx context.Context, tenantID uint64, ownerID string,
) ([]types.CraftScheduledTask, error) {
	return s.store.ListByOwner(ctx, tenantID, ownerID)
}

// DeleteScheduledTask tombstones the owner's recipe. Idempotent: an already
// deleted, missing or foreign recipe is the same success — the desired end
// state holds and nothing is leaked (the API's 204-every-time).
func (s *CraftScheduledService) DeleteScheduledTask(ctx context.Context, tenantID uint64, ownerID, id string) error {
	return s.store.SoftDelete(ctx, tenantID, ownerID, id)
}

// RunScheduledTaskNow manually fires the owner's recipe: one queued run row
// with the manual trigger source, its id returned for the API. A paused
// recipe may be fired manually; the claim ticket is untouched (a manual
// fire is not a scheduled fire). Enqueueing is the Task 4 dispatcher's
// wiring.
func (s *CraftScheduledService) RunScheduledTaskNow(
	ctx context.Context, tenantID uint64, ownerID, id string,
) (string, error) {
	task, err := s.store.GetByID(ctx, tenantID, ownerID, id)
	if err != nil {
		return "", err
	}
	return s.insertManualRun(ctx, task)
}

// ListScheduledTaskRuns pages the owner's run history of one task
// newest-first (spec §3: before = started_at ISO, limit 1..100 default 50).
// The owner scope resolves first: a foreign, deleted or missing task answers
// the same craft.ErrNotFound as every other owner-facing read — the ledger
// rows themselves are only task-keyed.
func (s *CraftScheduledService) ListScheduledTaskRuns(
	ctx context.Context, tenantID uint64, ownerID, taskID string, before *time.Time, limit int,
) ([]types.CraftScheduledTaskRun, error) {
	if _, err := s.store.GetByID(ctx, tenantID, ownerID, taskID); err != nil {
		return nil, err
	}
	return s.store.ListRunsByTask(ctx, taskID, before, limit)
}

// insertManualRun appends the manual queued ledger row of a run-now fire.
func (s *CraftScheduledService) insertManualRun(
	ctx context.Context, task *types.CraftScheduledTask,
) (string, error) {
	run := &types.CraftScheduledTaskRun{
		TaskID:        task.ID,
		Status:        types.CraftScheduledRunStatusQueued,
		TriggerSource: types.CraftScheduledTriggerManualRunNow,
	}
	if err := s.store.InsertRun(ctx, run); err != nil {
		return "", fmt.Errorf("insert manual run for task %s: %w", task.ID, err)
	}
	return run.ID, nil
}

func validScheduledEditorMode(mode string) bool {
	switch mode {
	case types.CraftScheduledEditorModeInterval,
		types.CraftScheduledEditorModeDaily,
		types.CraftScheduledEditorModeAdvanced:
		return true
	}
	return false
}

func validScheduledStatus(status string) bool {
	return status == types.CraftScheduledTaskStatusActive ||
		status == types.CraftScheduledTaskStatusPaused
}

// validateScheduledName enforces the recipe name: present after trimming and
// within the column width, so PostgreSQL never rejects the insert with a
// driver error an API caller cannot address.
func validateScheduledName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: name is required", craft.ErrInvalidInput)
	}
	if len(trimmed) > craftScheduledNameMax {
		return fmt.Errorf("%w: name exceeds %d bytes", craft.ErrInvalidInput, craftScheduledNameMax)
	}
	return nil
}
