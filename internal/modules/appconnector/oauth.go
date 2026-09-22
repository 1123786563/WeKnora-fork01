package appconnector

import (
	"errors"
	"time"
)

// OAuthBinding is the pending one-time authorization state issued when an
// actor starts an OAuth flow against an installation inside a tenant.
// State is the opaque "state" parameter round-tripped through the provider;
// it is consumed exactly once at callback time.
type OAuthBinding struct {
	State          string
	InstallationID string
	ActorID        string
	TenantID       uint64
	ExpiresAt      time.Time
	Used           bool
}

// ValidateOAuthBinding reports whether a callback presenting this binding may
// redeem it: the state must exist, belong to the same tenant and the same
// initiating actor, never have been used, and not have expired. The window is
// strictly now.Before(ExpiresAt), so a state expiring exactly now is already
// invalid.
func ValidateOAuthBinding(b OAuthBinding, tenant uint64, actor string, now time.Time) error {
	if b.State == "" || b.TenantID != tenant || b.ActorID != actor || b.Used || !now.Before(b.ExpiresAt) {
		return errors.New("oauth_binding_invalid")
	}
	return nil
}
