package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// App holds Wails-bound state for the desktop shell.
type App struct {
	ctx           context.Context
	backendURL    string
	apiLanBaseURL string
	listenPublic  bool
	shutdownCh    chan struct{}
	credentialMu  sync.RWMutex
	credentials   map[string]string
}

const desktopCredentialService = "com.tencent.weknora.desktop"

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		shutdownCh:  make(chan struct{}, 1),
		credentials: make(map[string]string),
	}
}

// GetCredential reads a desktop credential through the Wails bridge. macOS
// builds use the login keychain; keeping the store behind App means bearer
// material never falls back to WebView storage.
func (a *App) GetCredential(key string) string {
	a.credentialMu.RLock()
	value := a.credentials[key]
	a.credentialMu.RUnlock()
	if value != "" {
		return value
	}
	// macOS Wails builds use the user login keychain. The in-memory map is a
	// test/non-macOS fallback and is never copied into WebView storage.
	if output, err := exec.Command("security", "find-generic-password", "-a", key, "-s", desktopCredentialService, "-w").Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// SetCredential is used by the desktop auth callback to hand an opaque
// credential to the native shell. It is deliberately not exposed to the
// renderer as localStorage.
func (a *App) SetCredential(key, value string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	a.credentialMu.Lock()
	a.credentials[key] = value
	a.credentialMu.Unlock()
	if err := exec.Command("security", "add-generic-password", "-U", "-a", key, "-s", desktopCredentialService, "-w", value).Run(); err != nil {
		// Non-macOS/test environments retain the value for the current process.
	}
}

// DeleteCredential is idempotent and is called when the personal node is
// revoked or the desktop session logs out.
func (a *App) DeleteCredential(key string) {
	_ = exec.Command("security", "delete-generic-password", "-a", key, "-s", desktopCredentialService).Run()
	a.credentialMu.Lock()
	delete(a.credentials, key)
	a.credentialMu.Unlock()
}

// GetPaseoURL and GetPaseoAllowedOrigins are deployment-owned values. Empty
// values intentionally fail closed in the renderer composition.
func (a *App) GetPaseoURL() string { return strings.TrimSpace(os.Getenv("PASEO_URL")) }

func (a *App) GetPaseoAllowedOrigins() []string {
	raw := strings.Split(os.Getenv("PASEO_ALLOWED_ORIGINS"), ",")
	result := make([]string, 0, len(raw))
	for _, origin := range raw {
		if value := strings.TrimSpace(origin); value != "" {
			result = append(result, value)
		}
	}
	return result
}

// startup is called when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.shutdownCh <- struct{}{}
}

// GetAPIBaseURL returns the local HTTP base URL for REST API calls (e.g. http://127.0.0.1:PORT/api/v1).
// The desktop shell proxies the webview to this address; window.location.origin is not the API host.
func (a *App) GetAPIBaseURL() string {
	if a.backendURL == "" {
		return ""
	}
	return strings.TrimRight(a.backendURL, "/") + "/api/v1"
}

// GetDesktopHTTPPortSetting returns the saved local API port (0 = random port each launch).
func (a *App) GetDesktopHTTPPortSetting() int {
	return LoadDesktopPrefsHTTPPort()
}

// SetDesktopHTTPPortSetting saves the preferred local API port to application support. Restart the app for it to take effect unless it matches the current listener.
func (a *App) SetDesktopHTTPPortSetting(port int) error {
	return SaveDesktopHTTPPortPreference(port)
}

// GetDesktopHTTPBindPublicSetting returns whether API listens on all interfaces (0.0.0.0).
func (a *App) GetDesktopHTTPBindPublicSetting() bool {
	return LoadDesktopHTTPBindPublic()
}

// SetDesktopHTTPBindPublicSetting saves LAN/public listen preference. Restart the app for it to take effect.
func (a *App) SetDesktopHTTPBindPublicSetting(v bool) error {
	return SaveDesktopHTTPBindPublicPreference(v)
}

// GetAPILanBaseURL returns a suggested base URL for other devices on the LAN (…/api/v1), or empty if not in bind-public mode or IP detection failed.
func (a *App) GetAPILanBaseURL() string {
	return a.apiLanBaseURL
}

// GetDesktopListenPublicActive is true when this session’s API server is listening on all interfaces (runtime), not the saved preference.
func (a *App) GetDesktopListenPublicActive() bool {
	return a.listenPublic
}

// CheckForUpdates manually triggers the update check from the frontend.
func (a *App) CheckForUpdates() {
	if a.ctx != nil {
		checkUpdate(a.ctx, desktopAboutVersion(), true, false)
	}
}

// AutoCheckForUpdates silently checks for updates and downloads them.
func (a *App) AutoCheckForUpdates() {
	if a.ctx != nil {
		checkUpdate(a.ctx, desktopAboutVersion(), false, true)
	}
}
