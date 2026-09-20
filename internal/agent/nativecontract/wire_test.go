package nativecontract

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type wireFixture struct {
	Protocol    string            `json:"protocol"`
	Schema      int               `json:"schema_version"`
	LastEventID string            `json:"last_event_id"`
	Events      []json.RawMessage `json:"events"`
	Pending     json.RawMessage   `json:"pending"`
	Errors      []json.RawMessage `json:"command_errors"`
	Archive     json.RawMessage   `json:"archive"`
}

func loadWireFixture(t *testing.T) wireFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve wire fixture path")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "tests", "native-agent", "wire-v1.json"))
	if err != nil {
		t.Fatalf("read wire fixture: %v", err)
	}
	var fixture wireFixture
	if err := json.Unmarshal(contents, &fixture); err != nil {
		t.Fatalf("decode wire fixture: %v", err)
	}
	return fixture
}

func decodeV1BusinessEvent(raw []byte) (BusinessEvent, error) {
	var event BusinessEvent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return BusinessEvent{}, err
	}
	if event.Protocol != EventProtocol || event.SchemaVersion != ContractVersion {
		return BusinessEvent{}, fmt.Errorf("native wire version is incompatible")
	}
	if event.EventID == "" || event.TenantID == "" || event.SessionID == "" || event.RunID == "" || event.Sequence == "" || event.Kind == "" {
		return BusinessEvent{}, fmt.Errorf("native wire event is incomplete")
	}
	if _, err := strconv.ParseInt(event.Sequence, 10, 64); err != nil || event.Sequence == "0" || strings.HasPrefix(event.Sequence, "-") {
		return BusinessEvent{}, fmt.Errorf("native wire sequence is invalid")
	}
	if !wireEventKind(event.Kind) {
		return BusinessEvent{}, fmt.Errorf("native wire event kind is unknown")
	}
	if outcome := event.Payload.Outcome; outcome != nil && (outcome.ProviderReceipt != "" || outcome.QueryAnchor != "" || string(outcome.Content) != "null") {
		return BusinessEvent{}, fmt.Errorf("native wire outcome leaks non-public data")
	}
	return event, nil
}

func wireEventKind(kind EventKind) bool {
	switch kind {
	case EventRunStatus, EventAttemptStarted, EventAttemptReplaced, EventAttemptFinished, EventTextDelta, EventReasoningDelta,
		EventToolPlanned, EventToolResult, EventDecisionRequired, EventUsage, EventArtifact, EventFailure:
		return true
	default:
		return false
	}
}

func parseV1LastEventID(value string) (runID, sequence string, err error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != "v1" {
		return "", "", fmt.Errorf("invalid Last-Event-ID")
	}
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decodeErr != nil || len(decoded) == 0 || base64.RawURLEncoding.EncodeToString(decoded) != parts[1] {
		return "", "", fmt.Errorf("invalid Last-Event-ID")
	}
	if _, parseErr := strconv.ParseInt(parts[2], 10, 64); parseErr != nil || parts[2] == "0" || strings.HasPrefix(parts[2], "-") {
		return "", "", fmt.Errorf("invalid Last-Event-ID")
	}
	return string(decoded), parts[2], nil
}

func TestWireV1FixtureRoundTripsThroughBusinessEventJSONTags(t *testing.T) {
	fixture := loadWireFixture(t)
	if fixture.Protocol != EventProtocol || fixture.Schema != ContractVersion {
		t.Fatalf("fixture version = %q/%d", fixture.Protocol, fixture.Schema)
	}
	if len(fixture.Events) == 0 {
		t.Fatal("fixture must contain events")
	}

	for index, raw := range fixture.Events {
		event, err := decodeV1BusinessEvent(raw)
		if err != nil {
			t.Fatalf("fixture event %d: %v", index, err)
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event %d: %v", index, err)
		}
		var want, got any
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("decode expected event %d: %v", index, err)
		}
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatalf("decode encoded event %d: %v", index, err)
		}
		if !jsonEqual(want, got) {
			t.Fatalf("event %d changed public wire shape\nwant: %s\n got: %s", index, raw, encoded)
		}
	}

	var detail PendingDecisionDetail
	if err := json.Unmarshal(fixture.Pending, &detail); err != nil {
		t.Fatalf("decode pending detail: %v", err)
	}
	assertWireJSONEqual(t, "pending detail", fixture.Pending, detail)
	for index, raw := range fixture.Errors {
		var failure Failure
		if err := json.Unmarshal(raw, &failure); err != nil {
			t.Fatalf("decode command error %d: %v", index, err)
		}
		assertWireJSONEqual(t, fmt.Sprintf("command error %d", index), raw, failure)
	}
	var archive ArchivePage
	if err := json.Unmarshal(fixture.Archive, &archive); err != nil {
		t.Fatalf("decode archive: %v", err)
	}
	assertWireJSONEqual(t, "archive", fixture.Archive, archive)
	if got := PublicArtifact(ArtifactRef{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:artifact", SizeBytes: 42}).SizeBytes; got != "42" {
		t.Fatalf("public artifact size = %q", got)
	}
}

func TestWireV1RejectsIncompatibleEventVersionsAndMalformedSequence(t *testing.T) {
	fixture := loadWireFixture(t)
	for _, mutation := range []func(map[string]any){
		func(event map[string]any) { event["protocol"] = "weknora.agent.v2" },
		func(event map[string]any) { event["schema_version"] = 2 },
		func(event map[string]any) { event["seq"] = "9007199254740993.1" },
		func(event map[string]any) { event["run_id"] = "" },
		func(event map[string]any) { event["kind"] = "sdk.internal" },
	} {
		var event map[string]any
		if err := json.Unmarshal(fixture.Events[0], &event); err != nil {
			t.Fatal(err)
		}
		mutation(event)
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeV1BusinessEvent(raw); err == nil {
			t.Fatalf("mutation %s was accepted", raw)
		}
	}
}

func TestWireV1RejectsPrivateToolOutcomeFields(t *testing.T) {
	fixture := loadWireFixture(t)
	var event map[string]any
	if err := json.Unmarshal(fixture.Events[3], &event); err != nil {
		t.Fatal(err)
	}
	payload := event["payload"].(map[string]any)
	outcome := payload["outcome"].(map[string]any)
	for field, value := range map[string]string{"provider_receipt": "provider-secret", "query_anchor": "request-secret"} {
		outcome[field] = value
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeV1BusinessEvent(raw); err == nil {
			t.Fatalf("private outcome field accepted: %s", field)
		}
		delete(outcome, field)
	}
}

func TestWireV1LastEventIDIsCanonicalAndScopedToFixtureRun(t *testing.T) {
	fixture := loadWireFixture(t)
	runID, sequence, err := parseV1LastEventID(fixture.LastEventID)
	if err != nil {
		t.Fatalf("parse fixture Last-Event-ID: %v", err)
	}
	if runID != "run-9007199254740993" || sequence != "8" {
		t.Fatalf("Last-Event-ID = %q/%q", runID, sequence)
	}
	for _, value := range []string{"v2:cnVuLTE:7", "v1:not-base64!:7", "v1:cnVuLTE:0", "v1:cnVuLTE:7:extra"} {
		if _, _, err := parseV1LastEventID(value); err == nil {
			t.Fatalf("invalid Last-Event-ID accepted: %q", value)
		}
	}
}

func jsonEqual(want, got any) bool {
	wantJSON, wantErr := json.Marshal(want)
	gotJSON, gotErr := json.Marshal(got)
	return wantErr == nil && gotErr == nil && bytes.Equal(wantJSON, gotJSON)
}

func assertWireJSONEqual(t *testing.T, name string, wantRaw []byte, got any) {
	t.Helper()
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	var want, actual any
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("decode expected %s: %v", name, err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatalf("decode encoded %s: %v", name, err)
	}
	if !jsonEqual(want, actual) {
		t.Fatalf("%s changed public wire shape\nwant: %s\n got: %s", name, wantRaw, encoded)
	}
}
