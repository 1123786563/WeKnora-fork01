package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentRunStore owns durable tRPC admission, leases and graph checkpoints.
// Every mutation of an executing run must lock its row with a valid fence in
// the same transaction as the mutation. Checking a fence and then writing in
// a separate transaction would let a superseded worker commit stale state.
type AgentRunStore struct {
	db            *gorm.DB
	cleanupSource CleanupObservationSource
	cleanupFiles  CleanupFilePurger
}

var _ agentruntime.RunStore = (*AgentRunStore)(nil)

// NewAgentRunStore constructs a store backed by the migrated business database.
func NewAgentRunStore(db *gorm.DB) *AgentRunStore {
	return &AgentRunStore{db: db, cleanupSource: NewDurableCleanupObservationSource(db)}
}

// DB exposes the already-scoped business DB for container-owned adapters.
func (s *AgentRunStore) DB() *gorm.DB {
	if s == nil {
		return nil
	}
	return s.db
}

type agentRunRow struct {
	TenantID                                                              uint64
	RunID, SessionID, OwnerID, RequestID, AssistantMessageID, RequestHash string
	EngineType, Driver, TargetID, BudgetRef, Status, WaitReason           string
	Snapshot                                                              string
	GraphVersion, SDKVersion                                              string
	SchemaVersion                                                         int
	LeaseOwner                                                            string
	LeaseUntil                                                            *time.Time
	Epoch, Revision                                                       int64
	MaxRounds, MaxToolCalls                                               int
	TokenBudget                                                           int64
	Deadline, CreatedAt, UpdatedAt                                        time.Time
}

func (agentRunRow) TableName() string { return "agent_runs" }

func (r agentRunRow) view() agentruntime.Run {
	run := agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: r.TenantID, RunID: r.RunID},
		SessionID: r.SessionID, UserID: r.OwnerID, RequestID: r.RequestID,
		AssistantMessageID: r.AssistantMessageID, Driver: r.Driver,
		TargetID: r.TargetID, BudgetRef: r.BudgetRef, Status: r.Status,
		WaitReason: r.WaitReason, Owner: r.LeaseOwner, Revision: r.Revision,
		Epoch: r.Epoch, Deadline: r.Deadline, Snapshot: json.RawMessage(r.Snapshot),
	}
	if r.LeaseUntil != nil {
		run.LeaseUntil = *r.LeaseUntil
	}
	return run
}

func normalizeRunDriver(driver string) (string, error) {
	if driver == "" {
		return "platform", nil
	}
	if driver == "platform" || driver == "paseo" {
		return driver, nil
	}
	return "", agentruntime.ErrConflict
}

func runScope(db *gorm.DB, key agentruntime.RunKey) *gorm.DB {
	return db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID)
}

// Get loads a run in the specified tenant.
func (s *AgentRunStore) Get(ctx context.Context, key agentruntime.RunKey) (agentruntime.Run, error) {
	var row agentRunRow
	err := runScope(s.db.WithContext(ctx), key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return row.view(), err
}

// GetOwnedRun reads a run only when the authenticated tenant and actor own it.
// The complete predicate is deliberately issued as one query so no unscoped
// run can leak between owner checks.
func (s *AgentRunStore) GetOwnedRun(
	ctx context.Context, tenantID uint64, ownerID, runID string,
) (agentruntime.Run, error) {
	if tenantID == 0 || ownerID == "" || runID == "" {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	var row agentRunRow
	err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND owner_id = ? AND run_id = ?", tenantID, ownerID, runID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return row.view(), err
}

type runInputAdmissionClaimRow struct {
	DecisionKey string     `gorm:"column:request_id"`
	SessionID   string     `gorm:"column:session_id"`
	RunID       string     `gorm:"column:admission_run_id"`
	Token       string     `gorm:"column:admission_token"`
	State       string     `gorm:"column:admission_state"`
	LeaseUntil  *time.Time `gorm:"column:lease_expires_at"`
}

func (s *AgentRunStore) lockInputAdmissionClaims(tx *gorm.DB, in agentruntime.Admission) ([]runInputAdmissionClaimRow, error) {
	if len(in.InputClaims) == 0 {
		return nil, nil
	}
	if in.Driver != "platform" {
		return nil, agentruntime.ErrConflict
	}
	seen := make(map[string]struct{}, len(in.InputClaims))
	claims := make([]runInputAdmissionClaimRow, 0, len(in.InputClaims))
	for _, selected := range in.InputClaims {
		if selected.DecisionKey == "" || selected.RunID == "" || selected.Token == "" || selected.RunID != in.Key.RunID {
			return nil, agentruntime.ErrConflict
		}
		if _, duplicate := seen[selected.DecisionKey]; duplicate {
			return nil, agentruntime.ErrConflict
		}
		seen[selected.DecisionKey] = struct{}{}
		fence := tx.Table("craft_session_requests").Where(
			"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_run_id = ? AND admission_token = ? AND admission_state IN ('claimed', 'admitted')",
			in.Key.TenantID, in.UserID, in.SessionID, "input_admission", selected.DecisionKey, selected.RunID, selected.Token,
		).UpdateColumn("admission_token", gorm.Expr("admission_token"))
		if fence.Error != nil {
			return nil, fence.Error
		}
		if fence.RowsAffected != 1 {
			return nil, agentruntime.ErrConflict
		}
		var claim runInputAdmissionClaimRow
		if err := tx.Table("craft_session_requests").Select(
			"request_id, session_id, admission_run_id, admission_token, admission_state, lease_expires_at",
		).Where(
			"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
			in.Key.TenantID, in.UserID, in.SessionID, "input_admission", selected.DecisionKey,
		).Take(&claim).Error; err != nil {
			return nil, err
		}
		if claim.RunID != in.Key.RunID || claim.Token != selected.Token || claim.SessionID != in.SessionID ||
			(claim.State != "claimed" && claim.State != "admitted") {
			return nil, agentruntime.ErrConflict
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

func (s *AgentRunStore) inputAdmissionLeaseValid(tx *gorm.DB, in agentruntime.Admission, claim runInputAdmissionClaimRow) (bool, error) {
	leaseColumn := "lease_expires_at"
	if s.db.Name() == "sqlite" {
		leaseColumn = "julianday(lease_expires_at)"
	}
	var marker int
	err := tx.Table("craft_session_requests").Select("1").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_run_id = ? AND admission_token = ? AND admission_state = 'claimed' AND "+leaseColumn+" > "+s.nowSQL(),
		in.Key.TenantID, in.UserID, in.SessionID, "input_admission", claim.DecisionKey, claim.RunID, claim.Token,
	).Limit(1).Scan(&marker).Error
	return marker == 1, err
}

func (s *AgentRunStore) markInputAdmissionClaimsAdmitted(tx *gorm.DB, in agentruntime.Admission, claims []runInputAdmissionClaimRow) error {
	for _, claim := range claims {
		if claim.State == "admitted" {
			continue
		}
		updated := tx.Table("craft_session_requests").Where(
			"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_run_id = ? AND admission_token = ? AND admission_state = 'claimed'",
			in.Key.TenantID, in.UserID, in.SessionID, "input_admission", claim.DecisionKey, claim.RunID, claim.Token,
		).Update("admission_state", "admitted")
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
	}
	return nil
}

// rejectUnfencedCraftAdmission prevents an ordinary submission path from
// bypassing a live or legacy Craft input claim by omitting the optional typed
// claim bundle. Non-Craft Runs and idempotent replays are unaffected.
func (s *AgentRunStore) rejectUnfencedCraftAdmission(tx *gorm.DB, in agentruntime.Admission) error {
	var craftSession int
	if err := tx.Table("craft_sessions").Select("1").Where(
		"tenant_id = ? AND session_id = ?", in.Key.TenantID, in.SessionID,
	).Limit(1).Scan(&craftSession).Error; err != nil {
		return err
	}
	if craftSession == 0 {
		return nil
	}
	var pending []runInputAdmissionClaimRow
	if err := tx.Table("craft_session_requests").Select(
		"request_id, session_id, admission_run_id, admission_token, admission_state, lease_expires_at",
	).Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND admission_state <> ?",
		in.Key.TenantID, in.UserID, in.SessionID, "input_admission", "admitted",
	).Find(&pending).Error; err != nil {
		return err
	}
	provided := make(map[string]agentruntime.InputAdmissionClaim, len(in.InputClaims))
	for _, claim := range in.InputClaims {
		provided[claim.DecisionKey] = claim
	}
	for _, claim := range pending {
		selected, ok := provided[claim.DecisionKey]
		if !ok || selected.RunID != claim.RunID || selected.Token != claim.Token || claim.SessionID != in.SessionID {
			return agentruntime.ErrConflict
		}
	}
	return nil
}

// Admit atomically reserves a session, creates both business messages and
// persists the immutable request snapshot. Request retries are scoped to the
// authenticated tenant and owner; session validation precedes idempotency reads.
func (s *AgentRunStore) Admit(ctx context.Context, in agentruntime.Admission) (agentruntime.Run, error) {
	if in.Key.TenantID == 0 || in.Key.RunID == "" || in.SessionID == "" || in.UserID == "" ||
		in.RequestID == "" || in.AssistantMessageID == "" || in.RequestHash == "" || in.Deadline.IsZero() ||
		!json.Valid(in.Snapshot) {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	driver, err := normalizeRunDriver(in.Driver)
	if err != nil {
		return agentruntime.Run{}, err
	}
	in.Driver = driver
	// Merge the server-owned usage binding into the immutable snapshot at the
	// repository boundary. Provider or client payloads can still be stored as
	// ordinary remote observations, but they cannot replace these fields.
	in.Snapshot, err = persistUsageBinding(in.Snapshot, in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	user, err := admissionMessage(in.UserMessage, "user", in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	assistant, err := admissionMessage(in.AssistantMessage, "assistant", in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	assistant.ID = in.AssistantMessageID
	if in.UserMessageID != "" {
		// Reuse the handler-persisted user row: exactly one user message per
		// request regardless of which side wrote it first.
		user.ID = in.UserMessageID
	}
	var result agentruntime.Run
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The write locks this session before any reads. This also avoids a
		// deferred SQLite read transaction trying to upgrade to a write lock.
		lock := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND user_id = ? AND deleted_at IS NULL",
			in.Key.TenantID, in.SessionID, in.UserID).
			UpdateColumn("active_agent_run_id", gorm.Expr("active_agent_run_id"))
		if lock.Error != nil {
			return lock.Error
		}
		if lock.RowsAffected != 1 {
			return agentruntime.ErrNotFound
		}
		var session struct {
			EngineType       string
			ActiveAgentRunID *string
		}
		if e := tx.Table("sessions").Where("tenant_id = ? AND id = ?", in.Key.TenantID, in.SessionID).
			Take(&session).Error; e != nil {
			return e
		}
		if in.Driver == "platform" && session.EngineType != "trpc" {
			return agentruntime.ErrConflict
		}
		inputClaims, claimErr := s.lockInputAdmissionClaims(tx, in)
		if claimErr != nil {
			return claimErr
		}
		var existing agentRunRow
		e := tx.Where("tenant_id = ? AND owner_id = ? AND request_id = ?",
			in.Key.TenantID, in.UserID, in.RequestID).Take(&existing).Error
		if e == nil {
			if existing.RequestHash != in.RequestHash || existing.SessionID != in.SessionID ||
				existing.Driver != in.Driver || existing.TargetID != in.TargetID || existing.BudgetRef != in.BudgetRef {
				return agentruntime.ErrConflict
			}
			if len(inputClaims) != 0 {
				if existing.RunID != in.Key.RunID {
					return agentruntime.ErrConflict
				}
				if err := s.markInputAdmissionClaimsAdmitted(tx, in, inputClaims); err != nil {
					return err
				}
			}
			result = existing.view()
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if err := s.rejectUnfencedCraftAdmission(tx, in); err != nil {
			return err
		}
		for _, claim := range inputClaims {
			if claim.State != "claimed" {
				return agentruntime.ErrConflict
			}
			valid, err := s.inputAdmissionLeaseValid(tx, in, claim)
			if err != nil {
				return err
			}
			if !valid {
				return agentruntime.ErrConflict
			}
		}
		slot := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id IS NULL",
			in.Key.TenantID, in.SessionID).UpdateColumn("active_agent_run_id", in.Key.RunID)
		if slot.Error != nil {
			return slot.Error
		}
		if slot.RowsAffected != 1 {
			return agentruntime.ErrRunActive
		}
		row := agentRunRow{
			TenantID: in.Key.TenantID, RunID: in.Key.RunID,
			SessionID: in.SessionID, OwnerID: in.UserID, RequestID: in.RequestID,
			AssistantMessageID: in.AssistantMessageID, RequestHash: in.RequestHash,
			Driver: in.Driver, TargetID: in.TargetID, BudgetRef: in.BudgetRef,
			Status: "queued", Snapshot: string(in.Snapshot),
			GraphVersion: "1", SchemaVersion: 1, Deadline: in.Deadline,
		}
		if in.Driver == "platform" {
			row.EngineType = "trpc"
		}
		// A concurrent request may target a different session: the database
		// unique key is the final arbiter and the slot reservation rolls back.
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		// BeforeCreate unconditionally generates an ID. It was already applied
		// during normalization; skip it here to preserve the admitted IDs.
		// The HTTP handler persists the assistant placeholder (and on retries
		// the user message) before admission runs, so both creates are
		// idempotent by id: an existing row is reused, never a conflict.
		// Finalization owns the assistant row content by id regardless.
		if e := tx.Session(&gorm.Session{SkipHooks: true}).
			Clauses(clause.OnConflict{DoNothing: true}).Create(&user).Error; e != nil {
			return e
		}
		if e := tx.Session(&gorm.Session{SkipHooks: true}).
			Clauses(clause.OnConflict{DoNothing: true}).Create(&assistant).Error; e != nil {
			return e
		}
		if err := s.markInputAdmissionClaimsAdmitted(tx, in, inputClaims); err != nil {
			return err
		}
		result = row.view()
		return nil
	})
	return result, err
}

func persistUsageBinding(raw json.RawMessage, in agentruntime.Admission) (json.RawMessage, error) {
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("%w: invalid snapshot", agentruntime.ErrConflict)
	}
	if snapshot == nil {
		snapshot = make(map[string]any)
	}
	// Remove any client-supplied copies first, then write only the values
	// carried by this trusted admission object. Empty values are intentionally
	// absent so Fence construction applies its fail-closed defaults.
	for _, key := range []string{"parent_run_id", "credential_version", "usage_source", "usage_funding", "usage_service", "price_version", "usage_upper", "usage_revision", "usage_status", "usage_dimensions"} {
		delete(snapshot, key)
	}
	if in.ParentRunID != "" {
		snapshot["parent_run_id"] = in.ParentRunID
	}
	if in.UsageCredentialVersion > 0 {
		snapshot["credential_version"] = in.UsageCredentialVersion
	}
	if in.UsageSource != "" {
		snapshot["usage_source"] = in.UsageSource
	}
	if in.UsageFunding != "" {
		snapshot["usage_funding"] = in.UsageFunding
	}
	if in.UsageService != "" {
		snapshot["usage_service"] = in.UsageService
	}
	if in.UsagePriceVersion != "" {
		snapshot["price_version"] = in.UsagePriceVersion
	}
	if in.UsageUpper > 0 {
		snapshot["usage_upper"] = in.UsageUpper
	}
	if in.UsageRevision > 0 {
		snapshot["usage_revision"] = in.UsageRevision
	}
	if in.UsageStatus != "" {
		snapshot["usage_status"] = in.UsageStatus
	}
	if in.UsageDimensions != nil {
		snapshot["usage_dimensions"] = in.UsageDimensions
	}
	out, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: usage binding", agentruntime.ErrConflict)
	}
	return out, nil
}

func admissionMessage(raw json.RawMessage, role string, in agentruntime.Admission) (types.Message, error) {
	var message types.Message
	if len(raw) == 0 || json.Unmarshal(raw, &message) != nil || message.Role != role {
		return message, fmt.Errorf("%w: invalid %s message", agentruntime.ErrConflict, role)
	}
	// Identity and lifecycle fields come only from admission, never from JSON.
	if err := message.BeforeCreate(nil); err != nil {
		return message, err
	}
	message.SessionID, message.RequestID = in.SessionID, in.RequestID
	message.CreatedAt, message.UpdatedAt = time.Time{}, time.Time{}
	message.DeletedAt = gorm.DeletedAt{}
	message.IsCompleted = role == "user"
	return message, nil
}

// SQLite stores dates as text; julianday accepts both driver timestamps and
// SQLite's UTC strftime format, without relying on lexical timezone ordering.
func (s *AgentRunStore) nowSQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday('now')"
	}
	return "clock_timestamp()"
}

func (s *AgentRunStore) leaseColumnSQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday(lease_until)"
	}
	return "lease_until"
}

func (s *AgentRunStore) leaseExpiry(ttl time.Duration) clause.Expr {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("strftime('%Y-%m-%d %H:%M:%f', 'now', ?)", fmt.Sprintf("+%.3f seconds", ttl.Seconds()))
	}
	return gorm.Expr("clock_timestamp() + (? * interval '1 second')", ttl.Seconds())
}

func (s *AgentRunStore) claimableSQL() string {
	return "(((status = 'queued' OR (status IN ('running', 'recovering') AND (lease_until IS NULL OR " +
		s.leaseColumnSQL() + " <= " + s.nowSQL() + "))) AND NOT EXISTS (SELECT 1 FROM craft_charge_start_journal j " +
		"WHERE j.tenant_id = agent_runs.tenant_id AND j.run_id = agent_runs.run_id AND j.state IN ('intent','unknown'))) " +
		"OR (status = 'reconciling' AND wait_reason = 'craft_charge_start_pending' AND NOT EXISTS " +
		"(SELECT 1 FROM craft_charge_start_journal j WHERE j.tenant_id = agent_runs.tenant_id AND j.run_id = agent_runs.run_id AND j.state IN ('intent','unknown'))))"
}

const craftChargeStartPendingWaitReason = "craft_charge_start_pending"

func hasUnresolvedCraftChargeStart(tx *gorm.DB, key agentruntime.RunKey) (bool, error) {
	var count int64
	err := tx.Table("craft_charge_start_journal").Where(
		"tenant_id = ? AND run_id = ? AND state IN ('intent','unknown')", key.TenantID, key.RunID,
	).Count(&count).Error
	return count != 0, err
}

func lockRunTransitionRow(tx *gorm.DB, key agentruntime.RunKey) error {
	locked := runScope(tx, key).UpdateColumn("revision", gorm.Expr("revision"))
	if locked.Error != nil {
		return locked.Error
	}
	if locked.RowsAffected != 1 {
		return agentruntime.ErrNotFound
	}
	return nil
}

func requestCraftChargePause(tx *gorm.DB, key agentruntime.RunKey, reason string) error {
	updated := runScope(tx, key).Where("status IN ('running','recovering','queued')").Updates(map[string]any{
		"status": "reconciling", "wait_reason": craftChargeStartPendingWaitReason,
		"lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return agentruntime.ErrConflict
	}
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	return appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "craft_charge_pause_requested", string(payload))
}

func requestCraftChargeCancel(tx *gorm.DB, key agentruntime.RunKey, reason string) error {
	updated := runScope(tx, key).Where("status IN ('running','recovering','queued','waiting_user') OR (status = 'reconciling' AND wait_reason = ?)", craftChargeStartPendingWaitReason).Updates(map[string]any{
		"status": "reconciling", "wait_reason": craftChargeStartPendingWaitReason,
		"lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return agentruntime.ErrConflict
	}
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	return appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "craft_charge_cancel_requested", string(payload))
}

// resolveCraftChargePendingTransition finalizes a pause/cancel request only
// after every intent/unknown start for the Run has resolved.
func resolveCraftChargePendingTransition(tx *gorm.DB, key agentruntime.RunKey) (bool, error) {
	var run agentRunRow
	if err := runScope(tx, key).Take(&run).Error; err != nil {
		return false, err
	}
	if run.Status != "reconciling" || run.WaitReason != craftChargeStartPendingWaitReason {
		return false, nil
	}
	unresolved, err := hasUnresolvedCraftChargeStart(tx, key)
	if err != nil || unresolved {
		return false, err
	}
	var event agentRunEventRow
	if err := tx.Where("tenant_id = ? AND run_id = ? AND event_type IN ?", key.TenantID, key.RunID,
		[]string{"craft_charge_pause_requested", "craft_charge_cancel_requested"}).
		Order("CASE WHEN event_type = 'craft_charge_cancel_requested' THEN 0 ELSE 1 END ASC").
		Order("seq DESC").Take(&event).Error; err != nil {
		return false, err
	}
	var request struct{ Reason string }
	if err := json.Unmarshal([]byte(event.Payload), &request); err != nil {
		return false, err
	}
	status := "waiting_user"
	if event.EventType == "craft_charge_cancel_requested" {
		status = "canceled"
	}
	updated := runScope(tx, key).Where("status = 'reconciling' AND wait_reason = ?", craftChargeStartPendingWaitReason).
		Updates(map[string]any{"status": status, "wait_reason": request.Reason, "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if updated.Error != nil {
		return false, updated.Error
	}
	if updated.RowsAffected != 1 {
		return false, agentruntime.ErrConflict
	}
	if status == "canceled" {
		if err := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id = ?", key.TenantID, run.SessionID, key.RunID).
			Update("active_agent_run_id", nil).Error; err != nil {
			return false, err
		}
		if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "cancellation_confirmed", event.Payload); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Claim takes a queued or expired run using database time and increments its epoch.
func (s *AgentRunStore) Claim(
	ctx context.Context, key agentruntime.RunKey, owner string, ttl time.Duration,
) (agentruntime.Fence, error) {
	return s.ClaimDriver(ctx, key, "platform", owner, ttl)
}

// ClaimDriver takes a queued or expired run only when its persisted driver
// matches the worker's driver. The legacy Claim method remains platform-only.
func (s *AgentRunStore) ClaimDriver(
	ctx context.Context, key agentruntime.RunKey, driver, owner string, ttl time.Duration,
) (agentruntime.Fence, error) {
	driver, err := normalizeRunDriver(driver)
	if err != nil {
		return agentruntime.Fence{}, err
	}
	if owner == "" || ttl < time.Millisecond {
		return agentruntime.Fence{}, agentruntime.ErrConflict
	}
	var fence agentruntime.Fence
	transitionFinalized := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRunTransitionRow(tx, key); err != nil {
			return agentruntime.ErrLeaseLost
		}
		var current agentRunRow
		if err := runScope(tx, key).Take(&current).Error; err != nil {
			return err
		}
		if current.Status == "reconciling" && current.WaitReason == craftChargeStartPendingWaitReason {
			finalized, err := resolveCraftChargePendingTransition(tx, key)
			if err != nil {
				return err
			}
			if finalized {
				transitionFinalized = true
				return nil
			}
			return agentruntime.ErrLeaseLost
		}
		unresolved, err := hasUnresolvedCraftChargeStart(tx, key)
		if err != nil {
			return err
		}
		if unresolved {
			return agentruntime.ErrLeaseLost
		}
		claimed := runScope(tx, key).Where("driver = ?", driver).Where(s.claimableSQL()).Updates(map[string]any{
			"lease_owner": owner, "lease_until": s.leaseExpiry(ttl), "epoch": gorm.Expr("epoch + 1"),
			"revision": gorm.Expr("revision + 1"), "status": "running", "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		var row agentRunRow
		if e := runScope(tx, key).Take(&row).Error; e != nil {
			return e
		}
		var snapshot struct {
			Prompt            string           `json:"prompt"`
			Text              string           `json:"text"`
			WorkspaceRef      string           `json:"workspaceRef"`
			Provider          string           `json:"provider"`
			ParentRunID       string           `json:"parent_run_id"`
			UsageSource       string           `json:"usage_source"`
			CredentialVersion int64            `json:"credential_version"`
			UsageFunding      string           `json:"usage_funding"`
			UsageService      string           `json:"usage_service"`
			PriceVersion      string           `json:"price_version"`
			UsageUpper        int64            `json:"usage_upper"`
			UsageRevision     int64            `json:"usage_revision"`
			UsageStatus       string           `json:"usage_status"`
			UsageDimensions   map[string]int64 `json:"usage_dimensions"`
		}
		_ = json.Unmarshal([]byte(row.Snapshot), &snapshot)
		prompt := snapshot.Prompt
		if prompt == "" {
			prompt = snapshot.Text
		}
		workspace := snapshot.WorkspaceRef
		if workspace == "" {
			workspace = row.TargetID
		}
		provider := snapshot.Provider
		if provider == "" {
			provider = driver
		}
		fence = agentruntime.Fence{
			RunKey: key, Owner: owner, Epoch: row.Epoch, TargetID: row.TargetID, WorkspaceRef: workspace, Prompt: prompt, Provider: provider, //nolint:lll // 预存长行,import 修复入 range
			ParentRunID: snapshot.ParentRunID, UsageCredentialVersion: snapshot.CredentialVersion, UsageSource: snapshot.UsageSource, UsageFunding: snapshot.UsageFunding,
			UsageService: snapshot.UsageService, UsagePriceVersion: snapshot.PriceVersion, UsageUpper: snapshot.UsageUpper,
			UsageRevision: snapshot.UsageRevision, UsageStatus: snapshot.UsageStatus, UsageDimensions: snapshot.UsageDimensions, //nolint:lll // 预存长行,import 修复入 range
		}
		return nil
	})
	if err == nil && transitionFinalized {
		return agentruntime.Fence{}, agentruntime.ErrLeaseLost
	}
	return fence, err
}

func (s *AgentRunStore) fenced(tx *gorm.DB, fence agentruntime.Fence) *gorm.DB {
	return runScope(tx, fence.RunKey).Where(
		"lease_owner = ? AND epoch = ? AND status IN ('running', 'recovering') AND "+
			s.leaseColumnSQL()+" > "+s.nowSQL(), fence.Owner, fence.Epoch)
}

// Renew extends only a still-valid lease owned by the supplied fence.
func (s *AgentRunStore) Renew(ctx context.Context, fence agentruntime.Fence, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return agentruntime.ErrConflict
	}
	result := s.fenced(s.db.WithContext(ctx), fence).Updates(map[string]any{
		"lease_until": s.leaseExpiry(ttl), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return agentruntime.ErrLeaseLost
	}
	return nil
}

// Scan lists claimable work, including expired runs needing recovery.
func (s *AgentRunStore) Scan(ctx context.Context, limit int) ([]agentruntime.RunKey, error) {
	return s.ScanDriver(ctx, "platform", limit)
}

// ScanDriver lists claimable work for one persisted execution driver.
func (s *AgentRunStore) ScanDriver(
	ctx context.Context, driver string, limit int,
) ([]agentruntime.RunKey, error) {
	driver, err := normalizeRunDriver(driver)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, agentruntime.ErrConflict
	}
	var keys []agentruntime.RunKey
	err = s.db.WithContext(ctx).Table("agent_runs").Select("tenant_id, run_id").
		Where("driver = ?", driver).Where(s.claimableSQL()).
		Order("created_at ASC, tenant_id ASC, run_id ASC").Limit(limit).Scan(&keys).Error
	return keys, err
}

type agentCheckpointRow struct {
	TenantID                                 uint64
	RunID, Namespace, CheckpointID, ParentID string
	Seq                                      int64
	State, PendingWrites                     string
	CreatedAt, UpdatedAt                     time.Time
}

func (agentCheckpointRow) TableName() string { return "agent_run_checkpoints" }

// SaveCheckpoint saves graph state and pending writes under a transactional fence.
func (s *AgentRunStore) SaveCheckpoint(
	ctx context.Context, fence agentruntime.Fence, cp agentruntime.CheckpointRecord,
) error {
	if cp.Namespace == "" || cp.ID == "" || cp.Seq < 0 || !json.Valid(cp.State) || !json.Valid(cp.PendingWrites) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := s.fenced(tx, fence).Updates(map[string]any{
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		row := agentCheckpointRow{
			TenantID: fence.TenantID, RunID: fence.RunID,
			Namespace: cp.Namespace, CheckpointID: cp.ID, ParentID: cp.ParentID, Seq: cp.Seq,
			State: string(cp.State), PendingWrites: string(cp.PendingWrites),
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "run_id"}, {Name: "namespace"}, {Name: "checkpoint_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"parent_id", "seq", "state", "pending_writes", "updated_at"}),
		}).Create(&row).Error
	})
}

// SetStatus durably records execution outcome under the current fence.
// A terminal failure releases the session's active-run slot in the same
// transaction: only non-terminal runs (including waiting_user) may hold it,
// otherwise one failed run would wedge the session's future admissions.
func (s *AgentRunStore) SetStatus(ctx context.Context, fence agentruntime.Fence, status, reason string) error {
	if status != "succeeded" && status != "failed" && status != "waiting_user" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := s.fenced(tx, fence).UpdateColumn("revision", gorm.Expr("revision"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		unresolved, err := hasUnresolvedCraftChargeStart(tx, fence.RunKey)
		if err != nil {
			return err
		}
		if status == "waiting_user" && unresolved {
			return requestCraftChargePause(tx, fence.RunKey, reason)
		}
		result := s.fenced(tx, fence).Updates(map[string]any{
			"status": status, "wait_reason": reason, "lease_owner": "", "lease_until": nil,
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		if status == "failed" {
			var row agentRunRow
			if err := runScope(tx, fence.RunKey).Take(&row).Error; err != nil {
				return err
			}
			return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?",
				fence.TenantID, row.SessionID, fence.RunID).Update("active_agent_run_id", nil).Error
		}
		return nil
	})
}

// LoadCheckpoint returns the latest committed graph snapshot for one tenant/run.
func (s *AgentRunStore) LoadCheckpoint(
	ctx context.Context, key agentruntime.RunKey,
) (agentruntime.CheckpointRecord, error) {
	var row agentCheckpointRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Order("seq DESC, namespace ASC, checkpoint_id DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
	}
	return agentruntime.CheckpointRecord{
		Namespace: row.Namespace, ID: row.CheckpointID,
		ParentID: row.ParentID, Seq: row.Seq,
		State: json.RawMessage(row.State), PendingWrites: json.RawMessage(row.PendingWrites),
	}, err
}
