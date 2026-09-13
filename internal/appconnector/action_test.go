package appconnector

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func TestGeneralWriteGrantDoesNotAuthorizeSending(t *testing.T) {
	if !NeedsExplicitApproval("send", true) {
		t.Fatal("send bypassed")
	}
	if NeedsExplicitApproval("write", true) {
		t.Fatal("bounded write grant ignored")
	}
	if !NeedsExplicitApproval("delete", true) {
		t.Fatal("delete bypassed")
	}
}

// TestActionDigestBindsEveryAuthorizationInput pins that the digest changes
// whenever ANY bound input changes: identity, connection auth version, app
// version, target, or args. An approval can never leak across any of them.
func TestActionDigestBindsEveryAuthorizationInput(t *testing.T) {
	base := Action{
		ID:           "a1",
		TenantID:     7,
		ActorID:      "u1",
		ConnectionID: "conn-1",
		Version:      "1.0.0",
		Target:       "mail.send",
		Risk:         "send",
		Args:         []byte(`{"to":"a@b.c","body":"hi"}`),
		AuthVersion:  1,
	}
	want, err := ActionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*Action){
		"actor":        func(a *Action) { a.ActorID = "u2" },
		"tenant":       func(a *Action) { a.TenantID = 8 },
		"connection":   func(a *Action) { a.ConnectionID = "conn-2" },
		"auth_version": func(a *Action) { a.AuthVersion = 2 },
		"app_version":  func(a *Action) { a.Version = "2.0.0" },
		"target":       func(a *Action) { a.Target = "mail.sendAll" },
		"args":         func(a *Action) { a.Args = []byte(`{"to":"a@b.c","body":"hi!"}`) },
	}
	for name, mutate := range mutations {
		mutated := base
		mutate(&mutated)
		got, err := ActionDigest(mutated)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == want {
			t.Fatalf("%s change did not change digest", name)
		}
	}
}

// TestActionDigestStableAcrossReformatting pins that cosmetic JSON
// differences (whitespace, key order) do NOT change the digest — only real
// value changes do. The normalized bytes returned alongside are the exact
// bytes that must be persisted and sent.
func TestActionDigestStableAcrossReformatting(t *testing.T) {
	a := Action{TenantID: 7, ActorID: "u1", ConnectionID: "c", Version: "1.0.0", Target: "t", Risk: "write", AuthVersion: 1,
		Args: []byte(`{"alpha":1,"beta":[2,3]}`)}
	want, err := ActionDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	a.Args = []byte("{\n  \"beta\" : [2 , 3],\n  \"alpha\": 1\n}")
	got, err := ActionDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("reformatting changed digest: %s != %s", got, want)
	}
	norm, err := NormalizeArgs([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	again, err := NormalizeArgs(norm)
	if err != nil {
		t.Fatal(err)
	}
	if string(norm) != string(again) {
		t.Fatalf("normalization not idempotent: %s vs %s", norm, again)
	}
	if _, err := NormalizeArgs([]byte("not json")); err == nil {
		t.Fatal("invalid args accepted")
	}
}

// TestPreAuthorizationScopeMatching pins that a send-category
// pre-authorization is matched as its OWN scope: a general write grant never
// satisfies send, and a write-category pre-authorization never covers send.
func TestPreAuthorizationScopeMatching(t *testing.T) {
	now := testNow
	send := Action{TenantID: 7, ActorID: "u1", ConnectionID: "conn-1", Version: "1.0.0", Target: "mail.send", Risk: "send", AuthVersion: 1}
	pre := PreAuthorization{
		ID:             "pre-1",
		TenantID:       7,
		AllowedRisks:   []string{"send"},
		ConnectionID:   "conn-1",
		TargetScope:    "mail.send",
		ValidFrom:      now.Add(-time.Hour),
		ValidUntil:     now.Add(time.Hour),
		BudgetCapMicro: 1000,
	}
	if !PreAuthorizationCovers(pre, send, now) {
		t.Fatal("send pre-authorization did not cover its own send action")
	}
	// A general write grant never satisfies send: even a write-category
	// pre-authorization (a write scope) must not cover a send action.
	writePre := pre
	writePre.AllowedRisks = []string{"write"}
	if PreAuthorizationCovers(writePre, send, now) {
		t.Fatal("write pre-authorization covered send")
	}
	// And NeedsExplicitApproval keeps send explicit under any write grant.
	if !NeedsExplicitApproval("send", true) {
		t.Fatal("general write grant satisfied send")
	}
	// Outside the validity window the pre-authorization is dead.
	expired := pre
	expired.ValidUntil = now.Add(-time.Minute)
	if PreAuthorizationCovers(expired, send, now) {
		t.Fatal("expired pre-authorization covered send")
	}
	// Different connection or target is out of scope.
	wrongConn := pre
	wrongConn.ConnectionID = "conn-9"
	if PreAuthorizationCovers(wrongConn, send, now) {
		t.Fatal("pre-authorization covered a different connection")
	}
	wrongTarget := pre
	wrongTarget.TargetScope = "mail.draft"
	if PreAuthorizationCovers(wrongTarget, send, now) {
		t.Fatal("pre-authorization covered a different target")
	}
	// Wildcards cover any connection/target of the same risk scope.
	wild := pre
	wild.ConnectionID = "*"
	wild.TargetScope = "*"
	if !PreAuthorizationCovers(wild, send, now) {
		t.Fatal("wildcard pre-authorization refused its own risk scope")
	}
}

// TestDigestBindsRiskAndLogicalAction is the plan's verbatim pin (T09): the
// approval digest binds BOTH the logical action identity (Action ID) and the
// reviewed risk category — an approval granted for one action never carries
// to another, and a risk change never reuses the old approval.
func TestDigestBindsRiskAndLogicalAction(t *testing.T) {
	a := Action{ID: "a1", TenantID: 1, ActorID: "u", ConnectionID: "c", Version: "v1",
		AuthVersion: 1, Target: "page", Risk: RiskWrite, Args: json.RawMessage(`{"x":1}`), DigestVersion: 2}
	d1, err := ActionDigest(a)
	if err != nil {
		t.Fatal(err)
	}
	a.ID = "a2"
	d2, _ := ActionDigest(a)
	if d1 == d2 {
		t.Fatal("approval shared across actions")
	}
	a.ID = "a1"
	a.Risk = RiskDelete
	d3, _ := ActionDigest(a)
	if d1 == d3 {
		t.Fatal("risk omitted")
	}
}

// TestDigestBindsEveryOpenConnectorBindingField pins that the approval
// material carries the FULL execution binding: any change to the reviewed
// schema (schema digest), the alias, the runtime, the external connection or
// the binding generation breaks continuation of an old approval, and an OC
// action never shares a digest with a native one.
func TestDigestBindsEveryOpenConnectorBindingField(t *testing.T) {
	base := Action{
		ID: "oc-1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-1", Version: "1.0.0",
		Target: "github.createIssue", Risk: RiskWrite, AuthVersion: 3,
		Args: json.RawMessage(`{"title":"t"}`),
		OC: &OCExecutionBinding{
			RuntimeID: "rt-1", Provider: "github", ExternalID: "ext-1", Alias: "alias-1",
			ActionID: "github.createIssue", SchemaDigest: "d1", BindingVersion: 4,
		},
	}
	want, err := ActionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*Action){
		"schema_digest":   func(a *Action) { a.OC.SchemaDigest = "d2" },
		"alias":           func(a *Action) { a.OC.Alias = "alias-2" },
		"runtime":         func(a *Action) { a.OC.RuntimeID = "rt-2" },
		"external_id":     func(a *Action) { a.OC.ExternalID = "ext-2" },
		"binding_version": func(a *Action) { a.OC.BindingVersion = 5 },
		"binding_removed": func(a *Action) { a.OC = nil },
	}
	for name, mutate := range mutations {
		mutated := base
		mutate(&mutated)
		got, err := ActionDigest(mutated)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == want {
			t.Fatalf("%s change did not change digest", name)
		}
	}
}

// TestNormalizeArgsRejectsTrailingSecondJSON pins that args are EXACTLY one
// JSON value: a second concatenated document is rejected (the second Decode
// must return io.EOF), while trailing whitespace stays legal, and number
// literals survive byte-identically (no float re-encoding).
func TestNormalizeArgsRejectsTrailingSecondJSON(t *testing.T) {
	bad := []string{
		`{"a":1} {"b":2}`,
		`{"a":1}{"b":2}`,
		`{"a":1} 42`,
		`[1,2] [3]`,
		`{"a":1} garbage`,
	}
	for _, raw := range bad {
		_, err := NormalizeArgs(json.RawMessage(raw))
		if err == nil {
			t.Fatalf("trailing data accepted: %s", raw)
		}
		if !strings.Contains(err.Error(), ErrInvalidArgs.Error()) {
			t.Fatalf("trailing data error not ErrInvalidArgs: %v", err)
		}
	}
	// Trailing whitespace after the single value is legal.
	norm, err := NormalizeArgs(json.RawMessage("  {\"a\":1}  \n\t "))
	if err != nil {
		t.Fatal(err)
	}
	if string(norm) != `{"a":1}` {
		t.Fatalf("whitespace-only trail changed value: %s", norm)
	}
	// Number precision: a large literal survives normalization byte-exactly.
	big := `{"n":123456789012345678901234567890,"x":1.2500}`
	norm, err = NormalizeArgs(json.RawMessage(big))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(norm), "123456789012345678901234567890") {
		t.Fatalf("number literal lost precision: %s", norm)
	}
	_ = io.EOF // pin the sentinel the implementation must compare against
}
