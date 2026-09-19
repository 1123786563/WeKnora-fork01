package identityprobe

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestIdentityTenantSeparation(t *testing.T) {
	a := Identity{TenantID: 1, OwnerID: "u:/你好", SubjectID: "visitor:x", SessionID: "s/a"}
	b := a
	b.TenantID = 2

	ka, err := SessionKey(a)
	if err != nil {
		t.Fatalf("SessionKey(a) error = %v", err)
	}
	kb, err := SessionKey(b)
	if err != nil {
		t.Fatalf("SessionKey(b) error = %v", err)
	}

	if ka.AppName == kb.AppName {
		t.Fatalf("tenant-specific AppName collision: %q", ka.AppName)
	}
	if ka.UserID != kb.UserID {
		t.Fatalf("tenant change altered encoded owner: %q != %q", ka.UserID, kb.UserID)
	}
	wantOwner := "owner/" + base64.RawURLEncoding.EncodeToString([]byte(a.OwnerID))
	if ka.UserID != wantOwner {
		t.Fatalf("owner encoding = %q, want %q", ka.UserID, wantOwner)
	}
}

func TestIdentityUsesIndependentOwnerAndMemorySubject(t *testing.T) {
	in := Identity{TenantID: 7, OwnerID: "owner/a:b", SubjectID: "subject/a:b", SessionID: "  session  "}

	sessionKey, err := SessionKey(in)
	if err != nil {
		t.Fatalf("SessionKey() error = %v", err)
	}
	memoryKey, err := MemoryKey(in)
	if err != nil {
		t.Fatalf("MemoryKey() error = %v", err)
	}

	if sessionKey.AppName != "weknora/native-v1/tenant/7" {
		t.Fatalf("SessionKey AppName = %q", sessionKey.AppName)
	}
	if memoryKey.AppName != sessionKey.AppName {
		t.Fatalf("MemoryKey AppName = %q, want %q", memoryKey.AppName, sessionKey.AppName)
	}
	wantOwner := "owner/" + base64.RawURLEncoding.EncodeToString([]byte("owner/a:b"))
	wantSubject := "subject/" + base64.RawURLEncoding.EncodeToString([]byte("subject/a:b"))
	wantSession := "session/" + base64.RawURLEncoding.EncodeToString([]byte("  session  "))
	if sessionKey.UserID != wantOwner || memoryKey.UserID != wantSubject || sessionKey.SessionID != wantSession {
		t.Fatalf("unexpected keys: session=%+v memory=%+v", sessionKey, memoryKey)
	}
	if sessionKey.UserID == memoryKey.UserID {
		t.Fatalf("owner and subject unexpectedly share a key: %q", sessionKey.UserID)
	}
}

func TestKeysValidateOnlyTheirOwnOpaqueScopeFields(t *testing.T) {
	sessionOnly := Identity{TenantID: 1, OwnerID: "owner", SubjectID: " \t", SessionID: "session"}
	if _, err := SessionKey(sessionOnly); err != nil {
		t.Fatalf("SessionKey() error = %v; it must not require memory subject", err)
	}

	memoryOnly := Identity{TenantID: 1, OwnerID: " \t", SubjectID: "subject", SessionID: "\r\n"}
	if _, err := MemoryKey(memoryOnly); err != nil {
		t.Fatalf("MemoryKey() error = %v; it must not require session owner or session ID", err)
	}
}

func TestIdentityDelimiterStringsDoNotCollide(t *testing.T) {
	base := Identity{TenantID: 1, OwnerID: "owner/a:b", SubjectID: "subject/a:b", SessionID: "session/a:b"}
	other := base
	other.OwnerID = "owner:a/b"
	other.SubjectID = "subject:a/b"
	other.SessionID = "session:a/b"

	baseSession, err := SessionKey(base)
	if err != nil {
		t.Fatalf("SessionKey(base) error = %v", err)
	}
	otherSession, err := SessionKey(other)
	if err != nil {
		t.Fatalf("SessionKey(other) error = %v", err)
	}
	baseMemory, err := MemoryKey(base)
	if err != nil {
		t.Fatalf("MemoryKey(base) error = %v", err)
	}
	otherMemory, err := MemoryKey(other)
	if err != nil {
		t.Fatalf("MemoryKey(other) error = %v", err)
	}
	if baseSession.UserID == otherSession.UserID || baseSession.SessionID == otherSession.SessionID || baseMemory.UserID == otherMemory.UserID {
		t.Fatalf("delimiter-containing opaque IDs collided: base=%+v/%+v other=%+v/%+v", baseSession, baseMemory, otherSession, otherMemory)
	}
}

func TestIdentityRejectsMissingBlankAndInvalidUTF8Fields(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	valid := Identity{TenantID: 1, OwnerID: "owner", SubjectID: "subject", SessionID: "session"}
	cases := []struct {
		name                          string
		in                            Identity
		wantSessionErr, wantMemoryErr bool
	}{
		{name: "zero tenant", in: Identity{OwnerID: "owner", SubjectID: "subject", SessionID: "session"}, wantSessionErr: true, wantMemoryErr: true},
		{name: "empty owner", in: Identity{TenantID: 1, SubjectID: "subject", SessionID: "session"}, wantSessionErr: true},
		{name: "blank owner", in: Identity{TenantID: 1, OwnerID: " \t\n", SubjectID: "subject", SessionID: "session"}, wantSessionErr: true},
		{name: "invalid owner", in: Identity{TenantID: 1, OwnerID: invalidUTF8, SubjectID: "subject", SessionID: "session"}, wantSessionErr: true},
		{name: "empty subject", in: Identity{TenantID: 1, OwnerID: "owner", SessionID: "session"}, wantMemoryErr: true},
		{name: "blank subject", in: Identity{TenantID: 1, OwnerID: "owner", SubjectID: "\u3000", SessionID: "session"}, wantMemoryErr: true},
		{name: "invalid subject", in: Identity{TenantID: 1, OwnerID: "owner", SubjectID: invalidUTF8, SessionID: "session"}, wantMemoryErr: true},
		{name: "empty session", in: Identity{TenantID: 1, OwnerID: "owner", SubjectID: "subject"}, wantSessionErr: true},
		{name: "blank session", in: Identity{TenantID: 1, OwnerID: "owner", SubjectID: "subject", SessionID: "\r\n"}, wantSessionErr: true},
		{name: "invalid session", in: Identity{TenantID: 1, OwnerID: "owner", SubjectID: "subject", SessionID: invalidUTF8}, wantSessionErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, sessionErr := SessionKey(tc.in)
			if (sessionErr != nil) != tc.wantSessionErr {
				t.Fatalf("SessionKey() error = %v, want error = %v", sessionErr, tc.wantSessionErr)
			}
			_, memoryErr := MemoryKey(tc.in)
			if (memoryErr != nil) != tc.wantMemoryErr {
				t.Fatalf("MemoryKey() error = %v, want error = %v", memoryErr, tc.wantMemoryErr)
			}
		})
	}

	max := valid
	max.TenantID = math.MaxUint64
	key, err := SessionKey(max)
	if err != nil {
		t.Fatalf("SessionKey(max tenant) error = %v", err)
	}
	if key.AppName != "weknora/native-v1/tenant/18446744073709551615" {
		t.Fatalf("max tenant AppName = %q", key.AppName)
	}
}

func TestCheckpointNamespaceUsesCanonicalOpaqueSegments(t *testing.T) {
	namespace, err := CheckpointNamespace(math.MaxUint64, "run/:你好", "graph / v1")
	if err != nil {
		t.Fatalf("CheckpointNamespace() error = %v", err)
	}
	want := "native-v1/tenant/18446744073709551615/run/" +
		base64.RawURLEncoding.EncodeToString([]byte("run/:你好")) + "/graph/" +
		base64.RawURLEncoding.EncodeToString([]byte("graph / v1"))
	if namespace != want {
		t.Fatalf("CheckpointNamespace() = %q, want %q", namespace, want)
	}
}

func TestCheckpointNamespaceRejectsInvalidInputs(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	cases := []struct {
		name                string
		tenant              uint64
		runID, graphVersion string
	}{
		{name: "zero tenant", runID: "run", graphVersion: "graph"},
		{name: "empty run", tenant: 1, graphVersion: "graph"},
		{name: "blank run", tenant: 1, runID: " \t", graphVersion: "graph"},
		{name: "invalid run", tenant: 1, runID: invalidUTF8, graphVersion: "graph"},
		{name: "empty graph", tenant: 1, runID: "run"},
		{name: "blank graph", tenant: 1, runID: "run", graphVersion: "\u3000"},
		{name: "invalid graph", tenant: 1, runID: "run", graphVersion: invalidUTF8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CheckpointNamespace(tc.tenant, tc.runID, tc.graphVersion); err == nil {
				t.Fatal("CheckpointNamespace() error = nil")
			}
		})
	}
}

func TestCursorRoundTripUsesCanonicalBase64URLAndDecimal(t *testing.T) {
	cursor, err := EncodeCursor("run/:你好", math.MaxInt64)
	if err != nil {
		t.Fatalf("EncodeCursor() error = %v", err)
	}
	want := "v1:" + base64.RawURLEncoding.EncodeToString([]byte("run/:你好")) + ":9223372036854775807"
	if cursor != want {
		t.Fatalf("EncodeCursor() = %q, want %q", cursor, want)
	}
	sequence, err := DecodeCursor(cursor, "run/:你好")
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if sequence != math.MaxInt64 {
		t.Fatalf("DecodeCursor() = %d, want %d", sequence, int64(math.MaxInt64))
	}
}

func TestCursorRejectsMalformedForeignAndNonCanonicalValues(t *testing.T) {
	valid, err := EncodeCursor("run", 0)
	if err != nil {
		t.Fatalf("EncodeCursor() setup error = %v", err)
	}
	cases := []struct {
		name, cursor, expectedRun string
	}{
		{name: "wrong version", cursor: "v2:cnVu:0", expectedRun: "run"},
		{name: "wrong part count", cursor: "v1:cnVu:0:extra", expectedRun: "run"},
		{name: "malformed base64", cursor: "v1:***:0", expectedRun: "run"},
		{name: "padded base64", cursor: "v1:cnVu=:0", expectedRun: "run"},
		{name: "noncanonical base64", cursor: "v1:cnV:0", expectedRun: "run"},
		{name: "empty encoded run", cursor: "v1::0", expectedRun: "run"},
		{name: "canonical base64 decoded blank run", cursor: "v1:IA:0", expectedRun: "run"},
		{name: "canonical base64 decoded invalid UTF-8 run", cursor: "v1:_w:0", expectedRun: "run"},
		{name: "foreign run", cursor: valid, expectedRun: "other"},
		{name: "negative sequence", cursor: "v1:cnVu:-1", expectedRun: "run"},
		{name: "overflow sequence", cursor: "v1:cnVu:9223372036854775808", expectedRun: "run"},
		{name: "signed sequence", cursor: "v1:cnVu:+1", expectedRun: "run"},
		{name: "leading zero sequence", cursor: "v1:cnVu:01", expectedRun: "run"},
		{name: "empty expected run", cursor: valid, expectedRun: ""},
		{name: "blank expected run", cursor: valid, expectedRun: " \t"},
		{name: "invalid UTF-8 expected run", cursor: valid, expectedRun: string([]byte{0xff})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeCursor(tc.cursor, tc.expectedRun); err == nil {
				t.Fatal("DecodeCursor() error = nil")
			}
		})
	}

	if _, err := EncodeCursor("", 0); err == nil {
		t.Fatal("EncodeCursor(empty run) error = nil")
	}
	if _, err := EncodeCursor(" \t", 0); err == nil {
		t.Fatal("EncodeCursor(blank run) error = nil")
	}
	if _, err := EncodeCursor(string([]byte{0xff}), 0); err == nil {
		t.Fatal("EncodeCursor(invalid UTF-8 run) error = nil")
	}
	if _, err := EncodeCursor("run", -1); err == nil {
		t.Fatal("EncodeCursor(negative sequence) error = nil")
	}
}
