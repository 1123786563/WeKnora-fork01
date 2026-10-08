package craft

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"
)

// The brief's RED test, verbatim: a pinned skill can never silently upgrade —
// the same name with a different digest is NOT the same skill.
func TestSkillPinCannotSilentlyUpgrade(t *testing.T) {
	if SameSkillPin(SkillPin{"web", "a"}, SkillPin{"web", "b"}) {
		t.Fatal("silent upgrade")
	}
	if !SameSkillPin(SkillPin{"web", "a"}, SkillPin{"web", "a"}) {
		t.Fatal("same pin rejected")
	}
}

// An incomplete pin matches nothing — a missing name or digest is not
// "any version of this skill".
func TestSkillPinRequiresCompleteIdentity(t *testing.T) {
	if SameSkillPin(SkillPin{}, SkillPin{}) {
		t.Fatal("empty pin matched")
	}
	if SameSkillPin(SkillPin{Name: "web"}, SkillPin{Name: "web"}) {
		t.Fatal("digest-less pin matched")
	}
	if SameSkillPin(SkillPin{Digest: "a"}, SkillPin{Digest: "a"}) {
		t.Fatal("name-less pin matched")
	}
}

// The pinned digest follows the skill's content: same files same digest,
// changed content different digest, and file ORDER cannot change identity.
func TestSkillVersionDigestFollowsContent(t *testing.T) {
	files := []File{
		{Path: "SKILL.md", SHA256: sha256Of(t, []byte("instructions")), Bytes: 12},
		{Path: "manifest.json", SHA256: sha256Of(t, []byte("{}")), Bytes: 2},
	}
	first, err := SkillVersionDigest("craft-web-report", files)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SkillVersionDigest("craft-web-report", []File{files[1], files[0]})
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatal("listing order changed the skill digest")
	}
	changed, err := SkillVersionDigest("craft-web-report", []File{
		{Path: "SKILL.md", SHA256: sha256Of(t, []byte("instructions v2")), Bytes: 15},
		{Path: "manifest.json", SHA256: sha256Of(t, []byte("{}")), Bytes: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("changed content kept the skill digest")
	}
	otherName, err := SkillVersionDigest("craft-web-report-v2", files)
	if err != nil {
		t.Fatal(err)
	}
	if first == otherName {
		t.Fatal("skill name is not part of the digest")
	}
	pin, err := SkillPinOf("craft-web-report", files)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Name != "craft-web-report" || pin.Digest != first {
		t.Fatalf("pin %+v does not carry the derived digest %s", pin, first)
	}
	if _, err := SkillVersionDigest("x", []File{{Path: "SKILL.md", SHA256: "nothex", Bytes: 1}}); err == nil {
		t.Fatal("malformed file digest accepted")
	}
}

// Recovery verifies the pinned digest: only a verbatim match reuses the
// skill; a missing or changed skill WAITS durably — never a silent swap to
// newer content under the same name.
func TestSkillRecoveryWaitsOnMissingOrChangedSkill(t *testing.T) {
	pinned := SkillPin{Name: "craft-web-report", Digest: "d1"}
	if got := SkillRecoveryRoute(pinned, SkillPin{Name: "craft-web-report", Digest: "d1"}, true); got != SkillRouteReuse {
		t.Fatalf("verbatim match must reuse, got %s", got)
	}
	if got := SkillRecoveryRoute(pinned, SkillPin{}, false); got != SkillRouteWait {
		t.Fatalf("missing skill must wait, got %s", got)
	}
	if got := SkillRecoveryRoute(pinned, SkillPin{Name: "craft-web-report", Digest: "d2"}, true); got != SkillRouteWait {
		t.Fatalf("changed skill must wait, got %s", got)
	}
	if got := SkillRecoveryRoute(SkillPin{}, SkillPin{}, false); got != SkillRouteReuse {
		t.Fatalf("no pin has no skill dependency, got %s", got)
	}
}

// Old skill content is retained exactly as long as some Run or snapshot that
// pinned it has not expired yet; once every expiry passed, the content may go.
func TestSkillContentRetainedUntilLastExpiry(t *testing.T) {
	now := time.Now()
	expiries := []time.Time{now.Add(-time.Hour), now.Add(time.Hour)}
	if !SkillContentRetained(expiries, now) {
		t.Fatal("content dropped while a pinning run is still live")
	}
	if SkillContentRetained([]time.Time{now.Add(-time.Hour)}, now) {
		t.Fatal("content retained after every expiry passed")
	}
	if SkillContentRetained(nil, now) {
		t.Fatal("content retained with nothing pinning it")
	}
}

// The manifest is description only: a skill that claims authority — admin
// permissions, grants, sudo, elevated scopes — is REJECTED, because skill
// text never grants permissions. A well-formed manifest with only artifact
// and tool-requirement facts decodes.
func TestSkillManifestRejectsAuthorityClaims(t *testing.T) {
	good := `{"name":"craft-web-report","version":"1.0.0","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","tool_requirements":["shell_exec"]}`
	m, err := DecodeSkillManifest([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "craft-web-report" || m.Version != "1.0.0" || m.ArtifactKind != KindWeb {
		t.Fatalf("unexpected manifest %+v", m)
	}
	if len(m.ToolRequirements) != 1 || m.ToolRequirements[0] != "shell_exec" {
		t.Fatalf("tool requirements lost: %+v", m.ToolRequirements)
	}

	claims := []string{
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","permissions":["admin"]}`,
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","grants":["knowledge:read"]}`,
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","tool_requirements":["sudo"]}`,
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","admin":true}`,
	}
	for _, raw := range claims {
		if _, err := DecodeSkillManifest([]byte(raw)); err == nil {
			t.Fatalf("authority claim accepted: %s", raw)
		}
	}

	// Shape rules: unknown fields refuse (no smuggling), required fields
	// and a well-formed digest are mandatory.
	bad := []string{
		``,
		`{}`,
		`{"name":"s","version":"1","digest":"zz","artifact_kind":"web"}`,
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"database"}`,
		`{"name":"s","version":"1","digest":"0000000000000000000000000000000000000000000000000000000000000000","artifact_kind":"web","extra":1}`,
	}
	for _, raw := range bad {
		if _, err := DecodeSkillManifest([]byte(raw)); err == nil {
			t.Fatalf("malformed manifest accepted: %s", raw)
		}
	}
}

// The shipped craft-web-report skill satisfies its own contract: the
// manifest decodes under the strict rules and its digest pins the actual
// SKILL.md bytes shipped beside it.
func TestShippedWebReportSkillManifest(t *testing.T) {
	raw, err := osReadFile("../../skills/craft-web-report/manifest.json")
	if err != nil {
		t.Fatalf("read shipped manifest: %v", err)
	}
	m, err := DecodeSkillManifest(raw)
	if err != nil {
		t.Fatalf("shipped manifest rejected by its own contract: %v", err)
	}
	skillMD, err := osReadFile("../../skills/craft-web-report/SKILL.md")
	if err != nil {
		t.Fatalf("read shipped SKILL.md: %v", err)
	}
	if want := sha256Of(t, skillMD); m.Digest != want {
		t.Fatalf("shipped digest %s does not pin SKILL.md (%s)", m.Digest, want)
	}
	if m.DigestOf != "SKILL.md" {
		t.Fatalf("digest_of must name SKILL.md, got %q", m.DigestOf)
	}
}

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func sha256Of(t *testing.T, content []byte) string {
	t.Helper()
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
