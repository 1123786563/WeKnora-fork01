package service

// O04: the usage + execution-diagnostics view of one craft session. This
// service COMPOSES; it never prices. The aggregation mirrors the frontend
// UsageView vocabulary (packages/domain/src/craft/usage.ts) exactly: known
// calls carry tokens, unknown calls stay their own visible line, and money is
// deliberately absent — amounts come only from the commercial view. BYOK
// facts are marked so the UI can state plainly that those model calls are
// borne by the space's own credentials, never rendered as "free".

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// CraftFundingMixed marks a session whose recorded physical calls used BOTH
// funding sources. It extends the commercial vocabulary only at the display
// seam; the ledger itself only ever stores server-recognized funding.
const CraftFundingMixed = "mixed"

// CraftSessionUsageRun is one durable main run of the session, with the
// honest failure surface the run table keeps: status plus its recorded wait
// reason (empty when none was recorded — never a fabricated explanation).
type CraftSessionUsageRun struct {
	RunID         string
	Status        string
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CraftSessionUsageCall is one physical model attempt exactly as the O01
// ledger records it: which runtime made it (main agent or OC child), under
// which run and delegation, with which server-bound model and funding.
// status "unknown" means the usage could not be observed — no numbers.
type CraftSessionUsageCall struct {
	CallID       string
	AttemptID    string
	RunID        string
	Runtime      string
	DelegationID string
	ModelID      string
	Funding      string
	Status       string
	Input        int64
	Output       int64
	Cached       int64
}

// CraftSessionResidency is the session's sandbox residency and storage
// accounting folded from the O03 lifecycle events.
type CraftSessionResidency struct {
	Starts          int
	Stops           int
	OpenStarts      int
	DwellSeconds    float64
	StorageBytesDay float64
}

// CraftSessionVersionCheck is one verification check of the session's
// current version (the preview origin check W02 owns).
type CraftSessionVersionCheck struct {
	VersionID string
	Name      string
	Status    string
	Detail    string
}

// CraftSessionUsageView is the GET /sessions/:session_id/craft/usage body
// source. AsOf is the moment the ledger was read — a late usage correction
// arrives as a new revision and a refreshed read moves AsOf forward.
type CraftSessionUsageView struct {
	AsOf                  time.Time
	KnownCalls            int
	UnknownCalls          int
	InputTokens           int64
	OutputTokens          int64
	CachedTokens          int64
	Funding               string
	ByokModelBorneBySpace bool
	Runs                  []CraftSessionUsageRun
	Calls                 []CraftSessionUsageCall
	Residency             *CraftSessionResidency
	Checks                []CraftSessionVersionCheck
}

// CraftSessionResidencySource is the lifecycle half of the view: the O03
// *CraftLifecycle satisfies it through SessionLifecycleUsage.
type CraftSessionResidencySource interface {
	SessionLifecycleUsage(ctx context.Context, tenantID uint64, sessionID string, since time.Time) (CraftLifecycleUsage, error)
}

// CraftUsageViewConfig assembles the usage view service. Usage, Residency
// and Versions are optional: a nil port reports its section as empty instead
// of failing the whole view — the usage endpoint must stay readable even
// while a specialty is not assembled.
type CraftUsageViewConfig struct {
	DB        *gorm.DB
	Sessions  interfaces.SessionService
	Usage     *CraftUsageService
	Residency CraftSessionResidencySource
	Versions  craft.VersionStore
	Now       func() time.Time
}

// CraftUsageViewService composes the O04 usage view.
type CraftUsageViewService struct {
	db        *gorm.DB
	sessions  interfaces.SessionService
	usage     *CraftUsageService
	residency CraftSessionResidencySource
	versions  craft.VersionStore
	now       func() time.Time
}

// NewCraftUsageViewService validates the assembly.
func NewCraftUsageViewService(cfg CraftUsageViewConfig) (*CraftUsageViewService, error) {
	if cfg.DB == nil || cfg.Sessions == nil {
		return nil, errors.New("craft: usage view requires db and sessions")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CraftUsageViewService{
		db: cfg.DB, sessions: cfg.Sessions, usage: cfg.Usage,
		residency: cfg.Residency, versions: cfg.Versions, now: now,
	}, nil
}

// SessionUsage composes the view. The read follows the existing session
// read ACL (owner scope with the service's own sharing rules; a session of
// another tenant answers ErrNotFound — it does not exist here), exactly like
// the W03 workspace GET.
func (s *CraftUsageViewService) SessionUsage(ctx context.Context, scope craft.Scope) (CraftSessionUsageView, error) {
	if s == nil {
		return CraftSessionUsageView{}, fmt.Errorf("%w: usage view service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return CraftSessionUsageView{}, fmt.Errorf("%w: incomplete craft scope", craft.ErrInvalidInput)
	}
	session, err := s.sessions.GetSession(ctx, scope.SessionID)
	if err != nil || session == nil {
		return CraftSessionUsageView{}, craft.ErrNotFound
	}
	if session.TenantID != scope.TenantID {
		return CraftSessionUsageView{}, craft.ErrNotFound
	}

	view := CraftSessionUsageView{AsOf: s.now(), Funding: commercial.FundingPlatform}

	// The session's main runs: status and the recorded failure surface.
	view.Runs, err = s.sessionRuns(ctx, scope.TenantID, scope.SessionID)
	if err != nil {
		return CraftSessionUsageView{}, err
	}

	// The physical-call ledger (main + OC child runtimes side by side).
	if s.usage != nil {
		facts, ferr := s.usage.SessionFacts(ctx, scope.TenantID, scope.SessionID)
		if ferr != nil {
			return CraftSessionUsageView{}, ferr
		}
		hasPlatform, hasByok := false, false
		view.Calls = make([]CraftSessionUsageCall, 0, len(facts))
		for _, f := range facts {
			if f.Status == craft.UsageStatusUnknown {
				view.UnknownCalls++
			} else {
				view.KnownCalls++
				view.InputTokens += f.Input
				view.OutputTokens += f.Output
				view.CachedTokens += f.Cached
			}
			switch f.Funding {
			case commercial.FundingPlatform:
				hasPlatform = true
			case commercial.FundingBYOK:
				hasByok = true
			}
			view.Calls = append(view.Calls, CraftSessionUsageCall{
				CallID: f.CallID, AttemptID: f.AttemptID, RunID: f.RunID,
				Runtime: f.Runtime, DelegationID: f.DelegationID,
				ModelID: f.ModelID, Funding: f.Funding, Status: f.Status,
				Input: f.Input, Output: f.Output, Cached: f.Cached,
			})
		}
		view.ByokModelBorneBySpace = hasByok
		switch {
		case hasPlatform && hasByok:
			view.Funding = CraftFundingMixed
		case hasByok:
			view.Funding = commercial.FundingBYOK
		default:
			// Platform-only, or an empty ledger (the counts are zero then —
			// the label renders the empty state, so the value carries no
			// claim).
			view.Funding = commercial.FundingPlatform
		}
	}

	// Sandbox residency from the O03 lifecycle events.
	if s.residency != nil {
		usage, rerr := s.residency.SessionLifecycleUsage(ctx, scope.TenantID, scope.SessionID, time.Time{})
		if rerr != nil {
			return CraftSessionUsageView{}, rerr
		}
		view.Residency = &CraftSessionResidency{
			Starts: usage.Starts, Stops: usage.Stops, OpenStarts: usage.OpenStarts,
			DwellSeconds: usage.Dwell.Seconds(), StorageBytesDay: usage.StorageBytesDay,
		}
	}

	// The current version's verification checks.
	if s.versions != nil {
		owner := ownerScopeOf(session)
		if versions, verr := s.versions.List(ctx, owner); verr == nil && len(versions) > 0 {
			current := versions[0]
			view.Checks = make([]CraftSessionVersionCheck, 0, len(current.Checks))
			for _, c := range current.Checks {
				view.Checks = append(view.Checks, CraftSessionVersionCheck{
					VersionID: current.ID, Name: c.Name, Status: c.Status, Detail: c.Detail,
				})
			}
		}
	}
	return view, nil
}

// sessionRuns lists the session's durable runs, oldest first.
func (s *CraftUsageViewService) sessionRuns(ctx context.Context, tenantID uint64, sessionID string) ([]CraftSessionUsageRun, error) {
	var rows []struct {
		RunID      string
		Status     string
		WaitReason string
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}
	err := s.db.WithContext(ctx).Table("agent_runs").
		Select("run_id, status, wait_reason, created_at, updated_at").
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Order("created_at ASC, run_id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	runs := make([]CraftSessionUsageRun, 0, len(rows))
	for _, r := range rows {
		failure := ""
		if r.Status == "failed" {
			failure = r.WaitReason
		}
		runs = append(runs, CraftSessionUsageRun{
			RunID: r.RunID, Status: r.Status, FailureReason: failure,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return runs, nil
}
