package types

import "time"

type SemanticMutation struct {
	TenantID         uint64
	KBID, DocumentID string
	ExpectedRevision uint64
	ContentHash      string
	Deleted          bool
	Payload          []byte
}
type SemanticOutboxEvent struct {
	EventID        string
	Scope          SemanticScopeKey
	DocumentID     string
	Revision       uint64
	ContentHash    string
	Deleted        bool
	Payload        []byte
	PayloadHash    string
	AttemptCount   uint64
	LeaseToken     uint64
	LeaseExpiresAt time.Time
	RetryAt        time.Time
}
