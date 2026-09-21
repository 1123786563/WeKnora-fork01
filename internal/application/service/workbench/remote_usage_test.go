package workbench

import (
	"context"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"testing"
	"time"
)

type remoteUsageGateStub struct {
	begins   int
	finishes int
	last     commercial.UsageFact
}

type remoteBudgetStub struct {
	child, parent string
}

func (b *remoteBudgetStub) AttachChildRun(_ context.Context, tenant uint64, child, parent string) error {
	if tenant == 0 {
		return context.Canceled
	}
	b.child, b.parent = child, parent
	return nil
}

func (g *remoteUsageGateStub) Begin(_ context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	g.begins++
	return commercial.Reservation{ID: req.Key}, nil
}
func (g *remoteUsageGateStub) Finish(_ context.Context, _ string, f commercial.UsageFact) error {
	g.finishes++
	g.last = f
	return nil
}
func remoteReq() RemoteUsageRequest {
	return RemoteUsageRequest{TenantID: 1, RunID: "run", CallID: "call", AttemptID: "attempt", Upper: 10, Deadline: time.Now().Add(time.Minute), Source: "platform_gateway", Funding: commercial.FundingPlatform, Service: commercial.ServiceModel, PriceVersion: "p1", Revision: 1, OccurredAt: time.Now(), Dimensions: map[string]int64{commercial.DimensionModel: 1}, Status: commercial.UsageStatusFinal}
}
func TestRemoteUsageRejectsSelfReportedAndBYOKModel(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, _ := NewRemoteUsageService(g)
	r := remoteReq()
	r.Source = "personal_node"
	if _, err := s.beginBound(context.Background(), r); err == nil {
		t.Fatal("personal node must not reserve")
	}
	r = remoteReq()
	r.Funding = commercial.FundingBYOK
	if _, err := s.beginBound(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if g.begins != 0 {
		t.Fatalf("BYOK model reserved %d times", g.begins)
	}
}
func TestRemoteUsageFinishTrustAndIdempotentIdentity(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, _ := NewRemoteUsageService(g)
	r := remoteReq()
	if _, err := s.beginBound(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := s.finishBound(context.Background(), stableUsageKey(r), r); err != nil {
		t.Fatal(err)
	}
	if g.finishes != 1 || g.last.CallID != "call" {
		t.Fatalf("finish=%d fact=%+v", g.finishes, g.last)
	}
	r.Source = "personal_node"
	if err := s.finishBound(context.Background(), stableUsageKey(r), r); err == nil {
		t.Fatal("untrusted finish accepted")
	}
}
func TestRemoteUsageDoesNotSettleUnknownOrDisplayOnly(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, _ := NewRemoteUsageService(g)
	for _, status := range []string{commercial.UsageStatusUnknown, commercial.UsageStatusDisplayOnly, commercial.UsageStatusPartial} {
		r := remoteReq()
		r.Status = status
		if err := s.finishBound(context.Background(), stableUsageKey(r), r); err == nil {
			t.Fatalf("status %q accepted", status)
		}
	}
	if g.finishes != 0 {
		t.Fatal("non-billable observation reached gate")
	}
}

func TestRemoteUsagePublicRequestCannotSelfAuthorize(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, _ := NewRemoteUsageService(g)
	r := remoteReq()
	if _, err := s.Begin(context.Background(), r); err == nil {
		t.Fatal("caller-controlled source reached the gate")
	}
	if err := s.Finish(context.Background(), stableUsageKey(r), r); err == nil {
		t.Fatal("caller-controlled finish reached the gate")
	}
	if g.begins != 0 || g.finishes != 0 {
		t.Fatalf("untrusted request invoked gate: begins=%d finishes=%d", g.begins, g.finishes)
	}
}

func TestRemoteUsageDerivesStableKeyAndAttachesChildBeforeReserve(t *testing.T) {
	g := &remoteUsageGateStub{}
	b := &remoteBudgetStub{}
	s, _ := NewRemoteUsageService(g)
	s.WithBudgetTree(b)
	r := remoteReq()
	r.RunID, r.ParentRunID = "child", "root"
	if _, err := s.beginBound(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if b.child != "child" || b.parent != "root" {
		t.Fatalf("child budget was not attached: %+v", b)
	}
	r.Key = "forged"
	if _, err := s.beginBound(context.Background(), r); err == nil {
		t.Fatal("forged idempotency key accepted")
	}
}

func TestRemoteUsageBYOKFinishWithoutReservationIsNoop(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, _ := NewRemoteUsageService(g)
	r := remoteReq()
	r.Funding = commercial.FundingBYOK
	if err := s.finishBound(context.Background(), "", r); err != nil {
		t.Fatal(err)
	}
	if g.finishes != 0 {
		t.Fatal("BYOK model finish reached platform gate")
	}
}

func TestRemoteUsageConstructorsFailClosed(t *testing.T) {
	if _, err := NewRemoteUsageService(nil); err == nil {
		t.Fatal("nil gate must fail closed")
	}
	g := &remoteUsageGateStub{}
	if _, err := NewRemoteUsageServiceWithDB(g, nil); err == nil {
		t.Fatal("nil budget database must fail closed")
	}
}

func TestRemoteUsageProviderCannotOverrideFenceRevisionOrDimensions(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, err := NewRemoteUsageService(g)
	if err != nil {
		t.Fatal(err)
	}
	fence := agentruntime.Fence{
		RunKey:      agentruntime.RunKey{TenantID: 1, RunID: "run"},
		UsageSource: "platform_gateway", UsageFunding: commercial.FundingPlatform,
		UsageService: commercial.ServiceConnector, UsagePriceVersion: "pv-1",
		UsageUpper: 1000, UsageRevision: 2, UsageStatus: commercial.UsageStatusFinal,
		UsageDimensions: map[string]int64{commercial.DimensionConnector: 4},
	}
	h, err := s.BeginRemote(context.Background(), fence, "call-1")
	if err != nil {
		t.Fatal(err)
	}
	bad := &agentruntime.RemoteUsageObservation{Service: commercial.ServiceConnector, PriceVersion: "pv-1", Revision: 3, Status: commercial.UsageStatusFinal, Dimensions: map[string]int64{commercial.DimensionConnector: 1}, OccurredAt: time.Now().UTC()}
	if err := s.FinishRemoteObservation(context.Background(), h, bad); err == nil {
		t.Fatal("provider override accepted")
	}
	if g.finishes != 0 {
		t.Fatal("invalid provider observation reached gate")
	}
}

func TestRemoteUsageUnknownPartialDisplayOnlyThenLateFinal(t *testing.T) {
	g := &remoteUsageGateStub{}
	s, err := NewRemoteUsageService(g)
	if err != nil {
		t.Fatal(err)
	}
	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run"}, UsageSource: "platform_gateway", UsageFunding: commercial.FundingPlatform, UsageService: commercial.ServiceConnector, UsagePriceVersion: "pv-1", UsageUpper: 1000, UsageRevision: 1, UsageDimensions: map[string]int64{commercial.DimensionConnector: 1}, UsageStatus: commercial.UsageStatusFinal}
	h, err := s.BeginRemote(context.Background(), fence, "late-call")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{commercial.UsageStatusUnknown, commercial.UsageStatusPartial, commercial.UsageStatusDisplayOnly} {
		if err := s.FinishRemoteObservation(context.Background(), h, &agentruntime.RemoteUsageObservation{Status: status}); err == nil {
			t.Fatalf("status %q settled", status)
		}
	}
	final := &agentruntime.RemoteUsageObservation{Service: commercial.ServiceConnector, PriceVersion: "pv-1", Revision: 1, Status: commercial.UsageStatusFinal, Dimensions: map[string]int64{commercial.DimensionConnector: 1}, OccurredAt: time.Now().UTC()}
	if err := s.FinishRemoteObservation(context.Background(), h, final); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRemoteObservation(context.Background(), h, final); err != nil {
		t.Fatal(err)
	}
	if g.finishes != 2 {
		t.Fatalf("late final finish calls=%d", g.finishes)
	}
}
