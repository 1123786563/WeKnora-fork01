package container

import (
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// newSessionForkService builds the session-fork service from the repositories.
// The sandbox snapshot port stays nil until the git-checkpoint infrastructure
// (A11 phase 3) is wired; per the upstream contract a nil port makes every
// sandbox-carrying fork degrade to a fresh sandbox instead of failing, while
// message-only forks copy history and lineage unchanged.
func newSessionForkService(
	sessions interfaces.SessionRepository,
	messages interfaces.MessageRepository,
) *service.SessionForkService {
	return service.NewSessionForkServiceFromRepos(sessions, messages, nil)
}
