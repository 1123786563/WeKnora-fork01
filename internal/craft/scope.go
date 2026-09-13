package craft

// SameScope reports whether two scopes carry the exact same complete
// execution identity: same non-zero tenant, same non-empty user and same
// non-empty session. It is used for execution-binding identity equality and
// does not replace the shared-session read ACL: shared viewing stays with the
// existing permission services.
func SameScope(a, b Scope) bool {
	return a.TenantID != 0 && a.UserID != "" && a.SessionID != "" &&
		a.TenantID == b.TenantID && a.UserID == b.UserID && a.SessionID == b.SessionID
}
