// Canonical sanitized Agent Release bundle export (T28, Ticket #58, Task 2).
//
// BuildAgentReleaseBundle projects one frozen AgentVersion snapshot — the
// immutable source the Agent domain owns — into the portable Release bundle
// the tenant Marketplace submits, reviews, stores and distributes. The
// projection is an ALLOW-LIST: only the portable system behavior (prompt,
// persona, tools, skill/subagent references, starters, agent mode) plus the
// author-supplied Manifest metadata and the server-resolved Dependency Lock
// enter the bundle. KB bindings, model IDs and every model-key-related
// config field, credential-shaped content, the Sandbox binding, Memory
// flags, MCP services and Task/session content stay in the publisher's
// workspace — they are re-established per Adoption through local capability
// mapping, never shipped.
//
// The whole bundle is canonically serialized once (deterministic JSON) and
// digested with SHA-256: identical snapshots produce byte-identical bundles,
// and any change to portable behavior, lock, Manifest or license produces a
// different digest — the immutability boundary a Review approves and a
// Release stores.
package experts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// supportedReleaseLanguages is the closed set of languages a Release
// Manifest may declare: the product's two locale surfaces (LocaleText's zh
// and en keys, the persona renderer's zh* switch). Anything else fails the
// submission instead of shipping an unverifiable declaration.
var (
	supportedReleaseLanguages = map[string]bool{"zh": true, "en": true}
	releaseSemanticVersion    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`) //nolint:lll // 预存长行（完整语义化版本正则，拆分会改变字符串内容）
)

// agentReleaseEnvelope is the exact document set the bundle digest covers:
// portable payload + Manifest + Dependency Lock (the Manifest's license_id
// carries the license; it shares the digest boundary). Field order is fixed
// by the struct, which is what makes the serialization canonical.
type agentReleaseEnvelope struct {
	Payload        types.AgentReleasePayload  `json:"payload"`
	Manifest       types.AgentReleaseManifest `json:"manifest"`
	DependencyLock types.DependencyLock       `json:"dependency_lock"`
}

// BuildAgentReleaseBundle exports the immutable AgentVersion snapshot as a
// canonical sanitized Release bundle. It is PURE: no disk, no network — the
// submission flow (Task 4) persists the returned bytes by digest.
//
// input is the author-supplied Manifest metadata; lock is the
// server-resolved Dependency Lock (resolution itself is Task 4's job — here
// every entry is validated immutable, redistributable and licensed). The
// returned bundle carries the normalized Manifest, the canonically ordered
// lock, the allow-listed payload, the canonical envelope bytes and their
// SHA-256 digest.
func BuildAgentReleaseBundle(
	version types.AgentVersionSnapshot,
	input types.ReleaseMetadata,
	lock types.DependencyLock,
) (types.AgentReleaseBundle, error) {
	if version.Agent == nil {
		return types.AgentReleaseBundle{}, fmt.Errorf("experts: release bundle: the agent snapshot is nil")
	}
	manifest, err := releaseManifest(version, input)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}
	canonicalLock, err := canonicalDependencyLock(lock)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}
	payload, err := releasePayload(version.Agent)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}

	raw, err := canonicalReleaseJSON(agentReleaseEnvelope{
		Payload:        payload,
		Manifest:       manifest,
		DependencyLock: canonicalLock,
	})
	if err != nil {
		return types.AgentReleaseBundle{}, fmt.Errorf("experts: release bundle: serialize the canonical envelope: %w", err)
	}
	sum := sha256.Sum256(raw)
	return types.AgentReleaseBundle{
		Manifest: manifest,
		Lock:     canonicalLock,
		Payload:  payload,
		Bytes:    raw,
		SHA256:   hex.EncodeToString(sum[:]),
	}, nil
}

// releaseManifest validates and normalizes the author-supplied metadata and
// binds it to the immutable source version. Validation failures are the
// submission-time rejections: missing license, unsupported (or absent)
// language, empty use cases and absent minimum capability.
func releaseManifest(version types.AgentVersionSnapshot, input types.ReleaseMetadata) (types.AgentReleaseManifest, error) {
	manifest := types.AgentReleaseManifest{
		ReleaseMetadata: types.ReleaseMetadata{
			SemanticVersion:          strings.TrimSpace(input.SemanticVersion),
			DisplayName:              strings.TrimSpace(input.DisplayName),
			Summary:                  strings.TrimSpace(input.Summary),
			SupportedLanguages:       make([]string, 0, len(input.SupportedLanguages)),
			UseCases:                 trimmedList(input.UseCases),
			NonUseCases:              trimmedList(input.NonUseCases),
			CapabilityRequirements:   trimmedList(input.CapabilityRequirements),
			DataCategories:           trimmedList(input.DataCategories),
			ExternalSideEffects:      trimmedList(input.ExternalSideEffects),
			MinimumWeKnoraCapability: strings.TrimSpace(input.MinimumWeKnoraCapability),
			LicenseID:                strings.TrimSpace(input.LicenseID),
			ChangeNotes:              strings.TrimSpace(input.ChangeNotes),
		},
		Source: types.AgentReleaseSource{
			AgentVersionID: strings.TrimSpace(version.ID),
			VersionNumber:  version.VersionNumber,
			SourceSHA256:   strings.TrimSpace(version.SourceSHA256),
		},
	}

	if manifest.Source.AgentVersionID == "" {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: the agent version id is required")
	}

	// Languages: normalize tags (trim, lowercase, primary subtag — "zh-CN"
	// declares "zh") and reject anything outside the supported set. The
	// supported set stays explicit so an unsupported language fails the
	// submission instead of shipping an unverifiable declaration.
	for _, lang := range input.SupportedLanguages {
		normalized := normalizeReleaseLanguage(lang)
		if !supportedReleaseLanguages[normalized] {
			return types.AgentReleaseManifest{}, fmt.Errorf(
				"experts: release bundle: unsupported language %q (supported: en, zh)", lang)
		}
		if !containsString(manifest.SupportedLanguages, normalized) {
			manifest.SupportedLanguages = append(manifest.SupportedLanguages, normalized)
		}
	}
	if len(manifest.SupportedLanguages) == 0 {
		return types.AgentReleaseManifest{}, fmt.Errorf(
			"experts: release bundle: at least one supported language is required (supported: en, zh)")
	}

	if manifest.LicenseID == "" {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: the license id is required")
	}
	if !releaseSemanticVersion.MatchString(manifest.SemanticVersion) || len(manifest.SemanticVersion) > 64 {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: semantic version must be a valid SemVer value")
	}
	if manifest.DisplayName == "" || len(manifest.DisplayName) > 255 {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: display name is required and must be at most 255 characters")
	}
	if len(manifest.UseCases) == 0 {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: at least one use case is required")
	}
	if manifest.MinimumWeKnoraCapability == "" {
		return types.AgentReleaseManifest{}, fmt.Errorf("experts: release bundle: the minimum weknora capability is required")
	}
	return manifest, nil
}

// canonicalDependencyLock validates every entry as immutable (concrete
// version that is not a moving reference, plus an exact SHA-256 digest) and
// redistributable/licensed (a license id), rejects duplicate identities and
// returns the lock in canonical (type, id) order so the same dependency set
// always serializes to the same bytes.
func canonicalDependencyLock(lock types.DependencyLock) (types.DependencyLock, error) {
	canonical := types.DependencyLock{Dependencies: make([]types.AgentReleaseDependency, 0, len(lock.Dependencies))}
	seen := make(map[string]bool, len(lock.Dependencies))
	for _, dep := range lock.Dependencies {
		dep.Type = strings.TrimSpace(dep.Type)
		dep.ID = strings.TrimSpace(dep.ID)
		dep.Version = strings.TrimSpace(dep.Version)
		dep.Digest = strings.ToLower(strings.TrimSpace(dep.Digest))
		dep.LicenseID = strings.TrimSpace(dep.LicenseID)

		if dep.Type == "" || dep.ID == "" {
			return types.DependencyLock{}, fmt.Errorf("experts: release bundle: every dependency needs a type and a stable id")
		}
		if dep.Version == "" || strings.EqualFold(dep.Version, "latest") {
			return types.DependencyLock{}, fmt.Errorf(
				"experts: release bundle: dependency %s %q has no stable immutable version", dep.Type, dep.ID)
		}
		if !isHex64(dep.Digest) {
			return types.DependencyLock{}, fmt.Errorf(
				"experts: release bundle: dependency %s %q has no sha-256 content digest", dep.Type, dep.ID)
		}
		if dep.LicenseID == "" {
			return types.DependencyLock{}, fmt.Errorf(
				"experts: release bundle: dependency %s %q has no license permitting redistribution", dep.Type, dep.ID)
		}

		key := dep.Type + "\x00" + dep.ID
		if seen[key] {
			return types.DependencyLock{}, fmt.Errorf(
				"experts: release bundle: duplicate dependency %s %q in the lock", dep.Type, dep.ID)
		}
		seen[key] = true
		canonical.Dependencies = append(canonical.Dependencies, dep)
	}
	sort.SliceStable(canonical.Dependencies, func(i, j int) bool {
		if canonical.Dependencies[i].Type != canonical.Dependencies[j].Type {
			return canonical.Dependencies[i].Type < canonical.Dependencies[j].Type
		}
		return canonical.Dependencies[i].ID < canonical.Dependencies[j].ID
	})
	return canonical, nil
}

// releasePayload projects the frozen agent onto the portable allow-list —
// the same M2 §5 whitelist family MaterializePublishedAgent uses: prompt
// material and persona, tool allowlist, skill/subagent references and
// starter prompts. Everything else in CustomAgent.Config is tenant-local
// and must never enter the bundle. Reference lists reuse referenceList's
// safe-base-name discipline (they become names the installer resolves).
func releasePayload(agent *types.CustomAgent) (types.AgentReleasePayload, error) {
	cfg := agent.Config

	skills, err := referenceList(cfg.SelectedSkills, "skill")
	if err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("experts: release bundle: agent %q: %w", agent.ID, err)
	}
	subagents, err := referenceList(cfg.Subagents, "subagent")
	if err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("experts: release bundle: agent %q: %w", agent.ID, err)
	}
	tools, err := referenceList(cfg.AllowedTools, "tool")
	if err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("experts: release bundle: agent %q: %w", agent.ID, err)
	}

	starters := make([]string, 0)
	if suggestions := cfg.QuestionSuggestions; suggestions != nil {
		for _, item := range suggestions.Starters.Items {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" || containsString(starters, trimmed) {
				continue
			}
			starters = append(starters, trimmed)
		}
	}

	return types.AgentReleasePayload{
		AgentMode:      strings.TrimSpace(cfg.AgentMode),
		SystemPrompt:   strings.TrimSpace(cfg.SystemPrompt),
		PersonaMBTI:    strings.TrimSpace(cfg.PersonaMBTI),
		PersonaStyle:   strings.TrimSpace(cfg.PersonaStyle),
		AllowedTools:   tools,
		Skills:         skills,
		Subagents:      subagents,
		StarterPrompts: starters,
	}, nil
}

// canonicalReleaseJSON serializes deterministically: struct-field order
// fixed by the type, no HTML escaping (so text survives byte-for-byte
// comparisons), and the encoder's trailing newline dropped — identical
// values always yield identical bytes.
func canonicalReleaseJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// trimmedList trims entries, drops blanks and de-duplicates, preserving
// first-appearance order. Nil becomes an empty slice so canonical JSON
// always emits an array, never null.
func trimmedList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || containsString(out, trimmed) {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

// normalizeReleaseLanguage maps a BCP-47-ish tag onto its primary subtag:
// trim, lowercase, cut at the first "-" ("zh-CN" → "zh").
func normalizeReleaseLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexByte(lang, '-'); i >= 0 {
		lang = lang[:i]
	}
	return lang
}

// isHex64 reports whether s is exactly 64 lowercase hex characters — the
// SHA-256 digest shape used across the version/marketplace boundary.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
