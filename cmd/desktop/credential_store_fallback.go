//go:build !darwin

package main

// Non-macOS builds use an explicitly process-scoped fallback. It is never
// compiled into macOS, where keychain failures must fail closed.
func (a *App) credentialGet(key string) string {
	a.credentialMu.RLock()
	defer a.credentialMu.RUnlock()
	return a.credentials[key]
}

func (a *App) credentialSet(key, value string) {
	a.credentialMu.Lock()
	defer a.credentialMu.Unlock()
	a.credentials[key] = value
}

func (a *App) credentialDelete(key string) {
	a.credentialMu.Lock()
	defer a.credentialMu.Unlock()
	delete(a.credentials, key)
}
