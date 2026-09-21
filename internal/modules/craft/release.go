package craft

// ReleaseFacts is the release gate's fact sheet: one boolean per capability
// axis the 27-task Craft program committed to, each backed by machine
// evidence collected in docs/testing/craft/release-evidence.json (never a
// hand-filled assertion — scripts/check-craft-release.py verifies the
// evidence file and the artifacts it points at).
//
// The axes map onto the program's pillar tasks:
//   - Web: W01–W06 workbench surfaces (create/run/versions/preview/reconnect)
//   - Permissions: W03/R06 read/write ACLs and interaction approvals
//   - Cancel: W04/O03 run cancellation and lifecycle teardown guards
//   - Reconnect: W06 browser reconnect (refresh/killed-stream, single admission)
//   - Recovery: C04/G3 crash recovery + C05 snapshot restore after real kills
//   - Versioning: W01/W02 immutable versions and per-kind manifests
//   - Isolation: R02/R03 tenant/scope isolation and sandbox binding
//   - Quota: O01/O03 budget admission and call authorization
//   - Usage: O04 usage ledger/views (unknown stays visible, never folded to 0)
//   - Billing: O02 commercial evidence — ONLY meaningful for paid releases
//     and, per the G4 external gap, currently NOT satisfiable (missing
//     commercial_reservations owner column + four AutoMigrate-only tables),
//     so a paid release fails this gate until that side lands.
type ReleaseFacts struct {
	Web, Permissions, Cancel, Reconnect, Recovery, Versioning, Isolation, Quota, Usage, Billing bool
}

// CanRelease answers whether the Craft feature set may ship in the requested
// mode. Every capability axis must hold; a paid (commercial) release
// additionally requires the Billing fact, which only real commercial
// evidence can set. A controlled (unpaid) release is the default posture and
// is never blocked by a missing Billing fact.
func CanRelease(f ReleaseFacts, paid bool) bool {
	return f.Web && f.Permissions && f.Cancel && f.Reconnect && f.Recovery && f.Versioning && f.Isolation && f.Quota && f.Usage && (!paid || f.Billing)
}
