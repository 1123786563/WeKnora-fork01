package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Craft usage outbox delivery states, mirroring the commercial outbox
// vocabulary: pending awaits the OpenMeter delivery worker (G4 specialty),
// sent was acknowledged, dead exhausted its delivery attempts.
const (
	CraftUsageOutboxPending = "pending"
	CraftUsageOutboxSent    = "sent"
	CraftUsageOutboxDead    = "dead"
)

// craftUsageDeliveryAttempts bounds how often the delivery worker may retry
// one usage event before it is parked as dead for operator reconciliation.
const craftUsageDeliveryAttempts = 10

// ErrUsageEventUnknown reports a delivery-state transition for an event key
// the outbox does not hold.
var ErrUsageEventUnknown = errors.New("craft usage event unknown")

// craftUsageFactRow stores one revision of one physical attempt observation.
// The append-only (tenant_id, call_id, attempt_id, revision) UNIQUE constraint
// is the storage-level dedup: replays conflict and no-op, corrections arrive
// as new revisions, history is never edited in place.
type craftUsageFactRow struct {
	ID           string    `gorm:"column:id"`
	TenantID     uint64    `gorm:"column:tenant_id"`
	RunID        string    `gorm:"column:run_id"`
	DelegationID string    `gorm:"column:delegation_id"`
	CallID       string    `gorm:"column:call_id"`
	AttemptID    string    `gorm:"column:attempt_id"`
	Revision     int64     `gorm:"column:revision"`
	Runtime      string    `gorm:"column:runtime"`
	ModelID      string    `gorm:"column:model_id"`
	Funding      string    `gorm:"column:funding"`
	Status       string    `gorm:"column:status"`
	InputTokens  int64     `gorm:"column:input_tokens"`
	OutputTokens int64     `gorm:"column:output_tokens"`
	CachedTokens int64     `gorm:"column:cached_tokens"`
	FactJSON     string    `gorm:"column:fact_json"`
	ObservedAt   time.Time `gorm:"column:observed_at"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (craftUsageFactRow) TableName() string { return "craft_usage_facts" }

func (r craftUsageFactRow) fact() (craft.UsageFact, error) {
	var f craft.UsageFact
	if err := json.Unmarshal([]byte(r.FactJSON), &f); err != nil {
		return craft.UsageFact{}, fmt.Errorf("craft usage: decode fact %s: %w", r.ID, err)
	}
	return f, nil
}

// craftUsageOutboxRow is one durable delivery event per fact revision under
// the stable identity usage:<usage_key>:<revision>: redelivery after a
// failed OpenMeter push replays the SAME event key, so emission is
// exactly-once at the storage layer no matter how often the worker retries.
type craftUsageOutboxRow struct {
	EventKey     string    `gorm:"column:event_key"`
	TenantID     uint64    `gorm:"column:tenant_id"`
	CallID       string    `gorm:"column:call_id"`
	AttemptID    string    `gorm:"column:attempt_id"`
	Revision     int64     `gorm:"column:revision"`
	PayloadJSON  string    `gorm:"column:payload_json"`
	State        string    `gorm:"column:state"`
	AttemptCount int64     `gorm:"column:attempt_count"`
	ObservedAt   time.Time `gorm:"column:observed_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (craftUsageOutboxRow) TableName() string { return "craft_usage_outbox" }

// CraftUsageEvent is one outbox entry handed to the delivery worker: the
// stable redelivery identity and the raw envelope payload.
type CraftUsageEvent struct {
	EventKey     string
	TenantID     uint64
	CallID       string
	AttemptID    string
	Revision     int64
	PayloadJSON  string
	State        string
	AttemptCount int64
	ObservedAt   time.Time
}

// craftUsageEnvelope is the outbox payload: the raw UsageFact exactly as
// validated, plus the revision and observation time the worker needs to map
// the OpenMeter CloudEvent (G4 specialty owns the field/meter mapping).
type craftUsageEnvelope struct {
	Revision   int64           `json:"revision"`
	ObservedAt time.Time       `json:"observed_at"`
	Fact       craft.UsageFact `json:"fact"`
}

// CraftUsageStore is the durable craft.UsageSink: it persists physical usage
// facts append-only and enqueues one stable-identity delivery event per
// revision in the same transaction. It performs NO pricing and NO OpenMeter
// delivery — the Credits ledger stays with the commercial specialty; only
// the raw observation and its delivery state live here.
type CraftUsageStore struct {
	db *gorm.DB
}

var _ craft.UsageSink = (*CraftUsageStore)(nil)

// NewCraftUsageStore constructs the usage sink backed by the migrated
// business database.
func NewCraftUsageStore(db *gorm.DB) *CraftUsageStore {
	return &CraftUsageStore{db: db}
}

// Append implements craft.UsageSink: the first observation of one physical
// attempt is stored as revision 1; re-appending ANY previously stored
// revision of the same attempt is an idempotent no-op (at-least-once
// observation, worker replay, OC summary replay); a fact that matches no
// stored revision is a conflict — changed observations must travel through
// Correct so recorded history is never overwritten.
func (s *CraftUsageStore) Append(ctx context.Context, f craft.UsageFact) error {
	return s.record(ctx, f, false)
}

// Correct implements craft.UsageSink: files a later observation of an
// already-recorded attempt as a NEW revision with status corrected. The
// earlier revision stays readable: the full revision trail is the
// reconciliation record for stream breaks and post-cancellation arrivals.
// Replaying the current correction is idempotent; a correction for an
// attempt nobody recorded is ErrUsageNotRecorded.
func (s *CraftUsageStore) Correct(ctx context.Context, f craft.UsageFact) error {
	return s.record(ctx, f, true)
}

func (s *CraftUsageStore) record(ctx context.Context, f craft.UsageFact, correction bool) error {
	key, err := f.ResolvedID()
	if err != nil {
		return err
	}
	if correction {
		// A correction supersedes; the stored marker is always corrected.
		f.Status = craft.UsageStatusCorrected
		if err := f.Validate(); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	var recordedStatus string
	txErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []craftUsageFactRow
		if err := tx.Where("tenant_id = ? AND call_id = ? AND attempt_id = ?",
			f.TenantID, f.CallID, f.AttemptID).
			Order("revision ASC").Find(&rows).Error; err != nil {
			return err
		}

		// Idempotency: an exact replay of any stored revision of this
		// attempt (same observation content) is a no-op.
		for _, row := range rows {
			if stored, err := row.fact(); err == nil && usageContentEqual(stored, f) {
				return nil
			}
		}

		if len(rows) == 0 {
			if correction {
				return fmt.Errorf("%w: usage attempt %s", craft.ErrUsageNotRecorded, key)
			}
		} else if !correction {
			return fmt.Errorf("%w: usage attempt %s already recorded with a different observation",
				craft.ErrConflict, key)
		}

		next := int64(1)
		if len(rows) > 0 {
			next = rows[len(rows)-1].Revision + 1
		}

		stored := f
		stored.ID = key
		encoded, err := json.Marshal(stored)
		if err != nil {
			return fmt.Errorf("%w: encode usage fact: %v", craft.ErrInvalidInput, err)
		}
		row := craftUsageFactRow{
			ID: craftUsageRowID(key, next), TenantID: f.TenantID,
			RunID: f.RunID, DelegationID: f.DelegationID,
			CallID: f.CallID, AttemptID: f.AttemptID, Revision: next,
			Runtime: f.Runtime, ModelID: f.ModelID, Funding: f.Funding,
			Status: f.Status, InputTokens: f.Input, OutputTokens: f.Output,
			CachedTokens: f.Cached, FactJSON: string(encoded), ObservedAt: now,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		// SP12: fold the freshly recorded revision into the owner's daily
		// user_usage bucket, inside the SAME transaction. The fold runs only
		// when THIS call created the fact row — a concurrent creator of the
		// same revision that no-opped on the UNIQUE constraint must not fold
		// the tokens twice — and never for corrections: the daily bucket
		// keeps the first Append's numbers, corrected revisions flow through
		// the commercial outbox pipeline, whose pricing owns craft cost.
		if created.RowsAffected > 0 && !correction {
			if err := foldCraftUsageIntoUserBucket(tx, row); err != nil {
				return err
			}
		}
		if err := enqueueCraftUsageEvent(tx, row, stored); err != nil {
			return err
		}
		recordedStatus = f.Status
		return nil
	})
	if txErr != nil {
		return txErr
	}
	// O04: an unobserved attempt keeps its own visible line — the metric is
	// the fleet-level view of the same fact, never a zero-token fabrication.
	if recordedStatus == craft.UsageStatusUnknown {
		metrics.CraftUsageUnknown()
	}
	return nil
}

// foldCraftUsageIntoUserBucket upserts one craft-flow daily user_usage row for
// a freshly recorded fact, in the record transaction. Attribution joins the
// durable run table: agent_runs stores the sessions.user_id scope as owner_id
// (there is no user_id column), and a fact whose run matches no row — or
// carries no run at all — attributes to the empty user rather than being
// dropped. Cost and cache_write stay zero (Ruling P-2): craft pricing lives
// in the commercial outbox pipeline and craft facts expose no cache-write
// dimension, cached tokens fold into cache_read. The bucket day comes from
// the fact row's observed_at (record-time UTC clock), bound as a Go-side UTC
// midnight value so no dialect date function is needed; the DO UPDATE arms
// accumulate in place, table-qualified for PostgreSQL's ON CONFLICT.
func foldCraftUsageIntoUserBucket(tx *gorm.DB, row craftUsageFactRow) error {
	observed := row.ObservedAt.UTC()
	day := time.Date(observed.Year(), observed.Month(), observed.Day(), 0, 0, 0, 0, time.UTC)
	const foldSQL = `INSERT INTO user_usage
	(tenant_id, user_id, window_start, model, flow,
	 input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_microcredits,
	 created_at, updated_at)
	VALUES (?, COALESCE((SELECT ar.owner_id FROM agent_runs ar
	                     WHERE ar.tenant_id = ? AND ar.run_id = ? LIMIT 1), ''),
	        ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)
	ON CONFLICT (tenant_id, user_id, window_start, model, flow) DO UPDATE SET
	  input_tokens      = user_usage.input_tokens + excluded.input_tokens,
	  output_tokens     = user_usage.output_tokens + excluded.output_tokens,
	  cache_read_tokens = user_usage.cache_read_tokens + excluded.cache_read_tokens,
	  cache_write_tokens = user_usage.cache_write_tokens + excluded.cache_write_tokens,
	  cost_microcredits = user_usage.cost_microcredits + excluded.cost_microcredits,
	  updated_at        = excluded.updated_at`
	return tx.Exec(foldSQL,
		row.TenantID, row.TenantID, row.RunID,
		day, row.ModelID, types.UsageFlowCraft,
		row.InputTokens, row.OutputTokens, row.CachedTokens,
		observed, observed,
	).Error
}

// usageContentEqual compares the observation content of two facts of the
// same identity; the derived ID and the observation time are not content.
func usageContentEqual(a, b craft.UsageFact) bool {
	return a.RunID == b.RunID && a.DelegationID == b.DelegationID &&
		a.CallID == b.CallID && a.AttemptID == b.AttemptID &&
		a.Runtime == b.Runtime && a.ModelID == b.ModelID &&
		a.Funding == b.Funding && a.Status == b.Status &&
		a.TenantID == b.TenantID && a.Input == b.Input &&
		a.Output == b.Output && a.Cached == b.Cached
}

// craftUsageRowID builds the per-revision row primary key from the stable
// attempt identity plus its revision; the uuid suffix keeps concurrent
// creators of the same revision collision-free (the loser no-ops on the
// UNIQUE constraint instead of failing the transaction).
func craftUsageRowID(key string, revision int64) string {
	return fmt.Sprintf("%s:%d:%s", key, revision, uuid.NewString()[:8])
}

// craftUsageEventKey is the stable delivery identity of one fact revision:
// redelivering after a failed OpenMeter push replays this exact key.
func craftUsageEventKey(key string, revision int64) string {
	return fmt.Sprintf("usage:%s:%d", key, revision)
}

// enqueueCraftUsageEvent writes the delivery event of one fact revision in
// the caller's transaction. Unknown and corrected revisions are events too:
// downstream reconciliation must observe them, and the payload status tells
// the mapper an unknown fact must never become a zero-usage meter event.
func enqueueCraftUsageEvent(tx *gorm.DB, row craftUsageFactRow, f craft.UsageFact) error {
	key := craft.UsageKey(f.TenantID, f.CallID, f.AttemptID)
	payload, err := json.Marshal(craftUsageEnvelope{
		Revision: row.Revision, ObservedAt: row.ObservedAt, Fact: f,
	})
	if err != nil {
		return fmt.Errorf("craft usage: encode event %s: %w", row.ID, err)
	}
	event := craftUsageOutboxRow{
		EventKey: craftUsageEventKey(key, row.Revision),
		TenantID: row.TenantID, CallID: row.CallID, AttemptID: row.AttemptID,
		Revision: row.Revision, PayloadJSON: string(payload),
		State: CraftUsageOutboxPending, ObservedAt: row.ObservedAt, UpdatedAt: row.ObservedAt,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
		return err
	}
	return nil
}

// Facts returns the CURRENT revision (highest revision per physical
// attempt) of every attempt recorded in one tenant's run, oldest observation
// first. Cross-tenant rows are invisible.
func (s *CraftUsageStore) Facts(ctx context.Context, tenant uint64, runID string) ([]craft.UsageFact, error) {
	var rows []craftUsageFactRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", tenant, runID).
		Order("observed_at ASC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return craftCurrentFacts(rows)
}

// FactsBySession returns the CURRENT revision of every attempt recorded
// across one tenant's session — all of the session's runs joined through the
// durable run table — oldest observation first. Cross-tenant rows are
// invisible. This is the O04 usage view's read: which space/run produced
// which physical call stays traceable from the fact itself.
func (s *CraftUsageStore) FactsBySession(ctx context.Context, tenant uint64, sessionID string) ([]craft.UsageFact, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("craft usage: session scope requires a session id")
	}
	var rows []craftUsageFactRow
	err := s.db.WithContext(ctx).
		Joins("JOIN agent_runs ar ON ar.run_id = craft_usage_facts.run_id AND ar.tenant_id = craft_usage_facts.tenant_id").
		Where("craft_usage_facts.tenant_id = ? AND ar.session_id = ?", tenant, sessionID).
		Order("craft_usage_facts.observed_at ASC, craft_usage_facts.id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return craftCurrentFacts(rows)
}

// craftCurrentFacts folds revision rows into the CURRENT fact of every
// physical attempt (highest revision per call+attempt), preserving the
// oldest-first observation order of the input rows.
func craftCurrentFacts(rows []craftUsageFactRow) ([]craft.UsageFact, error) {
	attemptID := func(row craftUsageFactRow) string { return row.CallID + "\x00" + row.AttemptID }
	latest := make(map[string]craftUsageFactRow)
	for _, row := range rows {
		id := attemptID(row)
		if cur, ok := latest[id]; !ok || row.Revision > cur.Revision {
			latest[id] = row
		}
	}
	facts := make([]craft.UsageFact, 0, len(latest))
	for _, row := range rows {
		if latest[attemptID(row)].ID == row.ID {
			f, err := row.fact()
			if err != nil {
				return nil, err
			}
			facts = append(facts, f)
		}
	}
	return facts, nil
}

// Revisions returns the full revision trail of one physical attempt in
// revision order — the reconciliation record behind stream breaks
// (revision 1 unknown) and post-cancellation late arrivals (corrected
// revisions appended later).
func (s *CraftUsageStore) Revisions(ctx context.Context, tenant uint64, callID, attemptID string) ([]craft.UsageFact, error) {
	var rows []craftUsageFactRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND call_id = ? AND attempt_id = ?", tenant, callID, attemptID).
		Order("revision ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	facts := make([]craft.UsageFact, 0, len(rows))
	for _, row := range rows {
		f, err := row.fact()
		if err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	return facts, nil
}

// FactsByDelegation returns the CURRENT revision of every physical attempt
// recorded under one delegation (the OC child calls of one sub-execution),
// oldest observation first. This is the read side of the OC aggregate
// cross-check: aggregate events are verified against exactly these facts.
func (s *CraftUsageStore) FactsByDelegation(ctx context.Context, tenant uint64, delegationID string) ([]craft.UsageFact, error) {
	var rows []craftUsageFactRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND delegation_id = ?", tenant, delegationID).
		Order("observed_at ASC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	attemptKey := func(row craftUsageFactRow) string { return row.CallID + "\x00" + row.AttemptID }
	latest := make(map[string]craftUsageFactRow)
	for _, row := range rows {
		id := attemptKey(row)
		if cur, ok := latest[id]; !ok || row.Revision > cur.Revision {
			latest[id] = row
		}
	}
	facts := make([]craft.UsageFact, 0, len(latest))
	for _, row := range rows {
		if latest[attemptKey(row)].ID == row.ID {
			f, err := row.fact()
			if err != nil {
				return nil, err
			}
			facts = append(facts, f)
		}
	}
	return facts, nil
}

// ListUsageEvents returns the outbox entries of one attempt's revisions in
// revision order, delivery state included.
func (s *CraftUsageStore) ListUsageEvents(ctx context.Context, tenant uint64, callID, attemptID string) ([]CraftUsageEvent, error) {
	var rows []craftUsageOutboxRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND call_id = ? AND attempt_id = ?", tenant, callID, attemptID).
		Order("revision ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]CraftUsageEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, craftUsageEventView(row))
	}
	return events, nil
}

// PendingUsageEvents hands the delivery worker up to limit pending events,
// oldest observation first. The worker (G4 OpenMeter specialty) owns the
// CloudEvent mapping; this only exposes durable state.
func (s *CraftUsageStore) PendingUsageEvents(ctx context.Context, limit int) ([]CraftUsageEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []craftUsageOutboxRow
	err := s.db.WithContext(ctx).
		Where("state = ?", CraftUsageOutboxPending).
		Order("observed_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]CraftUsageEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, craftUsageEventView(row))
	}
	return events, nil
}

func craftUsageEventView(row craftUsageOutboxRow) CraftUsageEvent {
	return CraftUsageEvent{
		EventKey: row.EventKey, TenantID: row.TenantID,
		CallID: row.CallID, AttemptID: row.AttemptID, Revision: row.Revision,
		PayloadJSON: row.PayloadJSON, State: row.State,
		AttemptCount: row.AttemptCount, ObservedAt: row.ObservedAt,
	}
}

// MarkUsageEventDelivered records a successful delivery under the event's
// stable identity; replaying the mark is idempotent (an already-sent event
// stays sent instead of being reported unknown).
func (s *CraftUsageStore) MarkUsageEventDelivered(ctx context.Context, eventKey string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftUsageOutboxRow
		err := tx.Where("event_key = ?", eventKey).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUsageEventUnknown
		}
		if err != nil {
			return err
		}
		if row.State == CraftUsageOutboxSent {
			return nil
		}
		return tx.Model(&craftUsageOutboxRow{}).
			Where("event_key = ?", eventKey).
			Updates(map[string]any{
				"state":      CraftUsageOutboxSent,
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
			}).Error
	})
}

// FailUsageEventDelivery counts one failed delivery attempt; an event that
// exhausted craftUsageDeliveryAttempts is parked as dead for operator
// reconciliation instead of being retried forever.
func (s *CraftUsageStore) FailUsageEventDelivery(ctx context.Context, eventKey string) error {
	res := s.db.WithContext(ctx).Model(&craftUsageOutboxRow{}).
		Where("event_key = ?", eventKey).
		Updates(map[string]any{
			"attempt_count": gorm.Expr("attempt_count + 1"),
			"state": gorm.Expr("CASE WHEN attempt_count + 1 >= ? THEN ? ELSE ? END",
				craftUsageDeliveryAttempts, CraftUsageOutboxDead, CraftUsageOutboxPending),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUsageEventUnknown
	}
	return nil
}
