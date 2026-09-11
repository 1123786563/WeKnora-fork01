package trpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CapabilitySnapshot is the immutable, serializable capability contract used
// to rebuild a tRPC run after a process restart. Live registries and providers
// are deliberately excluded; only stable identities and prompt state cross a
// checkpoint boundary.
type CapabilitySnapshot struct {
	ToolIdentities  []string          `json:"tool_identities,omitempty"`
	DeferredNames   []string          `json:"deferred_names,omitempty"`
	SkillDigests    map[string]string `json:"skill_digests,omitempty"`
	SystemPrompt    string            `json:"system_prompt,omitempty"`
	MemoryPrompt    string            `json:"memory_prompt,omitempty"`
	ImageReferences []string          `json:"image_references,omitempty"`
}

func (s CapabilitySnapshot) IsEmpty() bool {
	return len(s.ToolIdentities) == 0 && len(s.DeferredNames) == 0 && len(s.SkillDigests) == 0 && s.SystemPrompt == "" && s.MemoryPrompt == "" && len(s.ImageReferences) == 0
}

func (s CapabilitySnapshot) Validate() error {
	if hasDuplicate(s.ToolIdentities) || hasDuplicate(s.DeferredNames) || hasDuplicate(s.ImageReferences) {
		return fmt.Errorf("capability snapshot contains duplicate identities")
	}
	for _, name := range s.DeferredNames {
		found := false
		for _, identity := range s.ToolIdentities {
			if identity == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("deferred tool %q is not registered", name)
		}
	}
	for name, digest := range s.SkillDigests {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(digest) == "" {
			return fmt.Errorf("skill digest has empty identity")
		}
	}
	return nil
}

// CompatibleWith rejects a resumed run when its advertised capabilities have
// changed. This prevents a skill or deferred MCP definition from changing
// underneath a pending model plan without an explicit new run.
func (s CapabilitySnapshot) CompatibleWith(current CapabilitySnapshot) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := current.Validate(); err != nil {
		return err
	}
	if !equalStrings(s.ToolIdentities, current.ToolIdentities) || !equalStrings(s.DeferredNames, current.DeferredNames) {
		return fmt.Errorf("tool capability set changed")
	}
	if !equalStringMap(s.SkillDigests, current.SkillDigests) {
		return fmt.Errorf("skill capability digest changed")
	}
	return nil
}

func MarshalCapabilitySnapshot(s CapabilitySnapshot) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(s)
}

func UnmarshalCapabilitySnapshot(raw []byte) (CapabilitySnapshot, error) {
	var s CapabilitySnapshot
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return CapabilitySnapshot{}, err
	}
	if err := s.Validate(); err != nil {
		return CapabilitySnapshot{}, err
	}
	return s, nil
}

// SkillDigest creates a deterministic digest from the metadata used in the
// existing system prompt. It is stable across process restarts and does not
// hash host absolute paths.
func SkillDigest(name, description string) string {
	h := sha256.Sum256([]byte(name + "\x00" + description))
	return "sha256:" + hex.EncodeToString(h[:])
}

func hasDuplicate(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if seen[v] {
			return true
		}
		seen[v] = true
	}
	return false
}
func equalStrings(a, b []string) bool {
	aa, bb := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	if len(aa) != len(bb) {
		return false
	}
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
