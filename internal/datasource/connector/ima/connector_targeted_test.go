package ima

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The ima external id is a one-way logical-key hash, so targeted refetch
// degrades to a recognizable precondition error (Ruling P-1). These tests pin
// the degradation contract: the error is identifiable, actionable, and safe on
// any input — including nil config and empty ids — without panicking or
// touching the network.
func TestFetchByExternalID_DegradesWithPreconditionError(t *testing.T) {
	for _, tc := range []struct {
		name       string
		config     interface{}
		externalID string
	}{
		{name: "typical id", externalID: "ima_0123456789abcdef0123456789abcdef"},
		{name: "empty id", externalID: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item, err := NewConnector().FetchByExternalID(
				context.Background(), nil, tc.externalID,
			)
			if item != nil {
				t.Fatalf("FetchByExternalID() item = %#v, want nil", item)
			}
			if err == nil {
				t.Fatal("FetchByExternalID() must return the degradation error")
			}
			if !errors.Is(err, ErrTargetedRefetchUnsupported) {
				t.Fatalf("error must wrap ErrTargetedRefetchUnsupported, got: %v", err)
			}
			// The message must name the remedy so operators know what to do.
			for _, want := range []string{"one-way", "incremental"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error message should mention %q, got: %v", want, err)
				}
			}
			if !strings.Contains(err.Error(), tc.externalID) {
				t.Errorf("error should carry the external id %q, got: %v", tc.externalID, err)
			}
		})
	}
}

// TestFetchByExternalID_NilReceiverSafety pins the "never panics" part of the
// degradation contract: the method must not dereference the connector.
func TestFetchByExternalID_NilReceiverSafety(t *testing.T) {
	var c *Connector
	_, err := c.FetchByExternalID(context.Background(), nil, "ima_anything")
	if !errors.Is(err, ErrTargetedRefetchUnsupported) {
		t.Fatalf("nil receiver must still return the degradation error, got: %v", err)
	}
}
