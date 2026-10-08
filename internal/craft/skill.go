package craft

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Versioned Skills (C06). A skill is pinned by name AND content digest: the
// pin is the only identity a delegation may rely on, and skill TEXT —
// instructions or manifest alike — never grants any permission (total plan
// line 281). Permissions come exclusively from the existing authorization
// layers; a skill can only describe what it needs, never what it is allowed
// to do.

// SkillPin pins one skill version by install name and content digest. Two
// pins are the same skill only when BOTH fields match: a changed skill can
// never silently upgrade a run that pinned the old content.
type SkillPin struct {
	Name, Digest string
}

// SameSkillPin reports whether actual is exactly the pinned skill: a
// non-empty name, a non-empty digest and full equality. Everything else —
// same name with a different digest, an incomplete pin — is NOT the pinned
// skill.
func SameSkillPin(expected, actual SkillPin) bool {
	return expected.Name != "" && expected.Digest != "" && expected == actual
}

// SkillVersionDigest derives the pinned digest of one skill version from its
// install name and its file manifest: the file manifest is content-sorted
// (ManifestDigest), then bound under the skill name so two skills sharing
// file content still pin differently. The digest is a pure function of the
// content, so a restore re-derives the exact pin a run recorded.
func SkillVersionDigest(name string, files []File) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("%w: skill digest requires a name", ErrInvalidInput)
	}
	manifest, err := ManifestDigest(files)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("craft-skill\x00" + name + "\x00" + manifest))
	return hex.EncodeToString(sum[:]), nil
}

// SkillPinOf builds the pin of one skill version from its content.
func SkillPinOf(name string, files []File) (SkillPin, error) {
	digest, err := SkillVersionDigest(name, files)
	if err != nil {
		return SkillPin{}, err
	}
	return SkillPin{Name: name, Digest: digest}, nil
}

// Skill recovery routes. The vocabulary deliberately mirrors the C04
// recovery program: a verified pin reuses the pinned content, everything
// else waits durably.
const (
	// SkillRouteReuse answers with the pinned content, digest verified.
	SkillRouteReuse = "reuse"
	// SkillRouteWait parks the run durably until the pinned content is
	// available again. It is the ONLY answer for a missing or changed skill.
	SkillRouteWait = "wait"
)

// SkillRecoveryRoute chooses the only safe continuation for one pinned skill
// at recovery:
//
//   - a pin that matches the available content verbatim reuses it;
//   - a MISSING skill waits explicitly — the run wanted that exact version,
//     and substituting whatever is installed now would silently execute
//     different instructions;
//   - a CHANGED skill (same name, different digest) also waits: the pinned
//     content stays retained until the Runs/snapshots that pinned it expire
//     (SkillContentRetained), so waiting can still succeed.
//
// A task that pinned no skill has no skill dependency to verify.
func SkillRecoveryRoute(pinned, available SkillPin, present bool) string {
	if pinned.Name == "" && pinned.Digest == "" {
		return SkillRouteReuse
	}
	if present && SameSkillPin(pinned, available) {
		return SkillRouteReuse
	}
	return SkillRouteWait
}

// SkillContentRetained reports whether one pinned skill version's content
// must still be served: retention follows the Runs and snapshots that pinned
// it, so old content may disappear only after the LAST expiry passed. A nil
// or empty expiry list pins nothing and retains nothing.
func SkillContentRetained(expiries []time.Time, now time.Time) bool {
	for _, at := range expiries {
		if now.Before(at) {
			return true
		}
	}
	return false
}

// SkillManifest is the description-only manifest shipped beside a skill's
// SKILL.md. It records WHAT the skill produces (artifact kind) and WHICH
// tools it needs to run; it carries no authorization decision of any kind —
// a manifest never opens a permission the caller does not already hold.
type SkillManifest struct {
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	Digest           string   `json:"digest"`
	DigestOf         string   `json:"digest_of,omitempty"`
	ArtifactKind     string   `json:"artifact_kind"`
	ToolRequirements []string `json:"tool_requirements"`
}

// skillAuthorityKeys are manifest keys that would turn a description into an
// authorization decision. Any of them rejects the whole manifest.
var skillAuthorityKeys = []string{
	"permissions", "permission", "grants", "grant", "admin", "sudo",
	"allow", "allowed", "authorize", "authorized", "scopes", "scope",
	"credentials", "credential", "secrets", "secret", "roles", "role",
}

// skillAuthorityToolRequirements names a tool may not declare needing:
// privileged/elevated execution is not a tool requirement a skill can state.
var skillAuthorityToolRequirements = []string{"sudo", "admin", "root", "su "}

// DecodeSkillManifest parses one skill manifest strictly:
//
//   - unknown fields reject — nothing can be smuggled past the contract;
//   - name, version, digest and a KNOWN artifact kind are required, and the
//     digest must be the canonical 64-hex form;
//   - tool requirements may only name tools; privileged execution
//     (sudo/admin/root) is refused;
//   - any authority vocabulary (permissions, grants, credentials, scopes,
//     roles…) rejects the manifest: skill text claiming to need or bestow
//     admin privileges is refused by this layer, exactly as the existing
//     permission layers refuse to honor it at execution time.
func DecodeSkillManifest(raw []byte) (SkillManifest, error) {
	var m SkillManifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest decode: %v", ErrInvalidInput, err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest carries trailing content", ErrInvalidInput)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest requires name and version", ErrInvalidInput)
	}
	if !ValidSHA256(m.Digest) {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest digest %q", ErrInvalidInput, m.Digest)
	}
	if _, known := EntryPath(m.ArtifactKind); !known {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest artifact kind %q", ErrInvalidInput, m.ArtifactKind)
	}
	for _, req := range m.ToolRequirements {
		lower := strings.ToLower(strings.TrimSpace(req))
		if lower == "" {
			return SkillManifest{}, fmt.Errorf("%w: empty tool requirement", ErrInvalidInput)
		}
		for _, banned := range skillAuthorityToolRequirements {
			if strings.Contains(lower, banned) {
				return SkillManifest{}, fmt.Errorf("%w: tool requirement %q claims privileged execution", ErrInvalidInput, req)
			}
		}
	}
	// Authority scan over the raw object: a manifest that claims ANY
	// permission-granting key is refused, whatever its value.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return SkillManifest{}, fmt.Errorf("%w: skill manifest scan: %v", ErrInvalidInput, err)
	}
	for _, key := range skillAuthorityKeys {
		if _, ok := probe[key]; ok {
			return SkillManifest{}, fmt.Errorf("%w: skill manifest key %q is an authorization decision; skill text grants no permissions", ErrInvalidInput, key)
		}
	}
	return m, nil
}
