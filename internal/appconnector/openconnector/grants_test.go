package openconnector

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestGrantRejectsEmptyAndWildcard is the unsafe-input rejection test pinned by
// the implementation plan: upstream treats an empty allow-list as ALLOW-ALL
// (runtime-proven, fixtures/empty_grant.json; contract doc §3.2), so a locally
// empty or wildcard-carrying grant must never be constructible.
func TestGrantRejectsEmptyAndWildcard(t *testing.T) {
	for _, tc := range []struct {
		id      string
		actions []string
	}{
		{"", []string{"github.get_current_user"}}, {"c1", nil}, {"c1", []string{"*"}},
	} {
		if _, err := NewGrant(tc.id, tc.actions); err == nil {
			t.Fatal("unsafe grant accepted")
		}
	}
}

func TestGrantRejectsBlankIDBlankAndPathActions(t *testing.T) {
	for _, tc := range []struct {
		id      string
		actions []string
	}{
		{"   ", []string{"github.get_current_user"}},
		{"c1", []string{}},
		{"c1", []string{"github.get_current_user", ""}},
		{"c1", []string{"svc.*"}},
		{"c1", []string{"svc/get"}},
		{"c1", []string{"svc?get"}},
	} {
		if _, err := NewGrant(tc.id, tc.actions); err == nil {
			t.Fatalf("unsafe grant accepted: id=%q actions=%v", tc.id, tc.actions)
		}
	}
}

func TestNewGrantBuildsScopedGrant(t *testing.T) {
	g, err := NewGrant("conn-1", []string{"github.get_current_user", "arxiv.get_paper"})
	if err != nil {
		t.Fatalf("valid grant rejected: %v", err)
	}
	if !reflect.DeepEqual(g.AllowedConnections, []string{"conn-1"}) {
		t.Fatalf("allowedConnections = %v", g.AllowedConnections)
	}
	if !reflect.DeepEqual(g.AllowedActions, []string{"github.get_current_user", "arxiv.get_paper"}) {
		t.Fatalf("allowedActions = %v", g.AllowedActions)
	}
	if g.AllowedProxies == nil || len(g.AllowedProxies) != 0 {
		t.Fatalf("allowedProxies must be non-nil empty, got %#v", g.AllowedProxies)
	}
	if g.BlockedActions == nil || len(g.BlockedActions) != 0 {
		t.Fatalf("blockedActions must be non-nil empty, got %#v", g.BlockedActions)
	}
}

// TestGrantWireFieldNames pins the exact camelCase wire field names frozen by
// T01 (contract doc §3.2): upstream PUT /api/runtime-tokens/:id requires ALL
// FOUR arrays; a missing array is a 400 and an extra field (e.g.
// blockedProxies) is rejected as invalid_input.
func TestGrantWireFieldNames(t *testing.T) {
	g, err := NewGrant("conn-1", []string{"github.get_current_user"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"allowedConnections", "allowedActions", "allowedProxies", "blockedActions"}
	if len(got) != len(wantKeys) {
		t.Fatalf("grant marshals to %d fields, want exactly %v: %s", len(got), wantKeys, raw)
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Fatalf("grant JSON missing field %q: %s", k, raw)
		}
	}
	for _, k := range []string{"allowedProxies", "blockedActions"} {
		if !strings.Contains(string(raw), `"`+k+`":[]`) {
			t.Fatalf("field %s must marshal as [] (all four arrays required upstream): %s", k, raw)
		}
	}
}

// TestNewGrantCopiesActions ensures a caller mutating its input slice after
// construction cannot widen a grant already handed to token issuance.
func TestNewGrantCopiesActions(t *testing.T) {
	actions := []string{"github.get_current_user"}
	g, err := NewGrant("conn-1", actions)
	if err != nil {
		t.Fatal(err)
	}
	actions[0] = "*"
	if g.AllowedActions[0] != "github.get_current_user" {
		t.Fatalf("grant aliases caller slice: %v", g.AllowedActions)
	}
}
