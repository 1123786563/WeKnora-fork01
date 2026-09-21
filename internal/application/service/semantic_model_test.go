package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/models/asr"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/models/vlm"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// This catches an authorization regression that resolves owner credentials
// before the signed capability and fresh A01 scope have been accepted.
func TestSemanticModelDeniedBeforeModelResolution(t *testing.T) {
	models := &semanticGatewayModels{}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{err: errors.New("bad capability")}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), semanticGatewayInvocations{})
	_, err := gateway.Invoke(context.Background(), types.SemanticModelWireRequest{CapabilityToken: "bad"})
	require.ErrorIs(t, err, ErrSemanticModelDenied)
	require.Zero(t, models.chatCalls)
}

// This catches a replay path that takes a fresh quota/budget hold or reaches
// provider credentials after an identical completed invocation already exists.
func TestSemanticModelCompletedReplaySkipsModelResolution(t *testing.T) {
	result := []byte(`{"text":"saved"}`)
	models := &semanticGatewayModels{}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationCompletedReplay, Result: types.SemanticModelInvocationResult{Result: result}}}
	reserves := 0
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		reserves++
		return domain.Reservation{}, nil
	})
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{}, budget, invocations)
	got, err := gateway.Invoke(context.Background(), semanticGatewayWire())
	require.NoError(t, err)
	require.Equal(t, "saved", got.Text)
	require.Equal(t, 1, invocations.claimCalls)
	require.Zero(t, reserves)
	require.Zero(t, models.chatCalls)
	require.Zero(t, models.providerCalls)
	require.Zero(t, invocations.markDispatched)
	require.Zero(t, budget.finishCalls)
	require.Zero(t, invocations.complete)
}

// Policy-disabled verification is a distinct admission denial: no claim,
// platform hold, credential lookup, or provider work is authorized.
func TestSemanticModelPolicyDisabledDenialPerformsNoGatewayWork(t *testing.T) {
	models := &semanticGatewayModels{}
	invocations := &semanticCountingInvocations{}
	reserves := 0
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		reserves++
		return domain.Reservation{}, nil
	})
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{err: errors.New("policy disabled")}, models, semanticGatewayScope{}, budget, invocations)
	_, err := gateway.Invoke(context.Background(), semanticGatewayWire())
	require.ErrorIs(t, err, ErrSemanticModelDenied)
	require.Zero(t, invocations.claimCalls)
	require.Zero(t, reserves)
	require.Zero(t, models.chatCalls)
	require.Zero(t, models.providerCalls)
}

// A valid BYOK capability still needs a pinned immutable rate version before
// provider credentials may be resolved; BYOK only waives Credits settlement.
func TestSemanticModelBYOKMissingRatesFailsBeforeModelResolution(t *testing.T) {
	models := &semanticGatewayModels{}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), invocations)
	_, err := gateway.Invoke(context.Background(), semanticGatewayWire())
	require.ErrorIs(t, err, ErrSemanticModelRatesUnavailable)
	require.Zero(t, models.chatCalls)
	require.Equal(t, 1, invocations.failBeforeDispatch)
	require.Zero(t, invocations.markDispatched)
}

func TestSemanticModelExpiredDeadlineFailsBeforeModelResolution(t *testing.T) {
	capability := semanticGatewayCapability()
	capability.Deadline = time.Now().Add(-time.Second)
	models := &semanticGatewayModels{}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), &semanticCountingInvocations{})
	_, err := gateway.Invoke(context.Background(), semanticGatewayWire())
	require.ErrorIs(t, err, ErrSemanticModelDenied)
	require.Zero(t, models.chatCalls)
}

func TestSemanticUsageRejectsPartialAndInconsistentProviderFacts(t *testing.T) {
	capability := semanticGatewayCapability()
	cases := []struct {
		name  string
		usage types.TokenUsage
		valid bool
	}{
		{"missing total", types.TokenUsage{PromptTokens: 2, CompletionTokens: 3}, false},
		{"unequal total", types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 4}, false},
		{"negative component", types.TokenUsage{PromptTokens: -1, CompletionTokens: 3, TotalTokens: 2}, false},
		{"input cap", types.TokenUsage{PromptTokens: 21, CompletionTokens: 3, TotalTokens: 24}, false},
		{"output cap", types.TokenUsage{PromptTokens: 2, CompletionTokens: 21, TotalTokens: 23}, false},
		{"exact observed usage", types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := semanticUsage(&types.ChatResponse{Usage: tc.usage}, capability)
			require.Equal(t, tc.valid, ok)
		})
	}
}

func TestSemanticModelPlatformBudgetDenialPreventsModelLookup(t *testing.T) {
	models := &semanticGatewayModels{}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		return domain.Reservation{}, errors.New("denied")
	})
	capability := semanticGatewayCapability()
	capability.Funding = domain.FundingPlatform
	_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.Error(t, err)
	require.Zero(t, models.chatCalls)
	require.Equal(t, 1, invocations.failBeforeDispatch)
}

// These denials must happen before credentials/model resolution. A disabled
// policy or ACL failure arrives through the verified capability/scope seam;
// exhausted owner task quota arrives through Claim.
func TestSemanticModelOwnerAdmissionDenialsPrecedeModelResolution(t *testing.T) {
	t.Run("acl scope denied", func(t *testing.T) {
		models := &semanticGatewayModels{}
		gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{err: errors.New("acl denied")}, NewSemanticModelBudgetAdapter(nil, nil, nil), &semanticCountingInvocations{})
		_, err := gateway.Invoke(context.Background(), semanticGatewayWire())
		require.ErrorIs(t, err, ErrSemanticModelDenied)
		require.Zero(t, models.chatCalls)
	})
	t.Run("task quota exhausted", func(t *testing.T) {
		models := &semanticGatewayModels{}
		invocations := &semanticCountingInvocations{claimErr: errors.New("quota exhausted")}
		gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), invocations)
		_, err := gateway.Invoke(context.Background(), semanticGatewayWire())
		require.Error(t, err)
		require.Equal(t, 1, invocations.claimCalls)
		require.Zero(t, models.chatCalls)
	})
}

func TestSemanticModelResolutionFailureReleasesOnlyUnstartedHoldAndQuota(t *testing.T) {
	models := &semanticGatewayModels{err: errors.New("model unavailable")}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		return domain.Reservation{ID: "hold"}, nil
	})
	capability := semanticGatewayCapability()
	capability.Funding = domain.FundingPlatform
	_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.Error(t, err)
	require.Equal(t, 1, models.chatCalls)
	require.Zero(t, models.providerCalls)
	require.Equal(t, 1, budget.releaseCalls)
	require.Equal(t, 1, invocations.failBeforeDispatch)
	require.Zero(t, invocations.markDispatched)
}

func TestSemanticModelInvalidUsageAfterDispatchRetainsQuotaAndHold(t *testing.T) {
	models := &semanticGatewayModels{response: &types.ChatResponse{Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3}}}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		return domain.Reservation{ID: "hold"}, nil
	})
	capability := semanticGatewayCapability()
	capability.Funding = domain.FundingPlatform
	_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.ErrorIs(t, err, ErrSemanticModelUnknown)
	require.Equal(t, 1, models.providerCalls)
	require.Zero(t, budget.finishCalls)
	require.Zero(t, budget.releaseCalls)
	require.Equal(t, 1, invocations.markUnknown)
	require.Zero(t, invocations.failBeforeDispatch)
}

func TestSemanticModelDispatchFailureRetainsHoldAndQuota(t *testing.T) {
	models := &semanticGatewayModels{response: &types.ChatResponse{Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}, markDispatchedErr: errors.New("marker")}
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		return domain.Reservation{ID: "hold"}, nil
	})
	capability := semanticGatewayCapability()
	capability.Funding = domain.FundingPlatform
	_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.Error(t, err)
	require.Equal(t, 1, models.chatCalls)
	require.Zero(t, models.providerCalls)
	require.Zero(t, budget.releaseCalls)
	require.Equal(t, 1, invocations.markUnknown)
	require.Zero(t, invocations.failBeforeDispatch)
}

func TestSemanticModelBudgetDispatchMarkerFailureRetainsHoldAndSkipsProvider(t *testing.T) {
	models := &semanticGatewayModels{response: &types.ChatResponse{Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
	budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
		return domain.Reservation{ID: "owner-hold"}, nil
	})
	budget.markDispatchedFunc = func(context.Context, semanticCapability, domain.Reservation) error {
		return errors.New("reservation marker")
	}
	capability := semanticGatewayCapability()
	capability.Funding = domain.FundingPlatform
	_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.Error(t, err)
	require.Equal(t, 1, invocations.markDispatched)
	require.Equal(t, 1, invocations.markUnknown)
	require.Zero(t, models.providerCalls)
	require.Zero(t, budget.releaseCalls)
	require.Zero(t, budget.finishCalls)
}

func TestSemanticModelMalformedUsageBecomesUnknownWithoutSettlement(t *testing.T) {
	for _, usage := range []types.TokenUsage{
		{PromptTokens: -1, CompletionTokens: 3, TotalTokens: 2},
		{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 4},
		{PromptTokens: 21, CompletionTokens: 3, TotalTokens: 24},
	} {
		t.Run("provider usage", func(t *testing.T) {
			models := &semanticGatewayModels{response: &types.ChatResponse{Usage: usage}}
			invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
			budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
				return domain.Reservation{ID: "hold"}, nil
			})
			capability := semanticGatewayCapability()
			capability.Funding = domain.FundingPlatform
			_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
			require.ErrorIs(t, err, ErrSemanticModelUnknown)
			require.Equal(t, 1, models.providerCalls)
			require.Equal(t, 1, invocations.markUnknown)
			require.Zero(t, budget.finishCalls)
			require.Zero(t, budget.releaseCalls)
		})
	}
}

func TestSemanticModelSuccessSettlesOnceAndDeadlineCancellationBecomesUnknown(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		models := &semanticGatewayModels{response: &types.ChatResponse{Content: "ok", Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}}
		invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
		budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
			return domain.Reservation{ID: "hold"}, nil
		})
		var finishedCapability semanticCapability
		var finishedReservation domain.Reservation
		budget.finishFunc = func(_ context.Context, c semanticCapability, reservation domain.Reservation, _, _ int64) error {
			budget.finishCalls++
			finishedCapability, finishedReservation = c, reservation
			return nil
		}
		capability := semanticGatewayCapability()
		capability.Funding = domain.FundingPlatform
		got, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
		require.NoError(t, err)
		require.Equal(t, "ok", got.Text)
		require.Equal(t, 1, models.providerCalls)
		require.Equal(t, 1, budget.finishCalls)
		require.Equal(t, 1, invocations.complete)
		require.Zero(t, invocations.markUnknown)
		require.Equal(t, uint64(7), finishedCapability.owner)
		require.Equal(t, domain.FundingPlatform, finishedCapability.funding)
		require.Equal(t, "pv", finishedCapability.priceVersion)
		require.Equal(t, "hold", finishedReservation.ID)
	})
	t.Run("cancelled after dispatch", func(t *testing.T) {
		capability := semanticGatewayCapability()
		capability.Deadline = time.Now().Add(20 * time.Millisecond)
		models := &semanticGatewayModels{waitForContext: true}
		invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}
		budget := semanticTestBudget(func(context.Context, semanticCapability) (domain.Reservation, error) {
			return domain.Reservation{}, nil
		})
		_, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
		require.ErrorIs(t, err, ErrSemanticModelUnknown)
		require.Equal(t, 1, models.providerCalls)
		require.Equal(t, 1, invocations.markUnknown)
		require.Zero(t, invocations.failBeforeDispatch)
	})
}

// This catches the gateway routing a final BYOK model fact through generic
// usage settlement instead of the owner-scoped raw observation path.
func TestSemanticModelBYOKPersistsOwnerRawUsageWithoutPlatformSettlement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{}))
	rates := func(version string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: version, Rates: map[string]domain.DimensionRate{domain.DimensionModel: {RateMicro: 1000, Units: 1000}}}, nil
	}
	usage := repocommercial.NewUsageStore(db).WithRates(rates)
	budget := NewSemanticModelBudgetAdapter(nil, nil, rates).WithUsageStore(usage)
	models := &semanticGatewayModels{response: &types.ChatResponse{Content: "ok", Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}}
	invocations := &semanticCountingInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationClaimedNew}}

	got, err := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{}, budget, invocations).Invoke(context.Background(), semanticGatewayWire())
	require.NoError(t, err)
	require.Equal(t, "ok", got.Text)
	require.Equal(t, 1, invocations.claimCalls)
	require.Equal(t, 1, models.chatCalls)
	require.Equal(t, 1, models.providerCalls)
	require.Equal(t, 1, invocations.complete)

	var raw repocommercial.UsageRow
	require.NoError(t, db.Where("tenant_id = ? AND call_id = ?", uint64(7), "call").First(&raw).Error)
	require.Equal(t, uint64(7), raw.TenantID)
	require.Equal(t, domain.FundingBYOK, raw.Funding)
	require.Equal(t, domain.ServiceModel, raw.Service)
	require.Equal(t, "pv", raw.PriceVersion)
	require.Zero(t, raw.ChargeMicro)
	var settlements int64
	require.NoError(t, db.Model(&repocommercial.OutboxEvent{}).Where("kind = ?", repocommercial.OutboxKindUsageSettlement).Count(&settlements).Error)
	require.Zero(t, settlements)
}

// This is deliberately not a counting fake: the successful BYOK path writes
// raw observed usage and completes the real invocation ledger. A later call
// in the same owner task must be quota-denied before model/provider I/O.
func TestSemanticModelBYOKRealLedgerPersistsUsageAndBlocksLaterOverQuotaCall(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{}))
	migration, err := os.ReadFile("../../../migrations/sqlite/000101_semantic_model_invocations.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)

	rates := func(version string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: version, Rates: map[string]domain.DimensionRate{domain.DimensionModel: {RateMicro: 1000, Units: 1000}}}, nil
	}
	usage := repocommercial.NewUsageStore(db).WithRates(rates)
	budget := NewSemanticModelBudgetAdapter(nil, nil, rates).WithUsageStore(usage)
	capability := semanticGatewayCapability()
	capability.PolicyVersion = 1
	capability.ExpiresAt = time.Now().Add(time.Hour).UTC()
	capability.MaxCallsPerTask = 1
	capability.MaxInputTokensPerTask = 20
	capability.MaxOutputTokensPerTask = 20
	ledger := repository.NewSemanticModelInvocationStore(db)
	require.NoError(t, ledger.EnsureRun(context.Background(), capability))
	models := &semanticGatewayModels{response: &types.ChatResponse{Content: "ok", Usage: types.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}}}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: capability}, models, semanticGatewayScope{}, budget, ledger)
	_, err = gateway.Invoke(context.Background(), semanticGatewayWire())
	require.NoError(t, err)

	var raw repocommercial.UsageRow
	require.NoError(t, db.Where("tenant_id = ? AND call_id = ?", uint64(7), "call").First(&raw).Error)
	require.Equal(t, int64(5), func() int64 {
		var dims map[string]int64
		require.NoError(t, json.Unmarshal([]byte(raw.DimensionsJSON), &dims))
		return dims[domain.DimensionModel]
	}())
	var invocationCount int64
	require.NoError(t, db.Table("semantic_model_invocations").Where("tenant_id = ? AND state = ?", uint64(7), "completed").Count(&invocationCount).Error)
	require.Equal(t, int64(1), invocationCount)

	later := capability
	later.CallID = "later-over-quota"
	_, err = NewSemanticModelGateway(semanticGatewayIssuer{cap: later}, models, semanticGatewayScope{}, budget, ledger).Invoke(context.Background(), semanticGatewayWire())
	require.Error(t, err)
	require.Equal(t, 1, models.chatCalls)
	require.Equal(t, 1, models.providerCalls)
}

func semanticGatewayCapability() types.SemanticModelCapability {
	return types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb", ScopeRef: "scope", ScopeHash: "hash", ModelID: "model", Funding: "byok", PriceVersion: "pv", RunID: "run", CallID: "call", MaxInputTokensPerCall: 20, MaxOutputTokensPerCall: 20, PerCallUpperMicro: 10, Deadline: time.Now().Add(time.Hour)}
}
func semanticGatewayWire() types.SemanticModelWireRequest {
	return types.SemanticModelWireRequest{CapabilityToken: "ok", Messages: []types.SemanticModelMessage{{Role: "user", Content: "hello"}}, Parameters: types.SemanticModelParameters{MaxOutputTokens: 5}}
}

type semanticGatewayIssuer struct {
	cap types.SemanticModelCapability
	err error
}

func (s semanticGatewayIssuer) Issue(context.Context, string, string) (types.SemanticModelIssuedCapability, error) {
	return types.SemanticModelIssuedCapability{}, errors.New("unused")
}
func (s semanticGatewayIssuer) Verify(context.Context, string) (types.SemanticModelCapability, error) {
	return s.cap, s.err
}

type semanticGatewayScope struct{ err error }

func (s semanticGatewayScope) Resolve(context.Context, string) (SemanticScopeSnapshot, error) {
	if s.err != nil {
		return SemanticScopeSnapshot{}, s.err
	}
	return SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb"}, ScopeHash: "hash"}, nil
}

type semanticGatewayInvocations struct {
	claim types.SemanticModelInvocationClaim
}

type semanticCountingInvocations struct {
	claim                                                                 types.SemanticModelInvocationClaim
	claimErr, markDispatchedErr                                           error
	claimCalls, failBeforeDispatch, markDispatched, markUnknown, complete int
}

func (*semanticCountingInvocations) EnsureRun(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (s *semanticCountingInvocations) Claim(context.Context, types.SemanticModelCapability, string) (types.SemanticModelInvocationClaim, error) {
	s.claimCalls++
	return s.claim, s.claimErr
}
func (s *semanticCountingInvocations) MarkDispatched(context.Context, types.SemanticModelCapability) error {
	s.markDispatched++
	return s.markDispatchedErr
}
func (s *semanticCountingInvocations) Complete(context.Context, types.SemanticModelCapability, types.SemanticModelInvocationResult) error {
	s.complete++
	return nil
}
func (s *semanticCountingInvocations) FailBeforeDispatch(context.Context, types.SemanticModelCapability) error {
	s.failBeforeDispatch++
	return nil
}
func (s *semanticCountingInvocations) MarkUnknown(context.Context, types.SemanticModelCapability) error {
	s.markUnknown++
	return nil
}

func (s semanticGatewayInvocations) EnsureRun(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (s semanticGatewayInvocations) Claim(context.Context, types.SemanticModelCapability, string) (types.SemanticModelInvocationClaim, error) {
	return s.claim, nil
}
func (semanticGatewayInvocations) MarkDispatched(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (semanticGatewayInvocations) Complete(context.Context, types.SemanticModelCapability, types.SemanticModelInvocationResult) error {
	return nil
}
func (semanticGatewayInvocations) FailBeforeDispatch(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (semanticGatewayInvocations) MarkUnknown(context.Context, types.SemanticModelCapability) error {
	return nil
}

type semanticGatewayModels struct {
	chatCalls, providerCalls int
	err                      error
	response                 *types.ChatResponse
	waitForContext           bool
}

func (*semanticGatewayModels) CreateModel(context.Context, *types.Model) error { return nil }
func (*semanticGatewayModels) GetModelByID(context.Context, string) (*types.Model, error) {
	return nil, nil
}
func (*semanticGatewayModels) ListModels(context.Context) ([]*types.Model, error) { return nil, nil }
func (*semanticGatewayModels) UpdateModel(context.Context, *types.Model) error    { return nil }
func (*semanticGatewayModels) DeleteModel(context.Context, string) error          { return nil }
func (*semanticGatewayModels) UpdateModelCredentials(context.Context, string, *string, *string) (*types.Model, error) {
	return nil, nil
}
func (*semanticGatewayModels) ClearModelCredential(context.Context, string, string) error { return nil }
func (*semanticGatewayModels) GetEmbeddingModel(context.Context, string) (embedding.Embedder, error) {
	return nil, nil
}
func (*semanticGatewayModels) GetEmbeddingModelForTenant(context.Context, string, uint64) (embedding.Embedder, error) {
	return nil, nil
}
func (*semanticGatewayModels) GetRerankModel(context.Context, string) (rerank.Reranker, error) {
	return nil, nil
}
func (s *semanticGatewayModels) GetChatModel(context.Context, string) (chat.Chat, error) {
	s.chatCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s, nil
}
func (*semanticGatewayModels) GetVLMModel(context.Context, string) (vlm.VLM, error) { return nil, nil }
func (*semanticGatewayModels) GetASRModel(context.Context, string) (asr.ASR, error) { return nil, nil }
func (s *semanticGatewayModels) Chat(ctx context.Context, _ []chat.Message, _ *chat.ChatOptions) (*types.ChatResponse, error) {
	s.providerCalls++
	if s.waitForContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return s.response, nil
}
func (*semanticGatewayModels) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	return nil, errors.New("unused")
}
func (*semanticGatewayModels) GetModelName() string { return "test" }
func (*semanticGatewayModels) GetModelID() string   { return "test" }

func semanticTestBudget(reserve func(context.Context, semanticCapability) (domain.Reservation, error)) *SemanticModelBudgetAdapter {
	b := &SemanticModelBudgetAdapter{reserveFunc: reserve}
	b.releaseFunc = func(context.Context, semanticCapability, domain.Reservation) error { b.releaseCalls++; return nil }
	b.markDispatchedFunc = func(context.Context, semanticCapability, domain.Reservation) error { return nil }
	b.finishFunc = func(context.Context, semanticCapability, domain.Reservation, int64, int64) error {
		b.finishCalls++
		return nil
	}
	return b
}
