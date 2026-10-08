package commercial

import (
	"strings"
	"testing"
)

func TestCreatePurchaseSubscriptionPayloadValidate(t *testing.T) {
	base := func() CreatePurchaseSubscriptionPayload {
		return CreatePurchaseSubscriptionPayload{
			TenantID: 42, ExternalCustomerID: ExternalCustomerID(42),
			ExternalPurchaseSubscriptionID: ExternalPurchaseSubscriptionID(42),
			PlanCode:                       "weknora-pro-v1", AmountFen: 9900, Currency: CurrencyCNY,
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	cases := map[string]func(*CreatePurchaseSubscriptionPayload){
		"zero tenant":          func(p *CreatePurchaseSubscriptionPayload) { p.TenantID = 0 },
		"customer id mismatch": func(p *CreatePurchaseSubscriptionPayload) { p.ExternalCustomerID = "weknora-tenant-43" },
		"purchase id mismatch": func(p *CreatePurchaseSubscriptionPayload) {
			p.ExternalPurchaseSubscriptionID = "weknora-tenant-43-purchase"
		},
		"empty plan code":  func(p *CreatePurchaseSubscriptionPayload) { p.PlanCode = "" },
		"zero amount":      func(p *CreatePurchaseSubscriptionPayload) { p.AmountFen = 0 },
		"negative amount":  func(p *CreatePurchaseSubscriptionPayload) { p.AmountFen = -1 },
		"non-cny currency": func(p *CreatePurchaseSubscriptionPayload) { p.Currency = "USD" },
	}
	for name, mutate := range cases {
		p := base()
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

func TestExternalPurchaseSubscriptionIDDeterministic(t *testing.T) {
	if got := ExternalPurchaseSubscriptionID(7); got != "weknora-tenant-7-purchase" {
		t.Fatalf("identity = %q", got)
	}
	if ExternalPurchaseSubscriptionID(7) != ExternalPurchaseSubscriptionID(7) {
		t.Fatal("identity must be a pure function of tenant")
	}
}

func TestCreatePurchaseSubscriptionCommandKeyForm(t *testing.T) {
	got := CreatePurchaseSubscriptionCommandKey("weknora-tenant-7-purchase", "weknora-pro-v1")
	if got != "create_purchase_subscription:weknora-tenant-7-purchase:weknora-pro-v1" {
		t.Fatalf("key = %q", got)
	}
	if !strings.HasPrefix(got, string(CommandKindCreatePurchaseSubscription)) {
		t.Fatal("key must be prefixed with the command kind")
	}
}
