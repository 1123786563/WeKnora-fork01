package nativecontract

import (
	"encoding/base64"
	"strconv"
	"strings"
	"unicode/utf8"

	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const nativeAppPrefix = "weknora/native-v1/tenant/"

// SessionKey derives the SDK key from server-resolved tenant and session-owner
// scope. It encodes values only; authorization remains the facade's job.
func SessionKey(scope Scope, sessionID string) (session.Key, error) {
	if err := validateScopeID(scope.TenantID, "tenant"); err != nil {
		return session.Key{}, err
	}
	if err := validateOpaque(scope.SessionOwnerID, "session owner"); err != nil {
		return session.Key{}, err
	}
	if err := validateOpaque(sessionID, "session"); err != nil {
		return session.Key{}, err
	}
	return session.Key{AppName: appName(scope.TenantID), UserID: "owner/" + encode(scope.SessionOwnerID), SessionID: "session/" + encode(sessionID)}, nil
}

// MemoryKey derives the SDK key from the independently server-resolved memory
// subject. It intentionally never falls back to SessionOwnerID.
func MemoryKey(scope Scope) (memory.UserKey, error) {
	if err := validateScopeID(scope.TenantID, "tenant"); err != nil {
		return memory.UserKey{}, err
	}
	if err := validateOpaque(scope.MemorySubjectID, "memory subject"); err != nil {
		return memory.UserKey{}, err
	}
	return memory.UserKey{AppName: appName(scope.TenantID), UserID: "subject/" + encode(scope.MemorySubjectID)}, nil
}

func appName(tenantID uint64) string { return nativeAppPrefix + strconv.FormatUint(tenantID, 10) }
func encode(value string) string     { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
func validateScopeID(tenantID uint64, _ string) error {
	if tenantID == 0 {
		return invalidScope("tenant is required")
	}
	return nil
}

func validateOpaque(value, field string) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return invalidScope(field + " is required")
	}
	return nil
}

func invalidScope(message string) error {
	return &Failure{Code: ErrInvalid, Message: message, Effect: EffectNotDispatched}
}
