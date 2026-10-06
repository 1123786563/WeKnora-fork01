package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
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
	TenantID                                                                           uint64
	RunID, SessionID, OwnerID, ActorUserID, RequestID, AssistantMessageID, RequestHash string
	EngineType, Driver, TargetID, BudgetRef, Status, WaitReason                        string
	Snapshot                                                                           string
	SnapshotDigest                                                                     string
	SnapshotDigestVersion                                                              int
	GraphVersion, SDKVersion                                                           string
	SchemaVersion                                                                      int
	LeaseOwner                                                                         string
	SecurityAgentID, SecurityLocalAgentVersionID, SecurityReleaseID, SecurityPinSource string
	LeaseUntil                                                                         *time.Time
	Epoch, Revision                                                                    int64
	MaxRounds, MaxToolCalls                                                            int
	TokenBudget                                                                        int64
	Deadline, CreatedAt, UpdatedAt                                                     time.Time
}

const craftSnapshotDigestVersion = 1
const craftSnapshotDigestDomain = "craft-admitted-snapshot-v1\x00"

func craftAdmittedSnapshotDigest(raw []byte) (string, error) {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return "", err
	}
	if _, ok := value.(map[string]any); !ok {
		return "", fmt.Errorf("admitted snapshot must be a JSON object")
	}
	value, err = canonicalizeJSONNumbers(value)
	if err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(craftSnapshotDigestDomain))
	_, _ = h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CraftAdmittedSnapshotIdentity returns the version and canonical digest for
// the exact final snapshot bytes persisted by Craft admission. Callers that
// compare a Task identity must still compare this result with the independent
// digest carried on that original Task/Fence.
func CraftAdmittedSnapshotIdentity(raw []byte) (int, string, error) {
	digest, err := craftAdmittedSnapshotDigest(raw)
	if err != nil {
		return 0, "", err
	}
	return craftSnapshotDigestVersion, digest, nil
}

func validCraftAdmittedSnapshot(row agentRunRow) bool {
	if row.SnapshotDigestVersion != craftSnapshotDigestVersion || len(row.SnapshotDigest) != sha256.Size*2 {
		return false
	}
	stored, err := hex.DecodeString(row.SnapshotDigest)
	if err != nil || hex.EncodeToString(stored) != row.SnapshotDigest {
		return false
	}
	computed, err := craftAdmittedSnapshotDigest([]byte(row.Snapshot))
	return err == nil && computed == row.SnapshotDigest
}

func (agentRunRow) TableName() string { return "agent_runs" }

func (r agentRunRow) view() agentruntime.Run {
	run := agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: r.TenantID, RunID: r.RunID},
		SessionID: r.SessionID, UserID: r.OwnerID, ActorUserID: r.ActorUserID, RequestID: r.RequestID,
		AssistantMessageID: r.AssistantMessageID, Driver: r.Driver,
		TargetID: r.TargetID, BudgetRef: r.BudgetRef, Status: r.Status,
		WaitReason: r.WaitReason, Owner: r.LeaseOwner, Revision: r.Revision,
		Epoch: r.Epoch, Deadline: r.Deadline, Snapshot: json.RawMessage(r.Snapshot),
		SnapshotDigestVersion: r.SnapshotDigestVersion, SnapshotDigest: r.SnapshotDigest,
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

func craftSessionExists(tx *gorm.DB, tenantID uint64, sessionID string) (bool, error) {
	var marker int
	err := tx.Table("craft_sessions").Select("1").Where(
		"tenant_id = ? AND session_id = ?", tenantID, sessionID,
	).Limit(1).Scan(&marker).Error
	return marker == 1, err
}

func actorBelongsToTenant(tx *gorm.DB, tenantID uint64, actorUserID string) (bool, error) {
	var marker int
	err := tx.Table("tenant_members AS tm").Select("1").
		Joins("JOIN users AS u ON u.id = tm.user_id").
		Where("tm.tenant_id = ? AND tm.user_id = ? AND tm.status = ? AND tm.deleted_at IS NULL AND u.is_active = ? AND u.deleted_at IS NULL",
			tenantID, actorUserID, "active", true).
		Limit(1).Scan(&marker).Error
	return marker == 1, err
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
		isCraft, err := craftSessionExists(tx, in.Key.TenantID, in.SessionID)
		if err != nil {
			return err
		}
		markedCraft, err := markedCraftAdmissionSnapshot(in.Snapshot)
		if err != nil {
			return err
		}
		if hasSnapshotField(in.Snapshot, "craft_workspace_seed") {
			return fmt.Errorf("%w: Craft Workspace seed is repository-owned admission data", agentruntime.ErrConflict)
		}
		if markedCraft && !isCraft {
			return fmt.Errorf("%w: Craft snapshot is not bound to a registered Craft Task", agentruntime.ErrConflict)
		}
		if isCraft && !markedCraft {
			return fmt.Errorf("%w: registered Craft Task admission requires a Craft input manifest", agentruntime.ErrConflict)
		}
		craftSeedAdmission := isCraft
		if in.ActorUserID == "" {
			if isCraft {
				return agentruntime.ErrConflict
			}
			// Preserve the established generic Run principal where callers have
			// no separate Task actor. Craft admissions never infer this identity.
			in.ActorUserID = in.UserID
		}
		actorExists, err := actorBelongsToTenant(tx, in.Key.TenantID, in.ActorUserID)
		if err != nil {
			return err
		}
		if !actorExists {
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
			if existing.SessionID != in.SessionID || existing.Driver != in.Driver ||
				existing.TargetID != in.TargetID || existing.BudgetRef != in.BudgetRef ||
				existing.AssistantMessageID != in.AssistantMessageID {
				return agentruntime.ErrConflict
			}
			existingActor := existing.ActorUserID
			if existingActor == "" && !isCraft {
				existingActor = existing.OwnerID
			}
			if existingActor != in.ActorUserID {
				return agentruntime.ErrConflict
			}
			if craftSeedAdmission {
				if !sameCraftAdmissionIntent(existing, in.Snapshot) {
					return agentruntime.ErrConflict
				}
				if !validCraftAdmittedSnapshot(existing) {
					return agentruntime.ErrConflict
				}
			} else if existing.RequestHash != in.RequestHash {
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
		// The session row is the shared serialization point with effect claims:
		// BeginEffect must acquire it before inserting an intent. Check all Runs
		// in this session before reserving a replacement writer slot.
		if err := rejectSessionRunViewEffectSlotTransfer(tx, in.Key.TenantID, in.SessionID); err != nil {
			return err
		}
		if in.AgentID == "" && (in.LocalAgentVersionID != "" || in.ReleaseID != "") {
			return ErrAgentSecurityReleaseUnresolvable
		}
		if strings.TrimSpace(in.AgentID) != "" {
			// Serialize against RetireVariant before reading lifecycle state. The
			// no-op UPDATE locks every matching tenant-local variant row on
			// PostgreSQL and acquires SQLite's writer reservation. RetireVariant
			// updates the same row, so one transaction must observe the other's
			// committed state before deciding whether NEW work can be admitted.
			lockAgent := tx.Table("agent_adoption_variants").
				Where("tenant_id = ? AND local_agent_id = ?", in.Key.TenantID, in.AgentID).
				UpdateColumn("state", gorm.Expr("state"))
			if lockAgent.Error != nil {
				return lockAgent.Error
			}
			var retired int64
			if err := tx.Table("agent_adoption_variants").
				Where("tenant_id = ? AND local_agent_id = ? AND state = ?", in.Key.TenantID, in.AgentID, "retired").
				Count(&retired).Error; err != nil {
				return err
			}
			if retired > 0 {
				return agentruntime.ErrAgentUseDenied
			}
		}
		var releaseID string
		var adopted bool
		if in.AgentID != "" {
			var admissionErr error
			releaseID, adopted, admissionErr = checkLocalAgentReleaseAdmissionTx(tx, in.Key.TenantID, in.AgentID, in.LocalAgentVersionID)
			if admissionErr != nil {
				return admissionErr
			}
			if adopted && (in.ReleaseID == "" || releaseID != in.ReleaseID) {
				return ErrAgentSecurityReleaseUnresolvable
			}
			if !adopted && in.ReleaseID != "" {
				return ErrAgentSecurityReleaseUnresolvable
			}
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
		admittedSnapshot := in.Snapshot
		admittedRequestHash := in.RequestHash
		admittedDigestVersion := 0
		admittedDigest := ""
		if craftSeedAdmission {
			// The session row above is the writer fence shared with DraftHeadStore.Advance.
			// Read the head without a row lock: Advance takes the head lock before
			// waiting for this Session fence, so admission must never wait back on
			// the head. If Advance is in flight, it either commits before our
			// Session fence (and this read sees its head), or observes our reserved
			// Run slot and rolls back.
			admittedSnapshot, admittedRequestHash, err = s.craftSeededAdmissionSnapshot(tx, ctx, in)
			if err != nil {
				return err
			}
			admittedDigestVersion = craftSnapshotDigestVersion
			admittedDigest, err = craftAdmittedSnapshotDigest(admittedSnapshot)
			if err != nil {
				return fmt.Errorf("%w: invalid final Craft admission snapshot: %v", agentruntime.ErrConflict, err)
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
			SessionID: in.SessionID, OwnerID: in.UserID, ActorUserID: in.ActorUserID, RequestID: in.RequestID,
			AssistantMessageID: in.AssistantMessageID,
			Driver:             in.Driver, TargetID: in.TargetID, BudgetRef: in.BudgetRef,
			Status: "queued", Snapshot: string(admittedSnapshot), RequestHash: admittedRequestHash,
			SnapshotDigestVersion: admittedDigestVersion, SnapshotDigest: admittedDigest,
			GraphVersion: "1", SchemaVersion: 1, Deadline: in.Deadline,
		}
		if adopted {
			row.SecurityAgentID = in.AgentID
			row.SecurityLocalAgentVersionID = in.LocalAgentVersionID
			row.SecurityReleaseID = releaseID
			row.SecurityPinSource = "admission"
		}
		if in.Driver == "platform" {
			row.EngineType = "trpc"
		}
		// A concurrent request may target a different session: the database
		// unique key is the final arbiter and the slot reservation rolls back.
		createRun := tx.Clauses(clause.OnConflict{DoNothing: true})
		if !adopted {
			// 非 Marketplace Run 的四个 pin 列保持 SQL NULL（空串会触发
			// trg_agent_runs_security_pin_complete 的白名单校验）。
			createRun = createRun.Omit("SecurityAgentID", "SecurityLocalAgentVersionID", "SecurityReleaseID", "SecurityPinSource")
		}
		created := createRun.Create(&row)
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

type craftWorkspaceSeedAdmission struct {
	WorkspaceID    string               `json:"workspace_id"`
	State          craft.DraftHeadState `json:"state"`
	DraftRevision  int64                `json:"draft_revision"`
	SourceRunID    string               `json:"source_run_id,omitempty"`
	ManifestDigest string               `json:"manifest_digest,omitempty"`
}

func markedCraftAdmissionSnapshot(raw json.RawMessage) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false, fmt.Errorf("%w: invalid admission snapshot", agentruntime.ErrConflict)
	}
	manifest, marked := fields["craft_input_manifest"]
	if !marked {
		return false, nil
	}
	if len(manifest) == 0 || bytes.Equal(bytes.TrimSpace(manifest), []byte("null")) {
		return true, fmt.Errorf("%w: null Craft input manifest", agentruntime.ErrConflict)
	}
	var inputs []craft.Input
	if err := json.Unmarshal(manifest, &inputs); err != nil || inputs == nil {
		return true, fmt.Errorf("%w: malformed Craft input manifest", agentruntime.ErrConflict)
	}
	if err := craft.ValidateInputManifest(inputs); err != nil {
		return true, fmt.Errorf("%w: invalid Craft input manifest: %v", agentruntime.ErrConflict, err)
	}
	return true, nil
}

func hasSnapshotField(raw json.RawMessage, name string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	_, ok := fields[name]
	return ok
}

// craftAdmissionIntentEqual compares the canonical caller request while
// excluding only the repository-selected seed. A replay therefore remains
// tied to its original prompt/input/knowledge snapshot without rereading the
// current Workspace head or trusting the caller's RequestHash.
func sameCraftAdmissionIntent(existing agentRunRow, incoming json.RawMessage) bool {
	storedCanonical, err := canonicalCraftAdmissionIntent([]byte(existing.Snapshot))
	if err != nil {
		return false
	}
	incomingCanonical, err := canonicalCraftAdmissionIntent(incoming)
	if err != nil {
		return false
	}
	return bytes.Equal(storedCanonical, incomingCanonical)
}

// canonicalCraftAdmissionIntent compares the complete JSON value while
// ignoring object member order and excluding only the top-level server seed.
// decodeJSONValue rejects duplicate keys and retains number literals as
// json.Number so large integers are never rounded through float64.
func canonicalCraftAdmissionIntent(raw []byte) ([]byte, error) {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return nil, err
	}
	value, err = canonicalizeJSONNumbers(value)
	if err != nil {
		return nil, err
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Craft admission snapshot must be a JSON object")
	}
	delete(fields, "craft_workspace_seed")
	return json.Marshal(fields)
}

func canonicalizeJSONNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		normalized, err := canonicalJSONNumber(string(typed))
		if err != nil {
			return nil, err
		}
		return json.Number(normalized), nil
	case map[string]any:
		for key, child := range typed {
			normalized, err := canonicalizeJSONNumbers(child)
			if err != nil {
				return nil, err
			}
			typed[key] = normalized
		}
		return typed, nil
	case []any:
		for i, child := range typed {
			normalized, err := canonicalizeJSONNumbers(child)
			if err != nil {
				return nil, err
			}
			typed[i] = normalized
		}
		return typed, nil
	default:
		return value, nil
	}
}

// canonicalJSONNumber converts an exact decimal number to a normalized
// scientific form without passing through float64. PostgreSQL JSONB may
// render `1e3` as `1000`; both values normalize to the same representation,
// while adjacent integers above 2^53 remain distinct.
func canonicalJSONNumber(raw string) (string, error) {
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = raw[1:]
	}
	exponent := new(big.Int)
	if at := strings.IndexAny(raw, "eE"); at >= 0 {
		exponentText := raw[at+1:]
		raw = raw[:at]
		exponentText = strings.TrimPrefix(exponentText, "+")
		if _, ok := exponent.SetString(exponentText, 10); !ok {
			return "", fmt.Errorf("invalid JSON number exponent")
		}
	}
	integerPart, fractionPart, hasFraction := strings.Cut(raw, ".")
	if !hasFraction {
		fractionPart = ""
	}
	digits := integerPart + fractionPart
	significant := strings.TrimLeft(digits, "0")
	if significant == "" {
		return "0", nil
	}
	withoutTrailingZeros := strings.TrimRight(significant, "0")
	trailingZeros := len(significant) - len(withoutTrailingZeros)
	significant = withoutTrailingZeros
	scale := new(big.Int).Sub(exponent, big.NewInt(int64(len(fractionPart))))
	scale.Add(scale, big.NewInt(int64(trailingZeros)))
	power := new(big.Int).Add(scale, big.NewInt(int64(len(significant)-1)))
	coefficient := significant[:1]
	if len(significant) > 1 {
		coefficient += "." + significant[1:]
	}
	if negative {
		coefficient = "-" + coefficient
	}
	return coefficient + "e" + power.String(), nil
}

func decodeJSONValue(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeJSONValueToken(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("JSON has trailing values")
	}
	return value, nil
}

func decodeJSONValueToken(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("JSON object contains a non-string key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("JSON object contains duplicate key %q", key)
			}
			value, err := decodeJSONValueToken(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("JSON object is not closed")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeJSONValueToken(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("JSON array is not closed")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

func (s *AgentRunStore) craftSeededAdmissionSnapshot(
	tx *gorm.DB, ctx context.Context, in agentruntime.Admission,
) (json.RawMessage, string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(in.Snapshot, &fields); err != nil || fields == nil {
		return nil, "", fmt.Errorf("%w: invalid Craft admission snapshot", agentruntime.ErrConflict)
	}
	if _, supplied := fields["craft_workspace_seed"]; supplied {
		return nil, "", fmt.Errorf("%w: Craft Workspace seed is repository-owned admission data", agentruntime.ErrConflict)
	}
	scope := craft.Scope{TenantID: in.Key.TenantID, UserID: in.UserID, SessionID: in.SessionID}
	workspace, err := (&CraftStore{db: tx}).GetWorkspace(ctx, scope)
	if err != nil {
		return nil, "", err
	}
	head, err := (&CraftDraftHeadStore{db: tx}).Read(ctx, scope, workspace.ID)
	if err != nil {
		return nil, "", err
	}
	seed := craftWorkspaceSeedAdmission{
		WorkspaceID: head.WorkspaceID, State: head.State, DraftRevision: head.Revision,
		SourceRunID: head.SourceRunID, ManifestDigest: head.ManifestDigest,
	}
	if head.State == craft.DraftHeadSelected {
		var source agentRunRow
		if err := runScope(tx, agentruntime.RunKey{TenantID: in.Key.TenantID, RunID: head.SourceRunID}).Take(&source).Error; err != nil {
			return nil, "", fmt.Errorf("%w: selected Workspace seed source Run is missing", craft.ErrInvalidInput)
		}
		if source.OwnerID != in.UserID || source.SessionID != in.SessionID {
			return nil, "", fmt.Errorf("%w: selected Workspace seed source Run is outside its owner Session", craft.ErrForbidden)
		}
		switch source.Status {
		case "succeeded", "failed", "canceled":
		default:
			return nil, "", fmt.Errorf("%w: selected Workspace seed source Run is not terminal", craft.ErrInvalidInput)
		}
	}
	seedRaw, err := json.Marshal(seed)
	if err != nil {
		return nil, "", err
	}
	fields["craft_workspace_seed"] = seedRaw
	finalSnapshot, err := json.Marshal(fields)
	if err != nil {
		return nil, "", err
	}
	return finalSnapshot, admissionRequestHash(finalSnapshot, in.AssistantMessageID), nil
}

func admissionRequestHash(snapshot json.RawMessage, assistantMessageID string) string {
	preimage := append(append([]byte(nil), snapshot...), []byte(assistantMessageID)...)
	digest := sha256.Sum256(preimage)
	return hex.EncodeToString(digest[:])
}

func persistUsageBinding(raw json.RawMessage, in agentruntime.Admission) (json.RawMessage, error) {
	value, err := decodeJSONValue(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid snapshot", agentruntime.ErrConflict)
	}
	snapshot, ok := value.(map[string]any)
	if value == nil {
		snapshot = make(map[string]any)
	} else if !ok {
		return nil, fmt.Errorf("%w: invalid snapshot object", agentruntime.ErrConflict)
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

// ErrCraftRunViewEffectUnresolved reports that a RunView provider operation
// still needs exact reconciliation. Callers must retain the Run lease and
// session writer slot rather than completing a terminal or replacement
// transition.
var ErrCraftRunViewEffectUnresolved = errors.New("craft run has an unresolved RunView effect")

func rejectUnresolvedCraftRunViewEffects(tx *gorm.DB, key agentruntime.RunKey) error {
	unresolved, err := hasUnresolvedCraftRunViewEffects(tx, key)
	if err != nil {
		return err
	}
	if unresolved {
		return ErrCraftRunViewEffectUnresolved
	}
	return nil
}

// rejectSessionRunViewEffectSlotTransfer serializes admission with effect
// claims through the session row lock. BeginEffect must lock this same session
// before inserting its intent, so a committed intent is visible here and an
// in-flight claim cannot commit until admission releases the row.
func rejectSessionRunViewEffectSlotTransfer(tx *gorm.DB, tenantID uint64, sessionID string) error {
	var count int64
	err := tx.Table("craft_run_view_effect_intents AS e").
		Joins("JOIN agent_runs AS r ON r.tenant_id = e.tenant_id AND r.run_id = e.run_id").
		Where("e.tenant_id = ? AND r.session_id = ? AND e.state IN (?, ?)", tenantID, sessionID,
			craft.RunViewEffectStatePending, craft.RunViewEffectStateUnknown).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrCraftRunViewEffectUnresolved
	}
	return nil
}

func requestCraftChargePause(tx *gorm.DB, key agentruntime.RunKey, reason string) error {
	updated := runScope(tx, key).Where("status IN ('running','recovering','queued')").Updates(map[string]any{
		"status": "reconciling", "wait_reason": craftChargeStartPendingWaitReason,
		"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
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

// transitionCraftBudgetPauseTx applies the journal-aware waiting transition in
// the caller's transaction. A non-nil fence scopes the write to SetStatus's
// active worker; budget callers instead hold and validate the Run row lock.
func (s *AgentRunStore) transitionCraftBudgetPauseTx(tx *gorm.DB, key agentruntime.RunKey, reason string, fence *agentruntime.Fence) error {
	var run agentRunRow
	if err := runScope(tx, key).Take(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agentruntime.ErrNotFound
		}
		return err
	}
	if err := rejectUnresolvedCraftRunViewEffects(tx, key); err != nil {
		return err
	}

	if run.Status == "reconciling" && run.WaitReason == craftChargeStartPendingWaitReason {
		var cancelRequests int64
		if err := tx.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?",
			key.TenantID, key.RunID, "craft_charge_cancel_requested").Count(&cancelRequests).Error; err != nil {
			return err
		}
		if cancelRequests > 0 {
			return agentruntime.ErrConflict
		}
		var pauseRequests int64
		if err := tx.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?",
			key.TenantID, key.RunID, "craft_charge_pause_requested").Count(&pauseRequests).Error; err != nil {
			return err
		}
		if pauseRequests > 0 {
			return nil // replay of an already committed pause request
		}
		return agentruntime.ErrConflict
	}
	if run.Status == "waiting_user" {
		if run.WaitReason == reason {
			return nil
		}
		return agentruntime.ErrConflict
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "recovering" {
		return agentruntime.ErrConflict
	}

	unresolved, err := hasUnresolvedCraftChargeStart(tx, key)
	if err != nil {
		return err
	}
	if unresolved {
		return requestCraftChargePause(tx, key, reason)
	}

	scope := runScope(tx, key)
	if fence != nil {
		scope = s.fenced(tx, *fence)
	}
	updated := scope.Where("status IN ('queued','running','recovering')").Updates(map[string]any{
		"status": "waiting_user", "wait_reason": reason, "lease_owner": "", "lease_until": nil,
		"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		if fence != nil {
			return agentruntime.ErrLeaseLost
		}
		return agentruntime.ErrConflict
	}
	return nil
}

// PauseForCraftBudgetInTx records a budget pause inside the caller's existing
// locked Run transaction. It never opens a transaction or releases the active
// session slot; unresolved charge starts remain reconciling until resolved.
func (s *AgentRunStore) PauseForCraftBudgetInTx(ctx context.Context, tx *gorm.DB, key agentruntime.RunKey, reason string) error {
	if tx == nil || key.TenantID == 0 || key.RunID == "" || reason == "" {
		return agentruntime.ErrConflict
	}
	return s.transitionCraftBudgetPauseTx(tx.WithContext(ctx), key, reason, nil)
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
	if err := rejectUnresolvedCraftRunViewEffects(tx, key); err != nil {
		return false, err
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
		Updates(map[string]any{"status": status, "wait_reason": request.Reason, "lease_owner": "", "lease_until": nil,
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
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
		if err := rejectUnresolvedCraftRunViewEffects(tx, key); err != nil {
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
		isCraft, craftErr := markedCraftAdmissionSnapshot(json.RawMessage(row.Snapshot))
		if craftErr != nil {
			return craftErr
		}
		if row.SnapshotDigestVersion != 0 || row.SnapshotDigest != "" {
			if !validCraftAdmittedSnapshot(row) {
				return agentruntime.ErrConflict
			}
		} else if isCraft {
			// Legacy Craft Runs have no unambiguous admission identity. They may
			// still be inspected, but may not acquire a Craft execution fence.
			return agentruntime.ErrConflict
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
			SnapshotDigestVersion: row.SnapshotDigestVersion, SnapshotDigest: row.SnapshotDigest,
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
		if err := rejectUnresolvedCraftRunViewEffects(tx, fence.RunKey); err != nil {
			return err
		}
		if status == "waiting_user" {
			return s.transitionCraftBudgetPauseTx(tx, fence.RunKey, reason, &fence)
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

// GetRunForGrantedReader reads a run for a task-grant holder: a Viewer or
// Collaborator whose grant row exists for the run's task (= session, ADR-0004)
// AND whose tenant membership is still active. The membership join makes a
// suspended or removed member's read fail closed on the very next request,
// without needing a grant sweep. Owners keep using GetOwnedRun; the owner
// holds no grant row by construction. The complete predicate is one query so
// no unscoped run can leak between checks.
func (s *AgentRunStore) GetRunForGrantedReader(
	ctx context.Context, tenantID uint64, readerID, runID string,
) (agentruntime.Run, error) {
	if tenantID == 0 || readerID == "" || runID == "" {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	var row agentRunRow
	err := s.db.WithContext(ctx).Table("agent_runs").
		Where(`tenant_id = ? AND run_id = ? AND EXISTS (
			SELECT 1 FROM task_grants tg
			WHERE tg.tenant_id = agent_runs.tenant_id
			  AND tg.task_id = agent_runs.session_id
			  AND tg.grantee_id = ?
			  AND EXISTS (
				SELECT 1 FROM tenant_members tm
				WHERE tm.tenant_id = tg.tenant_id
				  AND tm.user_id = tg.grantee_id
				  AND tm.status = 'active'
				  AND tm.deleted_at IS NULL))`, tenantID, runID, readerID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return row.view(), err
}

// RequeueBudgetPausedRuns resumes the runs durably parked at
// waiting_user/budget_exhausted under one task budget — the root run plus
// every attached child run — flipping them back to claimable queued. The
// guarded UPDATE mirrors ApplyDecision's waiting_user→queued transition
// (revision advances; lease fields clear); rows parked for any other wait
// reason, other budget roots, or non-parked states are untouched, and a
// replay affecting already-queued rows is a no-op. T09 (#39): called from
// the authorized budget-extension success path.
func (s *AgentRunStore) RequeueBudgetPausedRuns(ctx context.Context, tenantID uint64, budgetRootRunID string) (int64, error) {
	if tenantID == 0 || budgetRootRunID == "" {
		return 0, agentruntime.ErrConflict
	}
	result := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND status = ? AND wait_reason = ?",
			tenantID, "waiting_user", "budget_exhausted").
		Where("run_id = ? OR run_id IN (SELECT run_id FROM commercial_task_budgets WHERE tenant_id = ? AND root_run_id = ?)",
			budgetRootRunID, tenantID, budgetRootRunID).
		Updates(map[string]any{
			"status": "queued", "wait_reason": "", "lease_owner": "", "lease_until": nil,
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
