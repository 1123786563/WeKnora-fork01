package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"go.uber.org/dig"
)

func NewAgentSecurityService(store *repository.AgentSecurityStore, runs *repository.AgentRunStore) *service.AgentSecurityService {
	return service.NewAgentSecurityService(store, runs)
}

type agentSecuritySessionWiring struct {
	dig.In
	Handler  *sessionhandler.Handler                  `optional:"true"`
	Claims   *repository.AgentChatTurnClaimRepository `optional:"true"`
	Versions interfaces.AgentVersionService           `optional:"true"`
	Security *service.AgentSecurityService            `optional:"true"`
}

func wireAgentSecuritySessionClaims(in agentSecuritySessionWiring) {
	if in.Handler == nil {
		return
	}
	in.Handler.SetAgentChatTurnClaimStore(in.Claims)
	in.Handler.SetAgentVersionService(in.Versions)
	if in.Security != nil {
		in.Security.SetAgentVersionService(in.Versions)
	}
}

func NewAgentSecurityHandler(security *service.AgentSecurityService) *handler.AgentSecurityHandler {
	return handler.NewAgentSecurityHandler(security)
}

func wireAgentSecurityGates(adoption *service.AgentAdoptionService, upgrade *service.AgentUpgradeService, security *service.AgentSecurityService) {
	if security == nil {
		return
	}
	var gate service.ReleaseSecurityGate = security
	if adoption != nil {
		adoption.SetReleaseSecurityGate(gate)
	}
	if upgrade != nil {
		upgrade.SetReleaseSecurityGate(gate)
	}
}
