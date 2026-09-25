package craft

// T11 (#128) module rules: the restricted-share consent facts. These tests
// pin the pure domain seam every service and handler answer reduces to:
//
//   - restricted contribution is DERIVED from recorded evidence only — the
//     version's admitted citation manifest supplies the evidence digest and
//     the Run's immutable knowledge record supplies the restricted fact: a
//     cross-tenant (organization-shared library) original that contributed
//     to the Run marks the version restricted;
//   - a consent binds the immutable Version ID AND the evidence digest;
//     any drift unbinds it;
//   - only a live, unexpired, unrevoked APPROVED decision that binds the
//     current contribution grants sharing authority — reject, expiry,
//     replay (stale/unbound) and revocation never do.
import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func t11FactManifest(ids ...string) WebCitationManifest {
	entries := make([]WebCitationEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, WebCitationEntry{Kind: WebCitationFact, CitationID: id, Claim: "claim of " + id})
	}
	return WebCitationManifest{Schema: WebCitationSchema, Entries: entries}
}

func t11Record(sources ...KnowledgeSourceRecord) KnowledgeRecord {
	return KnowledgeRecord{
		Scope:            Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-craft"},
		RunID:            "run-t11",
		PublicationState: KnowledgePublicationPublished,
		Sources:          sources,
	}
}

func TestCraftT11RestrictedContributionFromRecordedEvidence(t *testing.T) {
	shared := KnowledgeSourceRecord{ID: "kc_" + strings.Repeat("a", 24), Ref: "craftkb://kb/k-shared/knowledge/k-s/chunk/c-s", Digest: "d-shared", TenantID: 7}
	own := KnowledgeSourceRecord{ID: "kc_" + strings.Repeat("b", 24), Ref: "craftkb://kb/k-own/knowledge/k-o/chunk/c-o", Digest: "d-own", TenantID: 1}
	record := t11Record(shared, own)

	// A version whose Run recorded the organization-shared original is
	// restricted — the restricted original CONTRIBUTED to the derived
	// result — and the digest is exactly the manifest's evidence digest.
	manifest := t11FactManifest(shared.ID, own.ID)
	contribution, err := RestrictedContributionFrom("v-1", manifest, record)
	require.NoError(t, err)
	require.True(t, contribution.Restricted, "a cross-tenant recorded source marks the contribution restricted")
	digest, err := WebCitationsDigest(manifest)
	require.NoError(t, err)
	require.Equal(t, digest, contribution.EvidenceDigest, "the contribution carries the manifest's stable evidence digest")
	require.Equal(t, "v-1", contribution.VersionID)
	require.NoError(t, contribution.Validate())

	// A Run that recorded only same-tenant originals is not restricted.
	sameTenant, err := RestrictedContributionFrom("v-2", t11FactManifest(own.ID), t11Record(own))
	require.NoError(t, err)
	require.False(t, sameTenant.Restricted)

	// The restriction follows CONTRIBUTION, not the rendered citations: a
	// version whose manifest cites nothing still derives from the shared
	// original the Run consumed.
	uncited, err := RestrictedContributionFrom("v-3", WebCitationManifest{Schema: WebCitationSchema, Entries: []WebCitationEntry{}}, record)
	require.NoError(t, err)
	require.True(t, uncited.Restricted)

	// The identity inputs are mandatory.
	_, err = RestrictedContributionFrom("", manifest, record)
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = RestrictedContributionFrom("v-1", WebCitationManifest{Schema: 99, Entries: []WebCitationEntry{}}, record)
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestCraftT11DecisionBindsVersionAndEvidence(t *testing.T) {
	shared := KnowledgeSourceRecord{ID: "kc_" + strings.Repeat("a", 24), TenantID: 7}
	record := t11Record(shared)
	contribution, err := RestrictedContributionFrom("v-1", t11FactManifest(shared.ID), record)
	require.NoError(t, err)

	exact := ShareDecision{VersionID: "v-1", EvidenceDigest: contribution.EvidenceDigest, OwnerID: "u-owner", Decision: DecisionApproved}
	require.NoError(t, exact.Validate())
	require.True(t, DecisionBinds(exact, contribution), "the consent binds the exact version and evidence digest")

	require.False(t, DecisionBinds(ShareDecision{VersionID: "v-2", EvidenceDigest: contribution.EvidenceDigest, OwnerID: "u-owner", Decision: DecisionApproved}, contribution), "another version never inherits a consent")
	require.False(t, DecisionBinds(ShareDecision{VersionID: "v-1", EvidenceDigest: strings.Repeat("0", 64), OwnerID: "u-owner", Decision: DecisionApproved}, contribution), "changed evidence unbinds a prior consent")
}

func TestCraftT11ShareAuthorityRules(t *testing.T) {
	shared := KnowledgeSourceRecord{ID: "kc_" + strings.Repeat("a", 24), TenantID: 7}
	record := t11Record(shared)
	contribution, err := RestrictedContributionFrom("v-1", t11FactManifest(shared.ID), record)
	require.NoError(t, err)
	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

	approved := &RecordedShareDecision{Decision: ShareDecision{VersionID: "v-1", EvidenceDigest: contribution.EvidenceDigest, OwnerID: "u-owner", Decision: DecisionApproved}, DecidedAt: now.Add(-time.Hour)}
	require.NoError(t, approved.Validate())

	// The single positive rule: live approved consent that binds this
	// contribution grants authority inside its TTL window.
	require.True(t, GrantsShareAuthority(contribution, approved, now))
	require.Equal(t, ShareStateConsented, ShareStateOf(contribution, approved, now))

	// Rejected, unknown, missing, expired, revoked and unbound decisions
	// never create sharing authority.
	rejected := &RecordedShareDecision{Decision: ShareDecision{VersionID: "v-1", EvidenceDigest: contribution.EvidenceDigest, OwnerID: "u-owner", Decision: DecisionRejected}, DecidedAt: now.Add(-time.Hour)}
	require.False(t, GrantsShareAuthority(contribution, rejected, now))
	require.Equal(t, ShareStateDeclined, ShareStateOf(contribution, rejected, now))

	unknown := &RecordedShareDecision{Decision: ShareDecision{VersionID: "v-1", EvidenceDigest: contribution.EvidenceDigest, OwnerID: "u-owner", Decision: DecisionUnknown}, DecidedAt: now.Add(-time.Hour)}
	require.False(t, GrantsShareAuthority(contribution, unknown, now))

	require.False(t, GrantsShareAuthority(contribution, nil, now), "a restricted version is private without any owner decision")
	require.Equal(t, ShareStatePrivate, ShareStateOf(contribution, nil, now))

	expired := &RecordedShareDecision{Decision: approved.Decision, DecidedAt: now.Add(-ShareDecisionTTL - time.Minute)}
	require.False(t, GrantsShareAuthority(contribution, expired, now), "an expired consent grants nothing")
	require.Equal(t, ShareStatePrivate, ShareStateOf(contribution, expired, now), "expiry returns the version to awaiting-consent, never to authority")

	revoked := &RecordedShareDecision{Decision: approved.Decision, DecidedAt: now.Add(-time.Hour), RevokedAt: now.Add(-time.Minute)}
	require.NoError(t, revoked.Validate())
	require.False(t, GrantsShareAuthority(contribution, revoked, now), "a revoked consent is dead even inside its TTL")

	stale := &RecordedShareDecision{Decision: ShareDecision{VersionID: "v-1", EvidenceDigest: strings.Repeat("0", 64), OwnerID: "u-owner", Decision: DecisionApproved}, DecidedAt: now.Add(-time.Hour)}
	require.False(t, GrantsShareAuthority(contribution, stale, now), "replaying a consent recorded for other evidence creates no authority")

	// An unrestricted version needs no owner consent: authority exists by
	// default and no decision is projected.
	unrestricted := RestrictedContribution{VersionID: "v-9", EvidenceDigest: strings.Repeat("1", 64), Restricted: false}
	require.NoError(t, unrestricted.Validate())
	require.Equal(t, ShareStateConsented, ShareStateOf(unrestricted, nil, now))

	// The persisted fact is shape-checked: a revoked record must carry a
	// sane lifecycle.
	require.ErrorIs(t, (&RecordedShareDecision{Decision: approved.Decision, DecidedAt: now.Add(time.Hour), RevokedAt: now}).Validate(), ErrInvalidInput)
	require.ErrorIs(t, (&RecordedShareDecision{Decision: approved.Decision}).Validate(), ErrInvalidInput)
}
