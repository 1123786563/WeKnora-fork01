package appconnector

import (
	"context"
	"errors"
)

// Pause reasons for scoped connector sync (A07). Each reason is a stable
// machine-readable string persisted on the sync log so the UI can explain why
// a team sync stopped and what the operator must change. Resume after any of
// them continues from the persisted cursor — a pause never discards progress.
const (
	// PauseReasonBudget marks the space budget gate denying a new hold.
	PauseReasonBudget = "budget"
	// PauseReasonPlan marks an expired/canceled plan pausing NEW sync
	// dispatch while already-persisted settlements still complete.
	PauseReasonPlan = "plan"
	// PauseReasonPermission marks a missing/revoked space grant or a
	// requires_reauthorization connection.
	PauseReasonPermission = "permission"
	// PauseReasonQuota marks an exhausted quota dimension.
	PauseReasonQuota = "quota"
	// PauseReasonProviderLimit marks a provider-side rate/limit rejection.
	PauseReasonProviderLimit = "provider_limit"
)

// validPauseReasons is the closed vocabulary of sync pause reasons.
var validPauseReasons = map[string]bool{
	PauseReasonBudget:        true,
	PauseReasonPlan:          true,
	PauseReasonPermission:    true,
	PauseReasonQuota:         true,
	PauseReasonProviderLimit: true,
}

// IsValidPauseReason reports whether r belongs to the pause-reason vocabulary.
func IsValidPauseReason(r string) bool { return validPauseReasons[r] }

// SyncPausedError stops further sync dispatch without treating the pause as a
// data-source failure: the persisted cursor stays untouched and the reason is
// surfaced to the operator.
type SyncPausedError struct {
	Reason  string
	Message string
}

// NewSyncPausedError builds a typed pause error with a validated reason.
func NewSyncPausedError(reason, message string) error {
	if !IsValidPauseReason(reason) {
		reason = PauseReasonBudget
	}
	return &SyncPausedError{Reason: reason, Message: message}
}

func (e *SyncPausedError) Error() string {
	if e.Message == "" {
		return "sync paused: " + e.Reason
	}
	return "sync paused (" + e.Reason + "): " + e.Message
}

// SyncBinding is the resolved authorization a team sync executes under: which
// installation and space connection the writes are attributed to, at which
// credential version. The cursor/checkpoint of a data source is only valid for
// the AuthVersion it was produced with.
type SyncBinding struct {
	TenantID       uint64
	InstallationID string
	ConnectionID   string
	DataSourceID   string
	AuthVersion    int64
	// RequiresReauthorization is set when the stored credentials cannot prove
	// space ownership (legacy pre-appconnector credentials) or the connection
	// or installation is no longer usable for space execution. Such data
	// sources keep their local content and resume only after re-consent.
	RequiresReauthorization bool
}

// StoredSyncBinding is the persisted row shape of the app_datasource_bindings
// relation (A01 relation table, migrations PG 000120 / SQLite 000040).
type StoredSyncBinding struct {
	TenantID       uint64
	DataSourceID   string
	InstallationID string
	ConnectionID   string
	AuthVersion    int64
}

// ErrSyncBindingNotFound reports that a data source has no row in
// app_datasource_bindings (never connected through an app installation).
var ErrSyncBindingNotFound = errors.New("sync_binding_not_found")

// SyncBindingStore reads the app_datasource_bindings relation.
type SyncBindingStore interface {
	FindSyncBinding(ctx context.Context, tenantID uint64, dataSourceID string) (*StoredSyncBinding, error)
}

// BindingState is the live state of the bound installation/connection at
// resolution time. A nil state means only legacy credentials exist and
// personal/space ownership cannot be proven.
type BindingState struct {
	ConnectionState   string
	ConnectionKind    string
	InstallationState string
	ConnectionAuthVer int64
}

// ResolveSyncBinding loads the app_datasource_bindings row for the data source
// and stamps it with the live authorization state. Rules:
//   - no relation row  → ErrSyncBindingNotFound (the data source keeps the
//     legacy execution path);
//   - nil state (legacy credentials that cannot prove personal/space
//     ownership) → binding resolved but RequiresReauthorization = true;
//   - anything other than an active space connection on an active
//     installation → RequiresReauthorization = true. Team sync writes and
//     scheduling execute with a space connection, never a personal one.
//
// The returned AuthVersion is the live connection auth version when known and
// falls back to the persisted binding version for legacy rows.
func ResolveSyncBinding(
	ctx context.Context, store SyncBindingStore, tenantID uint64, dataSourceID string, state *BindingState,
) (SyncBinding, error) {
	row, err := store.FindSyncBinding(ctx, tenantID, dataSourceID)
	if err != nil {
		return SyncBinding{}, err
	}
	if row == nil {
		return SyncBinding{}, ErrSyncBindingNotFound
	}
	binding := SyncBinding{
		TenantID:       row.TenantID,
		InstallationID: row.InstallationID,
		ConnectionID:   row.ConnectionID,
		DataSourceID:   row.DataSourceID,
		AuthVersion:    row.AuthVersion,
	}
	if state == nil {
		// Legacy credentials: ownership cannot be proven, so the binding is
		// resolved for bookkeeping but must not authorize new team syncs.
		binding.RequiresReauthorization = true
		return binding, nil
	}
	if state.ConnectionAuthVer > 0 {
		binding.AuthVersion = state.ConnectionAuthVer
	}
	binding.RequiresReauthorization = !bindingAuthorizesSpaceSync(state)
	return binding, nil
}

// bindingAuthorizesSpaceSync reports whether the live states authorize team
// sync execution through a space connection on an active installation.
func bindingAuthorizesSpaceSync(state *BindingState) bool {
	return state.ConnectionKind == ConnectionKindSpace &&
		state.ConnectionState == ConnectionActive &&
		(state.InstallationState == InstallationActive)
}

// CanAdvanceCheckpoint reports whether a sync worker may persist a new cursor.
// The content (and its index-task handoff) must already be durable so the
// checkpoint can never skip past uncommitted work, and the worker must still
// hold the lease fence — a stale worker that lost its lease must not overwrite
// the cursor of the current holder.
func CanAdvanceCheckpoint(contentCommitted bool, currentFence, workerFence int64) bool {
	return contentCommitted && currentFence == workerFence
}

// ShouldPauseNewSync reports whether plan expiry must pause NEW sync dispatch.
func ShouldPauseNewSync(planActive bool) bool { return !planActive }

// MaySettlePersistedSync reports whether already-persisted settlements may
// still complete while new syncs are paused: a plan expiry never strands
// durable reservations — the settlement of work already handed off proceeds.
func MaySettlePersistedSync(planActive bool) bool { return true }

// SourceFailureKind classifies why a provider fetch failed. The kinds drive
// different operator actions and, critically, never delete local content.
type SourceFailureKind string

const (
	// SourceFailureNotFound is an HTTP 404: the resource vanished between
	// listing and fetching. Local copy is kept; the next full sync reconciles.
	SourceFailureNotFound SourceFailureKind = "source_not_found"
	// SourceFailurePermission is an HTTP 401/403: the space connection lost
	// access. Local copy is kept; the operator re-authorizes the connection.
	SourceFailurePermission SourceFailureKind = "source_permission_denied"
	// SourceFailureUnavailable is any other provider error (5xx, rate limit).
	SourceFailureUnavailable SourceFailureKind = "source_unavailable"
)

// ClassifySourceFailure maps a provider HTTP status onto a failure kind,
// distinguishing 404 (gone) from permission denials from transient errors.
func ClassifySourceFailure(statusCode int) SourceFailureKind {
	switch {
	case statusCode == 404 || statusCode == 410:
		return SourceFailureNotFound
	case statusCode == 401 || statusCode == 403:
		return SourceFailurePermission
	default:
		return SourceFailureUnavailable
	}
}

// LocalCopyPreserved reports whether the local KB copy of an item survives a
// sync observation. A fetch failure (404, permission denied, unavailable)
// always keeps it — only an explicit source-side deletion notice combined with
// the operator's sync-deletions opt-in may remove local content.
func LocalCopyPreserved(sourceReportsDeletion bool, syncDeletionsEnabled bool) bool {
	return !(sourceReportsDeletion && syncDeletionsEnabled)
}
