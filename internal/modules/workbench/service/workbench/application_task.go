package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const careerApplicationTaskOrigin = "career_application"

// The typed outcomes of the linker contract live in the interfaces package so
// Career can classify definite rejections without importing Workbench. The
// aliases keep the in-package sentinels (and their messages) stable.
var (
	ErrApplicationTaskNotFound  = interfaces.ErrCareerApplicationTaskNotFound
	ErrApplicationTaskConflict  = interfaces.ErrCareerApplicationTaskConflict
	ErrApplicationTaskInvalid   = interfaces.ErrCareerApplicationTaskInvalid
	ErrApplicationTaskUndecided = interfaces.ErrCareerApplicationTaskUndecided
)

var _ interfaces.CareerApplicationTaskLinker = (*ApplicationTaskCoordinator)(nil)

type ApplicationTaskCoordinator struct{ db *gorm.DB }

func NewApplicationTaskCoordinator(db *gorm.DB) *ApplicationTaskCoordinator {
	return &ApplicationTaskCoordinator{db: db}
}

type applicationTaskRow struct {
	TenantID        uint64
	OwnerID         string
	Origin          string
	OriginRequestID string
	ApplicationID   string
	TaskID          string
	RunID           string
	Title           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (applicationTaskRow) TableName() string { return "workbench_application_tasks" }

type applicationTaskSnapshot struct {
	Origin        string `json:"origin"`
	ApplicationID string `json:"application_id"`
	RequestID     string `json:"request_id"`
	Title         string `json:"title"`
}

// applicationTaskMaxAttempts bounds how often EnsureCareerApplicationTask
// retries a creation race. applicationTaskRetryPause spaces the retries far
// enough apart that SQLite lock contention on a busy host clears between
// attempts; the previous 1ms+2ms budget flaked on slow CI.
const (
	applicationTaskMaxAttempts = 3
)

func applicationTaskRetryPause(attempt int) time.Duration {
	return time.Duration(attempt+1) * 10 * time.Millisecond
}

type applicationTaskEnsure func(
	context.Context, uint64, string, interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskLink, error)

func (c *ApplicationTaskCoordinator) EnsureCareerApplicationTask(
	ctx context.Context,
	tenantID uint64,
	ownerID string,
	intent interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskLink, error) {
	intent, err := normalizeApplicationTaskIntent(tenantID, ownerID, intent)
	if err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	}
	return c.ensureWithRetry(ctx, tenantID, ownerID, intent, c.ensureOnce)
}

// ensureWithRetry retries only errors attributable to concurrent request
// creation. When the bounded budget is exhausted while the error is still a
// race, the outcome is NOT a definite conflict: a twin request may have
// committed between the last attempt and now. A recovery Find resolves the
// commits that landed; when nothing durable exists the caller receives the
// typed ErrApplicationTaskUndecided (wrapping the last race evidence) so it
// keeps its recovering state instead of terminally failing a request ID
// whose task may already be ready.
func (c *ApplicationTaskCoordinator) ensureWithRetry(
	ctx context.Context,
	tenantID uint64,
	ownerID string,
	intent interfaces.CareerApplicationTaskIntent,
	ensure applicationTaskEnsure,
) (interfaces.CareerApplicationTaskLink, error) {
	intent, err := normalizeApplicationTaskIntent(tenantID, ownerID, intent)
	if err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	}
	var lastRace error
	for attempt := 0; attempt < applicationTaskMaxAttempts; attempt++ {
		link, err := ensure(ctx, tenantID, ownerID, intent)
		if err == nil {
			return link, nil
		}
		if !isApplicationTaskCreationRace(err) {
			return interfaces.CareerApplicationTaskLink{}, err
		}
		lastRace = err
		if attempt == applicationTaskMaxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return interfaces.CareerApplicationTaskLink{}, ctx.Err()
		case <-time.After(applicationTaskRetryPause(attempt)):
		}
	}
	if row, found, err := findApplicationTask(c.db.WithContext(ctx), tenantID, ownerID, intent.RequestID, false); err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	} else if found {
		// The durable row must still match this intent: a twin that won the
		// same request ID with different content is a typed conflict, never
		// a successful link handed to the losing application.
		return applicationTaskReplay(row, intent)
	}
	return interfaces.CareerApplicationTaskLink{}, fmt.Errorf(
		"%w: creation still racing after %d attempts: %v",
		ErrApplicationTaskUndecided, applicationTaskMaxAttempts, lastRace,
	)
}

func (c *ApplicationTaskCoordinator) ensureOnce(
	ctx context.Context,
	tenantID uint64,
	ownerID string,
	intent interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskLink, error) {
	var link interfaces.CareerApplicationTaskLink
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, found, err := findApplicationTask(tx, tenantID, ownerID, intent.RequestID, true)
		if err != nil {
			return err
		}
		if found {
			link, err = applicationTaskReplay(existing, intent)
			return err
		}

		byApplication, found, err := findApplicationTaskByApplicationID(tx, tenantID, ownerID, intent.ApplicationID)
		if err != nil {
			return err
		}
		if found {
			return fmt.Errorf(
				"%w: application %s is bound to request %s",
				ErrApplicationTaskConflict,
				intent.ApplicationID,
				byApplication.OriginRequestID,
			)
		}

		snapshot, err := json.Marshal(applicationTaskSnapshot{
			Origin:        careerApplicationTaskOrigin,
			ApplicationID: intent.ApplicationID,
			RequestID:     intent.RequestID,
			Title:         intent.Title,
		})
		if err != nil {
			return err
		}
		intentHash := sha256.Sum256(snapshot)
		now := time.Now().UTC()
		taskID := uuid.NewString()
		runID := uuid.NewString()

		session := types.Session{
			ID: taskID, Title: intent.Title, TenantID: tenantID, UserID: ownerID,
			EngineType: "builtin", CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&session).Error; err != nil {
			return err
		}

		run := map[string]any{
			"tenant_id": tenantID, "run_id": runID, "session_id": taskID,
			"owner_id": ownerID, "request_id": intent.RequestID,
			"assistant_message_id": "", "request_hash": hex.EncodeToString(intentHash[:]),
			"engine_type": "trpc", "driver": "platform", "target_id": careerApplicationTaskOrigin,
			"budget_ref": "", "status": "waiting_user",
			"wait_reason": "career_application_linking", "snapshot": string(snapshot),
			"graph_version": "1", "sdk_version": "", "schema_version": 1,
			"lease_owner": "", "epoch": 0, "revision": 1,
			"max_rounds": 0, "max_tool_calls": 0, "token_budget": 0,
			"deadline": now.Add(time.Hour), "created_at": now, "updated_at": now,
		}
		if err := tx.Table("agent_runs").Create(run).Error; err != nil {
			return err
		}

		mapping := applicationTaskRow{
			TenantID: tenantID, OwnerID: ownerID, Origin: careerApplicationTaskOrigin,
			OriginRequestID: intent.RequestID, ApplicationID: intent.ApplicationID,
			TaskID: taskID, RunID: runID, Title: intent.Title,
			CreatedAt: now, UpdatedAt: now,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mapping)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			raced, found, raceErr := findApplicationTask(tx, tenantID, ownerID, intent.RequestID, false)
			if raceErr != nil {
				return raceErr
			}
			if found {
				link, raceErr = applicationTaskReplay(raced, intent)
				return raceErr
			}
			racedApplication, found, raceErr := findApplicationTaskByApplicationID(
				tx, tenantID, ownerID, intent.ApplicationID,
			)
			if raceErr != nil {
				return raceErr
			}
			if found {
				return fmt.Errorf(
					"%w: application %s is bound to request %s",
					ErrApplicationTaskConflict,
					intent.ApplicationID,
					racedApplication.OriginRequestID,
				)
			}
			return ErrApplicationTaskConflict
		}

		link = interfaces.CareerApplicationTaskLink{TaskID: taskID, RunID: runID}
		return nil
	})
	if err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	}
	return link, nil
}

// isApplicationTaskCreationRace accepts exactly the three evidence classes
// the fix-r1 plan allows: gorm.ErrDuplicatedKey, the two request-uniqueness
// constraint names, and SQLite database-lock contention. Generic unique-key
// markers (unique constraint / duplicate key / 23505) would also swallow
// unrelated tables' constraint failures, so they must not match.
func isApplicationTaskCreationRace(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"uq_agent_runs_request",
		"uq_workbench_application_tasks_request",
		"database is locked",
		"database table is locked",
		"sqlite_busy",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (c *ApplicationTaskCoordinator) FindCareerApplicationTask(
	ctx context.Context,
	tenantID uint64,
	ownerID string,
	requestID string,
) (interfaces.CareerApplicationTaskLink, error) {
	tenantID, ownerID, requestID, err := normalizeApplicationTaskScope(tenantID, ownerID, requestID)
	if err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	}
	row, found, err := findApplicationTask(c.db.WithContext(ctx), tenantID, ownerID, requestID, false)
	if err != nil {
		return interfaces.CareerApplicationTaskLink{}, err
	}
	if !found {
		return interfaces.CareerApplicationTaskLink{}, ErrApplicationTaskNotFound
	}
	return interfaces.CareerApplicationTaskLink{TaskID: row.TaskID, RunID: row.RunID, ApplicationID: row.ApplicationID}, nil
}

func normalizeApplicationTaskIntent(
	tenantID uint64, ownerID string, intent interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskIntent, error) {
	tenantID, ownerID, requestID, err := normalizeApplicationTaskScope(tenantID, ownerID, intent.RequestID)
	if err != nil {
		return intent, err
	}
	intent.RequestID = requestID
	intent.ApplicationID = strings.TrimSpace(intent.ApplicationID)
	parsedApplicationID, err := uuid.Parse(intent.ApplicationID)
	if err != nil {
		return intent, fmt.Errorf("%w: application id must be a UUID", ErrApplicationTaskInvalid)
	}
	intent.ApplicationID = parsedApplicationID.String()
	intent.Title = strings.Join(strings.Fields(intent.Title), " ")
	if intent.Title == "" || len(intent.Title) > 255 {
		return intent, fmt.Errorf("%w: title must be 1 to 255 characters", ErrApplicationTaskInvalid)
	}
	return intent, nil
}

// normalizeApplicationTaskScope validates the durable request-ID width. The
// 64-character ceiling matches the physical column width of
// workbench_application_tasks.origin_request_id and agent_runs.request_id;
// Career enforces the same limit up front so a valid Career request can
// never reach this seam only to fail deterministically.
func normalizeApplicationTaskScope(tenantID uint64, ownerID, requestID string) (uint64, string, string, error) {
	ownerID = strings.TrimSpace(ownerID)
	requestID = strings.TrimSpace(requestID)
	if tenantID == 0 || ownerID == "" || len(ownerID) > 512 {
		return 0, "", "", fmt.Errorf("%w: tenant and owner are required", ErrApplicationTaskInvalid)
	}
	if requestID == "" || len(requestID) > 64 {
		return 0, "", "", fmt.Errorf("%w: request id must be 1 to 64 characters", ErrApplicationTaskInvalid)
	}
	return tenantID, ownerID, requestID, nil
}

func findApplicationTask(
	db *gorm.DB, tenantID uint64, ownerID, requestID string, lock bool,
) (applicationTaskRow, bool, error) {
	query := db.Table("workbench_application_tasks").
		Where(
			"tenant_id = ? AND owner_id = ? AND origin = ? AND origin_request_id = ?",
			tenantID, ownerID, careerApplicationTaskOrigin, requestID,
		)
	if lock && db.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row applicationTaskRow
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return applicationTaskRow{}, false, nil
		}
		return applicationTaskRow{}, false, err
	}
	return row, true, nil
}

func findApplicationTaskByApplicationID(
	db *gorm.DB, tenantID uint64, ownerID, applicationID string,
) (applicationTaskRow, bool, error) {
	var row applicationTaskRow
	err := db.Table("workbench_application_tasks").
		Where(
			"tenant_id = ? AND owner_id = ? AND origin = ? AND application_id = ?",
			tenantID, ownerID, careerApplicationTaskOrigin, applicationID,
		).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return applicationTaskRow{}, false, nil
	}
	if err != nil {
		return applicationTaskRow{}, false, err
	}
	return row, true, nil
}

func applicationTaskReplay(
	row applicationTaskRow, intent interfaces.CareerApplicationTaskIntent,
) (interfaces.CareerApplicationTaskLink, error) {
	if row.ApplicationID != intent.ApplicationID || row.Title != intent.Title {
		return interfaces.CareerApplicationTaskLink{}, ErrApplicationTaskConflict
	}
	return interfaces.CareerApplicationTaskLink{TaskID: row.TaskID, RunID: row.RunID}, nil
}
