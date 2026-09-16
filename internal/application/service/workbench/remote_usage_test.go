package workbench

import (
	"context"
	"github.com/Tencent/WeKnora/internal/commercial"
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

func (g *remoteUsageGateStub) Begin(context.Context, commercial.BudgetRequest) (commercial.Reservation, error) {
	g.begins++
	return commercial.Reservation{ID: "r1"}, nil
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
