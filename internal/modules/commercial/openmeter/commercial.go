// Package openmeter implements the commercial benefit gateway against the
// OpenMeter API family selected by V03 (selected-model.json, family
// official_v3). Only that one model is implemented; every field maps onto
// the schema recorded in the validated contract-case inventory
// (docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json,
// operations create-credit-grant / list-credit-grant /
// update-credit-grant-external-settlement, all status schema_only).
//
// OM-01..OM-10 remain blocked-env: no OpenMeter service or merchant
// credentials were available, so this adapter carries no runtime evidence.
// Construction always succeeds so operators can boot with the connector
// unconfigured; calls then fail fast with ErrGatewayUnconfigured instead of
// fabricating grants.
package openmeter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
)

// FamilyOfficialV3 is the single model family this gateway implements, per
// the V03 selected-model record.
const FamilyOfficialV3 = "official_v3"

// Environment references read by ConfigFromEnv.
const (
	EnvFamily  = "WEKNORA_COMMERCIAL_GATEWAY_FAMILY"
	EnvBaseURL = "WEKNORA_COMMERCIAL_GATEWAY_URL"
	EnvAPIKey  = "WEKNORA_COMMERCIAL_GATEWAY_API_KEY"
)

// Config holds the gateway config references. An empty BaseURL or APIKey is
// legal at construction time and surfaces as ErrGatewayUnconfigured on call.
type Config struct {
	Family  string
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// ConfigFromEnv reads the config references; unset env means unconfigured.
func ConfigFromEnv() Config {
	return Config{
		Family:  os.Getenv(EnvFamily),
		BaseURL: os.Getenv(EnvBaseURL),
		APIKey:  os.Getenv(EnvAPIKey),
	}
}

// ErrUnsupportedFamily rejects anything but the V03-selected model.
var ErrUnsupportedFamily = fmt.Errorf("gateway family must be %q", FamilyOfficialV3)

// Gateway implements domain.CommercialGateway for the official_v3 model.
type Gateway struct {
	cfg    Config
	client *http.Client
}

// NewGateway validates the family and returns the gateway. Unconfigured
// endpoint/credentials are accepted here on purpose: the blocker is
// environmental (blocked-env), and boot must not depend on it.
func NewGateway(cfg Config) (*Gateway, error) {
	if cfg.Family == "" {
		cfg.Family = FamilyOfficialV3
	}
	if cfg.Family != FamilyOfficialV3 {
		return nil, ErrUnsupportedFamily
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Gateway{cfg: cfg, client: client}, nil
}

// NewGatewayFromEnv builds the gateway from its environment references.
func NewGatewayFromEnv() (*Gateway, error) {
	return NewGateway(ConfigFromEnv())
}

type grantRequest struct {
	IdempotencyKey string            `json:"idempotencyKey"`
	Amount         string            `json:"amount"`
	FeatureKey     string            `json:"featureKey,omitempty"`
	EffectiveAt    string            `json:"effectiveAt"`
	ExpiresAt      string            `json:"expiresAt,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type grantResponse struct {
	ID          string `json:"id"`
	EffectiveAt string `json:"effectiveAt"`
}

type grantListResponse struct {
	Items []grantResponse `json:"items"`
}

// ApplyBenefit creates one credit grant keyed by the request's
// FulfillmentKey (create-credit-grant). The idempotencyKey is the
// settlement identity, so a replay of the same key can never double-grant.
// Timeouts and 5xx are indeterminate: the remote may have persisted the
// grant while the response was lost, so they surface as
// ErrGatewayIndeterminate, never as success.
func (g *Gateway) ApplyBenefit(ctx context.Context, req domain.BenefitRequest) (domain.BenefitReceipt, error) {
	if err := req.Validate(); err != nil {
		return domain.BenefitReceipt{}, err
	}
	if err := g.configured(); err != nil {
		return domain.BenefitReceipt{}, err
	}
	body := grantRequest{
		IdempotencyKey: req.Key,
		Amount:         req.Credits.String(),
		FeatureKey:     req.PlanRef,
		EffectiveAt:    req.EffectiveAt.UTC().Format(time.RFC3339Nano),
		Metadata: map[string]string{
			"tenantId": strconv.FormatUint(req.TenantID, 10),
			"kind":     req.Kind,
			"planRef":  req.PlanRef,
		},
	}
	if !req.ExpiresAt.IsZero() {
		body.ExpiresAt = req.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	var resp grantResponse
	if err := g.call(ctx, http.MethodPost,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants", url.PathEscape(req.CustomerID)),
		body, &resp); err != nil {
		return domain.BenefitReceipt{}, err
	}
	if resp.ID == "" {
		// A 2xx without a settlement identity proves nothing about the
		// remote state; treat it as indeterminate, not as success.
		return domain.BenefitReceipt{}, fmt.Errorf("%w: grant response carried no id", domain.ErrGatewayIndeterminate)
	}
	receipt := domain.BenefitReceipt{ExternalID: resp.ID}
	if t, err := time.Parse(time.RFC3339Nano, resp.EffectiveAt); err == nil {
		receipt.EffectiveAt = t
	}
	return receipt, nil
}

// FindBenefit maps the provider lookup (list-credit-grants filtered by
// idempotencyKey, scoped to the customer carried on the context by the
// worker) onto a receipt. An empty list is a provable miss
// (ErrBenefitNotFound); an unresolvable customer scope is indeterminate:
// claiming not-found without looking would license a duplicate grant.
func (g *Gateway) FindBenefit(ctx context.Context, key string) (domain.BenefitReceipt, error) {
	if key == "" {
		return domain.BenefitReceipt{}, domain.ErrInvalidBenefitRequest
	}
	if err := g.configured(); err != nil {
		return domain.BenefitReceipt{}, err
	}
	customer := domain.BenefitCustomerFrom(ctx)
	if customer == "" {
		return domain.BenefitReceipt{}, fmt.Errorf("%w: no customer scope for lookup", domain.ErrGatewayIndeterminate)
	}
	var list grantListResponse
	q := url.Values{"idempotencyKey": []string{key}}
	if err := g.call(ctx, http.MethodGet,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants?%s", url.PathEscape(customer), q.Encode()),
		nil, &list); err != nil {
		return domain.BenefitReceipt{}, err
	}
	if len(list.Items) == 0 {
		return domain.BenefitReceipt{}, domain.ErrBenefitNotFound
	}
	item := list.Items[0]
	receipt := domain.BenefitReceipt{ExternalID: item.ID}
	if t, err := time.Parse(time.RFC3339Nano, item.EffectiveAt); err == nil {
		receipt.EffectiveAt = t
	}
	return receipt, nil
}

type settlementRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
	Amount         string `json:"amount"`
}

// RevokeBenefit claws back exactly credits from the grant settled under
// key, mapped onto the SAME validated schema family (update-credit-grant
// -external-settlement, schema_only per the V03 inventory): the grant is
// first resolved by its idempotencyKey (list-credit-grants, scoped to the
// customer carried on the context), then settled by the precise negative
// amount under a revoke idempotency key derived from (key, credits) — a
// replay of the same revocation can never revoke twice. Unconfigured
// endpoint/credentials fail fast with ErrGatewayUnconfigured, exactly
// like ApplyBenefit/FindBenefit (OM-01..OM-10 remain blocked-env; this
// adapter carries no runtime evidence for revocation either).
func (g *Gateway) RevokeBenefit(ctx context.Context, key string, credits domain.Credits) error {
	if key == "" || credits <= 0 {
		return domain.ErrInvalidBenefitRequest
	}
	if err := g.configured(); err != nil {
		return err
	}
	customer := domain.BenefitCustomerFrom(ctx)
	if customer == "" {
		return fmt.Errorf("%w: no customer scope for revocation", domain.ErrGatewayIndeterminate)
	}
	var list grantListResponse
	q := url.Values{"idempotencyKey": []string{key}}
	if err := g.call(ctx, http.MethodGet,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants?%s", url.PathEscape(customer), q.Encode()),
		nil, &list); err != nil {
		return err
	}
	if len(list.Items) == 0 {
		// The grant never settled, so there is provably nothing to revoke.
		return domain.ErrBenefitNotFound
	}
	body := settlementRequest{
		IdempotencyKey: "revoke:" + key + ":" + credits.String(),
		Amount:         (-credits).String(),
	}
	return g.call(ctx, http.MethodPatch,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants/%s/settlement", url.PathEscape(customer), url.PathEscape(list.Items[0].ID)),
		body, nil)
}

type settlementCallResponse struct {
	ID        string `json:"id"`
	Watermark string `json:"watermark,omitempty"`
}

// Settle maps one call's final consumption onto the V03-selected
// official_v3 family (update-credit-grant-external-settlement, schema_only
// in the validated inventory): the funding grant of the customer carried on
// the context is resolved via list-credit-grants, then the precise negative
// amount is settled under the settlement's revision-derived idempotency key
// — a replay of the same revision can never double-settle. HONESTY NOTE:
// grant resolution and the response correlation fields below are
// schema_only; OM-01..OM-10 remain blocked-env and the V03 gate cannot
// verify the ack strategy without the real service, so this adapter carries
// NO runtime evidence. Ingest acceptance here is NOT confirmation: the
// receipt carries a watermark only when the provider returned one.
func (g *Gateway) Settle(ctx context.Context, st domain.Settlement) (domain.SettlementReceipt, error) {
	if err := st.Validate(); err != nil {
		return domain.SettlementReceipt{}, err
	}
	if err := g.configured(); err != nil {
		return domain.SettlementReceipt{}, err
	}
	customer := domain.BenefitCustomerFrom(ctx)
	if customer == "" {
		return domain.SettlementReceipt{}, fmt.Errorf("%w: no customer scope for settlement", domain.ErrGatewayIndeterminate)
	}
	var list grantListResponse
	if err := g.call(ctx, http.MethodGet,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants", url.PathEscape(customer)),
		nil, &list); err != nil {
		return domain.SettlementReceipt{}, err
	}
	if len(list.Items) == 0 {
		// No funding grant: provably nothing to settle against.
		return domain.SettlementReceipt{}, domain.ErrBenefitNotFound
	}
	body := settlementRequest{
		IdempotencyKey: st.ID,
		Amount:         (-st.Amount).String(),
	}
	var resp settlementCallResponse
	if err := g.call(ctx, http.MethodPatch,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants/%s/settlement", url.PathEscape(customer), url.PathEscape(list.Items[0].ID)),
		body, &resp); err != nil {
		return domain.SettlementReceipt{}, err
	}
	if resp.ID == "" {
		// A 2xx without a transaction identity proves nothing about the
		// remote state; treat it as indeterminate, never as success.
		return domain.SettlementReceipt{}, fmt.Errorf("%w: settlement response carried no id", domain.ErrGatewayIndeterminate)
	}
	return domain.SettlementReceipt{ExternalID: resp.ID, Watermark: resp.Watermark}, nil
}

// ConfirmSettlement resolves the explicit confirmation of a settled
// transaction by re-issuing the SAME idempotent settlement call: the
// provider answers with the current externally-recorded state of that exact
// transaction key, so confirmation evidence is event/transaction
// correlation, never an inferred balance drop. A response without a
// watermark is NOT confirmation (ErrGatewayIndeterminate) — the caller
// retains the settlement for reconciliation. Same schema_only/blocked-env
// honesty note as Settle.
func (g *Gateway) ConfirmSettlement(ctx context.Context, settlementID string) (domain.SettlementReceipt, error) {
	if settlementID == "" {
		return domain.SettlementReceipt{}, domain.ErrInvalidSettlement
	}
	if err := g.configured(); err != nil {
		return domain.SettlementReceipt{}, err
	}
	customer := domain.BenefitCustomerFrom(ctx)
	if customer == "" {
		return domain.SettlementReceipt{}, fmt.Errorf("%w: no customer scope for settlement confirmation", domain.ErrGatewayIndeterminate)
	}
	var list grantListResponse
	if err := g.call(ctx, http.MethodGet,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants", url.PathEscape(customer)),
		nil, &list); err != nil {
		return domain.SettlementReceipt{}, err
	}
	if len(list.Items) == 0 {
		return domain.SettlementReceipt{}, domain.ErrBenefitNotFound
	}
	// Idempotent replay of the same settlement key reads back the recorded
	// transaction instead of creating a second one.
	body := settlementRequest{IdempotencyKey: settlementID, Amount: "0"}
	var resp settlementCallResponse
	if err := g.call(ctx, http.MethodPatch,
		fmt.Sprintf("/api/v3/openmeter/customers/%s/credits/grants/%s/settlement", url.PathEscape(customer), url.PathEscape(list.Items[0].ID)),
		body, &resp); err != nil {
		return domain.SettlementReceipt{}, err
	}
	if resp.ID == "" {
		return domain.SettlementReceipt{}, fmt.Errorf("%w: confirmation carried no transaction identity", domain.ErrGatewayIndeterminate)
	}
	if resp.Watermark == "" {
		return domain.SettlementReceipt{}, fmt.Errorf("%w: confirmation carried no watermark — not evidence of consumption", domain.ErrGatewayIndeterminate)
	}
	return domain.SettlementReceipt{ExternalID: resp.ID, Watermark: resp.Watermark}, nil
}

func (g *Gateway) configured() error {
	if g.cfg.BaseURL == "" || g.cfg.APIKey == "" {
		return fmt.Errorf("%w: openmeter %s endpoint/credentials not set (OM-01..OM-10 blocked-env)",
			domain.ErrGatewayUnconfigured, FamilyOfficialV3)
	}
	return nil
}

func (g *Gateway) call(ctx context.Context, method, path string, req any, out any) error {
	var reader io.Reader
	if req != nil {
		blob, err := json.Marshal(req)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(blob)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, g.cfg.BaseURL+path, reader)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.cfg.APIKey)
	if req != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client.Do(httpReq)
	if err != nil {
		// Timeouts and dropped connections cannot prove the remote state.
		return fmt.Errorf("%w: %v", domain.ErrGatewayIndeterminate, err)
	}
	defer resp.Body.Close()
	blob, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: reading response: %v", domain.ErrGatewayIndeterminate, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
	case resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests:
		return fmt.Errorf("%w: %d %s", domain.ErrGatewayBusinessRefusal, resp.StatusCode, bytes.TrimSpace(blob))
	default:
		return fmt.Errorf("%w: %d %s", domain.ErrGatewayIndeterminate, resp.StatusCode, bytes.TrimSpace(blob))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(blob, out); err != nil {
		return fmt.Errorf("%w: undecodable response: %v", domain.ErrGatewayIndeterminate, err)
	}
	return nil
}
