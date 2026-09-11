package appconnector

// CanUseConnection reports whether actor may invoke a call through c inside
// tenant. Both connection kinds are tenant-scoped and require an active
// connection. Personal connections are owner-only regardless of any space
// grant; space connections require an explicit grant for the actor.
func CanUseConnection(c Connection, tenant uint64, actor string, spaceGrant bool) bool {
	if c.TenantID != tenant || c.State != "active" {
		return false
	}
	if c.Kind == "personal" {
		return c.OwnerID == actor
	}
	return c.Kind == "space" && spaceGrant
}

// CanInstallInstallation reports whether a tenant role may install or
// upgrade an app. Owners and admins may; every other role (regular members)
// may only request an installation. Installing an app is deliberately NOT a
// commercial billing decision — this helper never consults
// commercial.CanManageBilling, because installing an app is not purchasing
// anything.
func CanInstallInstallation(role string) bool {
	return role == "owner" || role == "admin"
}
