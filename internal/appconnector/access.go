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

// CanManageConnections reports whether a tenant role may manage space
// connections (start an authorization flow, revoke). The permission
// matrix keeps 管理空间连接与授权 as its own row, separate from
// installation management: the predicates coincide today, but the
// vocabulary stays distinct so the two policies can diverge without
// touching each other's gates.
func CanManageConnections(role string) bool {
	return role == "owner" || role == "admin"
}

// CanDriveActionWrites reports whether a tenant role may drive the action
// pipeline's WRITE endpoints (prepare/approve/execute). Action approval is
// its own lifecycle (B12/B13): each approval binds the actual target and
// parameters at the service layer, and this role gate is declared
// separately from installation authority so action policy can follow the
// resource-permission matrix independently.
func CanDriveActionWrites(role string) bool {
	return role == "owner" || role == "admin"
}
