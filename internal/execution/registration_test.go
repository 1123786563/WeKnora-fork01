package execution

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openRegistrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+url.PathEscape(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&registrationChallengeRow{}, &registrationRow{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX uq_execution_registrations_idempotency ON execution_registrations (tenant_id, owner_id, idempotency_key)").Error)
	return db
}

func makeRegistrationRequest(t *testing.T, svc *RegistrationService, now time.Time, idempotency string) NodeRegistrationRequest {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicText := base64.RawURLEncoding.EncodeToString(public)
	challenge, err := svc.CreateChallenge(context.Background(), 1, "u1", NodeChallengeRequest{RuntimeID: "r1", ExternalTargetID: "x1", PublicKey: publicText}, now)
	require.NoError(t, err)
	nonce, err := base64.RawURLEncoding.DecodeString(challenge.Nonce)
	require.NoError(t, err)
	return NodeRegistrationRequest{ChallengeID: challenge.ID, Nonce: challenge.Nonce, RuntimeID: "r1", ExternalTargetID: "x1", PublicKey: publicText, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, nonce)), IdempotencyKey: idempotency, Confirmed: true}
}

func TestValidateNodeGrantRejectsOldEpochAndExpired(t *testing.T) {
	g := NodeGrant{TargetID: "n", Epoch: 1, ExpiresAt: 100, Operations: []string{"start"}}
	require.ErrorIs(t, ValidateNodeGrant(g, struct {
		TargetID string
		Epoch    int
	}{"n", 2}, "start", 50), ErrRegistrationUnauthorized)
	require.ErrorIs(t, ValidateNodeGrant(g, struct {
		TargetID string
		Epoch    int
	}{"n", 1}, "start", 101), ErrRegistrationUnauthorized)
}

func TestRegistrationProofConsumesChallengeAndIsIdempotent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	svc := NewRegistrationService(openRegistrationDB(t), nil)
	req := makeRegistrationRequest(t, svc, now, "req-1")
	registration, grant, err := svc.Complete(context.Background(), 1, "u1", req, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), registration.CredentialVersion)
	require.Equal(t, registration.ID, grant.TargetID)
	require.Equal(t, 1, grant.Epoch)
	second, secondGrant, err := svc.Complete(context.Background(), 1, "u1", req, now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, registration.ID, second.ID)
	require.Equal(t, grant.TargetID, secondGrant.TargetID)
	bad := req
	bad.ExternalTargetID = "other"
	_, _, err = svc.Complete(context.Background(), 1, "u1", bad, now.Add(time.Second))
	require.ErrorIs(t, err, ErrRegistrationReplay)
	_, _, err = svc.Complete(context.Background(), 1, "u2", req, now)
	require.Error(t, err)
}

func TestRegistrationRejectsReplayAndCrossTargetProof(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	svc := NewRegistrationService(openRegistrationDB(t), nil)
	req := makeRegistrationRequest(t, svc, now, "req-1")
	req.ExternalTargetID = "other"
	_, _, err := svc.Complete(context.Background(), 1, "u1", req, now)
	require.ErrorIs(t, err, ErrRegistrationChallenge)
	req = makeRegistrationRequest(t, svc, now, "req-2")
	req.Confirmed = false
	_, _, err = svc.Complete(context.Background(), 1, "u1", req, now)
	require.ErrorIs(t, err, ErrRegistrationInvalid)
}

func TestRegistrationRevocationBumpsCredentialVersionAndScopesOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	svc := NewRegistrationService(openRegistrationDB(t), nil)
	req := makeRegistrationRequest(t, svc, now, "req-1")
	registration, _, err := svc.Complete(context.Background(), 1, "u1", req, now)
	require.NoError(t, err)
	require.ErrorIs(t, svc.Revoke(context.Background(), 1, "u2", registration.ID, now), ErrRegistrationNotFound)
	require.NoError(t, svc.Revoke(context.Background(), 1, "u1", registration.ID, now))
	var row registrationRow
	require.NoError(t, svc.db.First(&row, "registration_id = ?", registration.ID).Error)
	require.Equal(t, int64(2), row.CredentialVersion)
	require.Equal(t, "revoked", row.State)
}

func TestRegistrationReplayAfterRevocationReturnsNoGrant(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	svc := NewRegistrationService(openRegistrationDB(t), nil)
	req := makeRegistrationRequest(t, svc, now, "revoked-replay")
	registration, grant, err := svc.Complete(context.Background(), 1, "u1", req, now)
	require.NoError(t, err)
	require.NotEmpty(t, grant.TargetID)
	require.NoError(t, svc.Revoke(context.Background(), 1, "u1", registration.ID, now.Add(time.Second)))
	_, replayGrant, err := svc.Complete(context.Background(), 1, "u1", req, now.Add(2*time.Second))
	require.ErrorIs(t, err, ErrRegistrationRevoked)
	require.Empty(t, replayGrant.TargetID)
	require.Empty(t, replayGrant.Operations)
}
