package craft

import "testing"

// TestRestoreNeedsSessionAndQuiescence pins the C05 acceptance rule verbatim:
// files alone are never a complete recovery — the OpenCode session digest is
// mandatory — an active execution is never restored over, and the snapshot's
// runtime must match the deployment's runtime.
func TestRestoreNeedsSessionAndQuiescence(t *testing.T) {
	s := Snapshot{WorkspaceID: "w", VersionID: "v", FilesDigest: "f", RuntimeDigest: "d", Quiescent: true}
	if CanRestore(s, false, "d") {
		t.Fatal("files alone accepted")
	}
	s.SessionDigest = "oc"
	if !CanRestore(s, false, "d") || CanRestore(s, true, "d") {
		t.Fatal("active restore")
	}
}

// TestSnapshotIDIsContentIdentity pins idempotent capture identity: the same
// quiescent identity always derives the same id, and any identity change —
// files, session, runtime — derives a different one, so a retried capture
// after a crash adopts the stored row instead of duplicating it.
func TestSnapshotIDIsContentIdentity(t *testing.T) {
	base := Snapshot{
		WorkspaceID: "w", VersionID: "v",
		FilesDigest: stringOfLen(64), SessionDigest: stringOfLen(64),
		RuntimeDigest: "d", Quiescent: true,
	}
	if got := SnapshotID(base); got != SnapshotID(base) || len(got) != len(SnapshotIDPrefix)+64 {
		t.Fatalf("snapshot id not stable: %q", got)
	}
	changed := base
	changed.SessionDigest = stringOfLen(64)[:63] + "a"
	if SnapshotID(changed) == SnapshotID(base) {
		t.Fatal("session digest change must change the snapshot id")
	}
	changed = base
	changed.RuntimeDigest = "other"
	if SnapshotID(changed) == SnapshotID(base) {
		t.Fatal("runtime change must change the snapshot id")
	}
}

// TestCanRestoreRejectsChangedRuntime pins the runtime comparability rule:
// a snapshot from a replaced runtime never restores, whatever else holds.
func TestCanRestoreRejectsChangedRuntime(t *testing.T) {
	s := Snapshot{
		WorkspaceID: "w", VersionID: "v", FilesDigest: "f", SessionDigest: "oc",
		RuntimeDigest: "d", Quiescent: true,
	}
	if CanRestore(s, false, "d2") {
		t.Fatal("snapshot restored onto a changed runtime")
	}
	if !CanRestore(s, false, "d") {
		t.Fatal("matching runtime refused")
	}
}

// TestSessionDigestOrderIndependentAndChainChecked pins the canonical
// session digest: export order does not change identity, duplicate or empty
// ids are refused, and a parent pointing outside the chain is never a valid
// message chain.
func TestSessionDigestOrderIndependentAndChainChecked(t *testing.T) {
	a := SessionRecord{ID: "msg_a", Role: "user", Parts: []string{"{\"type\":\"text\",\"text\":\"hi\"}"}}
	b := SessionRecord{ID: "msg_b", ParentID: "msg_a", Role: "assistant", Finish: "stop", Parts: []string{"{\"type\":\"text\",\"text\":\"ok\"}"}}
	d1, err := SessionDigest([]SessionRecord{a, b})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := SessionDigest([]SessionRecord{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatal("listing order changed the session digest")
	}
	if _, err := SessionDigest(nil); err == nil {
		t.Fatal("empty export accepted")
	}
	if _, err := SessionDigest([]SessionRecord{a, a}); err == nil {
		t.Fatal("duplicate records accepted")
	}
	if err := ValidSessionChain([]SessionRecord{a, b}); err != nil {
		t.Fatal(err)
	}
	broken := b
	broken.ParentID = "msg_missing"
	if err := ValidSessionChain([]SessionRecord{a, broken}); err == nil {
		t.Fatal("forward/unknown parent accepted as a chain")
	}
}

// TestStoredSnapshotRequiresSessionObjects pins that a files-only pin never
// persists: every stored snapshot must carry exported OpenCode data.
func TestStoredSnapshotRequiresSessionObjects(t *testing.T) {
	base := StoredSnapshot{
		Snapshot: Snapshot{
			WorkspaceID: "w", VersionID: "v", FilesDigest: stringOfLen(64),
			SessionDigest: stringOfLen(64), RuntimeDigest: "d", Quiescent: true,
		},
		Manifest: SnapshotManifest{Version: SnapshotManifestVersion, RuntimeDigest: "d"},
		Objects:  []SnapshotObject{{Kind: SnapshotObjectFile, Name: "index.html", Ref: "r", SHA256: stringOfLen(64), Bytes: 1}},
	}
	if _, err := ValidateStoredSnapshot(base); err == nil {
		t.Fatal("files-only snapshot accepted for persistence")
	}
	base.Objects = append(base.Objects, SnapshotObject{Kind: SnapshotObjectSession, Name: "session/export.json", Ref: "r2", SHA256: stringOfLen(64), Bytes: 2})
	base.ID = SnapshotID(base.Snapshot)
	if stored, err := ValidateStoredSnapshot(base); err != nil || stored.ID != SnapshotID(base.Snapshot) {
		t.Fatalf("complete snapshot rejected: %v", err)
	}
	if err := ValidateSnapshotObject(SnapshotObject{Kind: "other", Name: "a.txt", Ref: "r", SHA256: stringOfLen(64)}); err == nil {
		t.Fatal("unknown object kind accepted")
	}
	if err := ValidateSnapshotObject(SnapshotObject{Kind: SnapshotObjectSession, Name: ".env", Ref: "r", SHA256: stringOfLen(64)}); err == nil {
		t.Fatal("credential-shaped object name accepted")
	}
}

// stringOfLen builds a deterministic hex-only string of n characters, so
// digest-shaped test values satisfy ValidSHA256.
func stringOfLen(n int) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, n)
	for i := range out {
		out[i] = hexDigits[i%len(hexDigits)]
	}
	return string(out)
}
