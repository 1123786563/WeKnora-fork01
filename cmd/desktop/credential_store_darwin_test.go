//go:build darwin

package main

import (
	"os/exec"
	"testing"
)

func TestDarwinCredentialStoreFailsClosedWhenSecurityCommandFails(t *testing.T) {
	original := securityCommand
	t.Cleanup(func() { securityCommand = original })

	var calls int
	securityCommand = func(_ string, _ ...string) *exec.Cmd {
		calls++
		return exec.Command("sh", "-c", "exit 1")
	}

	app := NewApp()
	// If the Darwin implementation accidentally consults App.credentials after
	// a keychain error, this sentinel would leak into the renderer bridge.
	app.credentials["node"] = "process-memory-secret"
	app.SetCredential("node", "new-secret")
	if got := app.GetCredential("node"); got != "" {
		t.Fatalf("failed keychain read must fail closed, got %q", got)
	}
	app.DeleteCredential("node")
	if calls != 3 {
		t.Fatalf("expected failed set/get/delete to invoke security three times, got %d", calls)
	}
}
