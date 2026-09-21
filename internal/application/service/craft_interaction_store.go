package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormCraftInteractionStore is the C02 production CraftInteractionStore: one
// durable craft_interactions row per pending question/permission, and the
// user decision plus its craft_decision_outbox item committed in the SAME
// transaction, so a crash can never leave a recorded decision without its
// redelivery record. Reads are scope-guarded exactly like the craft workspace
// store: cross-tenant rows do not exist for the caller, cross-user rows are
// forbidden.
type GormCraftInteractionStore struct {
	db *gorm.DB
}

var (
	_ CraftInteractionStore = (*GormCraftInteractionStore)(nil)
	_ CraftDecisionOutboxRo = (*GormCraftInteractionStore)(nil)
)

// NewGormCraftInteractionStore assembles the production interaction store.
func NewGormCraftInteractionStore(db *gorm.DB) *GormCraftInteractionStore {
	return &GormCraftInteractionStore{db: db}
}

type craftInteractionRow struct {
	ID            string    `gorm:"column:id"`
	TenantID      uint64    `gorm:"column:tenant_id"`
	SessionID     string    `gorm:"column:session_id"`
	OwnerID       string    `gorm:"column:owner_id"`
	RunID         string    `gorm:"column:run_id"`
	TaskID        string    `gorm:"column:task_id"`
	ToolCallID    string    `gorm:"column:tool_call_id"`
	PendingID     string    `gorm:"column:pending_id"`
	Kind          string    `gorm:"column:kind"`
	ArgsHash      string    `gorm:"column:args_hash"`
	Prompt        string    `gorm:"column:prompt"`
	OcSessionID   string    `gorm:"column:oc_session_id"`
	OcRequestID   string    `gorm:"column:oc_request_id"`
	PayloadJSON   *string   `gorm:"column:payload_json"`
	Status        string    `gorm:"column:status"`
	Delivery      string    `gorm:"column:delivery"`
	DecisionID    string    `gorm:"column:decision_id"`
	DecidedAction string    `gorm:"column:decided_action"`
	AnswersJSON   *string   `gorm:"column:answers_json"`
	DecidedBy     string    `gorm:"column:decided_by"`
	Revision      int64     `gorm:"column:revision"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (craftInteractionRow) TableName() string { return "craft_interactions" }

func (r craftInteractionRow) view() (CraftInteractionRecord, error) {
	record := CraftInteractionRecord{
		Interaction: craft.Interaction{
			ID: r.ID, Kind: r.Kind, ArgsHash: r.ArgsHash, Prompt: r.Prompt, Revision: r.Revision,
		},
		Scope:             craft.Scope{TenantID: r.TenantID, UserID: r.OwnerID, SessionID: r.SessionID},
		RunID:             r.RunID,
		TaskID:            r.TaskID,
		ToolCallID:        r.ToolCallID,
		PendingID:         r.PendingID,
		OpenCodeSessionID: r.OcSessionID,
		OpenCodeRequestID: r.OcRequestID,
		Status:            r.Status,
		Delivery:          r.Delivery,
		DecisionID:        r.DecisionID,
		DecidedAction:     r.DecidedAction,
		DecidedBy:         r.DecidedBy,
	}
	if r.PayloadJSON != nil && *r.PayloadJSON != "" {
		var pending craft.PendingDecision
		if err := json.Unmarshal([]byte(*r.PayloadJSON), &pending); err != nil {
			return CraftInteractionRecord{}, fmt.Errorf("craft: decode interaction %s payload: %w", r.ID, err)
		}
		// The row is the authoritative identity: heal the embedded interaction
		// fields from it so a payload recorded without them still validates.
		pending.Interaction = record.Interaction
		record.Pending = pending
	}
	if r.AnswersJSON != nil && *r.AnswersJSON != "" {
		if err := json.Unmarshal([]byte(*r.AnswersJSON), &record.RecordedAnswers); err != nil {
			return CraftInteractionRecord{}, fmt.Errorf("craft: decode interaction %s answers: %w", r.ID, err)
		}
	}
	return record, nil
}

// PutInteraction registers one pending interaction idempotently: repeating
// the same registration returns the stored row unchanged; the same identity
// with different arguments is a conflict (a decision for one payload can
// never authorize another).
func (s *GormCraftInteractionStore) PutInteraction(ctx context.Context, in CraftInteractionRecord) (CraftInteractionRecord, error) {
	if in.ID == "" || in.Kind == "" || in.ArgsHash == "" || in.Prompt == "" || in.PendingID == "" ||
		in.OpenCodeSessionID == "" || in.OpenCodeRequestID == "" ||
		in.Scope.TenantID == 0 || in.Scope.UserID == "" || in.Scope.SessionID == "" || in.RunID == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: incomplete interaction registration", craft.ErrInvalidInput)
	}
	if in.Kind != craft.InteractionQuestion && in.Kind != craft.InteractionPermission {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction kind %q", craft.ErrInvalidInput, in.Kind)
	}
	var payload *string
	if hasPayload(in.Pending) {
		encoded, err := json.Marshal(in.Pending)
		if err != nil {
			return CraftInteractionRecord{}, err
		}
		value := string(encoded)
		payload = &value
	}
	var out CraftInteractionRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := craftInteractionRow{
			ID: in.ID, TenantID: in.Scope.TenantID, SessionID: in.Scope.SessionID, OwnerID: in.Scope.UserID,
			RunID: in.RunID, TaskID: in.TaskID, ToolCallID: in.ToolCallID, PendingID: in.PendingID,
			Kind: in.Kind, ArgsHash: in.ArgsHash, Prompt: in.Prompt,
			OcSessionID: in.OpenCodeSessionID, OcRequestID: in.OpenCodeRequestID, PayloadJSON: payload,
			Status: "pending", Delivery: "pending", Revision: 1,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			view, verr := row.view()
			if verr != nil {
				return verr
			}
			view.Pending = in.Pending
			out = view
			return nil
		}
		var stored craftInteractionRow
		if err := tx.Where("tenant_id = ? AND id = ?", in.Scope.TenantID, in.ID).Take(&stored).Error; err != nil {
			return err
		}
		if stored.Kind != in.Kind || stored.ArgsHash != in.ArgsHash || stored.Prompt != in.Prompt {
			return fmt.Errorf("%w: interaction %s re-registered with different arguments", craft.ErrConflict, in.ID)
		}
		view, verr := stored.view()
		if verr != nil {
			return verr
		}
		out = view
		return nil
	})
	if err != nil {
		return CraftInteractionRecord{}, err
	}
	return out, nil
}

func hasPayload(pending craft.PendingDecision) bool {
	return len(pending.Options) > 0 || len(pending.Multiple) > 0 || len(pending.Permissions) > 0
}

func (s *GormCraftInteractionStore) getRow(db *gorm.DB, scope craft.Scope, id string) (craftInteractionRow, error) {
	var row craftInteractionRow
	err := db.Where("tenant_id = ? AND id = ?", scope.TenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftInteractionRow{}, fmt.Errorf("%w: interaction %s", craft.ErrNotFound, id)
	}
	if err != nil {
		return craftInteractionRow{}, err
	}
	if row.SessionID != scope.SessionID {
		return craftInteractionRow{}, fmt.Errorf("%w: interaction %s", craft.ErrNotFound, id)
	}
	if row.OwnerID != scope.UserID {
		return craftInteractionRow{}, fmt.Errorf("%w: interaction %s", craft.ErrForbidden, id)
	}
	return row, nil
}

// GetInteraction returns the durable interaction in the requesting scope.
func (s *GormCraftInteractionStore) GetInteraction(ctx context.Context, scope craft.Scope, id string) (CraftInteractionRecord, error) {
	row, err := s.getRow(s.db.WithContext(ctx), scope, id)
	if err != nil {
		return CraftInteractionRecord{}, err
	}
	return row.view()
}

// ListInteractions returns the session's interactions newest-first for the
// pending-decision surface, newest decided rows included so the UI can show
// the already-processed state.
func (s *GormCraftInteractionStore) ListInteractions(ctx context.Context, scope craft.Scope, sessionID string) ([]CraftInteractionRecord, error) {
	var rows []craftInteractionRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", scope.TenantID, sessionID).
		Order("created_at DESC, id DESC").Limit(50).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]CraftInteractionRecord, 0, len(rows))
	for _, row := range rows {
		if row.OwnerID != scope.UserID {
			continue // visible but not actionable for a shared-session reader
		}
		view, verr := row.view()
		if verr != nil {
			return nil, verr
		}
		out = append(out, view)
	}
	return out, nil
}

// decisionPayloadHash digests the caller-controlled decision payload: the
// same decision id with the same payload replays the original result; any
// change answers ErrConflict.
func decisionPayloadHash(req CraftDecisionRequest) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-decision/%s/%s/%s/%d/%s/%s",
		req.InteractionID, req.DecisionID, req.Action, req.ExpectedRevision, req.ArgsHash,
		encodeAnswersForHash(req.Answers))))
	return hex.EncodeToString(sum[:16])
}

func encodeAnswersForHash(answers []craft.Answer) string {
	rows := make([]string, 0, len(answers))
	for _, answer := range answers {
		choices := append([]string(nil), answer.Choices...)
		sort.Strings(choices)
		rows = append(rows, fmt.Sprintf("%s|%s|%s", answer.QuestionID, joinNonEmpty(choices), answer.Text))
	}
	sort.Strings(rows)
	return fmt.Sprintf("%v", rows)
}

func joinNonEmpty(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += ","
		}
		out += part
	}
	return out
}

// ApplyInteractionDecision writes the durable decision and its outbox item in
// ONE transaction. Idempotency and conflict rules:
//   - the same decision id with the same payload returns the stored result
//     unchanged (no revision bump, no duplicate outbox row);
//   - the same decision id with a different payload, or a stale expected
//     revision, answers ErrConflict (409);
//   - an already decided or canceled interaction is terminal: ErrGone (410);
//   - a question is never approvable (DecisionAllowed, defense in depth).
func (s *GormCraftInteractionStore) ApplyInteractionDecision(ctx context.Context, req CraftDecisionRequest) (CraftInteractionRecord, error) {
	if req.InteractionID == "" || req.DecisionID == "" || req.Action == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction, decision and action are required", craft.ErrInvalidInput)
	}
	var out CraftInteractionRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row craftInteractionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ?", req.Scope.TenantID, req.InteractionID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: interaction %s", craft.ErrNotFound, req.InteractionID)
			}
			return err
		}
		if row.SessionID != req.Scope.SessionID {
			return fmt.Errorf("%w: interaction %s", craft.ErrNotFound, req.InteractionID)
		}
		if row.OwnerID != req.Scope.UserID {
			return fmt.Errorf("%w: interaction %s", craft.ErrForbidden, req.InteractionID)
		}
		if !craft.DecisionAllowed(row.Kind, req.Action) {
			return fmt.Errorf("%w: action %q is not a user decision for a %s", craft.ErrInvalidInput, req.Action, row.Kind)
		}
		// Idempotent replay of the exact same decision.
		if row.DecisionID == req.DecisionID && row.Status == "decided" {
			var stored craftOutboxRow
			if err := tx.Where("tenant_id = ? AND interaction_id = ? AND id = ?",
				req.Scope.TenantID, req.InteractionID, req.DecisionID).Take(&stored).Error; err == nil {
				if stored.PayloadHash != decisionPayloadHash(req) {
					return fmt.Errorf("%w: decision %s was recorded with a different payload", craft.ErrConflict, req.DecisionID)
				}
			}
			view, verr := row.view()
			if verr != nil {
				return verr
			}
			out = view
			return nil
		}
		if row.Status != "pending" {
			return fmt.Errorf("%w: interaction %s is %s", craft.ErrGone, req.InteractionID, row.Status)
		}
		if req.ArgsHash == "" || req.ArgsHash != row.ArgsHash {
			return fmt.Errorf("%w: interaction %s arguments changed since the user decided", craft.ErrConflict, req.InteractionID)
		}
		if req.ExpectedRevision != row.Revision {
			return fmt.Errorf("%w: interaction %s revision %d is not current %d",
				craft.ErrConflict, req.InteractionID, req.ExpectedRevision, row.Revision)
		}

		var answersJSON *string
		if len(req.Answers) > 0 {
			encoded, err := json.Marshal(req.Answers)
			if err != nil {
				return err
			}
			value := string(encoded)
			answersJSON = &value
		}
		updated := tx.Model(&craftInteractionRow{}).
			Where("tenant_id = ? AND id = ? AND revision = ?", req.Scope.TenantID, req.InteractionID, row.Revision).
			Updates(map[string]any{
				"status":         "decided",
				"delivery":       "pending",
				"decision_id":    req.DecisionID,
				"decided_action": req.Action,
				"answers_json":   answersJSON,
				"decided_by":     req.Operator,
				"revision":       gorm.Expr("revision + 1"),
				"updated_at":     gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("%w: interaction %s changed concurrently", craft.ErrConflict, req.InteractionID)
		}
		// The outbox item joins the decision in the SAME transaction: a crash
		// between the two writes is impossible.
		pending := ""
		if answersJSON != nil {
			pending = *answersJSON
		}
		outbox := craftOutboxRow{
			TenantID: req.Scope.TenantID, InteractionID: req.InteractionID, ID: req.DecisionID,
			RunID: row.RunID, PendingID: row.PendingID, Kind: row.Kind, Action: req.Action,
			AnswersJSON: pending, ArgsHash: req.ArgsHash, ExpectedRevision: row.Revision,
			PayloadHash: decisionPayloadHash(req), Delivery: "pending", DecidedBy: req.Operator,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&outbox).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND id = ?", req.Scope.TenantID, req.InteractionID).Take(&row).Error; err != nil {
			return err
		}
		view, verr := row.view()
		if verr != nil {
			return verr
		}
		out = view
		return nil
	})
	if err != nil {
		return CraftInteractionRecord{}, err
	}
	logger.Infof(ctx, "[CraftInteraction] decision %s on %s by %s (scope %d/%s) recorded",
		req.DecisionID, req.InteractionID, req.Operator, req.Scope.TenantID, req.Scope.SessionID)
	return out, nil
}

// MarkInteractionDelivery moves the interaction-level delivery marker.
func (s *GormCraftInteractionStore) MarkInteractionDelivery(ctx context.Context, scope craft.Scope, id, delivery string) error {
	switch delivery {
	case "pending", "delivered", "unknown":
	default:
		return fmt.Errorf("%w: delivery %q", craft.ErrInvalidInput, delivery)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.getRow(tx, scope, id); err != nil {
			return err
		}
		return tx.Model(&craftInteractionRow{}).
			Where("tenant_id = ? AND id = ?", scope.TenantID, id).
			Updates(map[string]any{"delivery": delivery, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
	})
}

type craftOutboxRow struct {
	TenantID         uint64     `gorm:"column:tenant_id"`
	OwnerID          string     `gorm:"->;column:owner_id"`
	SessionID        string     `gorm:"->;column:session_id"`
	InteractionID    string     `gorm:"column:interaction_id"`
	ID               string     `gorm:"column:id"`
	RunID            string     `gorm:"column:run_id"`
	PendingID        string     `gorm:"column:pending_id"`
	Kind             string     `gorm:"column:kind"`
	Action           string     `gorm:"column:action"`
	AnswersJSON      string     `gorm:"column:answers_json"`
	ArgsHash         string     `gorm:"column:args_hash"`
	ExpectedRevision int64      `gorm:"column:expected_revision"`
	PayloadHash      string     `gorm:"column:payload_hash"`
	Delivery         string     `gorm:"column:delivery"`
	Attempts         int        `gorm:"column:attempts"`
	LastError        *string    `gorm:"column:last_error"`
	DecidedBy        string     `gorm:"column:decided_by"`
	OcSessionID      string     `gorm:"column:oc_session_id"`
	OcRequestID      string     `gorm:"column:oc_request_id"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	DeliveredAt      *time.Time `gorm:"column:delivered_at"`
}

func (craftOutboxRow) TableName() string { return "craft_decision_outbox" }

func (r craftOutboxRow) item() CraftOutboxItem {
	item := CraftOutboxItem{
		TenantID: r.TenantID, OwnerID: r.OwnerID, SessionID: r.SessionID,
		InteractionID: r.InteractionID, PendingID: r.PendingID,
		DecisionID: r.ID, RunID: r.RunID, Kind: r.Kind, Action: r.Action,
		ArgsHash: r.ArgsHash, ExpectedRevision: r.ExpectedRevision,
		Delivery: r.Delivery, Attempts: r.Attempts, DecidedBy: r.DecidedBy,
		OpenCodeSessionID: r.OcSessionID, OpenCodeRequestID: r.OcRequestID,
	}
	if r.AnswersJSON != "" {
		_ = json.Unmarshal([]byte(r.AnswersJSON), &item.Answers)
	}
	return item
}

// PendingDecisions returns the run's undelivered (pending or unknown) outbox
// items, joined with the interaction's OpenCode addresses so the delivery
// worker can forward to the bound request id.
func (s *GormCraftInteractionStore) PendingDecisions(ctx context.Context, key agentruntime.RunKey) ([]CraftOutboxItem, error) {
	var rows []craftOutboxRow
	err := s.db.WithContext(ctx).
		Select("craft_decision_outbox.*, "+
			"craft_interactions.oc_session_id AS oc_session_id, "+
			"craft_interactions.oc_request_id AS oc_request_id, "+
			"craft_interactions.owner_id AS owner_id, "+
			"craft_interactions.session_id AS session_id").
		Where("craft_decision_outbox.tenant_id = ? AND craft_decision_outbox.run_id = ? AND craft_decision_outbox.delivery IN (?, ?)",
			key.TenantID, key.RunID, "pending", "unknown").
		Joins("JOIN craft_interactions ON craft_interactions.tenant_id = craft_decision_outbox.tenant_id AND craft_interactions.id = craft_decision_outbox.interaction_id").
		Order("craft_decision_outbox.created_at, craft_decision_outbox.id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]CraftOutboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.item())
	}
	return items, nil
}

// PendingDecisionRuns lists the run keys that still carry undelivered
// (pending) decisions — the sweep loop's work list.
func (s *GormCraftInteractionStore) PendingDecisionRuns(ctx context.Context) ([]agentruntime.RunKey, error) {
	var rows []struct {
		TenantID uint64 `gorm:"column:tenant_id"`
		RunID    string `gorm:"column:run_id"`
	}
	err := s.db.WithContext(ctx).
		Table("craft_decision_outbox").
		Select("DISTINCT tenant_id, run_id").
		Where("delivery = ?", "pending").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	keys := make([]agentruntime.RunKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, agentruntime.RunKey{TenantID: row.TenantID, RunID: row.RunID})
	}
	return keys, nil
}

// AckDecision marks one outbox item delivered (and the interaction with it).
func (s *GormCraftInteractionStore) AckDecision(ctx context.Context, key agentruntime.RunKey, interactionID, decisionID, note string) error {
	return s.markOutbox(ctx, key, interactionID, decisionID, "delivered", note)
}

// MarkDecisionUnknown keeps an unconfirmable delivery honest: the decision is
// durable, the delivery is not confirmed (delivery_unknown).
func (s *GormCraftInteractionStore) MarkDecisionUnknown(ctx context.Context, key agentruntime.RunKey, interactionID, decisionID, note string) error {
	return s.markOutbox(ctx, key, interactionID, decisionID, "unknown", note)
}

func (s *GormCraftInteractionStore) markOutbox(ctx context.Context, key agentruntime.RunKey, interactionID, decisionID, state, note string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"delivery":   state,
			"attempts":   gorm.Expr("attempts + 1"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}
		if note != "" {
			updates["last_error"] = note
		}
		if state == "delivered" {
			updates["delivered_at"] = gorm.Expr("CURRENT_TIMESTAMP")
		}
		result := tx.Model(&craftOutboxRow{}).
			Where("tenant_id = ? AND interaction_id = ? AND id = ? AND run_id = ?",
				key.TenantID, interactionID, decisionID, key.RunID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if state == "delivered" || state == "unknown" {
			return tx.Model(&craftInteractionRow{}).
				Where("tenant_id = ? AND id = ?", key.TenantID, interactionID).
				Updates(map[string]any{"delivery": state, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
		}
		return nil
	})
}
