package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// CraftRunViewEffectStore persists writer-fenced RunView allocation and
// mutating effect intents. It does not dispatch Docker or OpenCode calls.
type CraftRunViewEffectStore struct {
	db   *gorm.DB
	runs *AgentRunStore
}

var _ craft.RunViewEffectAuthority = (*CraftRunViewEffectStore)(nil)
var _ craft.RunViewEffectRequestAuthority = (*CraftRunViewEffectStore)(nil)

func NewCraftRunViewEffectStore(db *gorm.DB) *CraftRunViewEffectStore {
	return &CraftRunViewEffectStore{db: db, runs: NewAgentRunStore(db)}
}

type craftRunViewEffectIntentRow struct {
	TenantID              uint64
	RunID                 string
	OwnerID               string
	SessionID             string
	Generation            string
	EffectKind            string
	RequestDigest         string
	ClaimToken            string
	ActorUserID           string
	WriterOwner           string
	FenceEpoch            int64
	SnapshotDigestVersion int
	SnapshotDigest        string
	State                 string
	Outcome               string
	Receipt               string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	FinishedAt            *time.Time
}

func (craftRunViewEffectIntentRow) TableName() string { return "craft_run_view_effect_intents" }

func (r craftRunViewEffectIntentRow) claim() craft.RunViewEffectClaim {
	return craft.RunViewEffectClaim{
		Key:           craft.RunViewKey{TenantID: r.TenantID, OwnerID: r.OwnerID, SessionID: r.SessionID, RunID: r.RunID},
		Generation:    r.Generation,
		Kind:          craft.RunViewEffectKind(r.EffectKind),
		RequestDigest: r.RequestDigest,
		Token:         r.ClaimToken,
	}
}

// AllocateAdmitted allocates exactly one RunView and records its completed
// database allocation intent in the same fenced transaction. An old key-only
// RunView allocation is never adopted as proof of this authority.
func (s *CraftRunViewEffectStore) AllocateAdmitted(ctx context.Context, task craft.Task) (craft.RunView, error) {
	if ctx == nil {
		return craft.RunView{}, fmt.Errorf("%w: missing RunView effect context", craft.ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return craft.RunView{}, err
	}
	if err := validateCraftRunViewEffectTask(task); err != nil {
		return craft.RunView{}, err
	}
	generation, err := newCraftRunViewGeneration()
	if err != nil {
		return craft.RunView{}, fmt.Errorf("craft: generate RunView generation: %w", err)
	}
	token, err := newCraftRunViewEffectToken()
	if err != nil {
		return craft.RunView{}, fmt.Errorf("craft: generate RunView effect token: %w", err)
	}
	key := runViewKeyForTask(task)
	var result craft.RunView
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := s.lockAndValidateRun(tx, ctx, task)
		if err != nil {
			return err
		}
		var existing craftRunViewRow
		err = tx.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&existing).Error
		if err == nil {
			view, err := existing.view()
			if err != nil {
				return err
			}
			if err := requireCraftRunViewAllocationIntent(tx, run, task, existing.Generation); err != nil {
				return err
			}
			result = view
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		viewRow := craftRunViewRow{
			TenantID: key.TenantID, RunID: key.RunID, OwnerID: key.OwnerID, SessionID: key.SessionID,
			Generation: generation, State: string(craft.RunViewStateAllocating),
		}
		if err := tx.Create(&viewRow).Error; err != nil {
			return err
		}
		intent := newCraftRunViewEffectIntent(run, task, generation, craft.RunViewEffectAllocate, token)
		intent.State = "finished"
		intent.Outcome = string(craft.RunViewEffectStateSucceeded)
		intent.Receipt = generation
		finished := time.Now().UTC()
		intent.FinishedAt = &finished
		if err := tx.Create(&intent).Error; err != nil {
			return err
		}
		result, err = viewRow.view()
		return err
	})
	return result, err
}

// BeginEffect issues at most one permission for this RunView generation and
// effect kind. A replay returns the same token with maySend=false, including
// after an unknown result; it never grants a second external send.
func (s *CraftRunViewEffectStore) BeginEffect(
	ctx context.Context,
	task craft.Task,
	generation string,
	kind craft.RunViewEffectKind,
) (craft.RunViewEffectClaim, bool, error) {
	return s.beginEffect(ctx, task, generation, kind, "")
}

// BeginEffectWithDigest persists the exact caller-canonicalized request
// identity in the one-shot claim. The store deliberately does not serialize
// provider options: Task2/3 must hash the complete deterministic request and
// pass its lowercase SHA-256 digest here.
func (s *CraftRunViewEffectStore) BeginEffectWithDigest(
	ctx context.Context,
	task craft.Task,
	generation string,
	kind craft.RunViewEffectKind,
	requestDigest string,
) (craft.RunViewEffectClaim, bool, error) {
	if !requiresRunViewEffectRequestDigest(kind) {
		return craft.RunViewEffectClaim{}, false, fmt.Errorf("%w: effect kind does not accept a request digest", craft.ErrInvalidInput)
	}
	return s.beginEffect(ctx, task, generation, kind, requestDigest)
}

func (s *CraftRunViewEffectStore) beginEffect(
	ctx context.Context,
	task craft.Task,
	generation string,
	kind craft.RunViewEffectKind,
	requestDigest string,
) (craft.RunViewEffectClaim, bool, error) {
	if ctx == nil {
		return craft.RunViewEffectClaim{}, false, fmt.Errorf("%w: missing RunView effect context", craft.ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return craft.RunViewEffectClaim{}, false, err
	}
	if err := validateCraftRunViewEffectTask(task); err != nil {
		return craft.RunViewEffectClaim{}, false, err
	}
	if generation == "" || !validCraftRunViewProviderEffect(kind) ||
		(requiresRunViewEffectRequestDigest(kind) && !validRunViewEffectRequestDigest(requestDigest)) ||
		(requestDigest != "" && !validRunViewEffectRequestDigest(requestDigest)) {
		return craft.RunViewEffectClaim{}, false, fmt.Errorf("%w: invalid RunView effect generation or kind", craft.ErrInvalidInput)
	}
	key := runViewKeyForTask(task)
	var claim craft.RunViewEffectClaim
	var maySend bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := s.lockAndValidateRun(tx, ctx, task)
		if err != nil {
			return err
		}
		var view craftRunViewRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&view).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: RunView has not been allocated under the admitted Task", craft.ErrConflict)
			}
			return err
		}
		if view.Generation != generation || view.OwnerID != key.OwnerID || view.SessionID != key.SessionID {
			return fmt.Errorf("%w: RunView generation or scope changed", craft.ErrConflict)
		}
		if err := requireCraftRunViewAllocationIntent(tx, run, task, generation); err != nil {
			return err
		}
		var existing craftRunViewEffectIntentRow
		err = tx.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?",
			key.TenantID, key.RunID, generation, kind).Take(&existing).Error
		if err == nil {
			if !effectIntentMatchesRun(existing, run, task) {
				return fmt.Errorf("%w: replayed RunView effect identity changed", craft.ErrConflict)
			}
			if existing.RequestDigest != requestDigest {
				return fmt.Errorf("%w: replayed RunView effect request changed", craft.ErrConflict)
			}
			claim = existing.claim()
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		token, err := newCraftRunViewEffectToken()
		if err != nil {
			return err
		}
		intent := newCraftRunViewEffectIntent(run, task, generation, kind, token)
		intent.RequestDigest = requestDigest
		if err := tx.Create(&intent).Error; err != nil {
			return err
		}
		claim = intent.claim()
		maySend = true
		return nil
	})
	if err != nil {
		return craft.RunViewEffectClaim{}, false, err
	}
	return claim, maySend, nil
}

// FinishEffect records the outcome against the exact opaque token. Unknown is
// intentionally unresolved; only an identical replay can be accepted here.
func (s *CraftRunViewEffectStore) FinishEffect(
	ctx context.Context,
	claim craft.RunViewEffectClaim,
	outcome craft.RunViewEffectOutcome,
) error {
	if ctx == nil {
		return fmt.Errorf("%w: missing RunView effect context", craft.ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if craft.ValidateRunViewKey(claim.Key) != nil || claim.Generation == "" || claim.Token == "" ||
		!validCraftRunViewProviderEffect(claim.Kind) ||
		(requiresRunViewEffectRequestDigest(claim.Kind) && !validRunViewEffectRequestDigest(claim.RequestDigest)) ||
		(claim.RequestDigest != "" && !validRunViewEffectRequestDigest(claim.RequestDigest)) {
		return fmt.Errorf("%w: incomplete RunView effect claim", craft.ErrInvalidInput)
	}
	if outcome.State != craft.RunViewEffectStateSucceeded && outcome.State != craft.RunViewEffectStateFailed && outcome.State != craft.RunViewEffectStateUnknown {
		return fmt.Errorf("%w: invalid RunView effect outcome", craft.ErrInvalidInput)
	}
	if len(outcome.Receipt) > 2048 || strings.ContainsRune(outcome.Receipt, '\x00') {
		return fmt.Errorf("%w: invalid RunView effect receipt", craft.ErrInvalidInput)
	}
	if outcome.State == craft.RunViewEffectStateSucceeded && strings.TrimSpace(outcome.Receipt) == "" {
		return fmt.Errorf("%w: successful RunView effect requires an observable receipt", craft.ErrInvalidInput)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		key := agentruntime.RunKey{TenantID: claim.Key.TenantID, RunID: claim.Key.RunID}
		if err := lockRunTransitionRow(tx, key); err != nil {
			return err
		}
		var stored craftRunViewEffectIntentRow
		if err := tx.Where("tenant_id = ? AND run_id = ? AND owner_id = ? AND session_id = ? AND generation = ? AND effect_kind = ? AND request_digest = ? AND claim_token = ?",
			claim.Key.TenantID, claim.Key.RunID, claim.Key.OwnerID, claim.Key.SessionID, claim.Generation, claim.Kind, claim.RequestDigest, claim.Token).Take(&stored).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return craft.ErrConflict
			}
			return err
		}
		if stored.State != string(craft.RunViewEffectStatePending) {
			if stored.Outcome == string(outcome.State) && stored.Receipt == outcome.Receipt {
				return nil
			}
			return fmt.Errorf("%w: RunView effect outcome replay changed", craft.ErrConflict)
		}
		updates := map[string]any{
			"state":      string(craft.RunViewEffectStateUnknown),
			"outcome":    string(outcome.State),
			"receipt":    outcome.Receipt,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}
		if outcome.State != craft.RunViewEffectStateUnknown {
			updates["state"] = "finished"
			updates["finished_at"] = gorm.Expr("CURRENT_TIMESTAMP")
		}
		updated := tx.Model(&craftRunViewEffectIntentRow{}).
			Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ? AND claim_token = ? AND state = ?",
				claim.Key.TenantID, claim.Key.RunID, claim.Generation, claim.Kind, claim.Token, craft.RunViewEffectStatePending).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return craft.ErrConflict
		}
		return nil
	})
	return err
}

func requireCraftRunViewAllocationIntent(tx *gorm.DB, run agentRunRow, task craft.Task, generation string) error {
	var intent craftRunViewEffectIntentRow
	err := tx.Where("tenant_id = ? AND run_id = ? AND generation = ? AND effect_kind = ?",
		run.TenantID, run.RunID, generation, craft.RunViewEffectAllocate).Take(&intent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: RunView generation lacks a fenced allocation intent", craft.ErrConflict)
	}
	if err != nil {
		return err
	}
	if !effectIntentMatchesRunIdentity(intent, run, task) || intent.WriterOwner == "" || intent.FenceEpoch <= 0 ||
		intent.State != "finished" || intent.Outcome != string(craft.RunViewEffectStateSucceeded) || intent.Receipt != generation {
		return fmt.Errorf("%w: stored RunView allocation identity is incomplete or inconsistent", craft.ErrConflict)
	}
	return nil
}

func (s *CraftRunViewEffectStore) lockAndValidateRun(tx *gorm.DB, ctx context.Context, task craft.Task) (agentRunRow, error) {
	if err := s.runs.lockToolRun(tx, task.Fence); err != nil {
		return agentRunRow{}, err
	}
	var run agentRunRow
	if err := runScope(tx, task.Fence.RunKey).Take(&run).Error; err != nil {
		return agentRunRow{}, err
	}
	if run.TenantID != task.Scope.TenantID || run.RunID != task.Fence.RunID || run.SessionID != task.Scope.SessionID || run.OwnerID != task.Scope.UserID {
		return agentRunRow{}, craft.ErrForbidden
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != run.TenantID || caller.UserID == "" || caller.UserID != run.ActorUserID {
		return agentRunRow{}, craft.ErrForbidden
	}
	markedCraft, err := markedCraftAdmissionSnapshot(json.RawMessage(run.Snapshot))
	if err != nil || !markedCraft || run.Driver != "platform" {
		return agentRunRow{}, fmt.Errorf("%w: effect claims require an admitted Craft Run", craft.ErrConflict)
	}
	if !validCraftAdmittedSnapshot(run) || task.SnapshotDigestVersion != run.SnapshotDigestVersion || task.SnapshotDigest != run.SnapshotDigest ||
		task.Fence.SnapshotDigestVersion != run.SnapshotDigestVersion || task.Fence.SnapshotDigest != run.SnapshotDigest {
		return agentRunRow{}, fmt.Errorf("%w: Task snapshot identity does not match the admitted Run", craft.ErrConflict)
	}
	var admitted struct {
		CraftWorkspaceSeed *struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"craft_workspace_seed"`
	}
	if err := json.Unmarshal([]byte(run.Snapshot), &admitted); err != nil || admitted.CraftWorkspaceSeed == nil ||
		admitted.CraftWorkspaceSeed.WorkspaceID == "" || admitted.CraftWorkspaceSeed.WorkspaceID != task.WorkspaceID {
		return agentRunRow{}, fmt.Errorf("%w: Task Workspace differs from the admitted Run snapshot", craft.ErrConflict)
	}
	return run, nil
}

func validateCraftRunViewEffectTask(task craft.Task) error {
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" || task.WorkspaceID == "" ||
		task.Fence.TenantID != task.Scope.TenantID || task.Fence.RunID == "" || task.Fence.Owner == "" || task.Fence.Epoch <= 0 ||
		task.SnapshotDigestVersion != craftSnapshotDigestVersion || task.SnapshotDigest == "" ||
		task.Fence.SnapshotDigestVersion != task.SnapshotDigestVersion || task.Fence.SnapshotDigest != task.SnapshotDigest {
		return fmt.Errorf("%w: incomplete or mismatched admitted Task fence", craft.ErrInvalidInput)
	}
	return craft.ValidateRunViewKey(runViewKeyForTask(task))
}

func runViewKeyForTask(task craft.Task) craft.RunViewKey {
	return craft.RunViewKey{TenantID: task.Scope.TenantID, OwnerID: task.Scope.UserID, SessionID: task.Scope.SessionID, RunID: task.Fence.RunID}
}

func newCraftRunViewEffectIntent(run agentRunRow, task craft.Task, generation string, kind craft.RunViewEffectKind, token string) craftRunViewEffectIntentRow {
	return craftRunViewEffectIntentRow{
		TenantID: run.TenantID, RunID: run.RunID, Generation: generation, EffectKind: string(kind), ClaimToken: token,
		OwnerID: task.Scope.UserID, SessionID: task.Scope.SessionID,
		ActorUserID: run.ActorUserID, WriterOwner: task.Fence.Owner, FenceEpoch: task.Fence.Epoch,
		SnapshotDigestVersion: run.SnapshotDigestVersion, SnapshotDigest: run.SnapshotDigest,
		State: string(craft.RunViewEffectStatePending),
	}
}

func effectIntentMatchesRun(intent craftRunViewEffectIntentRow, run agentRunRow, task craft.Task) bool {
	return effectIntentMatchesRunIdentity(intent, run, task) && intent.WriterOwner == task.Fence.Owner && intent.FenceEpoch == task.Fence.Epoch
}

// Allocation is a completed database effect, so replaying it after a valid
// lease recovery may reuse its generation across writer epochs. The current
// caller is still checked against the live Run/fence by lockAndValidateRun;
// only the historical allocation intent comparison omits the old writer.
func effectIntentMatchesRunIdentity(intent craftRunViewEffectIntentRow, run agentRunRow, task craft.Task) bool {
	return intent.TenantID == run.TenantID && intent.RunID == run.RunID && intent.OwnerID == task.Scope.UserID &&
		intent.SessionID == task.Scope.SessionID && intent.ActorUserID == run.ActorUserID &&
		intent.SnapshotDigestVersion == run.SnapshotDigestVersion && intent.SnapshotDigest == run.SnapshotDigest &&
		task.SnapshotDigestVersion == run.SnapshotDigestVersion && task.SnapshotDigest == run.SnapshotDigest
}

func validCraftRunViewProviderEffect(kind craft.RunViewEffectKind) bool {
	switch kind {
	case craft.RunViewEffectDockerNetworkCreate, craft.RunViewEffectDockerCreate, craft.RunViewEffectDockerStart, craft.RunViewEffectDockerProbe, craft.RunViewEffectOpenCodeCreate, craft.RunViewEffectWebBuild:
		return true
	default:
		return false
	}
}

func requiresRunViewEffectRequestDigest(kind craft.RunViewEffectKind) bool {
	return kind == craft.RunViewEffectDockerNetworkCreate || kind == craft.RunViewEffectDockerProbe || kind == craft.RunViewEffectWebBuild
}

func validRunViewEffectRequestDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func newCraftRunViewEffectToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hasUnresolvedCraftRunViewEffects(tx *gorm.DB, key agentruntime.RunKey) (bool, error) {
	var count int64
	err := tx.Table("craft_run_view_effect_intents").Where(
		"tenant_id = ? AND run_id = ? AND state IN (?, ?)",
		key.TenantID, key.RunID, craft.RunViewEffectStatePending, craft.RunViewEffectStateUnknown,
	).Count(&count).Error
	return count != 0, err
}
