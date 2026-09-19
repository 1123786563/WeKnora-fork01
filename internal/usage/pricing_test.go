package usage

import "testing"

func TestRatesFromEnvParsesValidJSON(t *testing.T) {
	raw := `{"qwen3-8b":{"rate_micro":1200,"units":1000000},"deepseek-chat":{"rate_micro":2500,"units":1000000}}`
	rates, err := RatesFromEnv(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 2 {
		t.Fatalf("rates = %d entries, want 2", len(rates))
	}
	if got := rates["qwen3-8b"]; got != (Rate{RateMicro: 1200, Units: 1000000}) {
		t.Fatalf("qwen3-8b rate = %+v, want {1200 1000000}", got)
	}
	if got := rates["deepseek-chat"]; got != (Rate{RateMicro: 2500, Units: 1000000}) {
		t.Fatalf("deepseek-chat rate = %+v, want {2500 1000000}", got)
	}
}

func TestRatesFromEnvRejectsBadJSON(t *testing.T) {
	for _, raw := range []string{
		"{",
		"not json",
		`[]`,
	} {
		if _, err := RatesFromEnv(raw); err == nil {
			t.Fatalf("RatesFromEnv(%q) succeeded, want error", raw)
		}
	}
}

func TestRatesFromEnvRejectsNonPositiveRates(t *testing.T) {
	for name, raw := range map[string]string{
		"rate_micro zero":        `{"m":{"rate_micro":0,"units":1}}`,
		"rate_micro negative":    `{"m":{"rate_micro":-5,"units":1}}`,
		"units zero":             `{"m":{"rate_micro":1,"units":0}}`,
		"units negative":         `{"m":{"rate_micro":1,"units":-2}}`,
		"units missing defaults": `{"m":{"rate_micro":1}}`,
	} {
		if _, err := RatesFromEnv(raw); err == nil {
			t.Fatalf("%s: RatesFromEnv succeeded, want error", name)
		}
	}
}

func TestRatesFromEnvEmptyReturnsEmptyRates(t *testing.T) {
	rates, err := RatesFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 0 {
		t.Fatalf("rates = %d entries, want 0", len(rates))
	}
}

func TestCostMicroUnknownModelIsFree(t *testing.T) {
	rates, err := RatesFromEnv(`{"m":{"rate_micro":1500,"units":1000000}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := rates.CostMicro("unknown-model", 3333, 3333); got != 0 {
		t.Fatalf("CostMicro(unknown) = %d, want 0", got)
	}
}

// Pricing must stay exact in big.Rat and round exactly once, half away from
// zero, matching the internal/commercial ChargeRat + roundHalfAwayFromZero
// contract reused per SP12 Ruling P-1.
func TestCostMicroRoundsOnceHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		name        string
		rate        Rate
		inputTok    int64
		outputTok   int64
		wantCostMic int64
	}{
		// 1500*3333/1e6 = 4.9995 → half away from zero → 5.
		{"4.9995 rounds up", Rate{RateMicro: 1500, Units: 1000000}, 3333, 0, 5},
		{"4.9995 rounds up via output", Rate{RateMicro: 1500, Units: 1000000}, 0, 3333, 5},
		// Input and output are summed before multiplying: 1000+2333 = 3333.
		{"input plus output then price", Rate{RateMicro: 1500, Units: 1000000}, 1000, 2333, 5},
		// 1*1/3 = 0.333... → 0.
		{"one third rounds down", Rate{RateMicro: 1, Units: 3}, 1, 0, 0},
		// 1*1/2 = 0.5 → half away from zero → 1.
		{"exact half rounds away from zero", Rate{RateMicro: 1, Units: 2}, 1, 0, 1},
		// 1200*1000000/1000000 = 1200 exactly.
		{"exact integer stays exact", Rate{RateMicro: 1200, Units: 1000000}, 1000000, 0, 1200},
	}
	for _, tc := range cases {
		rates := ModelRates{"m": tc.rate}
		if got := rates.CostMicro("m", tc.inputTok, tc.outputTok); got != tc.wantCostMic {
			t.Fatalf("%s: CostMicro = %d, want %d", tc.name, got, tc.wantCostMic)
		}
	}
}
