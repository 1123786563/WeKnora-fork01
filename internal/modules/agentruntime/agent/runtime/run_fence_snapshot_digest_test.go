package runtime

import (
	"context"
	"testing"
)

func TestRunFenceContextPreservesAdmittedSnapshotIdentity(t *testing.T) {
	want := Fence{
		RunKey:                RunKey{TenantID: 3, RunID: "run-3"},
		Owner:                 "worker",
		Epoch:                 9,
		SnapshotDigestVersion: 1,
		SnapshotDigest:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	got, ok := RunFenceFromContext(WithRunFence(context.Background(), want))
	if !ok {
		t.Fatal("RunFenceFromContext reported no fence")
	}
	if got.SnapshotDigestVersion != want.SnapshotDigestVersion || got.SnapshotDigest != want.SnapshotDigest {
		t.Fatalf("run fence identity = v%d/%q, want v%d/%q", got.SnapshotDigestVersion, got.SnapshotDigest, want.SnapshotDigestVersion, want.SnapshotDigest)
	}
}
