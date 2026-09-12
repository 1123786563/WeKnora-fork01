package commercial

// CanManageBilling reports whether an active tenant member may manage billing.
// Administrative role membership does not imply this capability; owners and
// members with an explicit billing grant are the only callers admitted.
func CanManageBilling(role string, active, billingGrant bool) bool {
	return active && (role == "owner" || billingGrant)
}
