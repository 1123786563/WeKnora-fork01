// Package execution contains lifecycle invariants shared by platform and
// remote execution cleanup workers. This file carries the W35 backup-restore
// admission policy: what a node must prove before it may dispatch NEW
// commands again after its state came back from a backup snapshot.
package execution

// MayDispatchAfterRestore reports whether NEW command dispatch may open on a
// node whose durable state was restored from a backup snapshot.
//
// It opens ONLY when both hold:
//
//   - reconciled: every command binding carried in the restored state has
//     been reconciled against the authoritative external systems — observed
//     still-live external processes, matched provider usage records against
//     command receipts, and rebuilt the local projections from that
//     evidence.
//   - unknown == 0: no dispatch outcome remained unknown (crash window /
//     lost ack). An unknown outcome is not equivalent to "never executed".
//
// Everything else stays fail-closed. In particular a stale backup that still
// shows a command as queued must NEVER be read as proof the command never
// executed: the external process may have started and billed usage after the
// snapshot was taken. Replaying such a command on restore is exactly the
// duplicate-side-effect window this predicate exists to close, so callers
// keep admission closed per binding until observation completes.
func MayDispatchAfterRestore(reconciled bool, unknown int) bool {
	return reconciled && unknown == 0
}
