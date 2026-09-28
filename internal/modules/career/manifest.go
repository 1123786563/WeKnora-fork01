package career

// Manifest declares the module's ownership and narrow cross-module ports.
type Manifest struct {
	Name         string
	Owns         []string
	Dependencies []string
}

// ModuleManifest is the machine-readable Career boundary declaration.
var ModuleManifest = Manifest{
	Name:         "career",
	Owns:         []string{"career_spaces", "career_profile_facts", "career_idempotency_receipts", "career_evidence"},
	Dependencies: []string{"identity.authenticated_scope", "workbench.task", "workbench.artifact_grant", "commercial.usage_budget"},
}
