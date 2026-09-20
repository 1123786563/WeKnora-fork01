package types

// Agent Marketplace portable Release value types (T28, Ticket #58, Task 2).
//
// The Agent domain owns AgentVersion; the Marketplace owns Listing, Release,
// Manifest, Dependency Lock and lineage. This file carries the PORTABLE
// value types that cross that boundary: the author-supplied Manifest
// metadata, the canonical Manifest document, the Dependency Lock and the
// sanitized Release bundle produced by the exporter in
// internal/agent/experts/agent_release.go. Persistence entities
// (Submission/Review/Release/Listing rows) are declared in
// agent_marketplace_persistence.go.
//
// Domain invariants these types encode (docs/specs/
// 2026-09-20-agent-marketplace-domain-model.md §5–§6, §11):
//   - Release content is an allow-list projection of the frozen agent
//     snapshot; KB bindings, model IDs, credentials, Sandbox config, Memory
//     and Task/session content never enter it.
//   - Manifest, Dependency Lock, portable payload and license share ONE
//     digest boundary: the bundle's canonical bytes.
//   - Dependency Lock entries pin a concrete immutable, redistributable,
//     licensed dependency — never a moving reference such as "latest".

// ReleaseMetadata is the author-supplied part of a Release Manifest: the
// semantic version, descriptive fields, declared capabilities and the
// license under which the Release is distributed. Per the domain model these
// are publisher DECLARATIONS and review input — they never grant Tenant
// resources or Task permissions.
type ReleaseMetadata struct {
	// SemanticVersion is the Release's semantic version string.
	SemanticVersion string `json:"semantic_version"`
	// DisplayName is the human-facing catalog name.
	DisplayName string `json:"display_name"`
	// Summary is the one-to-few-sentence catalog description.
	Summary string `json:"summary"`
	// SupportedLanguages lists the languages the Release declares support
	// for (the product's locale surfaces, currently "zh" and "en").
	SupportedLanguages []string `json:"supported_languages"`
	// UseCases names what the Release is for; at least one is required.
	UseCases []string `json:"use_cases"`
	// NonUseCases names what the Release explicitly is NOT for.
	NonUseCases []string `json:"non_use_cases"`
	// CapabilityRequirements declares model, knowledge, skill, subagent,
	// sandbox and connector needs as capability names — requirements to be
	// mapped locally, never embedded resources.
	CapabilityRequirements []string `json:"capability_requirements"`
	// DataCategories declares the categories of data the agent may read.
	DataCategories []string `json:"data_categories"`
	// ExternalSideEffects declares external effects the agent may cause
	// (e.g. web search queries, outbound mail).
	ExternalSideEffects []string `json:"external_side_effects"`
	// MinimumWeKnoraCapability is the minimum WeKnora capability/version a
	// Deployment needs to run the Release.
	MinimumWeKnoraCapability string `json:"minimum_weknora_capability"`
	// LicenseID identifies the license terms the Release is distributed
	// under. Absent license means the submission fails.
	LicenseID string `json:"license_id"`
	// ChangeNotes explains what changed relative to the prior Release.
	ChangeNotes string `json:"change_notes"`
}

// AgentReleaseSource binds a Release to the immutable Agent-domain version
// it was exported from. This is lineage, not ownership: the Marketplace
// references the version ID and source digest and never re-reads or mutates
// the frozen row.
type AgentReleaseSource struct {
	// AgentVersionID is the immutable agent_versions row ID.
	AgentVersionID string `json:"agent_version_id"`
	// VersionNumber is the 1-based sequence number within the source agent.
	VersionNumber int `json:"version_number"`
	// SourceSHA256 is the canonical source digest recorded at freeze time.
	SourceSHA256 string `json:"source_sha256"`
}

// AgentReleaseManifest is the canonical Manifest serialized inside the
// Release bundle: the author's metadata (embedded ReleaseMetadata, flattened
// into the same JSON keys) plus the immutable source binding. Listing and
// Release identities, publisher identity, publication time and the bundle
// digest are server-generated at submission/publication time and live on the
// persistence rows, not in this document.
type AgentReleaseManifest struct {
	ReleaseMetadata
	// Source is the immutable AgentVersion the bundle was projected from.
	Source AgentReleaseSource `json:"source"`
}

// AgentReleaseDependency is one locked dependency of a Release: a
// redistributable skill or subagent pinned to an exact immutable version
// with a content digest and a license that permits redistribution. Model,
// knowledge, connection, sandbox and credential needs are NOT dependencies —
// they are Manifest capability requirements.
type AgentReleaseDependency struct {
	// Type is the dependency kind ("skill", "subagent").
	Type string `json:"type"`
	// ID is the stable dependency identity (slug / catalog name).
	ID string `json:"id"`
	// Version is the concrete immutable version; moving references such as
	// "latest" are rejected.
	Version string `json:"version"`
	// Digest is the SHA-256 hex digest of the dependency's exact content.
	Digest string `json:"digest"`
	// LicenseID identifies a license permitting redistribution of the
	// dependency inside the Release.
	LicenseID string `json:"license_id"`
}

// DependencyLock pins every redistributable dependency of a Release to an
// exact immutable version, digest and license. The lock is server-resolved
// (T28 Task 4) from the frozen snapshot's portable skill/subagent
// references; caller-supplied digests are never trusted. Runtime never
// substitutes a same-name dependency or pulls a newer version.
type DependencyLock struct {
	Dependencies []AgentReleaseDependency `json:"dependencies"`
}

// AgentReleasePayload is the allow-listed portable projection of the frozen
// CustomAgent snapshot: system behavior only. It deliberately contains no
// KB bindings, no model IDs, no credentials, no Sandbox binding, no Memory
// flags and no Task/session content — those belong to the publisher's
// workspace and are re-established per Adoption through local capability
// mapping, never shipped inside a Release.
type AgentReleasePayload struct {
	// AgentMode is the frozen agent's mode ("smart-reasoning"/"quick-answer").
	AgentMode string `json:"agent_mode"`
	// SystemPrompt is the portable system prompt.
	SystemPrompt string `json:"system_prompt"`
	// PersonaMBTI is the optional persona profile code.
	PersonaMBTI string `json:"persona_mbti,omitempty"`
	// PersonaStyle is the optional free-text persona guidance.
	PersonaStyle string `json:"persona_style,omitempty"`
	// AllowedTools names the portable tool allowlist.
	AllowedTools []string `json:"allowed_tools"`
	// Skills lists skill REFERENCES resolved through the Dependency Lock —
	// never bundled payloads.
	Skills []string `json:"skills"`
	// Subagents lists subagent catalog references, also lock-resolved.
	Subagents []string `json:"subagents"`
	// StarterPrompts are the portable conversation starters.
	StarterPrompts []string `json:"starter_prompts"`
}

// AgentReleaseBundle is the canonical sanitized Release bundle: the exact
// document set a submission reviews, a Release stores and a digest verifies.
// Manifest, Dependency Lock and portable payload are the typed projection;
// Bytes is their canonical serialization (deterministic JSON: fixed field
// order, sorted lock entries, no incidental whitespace or HTML escaping) and
// SHA256 is the hex digest over exactly those bytes. Any change to portable
// behavior, lock, Manifest or license therefore changes the digest.
type AgentReleaseBundle struct {
	Manifest AgentReleaseManifest `json:"manifest"`
	Lock     DependencyLock       `json:"dependency_lock"`
	Payload  AgentReleasePayload  `json:"payload"`
	// Bytes is the canonical serialized envelope (payload + Manifest +
	// Dependency Lock) the digest is computed over.
	Bytes []byte `json:"-"`
	// SHA256 is the lowercase hex SHA-256 digest over Bytes.
	SHA256 string `json:"-"`
}
