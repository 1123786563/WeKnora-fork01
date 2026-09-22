package execution

import "testing"

// TestRestoreCannotReplayUnreconciledCommands pins the W35 backup-restore
// admission policy: after a snapshot restore, new command dispatch may open
// ONLY when every binding has been reconciled and no dispatch outcome stayed
// unknown. A stale backup showing a command as queued proves nothing — the
// command may already have executed against the real provider.
func TestRestoreCannotReplayUnreconciledCommands(t *testing.T) {
	if MayDispatchAfterRestore(false, 0) || MayDispatchAfterRestore(true, 1) {
		t.Fatal("unsafe redispatch")
	}
	if !MayDispatchAfterRestore(true, 0) {
		t.Fatal("reconciled restore blocked")
	}
}
