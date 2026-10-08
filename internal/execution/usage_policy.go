package execution

// AllowModelSettlement decides whether a remote model usage observation may
// become a model credit settlement. The source is resolved from the
// authenticated platform binding; the remote payload is never authoritative.
// Personal nodes are useful execution transports but cannot self-authorize
// billing. BYOK model calls are also excluded because their model cost is
// paid by the tenant's provider. Other platform services remain billable
// through their own service dimension.
func AllowModelSettlement(source, funding string) bool {
	return source == "platform_gateway" && funding == "platform"
}
