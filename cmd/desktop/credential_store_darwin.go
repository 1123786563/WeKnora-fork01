//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

// Kept behind a narrow seam so Darwin tests can exercise command failures
// without touching a user's login keychain. Production uses exec.Command.
var securityCommand = exec.Command

// macOS always uses the login keychain. There is deliberately no in-memory
// fallback: an unavailable keychain must not turn a bearer into a process
// memory credential.
func (a *App) credentialGet(key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	output, err := securityCommand("security", "find-generic-password", "-a", key, "-s", desktopCredentialService, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (a *App) credentialSet(key, value string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = securityCommand("security", "add-generic-password", "-U", "-a", key, "-s", desktopCredentialService, "-w", value).Run()
}

func (a *App) credentialDelete(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	_ = securityCommand("security", "delete-generic-password", "-a", key, "-s", desktopCredentialService).Run()
}
