// Package plan owns the multi-action Action Plan seam (T21 #51): an
// ORDERED set of already-prepared external actions approved as ONE
// decision under ONE plan digest (CONTEXT.md 操作计划). Any content,
// connection, target or set change forms a NEW plan with a NEW digest,
// so an old approval can never authorize new content (AC1); exclusion is
// an approval-time decision; per-item results stay on the authoritative
// app_actions rows and the plan only projects them. Partial-success
// recovery re-dispatches ONLY authorized (approved, unsent) items — a
// confirmed outcome is never repeated (AC2).
package plan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// Sentinel errors surfaced by the plan service.
var (
	// ErrPlanInvalidInput: malformed form/approve input (zero items,
	// unknown or duplicated exclusion seqs, empty digest/actor).
	ErrPlanInvalidInput = errors.New("plan_invalid_input")
	// ErrPlanDigestMismatch: the presented digest was issued for
	// DIFFERENT plan content — the AC1 refusal. An approval (or an
	// execute request) bound to one plan can never authorize another.
	ErrPlanDigestMismatch = errors.New("plan_digest_mismatch")
	// ErrPlanState: invalid plan lifecycle transition (executing an
	// unapproved plan, an exclusion-set rewrite after approval, a lost
	// approval CAS).
	ErrPlanState = errors.New("plan_state_conflict")
)

// Plan lifecycle states (persisted on app_action_plans.state).
const (
	PlanStateAwaitingApproval = "awaiting_approval"
	PlanStateAuthorized       = "authorized"
)

// Per-item dispositions of one plan execution pass.
const (
	ItemExecuted          = "executed"           // dispatched through the publish pipeline this pass
	ItemSkippedSucceeded  = "skipped_succeeded"  // AC2: already succeeded — never re-dispatched
	ItemSkippedUnapproved = "skipped_unapproved" // plan authorized but this item's approval absent
	ItemSkippedInFlight   = "skipped_in_flight"  // queued/dispatched — a live writer owns it
	ItemSettled           = "settled"            // failed/unknown — a confirmed outcome resume must not redo
	ItemExcluded          = "excluded"           // excluded at approval time — never dispatched
)

// planDigestVersion is the plan digest layout generation.
const planDigestVersion = 1

// DigestItem is one plan item's identity as bound by PlanDigest.
type DigestItem struct {
	Seq          int    `json:"seq"`
	ActionID     string `json:"action_id"`
	ActionDigest string `json:"action_digest"`
	Connection   string `json:"connection"`
	Target       string `json:"target"`
	Risk         string `json:"risk"`
}

// PlanDigest hashes the plan's full approval material as ONE structured
// JSON document (the ActionDigest layout discipline): the layout
// version, the tenant and actor identity, and the ORDERED items with
// their action ids, action digests, connections, targets and risks.
// Changing any item's content (→ a NEW action digest), connection,
// target, risk, the order or the set changes the digest — an approval
// for one exact plan can never authorize another (AC1).
func PlanDigest(tenantID uint64, actorID string, items []DigestItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("%w: a plan binds at least one item", ErrPlanInvalidInput)
	}
	material := struct {
		Version int          `json:"version"`
		Tenant  uint64       `json:"tenant"`
		Actor   string       `json:"actor"`
		Items   []DigestItem `json:"items"`
	}{planDigestVersion, tenantID, actorID, items}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// ItemInput is one plan item's formation input — exactly the
// single-action publish plan input (publish.PublishPlanInput minus the
// tenant/actor the plan carries once for all items).
type ItemInput struct {
	ConnectionID      string
	SessionID         string
	ArtifactVersionID string
	Title             string
	ParentPageID      string // exactly one of parent/page per item
	PageID            string
}

// FormInput forms one multi-action plan.
type FormInput struct {
	TenantID uint64
	ActorID  string
	Items    []ItemInput
}

// ItemView is one item's formation view.
type ItemView struct {
	Seq                     int    `json:"seq"`
	ActionID                string `json:"action_id"`
	Digest                  string `json:"digest"`
	Mode                    string `json:"mode"`
	Destination             string `json:"destination"`
	ExpectedExternalVersion string `json:"expected_external_version"`
	Title                   string `json:"title"`
}

// PlanView is the formed plan as returned to the caller: the plan digest
// an approval binds, plus the ordered per-item views. In Status
// projections the items carry identity only (Seq/ActionID) — the details
// live in the per-item outcomes.
type PlanView struct {
	ID     string     `json:"id"`
	State  string     `json:"state"`
	Digest string     `json:"digest"`
	Items  []ItemView `json:"items"`
}

// ApproveInput is the owner's whole-plan decision: the digest shown at
// approval time and the optional excluded seqs (排除单项). Excluding item
// N approves the plan WITHOUT it: the excluded action stays
// awaiting_approval forever and is never dispatched.
type ApproveInput struct {
	Digest      string
	ExcludeSeqs []int
}

// ItemOutcome is one item's result of one execution pass.
type ItemOutcome struct {
	Seq         int                        `json:"seq"`
	ActionID    string                     `json:"action_id"`
	Disposition string                     `json:"disposition"`
	ActionState string                     `json:"action_state"`
	Conflict    bool                       `json:"conflict"`
	Publication publish.PublishReceiptView `json:"publication"`
}

// ExecuteOutcome is one ordered execution pass over the plan.
type ExecuteOutcome struct {
	PlanID string        `json:"plan_id"`
	Digest string        `json:"digest"`
	Items  []ItemOutcome `json:"items"`
}

// PlanStatus is the durable projection: plan + per-item authoritative
// states + receipts (计划/逐项结果查询).
type PlanStatus struct {
	PlanView
	Excluded []int         `json:"excluded"`
	Outcomes []ItemOutcome `json:"outcomes"`
}

// PlanApprover is the per-item approval face of the A03 ActionService.
type PlanApprover interface {
	Approve(ctx context.Context, id, actor, digest string) error
}

// Service drives the persisted plan lifecycle on top of the #48 publish
// seam. Per-item actions stay the A03 authority (digests, approvals,
// dispatch claims); the plan layer adds the set digest (AC1), exclusions
// and ordered partial-success execution (AC2).
type Service struct {
	plans    *repoappconn.PlanStore
	actions  appconnectorsvc.ActionStoreSource
	approver PlanApprover
	publish  *publish.NotionPublishService
}

// NewService builds the plan service.
func NewService(plans *repoappconn.PlanStore, actions appconnectorsvc.ActionStoreSource,
	approver PlanApprover, pubs *publish.NotionPublishService) *Service {
	return &Service{plans: plans, actions: actions, approver: approver, publish: pubs}
}

// FormPlan forms one multi-action plan: every item goes through the #48
// single-action formation (server-derived snapshot, external baseline
// read, A03 Prepare, planned publication row); the plan digest then
// binds the ORDERED action identities read back from the AUTHORITATIVE
// rows (never the formation echo). A mid-formation failure leaves
// already-prepared actions orphaned in awaiting_approval (never
// approved, never dispatched, approval TTL expiry) and creates NO plan
// row — fail closed, mirror of the single-action failure mode.
func (s *Service) FormPlan(ctx context.Context, in FormInput) (PlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || len(in.Items) == 0 {
		return PlanView{}, fmt.Errorf("%w: tenant, actor and at least one item are required", ErrPlanInvalidInput)
	}
	items := make([]DigestItem, 0, len(in.Items))
	views := make([]ItemView, 0, len(in.Items))
	rows := make([]repoappconn.ActionPlanItemRow, 0, len(in.Items))
	for i, item := range in.Items {
		pv, err := s.publish.FormPlan(ctx, publish.PublishPlanInput{
			TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: item.ConnectionID,
			SessionID: item.SessionID, ArtifactVersionID: item.ArtifactVersionID,
			Title: item.Title, ParentPageID: item.ParentPageID, PageID: item.PageID,
		})
		if err != nil {
			return PlanView{}, err
		}
		row, err := s.actions.FindAction(ctx, pv.ActionID)
		if err != nil {
			return PlanView{}, err
		}
		items = append(items, DigestItem{
			Seq: i + 1, ActionID: row.ID, ActionDigest: row.ArgsDigest,
			Connection: row.ConnectionID, Target: row.Target, Risk: row.Risk,
		})
		views = append(views, ItemView{
			Seq: i + 1, ActionID: pv.ActionID, Digest: pv.Digest, Mode: pv.Mode,
			Destination: pv.Destination, ExpectedExternalVersion: pv.ExpectedExternalVersion,
			Title: pv.Title,
		})
		rows = append(rows, repoappconn.ActionPlanItemRow{TenantID: in.TenantID, Seq: i + 1, ActionID: row.ID})
	}
	digest, err := PlanDigest(in.TenantID, in.ActorID, items)
	if err != nil {
		return PlanView{}, err
	}
	planID := "plan_" + uuid.NewString()
	for i := range rows {
		rows[i].PlanID = planID
	}
	if err := s.plans.CreatePlan(ctx, repoappconn.ActionPlanRow{
		ID: planID, TenantID: in.TenantID, ActorID: in.ActorID,
		Digest: digest, State: PlanStateAwaitingApproval,
	}, rows); err != nil {
		return PlanView{}, err
	}
	return PlanView{ID: planID, State: PlanStateAwaitingApproval, Digest: digest, Items: views}, nil
}

// Approve records the owner's whole-plan decision: the digest must match
// the plan's CURRENT content (AC1 — a digest issued for different content
// is refused, never migrated), the exclusion set is recorded on the plan
// row, and every INCLUDED still-awaiting item receives its own
// digest-bound A03 approval. Excluded items are never approved and never
// dispatched. Re-approval is the recovery path: the exclusion set is
// frozen at first approval (a different set after approval is a state
// conflict), already-authorized items are idempotent no-ops, and items
// still awaiting approval (a prior partial approve) are approved now.
func (s *Service) Approve(ctx context.Context, tenantID uint64, planID, actor string, in ApproveInput) (PlanView, error) {
	if actor == "" || in.Digest == "" {
		return PlanView{}, fmt.Errorf("%w: actor and digest are required", ErrPlanInvalidInput)
	}
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	if row.Digest != in.Digest {
		return PlanView{}, fmt.Errorf("%w: approval digest does not match plan content %s", ErrPlanDigestMismatch, planID)
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	excluded, err := normalizeExclusions(in.ExcludeSeqs, len(items))
	if err != nil {
		return PlanView{}, err
	}
	if row.State == PlanStateAuthorized {
		// Recovery re-approval: the exclusion set is frozen at first
		// approval — a different set after approval is a state conflict,
		// never a silent rewrite.
		recorded, perr := parseExclusions(row.ExcludedJSON)
		if perr != nil {
			return PlanView{}, perr
		}
		if !equalSeqs(recorded, excluded) {
			return PlanView{}, fmt.Errorf("%w: plan already approved with exclusions %v", ErrPlanState, recorded)
		}
	}
	excludedJSON, err := json.Marshal(excluded)
	if err != nil {
		return PlanView{}, err
	}
	if err := s.plans.ApprovePlan(ctx, tenantID, planID, in.Digest, actor, string(excludedJSON), time.Now().UTC()); err != nil {
		return PlanView{}, err
	}
	exSet := map[int]bool{}
	for _, seq := range excluded {
		exSet[seq] = true
	}
	for _, item := range items {
		if exSet[item.Seq] {
			continue // 排除单项：永不批准、永不派发
		}
		action, aerr := s.actions.FindAction(ctx, item.ActionID)
		if aerr != nil {
			return PlanView{}, aerr
		}
		if action.State != appconn.ActionAwaitingApproval {
			// authorized: already approved (idempotent); queued/dispatched/
			// terminal: settled — an approval is no longer applicable.
			continue
		}
		if aerr := s.approver.Approve(ctx, item.ActionID, actor, action.ArgsDigest); aerr != nil {
			return PlanView{}, aerr
		}
	}
	return s.view(ctx, tenantID, planID)
}

// Execute runs one ordered pass over an approved plan. IMPLEMENT IN TASK 4.
// Status returns the durable plan projection. IMPLEMENT IN TASK 4.

// normalizeExclusions validates the caller's exclusion seqs: within
// 1..n, no duplicates; nil means approve everything. The result is
// sorted so the stored/compared form is canonical.
func normalizeExclusions(seqs []int, n int) ([]int, error) {
	if len(seqs) == 0 {
		return []int{}, nil
	}
	seen := map[int]bool{}
	out := make([]int, 0, len(seqs))
	for _, seq := range seqs {
		if seq < 1 || seq > n {
			return nil, fmt.Errorf("%w: exclude_seq %d outside plan items 1..%d", ErrPlanInvalidInput, seq, n)
		}
		if seen[seq] {
			return nil, fmt.Errorf("%w: duplicate exclude_seq %d", ErrPlanInvalidInput, seq)
		}
		seen[seq] = true
		out = append(out, seq)
	}
	sort.Ints(out)
	return out, nil
}

// parseExclusions reads the plan row's frozen exclusion record.
func parseExclusions(raw string) ([]int, error) {
	if raw == "" {
		return []int{}, nil
	}
	var out []int
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%w: corrupt exclusion record: %v", ErrPlanState, err)
	}
	return out, nil
}

func equalSeqs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// view rebuilds the plan identity view from the store.
func (s *Service) view(ctx context.Context, tenantID uint64, planID string) (PlanView, error) {
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	views := make([]ItemView, 0, len(items))
	for _, item := range items {
		views = append(views, ItemView{Seq: item.Seq, ActionID: item.ActionID})
	}
	return PlanView{ID: row.ID, State: row.State, Digest: row.Digest, Items: views}, nil
}
