package nativecontract

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestSessionKeySeparatesTenantsWithSameOwnerAndSession(t *testing.T) {
	a := Scope{TenantID: 1, SessionOwnerID: "same-owner"}
	b := Scope{TenantID: 2, SessionOwnerID: "same-owner"}

	first, err := SessionKey(a, "会话/a:b")
	if err != nil {
		t.Fatalf("SessionKey(first) error = %v", err)
	}
	second, err := SessionKey(b, "会话/a:b")
	if err != nil {
		t.Fatalf("SessionKey(second) error = %v", err)
	}

	if first == second {
		t.Fatalf("SessionKey() produced equal keys across tenants: %#v", first)
	}
	if first.AppName != "weknora/native-v1/tenant/1" {
		t.Fatalf("first AppName = %q", first.AppName)
	}
	if second.AppName != "weknora/native-v1/tenant/2" {
		t.Fatalf("second AppName = %q", second.AppName)
	}
}

func TestScopeKeysEncodeUnicodeAndDelimitersWithoutCollisions(t *testing.T) {
	firstScope := Scope{TenantID: 7, SessionOwnerID: "owner/a:b你好", MemorySubjectID: "subject/a:b你好"}
	secondScope := Scope{TenantID: 7, SessionOwnerID: "owner:a/b你好", MemorySubjectID: "subject:a/b你好"}

	firstSession, err := SessionKey(firstScope, "session/a:b你好")
	if err != nil {
		t.Fatalf("SessionKey(first) error = %v", err)
	}
	secondSession, err := SessionKey(secondScope, "session:a/b你好")
	if err != nil {
		t.Fatalf("SessionKey(second) error = %v", err)
	}
	firstMemory, err := MemoryKey(firstScope)
	if err != nil {
		t.Fatalf("MemoryKey(first) error = %v", err)
	}
	secondMemory, err := MemoryKey(secondScope)
	if err != nil {
		t.Fatalf("MemoryKey(second) error = %v", err)
	}

	if firstSession.UserID == secondSession.UserID || firstSession.SessionID == secondSession.SessionID || firstMemory.UserID == secondMemory.UserID {
		t.Fatalf("opaque identifiers collided: first=%#v/%#v second=%#v/%#v", firstSession, firstMemory, secondSession, secondMemory)
	}
	wantOwner := "owner/" + base64.RawURLEncoding.EncodeToString([]byte("owner/a:b你好"))
	wantSubject := "subject/" + base64.RawURLEncoding.EncodeToString([]byte("subject/a:b你好"))
	wantSession := "session/" + base64.RawURLEncoding.EncodeToString([]byte("session/a:b你好"))
	if firstSession.UserID != wantOwner || firstMemory.UserID != wantSubject || firstSession.SessionID != wantSession {
		t.Fatalf("encoded keys = %#v/%#v", firstSession, firstMemory)
	}
}

func TestScopeKeysKeepSessionOwnerAndMemorySubjectSeparate(t *testing.T) {
	scope := Scope{TenantID: 4, SessionOwnerID: "api_tenant_key:4:99", MemorySubjectID: "api_external_user:4:99"}

	sessionKey, err := SessionKey(scope, "s")
	if err != nil {
		t.Fatalf("SessionKey() error = %v", err)
	}
	memoryKey, err := MemoryKey(scope)
	if err != nil {
		t.Fatalf("MemoryKey() error = %v", err)
	}

	if sessionKey.UserID == memoryKey.UserID {
		t.Fatalf("owner and subject used the same key: %q", sessionKey.UserID)
	}
	if got := sessionKey.UserID; got != "owner/"+base64.RawURLEncoding.EncodeToString([]byte(scope.SessionOwnerID)) {
		t.Fatalf("session owner key = %q", got)
	}
	if got := memoryKey.UserID; got != "subject/"+base64.RawURLEncoding.EncodeToString([]byte(scope.MemorySubjectID)) {
		t.Fatalf("memory subject key = %q", got)
	}
}

func TestScopeKeysAreRepeatable(t *testing.T) {
	scope := Scope{TenantID: 9, SessionOwnerID: "owner", MemorySubjectID: "subject"}
	firstSession, err := SessionKey(scope, "session")
	if err != nil {
		t.Fatalf("first SessionKey() error = %v", err)
	}
	secondSession, err := SessionKey(scope, "session")
	if err != nil {
		t.Fatalf("second SessionKey() error = %v", err)
	}
	firstMemory, err := MemoryKey(scope)
	if err != nil {
		t.Fatalf("first MemoryKey() error = %v", err)
	}
	secondMemory, err := MemoryKey(scope)
	if err != nil {
		t.Fatalf("second MemoryKey() error = %v", err)
	}
	if firstSession != secondSession || firstMemory != secondMemory {
		t.Fatalf("same inputs did not rebuild keys: %#v/%#v != %#v/%#v", firstSession, firstMemory, secondSession, secondMemory)
	}
}

func TestScopeKeysRejectEmptyIdentityAsInvalidRequest(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	cases := []struct {
		name        string
		scope       Scope
		sessionID   string
		sessionTest bool
	}{
		{name: "zero tenant session", scope: Scope{SessionOwnerID: "owner"}, sessionID: "session", sessionTest: true},
		{name: "blank owner", scope: Scope{TenantID: 1, SessionOwnerID: " \t"}, sessionID: "session", sessionTest: true},
		{name: "invalid owner", scope: Scope{TenantID: 1, SessionOwnerID: invalidUTF8}, sessionID: "session", sessionTest: true},
		{name: "blank session", scope: Scope{TenantID: 1, SessionOwnerID: "owner"}, sessionID: "\n", sessionTest: true},
		{name: "invalid session", scope: Scope{TenantID: 1, SessionOwnerID: "owner"}, sessionID: invalidUTF8, sessionTest: true},
		{name: "zero tenant memory", scope: Scope{MemorySubjectID: "subject"}},
		{name: "blank subject", scope: Scope{TenantID: 1, MemorySubjectID: "\u3000"}},
		{name: "invalid subject", scope: Scope{TenantID: 1, MemorySubjectID: invalidUTF8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.sessionTest {
				_, err = SessionKey(tc.scope, tc.sessionID)
			} else {
				_, err = MemoryKey(tc.scope)
			}
			if err == nil {
				t.Fatal("expected invalid identity error")
			}
			var failure *Failure
			if !errors.As(err, &failure) || failure.Code != ErrInvalid {
				t.Fatalf("error = %v, want Failure{%q}", err, ErrInvalid)
			}
		})
	}
}
