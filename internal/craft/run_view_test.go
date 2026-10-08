package craft

import (
	"errors"
	"testing"
	"time"
)

func TestValidateRunViewKeyRequiresExactServerRunIdentity(t *testing.T) {
	valid := RunViewKey{TenantID: 1, OwnerID: "owner-1", SessionID: "session-1", RunID: "run-1"}
	if err := ValidateRunViewKey(valid); err != nil {
		t.Fatalf("valid key rejected: %v", err)
	}
	for name, key := range map[string]RunViewKey{
		"tenant":       {OwnerID: valid.OwnerID, SessionID: valid.SessionID, RunID: valid.RunID},
		"owner":        {TenantID: valid.TenantID, SessionID: valid.SessionID, RunID: valid.RunID},
		"session":      {TenantID: valid.TenantID, OwnerID: valid.OwnerID, RunID: valid.RunID},
		"run":          {TenantID: valid.TenantID, OwnerID: valid.OwnerID, SessionID: valid.SessionID},
		"pathlike run": {TenantID: valid.TenantID, OwnerID: valid.OwnerID, SessionID: valid.SessionID, RunID: "../run"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRunViewKey(key); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ValidateRunViewKey(%+v) error = %v, want ErrInvalidInput", key, err)
			}
		})
	}
}

func TestValidateRunViewDoesNotConfusePendingWithBoundRuntime(t *testing.T) {
	key := RunViewKey{TenantID: 1, OwnerID: "owner-1", SessionID: "session-1", RunID: "run-1"}
	base := RunView{Key: key, Generation: "rv_opaque", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	base.State = RunViewStateAllocating
	if err := ValidateRunView(base); err != nil {
		t.Fatalf("unresolved allocation rejected: %v", err)
	}
	base.Runtime = RunViewRuntime{RuntimeID: "runtime-1", ContainerID: "container-1", OpenCodeSessionID: "oc-session-1"}
	if err := ValidateRunView(base); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("partial allocation identity accepted while allocating: %v", err)
	}
	base.State = RunViewStateBound
	intentAt := time.Now()
	base.SessionCreateIntentAt = &intentAt
	if err := ValidateRunView(base); err != nil {
		t.Fatalf("complete runtime binding rejected: %v", err)
	}
	base.Runtime.ContainerID = "/var/run/docker.sock"
	if err := ValidateRunView(base); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("path-like runtime identity accepted: %v", err)
	}
	base.Runtime = RunViewRuntime{}
	base.State = RunViewState("unknown")
	if err := ValidateRunView(base); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown state accepted: %v", err)
	}
}

func TestValidateRunViewRequiresCreateIntentForBoundRuntime(t *testing.T) {
	view := RunView{
		Key:        RunViewKey{TenantID: 1, OwnerID: "owner-1", SessionID: "session-1", RunID: "run-1"},
		Generation: "rv_opaque",
		Runtime: RunViewRuntime{
			RuntimeID: "runtime-1", ContainerID: "container-1", OpenCodeSessionID: "oc-session-1",
		},
		State: RunViewStateBound,
	}
	if err := ValidateRunView(view); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bound runtime without durable create intent accepted: %v", err)
	}

	zero := time.Time{}
	view.SessionCreateIntentAt = &zero
	if err := ValidateRunView(view); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bound runtime with zero create intent accepted: %v", err)
	}

	intentAt := time.Now()
	view.SessionCreateIntentAt = &intentAt
	if err := ValidateRunView(view); err != nil {
		t.Fatalf("bound runtime with durable create intent rejected: %v", err)
	}
}
