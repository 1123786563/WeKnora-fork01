package appconnector

// Installation lifecycle states. A fresh install starts active; an operator
// can disable it; an upgrade whose new version expands the permission scope
// parks it in reauthorization_required until every affected connection is
// re-authorized — it never auto-activates.
const (
	InstallationActive                  = "active"
	InstallationDisabled                = "disabled"
	InstallationReauthorizationRequired = "reauthorization_required"
)

// Connection lifecycle states. Personal and space connections are revoked on
// owner request; a scope-expanding upgrade moves them to
// pending_reauthorization until the owner re-consents.
const (
	ConnectionActive                 = "active"
	ConnectionRevoked                = "revoked"
	ConnectionPendingReauthorization = "pending_reauthorization"
)

// Connection kinds. "personal" connections are owner-only; "space"
// connections are shared but require an explicit per-actor grant.
const (
	ConnectionKindPersonal = "personal"
	ConnectionKindSpace    = "space"
)

// Installation is one tenant's install of an app at a specific version.
// The same app installed in two spaces yields two independent rows.
type Installation struct {
	ID       string
	AppID    string
	Version  string
	State    string
	TenantID uint64
}

// Connection binds an installation to a credential. Credentials themselves
// never live here — CredentialRef points at the dedicated credential store.
type Connection struct {
	ID             string
	InstallationID string
	Kind           string
	OwnerID        string
	CredentialRef  string
	State          string
	TenantID       uint64
	AuthVersion    int64
}
