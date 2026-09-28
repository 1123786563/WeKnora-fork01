package container

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"go.uber.org/dig"
)

const defaultAgentSecurityReconcileInterval = 10 * time.Second

type agentSecurityReconciliationStore interface {
	ListPendingRunCancellations(context.Context, int) ([]repository.PendingAgentSecurityCancellation, error)
	ReconcileRunCancellation(context.Context, uint64, string) (int64, error)
}

func agentSecurityReconcileInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("AGENT_SECURITY_RECONCILE_INTERVAL")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
		log.Printf("[AgentSecurityReconciler] invalid AGENT_SECURITY_RECONCILE_INTERVAL=%q; using %s", raw, defaultAgentSecurityReconcileInterval)
	}
	return defaultAgentSecurityReconcileInterval
}

type agentSecurityReconcilerWiring struct {
	dig.In
	Runs    *repository.AgentRunStore
	Cleaner interfaces.ResourceCleaner `optional:"true"`
}

func startAgentSecurityReconciler(in agentSecurityReconcilerWiring) {
	if in.Runs == nil || in.Cleaner == nil {
		return
	}
	StartAgentSecurityReconciler(in.Runs, in.Cleaner)
}

func StartAgentSecurityReconciler(runs agentSecurityReconciliationStore, cleaner interfaces.ResourceCleaner) {
	if runs == nil || cleaner == nil {
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	reconcile := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pending, err := runs.ListPendingRunCancellations(ctx, 100)
		if err != nil {
			log.Printf("[AgentSecurityReconciler] list pending obligations failed: %v", err)
			return
		}
		for _, obligation := range pending {
			if _, err := runs.ReconcileRunCancellation(ctx, obligation.TenantID, obligation.RevocationID); err != nil {
				log.Printf("[AgentSecurityReconciler] reconcile failed tenant=%d revocation=%s: %v", obligation.TenantID, obligation.RevocationID, err)
			}
		}
	}
	go func() {
		defer close(done)
		reconcile()
		ticker := time.NewTicker(agentSecurityReconcileInterval())
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				reconcile()
			}
		}
	}()
	cleaner.RegisterWithName("AgentSecurityReconciler", func() error { once.Do(func() { close(stop) }); <-done; return nil })
}
