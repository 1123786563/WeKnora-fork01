package craft

import (
	"strings"
	"unicode/utf8"
)

// MaxTitleRunes caps a craft session title at creation. The cap counts runes,
// not bytes, so a CJK title of 120 characters is as acceptable as an ASCII
// one (W03: 作品会话与 HTTP 入口).
const MaxTitleRunes = 120

// craftKinds is the closed set of artwork kinds a create request may name.
// A syntactically valid kind is still subject to the deployment's feature
// gate (craft.kinds): type validity is necessary, not sufficient.
var craftKinds = map[string]struct{}{
	"web":         {},
	"document":    {},
	"spreadsheet": {},
	"slides":      {},
}

// Kinds returns the known artwork kinds in a stable order.
func Kinds() []string { return []string{"web", "document", "spreadsheet", "slides"} }

// KnownKind reports whether kind is part of the closed kind set.
func KnownKind(kind string) bool {
	_, ok := craftKinds[kind]
	return ok
}

// CreateRequest is the complete client-facing surface of a craft session
// creation: an idempotency key, a human title and the artwork kind. Every
// other field of the created session is server-derived.
type CreateRequest struct {
	RequestID string
	Title     string
	Kind      string
}

// ValidateCreate enforces the create rules (brief Step 3): a non-blank
// idempotency key, a non-blank title within MaxTitleRunes, and a kind from
// the closed set. It answers ErrInvalidInput for shape failures and
// ErrUnsupported for an unknown kind.
func ValidateCreate(r CreateRequest) error {
	if strings.TrimSpace(r.RequestID) == "" || strings.TrimSpace(r.Title) == "" || utf8.RuneCountInString(r.Title) > MaxTitleRunes {
		return ErrInvalidInput
	}
	switch r.Kind {
	case "web", "document", "spreadsheet", "slides":
		return nil
	}
	return ErrUnsupported
}
