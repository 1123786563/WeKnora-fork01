package craft

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestCreateRequiresKnownKindAndKey is the brief's Step 1 verbatim test: a
// create request without an idempotency key is refused, an unknown kind is
// refused, and a well-formed request passes.
func TestCreateRequiresKnownKindAndKey(t *testing.T) {
	if ValidateCreate(CreateRequest{Title: "x", Kind: "web"}) == nil {
		t.Fatal("no key")
	}
	if ValidateCreate(CreateRequest{RequestID: "a", Title: "x", Kind: "public-hosting"}) == nil {
		t.Fatal("unsupported kind")
	}
	if err := ValidateCreate(CreateRequest{RequestID: "a", Title: "x", Kind: "web"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCreateRules(t *testing.T) {
	// Every supported kind is accepted.
	for _, kind := range Kinds() {
		if err := ValidateCreate(CreateRequest{RequestID: "a", Title: "x", Kind: kind}); err != nil {
			t.Fatalf("kind %q: %v", kind, err)
		}
	}
	// Key and title are required.
	if ValidateCreate(CreateRequest{Title: "x", Kind: "web"}) == nil {
		t.Fatal("missing request id accepted")
	}
	if ValidateCreate(CreateRequest{RequestID: " ", Title: "x", Kind: "web"}) == nil {
		t.Fatal("blank request id accepted")
	}
	if ValidateCreate(CreateRequest{RequestID: "a", Title: " ", Kind: "web"}) == nil {
		t.Fatal("blank title accepted")
	}
	// The title cap is 120 runes, not bytes: a 120-rune CJK title passes and
	// a 121-rune one is refused.
	exact := strings.Repeat("作", MaxTitleRunes)
	if err := ValidateCreate(CreateRequest{RequestID: "a", Title: exact, Kind: "web"}); err != nil {
		t.Fatalf("title of exactly %d runes refused: %v", MaxTitleRunes, err)
	}
	over := exact + "品"
	if ValidateCreate(CreateRequest{RequestID: "a", Title: over, Kind: "web"}) == nil {
		t.Fatal("over-length title accepted")
	}
	if got := utf8.RuneCountInString(over); got != MaxTitleRunes+1 {
		t.Fatalf("fixture sanity: rune count %d", got)
	}
}
