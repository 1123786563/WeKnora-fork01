package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Reminders (T20) are the in-station todos of the personal career space. The
// durable todo row is the authoritative reminder fact (spec §5 跟进：站内待办
// 是提醒事实源); every push channel is a best-effort re-notification that can
// neither fail the write nor mutate the todo. Push bodies are drawn exclusively
// from the frozen template table below — company, job, and interview detail
// never leave the space (spec §7：通知正文不包含公司、岗位或面试详情).
const (
	ReminderKindSet = "reminder_set"

	// ReminderSourceProgressEvent binds a todo to one T17 progress event.
	ReminderSourceProgressEvent = "progress_event"
	// ReminderSourceDiscovery binds a todo to one T13 discovery todo.
	ReminderSourceDiscovery = "discovery"

	ReminderStatusOpen = "open"

	// Frozen notice template identities.
	ReminderNoticeProgressUpdated = "progress_updated"
	ReminderNoticeDiscoveryFound  = "discovery_found"

	// ReminderPushFactKey is the durable profile fact carrying the push
	// subscription choice. Absence means subscribed; only the exact
	// unsubscribed value opts out. It is written through the house confirm
	// flow (request ID + expected revision), never through a side channel.
	ReminderPushFactKey           = "notifications.push"
	ReminderPushSubscribedValue   = "subscribed"
	ReminderPushUnsubscribedValue = "unsubscribed"

	// Frozen push report reasons.
	ReminderPushReasonUnsubscribed   = "unsubscribed"
	ReminderPushReasonDeliveryFailed = "delivery_failed"

	maxReminderRequestIDLen = 128
	maxReminderSourceIDLen  = 36
)

var (
	// ErrReminderNotFound answers a reminder reference that does not exist
	// under the authenticated scope (foreign owners included).
	ErrReminderNotFound = errors.New("career reminder not found")
	// ErrReminderSourceNotFound answers a source event the scope cannot see;
	// unknown and foreign IDs are indistinguishable by design.
	ErrReminderSourceNotFound = errors.New("career reminder source not found")
)

// ReminderNoticeBodies is the frozen privacy template table. These literals
// are the complete push vocabulary — the in-station detail stays behind the
// authenticated API and is never interpolated into a push body.
var ReminderNoticeBodies = map[string]string{
	ReminderNoticeProgressUpdated: "你有新的求职进展，请登录查看。",
	ReminderNoticeDiscoveryFound:  "持续找岗有新发现，请登录查看。",
}

// reminderNoticeKeys maps the closed source enum onto its frozen template.
var reminderNoticeKeys = map[string]string{
	ReminderSourceProgressEvent: ReminderNoticeProgressUpdated,
	ReminderSourceDiscovery:     ReminderNoticeDiscoveryFound,
}

// ReminderNotice is everything a push channel ever receives: the frozen
// template identity and its literal body.
type ReminderNotice struct {
	TemplateKey string `json:"templateKey"`
	Body        string `json:"body"`
}

// ReminderNotifier is the injected push seam (T20). Production wires nil —
// the in-station todo alone is complete and authoritative; tests inject
// fakes. A non-nil error from Remind means the channel failed: it never
// fails the durable write and is never treated as a todo state update.
type ReminderNotifier interface {
	Remind(ctx context.Context, scope Scope, notice ReminderNotice) error
}

// SetReminderInput is the closed set_reminder intent: which source event the
// todo binds to, under the house request ID + expected revision discipline.
type SetReminderInput struct {
	RequestID        string `json:"requestId"`
	SourceKind       string `json:"sourceKind"`
	SourceID         string `json:"sourceId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// ReminderPushReport is the ephemeral, best-effort outcome of one push
// attempt. It is attached to the HTTP response only; the durable receipt and
// the todo row never record push state.
type ReminderPushReport struct {
	Attempted bool   `json:"attempted"`
	Delivered bool   `json:"delivered"`
	Reason    string `json:"reason,omitempty"`
}

// ReminderReceipt is the frozen durable receipt of one set_reminder call and
// the contract served by the receipt endpoint. Push is response-only.
type ReminderReceipt struct {
	Kind          string              `json:"kind"`
	RequestID     string              `json:"requestId"`
	ReminderID    string              `json:"reminderId"`
	SourceKind    string              `json:"sourceKind"`
	SourceID      string              `json:"sourceId"`
	Deduplicated  bool                `json:"deduplicated"`
	ApplicationID string              `json:"applicationId,omitempty"`
	OpportunityID string              `json:"opportunityId,omitempty"`
	NoticeKey     string              `json:"noticeKey"`
	Notice        string              `json:"notice"`
	Status        string              `json:"status"`
	Revision      uint64              `json:"revision"`
	CreatedAt     time.Time           `json:"createdAt"`
	Push          *ReminderPushReport `json:"push,omitempty"`
}

// ReminderView is one in-station todo on the authoritative reading surface.
// The notice is the same frozen literal the push channel would carry; every
// richer detail is reached through the referenced application/rule views,
// which require the authenticated session.
type ReminderView struct {
	ReminderID    string    `json:"reminderId"`
	SourceKind    string    `json:"sourceKind"`
	SourceID      string    `json:"sourceId"`
	ApplicationID string    `json:"applicationId,omitempty"`
	OpportunityID string    `json:"opportunityId,omitempty"`
	NoticeKey     string    `json:"noticeKey"`
	Notice        string    `json:"notice"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
}

// reminderRecord is the authoritative in-station todo. The composite unique
// index (tenant, user, source_kind, source_id) is the deterministic
// idempotency key: the same opportunity event — however often re-triggered —
// holds exactly one todo. The row deliberately carries no push columns: a
// push is a reminder, never a status update.
type reminderRecord struct {
	ID            string `gorm:"primaryKey;size:36"`
	TenantID      uint64 `gorm:"uniqueIndex:career_reminder_scope_source,priority:1;uniqueIndex:career_reminder_scope_request,priority:1;index:idx_career_reminder_scope,priority:1"`
	UserID        string `gorm:"uniqueIndex:career_reminder_scope_source,priority:2;uniqueIndex:career_reminder_scope_request,priority:2;index:idx_career_reminder_scope,priority:2;size:512"`
	SourceKind    string `gorm:"size:32;not null;uniqueIndex:career_reminder_scope_source,priority:3"`
	SourceID      string `gorm:"size:36;not null;uniqueIndex:career_reminder_scope_source,priority:4"`
	ApplicationID string `gorm:"size:36;not null;default:''"`
	OpportunityID string `gorm:"size:36;not null;default:''"`
	NoticeKey     string `gorm:"size:64;not null"`
	Status        string `gorm:"size:16;not null;default:'open';index:idx_career_reminder_scope,priority:3"`
	RequestID     string `gorm:"size:128;not null;uniqueIndex:career_reminder_scope_request,priority:3"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (reminderRecord) TableName() string { return "career_reminders" }

// reminderReceiptRecord stores one durable receipt per (scope, request ID) —
// including dedupe hits, so a request ID can never change intent afterwards.
type reminderReceiptRecord struct {
	TenantID    uint64 `gorm:"uniqueIndex:career_reminder_receipt_scope_request"`
	UserID      string `gorm:"uniqueIndex:career_reminder_receipt_scope_request;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_reminder_receipt_scope_request;size:128"`
	Fingerprint string `gorm:"size:64;not null"`
	Body        string `gorm:"type:text;not null"`
	CreatedAt   time.Time
}

func (reminderReceiptRecord) TableName() string { return "career_reminder_receipts" }

// SetReminderNotifier wires the push seam. Passing nil (the production
// default) keeps the office push-free: todos alone are produced and read.
func (o *Office) SetReminderNotifier(notifier ReminderNotifier) { o.reminderNotifier = notifier }

func validateReminderIntent(input SetReminderInput) error {
	if input.RequestID == "" || len(input.RequestID) > maxReminderRequestIDLen {
		return ErrInvalidRequest
	}
	if _, ok := reminderNoticeKeys[input.SourceKind]; !ok {
		return ErrInvalidRequest
	}
	if input.SourceID == "" || len(input.SourceID) > maxReminderSourceIDLen {
		return ErrInvalidRequest
	}
	return nil
}

// SetReminder produces exactly one in-station todo for one source event. The
// write follows the house discipline: request-ID replay/conflict, expected
// revision CAS on the profile head, scope from the authenticated context,
// and typed unknown-outcome recovery. Push happens only after the commit and
// only for a newly created todo.
func (o *Office) SetReminder(ctx context.Context, input SetReminderInput) (ReminderReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ReminderReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.SourceID = strings.TrimSpace(input.SourceID)
	if err = validateReminderIntent(input); err != nil {
		return ReminderReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		if isSQLiteBusy(err) {
			return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ReminderReceipt{}, err
	}
	fingerprint, err := materialFingerprint(ReminderKindSet, input.RequestID, input.SourceKind, input.SourceID, input.ExpectedRevision)
	if err != nil {
		return ReminderReceipt{}, err
	}
	if replay, found, lookupErr := o.replayReminderReceipt(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return ReminderReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	// Keep lock acquisition finite even when the HTTP client supplied no
	// deadline.
	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for attempt := 0; ; attempt++ {
		outcome, txErr := o.attemptReminderWrite(operationCtx, s, input, fingerprint)
		if txErr == nil {
			// The inbox fact is committed; the push is a post-commit,
			// best-effort reminder that can neither fail the write nor
			// mutate the todo.
			o.remindAfterCommit(ctx, s, &outcome.receipt, outcome.subscribed)
			return outcome.receipt, nil
		}
		if errors.Is(txErr, ErrIdempotencyConflict) || errors.Is(txErr, ErrInvalidRequest) ||
			errors.Is(txErr, ErrReminderSourceNotFound) || errors.Is(txErr, ErrApplicationNotFound) {
			return ReminderReceipt{}, txErr
		}
		var revisionConflict *RevisionConflictError
		ambiguous := errors.As(txErr, &revisionConflict) || isSQLiteBusy(txErr) || isReceiptRaceError(txErr)
		if ambiguous {
			// Never retry blindly with a new ID: the original request ID
			// decides whether the identical intent already committed.
			replay, found, lookupErr := o.replayReminderReceipt(operationCtx, s, input.RequestID, fingerprint)
			if lookupErr != nil {
				if operationCtx.Err() != nil || errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
					return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
				}
				return ReminderReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
			// A different request ID may have won the unique source key. Once
			// its todo is visible, bind this request to the same todo with a
			// durable deduplicated receipt of its own.
			if isSQLiteBusy(txErr) || isReceiptRaceError(txErr) {
				for reconcileAttempt := 0; reconcileAttempt < 20; reconcileAttempt++ {
					reconciled, reconciledFound, reconcileErr := o.reconcileReminderSource(operationCtx, s, input, fingerprint)
					if reconcileErr != nil {
						if operationCtx.Err() != nil || errors.Is(reconcileErr, context.Canceled) || errors.Is(reconcileErr, context.DeadlineExceeded) {
							return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
						}
						if !isSQLiteBusy(reconcileErr) && !isReceiptRaceError(reconcileErr) {
							return ReminderReceipt{}, reconcileErr
						}
					} else if reconciledFound {
						return reconciled, nil
					}
					if operationCtx.Err() != nil {
						return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
					}
					time.Sleep(progressWriteBackoff)
				}
			}
		}
		if operationCtx.Err() != nil || ctx.Err() != nil {
			return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		if isSQLiteBusy(txErr) && attempt < progressWriteBusyRetries {
			time.Sleep(progressWriteBackoff)
			continue
		}
		if isSQLiteBusy(txErr) {
			return ReminderReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ReminderReceipt{}, txErr
	}
}

type reminderWriteOutcome struct {
	receipt    ReminderReceipt
	subscribed bool
}

func (o *Office) reconcileReminderSource(ctx context.Context, s Scope, input SetReminderInput, fingerprint string) (ReminderReceipt, bool, error) {
	var receipt ReminderReceipt
	found := false
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize reconciliation with DeleteCareer's final profile lock and
		// purge. A source read before that lock could otherwise be followed by
		// deletion and then leave a new request receipt behind the deleted row.
		var head profile
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var stored reminderReceiptRecord
		err = tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).First(&stored).Error
		if err == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			if err = decodeReminderReceipt(stored.Body, &receipt); err == nil {
				found = true
			}
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireGateActiveTx(tx, s); err != nil {
			return err
		}
		var existing reminderRecord
		err = tx.Where("tenant_id=? AND user_id=? AND source_kind=? AND source_id=?", s.TenantID, s.UserID, input.SourceKind, input.SourceID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		receipt = reminderReceiptFromRow(existing, input.RequestID, true)
		body, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err = tx.Create(&reminderReceiptRecord{TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID, Fingerprint: fingerprint, Body: string(body), CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return ReminderReceipt{}, false, err
	}
	return receipt, found, nil
}

// attemptReminderWrite owns the single transaction: replay check, source
// resolution under the authenticated scope, deterministic dedupe against the
// existing todo, and — only for a genuinely new todo — the profile revision
// CAS plus the durable todo and receipt rows.
func (o *Office) attemptReminderWrite(ctx context.Context, s Scope, input SetReminderInput, fingerprint string) (reminderWriteOutcome, error) {
	var outcome reminderWriteOutcome
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Exact replay precedes every side effect: a stored request ID with
		// the same fingerprint returns the stored receipt, different content
		// conflicts.
		var stored reminderReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return decodeReminderReceipt(stored.Body, &outcome.receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		applicationID, opportunityID, noticeKey, resolveErr := resolveReminderSource(tx, s, input.SourceKind, input.SourceID)
		if resolveErr != nil {
			return resolveErr
		}
		// House expected-revision CAS against the profile head.
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		// Serialize same-source requests on the profile row before checking
		// the unique source key. A dedupe hit remains independent of a stale
		// profile revision, matching the sequential dedupe contract.
		var existing reminderRecord
		e = tx.Where("tenant_id=? AND user_id=? AND source_kind=? AND source_id=?",
			s.TenantID, s.UserID, input.SourceKind, input.SourceID).First(&existing).Error
		if e == nil {
			outcome.receipt = reminderReceiptFromRow(existing, input.RequestID, true)
			body, marshalErr := json.Marshal(outcome.receipt)
			if marshalErr != nil {
				return marshalErr
			}
			return tx.Create(&reminderReceiptRecord{
				TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
				Fingerprint: fingerprint, Body: string(body), CreatedAt: time.Now().UTC(),
			}).Error
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		now := time.Now().UTC()
		outcome.subscribed = reminderPushSubscribed(tx, s)
		outcome.receipt = ReminderReceipt{
			Kind:          ReminderKindSet,
			RequestID:     input.RequestID,
			ReminderID:    uuid.NewString(),
			SourceKind:    input.SourceKind,
			SourceID:      input.SourceID,
			ApplicationID: applicationID,
			OpportunityID: opportunityID,
			NoticeKey:     noticeKey,
			Notice:        ReminderNoticeBodies[noticeKey],
			Status:        ReminderStatusOpen,
			Revision:      head.Revision,
			CreatedAt:     now,
		}
		body, marshalErr := json.Marshal(outcome.receipt)
		if marshalErr != nil {
			return marshalErr
		}
		row := reminderRecord{
			ID: outcome.receipt.ReminderID, TenantID: s.TenantID, UserID: s.UserID,
			SourceKind: input.SourceKind, SourceID: input.SourceID,
			ApplicationID: applicationID, OpportunityID: opportunityID,
			NoticeKey: noticeKey, Status: ReminderStatusOpen,
			RequestID: input.RequestID, CreatedAt: now, UpdatedAt: now,
		}
		if e = tx.Create(&row).Error; e != nil {
			return e
		}
		return tx.Create(&reminderReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(body), CreatedAt: now,
		}).Error
	})
	if err != nil {
		return reminderWriteOutcome{}, err
	}
	return outcome, nil
}

// resolveReminderSource binds the todo to one visible source event and
// freezes its notice template. Cross-scope references simply do not resolve.
func resolveReminderSource(tx *gorm.DB, s Scope, sourceKind, sourceID string) (applicationID, opportunityID, noticeKey string, err error) {
	switch sourceKind {
	case ReminderSourceProgressEvent:
		var event progressEventRecord
		e := tx.Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, sourceID).First(&event).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return "", "", "", ErrReminderSourceNotFound
		}
		if e != nil {
			return "", "", "", e
		}
		var application applicationRecord
		e = tx.Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, event.ApplicationID).First(&application).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return "", "", "", ErrApplicationNotFound
		}
		if e != nil {
			return "", "", "", e
		}
		return event.ApplicationID, application.OpportunityID, ReminderNoticeProgressUpdated, nil
	case ReminderSourceDiscovery:
		var todo searchDiscoveryTodoRecord
		e := tx.Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, sourceID).First(&todo).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return "", "", "", ErrReminderSourceNotFound
		}
		if e != nil {
			return "", "", "", e
		}
		return "", "", ReminderNoticeDiscoveryFound, nil
	default:
		return "", "", "", ErrInvalidRequest
	}
}

// remindAfterCommit runs the best-effort push after the durable write. A
// dedupe hit never re-reminds; a missing channel reports nothing; an
// unsubscribed user reports the frozen reason; a failing channel reports the
// delivery failure without touching any durable state.
func (o *Office) remindAfterCommit(ctx context.Context, s Scope, receipt *ReminderReceipt, subscribed bool) {
	if receipt.Deduplicated {
		return
	}
	if o.reminderNotifier == nil {
		return
	}
	if !subscribed {
		receipt.Push = &ReminderPushReport{Attempted: false, Reason: ReminderPushReasonUnsubscribed}
		return
	}
	notice := ReminderNotice{TemplateKey: receipt.NoticeKey, Body: receipt.Notice}
	if err := o.reminderNotifier.Remind(ctx, s, notice); err != nil {
		receipt.Push = &ReminderPushReport{Attempted: true, Delivered: false, Reason: ReminderPushReasonDeliveryFailed}
		return
	}
	receipt.Push = &ReminderPushReport{Attempted: true, Delivered: true}
}

// reminderPushSubscribed reads the durable subscription fact. Absence (or an
// unreadable row) defaults to opted-in; only the exact unsubscribed value
// opts out.
func reminderPushSubscribed(db *gorm.DB, s Scope) bool {
	var row fact
	e := db.Where("tenant_id=? AND user_id=? AND key=?", s.TenantID, s.UserID, ReminderPushFactKey).First(&row).Error
	if e != nil {
		return true
	}
	return row.Value != ReminderPushUnsubscribedValue
}

func reminderReceiptFromRow(row reminderRecord, requestID string, deduplicated bool) ReminderReceipt {
	return ReminderReceipt{
		Kind:          ReminderKindSet,
		RequestID:     requestID,
		ReminderID:    row.ID,
		SourceKind:    row.SourceKind,
		SourceID:      row.SourceID,
		Deduplicated:  deduplicated,
		ApplicationID: row.ApplicationID,
		OpportunityID: row.OpportunityID,
		NoticeKey:     row.NoticeKey,
		Notice:        ReminderNoticeBodies[row.NoticeKey],
		Status:        row.Status,
		CreatedAt:     row.CreatedAt,
	}
}

// ListReminders serves the authoritative in-station todo list, newest first.
func (o *Office) ListReminders(ctx context.Context) ([]ReminderView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	var rows []reminderRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
		Order("created_at DESC").Order("id DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ReminderView, 0, len(rows))
	for _, row := range rows {
		out = append(out, ReminderView{
			ReminderID:    row.ID,
			SourceKind:    row.SourceKind,
			SourceID:      row.SourceID,
			ApplicationID: row.ApplicationID,
			OpportunityID: row.OpportunityID,
			NoticeKey:     row.NoticeKey,
			Notice:        ReminderNoticeBodies[row.NoticeKey],
			Status:        row.Status,
			CreatedAt:     row.CreatedAt,
		})
	}
	return out, nil
}

// FindReminderReceipt replays the stored reminder receipt by request ID under
// the authenticated scope.
func (o *Office) FindReminderReceipt(ctx context.Context, requestID string) (ReminderReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ReminderReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ReminderReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > maxReminderRequestIDLen {
		return ReminderReceipt{}, ErrInvalidRequest
	}
	var row reminderReceiptRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReminderReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return ReminderReceipt{}, err
	}
	var receipt ReminderReceipt
	if err = decodeReminderReceipt(row.Body, &receipt); err != nil {
		return ReminderReceipt{}, err
	}
	return receipt, nil
}

func (o *Office) replayReminderReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (ReminderReceipt, bool, error) {
	var row reminderReceiptRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReminderReceipt{}, false, nil
	}
	if err != nil {
		return ReminderReceipt{}, false, err
	}
	if fingerprint != "" && row.Fingerprint != fingerprint {
		return ReminderReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt ReminderReceipt
	if err = decodeReminderReceipt(row.Body, &receipt); err != nil {
		return ReminderReceipt{}, true, err
	}
	return receipt, true, nil
}

func decodeReminderReceipt(body string, receipt *ReminderReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career reminder receipt: %w", err)
	}
	return nil
}
