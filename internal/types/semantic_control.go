package types

// SemanticMutation describes one business change that must carry semantic
// side effects. Everything in the mutation - the business resource update
// (via the transaction callback), the document revision CAS, the outbox
// event, and (on delete) the denial barrier plus access-epoch bump -
// commits in a SINGLE database transaction (spec section 6).
type SemanticMutation struct {
	TenantID         uint64
	KBID             string
	DocumentID       string
	ExpectedRevision uint64
	Deleted          bool
	Payload          []byte
}

// SemanticOutboxEvent is one dispatchable semantic event. EventID is the
// deterministic identity (tenant, kb, document, revision) enabling
// at-least-once delivery with receiver-side exactly-once effects.
type SemanticOutboxEvent struct {
	EventID     string
	TenantID    uint64
	KBID        string
	DocumentID  string
	Revision    uint64
	PayloadHash string
	Payload     []byte
	Attempts    int
}
