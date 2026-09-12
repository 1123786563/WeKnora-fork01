package appconnector

import (
	"context"
	"errors"
	"testing"
)

func TestSyncCheckpointWaitsForDurableContentAndFence(t *testing.T) {
	if CanAdvanceCheckpoint(false, 2, 2) {
		t.Fatal("lost uncommitted content")
	}
	if CanAdvanceCheckpoint(true, 2, 1) {
		t.Fatal("stale worker advanced cursor")
	}
	if !CanAdvanceCheckpoint(true, 2, 2) {
		t.Fatal("valid checkpoint refused")
	}
}

// fakeBindingStore serves the app_datasource_bindings relation from memory.
type fakeBindingStore struct {
	row *StoredSyncBinding
	err error
}

func (f *fakeBindingStore) FindSyncBinding(ctx context.Context, tenantID uint64, dataSourceID string) (*StoredSyncBinding, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.row == nil {
		return nil, ErrSyncBindingNotFound
	}
	return f.row, nil
}

func TestResolveSyncBindingActiveSpaceConnection(t *testing.T) {
	store := &fakeBindingStore{row: &StoredSyncBinding{
		TenantID:       7,
		DataSourceID:   "ds-1",
		InstallationID: "inst-1",
		ConnectionID:   "conn-1",
		AuthVersion:    3,
	}}
	binding, err := ResolveSyncBinding(context.Background(), store, 7, "ds-1", &BindingState{
		ConnectionState:   ConnectionActive,
		ConnectionKind:    ConnectionKindSpace,
		ConnectionAuthVer: 4,
		InstallationState: InstallationActive,
	})
	if err != nil {
		t.Fatalf("active space connection rejected: %v", err)
	}
	if binding.InstallationID != "inst-1" || binding.ConnectionID != "conn-1" {
		t.Fatalf("binding lost installation/connection ids: %+v", binding)
	}
	if binding.AuthVersion != 4 {
		t.Fatalf("binding must carry the live connection auth version, got %d", binding.AuthVersion)
	}
	if binding.RequiresReauthorization {
		t.Fatal("active space connection must not require reauthorization")
	}
}

func TestResolveSyncBindingLegacyCredentialsRequireReauthorization(t *testing.T) {
	store := &fakeBindingStore{row: &StoredSyncBinding{
		TenantID: 7, DataSourceID: "ds-legacy", InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 1,
	}}
	// Legacy credentials: no live appconnector connection row exists, so
	// personal/space ownership cannot be proven.
	binding, err := ResolveSyncBinding(context.Background(), store, 7, "ds-legacy", nil)
	if err != nil {
		t.Fatalf("legacy binding must resolve (marked), not fail: %v", err)
	}
	if !binding.RequiresReauthorization {
		t.Fatal("legacy credentials that cannot prove space ownership must be marked requires_reauthorization")
	}
}

func TestResolveSyncBindingUnusableStatesRequireReauthorization(t *testing.T) {
	store := &fakeBindingStore{row: &StoredSyncBinding{
		TenantID: 7, DataSourceID: "ds-1", InstallationID: "inst-1", ConnectionID: "conn-1",
	}}
	cases := map[string]*BindingState{
		"revoked connection":                    {ConnectionState: ConnectionRevoked, ConnectionKind: ConnectionKindSpace, InstallationState: InstallationActive},
		"pending reauthorization":               {ConnectionState: ConnectionPendingReauthorization, ConnectionKind: ConnectionKindSpace, InstallationState: InstallationActive},
		"personal connection":                   {ConnectionState: ConnectionActive, ConnectionKind: ConnectionKindPersonal, InstallationState: InstallationActive},
		"reauthorization-required installation": {ConnectionState: ConnectionActive, ConnectionKind: ConnectionKindSpace, InstallationState: InstallationReauthorizationRequired},
		"disabled installation":                 {ConnectionState: ConnectionActive, ConnectionKind: ConnectionKindSpace, InstallationState: InstallationDisabled},
	}
	for name, state := range cases {
		binding, err := ResolveSyncBinding(context.Background(), store, 7, "ds-1", state)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if !binding.RequiresReauthorization {
			t.Fatalf("%s: must be marked requires_reauthorization", name)
		}
	}
}

func TestResolveSyncBindingMissingRelationRow(t *testing.T) {
	store := &fakeBindingStore{}
	if _, err := ResolveSyncBinding(context.Background(), store, 7, "ds-none", &BindingState{
		ConnectionState: ConnectionActive, ConnectionKind: ConnectionKindSpace, InstallationState: InstallationActive,
	}); err == nil {
		t.Fatal("missing app_datasource_bindings row must be an error")
	}
}

func TestSyncPauseReasonsDistinctAndValid(t *testing.T) {
	reasons := []string{
		PauseReasonBudget, PauseReasonPlan, PauseReasonPermission, PauseReasonQuota, PauseReasonProviderLimit,
	}
	seen := map[string]bool{}
	for _, r := range reasons {
		if seen[r] {
			t.Fatalf("duplicate pause reason %q", r)
		}
		seen[r] = true
		if !IsValidPauseReason(r) {
			t.Fatalf("pause reason %q must be valid", r)
		}
	}
	if IsValidPauseReason("whatever") {
		t.Fatal("unknown pause reason must be invalid")
	}
}

func TestPlanExpiryPausesNewSyncButSettlesPersisted(t *testing.T) {
	if !ShouldPauseNewSync(false) {
		t.Fatal("expired plan must pause new sync dispatch")
	}
	if ShouldPauseNewSync(true) {
		t.Fatal("active plan must not pause new sync dispatch")
	}
	if !MaySettlePersistedSync(false) {
		t.Fatal("persisted settlement must still complete while new syncs are paused")
	}
}

func TestSyncPausedErrorCarriesReason(t *testing.T) {
	err := NewSyncPausedError(PauseReasonPlan, "plan expired")
	var paused *SyncPausedError
	if !errors.As(err, &paused) {
		t.Fatal("pause error must expose its type")
	}
	if paused.Reason != PauseReasonPlan {
		t.Fatalf("pause reason lost: %q", paused.Reason)
	}
}

func TestSourceFailureClassificationDistinguishes404PermissionUnavailable(t *testing.T) {
	if got := ClassifySourceFailure(404); got != SourceFailureNotFound {
		t.Fatalf("404 must classify as not-found, got %q", got)
	}
	if got := ClassifySourceFailure(403); got != SourceFailurePermission {
		t.Fatalf("403 must classify as permission-denied, got %q", got)
	}
	if got := ClassifySourceFailure(401); got != SourceFailurePermission {
		t.Fatalf("401 must classify as permission-denied, got %q", got)
	}
	if got := ClassifySourceFailure(500); got != SourceFailureUnavailable {
		t.Fatalf("5xx must classify as unavailable, got %q", got)
	}
}

func TestSourceFailureKeepsLocalCopyByDefault(t *testing.T) {
	// A fetch failure (404 / permission / unavailable) never deletes local
	// content: the source did not report deletion.
	if !LocalCopyPreserved(false, true) {
		t.Fatal("source failure must keep the local copy")
	}
	// A source-side deletion notice alone is not enough without the operator
	// opting into deletion sync.
	if !LocalCopyPreserved(true, false) {
		t.Fatal("deletion without sync-deletions opt-in must keep the local copy")
	}
	// Only the explicit combination removes local content.
	if LocalCopyPreserved(true, true) {
		t.Fatal("source deletion with opt-in must remove the local copy")
	}
}
