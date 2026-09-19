// Package identityprobe defines an executable encoding specification for the
// native P1 identity and replay-cursor formats. It does not authorize access.
package identityprobe

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

var errInvalidIdentity = errors.New("identity value must be nonblank valid UTF-8")

// Identity contains server-resolved scope values for encoding only.
// It is not a trusted authorization or revocation result.
type Identity struct {
	TenantID                      uint64
	OwnerID, SubjectID, SessionID string
}

// SessionKey returns the tenant and owner-scoped SDK session key.
func SessionKey(in Identity) (session.Key, error) {
	if err := validateSessionIdentity(in); err != nil {
		return session.Key{}, err
	}
	return session.Key{
		AppName:   appName(in.TenantID),
		UserID:    "owner/" + encodeOpaque(in.OwnerID),
		SessionID: "session/" + encodeOpaque(in.SessionID),
	}, nil
}

// MemoryKey returns the tenant and subject-scoped SDK memory key.
func MemoryKey(in Identity) (memory.UserKey, error) {
	if err := validateMemoryIdentity(in); err != nil {
		return memory.UserKey{}, err
	}
	return memory.UserKey{
		AppName: appName(in.TenantID),
		UserID:  "subject/" + encodeOpaque(in.SubjectID),
	}, nil
}

// CheckpointNamespace returns the versioned checkpoint namespace for a run.
func CheckpointNamespace(tenant uint64, runID, graphVersion string) (string, error) {
	if tenant == 0 {
		return "", errors.New("tenant ID must be nonzero")
	}
	if err := validateOpaque(runID); err != nil {
		return "", fmt.Errorf("run ID: %w", err)
	}
	if err := validateOpaque(graphVersion); err != nil {
		return "", fmt.Errorf("graph version: %w", err)
	}
	return "native-v1/tenant/" + strconv.FormatUint(tenant, 10) +
		"/run/" + encodeOpaque(runID) + "/graph/" + encodeOpaque(graphVersion), nil
}

// EncodeCursor returns the versioned replay cursor. It carries no permission.
func EncodeCursor(runID string, sequence int64) (string, error) {
	if err := validateOpaque(runID); err != nil {
		return "", fmt.Errorf("run ID: %w", err)
	}
	if sequence < 0 {
		return "", errors.New("cursor sequence must be nonnegative")
	}
	return "v1:" + encodeOpaque(runID) + ":" + strconv.FormatInt(sequence, 10), nil
}

// DecodeCursor validates and decodes a cursor for exactly expectedRunID.
func DecodeCursor(cursor, expectedRunID string) (int64, error) {
	if err := validateOpaque(expectedRunID); err != nil {
		return 0, fmt.Errorf("expected run ID: %w", err)
	}
	parts := strings.Split(cursor, ":")
	if len(parts) != 3 || parts[0] != "v1" {
		return 0, errors.New("invalid cursor format")
	}
	runID, err := decodeCanonicalOpaque(parts[1])
	if err != nil {
		return 0, fmt.Errorf("cursor run ID: %w", err)
	}
	if runID != expectedRunID {
		return 0, errors.New("cursor run ID does not match expected run")
	}
	if !isCanonicalDecimal(parts[2]) {
		return 0, errors.New("invalid cursor sequence")
	}
	sequence, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || sequence < 0 {
		return 0, errors.New("invalid cursor sequence")
	}
	return sequence, nil
}

func validateSessionIdentity(in Identity) error {
	if in.TenantID == 0 {
		return errors.New("tenant ID must be nonzero")
	}
	for _, field := range []struct {
		name, value string
	}{
		{name: "owner ID", value: in.OwnerID},
		{name: "session ID", value: in.SessionID},
	} {
		if err := validateOpaque(field.value); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	return nil
}

func validateMemoryIdentity(in Identity) error {
	if in.TenantID == 0 {
		return errors.New("tenant ID must be nonzero")
	}
	if err := validateOpaque(in.SubjectID); err != nil {
		return fmt.Errorf("subject ID: %w", err)
	}
	return nil
}

func validateOpaque(value string) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return errInvalidIdentity
	}
	return nil
}

func appName(tenant uint64) string {
	return "weknora/native-v1/tenant/" + strconv.FormatUint(tenant, 10)
}

func encodeOpaque(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCanonicalOpaque(encoded string) (string, error) {
	if encoded == "" {
		return "", errInvalidIdentity
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return "", errors.New("must be canonical raw base64url")
	}
	value := string(decoded)
	if err := validateOpaque(value); err != nil {
		return "", err
	}
	return value, nil
}

func isCanonicalDecimal(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
