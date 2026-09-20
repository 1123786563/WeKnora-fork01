package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NativePendingDecisionRepository is the durable authority for a pending
// user decision. It stores only the display-safe detail payload; raw tool
// arguments remain in the immutable tool plan journal.
type NativePendingDecisionRepository struct{ db *gorm.DB }

func NewNativePendingDecisionRepository(db *gorm.DB) *NativePendingDecisionRepository {
	return &NativePendingDecisionRepository{db: db}
}

type nativePendingDecisionRow struct {
	TenantID, Revision, ExpectedRevision                                                                int64
	RunID, PendingID, CallID, ArgsHash, WaitKind, Status, DecisionID, DecisionHash, ResourceRef, Detail string
	PlanVersion                                                                                         int
	ExpiresAt                                                                                           *time.Time
}

type nativePendingRunRow struct {
	OwnerID, SessionID, Status string
	Revision                   int64
}

func nativePendingFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

func validNativePendingKey(key nativecontract.PendingKey) bool {
	return key.Run.TenantID != 0 && key.Run.RunID != "" && key.Run.SessionID != "" && key.PendingID != ""
}

// Create is the worker-side parking operation. Replaying the exact same
// pending record is safe; a changed immutable binding is refused.
func (r *NativePendingDecisionRepository) Create(ctx context.Context, runID nativecontract.RunIdentity, detail nativecontract.PendingDecisionDetail) error {
	if r == nil || r.db == nil {
		return nativePendingFailure(nativecontract.ErrStore, "pending decision store is unavailable")
	}
	if !validNativePendingKey(nativecontract.PendingKey{Run: runID, PendingID: detail.Ref.PendingID}) || runID.SessionID != detail.SessionID || runID.RunID != detail.RunID {
		return nativePendingFailure(nativecontract.ErrInvalid, "pending detail is not bound to a run")
	}
	if err := validateNativePendingDetail(detail); err != nil {
		return err
	}
	runRevision, _ := strconv.ParseInt(detail.RunRevision, 10, 64)
	payload, err := json.Marshal(detail)
	if err != nil {
		return nativePendingFailure(nativecontract.ErrInvalid, "pending detail cannot be encoded")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run nativePendingRunRow
		if err := tx.Table("native_agent_runs").Select("owner_id, session_id, status, revision").
			Where("tenant_id=? AND run_id=? AND session_id=?", runID.TenantID, detail.RunID, detail.SessionID).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nativePendingFailure(nativecontract.ErrNotFound, "pending run was not found")
			}
			return err
		}
		if run.Revision != runRevision || nativecontract.RunStatus(run.Status) != detail.RunStatus {
			return nativePendingFailure(nativecontract.ErrConflict, "pending run changed before parking")
		}
		row := nativePendingDecisionRow{TenantID: int64(runID.TenantID), RunID: detail.RunID, PendingID: detail.Ref.PendingID,
			Revision: 1, CallID: detail.CallID, PlanVersion: detail.PlanVersion, ArgsHash: detail.ArgsHash, WaitKind: string(detail.WaitKind),
			Status: string(detail.Status), ExpiresAt: &detail.ExpiresAt, ExpectedRevision: runRevision, ResourceRef: detail.Service.ResourceRef, Detail: string(payload)}
		created := tx.Table("native_agent_pending_decisions").Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		stored, err := r.load(tx, nativecontract.PendingKey{Run: runID, PendingID: detail.Ref.PendingID})
		if err != nil {
			return err
		}
		if stored.CallID == row.CallID && stored.PlanVersion == row.PlanVersion && stored.ArgsHash == row.ArgsHash && stored.WaitKind == row.WaitKind && stored.ExpectedRevision == row.ExpectedRevision && stored.ResourceRef == row.ResourceRef && stored.Detail == row.Detail {
			return nil
		}
		return nativePendingFailure(nativecontract.ErrConflict, "pending decision already exists")
	})
}

func validateNativePendingDetail(detail nativecontract.PendingDecisionDetail) error {
	if detail.Version < 1 || detail.Ref.PendingID == "" || detail.SessionID == "" || detail.RunID == "" || detail.CallID == "" ||
		detail.PlanVersion < 1 || detail.ArgsHash == "" || detail.WaitKind == "" || detail.Status != nativecontract.PendingOpen ||
		detail.RunStatus != nativecontract.RunWaiting || detail.ExpiresAt.IsZero() || !json.Valid(detail.RedactedArgs) {
		return nativePendingFailure(nativecontract.ErrInvalid, "pending decision detail is incomplete")
	}
	var args map[string]json.RawMessage
	if err := json.Unmarshal(detail.RedactedArgs, &args); err != nil {
		return nativePendingFailure(nativecontract.ErrInvalid, "redacted pending arguments must be an object")
	}
	if _, err := strconv.ParseInt(detail.RunRevision, 10, 64); err != nil || detail.RunRevision == "" {
		return nativePendingFailure(nativecontract.ErrInvalid, "pending run revision is invalid")
	}
	if detail.Ref.Revision != "" && detail.Ref.Revision != "1" {
		return nativePendingFailure(nativecontract.ErrInvalid, "new pending revision must be one")
	}
	return nil
}

func (r *NativePendingDecisionRepository) scopedRun(tx *gorm.DB, scope nativecontract.Scope, run nativecontract.RunIdentity) (nativePendingRunRow, error) {
	if !validNativePendingKey(nativecontract.PendingKey{Run: run, PendingID: "scope"}) || scope.TenantID == 0 || scope.TenantID != run.TenantID || scope.SessionOwnerID == "" {
		return nativePendingRunRow{}, nativePendingFailure(nativecontract.ErrForbidden, "pending scope is incomplete")
	}
	var row nativePendingRunRow
	err := tx.Table("native_agent_runs").Select("owner_id, session_id, status, revision").Where("tenant_id=? AND run_id=?", run.TenantID, run.RunID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nativePendingRunRow{}, nativePendingFailure(nativecontract.ErrNotFound, "pending run was not found")
	}
	if err != nil {
		return nativePendingRunRow{}, err
	}
	if row.OwnerID != scope.SessionOwnerID || row.SessionID != run.SessionID {
		return nativePendingRunRow{}, nativePendingFailure(nativecontract.ErrForbidden, "pending run is outside the current scope")
	}
	return row, nil
}

func (r *NativePendingDecisionRepository) load(tx *gorm.DB, key nativecontract.PendingKey) (nativePendingDecisionRow, error) {
	var row nativePendingDecisionRow
	err := tx.Table("native_agent_pending_decisions").Where("tenant_id=? AND run_id=? AND pending_id=?", key.Run.TenantID, key.Run.RunID, key.PendingID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nativePendingDecisionRow{}, nativePendingFailure(nativecontract.ErrNotFound, "pending decision was not found")
	}
	return row, err
}

func expireNativePending(tx *gorm.DB, key nativecontract.PendingKey, now time.Time) error {
	return tx.Exec(`UPDATE native_agent_pending_decisions SET status=? WHERE tenant_id=? AND run_id=? AND pending_id=? AND status=? AND expires_at <= ?`,
		string(nativecontract.PendingExpired), key.Run.TenantID, key.Run.RunID, key.PendingID, string(nativecontract.PendingOpen), now).Error
}

func nativePendingDetailFromRow(row nativePendingDecisionRow, run nativePendingRunRow) (nativecontract.PendingDecisionDetail, error) {
	var detail nativecontract.PendingDecisionDetail
	if err := json.Unmarshal([]byte(row.Detail), &detail); err != nil {
		return nativecontract.PendingDecisionDetail{}, nativePendingFailure(nativecontract.ErrStore, "pending detail is corrupt")
	}
	detail.Ref.PendingID, detail.Ref.Revision = row.PendingID, strconv.FormatInt(row.Revision, 10)
	detail.CallID, detail.PlanVersion, detail.ArgsHash = row.CallID, row.PlanVersion, row.ArgsHash
	detail.Status, detail.RunStatus, detail.RunRevision = nativecontract.PendingStatus(row.Status), nativecontract.RunStatus(run.Status), strconv.FormatInt(run.Revision, 10)
	return detail, nil
}

func (r *NativePendingDecisionRepository) Get(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error) {
	if r == nil || r.db == nil {
		return nativecontract.PendingDecisionDetail{}, nativePendingFailure(nativecontract.ErrStore, "pending decision store is unavailable")
	}
	var result nativecontract.PendingDecisionDetail
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := r.scopedRun(tx, scope, key.Run)
		if err != nil {
			return err
		}
		if err := expireNativePending(tx, key, time.Now().UTC()); err != nil {
			return err
		}
		row, err := r.load(tx, key)
		if err != nil {
			return err
		}
		result, err = nativePendingDetailFromRow(row, run)
		if err == nil && result.Status == nativecontract.PendingOpen && !result.ExpiresAt.After(time.Now().UTC()) {
			if err = tx.Exec(`UPDATE native_agent_pending_decisions SET status=? WHERE tenant_id=? AND run_id=? AND pending_id=? AND status=?`, string(nativecontract.PendingExpired), key.Run.TenantID, key.Run.RunID, key.PendingID, string(nativecontract.PendingOpen)).Error; err == nil {
				result.Status = nativecontract.PendingExpired
			}
		}
		return err
	})
	return result, err
}

func (r *NativePendingDecisionRepository) List(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity, cursor string, limit int) (nativecontract.PendingDecisionPage, error) {
	if r == nil || r.db == nil {
		return nativecontract.PendingDecisionPage{}, nativePendingFailure(nativecontract.ErrStore, "pending decision store is unavailable")
	}
	if limit < 1 || limit > 100 {
		return nativecontract.PendingDecisionPage{}, nativePendingFailure(nativecontract.ErrInvalid, "pending decision page limit is invalid")
	}
	page := nativecontract.PendingDecisionPage{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		runRow, err := r.scopedRun(tx, scope, run)
		if err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE native_agent_pending_decisions SET status=? WHERE tenant_id=? AND run_id=? AND status=? AND expires_at <= ?`, string(nativecontract.PendingExpired), run.TenantID, run.RunID, string(nativecontract.PendingOpen), time.Now().UTC()).Error; err != nil {
			return err
		}
		var rows []nativePendingDecisionRow
		q := tx.Table("native_agent_pending_decisions").Where("tenant_id=? AND run_id=?", run.TenantID, run.RunID)
		if cursor != "" {
			q = q.Where("pending_id > ?", cursor)
		}
		if err := q.Order("pending_id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > limit {
			page.NextCursor = rows[limit-1].PendingID
			rows = rows[:limit]
		}
		for _, row := range rows {
			detail, err := nativePendingDetailFromRow(row, runRow)
			if err != nil {
				return err
			}
			if detail.Status == nativecontract.PendingOpen && !detail.ExpiresAt.After(time.Now().UTC()) {
				if err := tx.Exec(`UPDATE native_agent_pending_decisions SET status=? WHERE tenant_id=? AND run_id=? AND pending_id=? AND status=?`, string(nativecontract.PendingExpired), run.TenantID, run.RunID, row.PendingID, string(nativecontract.PendingOpen)).Error; err != nil {
					return err
				}
				detail.Status = nativecontract.PendingExpired
			}
			page.Items = append(page.Items, detail)
		}
		return nil
	})
	return page, err
}

func nativePendingDecisionHash(req nativecontract.ResolvePendingRequest) (string, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:]), nil
}

func validateNativePendingResolve(req nativecontract.ResolvePendingRequest, detail nativecontract.PendingDecisionDetail, row nativePendingDecisionRow, run nativePendingRunRow) error {
	if req.DecisionID == "" || req.Reason == "" || req.CallID != row.CallID || req.PlanVersion != row.PlanVersion || req.ArgsHash != row.ArgsHash || req.ResourceRef != row.ResourceRef || req.ExpectedRevision != strconv.FormatInt(run.Revision, 10) || req.PendingRevision != strconv.FormatInt(row.Revision, 10) {
		return nativePendingFailure(nativecontract.ErrConflict, "pending decision binding changed")
	}
	allowed := false
	for _, action := range detail.AllowedActions {
		if req.Action == action {
			allowed = true
			break
		}
	}
	if !allowed {
		return nativePendingFailure(nativecontract.ErrForbidden, "pending decision action is not permitted")
	}
	if req.Action == nativecontract.DecisionProvideResult {
		if len(req.ProvidedResult) == 0 || len(req.ProvidedResult) > 1024*1024 || !json.Valid(req.ProvidedResult) {
			return nativePendingFailure(nativecontract.ErrInvalid, "provided result must be a JSON object smaller than one MiB")
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(req.ProvidedResult, &result); err != nil {
			return nativePendingFailure(nativecontract.ErrInvalid, "provided result must be a JSON object")
		}
	}
	return nil
}

func (r *NativePendingDecisionRepository) Resolve(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, req nativecontract.ResolvePendingRequest) (nativecontract.PendingResolution, error) {
	if r == nil || r.db == nil {
		return nativecontract.PendingResolution{}, nativePendingFailure(nativecontract.ErrStore, "pending decision store is unavailable")
	}
	hash, err := nativePendingDecisionHash(req)
	if err != nil {
		return nativecontract.PendingResolution{}, nativePendingFailure(nativecontract.ErrInvalid, "decision payload cannot be encoded")
	}
	var resolution nativecontract.PendingResolution
	expired := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := r.scopedRun(tx, scope, key.Run)
		if err != nil {
			return err
		}
		if err := expireNativePending(tx, key, time.Now().UTC()); err != nil {
			return err
		}
		row, err := r.load(tx, key)
		if err != nil {
			return err
		}
		detail, err := nativePendingDetailFromRow(row, run)
		if err != nil {
			return err
		}
		if row.Status == string(nativecontract.PendingOpen) && !detail.ExpiresAt.After(time.Now().UTC()) {
			if err := tx.Exec(`UPDATE native_agent_pending_decisions SET status=? WHERE tenant_id=? AND run_id=? AND pending_id=? AND status=?`, string(nativecontract.PendingExpired), key.Run.TenantID, key.Run.RunID, key.PendingID, string(nativecontract.PendingOpen)).Error; err != nil {
				return err
			}
			expired = true
			return nil
		}
		if row.Status == string(nativecontract.PendingResolved) {
			if row.DecisionID != req.DecisionID || row.DecisionHash != hash {
				return nativePendingFailure(nativecontract.ErrConflict, "decision ID was reused with a different payload")
			}
			resolution = nativecontract.PendingResolution{Detail: detail, RunStatus: detail.RunStatus, RunRevision: detail.RunRevision, ResumeState: nativePendingResumeState(detail.ResolvedAction)}
			return nil
		}
		if row.Status != string(nativecontract.PendingOpen) {
			return nativePendingFailure(nativecontract.ErrConflict, "pending decision is no longer open")
		}
		if err := validateNativePendingResolve(req, detail, row, run); err != nil {
			return err
		}
		var sameID nativePendingDecisionRow
		if err := tx.Table("native_agent_pending_decisions").Where("tenant_id=? AND run_id=? AND decision_id=?", key.Run.TenantID, key.Run.RunID, req.DecisionID).Take(&sameID).Error; err == nil {
			return nativePendingFailure(nativecontract.ErrConflict, "decision ID belongs to another pending decision")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		nextStatus, resume := nativePendingNextRun(req.Action)
		detail.Status, detail.ResolvedDecisionID, detail.ResolvedAction = nativecontract.PendingResolved, req.DecisionID, req.Action
		now := time.Now().UTC()
		detail.ResolvedAt = &now
		detail.RunStatus, detail.RunRevision = nextStatus, strconv.FormatInt(run.Revision+1, 10)
		detail.Ref.Revision = strconv.FormatInt(row.Revision+1, 10)
		encoded, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		updated := tx.Exec(`UPDATE native_agent_pending_decisions SET status=?, revision=revision+1, decision_id=?, decision_hash=?, action=?, detail=? WHERE tenant_id=? AND run_id=? AND pending_id=? AND status=? AND revision=?`, string(nativecontract.PendingResolved), req.DecisionID, hash, string(req.Action), string(encoded), key.Run.TenantID, key.Run.RunID, key.PendingID, string(nativecontract.PendingOpen), row.Revision)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return nativePendingFailure(nativecontract.ErrConflict, "pending decision was consumed concurrently")
		}
		updated = tx.Exec(`UPDATE native_agent_runs SET status=?, revision=revision+1, updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND run_id=? AND revision=?`, string(nextStatus), key.Run.TenantID, key.Run.RunID, run.Revision)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return nativePendingFailure(nativecontract.ErrConflict, "run changed while resolving pending decision")
		}
		resolution = nativecontract.PendingResolution{Detail: detail, RunStatus: nextStatus, RunRevision: detail.RunRevision, ResumeState: resume}
		return nil
	})
	if err == nil && expired {
		return nativecontract.PendingResolution{}, nativePendingFailure(nativecontract.ErrConflict, "pending decision expired")
	}
	return resolution, err
}

func nativePendingNextRun(action nativecontract.DecisionAction) (nativecontract.RunStatus, string) {
	if action == nativecontract.DecisionTerminate {
		return nativecontract.RunCancelled, "terminated"
	}
	return nativecontract.RunQueued, "queued"
}
func nativePendingResumeState(action nativecontract.DecisionAction) string {
	_, state := nativePendingNextRun(action)
	return state
}
