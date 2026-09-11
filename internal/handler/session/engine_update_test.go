package session

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestEngineUpdateValidation(t *testing.T) {
	cases := []struct {
		name    string
		current types.AgentEngineType
		next    types.AgentEngineType
		wantErr bool
	}{
		{
			name:    "builtin to trpc rejected",
			current: types.AgentEngineBuiltin,
			next:    types.AgentEngineTRPC,
			wantErr: true,
		},
		{
			name:    "trpc to builtin rejected",
			current: types.AgentEngineTRPC,
			next:    types.AgentEngineBuiltin,
			wantErr: true,
		},
		{
			name:    "builtin to builtin allowed",
			current: types.AgentEngineBuiltin,
			next:    types.AgentEngineBuiltin,
			wantErr: false,
		},
		{
			name:    "trpc to trpc allowed",
			current: types.AgentEngineTRPC,
			next:    types.AgentEngineTRPC,
			wantErr: false,
		},
		{
			name:    "empty next preserves builtin",
			current: types.AgentEngineBuiltin,
			next:    "",
			wantErr: false,
		},
		{
			name:    "empty next preserves trpc",
			current: types.AgentEngineTRPC,
			next:    "",
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEngineUpdate(tc.current, tc.next)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateEngineUpdate(%q, %q) = nil, want error", tc.current, tc.next)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateEngineUpdate(%q, %q) = %v, want nil", tc.current, tc.next, err)
			}
		})
	}
}
