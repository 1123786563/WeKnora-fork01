package workbench

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testSigningKey(t *testing.T) []byte {
	t.Helper()
	// 32 bytes of hex entropy, generated per test run; never a real secret.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

func validGrant() ArtifactGrant {
	return ArtifactGrant{
		TenantID:  7,
		SessionID: "sess-1",
		MessageID: "msg-1",
		Index:     2,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
}

func TestArtifactGrantSignVerifyRoundTrip(t *testing.T) {
	key := testSigningKey(t)
	grant := validGrant()
	sig, err := SignArtifactGrant(key, grant)
	require.NoError(t, err)
	require.Len(t, sig, 64) // hex sha256
	require.NoError(t, VerifyArtifactGrantAt(key, grant, sig, time.Now()))
	// Signature comparison is case-insensitive on hex.
	require.NoError(t, VerifyArtifactGrantAt(key, grant, strings.ToUpper(sig), time.Now()))
}

func TestArtifactGrantTamperRejected(t *testing.T) {
	key := testSigningKey(t)
	grant := validGrant()
	sig, err := SignArtifactGrant(key, grant)
	require.NoError(t, err)

	// Every field swap must invalidate the signature: a leaked link cannot be
	// replayed against a different artifact, tenant, or message.
	mutations := []ArtifactGrant{
		{TenantID: 8, SessionID: grant.SessionID, MessageID: grant.MessageID, Index: grant.Index, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, SessionID: "sess-2", MessageID: grant.MessageID, Index: grant.Index, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, SessionID: grant.SessionID, MessageID: "msg-2", Index: grant.Index, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, SessionID: grant.SessionID, MessageID: grant.MessageID, Index: grant.Index + 1, ExpiresAt: grant.ExpiresAt},
		{TenantID: grant.TenantID, SessionID: grant.SessionID, MessageID: grant.MessageID, Index: grant.Index, ExpiresAt: grant.ExpiresAt + 3600},
	}
	for _, mutated := range mutations {
		err := VerifyArtifactGrantAt(key, mutated, sig, time.Now())
		require.Error(t, err, "mutated grant must not verify: %+v", mutated)
	}

	// A different key must not verify either.
	otherKey := make([]byte, 32)
	for i := range otherKey {
		otherKey[i] = byte(i + 100)
	}
	require.Error(t, VerifyArtifactGrantAt(otherKey, grant, sig, time.Now()))
}

func TestArtifactGrantExpiryEnforced(t *testing.T) {
	key := testSigningKey(t)
	now := time.Unix(1_000_000, 0)
	grant := validGrant()
	grant.ExpiresAt = now.Add(-time.Second).Unix() // expired one second ago
	sig, err := SignArtifactGrant(key, grant)
	require.NoError(t, err)

	err = VerifyArtifactGrantAt(key, grant, sig, now)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired")

	// Valid until exactly the boundary second…
	boundary := validGrant()
	boundary.ExpiresAt = now.Add(time.Minute).Unix()
	boundarySig, err := SignArtifactGrant(key, boundary)
	require.NoError(t, err)
	require.NoError(t, VerifyArtifactGrantAt(key, boundary, boundarySig, now))
	// …and rejected once that second passes.
	require.Error(t, VerifyArtifactGrantAt(key, boundary, boundarySig, now.Add(61*time.Second)))
}

func TestArtifactGrantCanonicalRejectsAmbiguity(t *testing.T) {
	// '|' in ids could splice the canonical string into a different grant.
	bad := validGrant()
	bad.SessionID = "a|b"
	_, err := bad.Canonical()
	require.Error(t, err)

	// Zero tenant / missing ids / negative index are all invalid grants.
	invalid := []ArtifactGrant{
		{TenantID: 0, SessionID: "s", MessageID: "m", Index: 0, ExpiresAt: 1},
		{TenantID: 1, SessionID: "", MessageID: "m", Index: 0, ExpiresAt: 1},
		{TenantID: 1, SessionID: "s", MessageID: "", Index: 0, ExpiresAt: 1},
		{TenantID: 1, SessionID: "s", MessageID: "m", Index: -1, ExpiresAt: 1},
		{TenantID: 1, SessionID: "s", MessageID: "m", Index: 0, ExpiresAt: 0},
	}
	for _, g := range invalid {
		_, err := g.Canonical()
		require.Error(t, err, "grant %+v must be rejected", g)
	}

	// Distinct grants must canonicalize distinctly.
	a := validGrant()
	b := validGrant()
	b.Index++
	ca, _ := a.Canonical()
	cb, _ := b.Canonical()
	require.NotEqual(t, ca, cb)
}

func TestArtifactSigningKeyFromEnv(t *testing.T) {
	t.Setenv(ArtifactSigningKeyEnvVar, "")
	_, err := ArtifactSigningKeyFromEnv()
	require.ErrorIs(t, err, ErrArtifactSigningKeyMissing)

	// Short or non-hex values are rejected: the HMAC secret needs real entropy.
	t.Setenv(ArtifactSigningKeyEnvVar, "abcd")
	_, err = ArtifactSigningKeyFromEnv()
	require.Error(t, err)
	t.Setenv(ArtifactSigningKeyEnvVar, strings.Repeat("zz", 40))
	_, err = ArtifactSigningKeyFromEnv()
	require.Error(t, err)

	// 32 bytes of hex is the minimum accepted secret length.
	t.Setenv(ArtifactSigningKeyEnvVar, strings.Repeat("ab", 32))
	key, err := ArtifactSigningKeyFromEnv()
	require.NoError(t, err)
	require.Len(t, key, 32)
}
