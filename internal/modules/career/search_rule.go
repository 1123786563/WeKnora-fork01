package career

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Frozen recurring search rule statuses, run statuses, and receipt kinds (T13).
// A rule is the only continuous search structure: the user explicitly enables,
// pauses, or disables it, and a disabled or paused rule never triggers, never
// enqueues, and never reaches a source.
const (
	RuleStatusEnabled  = "enabled"
	RuleStatusPaused   = "paused"
	RuleStatusDisabled = "disabled"

	RuleRunStatusCompleted       = "completed"
	RuleRunStatusStarted         = "started"
	RuleRunStatusFailed          = "failed"
	RuleRunStatusBlockedNoQuota  = "blocked_no_quota"
	RuleRunStatusNoVettedSources = "no_vetted_sources"

	RuleTodoStatusOpen = "open"

	RuleKindSet = "rule_set"
	RuleKindRun = "rule_run"

	minRuleIntervalMinutes = 1
	maxRuleIntervalMinutes = 43200 // 30 days

	// ruleEstimateBasis is the frozen estimation methodology surfaced with
	// every enable preview: a deterministic projection from rule parameters,
	// never a fabricated quota balance.
	ruleEstimateBasis = "deterministic projection: triggers_per_day = 1440 / interval_minutes; estimated_searches_per_day = triggers_per_day × vetted sources per trigger; this is an estimate from rule parameters, not a quota balance"

	ruleRunNoteNoQuota         = "the search quota gate refused admission for this trigger; no search was consumed and nothing was fabricated"
	ruleRunNoteNoVettedSources = "no vetted search source is configured; the trigger was not searched and nothing was fabricated"
)

const maxRulePageSize = 50

var ErrRuleNotFound = errors.New("career search rule not found")
var errRuleCandidateStale = errors.New("career search rule candidate is stale")

// SetRuleInput is the frozen request body of the set_rule seam. An empty
// RuleID creates a rule; a non-empty one updates that rule under its scope.
type SetRuleInput struct {
	RequestID        string `json:"requestId"`
	RuleID           string `json:"ruleId,omitempty"`
	Query            string `json:"query"`
	IntervalMinutes  uint64 `json:"intervalMinutes"`
	Status           string `json:"status"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// RuleCostEstimate is the honest enable-time consumption preview computed
// deterministically from the rule parameters and the configured source
// registry. It is never a quota ledger balance.
type RuleCostEstimate struct {
	TriggersPerDay          float64 `json:"triggersPerDay"`
	SourcesPerTrigger       int     `json:"sourcesPerTrigger"`
	EstimatedSearchesPerDay float64 `json:"estimatedSearchesPerDay"`
	Basis                   string  `json:"basis"`
}

// SetRuleReceipt is the durable receipt of every set_rule write and the
// frozen contract served by the rule receipt endpoint.
type SetRuleReceipt struct {
	Kind            string           `json:"kind"`
	RequestID       string           `json:"requestId"`
	RuleID          string           `json:"ruleId"`
	Query           string           `json:"query"`
	IntervalMinutes uint64           `json:"intervalMinutes"`
	Status          string           `json:"status"`
	Revision        uint64           `json:"revision"`
	NextDueAt       *time.Time       `json:"nextDueAt,omitempty"`
	Estimate        RuleCostEstimate `json:"estimate"`
}

// RuleRunView is one rule execution record. Blocked runs (quota refused, no
// vetted sources) are durable visible statuses, never silent skips.
type RuleRunView struct {
	Kind        string    `json:"kind"`
	RuleID      string    `json:"ruleId"`
	Period      uint64    `json:"period"`
	RequestID   string    `json:"requestId"`
	Status      string    `json:"status"`
	SearchID    string    `json:"searchId,omitempty"`
	FailureCode string    `json:"failureCode,omitempty"`
	Note        string    `json:"note,omitempty"`
	TriggeredAt time.Time `json:"triggeredAt"`
}

// RuleTodoView is one discovery todo. The discovery identity is the job link
// surfaced by a vetted source; one link yields exactly one todo per scope.
type RuleTodoView struct {
	TodoID    string    `json:"todoId"`
	RuleID    string    `json:"ruleId"`
	RunID     string    `json:"runId"`
	SearchID  string    `json:"searchId"`
	SourceID  string    `json:"sourceId,omitempty"`
	Link      string    `json:"link"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// RuleView is the frozen rule contract served by GET /career/rules/:ruleId:
// the live configuration, the deterministic estimate, the full run history,
// and the discovery todos the rule produced.
type RuleView struct {
	RuleID          string           `json:"ruleId"`
	Query           string           `json:"query"`
	IntervalMinutes uint64           `json:"intervalMinutes"`
	Status          string           `json:"status"`
	Revision        uint64           `json:"revision"`
	LastPeriod      uint64           `json:"lastPeriod"`
	NextDueAt       *time.Time       `json:"nextDueAt,omitempty"`
	Estimate        RuleCostEstimate `json:"estimate"`
	Runs            []RuleRunView    `json:"runs"`
	Todos           []RuleTodoView   `json:"todos"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// RuleSummary is the bounded list projection. It intentionally excludes run
// and todo histories; those are available only from Rule's detail endpoint.
type RuleSummary struct {
	RuleID          string           `json:"ruleId"`
	Query           string           `json:"query"`
	IntervalMinutes uint64           `json:"intervalMinutes"`
	Status          string           `json:"status"`
	Revision        uint64           `json:"revision"`
	NextDueAt       *time.Time       `json:"nextDueAt"`
	Estimate        RuleCostEstimate `json:"estimate"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

type RulePage struct {
	Rules      []RuleSummary `json:"rules"`
	NextCursor *string       `json:"nextCursor"`
}

type rulePageCursor struct {
	Version  int       `json:"version"`
	TenantID uint64    `json:"tenantId"`
	UserID   string    `json:"userId"`
	Updated  time.Time `json:"updatedAt"`
	RuleID   string    `json:"ruleId"`
}

// RuleRunSummary is the one-line outcome of one triggered rule period.
type RuleRunSummary struct {
	RuleID    string
	Period    uint64
	RequestID string
	Status    string
}

// searchRuleRecord is the durable rule aggregate: conditions (the search
// instruction), frequency, the explicit lifecycle status, the rule write
// revision, and the next due plan (nil unless enabled).
type searchRuleRecord struct {
	// Field order fixes the composite unique index column order to
	// (tenant_id, user_id, id), matching the versioned migration exactly.
	TenantID        uint64     `gorm:"uniqueIndex:career_search_rule_scope;index:idx_career_search_rule_due,priority:1"`
	UserID          string     `gorm:"uniqueIndex:career_search_rule_scope;index:idx_career_search_rule_due,priority:2;size:512"`
	ID              string     `gorm:"primaryKey;size:36;uniqueIndex:career_search_rule_scope"`
	Query           string     `gorm:"size:512;not null"`
	IntervalMinutes uint64     `gorm:"not null"`
	Status          string     `gorm:"size:16;not null;index:idx_career_search_rule_due,priority:3"`
	Revision        uint64     `gorm:"not null"`
	LastPeriod      uint64     `gorm:"not null"`
	NextDueAt       *time.Time `gorm:"index:idx_career_search_rule_due,priority:4"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (searchRuleRecord) TableName() string { return "career_search_rules" }

// searchRuleReceiptRecord stores one set_rule receipt per request ID.
type searchRuleReceiptRecord struct {
	TenantID    uint64 `gorm:"uniqueIndex:career_search_rule_receipt_scope"`
	UserID      string `gorm:"uniqueIndex:career_search_rule_receipt_scope;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_search_rule_receipt_scope;size:128"`
	Fingerprint string `gorm:"size:64;not null"`
	Body        string `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

func (searchRuleReceiptRecord) TableName() string { return "career_search_rule_receipts" }

// searchRuleRunRecord is one durable execution record per rule period. The
// (scope, rule, period) uniqueness is the idempotency key that keeps a
// duplicated trigger from duplicating history.
type searchRuleRunRecord struct {
	ID        string `gorm:"primaryKey;size:36"`
	TenantID  uint64 `gorm:"uniqueIndex:career_search_rule_run_period;index:idx_career_search_rule_run_scope"`
	UserID    string `gorm:"uniqueIndex:career_search_rule_run_period;index:idx_career_search_rule_run_scope;size:512"`
	RuleID    string `gorm:"uniqueIndex:career_search_rule_run_period;size:36"`
	Period    uint64 `gorm:"uniqueIndex:career_search_rule_run_period"`
	RequestID string `gorm:"size:128;not null"`
	Status    string `gorm:"size:32;not null"`
	Body      string `gorm:"type:text;not null"`
	CreatedAt time.Time
}

// searchRuleRunBody extends the public run view with the frozen intent needed
// to recover a committed start after a process restart. Embedded fields keep
// the historic JSON projection compatible with RuleRunView decoding.
type searchRuleRunBody struct {
	RuleRunView
	Query                   string `json:"query,omitempty"`
	ExpectedProfileRevision uint64 `json:"expectedProfileRevision,omitempty"`
}

func (searchRuleRunRecord) TableName() string { return "career_search_rule_runs" }

// searchDiscoveryTodoRecord is one discovery todo per discovered job link.
// The (scope, link) uniqueness is the durable idempotency key: the same job
// surfaced by repeated triggers — or by another rule — stays one todo.
type searchDiscoveryTodoRecord struct {
	ID        string `gorm:"primaryKey;size:36"`
	TenantID  uint64 `gorm:"uniqueIndex:career_search_discovery_todo_scope;index:idx_career_search_discovery_todo_rule"`
	UserID    string `gorm:"uniqueIndex:career_search_discovery_todo_scope;size:512;index:idx_career_search_discovery_todo_rule"`
	RuleID    string `gorm:"size:36;index:idx_career_search_discovery_todo_rule"`
	RunID     string `gorm:"size:36;not null"`
	SearchID  string `gorm:"size:36;not null"`
	SourceID  string `gorm:"size:64;not null;default:''"`
	Link      string `gorm:"uniqueIndex:career_search_discovery_todo_scope;size:2048;not null"`
	Status    string `gorm:"size:16;not null;default:'open'"`
	CreatedAt time.Time
}

func (searchDiscoveryTodoRecord) TableName() string { return "career_search_discovery_todos" }

// ruleClock is the injected clock every rule scheduling decision uses. Tests
// pin Office.searchRuleNow; production reads the wall clock.
func (o *Office) ruleClock() time.Time {
	if o.searchRuleNow != nil {
		return o.searchRuleNow().UTC()
	}
	return time.Now().UTC()
}

// ruleVettedSources counts the actually configured vetted sources; nothing is
// ever invented here.
func (o *Office) ruleVettedSources() []VettedSearchSource {
	if o.searchRegistry == nil {
		return nil
	}
	return o.searchRegistry.SearchSources()
}

// ruleEstimate projects the deterministic consumption of one rule from its
// parameters and the configured registry. The basis string states the
// methodology so the number can never masquerade as a quota balance.
func (o *Office) ruleEstimate(intervalMinutes uint64) RuleCostEstimate {
	sources := len(o.ruleVettedSources())
	triggersPerDay := float64(1440) / float64(intervalMinutes)
	return RuleCostEstimate{
		TriggersPerDay:          triggersPerDay,
		SourcesPerTrigger:       sources,
		EstimatedSearchesPerDay: triggersPerDay * float64(sources),
		Basis:                   ruleEstimateBasis,
	}
}

// planNextDue freezes the scheduling semantics: only an enabled rule holds a
// due plan; pausing or disabling cancels it (nil), and enabling schedules the
// next trigger from the enabling moment.
func planNextDue(status string, now time.Time, intervalMinutes uint64) *time.Time {
	if status != RuleStatusEnabled {
		return nil
	}
	due := now.Add(time.Duration(intervalMinutes) * time.Minute)
	return &due
}

func ruleFingerprint(input SetRuleInput) (string, error) {
	intentBytes, err := json.Marshal([]any{RuleKindSet, input.RequestID, input.RuleID, input.Query, input.IntervalMinutes, input.Status, input.ExpectedRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(intentBytes)
	return hex.EncodeToString(sum[:]), nil
}

func validRuleStatus(status string) bool {
	switch status {
	case RuleStatusEnabled, RuleStatusPaused, RuleStatusDisabled:
		return true
	}
	return false
}

// SetRule creates or updates one recurring search rule. Like every Career
// write it is idempotent by request ID (content changes are typed conflicts),
// pins the caller's observed profile revision, and derives scope from the
// authenticated context only. Rule writes never mutate the profile.
func (o *Office) SetRule(ctx context.Context, input SetRuleInput) (SetRuleReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SetRuleReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SetRuleReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.RuleID = strings.TrimSpace(input.RuleID)
	input.Query = strings.TrimSpace(input.Query)
	if input.RequestID == "" || len(input.RequestID) > 128 || len(input.RuleID) > 36 ||
		input.Query == "" || len(input.Query) > maxSearchQueryBytes ||
		input.IntervalMinutes < minRuleIntervalMinutes || input.IntervalMinutes > maxRuleIntervalMinutes ||
		!validRuleStatus(input.Status) {
		return SetRuleReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := ruleFingerprint(input)
	if err != nil {
		return SetRuleReceipt{}, err
	}
	// Exact replay precedes every side effect.
	if replay, found, lookupErr := o.replayRuleReceipt(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return SetRuleReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	var receipt SetRuleReceipt
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored searchRuleReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return json.Unmarshal([]byte(stored.Body), &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		// The expected revision pins the caller's observed profile view; a
		// rule never mutates the profile, so no revision is advanced.
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		now := o.ruleClock()
		row := searchRuleRecord{
			TenantID: s.TenantID, UserID: s.UserID,
			Query: input.Query, IntervalMinutes: input.IntervalMinutes, Status: input.Status,
			NextDueAt: planNextDue(input.Status, now, input.IntervalMinutes),
			CreatedAt: now, UpdatedAt: now,
		}
		if input.RuleID != "" {
			e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, input.RuleID).
				First(&row).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrRuleNotFound
			}
			if e != nil {
				return e
			}
			row.Query = input.Query
			row.IntervalMinutes = input.IntervalMinutes
			row.Status = input.Status
			row.Revision++
			row.NextDueAt = planNextDue(input.Status, now, input.IntervalMinutes)
			row.UpdatedAt = now
			if e = tx.Save(&row).Error; e != nil {
				return e
			}
		} else {
			row.ID = uuid.NewString()
			row.Revision = 1
			if e = tx.Create(&row).Error; e != nil {
				return e
			}
		}
		receipt = SetRuleReceipt{
			Kind: RuleKindSet, RequestID: input.RequestID, RuleID: row.ID,
			Query: row.Query, IntervalMinutes: row.IntervalMinutes, Status: row.Status,
			Revision: row.Revision, NextDueAt: row.NextDueAt,
			Estimate: o.ruleEstimate(row.IntervalMinutes),
		}
		body, e := json.Marshal(receipt)
		if e != nil {
			return e
		}
		return tx.Create(&searchRuleReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(body), CreatedAt: now,
		}).Error
	})
	if err != nil {
		if isReceiptRaceError(err) {
			if replay, found, replayErr := o.replayRuleReceipt(ctx, s, input.RequestID, fingerprint); replayErr == nil && found {
				return replay, nil
			}
		}
		return SetRuleReceipt{}, err
	}
	return receipt, nil
}

func (o *Office) replayRuleReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (SetRuleReceipt, bool, error) {
	var stored searchRuleReceiptRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SetRuleReceipt{}, false, nil
	}
	if err != nil {
		return SetRuleReceipt{}, false, err
	}
	if fingerprint != "" && stored.Fingerprint != fingerprint {
		return SetRuleReceipt{}, true, ErrIdempotencyConflict
	}
	var replay SetRuleReceipt
	if err = json.Unmarshal([]byte(stored.Body), &replay); err != nil {
		return SetRuleReceipt{}, true, err
	}
	return replay, true, nil
}

// FindRuleReceipt replays a stored set_rule receipt by its original request
// ID under the authenticated scope.
func (o *Office) FindRuleReceipt(ctx context.Context, requestID string) (SetRuleReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return SetRuleReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return SetRuleReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return SetRuleReceipt{}, ErrInvalidRequest
	}
	replay, found, err := o.replayRuleReceipt(ctx, s, requestID, "")
	if err != nil && !errors.Is(err, ErrIdempotencyConflict) {
		return SetRuleReceipt{}, err
	}
	if !found || errors.Is(err, ErrIdempotencyConflict) {
		return SetRuleReceipt{}, ErrReceiptNotFound
	}
	return replay, nil
}

// Rule serves the live rule contract: configuration, deterministic estimate,
// full run history, and discovery todos. Cross-scope lookups fail as not
// found without leaking existence.
func (o *Office) Rule(ctx context.Context, ruleID string) (RuleView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return RuleView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return RuleView{}, err
	}
	ruleID = strings.TrimSpace(ruleID)
	if ruleID == "" || len(ruleID) > 36 {
		return RuleView{}, ErrInvalidRequest
	}
	var row searchRuleRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, ruleID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RuleView{}, ErrRuleNotFound
	}
	if err != nil {
		return RuleView{}, err
	}
	var runRows []searchRuleRunRecord
	if err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND rule_id=?", s.TenantID, s.UserID, ruleID).
		Order("period ASC").Find(&runRows).Error; err != nil {
		return RuleView{}, err
	}
	var todoRows []searchDiscoveryTodoRecord
	if err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND rule_id=?", s.TenantID, s.UserID, ruleID).
		Order("created_at ASC").Find(&todoRows).Error; err != nil {
		return RuleView{}, err
	}
	view := RuleView{
		RuleID: row.ID, Query: row.Query, IntervalMinutes: row.IntervalMinutes,
		Status: row.Status, Revision: row.Revision, LastPeriod: row.LastPeriod,
		NextDueAt: row.NextDueAt, Estimate: o.ruleEstimate(row.IntervalMinutes),
		Runs: []RuleRunView{}, Todos: []RuleTodoView{},
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	for _, runRow := range runRows {
		var run RuleRunView
		if err = json.Unmarshal([]byte(runRow.Body), &run); err != nil {
			return RuleView{}, err
		}
		view.Runs = append(view.Runs, run)
	}
	for _, todoRow := range todoRows {
		view.Todos = append(view.Todos, RuleTodoView{
			TodoID: todoRow.ID, RuleID: todoRow.RuleID, RunID: todoRow.RunID,
			SearchID: todoRow.SearchID, SourceID: todoRow.SourceID, Link: todoRow.Link,
			Status: todoRow.Status, CreatedAt: todoRow.CreatedAt,
		})
	}
	return view, nil
}

// ListRules returns one owner-scoped keyset page in stable updated_at DESC,
// id ASC order. The opaque cursor embeds and verifies its scope so it cannot
// be replayed to enumerate another owner's rules.
func (o *Office) ListRules(ctx context.Context, cursor string) (RulePage, error) {
	s, err := getScope(ctx)
	if err != nil {
		return RulePage{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return RulePage{}, err
	}
	query := o.db.WithContext(ctx).Model(&searchRuleRecord{}).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID)
	if len(cursor) > 2048 {
		return RulePage{}, ErrInvalidRequest
	}
	if cursor != "" {
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(cursor)
		if decodeErr != nil {
			return RulePage{}, ErrInvalidRequest
		}
		var key rulePageCursor
		if json.Unmarshal(decoded, &key) != nil || key.Version != 1 || key.TenantID != s.TenantID || key.UserID != s.UserID || key.RuleID == "" || key.Updated.IsZero() {
			return RulePage{}, ErrInvalidRequest
		}
		query = query.Where("(updated_at < ?) OR (updated_at = ? AND id > ?)", key.Updated, key.Updated, key.RuleID)
	}
	var rows []searchRuleRecord
	if err = query.Select("id, query, interval_minutes, status, revision, next_due_at, created_at, updated_at").Order("updated_at DESC, id ASC").Limit(maxRulePageSize + 1).Find(&rows).Error; err != nil {
		return RulePage{}, err
	}
	page := RulePage{Rules: []RuleSummary{}, NextCursor: nil}
	hasMore := len(rows) > maxRulePageSize
	if hasMore {
		rows = rows[:maxRulePageSize]
	}
	for _, row := range rows {
		if row.Status == RuleStatusEnabled && (row.NextDueAt == nil || row.NextDueAt.IsZero()) {
			return RulePage{}, fmt.Errorf("enabled career search rule %q has no valid due time", row.ID)
		}
		nextDueAt := row.NextDueAt
		if row.Status != RuleStatusEnabled {
			nextDueAt = nil
		}
		page.Rules = append(page.Rules, RuleSummary{
			RuleID: row.ID, Query: row.Query, IntervalMinutes: row.IntervalMinutes,
			Status: row.Status, Revision: row.Revision, NextDueAt: nextDueAt,
			Estimate: o.ruleEstimate(row.IntervalMinutes), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		payload, marshalErr := json.Marshal(rulePageCursor{Version: 1, TenantID: s.TenantID, UserID: s.UserID, Updated: last.UpdatedAt, RuleID: last.ID})
		if marshalErr != nil {
			return RulePage{}, marshalErr
		}
		next := base64.RawURLEncoding.EncodeToString(payload)
		page.NextCursor = &next
	}
	return page, nil
}

// TriggerDueRules is the explicit trigger seam: it evaluates the
// authenticated scope's enabled rules against the injected clock and runs
// every due rule exactly once per period. There is deliberately no resident
// goroutine or timer on the production path — whoever calls this seam owns
// the schedule (see the task report for the frozen ruling).
func (o *Office) TriggerDueRules(ctx context.Context, now time.Time) ([]RuleRunSummary, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	now = now.UTC()
	// A started run has crossed its linearization point. Recover it under the
	// same deterministic request ID even if the rule was paused or edited after
	// the claim committed.
	var started []searchRuleRunRecord
	if err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND status=?", s.TenantID, s.UserID, RuleRunStatusStarted).
		Order("created_at ASC, id ASC").Find(&started).Error; err != nil {
		return nil, err
	}
	outcomes := []RuleRunSummary{}
	var failed []error
	for _, run := range started {
		outcome, recoverErr := o.recoverRuleRun(ctx, s, run, now)
		if recoverErr != nil {
			// CAREER-OCR H8: one failing rule must not abort the sweep nor
			// discard the outcomes already earned — the failing rule keeps
			// its place at the head of the due order, so returning here
			// starved every other rule in the scope.
			failed = append(failed, recoverErr)
			continue
		}
		outcomes = append(outcomes, outcome)
	}
	var due []searchRuleRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND status=? AND next_due_at IS NOT NULL AND next_due_at <= ?",
			s.TenantID, s.UserID, RuleStatusEnabled, now).
		Order("next_due_at ASC, id ASC").Find(&due).Error
	if err != nil {
		return nil, err
	}
	for _, rule := range due {
		outcome, triggerErr := o.triggerRulePeriod(ctx, s, rule, now)
		if triggerErr != nil {
			if errors.Is(triggerErr, errRuleCandidateStale) {
				continue
			}
			failed = append(failed, triggerErr)
			continue
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, errors.Join(failed...)
}

// triggerRulePeriod runs one due period of one rule. The period request ID is
// deterministic (ruleID + period), so a crashed attempt reconciles through
// the T11 idempotency machinery instead of duplicating work. All network I/O
// stays inside SearchOnce, outside any rule transaction.
func (o *Office) triggerRulePeriod(ctx context.Context, s Scope, rule searchRuleRecord, now time.Time) (RuleRunSummary, error) {
	claim, existing, err := o.claimRulePeriod(ctx, s, rule, now)
	if err != nil {
		return RuleRunSummary{}, err
	}
	if existing != nil {
		if existing.Status != RuleRunStatusStarted {
			return summaryFromStoredRun(*existing)
		}
		claim, err = decodeRuleRunIntent(*existing)
		if err != nil {
			return RuleRunSummary{}, err
		}
	}
	return o.executeClaimedRuleRun(ctx, s, claim, now)
}

type claimedRulePeriod struct {
	Run                     RuleRunView
	Query                   string
	ExpectedProfileRevision uint64
}

func summaryFromStoredRun(row searchRuleRunRecord) (RuleRunSummary, error) {
	var run RuleRunView
	if err := json.Unmarshal([]byte(row.Body), &run); err != nil {
		return RuleRunSummary{}, err
	}
	if run.RuleID != row.RuleID || run.Period != row.Period || run.RequestID != row.RequestID || run.Status != row.Status {
		return RuleRunSummary{}, errors.New("invalid persisted career search rule run")
	}
	return RuleRunSummary{RuleID: row.RuleID, Period: row.Period, RequestID: row.RequestID, Status: run.Status}, nil
}

func decodeRuleRunIntent(row searchRuleRunRecord) (claimedRulePeriod, error) {
	var body searchRuleRunBody
	if err := json.Unmarshal([]byte(row.Body), &body); err != nil {
		return claimedRulePeriod{}, err
	}
	if body.Status != RuleRunStatusStarted || body.RuleID != row.RuleID || body.Period != row.Period ||
		body.RequestID != row.RequestID || body.Query == "" || body.RequestID != fmt.Sprintf("rule:%s:%d", row.RuleID, row.Period) {
		return claimedRulePeriod{}, errors.New("invalid persisted career search rule claim")
	}
	return claimedRulePeriod{Run: body.RuleRunView, Query: body.Query, ExpectedProfileRevision: body.ExpectedProfileRevision}, nil
}

func (o *Office) claimRulePeriod(ctx context.Context, s Scope, scanned searchRuleRecord, now time.Time) (claimedRulePeriod, *searchRuleRunRecord, error) {
	var claim claimedRulePeriod
	var existing *searchRuleRunRecord
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Match SetRule's profile -> rule lock order, then lock the durable
		// deletion gate before admitting this period.
		var head profile
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var row searchRuleRecord
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, scanned.ID).First(&row).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return errRuleCandidateStale
			}
			return e
		}
		if row.Status != RuleStatusEnabled || row.Revision != scanned.Revision || row.Query != scanned.Query ||
			row.LastPeriod != scanned.LastPeriod || scanned.NextDueAt == nil || row.NextDueAt == nil ||
			!row.NextDueAt.Equal(*scanned.NextDueAt) || row.NextDueAt.After(now) {
			return errRuleCandidateStale
		}
		period := scanned.LastPeriod + 1
		var run searchRuleRunRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND rule_id=? AND period=?", s.TenantID, s.UserID, row.ID, period).First(&run).Error
		if e == nil {
			existing = &run
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		requestID := fmt.Sprintf("rule:%s:%d", row.ID, period)
		if e = admitLifecycleClaimTx(tx, s, "rule_run", requestID, row.Query); e != nil {
			return e
		}
		claim.Run = RuleRunView{Kind: RuleKindRun, RuleID: row.ID, Period: period, RequestID: requestID, Status: RuleRunStatusStarted, TriggeredAt: now}
		claim.Query = row.Query
		claim.ExpectedProfileRevision = head.Revision
		body, marshalErr := json.Marshal(searchRuleRunBody{RuleRunView: claim.Run, Query: claim.Query, ExpectedProfileRevision: head.Revision})
		if marshalErr != nil {
			return marshalErr
		}
		run = searchRuleRunRecord{ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID, RuleID: row.ID,
			Period: period, RequestID: requestID, Status: RuleRunStatusStarted, Body: string(body), CreatedAt: now}
		if e = tx.Create(&run).Error; e != nil {
			return e
		}
		nextPeriodDue := planNextDue(row.Status, now, row.IntervalMinutes)
		update := tx.Model(&searchRuleRecord{}).Where("tenant_id=? AND user_id=? AND id=? AND revision=? AND status=? AND last_period=? AND next_due_at=?", s.TenantID, s.UserID, row.ID, row.Revision, RuleStatusEnabled, row.LastPeriod, row.NextDueAt).Updates(map[string]any{"last_period": period, "next_due_at": nextPeriodDue})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errRuleCandidateStale
		}
		existing = &run
		return nil
	})
	return claim, existing, err
}

func (o *Office) recoverRuleRun(ctx context.Context, s Scope, row searchRuleRunRecord, now time.Time) (RuleRunSummary, error) {
	if row.Status != RuleRunStatusStarted {
		return summaryFromStoredRun(row)
	}
	claim, err := decodeRuleRunIntent(row)
	if err != nil {
		return RuleRunSummary{}, err
	}
	return o.executeClaimedRuleRun(ctx, s, claim, now)
}

func (o *Office) executeClaimedRuleRun(ctx context.Context, s Scope, claim claimedRulePeriod, now time.Time) (RuleRunSummary, error) {
	run := claim.Run
	scope, err := getScope(ctx)
	if err != nil {
		return RuleRunSummary{}, err
	}
	// Re-establish admission on restart before recovering the external request.
	// During deletion the gate permits only this already-admitted request ID.
	if err = o.admitLifecycleClaim(ctx, scope, "rule_run", run.RequestID, claim.Query); err != nil {
		return RuleRunSummary{}, err
	}
	var receipt SearchOnceReceipt
	runResults := false
	if len(o.ruleVettedSources()) == 0 {
		run.Status, run.Note = RuleRunStatusNoVettedSources, ruleRunNoteNoVettedSources
	} else {
		input := SearchOnceInput{RequestID: run.RequestID, Query: claim.Query, ExpectedRevision: claim.ExpectedProfileRevision}
		result, searchErr := o.SearchOnce(ctx, input)
		if errors.Is(searchErr, ErrRevisionConflict) {
			// The rule claim has already committed. If SearchOnce has no row yet,
			// refresh only its profile pin and retry the same request identity.
			var searchRow searchRecord
			lookupErr := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, run.RequestID).First(&searchRow).Error
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				var head profile
				lookupErr = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
				if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
					head.Revision = 0
					lookupErr = nil
				}
				if lookupErr == nil {
					result, searchErr = o.SearchOnce(ctx, SearchOnceInput{RequestID: run.RequestID, Query: claim.Query, ExpectedRevision: head.Revision})
				}
			} else if lookupErr == nil {
				result, searchErr = o.searchOnceUnderStoredFingerprint(ctx, s, run.RequestID)
			}
		} else if errors.Is(searchErr, ErrIdempotencyConflict) {
			// The period request ID can outlive the profile revision used by
			// SearchOnce's first claim. Recover it under that durable fingerprint.
			result, searchErr = o.searchOnceUnderStoredFingerprint(ctx, s, run.RequestID)
		}
		if errors.Is(searchErr, ErrSearchQuotaRefused) {
			run.Status, run.Note = RuleRunStatusBlockedNoQuota, ruleRunNoteNoQuota
		} else if searchErr != nil {
			return RuleRunSummary{}, searchErr
		} else {
			receipt, runResults = result, true
			if receipt.Status == SearchStatusCompleted {
				run.Status, run.SearchID = RuleRunStatusCompleted, receipt.SearchID
			} else {
				run.Status, run.SearchID, run.FailureCode = RuleRunStatusFailed, receipt.SearchID, receipt.FailureCode
			}
		}
	}
	var summary RuleRunSummary
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current searchRuleRunRecord
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND rule_id=? AND period=?", s.TenantID, s.UserID, run.RuleID, run.Period).First(&current).Error; e != nil {
			return e
		}
		if current.Status != RuleRunStatusStarted {
			var e error
			summary, e = summaryFromStoredRun(current)
			return e
		}
		if current.RequestID != run.RequestID {
			return ErrIdempotencyConflict
		}
		body, e := json.Marshal(searchRuleRunBody{RuleRunView: run, Query: claim.Query, ExpectedProfileRevision: claim.ExpectedProfileRevision})
		if e != nil {
			return e
		}
		if e = tx.Model(&searchRuleRunRecord{}).Where("id=?", current.ID).Updates(map[string]any{"status": run.Status, "body": string(body)}).Error; e != nil {
			return e
		}
		if runResults && len(receipt.Results) > 0 {
			for _, result := range receipt.Results {
				todo := searchDiscoveryTodoRecord{ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID, RuleID: run.RuleID, RunID: current.ID,
					SearchID: receipt.SearchID, SourceID: result.SourceID, Link: result.Link, Status: RuleTodoStatusOpen, CreatedAt: now}
				if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&todo).Error; e != nil {
					return e
				}
			}
		}
		if e = tx.Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", s.TenantID, s.UserID, "rule_run", run.RequestID).Delete(&lifecycleClaim{}).Error; e != nil {
			return e
		}
		summary = RuleRunSummary{RuleID: run.RuleID, Period: run.Period, RequestID: run.RequestID, Status: run.Status}
		return nil
	})
	if err != nil {
		return RuleRunSummary{}, err
	}
	return summary, nil
}
