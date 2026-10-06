package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds Wails-bound state for the desktop shell.
type App struct {
	ctx           context.Context
	setupToken    string
	backendURL    string
	apiLanBaseURL string
	listenPublic  bool
	shutdownCh    chan struct{}
	shutdownDone  chan struct{}
	pickingDir    atomic.Bool
	credentialMu  sync.RWMutex
	credentials   map[string]string
}

const desktopCredentialService = "com.tencent.weknora.desktop"

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		shutdownCh:   make(chan struct{}, 1),
		shutdownDone: make(chan struct{}),
		credentials:  make(map[string]string),
	}
}

// GetCredential reads a desktop credential through the Wails bridge. macOS
// builds use the login keychain; keeping the store behind App means bearer
// material never falls back to WebView storage.
func (a *App) GetCredential(key string) string {
	return a.credentialGet(key)
}

// SetCredential is used by the desktop auth callback to hand an opaque
// credential to the native shell. It is deliberately not exposed to the
// renderer as localStorage.
func (a *App) SetCredential(key, value string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	a.credentialSet(key, value)
}

// DeleteCredential is idempotent and is called when the personal node is
// revoked or the desktop session logs out.
func (a *App) DeleteCredential(key string) {
	a.credentialDelete(key)
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
	select {
	case a.shutdownCh <- struct{}{}:
	default:
	}
	<-a.shutdownDone
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

// GetAutoSetupToken exposes the per-process capability only through the native bridge.
func (a *App) GetAutoSetupToken() string { return a.setupToken }

// GetApprovalMode returns the machine-local sandbox approval mode.
func (a *App) GetApprovalMode() string {
	return LoadApprovalMode()
}

// SetApprovalMode persists the approval mode. Unshipped modes ("ask", "full")
// are rejected; currently only "auto" can be stored.
func (a *App) SetApprovalMode(mode string) error {
	return SaveApprovalMode(mode)
}

// GetProjectDirs returns the user-approved project directories.
func (a *App) GetProjectDirs() []string {
	return LoadProjectDirs()
}

// RemoveProjectDir drops one approved directory. The only way to add a
// directory is PickProjectDir; typed replacement of the whole list is not
// an authorization path.
func (a *App) RemoveProjectDir(dir string) error {
	return removeApprovedProjectDir(dir)
}

// PickProjectDir opens the system directory picker and appends the result to
// the approved list. Users must pick a folder this way; typed paths are not
// an authorization path.
func (a *App) PickProjectDir() (string, error) {
	if !a.pickingDir.CompareAndSwap(false, true) {
		return "", fmt.Errorf("a folder picker is already open")
	}
	defer a.pickingDir.Store(false)

	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                "Select Project Directory",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	return appendApprovedProjectDir(dir)
}
