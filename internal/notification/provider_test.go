package notification

import "testing"

func TestPushClassification(t *testing.T) {
	cases := []struct {
		code          string
		revoke, retry bool
	}{
		{"DeviceNotRegistered", true, false},
		{"MessageRateExceeded", false, true},
		{"UnknownTransport", false, true},
		{"MessageTooBig", false, false},
	}
	for _, c := range cases {
		r, b := ClassifyPushFailure(c.code)
		if r != c.revoke || b != c.retry {
			t.Fatalf("%s: got revoke=%v retry=%v", c.code, r, b)
		}
	}
}
