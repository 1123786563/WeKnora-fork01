package career

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Frozen usage admission contract (T21). Exactly one operation is charged:
// a search_once run — including every period a recurring rule triggers,
// because a trigger is one search_once under the hood. Everything else
// (profile intake, JD import, evaluation, application tracking, materials,
// preparations, reminders, and every read) is free and stays free: those
// paths are local and deterministic, so charging for them would be a lie.
const (
	// UsageOperationSearchOnce is the only charged operation (frozen).
	UsageOperationSearchOnce = "search_once"

	// searchOnceCostUnits is the frozen cost of one charged run.
	searchOnceCostUnits = int64(1)

	// defaultSearchQuotaLimitUnits is the frozen monthly allowance used when
	// the environment names none. The spec keeps concrete numbers out of the
	// behavior contract; this default is an operations constant only.
	defaultSearchQuotaLimitUnits = int64(50)

	// careerSearchQuotaLimitEnv names the deployment override for the
	// monthly allowance (a positive integer of units).
	careerSearchQuotaLimitEnv = "CAREER_SEARCH_QUOTA_LIMIT"

	usageKindEstimate = "usage_estimate"

	usageStatusReserved = "reserved"
	usageStatusSettled  = "settled"
	usageStatusReleased = "released"

	// usageReservationLease bounds how long an admitted-but-never-claimed
	// reservation can hold a unit before the reconcile path releases it.
	usageReservationLease = searchClaimLease
)

// ErrAdmissionUnavailable is the typed reason surfaced when the quota ledger
// cannot be read. It is fail-closed: a charged run is never executed first
// and reported afterwards.
var ErrAdmissionUnavailable = errors.New("career usage admission unavailable")

// UsageEstimateView is the frozen pre-execution estimate: the cost that the
// next charged run would consume, the conditions under which it is charged,
// and the live balance of the current window. It is a read-only projection.
type UsageEstimateView struct {
	Kind           string    `json:"kind"`
	Operation      string    `json:"operation"`
	CostUnits      int64     `json:"costUnits"`
	Conditions     []string  `json:"conditions"`
	PeriodStart    time.Time `json:"periodStart"`
	PeriodEnd      time.Time `json:"periodEnd"`
	LimitUnits     int64     `json:"limitUnits"`
	ReservedUnits  int64     `json:"reservedUnits"`
	SettledUnits   int64     `json:"settledUnits"`
	RemainingUnits int64     `json:"remainingUnits"`
	WouldAdmit     bool      `json:"wouldAdmit"`
}

// usageReservationRecord is one row of the durable quota ledger: a unit
// pre-reserved by request ID before a charged run executes, settled once the
// run turns terminal, or released when an admitted run never claimed its
// search and the lease expired. Every row is scoped from the authenticated
// context, never from client input.
type usageReservationRecord struct {
	ID          string `gorm:"primaryKey;size:36"`
	TenantID    uint64 `gorm:"uniqueIndex:career_usage_scope_request;index:idx_career_usage_scope"`
	UserID      string `gorm:"uniqueIndex:career_usage_scope_request;index:idx_career_usage_scope;size:512"`
	Operation   string `gorm:"size:32;not null"`
	RequestID   string `gorm:"uniqueIndex:career_usage_scope_request;size:128"`
	CostUnits   int64  `gorm:"not null"`
	Status      string `gorm:"size:16;not null;index:idx_career_usage_status"`
	PeriodStart time.Time
	PeriodEnd   time.Time
	LeaseUntil  *time.Time
	CreatedAt   time.Time
	SettledAt   *time.Time
}

func (usageReservationRecord) TableName() string { return "career_usage_reservations" }

// searchUsageGate is the production SearchQuotaGate: the real admission
// mechanism replacing the honest pass-through of T11. A refusal is typed
// (ErrSearchQuotaRefused, recoverable by replaying the same request ID once
// the window admits again); an unreadable ledger is fail-closed
// (ErrAdmissionUnavailable, never execute-first).
type searchUsageGate struct{ office *Office }

func (g searchUsageGate) AdmitSearch(ctx context.Context, s Scope, requestID, _ string) error {
	return g.office.admitSearchUsage(ctx, s, requestID)
}

// usagePeriod freezes the accounting window: the UTC calendar month that
// contains now.
func usagePeriod(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return start, end
}

// resolveSearchQuotaLimitFromEnv resolves the monthly allowance at startup:
// the deployment override when present and valid, otherwise the frozen
// default. An invalid override is a startup error, never a silent fallback.
func resolveSearchQuotaLimitFromEnv() (int64, error) {
	raw := strings.TrimSpace(os.Getenv(careerSearchQuotaLimitEnv))
	if raw == "" {
		return defaultSearchQuotaLimitUnits, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s value %q: expected a positive integer", careerSearchQuotaLimitEnv, raw)
	}
	return parsed, nil
}

// SetSearchQuotaLimit pins the monthly allowance. Tests use it to shrink the
// window; deployment uses the environment instead.
func (o *Office) SetSearchQuotaLimit(units int64) {
	if units > 0 {
		o.searchQuotaLimit = units
	}
}

func (o *Office) searchQuotaLimitUnits() int64 {
	if o.searchQuotaLimit > 0 {
		return o.searchQuotaLimit
	}
	return defaultSearchQuotaLimitUnits
}

// usageAdmissionConditions is the frozen, user-facing statement of when the
// quota is charged. The estimate always carries it verbatim.
func usageAdmissionConditions() []string {
	return []string{
		"额度在执行前预占：每个 search_once（含规则触发的周期 Run）执行前预占 1 个单位，并在执行前向你展示本预估",
		"预占以 requestId 幂等：同一 requestId 重放或重试不会重复预占或收费",
		"预占在搜索终态后结算；已预占但从未执行的请求在租约过期后自动释放，不占余额",
		"额度按 UTC 自然月重置；本期额度耗尽时只阻止新的收费 Run，既有档案、申请、评估与搜索记录永远可读",
		"付费状态不改变岗位排序或资格判定：评估与排序输入不含任何付费维度",
	}
}

// admitSearchUsage is the single admission seam for charged runs. It is
// idempotent by request ID: a replay of an admitted request never reserves a
// second unit. A refusal leaves no durable state.
func (o *Office) admitSearchUsage(ctx context.Context, s Scope, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return ErrInvalidRequest
	}
	if o.usageLedgerUnavailable() {
		return ErrAdmissionUnavailable
	}
	if err := o.reconcileUsage(ctx, s); err != nil {
		return ErrAdmissionUnavailable
	}
	var existing usageReservationRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&existing).Error
	if err == nil {
		switch existing.Status {
		case usageStatusReserved, usageStatusSettled:
			// The request ID was already admitted (possibly by the rule
			// trigger's pre-check): replaying admission never charges twice.
			return nil
		case usageStatusReleased:
			return o.reviveReleasedReservation(ctx, s, existing)
		}
		return ErrAdmissionUnavailable
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAdmissionUnavailable
	}
	now := time.Now().UTC()
	start, end := usagePeriod(now)
	reserved, settled, totalsErr := o.usageTotals(ctx, s, start)
	if totalsErr != nil {
		return ErrAdmissionUnavailable
	}
	if o.searchQuotaLimitUnits()-reserved-settled < searchOnceCostUnits {
		return ErrSearchQuotaRefused
	}
	lease := now.Add(usageReservationLease)
	row := usageReservationRecord{
		ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
		Operation: UsageOperationSearchOnce, RequestID: requestID, CostUnits: searchOnceCostUnits,
		Status: usageStatusReserved, PeriodStart: start, PeriodEnd: end,
		LeaseUntil: &lease, CreatedAt: now,
	}
	if err = o.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isReceiptRaceError(err) {
			// A concurrent admission for the same request ID won the insert:
			// the ledger already holds exactly one reservation for it.
			return nil
		}
		return ErrAdmissionUnavailable
	}
	return nil
}

// reviveReleasedReservation re-admits a request ID whose earlier reservation
// was released (the run never claimed its search and the lease expired). The
// same row is re-armed so one request ID always owns at most one row.
func (o *Office) reviveReleasedReservation(ctx context.Context, s Scope, existing usageReservationRecord) error {
	now := time.Now().UTC()
	start, end := usagePeriod(now)
	reserved, settled, totalsErr := o.usageTotals(ctx, s, start)
	if totalsErr != nil {
		return ErrAdmissionUnavailable
	}
	if o.searchQuotaLimitUnits()-reserved-settled < existing.CostUnits {
		return ErrSearchQuotaRefused
	}
	lease := now.Add(usageReservationLease)
	if err := o.db.WithContext(ctx).Model(&usageReservationRecord{}).
		Where("id=? AND status=?", existing.ID, usageStatusReleased).
		Updates(map[string]any{
			"status": usageStatusReserved, "period_start": start, "period_end": end,
			"lease_until": lease, "settled_at": nil,
		}).Error; err != nil {
		return ErrAdmissionUnavailable
	}
	return nil
}

// usageLedgerUnavailable is the fail-closed injection seam for tests: a
// unreadable ledger never lets a charged run execute first.
func (o *Office) usageLedgerUnavailable() bool {
	if o.failUsageLedgerRead == nil {
		return false
	}
	return o.failUsageLedgerRead() != nil
}

// reconcileUsage drives the reserve→settle/release lifecycle lazily on the
// read paths. A reservation whose search turned terminal is settled; a
// reservation whose search was never claimed is released once its lease
// expired, so a failed attempt never strands the quota. Reconcile is
// idempotent and conservative: reserved units keep counting until they are
// provably settled or released.
func (o *Office) reconcileUsage(ctx context.Context, s Scope) error {
	var pending []usageReservationRecord
	if err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND status=?", s.TenantID, s.UserID, usageStatusReserved).
		Find(&pending).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range pending {
		var search searchRecord
		err := o.db.WithContext(ctx).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, row.RequestID).
			First(&search).Error
		switch {
		case err == nil:
			receipt, _, decodeErr := decodeSearchBody(search.ReceiptBody)
			if decodeErr != nil || receipt == nil {
				// The search is still claiming or in flight: the reserved
				// unit keeps holding until the run turns terminal.
				continue
			}
			if updateErr := o.db.WithContext(ctx).Model(&usageReservationRecord{}).
				Where("id=? AND status=?", row.ID, usageStatusReserved).
				Updates(map[string]any{"status": usageStatusSettled, "settled_at": now, "lease_until": nil}).Error; updateErr != nil {
				return updateErr
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if row.LeaseUntil == nil || !row.LeaseUntil.Before(now) {
				continue
			}
			if updateErr := o.db.WithContext(ctx).Model(&usageReservationRecord{}).
				Where("id=? AND status=?", row.ID, usageStatusReserved).
				Updates(map[string]any{"status": usageStatusReleased, "lease_until": nil}).Error; updateErr != nil {
				return updateErr
			}
		default:
			return err
		}
	}
	return nil
}

// usageTotals sums the units held in the given accounting window. Released
// rows never count; reserved rows count conservatively until reconciled.
func (o *Office) usageTotals(ctx context.Context, s Scope, periodStart time.Time) (reserved, settled int64, err error) {
	var rows []struct {
		Status string
		Total  int64
	}
	if err = o.db.WithContext(ctx).Model(&usageReservationRecord{}).
		Select("status, SUM(cost_units) AS total").
		Where("tenant_id=? AND user_id=? AND period_start=? AND status IN ?", s.TenantID, s.UserID, periodStart, []string{usageStatusReserved, usageStatusSettled}).
		Group("status").Scan(&rows).Error; err != nil {
		return 0, 0, err
	}
	for _, row := range rows {
		switch row.Status {
		case usageStatusReserved:
			reserved += row.Total
		case usageStatusSettled:
			settled += row.Total
		}
	}
	return reserved, settled, nil
}

// UsageEstimate answers, before anything executes, what the next charged run
// of one operation would consume and under which conditions, together with
// the live balance of the current window. An unreadable ledger is a typed
// refusal (ErrAdmissionUnavailable); an unknown operation is ErrInvalidRequest.
// The estimate itself is a free read and never mutates the ledger.
func (o *Office) UsageEstimate(ctx context.Context, operation string) (UsageEstimateView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return UsageEstimateView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return UsageEstimateView{}, err
	}
	operation = strings.TrimSpace(operation)
	if operation != UsageOperationSearchOnce {
		return UsageEstimateView{}, ErrInvalidRequest
	}
	if o.usageLedgerUnavailable() {
		return UsageEstimateView{}, ErrAdmissionUnavailable
	}
	if err = o.reconcileUsage(ctx, s); err != nil {
		return UsageEstimateView{}, ErrAdmissionUnavailable
	}
	now := time.Now().UTC()
	start, end := usagePeriod(now)
	reserved, settled, totalsErr := o.usageTotals(ctx, s, start)
	if totalsErr != nil {
		return UsageEstimateView{}, ErrAdmissionUnavailable
	}
	limit := o.searchQuotaLimitUnits()
	remaining := limit - reserved - settled
	return UsageEstimateView{
		Kind: usageKindEstimate, Operation: UsageOperationSearchOnce,
		CostUnits: searchOnceCostUnits, Conditions: usageAdmissionConditions(),
		PeriodStart: start, PeriodEnd: end,
		LimitUnits: limit, ReservedUnits: reserved, SettledUnits: settled,
		RemainingUnits: remaining, WouldAdmit: remaining >= searchOnceCostUnits,
	}, nil
}
