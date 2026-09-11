package semantic

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"google.golang.org/protobuf/proto"
)

func unmarshalWire(source proto.Message, target proto.Message) error {
	raw, err := proto.Marshal(source)
	if err != nil {
		return err
	}
	return proto.Unmarshal(raw, target)
}

func parseUint64String(value any) (uint64, error) {
	switch typed := value.(type) {
	case string:
		return strconv.ParseUint(typed, 10, 64)
	default:
		// never round uint64 through float64 in the oracle either
		return 0, strconv.ErrSyntax
	}
}

func parseOptionalUint32String(value any) (*uint32, error) {
	if value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, strconv.ErrSyntax
	}
	parsed, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return nil, err
	}
	casted := uint32(parsed)
	return &casted, nil
}

// fixtureJSON marshals a fixture subtree so domain types can be loaded
// through their own JSON boundary (decimal-string uint64 handling included).
func fixtureJSON(t *testing.T, key string) []byte {
	t.Helper()
	raw, err := json.Marshal(loadFixture(t)[key])
	if err != nil {
		t.Fatalf("remarshal fixture %q: %v", key, err)
	}
	return raw
}

// TestContractAllDTOsRoundTripWireAndJSON drives EVERY fixture entry through
// domain-JSON -> wire -> bytes -> domain, pinning the complete C01 mapping
// surface (a dropped optional or a missing mapping fails here).
func TestContractAllDTOsRoundTripWireAndJSON(t *testing.T) {
	t.Run("chunk", func(t *testing.T) {
		var domain types.SemanticChunkSnapshot
		if err := json.Unmarshal(fixtureJSON(t, "chunk"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		parsed := &semanticpb.ChunkSnapshot{}
		if err := unmarshalWire(ChunkToWire(domain), parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := ChunkFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if restored != domain {
			t.Fatalf("chunk mismatch: %+v != %+v", restored, domain)
		}
	})

	applyCase := func(t *testing.T, key string) {
		var domain types.SemanticApplyRequest
		if err := json.Unmarshal(fixtureJSON(t, key), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		wire, err := ApplyRequestToWire(domain)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		parsed := &semanticpb.ApplyRequest{}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := ApplyRequestFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("apply request mismatch: %+v != %+v", restored, domain)
		}
	}
	t.Run("apply_request", func(t *testing.T) { applyCase(t, "apply_request") })
	t.Run("apply_request_with_manifest", func(t *testing.T) { applyCase(t, "apply_request_with_manifest") })

	t.Run("access_scope", func(t *testing.T) {
		var domain types.SemanticAccessScope
		if err := json.Unmarshal(fixtureJSON(t, "access_scope"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		wire, err := AccessScopeToWire(domain)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		parsed := &semanticpb.AccessScope{}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := AccessScopeFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("access scope mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("search_request", func(t *testing.T) {
		var domain types.SemanticSearchRequest
		if err := json.Unmarshal(fixtureJSON(t, "search_request"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		parsed := &semanticpb.SearchRequest{}
		wire, err := SearchRequestToWire(domain, nil)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := SearchRequestFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("search request mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("search_response", func(t *testing.T) {
		var domain types.SemanticSearchResponse
		if err := json.Unmarshal(fixtureJSON(t, "search_response"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		parsed := &semanticpb.SearchResponse{}
		if err := unmarshalWire(SearchResponseToWire(domain), parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := SearchResponseFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("search response mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("reason_request", func(t *testing.T) {
		var search types.SemanticSearchRequest
		if err := json.Unmarshal(fixtureJSON(t, "search_request"), &search); err != nil {
			t.Fatalf("unmarshal search: %v", err)
		}
		domain := types.SemanticReasonRequest{Search: search, ReasoningMode: types.SemanticReasoningModeRules, RuleSetVersion: "r-1"}
		parsed := &semanticpb.ReasonRequest{}
		wire, err := ReasonRequestToWire(domain, nil)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := ReasonRequestFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("reason request mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("reason_response", func(t *testing.T) {
		var domain types.SemanticReasonResponse
		if err := json.Unmarshal(fixtureJSON(t, "reason_response"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		parsed := &semanticpb.ReasonResponse{}
		wire, err := ReasonResponseToWire(domain)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := ReasonResponseFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("reason response mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		var domain types.SemanticCapabilities
		if err := json.Unmarshal(fixtureJSON(t, "capabilities"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		parsed := &semanticpb.GetCapabilitiesResponse{}
		if err := unmarshalWire(CapabilitiesToWire(domain), parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := CapabilitiesFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if !reflect.DeepEqual(restored, domain) {
			t.Fatalf("capabilities mismatch: %+v != %+v", restored, domain)
		}
	})

	t.Run("operation_failed_keeps_error_detail", func(t *testing.T) {
		var domain types.SemanticOperation
		if err := json.Unmarshal(fixtureJSON(t, "operation_failed"), &domain); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		wire, err := OperationToWire(domain)
		if err != nil {
			t.Fatalf("to wire: %v", err)
		}
		parsed := &semanticpb.Operation{}
		if err := unmarshalWire(wire, parsed); err != nil {
			t.Fatalf("wire: %v", err)
		}
		restored, err := OperationFromWire(parsed)
		if err != nil {
			t.Fatalf("from wire: %v", err)
		}
		if restored.ErrorCode == nil || *restored.ErrorCode != "E_DOCUMENT_CONFLICT" {
			t.Fatalf("error detail lost: %+v", restored)
		}
		if restored.ResultGeneration != nil {
			t.Fatalf("absent result_generation must stay nil: %+v", restored)
		}
	})
}

// TestContractUnknownDomainEnumsAreErrors pins that encoding an unknown
// domain enum fails loudly instead of silently sending unspecified.
func TestContractUnknownDomainEnumsAreErrors(t *testing.T) {
	if _, err := AssertionToWire(types.SemanticAssertion{Kind: "bogus"}); err == nil {
		t.Fatal("bogus assertion kind must error")
	}
	if _, err := OperationToWire(types.SemanticOperation{State: "bogus"}); err == nil {
		t.Fatal("bogus operation state must error")
	}
	if _, err := AccessScopeToWire(types.SemanticAccessScope{Purpose: "bogus"}); err == nil {
		t.Fatal("bogus purpose must error")
	}
	if _, err := ReasonRequestToWire(types.SemanticReasonRequest{ReasoningMode: "bogus"}, nil); err == nil {
		t.Fatal("bogus reasoning mode must error")
	}
	if _, err := ReasonResponseToWire(types.SemanticReasonResponse{ConclusionKind: "bogus"}); err == nil {
		t.Fatal("bogus conclusion kind must error")
	}
}

// TestContractInvalidPurposeNeverDropsAccessScope pins that an invalid
// purpose inside a nested AccessScope must ERROR - silently dropping the
// authorization envelope would be a security hole, not a convenience.
func TestContractInvalidPurposeNeverDropsAccessScope(t *testing.T) {
	request := types.SemanticSearchRequest{
		QueryID: "q", Query: "q",
		AccessScope: &types.SemanticAccessScope{
			Scope:   types.SemanticScopeKey{TenantID: 1, KBID: "kb"},
			Purpose: types.SemanticAccessPurpose("bogus"),
		},
	}
	if _, err := SearchRequestToWire(request, nil); err == nil {
		t.Fatal("search request with invalid purpose must error, not drop access_scope")
	}
	reason := types.SemanticReasonRequest{Search: request, ReasoningMode: types.SemanticReasoningModeRules}
	if _, err := ReasonRequestToWire(reason, nil); err == nil {
		t.Fatal("reason request with invalid purpose must error, not drop access_scope")
	}
}

// TestContractNilMessagesError keeps every FromWire guarded.
func TestContractNilMessagesError(t *testing.T) {
	checks := []struct {
		name string
		call func() error
	}{
		{"scope", func() error { _, err := ScopeKeyFromWire(nil); return err }},
		{"document", func() error { _, err := DocumentRevisionFromWire(nil); return err }},
		{"operation", func() error { _, err := OperationFromWire(nil); return err }},
		{"evidence", func() error { _, err := EvidenceFromWire(nil); return err }},
		{"assertion", func() error { _, err := AssertionFromWire(nil); return err }},
		{"chunk", func() error { _, err := ChunkFromWire(nil); return err }},
		{"apply", func() error { _, err := ApplyRequestFromWire(nil); return err }},
		{"access", func() error { _, err := AccessScopeFromWire(nil); return err }},
		{"search_request", func() error { _, err := SearchRequestFromWire(nil); return err }},
		{"search_response", func() error { _, err := SearchResponseFromWire(nil); return err }},
		{"reason_request", func() error { _, err := ReasonRequestFromWire(nil); return err }},
		{"reason_response", func() error { _, err := ReasonResponseFromWire(nil); return err }},
		{"capabilities", func() error { _, err := CapabilitiesFromWire(nil); return err }},
	}
	for _, check := range checks {
		if check.call() == nil {
			t.Fatalf("%s: nil message must error", check.name)
		}
	}
}

// TestContractEvidenceJSONBoundary exercises the custom uint64/uint32 JSON
// marshalling: max revision as decimal string, absent span as null, and
// out-of-range spans rejected.
func TestContractEvidenceJSONBoundary(t *testing.T) {
	span := uint32(3)
	original := types.SemanticEvidence{EvidenceID: "ev", DocumentID: "doc", Revision: maxUint64, ChunkID: "c", ContentHash: "h", Quote: "控股", StartChar: &span}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if raw["revision"] != "18446744073709551615" {
		t.Fatalf("revision must stay a decimal string, got %v", raw["revision"])
	}
	if raw["start_char"] != "3" {
		t.Fatalf("span must be a decimal string, got %v", raw["start_char"])
	}
	if raw["end_char"] != nil {
		t.Fatalf("absent span must be null, got %v", raw["end_char"])
	}
	var restored types.SemanticEvidence
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !sameEvidence(restored, original) {
		t.Fatalf("evidence json mismatch: %+v != %+v", restored, original)
	}
	invalid := []byte("{\"evidence_id\":\"e\",\"document_id\":\"d\",\"revision\":\"1\",\"chunk_id\":\"c\",\"content_hash\":\"h\",\"quote\":\"q\",\"start_char\":\"4294967296\",\"end_char\":null}")
	var overflow types.SemanticEvidence
	if err := json.Unmarshal(invalid, &overflow); err == nil {
		t.Fatal("out-of-range span must be rejected")
	}
}

func evidenceFromFixture(raw map[string]any) (types.SemanticEvidence, error) {
	revision, err := parseUint64String(raw["revision"])
	if err != nil {
		return types.SemanticEvidence{}, err
	}
	start, err := parseOptionalUint32String(raw["start_char"])
	if err != nil {
		return types.SemanticEvidence{}, err
	}
	end, err := parseOptionalUint32String(raw["end_char"])
	if err != nil {
		return types.SemanticEvidence{}, err
	}
	return types.SemanticEvidence{
		EvidenceID:  raw["evidence_id"].(string),
		DocumentID:  raw["document_id"].(string),
		Revision:    revision,
		ChunkID:     raw["chunk_id"].(string),
		ContentHash: raw["content_hash"].(string),
		Quote:       raw["quote"].(string),
		StartChar:   start,
		EndChar:     end,
	}, nil
}

func equalOptionalUint32(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameOperation(a, b types.SemanticOperation) bool {
	return a.OperationID == b.OperationID &&
		a.Scope == b.Scope &&
		a.DocumentID == b.DocumentID &&
		a.Revision == b.Revision &&
		a.State == b.State &&
		a.Stage == b.Stage &&
		a.LeaseToken == b.LeaseToken &&
		equalOptionalString(a.ResultGeneration, b.ResultGeneration) &&
		equalOptionalString(a.ErrorCode, b.ErrorCode)
}

func sameEvidence(a, b types.SemanticEvidence) bool {
	return a.EvidenceID == b.EvidenceID &&
		a.DocumentID == b.DocumentID &&
		a.Revision == b.Revision &&
		a.ChunkID == b.ChunkID &&
		a.ContentHash == b.ContentHash &&
		a.Quote == b.Quote &&
		equalOptionalUint32(a.StartChar, b.StartChar) &&
		equalOptionalUint32(a.EndChar, b.EndChar)
}

func operationFromFixture(t *testing.T, raw map[string]any) (types.SemanticOperation, error) {
	t.Helper()
	revision, err := parseUint64String(raw["revision"])
	if err != nil {
		return types.SemanticOperation{}, err
	}
	lease, err := parseUint64String(raw["lease_token"])
	if err != nil {
		return types.SemanticOperation{}, err
	}
	operation := types.SemanticOperation{
		OperationID: raw["operation_id"].(string),
		Scope:       scopeFromFixture(t, raw["scope"].(map[string]any)),
		DocumentID:  raw["document_id"].(string),
		Revision:    revision,
		State:       types.SemanticOperationState(raw["state"].(string)),
		Stage:       raw["stage"].(string),
		LeaseToken:  lease,
	}
	if value, ok := raw["result_generation"].(string); ok {
		operation.ResultGeneration = &value
	}
	if value, ok := raw["error_code"].(string); ok {
		operation.ErrorCode = &value
	}
	return operation, nil
}

const maxUint64 = uint64(math.MaxUint64)

func loadFixture(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "semantic", "tests", "fixtures", "contract-v1.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return data
}

func scopeFromFixture(t *testing.T, raw map[string]any) types.SemanticScopeKey {
	t.Helper()
	tenant, err := parseUint64String(raw["tenant_id"])
	if err != nil {
		t.Fatalf("tenant_id: %v", err)
	}
	return types.SemanticScopeKey{TenantID: tenant, KBID: raw["kb_id"].(string)}
}

// TestContractFixtureRoundTripWire walks the shared cross-language fixture
// through domain -> wire -> bytes -> domain for the DTOs C01 owns.
func TestContractFixtureRoundTripWire(t *testing.T) {
	data := loadFixture(t)
	scope := scopeFromFixture(t, data["scope"].(map[string]any))
	wire := ScopeKeyToWire(scope)
	parsed := &semanticpb.ScopeKey{}
	if err := unmarshalWire(wire, parsed); err != nil {
		t.Fatalf("scope wire round-trip: %v", err)
	}
	restored, err := ScopeKeyFromWire(parsed)
	if err != nil {
		t.Fatalf("scope from wire: %v", err)
	}
	if restored != scope {
		t.Fatalf("scope mismatch: %+v != %+v", restored, scope)
	}

	evRaw := data["evidence_with_span"].(map[string]any)
	ev, err := evidenceFromFixture(evRaw)
	if err != nil {
		t.Fatalf("evidence from fixture: %v", err)
	}
	evWire := EvidenceToWire(ev)
	evParsed := &semanticpb.Evidence{}
	if err := unmarshalWire(evWire, evParsed); err != nil {
		t.Fatalf("evidence wire round-trip: %v", err)
	}
	evRestored, err := EvidenceFromWire(evParsed)
	if err != nil {
		t.Fatalf("evidence from wire: %v", err)
	}
	if evRestored.StartChar == nil || *evRestored.StartChar != 3 {
		t.Fatalf("evidence span lost: %+v", evRestored)
	}
	if !sameEvidence(evRestored, ev) {
		t.Fatalf("evidence mismatch: %+v != %+v", evRestored, ev)
	}

	opRaw := data["operation"].(map[string]any)
	op, err := operationFromFixture(t, opRaw)
	if err != nil {
		t.Fatalf("operation from fixture: %v", err)
	}
	opWire, err := OperationToWire(op)
	if err != nil {
		t.Fatalf("operation to wire: %v", err)
	}
	opParsed := &semanticpb.Operation{}
	if err := unmarshalWire(opWire, opParsed); err != nil {
		t.Fatalf("operation wire round-trip: %v", err)
	}
	opRestored, err := OperationFromWire(opParsed)
	if err != nil {
		t.Fatalf("operation from wire: %v", err)
	}
	if !sameOperation(opRestored, op) {
		t.Fatalf("operation mismatch: %+v != %+v", opRestored, op)
	}
}

// TestContractUint64MaxNoFloatLoss proves tenant/revision/lease values never
// pass through float64: the JSON boundary is decimal strings and the wire
// keeps full uint64 precision.
func TestContractUint64MaxNoFloatLoss(t *testing.T) {
	scope := types.SemanticScopeKey{TenantID: maxUint64, KBID: "kb"}
	encoded, err := json.Marshal(scope)
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatalf("unmarshal scope json: %v", err)
	}
	if back["tenant_id"] != "18446744073709551615" {
		t.Fatalf("tenant_id must be a decimal string, got %v (%T)", back["tenant_id"], back["tenant_id"])
	}
	var restored types.SemanticScopeKey
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatalf("unmarshal scope: %v", err)
	}
	if restored.TenantID != maxUint64 {
		t.Fatalf("uint64 lost precision: %d", restored.TenantID)
	}

	wire := ScopeKeyToWire(scope)
	parsed := &semanticpb.ScopeKey{}
	if err := unmarshalWire(wire, parsed); err != nil {
		t.Fatalf("wire round-trip: %v", err)
	}
	fromWire, err := ScopeKeyFromWire(parsed)
	if err != nil {
		t.Fatalf("from wire: %v", err)
	}
	if fromWire.TenantID != maxUint64 {
		t.Fatalf("uint64 lost precision on wire: %d", fromWire.TenantID)
	}
}

// TestContractOptionalSpanIsNotZero proves an absent span stays nil in the
// domain model instead of silently becoming 0.
func TestContractOptionalSpanIsNotZero(t *testing.T) {
	data := loadFixture(t)
	raw := data["evidence_without_span"].(map[string]any)
	ev, err := evidenceFromFixture(raw)
	if err != nil {
		t.Fatalf("evidence from fixture: %v", err)
	}
	if ev.StartChar != nil || ev.EndChar != nil {
		t.Fatalf("absent span must stay nil, got %+v", ev)
	}
	wire := EvidenceToWire(ev)
	parsed := &semanticpb.Evidence{}
	if err := unmarshalWire(wire, parsed); err != nil {
		t.Fatalf("wire round-trip: %v", err)
	}
	restored, err := EvidenceFromWire(parsed)
	if err != nil {
		t.Fatalf("from wire: %v", err)
	}
	if restored.StartChar != nil || restored.EndChar != nil {
		t.Fatalf("absent span became non-nil after wire: %+v", restored)
	}
}

// TestContractUnknownEnumNotDefaultedToSuccess proves an unrecognized enum
// value maps to an explicit unspecified state rather than a success value.
func TestContractUnknownEnumNotDefaultedToSuccess(t *testing.T) {
	wire := &semanticpb.Assertion{
		AssertionId: "a1",
		SubjectId:   "s",
		Predicate:   "controls",
		Kind:        semanticpb.Assertion_Kind(99),
	}
	domain, err := AssertionFromWire(wire)
	if err != nil {
		t.Fatalf("unknown enum must map to unspecified, got error: %v", err)
	}
	if domain.Kind != types.SemanticAssertionKindUnspecified {
		t.Fatalf("unknown enum leaked as %q, want unspecified", domain.Kind)
	}
}

// TestContractChineseTextSurvivesWire keeps CJK content byte-exact through
// the wire and the JSON boundary.
func TestContractChineseTextSurvivesWire(t *testing.T) {
	data := loadFixture(t)
	scope := scopeFromFixture(t, data["scope"].(map[string]any))
	if scope.KBID != "kb-中文-1" {
		t.Fatalf("chinese kb id mangled: %q", scope.KBID)
	}
	wire := ScopeKeyToWire(scope)
	parsed := &semanticpb.ScopeKey{}
	if err := unmarshalWire(wire, parsed); err != nil {
		t.Fatalf("wire round-trip: %v", err)
	}
	restored, err := ScopeKeyFromWire(parsed)
	if err != nil {
		t.Fatalf("from wire: %v", err)
	}
	if restored.KBID != "kb-中文-1" {
		t.Fatalf("chinese kb id mangled on wire: %q", restored.KBID)
	}
}
