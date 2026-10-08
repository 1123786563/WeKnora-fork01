package craft

// T13 (#133) — the restricted derived-export consent rules as pure
// functions over the T00 frozen DTOs. These tests pin the classification
// (which derived files are restricted by their RECORDED origins), the
// safe-bundle projection, the decision binding and the single rule that
// turns a persisted decision into export authority for exactly one
// immutable manifest identity:
//
//   - a derived file is restricted derived iff ANY recorded origin is
//     restricted — per file, never per manifest;
//   - the safe manifest drops exactly the restricted derived files and
//     re-derives its own digest (a different bundle, honestly labelled);
//   - a decision binds exactly one Version ID + Export Manifest digest;
//   - authority needs approval + binding + the CURRENT owner: rejected,
//     stale (other digest/version) and former-owner decisions grant
//     nothing;
//   - the consent state reduces all of it to what members see.
import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// t13DigestOf derives a well-formed SHA-256 hex digest from a seed so the
// test facts satisfy the same shape production records carry.
func t13DigestOf(seed string) string {
	sum := sha256.Sum256([]byte("t13-" + seed))
	return hex.EncodeToString(sum[:])
}

func t13OwnOrigin() ExportOriginRef {
	return ExportOriginRef{Kind: ExportOriginKnowledge, Ref: "craftkb://kb/kb-own/k-own/c-own", SHA256: t13DigestOf("own"), Restricted: false}
}

func t13SharedOrigin() ExportOriginRef {
	return ExportOriginRef{Kind: ExportOriginKnowledge, Ref: "craftkb://kb/kb-shared/k-shared/c-shared", SHA256: t13DigestOf("shared"), Restricted: true}
}

// t13Manifest builds one validated manifest whose files alternate between
// unrestricted-derived and restricted-derived members.
func t13Manifest(t *testing.T) ExportManifest {
	t.Helper()
	m := ExportManifest{
		VersionID: "ver_" + t13DigestOf("v"),
		Files: []ExportFile{
			{Path: "index.html", SHA256: t13DigestOf("index"), Restricted: false, Origins: []ExportOriginRef{t13OwnOrigin()}},
			{Path: "report.html", SHA256: t13DigestOf("report"), Restricted: false, Origins: []ExportOriginRef{t13OwnOrigin(), t13SharedOrigin()}},
			{Path: "clean.txt", SHA256: t13DigestOf("clean"), Restricted: false, Origins: []ExportOriginRef{t13OwnOrigin()}},
		},
	}
	digest, err := ExportManifestDigest(m)
	require.NoError(t, err)
	m.ManifestDigest = digest
	require.NoError(t, m.Validate())
	return m
}

func TestCraftT13DerivedFileClassification(t *testing.T) {
	m := t13Manifest(t)

	restricted := RestrictedDerivedPaths(m)
	require.Equal(t, []string{"report.html"}, restricted, "exactly the file with a restricted recorded origin is restricted derived")
	require.Len(t, restricted, 1, "per-file classification, never all-or-nothing")

	// The restricted flag ON the file marks restricted ORIGINALS (which never
	// enter a version manifest); the classification reads the ORIGINS.
	flagged := m.Files[1]
	flagged.Restricted = false
	require.True(t, DerivedFileRestricted(flagged), "the origin facts decide, not the original-material flag")
	clean := m.Files[0]
	require.False(t, DerivedFileRestricted(clean))
}

func TestCraftT13SafeExportManifest(t *testing.T) {
	m := t13Manifest(t)

	safe, err := SafeExportManifest(m)
	require.NoError(t, err)
	require.NoError(t, safe.Validate())
	require.Equal(t, m.VersionID, safe.VersionID, "the safe bundle stays bound to the same version")
	require.NotEqual(t, m.ManifestDigest, safe.ManifestDigest, "the safe bundle is a different bundle: its digest differs")
	require.Len(t, safe.Files, 2, "exactly the restricted derived file is withheld")
	paths := map[string]bool{}
	for _, f := range safe.Files {
		paths[f.Path] = true
		require.False(t, DerivedFileRestricted(f), "no safe member carries a restricted origin")
	}
	require.True(t, paths["index.html"] && paths["clean.txt"])
	require.False(t, paths["report.html"])

	// The withheld members' facts are unchanged in the full manifest —
	// filtering never rewrites the recorded facts.
	require.Equal(t, m.Files, t13Manifest(t).Files)

	// An all-restricted manifest degrades to an honestly empty safe bundle.
	all := ExportManifest{
		VersionID: m.VersionID,
		Files:     []ExportFile{m.Files[1]},
	}
	digest, err := ExportManifestDigest(all)
	require.NoError(t, err)
	all.ManifestDigest = digest
	empty, err := SafeExportManifest(all)
	require.NoError(t, err)
	require.Empty(t, empty.Files)
	require.NotEqual(t, all.ManifestDigest, empty.ManifestDigest)
	require.NoError(t, empty.Validate(), "an empty safe manifest is a first-class validated state")
}

func TestCraftT13ExportDecisionBinding(t *testing.T) {
	m := t13Manifest(t)
	decision := ExportDecision{
		VersionID: m.VersionID, ManifestDigest: m.ManifestDigest,
		OwnerID: "u-owner", Decision: DecisionApproved,
	}
	require.NoError(t, decision.Validate())
	require.True(t, ExportDecisionBinds(decision, m.VersionID, m.ManifestDigest))
	require.False(t, ExportDecisionBinds(decision, "ver_"+t13DigestOf("other"), m.ManifestDigest), "another version unbinds")
	require.False(t, ExportDecisionBinds(decision, m.VersionID, t13DigestOf("changed")), "a changed manifest digest unbinds")
	invalid := decision
	invalid.OwnerID = ""
	require.Error(t, invalid.Validate())
	require.False(t, ExportDecisionBinds(invalid, m.VersionID, m.ManifestDigest))
}

func TestCraftT13GrantsExportAuthority(t *testing.T) {
	m := t13Manifest(t)
	approved := ExportDecision{
		VersionID: m.VersionID, ManifestDigest: m.ManifestDigest,
		OwnerID: "u-owner", Decision: DecisionApproved,
	}
	require.True(t, GrantsExportAuthority(&approved, m.VersionID, m.ManifestDigest, "u-owner"))

	// Rejected decisions never grant.
	rejected := approved
	rejected.Decision = DecisionRejected
	require.False(t, GrantsExportAuthority(&rejected, m.VersionID, m.ManifestDigest, "u-owner"))

	// Stale decisions (a changed manifest, another version) never grant —
	// replay cannot broaden scope.
	stale := approved
	stale.ManifestDigest = t13DigestOf("old")
	require.False(t, GrantsExportAuthority(&stale, m.VersionID, m.ManifestDigest, "u-owner"))
	require.False(t, GrantsExportAuthority(&approved, m.VersionID, t13DigestOf("new"), "u-owner"))

	// Only the CURRENT owner's decision grants.
	require.False(t, GrantsExportAuthority(&approved, m.VersionID, m.ManifestDigest, "u-new-owner"), "a former owner's decision stops granting")
	require.False(t, GrantsExportAuthority(&approved, m.VersionID, m.ManifestDigest, ""), "an unresolvable owner grants nothing")

	// Missing or invalid decisions grant nothing.
	require.False(t, GrantsExportAuthority(nil, m.VersionID, m.ManifestDigest, "u-owner"))
	broken := approved
	broken.Decision = DecisionStatus("bogus")
	require.False(t, GrantsExportAuthority(&broken, m.VersionID, m.ManifestDigest, "u-owner"))
}

func TestCraftT13ExportConsentStateOf(t *testing.T) {
	m := t13Manifest(t)
	approved := ExportDecision{
		VersionID: m.VersionID, ManifestDigest: m.ManifestDigest,
		OwnerID: "u-owner", Decision: DecisionApproved,
	}
	rejected := approved
	rejected.Decision = DecisionRejected

	// No restricted derived data → nothing to consent.
	require.Equal(t, ExportConsentNone, ExportConsentStateOf(nil, &approved, m.VersionID, m.ManifestDigest, "u-owner"))
	require.Equal(t, ExportConsentNone, ExportConsentStateOf(nil, nil, m.VersionID, m.ManifestDigest, "u-owner"))

	// Restricted derived data without a binding decision awaits the owner.
	require.Equal(t, ExportConsentAwaiting, ExportConsentStateOf([]string{"report.html"}, nil, m.VersionID, m.ManifestDigest, "u-owner"))
	stale := approved
	stale.ManifestDigest = t13DigestOf("old")
	require.Equal(t, ExportConsentAwaiting, ExportConsentStateOf([]string{"report.html"}, &stale, m.VersionID, m.ManifestDigest, "u-owner"), "a stale decision is history: the state awaits again")
	require.Equal(t, ExportConsentAwaiting, ExportConsentStateOf([]string{"report.html"}, &approved, m.VersionID, m.ManifestDigest, "u-new-owner"), "a former owner's consent is not live authority")

	// An explicit rejection by the CURRENT owner is visible as declined.
	require.Equal(t, ExportConsentDeclined, ExportConsentStateOf([]string{"report.html"}, &rejected, m.VersionID, m.ManifestDigest, "u-owner"))
	// Round-1 OCR: a former owner's rejection is history too — after an
	// ownership move the state awaits the new owner's decision again,
	// whichever way the old decision went (symmetric with the approved
	// path above).
	require.Equal(t, ExportConsentAwaiting, ExportConsentStateOf([]string{"report.html"}, &rejected, m.VersionID, m.ManifestDigest, "u-new-owner"),
		"a former owner's rejection must not attribute the current ownership with a decision")
	require.Equal(t, ExportConsentAwaiting, ExportConsentStateOf([]string{"report.html"}, &rejected, m.VersionID, m.ManifestDigest, ""),
		"an unresolvable current owner never surfaces a recorded rejection")

	// The live authority state needs everything.
	require.Equal(t, ExportConsentConsented, ExportConsentStateOf([]string{"report.html"}, &approved, m.VersionID, m.ManifestDigest, "u-owner"))
}

func TestCraftT13ExportConsentStateValid(t *testing.T) {
	for _, state := range []ExportConsentState{ExportConsentNone, ExportConsentAwaiting, ExportConsentConsented, ExportConsentDeclined} {
		require.True(t, state.Valid())
	}
	require.False(t, ExportConsentState("bogus").Valid())
}
