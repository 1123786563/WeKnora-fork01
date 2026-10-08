// Package usage provides chat-usage pricing for the user_usage daily
// buckets. Per SP12 Ruling P-1 the production rate source is the
// WEKNORA_USAGE_RATES environment variable (JSON) rather than a rate table:
// internal/commercial owns the pricing algorithm semantics (exact big.Rat
// arithmetic with a single half-away-from-zero rounding) but no production
// rate source, so this package reuses those semantics with the environment
// as the source. Per Ruling P-2 the pipeline prices chat traffic only;
// callers record craft traffic as 0.
package usage

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// Rate is a model's fixed-point unit price: RateMicro micro-credits per
// Units tokens.
type Rate struct {
	RateMicro int64 `json:"rate_micro"`
	Units     int64 `json:"units"`
}

// ModelRates maps a model id to its unit price.
type ModelRates map[string]Rate

// RatesFromEnv parses the raw WEKNORA_USAGE_RATES value, a JSON object of
// the form {"<model_id>":{"rate_micro":1200,"units":1000000}}. An empty
// value yields empty rates with no error (pricing simply disabled). Malformed
// JSON, or any rate with rate_micro <= 0 or units <= 0, is an error rather
// than a silent free ride.
func RatesFromEnv(raw string) (ModelRates, error) {
	if strings.TrimSpace(raw) == "" {
		return ModelRates{}, nil
	}
	var rates ModelRates
	if err := json.Unmarshal([]byte(raw), &rates); err != nil {
		return nil, fmt.Errorf("usage: parse WEKNORA_USAGE_RATES: %w", err)
	}
	if rates == nil {
		return ModelRates{}, nil
	}
	for model, r := range rates {
		if r.RateMicro <= 0 || r.Units <= 0 {
			return nil, fmt.Errorf("usage: invalid WEKNORA_USAGE_RATES entry for model %q: rate_micro=%d units=%d, both must be > 0", model, r.RateMicro, r.Units)
		}
	}
	return rates, nil
}

// CostMicro prices one chat call of the model in micro-credits. A model
// with no configured rate costs 0. The exact amount rate_micro*(input+output)/units
// is computed in big.Rat and rounded exactly once, half away from zero;
// input and output are summed before pricing (single rounding point, the
// semantics reused from internal/commercial's ChargeForCall). Split
// input/output rates are deferred until per-dimension rates actually exist.
func (m ModelRates) CostMicro(model string, inputTokens, outputTokens int64) int64 {
	rate, ok := m[model]
	if !ok {
		return 0
	}
	exact := new(big.Rat).SetFrac(
		new(big.Int).Mul(big.NewInt(rate.RateMicro), big.NewInt(inputTokens+outputTokens)),
		big.NewInt(rate.Units),
	)
	return roundHalfAwayFromZero(exact)
}

// roundHalfAwayFromZero rounds an exact rational to an integer once.
// 语义对齐 internal/commercial 的 roundHalfAwayFromZero（彼处未导出，无法直接复用）。
func roundHalfAwayFromZero(r *big.Rat) int64 {
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	if twice.CmpAbs(r.Denom()) >= 0 {
		if r.Num().Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Int64()
}
