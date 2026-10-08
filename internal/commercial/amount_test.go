package commercial

import (
	"math"
	"testing"
)

func TestParseCredits(t *testing.T) {
	tests := []struct {
		input string
		want  Credits
	}{
		{"0.000001", 1},
		{"1.000000", 1_000_000},
		{"9223372036854.775807", Credits(math.MaxInt64)},
	}
	for _, tt := range tests {
		got, err := ParseCredits(tt.input)
		if err != nil || got != tt.want {
			t.Errorf("ParseCredits(%q) = %d, %v; want %d", tt.input, got, err, tt.want)
		}
	}
	for _, input := range []string{"0.0000001", "-1", "9223372036854.775808", "1e2", "1/2", "NaN", "Inf", ""} {
		if _, err := ParseCredits(input); err == nil {
			t.Errorf("ParseCredits(%q) unexpectedly succeeded", input)
		}
	}
}

func TestCreditsString(t *testing.T) {
	if got := Credits(1).String(); got != "0.000001" {
		t.Fatal(got)
	}
	if got := Credits(1_000_000).String(); got != "1.000000" {
		t.Fatal(got)
	}
}
