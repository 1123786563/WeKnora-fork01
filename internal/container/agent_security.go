package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
)

func NewAgentSecurityService(store *repository.AgentSecurityStore, runs *repository.AgentRunStore) *service.AgentSecurityService {
	return service.NewAgentSecurityService(store, runs)
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
