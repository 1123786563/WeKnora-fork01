package connectorcontrol

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Static encryption-at-rest for the secret sink (T16, T05-F-03 ruling): the
// phase-one FileSecretSink wrote PLAINTEXT token material into its directory;
// this sink seals every record with AES-256-GCM under a static data key that
// is provisioned as a mounted secret and never lives in the database (the DB
// keeps only the reference — the T03 schema rule is unchanged). The
// SecretSink interface itself is untouched, so the platform secret store can
// still replace this implementation later (swappable by design).

// encryptedSecretPrefix marks a sealed record. Versioned so a future key
// rotation scheme can carry its own format; anything without the prefix is
// legacy phase-one plaintext and is REFUSED by this sink (fail closed — the
// API-side read path must never silently fall back to unencrypted material).
const encryptedSecretPrefix = "ocenc1."

// SecretKeySource loads the static data-encryption key (32 bytes). It is a
// function so the key is re-read per operation, exactly like AdminSecret:
// the mount may appear after startup and rotation is a remount away.
type SecretKeySource func(ctx context.Context) ([]byte, error)

// ErrSecretKeyUnavailable is the fail-closed sentinel for an unusable key
// mount (missing file, open permissions, malformed material). It never
// carries key material.
var ErrSecretKeyUnavailable = errors.New("connectorcontrol: secret key unavailable")

// ErrSecretNotEncrypted is returned when a record on disk lacks the sealed
// format — legacy phase-one plaintext. Reading it would silently downgrade
// the at-rest contract, so the sink refuses instead.
var ErrSecretNotEncrypted = errors.New("connectorcontrol: secret material is not encrypted (legacy phase-one record)")

// FileSecretKeySource reads the data-encryption key from a dedicated mount
// path on EVERY call. Accepted material: 64 hex chars, base64 (std or URL,
// padded or raw) decoding to 32 bytes, or exactly 32 raw bytes; surrounding
// whitespace is trimmed. Group/world-readable permissions refuse, mirroring
// FileAdminSecretSource. Error text carries the path, never the key.
func FileSecretKeySource(path string) SecretKeySource {
	return func(ctx context.Context) ([]byte, error) {
		st, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("%w: mount %s", ErrSecretKeyUnavailable, path)
		}
		if st.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("%w: mount %s permissions too open", ErrSecretKeyUnavailable, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: mount %s", ErrSecretKeyUnavailable, path)
		}
		key, err := parseSecretKey(data)
		if err != nil {
			return nil, fmt.Errorf("%w: mount %s: %v", ErrSecretKeyUnavailable, path, err)
		}
		return key, nil
	}
}

// parseSecretKey accepts the documented key formats and rejects everything
// else. A 32-byte key is the AES-256 key length the sink seals with.
func parseSecretKey(data []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("key material empty")
	}
	if len(trimmed) == 64 {
		if b, err := hex.DecodeString(trimmed); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := encoding.DecodeString(trimmed); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	if len(trimmed) == 32 {
		return []byte(trimmed), nil
	}
	return nil, errors.New("key must be 64 hex chars, base64 of 32 bytes, or 32 raw bytes")
}

// EncryptedFileSecretSink is the production SecretSink: the phase-one
// FileSecretSink layout (sha256(ref) filename inside a dedicated directory,
// atomic write-to-temp + rename) with static encryption at rest and real
// directory-permission enforcement — the two hardening items the T05-Q-01
// comment deferred to this task.
type EncryptedFileSecretSink struct {
	dir string
	key SecretKeySource
}

// NewEncryptedFileSecretSink validates the wiring and enforces the secrets
// directory permissions: the directory is created 0700, and an existing
// directory that is group/world accessible is a STARTUP ERROR (leaving it
// as-is was exactly the phase-one overpromise). The key source is lazy —
// the mount may appear after startup — but every operation re-reads it and
// fails closed while it is unusable.
func NewEncryptedFileSecretSink(dir string, key SecretKeySource) (*EncryptedFileSecretSink, error) {
	if dir == "" {
		return nil, errors.New("connectorcontrol: secret dir required")
	}
	if key == nil {
		return nil, errors.New("connectorcontrol: secret key source required (static encryption is mandatory for the file sink)")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("connectorcontrol: secrets dir %s permissions too open (%04o); require 0700", dir, st.Mode().Perm())
	}
	return &EncryptedFileSecretSink{dir: dir, key: key}, nil
}

// secretFileName mirrors the shared on-disk layout: sha256 of the reference,
// hex, .secret — byte-identical to FileSecretSink so both sinks address the
// same slot and a migration re-put re-seals in place.
func secretFileName(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return hex.EncodeToString(sum[:]) + ".secret"
}

// aeadFor seals the current key into an AES-256-GCM AEAD.
func aeadFor(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// PutSecret atomically writes the sealed record under ref: fresh 12-byte
// nonce per record, AES-256-GCM over the material, written to a uniquely
// named temp file (T05-Q-02 discipline) then renamed into place, mode 0600.
// Re-putting the same ref overwrites — the newest token for a connection
// generation wins. The plaintext never touches the disk.
func (s *EncryptedFileSecretSink) PutSecret(ctx context.Context, ref, secret string) error {
	if ref == "" || secret == "" {
		return errors.New("connectorcontrol: secret ref and material required")
	}
	key, err := s.key(ctx)
	if err != nil {
		return err
	}
	aead, err := aeadFor(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := aead.Seal(nil, nonce, []byte(secret), nil)
	record := encryptedSecretPrefix +
		base64.RawURLEncoding.EncodeToString(nonce) + "." +
		base64.RawURLEncoding.EncodeToString(sealed)
	final := filepath.Join(s.dir, secretFileName(ref))
	tmp := final + "." + uuid.NewString() + ".tmp"
	if err := os.WriteFile(tmp, []byte(record), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// GetSecret reads one reference's sealed record back and opens it. A missing
// file fails closed; a record without the sealed prefix is legacy phase-one
// plaintext and is refused with ErrSecretNotEncrypted rather than silently
// served; a tampered or wrong-key record fails the GCM authentication.
func (s *EncryptedFileSecretSink) GetSecret(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("connectorcontrol: secret ref required")
	}
	data, err := os.ReadFile(filepath.Join(s.dir, secretFileName(ref)))
	if err != nil {
		return "", err
	}
	record := strings.TrimSpace(string(data))
	if record == "" {
		return "", errors.New("connectorcontrol: secret material empty")
	}
	if !strings.HasPrefix(record, encryptedSecretPrefix) {
		return "", ErrSecretNotEncrypted
	}
	rest := record[len(encryptedSecretPrefix):]
	dot := strings.IndexByte(rest, '.')
	if dot <= 0 || dot == len(rest)-1 {
		return "", errors.New("connectorcontrol: malformed sealed record")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(rest[:dot])
	if err != nil {
		return "", fmt.Errorf("connectorcontrol: malformed sealed record nonce: %w", err)
	}
	sealed, err := base64.RawURLEncoding.DecodeString(rest[dot+1:])
	if err != nil {
		return "", fmt.Errorf("connectorcontrol: malformed sealed record body: %w", err)
	}
	key, err := s.key(ctx)
	if err != nil {
		return "", err
	}
	aead, err := aeadFor(key)
	if err != nil {
		return "", err
	}
	if len(nonce) != aead.NonceSize() {
		return "", errors.New("connectorcontrol: sealed record nonce size mismatch")
	}
	opened, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("connectorcontrol: sealed record does not open under the provisioned key: %w", err)
	}
	if len(opened) == 0 {
		return "", errors.New("connectorcontrol: secret material empty")
	}
	return string(opened), nil
}
