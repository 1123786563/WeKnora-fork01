package execution

// Personal node registration is deliberately separate from execution target
// admission.  This package owns the cryptographic proof and the durable
// one-time/idempotency rules; the handler supplies the authenticated tenant
// and owner and never accepts a node secret or root path.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrRegistrationInvalid      = errors.New("registration_invalid")
	ErrRegistrationUnauthorized = errors.New("registration_unauthorized")
	ErrRegistrationReplay       = errors.New("registration_replay_conflict")
	ErrRegistrationNotFound     = errors.New("registration_not_found")
	ErrRegistrationRevoked      = errors.New("registration_revoked")
	ErrRegistrationChallenge    = errors.New("registration_challenge_invalid")
)

const registrationChallengeTTL = 10 * time.Minute

type NodeGrant struct {
	TargetID   string   `json:"targetID"`
	Epoch      int      `json:"epoch"`
	ExpiresAt  int64    `json:"expiresAt"`
	Operations []string `json:"operations"`
}

func validateNodeGrant(g NodeGrant, expected struct {
	TargetID string
	Epoch    int
}, operation string, now int64) error {
	if strings.TrimSpace(g.TargetID) == "" || g.TargetID != expected.TargetID || g.Epoch != expected.Epoch || g.ExpiresAt <= now || !contains(g.Operations, operation) {
		return ErrRegistrationUnauthorized
	}
	return nil
}

// ValidateNodeGrant is the final local capability check. Signature/TLS
// verification happens before a grant reaches this function.
func ValidateNodeGrant(g NodeGrant, expected struct {
	TargetID string
	Epoch    int
}, operation string, now int64) error {
	return validateNodeGrant(g, expected, operation, now)
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

type NodeChallengeRequest struct {
	RuntimeID        string `json:"runtime_id"`
	ExternalTargetID string `json:"external_target_id"`
	PublicKey        string `json:"public_key"`
}

type NodeChallenge struct {
	ID               string    `json:"challenge_id"`
	RuntimeID        string    `json:"runtime_id"`
	ExternalTargetID string    `json:"external_target_id"`
	PublicKey        string    `json:"public_key"`
	Nonce            string    `json:"nonce"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type NodeRegistrationRequest struct {
	ChallengeID      string `json:"challenge_id"`
	Nonce            string `json:"nonce"`
	RuntimeID        string `json:"runtime_id"`
	ExternalTargetID string `json:"external_target_id"`
	PublicKey        string `json:"public_key"`
	Signature        string `json:"signature"`
	IdempotencyKey   string `json:"idempotency_key"`
	Confirmed        bool   `json:"confirmed"`
}

type NodeRegistration struct {
	ID                string     `json:"id"`
	RuntimeID         string     `json:"runtime_id"`
	ExternalTargetID  string     `json:"external_target_id"`
	PublicKey         string     `json:"public_key"`
	CredentialVersion int64      `json:"credential_version"`
	State             string     `json:"state"`
	CreatedAt         time.Time  `json:"created_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}

type registrationChallengeRow struct {
	TenantID             uint64 `gorm:"primaryKey"`
	OwnerID              string `gorm:"primaryKey"`
	ID                   string `gorm:"primaryKey;column:challenge_id"`
	RuntimeID            string
	ExternalTargetID     string
	PublicKeyFingerprint string
	NonceHash            string
	Nonce                string
	ExpiresAt            time.Time
	ConsumedAt           *time.Time
}

func (registrationChallengeRow) TableName() string { return "execution_registration_challenges" }

type registrationRow struct {
	TenantID             uint64 `gorm:"primaryKey"`
	OwnerID              string `gorm:"primaryKey"`
	ID                   string `gorm:"primaryKey;column:registration_id"`
	RuntimeID            string
	ExternalTargetID     string
	PublicKeyFingerprint string
	CredentialVersion    int64
	State                string
	IdempotencyKey       string
	RequestHash          string
	CreatedAt            time.Time
	RevokedAt            *time.Time
}

func (registrationRow) TableName() string { return "execution_registrations" }

type RegistrationService struct{ db *gorm.DB }

func NewRegistrationService(db *gorm.DB) *RegistrationService { return &RegistrationService{db: db} }

func fingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}
func digestRequest(r NodeRegistrationRequest) string {
	h := sha256.New()
	for _, value := range []string{r.ChallengeID, r.Nonce, r.RuntimeID, r.ExternalTargetID, r.PublicKey, r.Signature, r.IdempotencyKey, fmt.Sprint(r.Confirmed)} {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func randomID(prefix string) (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}

func (s *RegistrationService) CreateChallenge(ctx context.Context, tenant uint64, owner string, req NodeChallengeRequest, now time.Time) (NodeChallenge, error) {
	if s == nil || s.db == nil || tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(req.RuntimeID) == "" || strings.TrimSpace(req.ExternalTargetID) == "" {
		return NodeChallenge{}, ErrRegistrationInvalid
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.PublicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return NodeChallenge{}, ErrRegistrationInvalid
	}
	id, err := randomID("ch")
	if err != nil {
		return NodeChallenge{}, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return NodeChallenge{}, err
	}
	nonceText := base64.RawURLEncoding.EncodeToString(nonce)
	nonceHash := sha256.Sum256(nonce)
	expires := now.UTC().Add(registrationChallengeTTL)
	row := registrationChallengeRow{TenantID: tenant, OwnerID: owner, ID: id, RuntimeID: req.RuntimeID, ExternalTargetID: req.ExternalTargetID, PublicKeyFingerprint: fingerprint(key), NonceHash: hex.EncodeToString(nonceHash[:]), Nonce: nonceText, ExpiresAt: expires}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return NodeChallenge{}, err
	}
	return NodeChallenge{ID: id, RuntimeID: req.RuntimeID, ExternalTargetID: req.ExternalTargetID, PublicKey: req.PublicKey, Nonce: nonceText, ExpiresAt: expires}, nil
}

func (s *RegistrationService) Complete(ctx context.Context, tenant uint64, owner string, req NodeRegistrationRequest, now time.Time) (NodeRegistration, NodeGrant, error) {
	if s == nil || s.db == nil || tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(req.IdempotencyKey) == "" || !req.Confirmed {
		return NodeRegistration{}, NodeGrant{}, ErrRegistrationInvalid
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.PublicKey))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return NodeRegistration{}, NodeGrant{}, ErrRegistrationInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.Signature))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return NodeRegistration{}, NodeGrant{}, ErrRegistrationChallenge
	}
	hash := digestRequest(req)
	var result NodeRegistration
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing registrationRow
		if err := tx.Where("tenant_id = ? AND owner_id = ? AND idempotency_key = ?", tenant, owner, req.IdempotencyKey).First(&existing).Error; err == nil {
			if existing.RequestHash != hash {
				return ErrRegistrationReplay
			}
			result = toRegistration(existing)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var challenge registrationChallengeRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_id = ? AND challenge_id = ?", tenant, owner, req.ChallengeID).First(&challenge).Error; err != nil {
			return ErrRegistrationChallenge
		}
		if challenge.ConsumedAt != nil || !challenge.ExpiresAt.After(now.UTC()) || challenge.RuntimeID != req.RuntimeID || challenge.ExternalTargetID != req.ExternalTargetID || challenge.PublicKeyFingerprint != fingerprint(key) {
			return ErrRegistrationChallenge
		}
		nonce, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.Nonce))
		if err != nil {
			return ErrRegistrationChallenge
		}
		nonceHash := sha256.Sum256(nonce)
		if hex.EncodeToString(nonceHash[:]) != challenge.NonceHash || !ed25519.Verify(ed25519.PublicKey(key), nonce, sig) {
			return ErrRegistrationChallenge
		}
		id, err := randomID("node")
		if err != nil {
			return err
		}
		row := registrationRow{TenantID: tenant, OwnerID: owner, ID: id, RuntimeID: req.RuntimeID, ExternalTargetID: req.ExternalTargetID, PublicKeyFingerprint: challenge.PublicKeyFingerprint, CredentialVersion: 1, State: "active", IdempotencyKey: req.IdempotencyKey, RequestHash: hash, CreatedAt: now.UTC()}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		consumed := now.UTC()
		if err := tx.Model(&registrationChallengeRow{}).Where("tenant_id = ? AND owner_id = ? AND challenge_id = ? AND consumed_at IS NULL", tenant, owner, req.ChallengeID).Updates(map[string]any{"consumed_at": consumed}).Error; err != nil {
			return err
		}
		result = toRegistration(row)
		return nil
	})
	if err != nil {
		return NodeRegistration{}, NodeGrant{}, err
	}
	return result, NodeGrant{TargetID: result.ID, Epoch: int(result.CredentialVersion), ExpiresAt: now.UTC().Add(5 * time.Minute).UnixMilli(), Operations: []string{"start", "observe", "stop"}}, nil
}

func (s *RegistrationService) Revoke(ctx context.Context, tenant uint64, owner, id string, now time.Time) error {
	if s == nil || s.db == nil || tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(id) == "" {
		return ErrRegistrationInvalid
	}
	result := s.db.WithContext(ctx).Model(&registrationRow{}).Where("tenant_id = ? AND owner_id = ? AND registration_id = ? AND state = ?", tenant, owner, id, "active").Updates(map[string]any{"state": "revoked", "revoked_at": now.UTC(), "credential_version": gorm.Expr("credential_version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRegistrationNotFound
	}
	return nil
}

func toRegistration(row registrationRow) NodeRegistration {
	return NodeRegistration{ID: row.ID, RuntimeID: row.RuntimeID, ExternalTargetID: row.ExternalTargetID, PublicKey: row.PublicKeyFingerprint, CredentialVersion: row.CredentialVersion, State: row.State, CreatedAt: row.CreatedAt, RevokedAt: row.RevokedAt}
}
