//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

// macOS always uses the login keychain. There is deliberately no in-memory
// fallback: an unavailable keychain must not turn a bearer into a process
// memory credential.
func (a *App) credentialGet(key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	output, err := exec.Command("security", "find-generic-password", "-a", key, "-s", desktopCredentialService, "-w").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (a *App) credentialSet(key, value string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = exec.Command("security", "add-generic-password", "-U", "-a", key, "-s", desktopCredentialService, "-w", value).Run()
}

func (a *App) credentialDelete(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	_ = exec.Command("security", "delete-generic-password", "-a", key, "-s", desktopCredentialService).Run()
}
