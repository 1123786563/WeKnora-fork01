package types

import "fmt"

// AgentEngineType selects the execution engine fixed to an agent session.
type AgentEngineType string

const (
	// AgentEngineBuiltin selects WeKnora's existing ReAct engine.
	AgentEngineBuiltin AgentEngineType = "builtin"
	// AgentEngineTRPC selects the tRPC-Agent-Go engine.
	AgentEngineTRPC AgentEngineType = "trpc"
)

// ParseAgentEngine validates a wire value and applies the backwards-compatible
// built-in default used by sessions created before engine selection existed.
func ParseAgentEngine(raw string) (AgentEngineType, error) {
	switch raw {
	case "", string(AgentEngineBuiltin):
		return AgentEngineBuiltin, nil
	case string(AgentEngineTRPC):
		return AgentEngineTRPC, nil
	default:
		return "", fmt.Errorf("unsupported agent engine: %q", raw)
	}
}
