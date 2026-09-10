package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

type workerStore struct {
	mu     sync.Mutex
	runs   map[string]agentruntime.Run
	keys   []agentruntime.RunKey
	claims int
}

func (s *workerStore) Admit(_ context.Context, in agentruntime.Admission) (agentruntime.Run, error) {
	r := agentruntime.Run{Key: in.Key, Status: "queued", Snapshot: append(json.RawMessage(nil), in.Snapshot...)}
	s.runs[in.Key.RunID] = r
	return r, nil
}
func (s *workerStore) Get(_ context.Context, k agentruntime.RunKey) (agentruntime.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[k.RunID]
	if !ok {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return r, nil
}
func (s *workerStore) Claim(_ context.Context, k agentruntime.RunKey, o string, _ time.Duration) (agentruntime.Fence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.runs[k.RunID]
	if !ok {
		return agentruntime.Fence{}, agentruntime.ErrNotFound
	}
	if r.Status != "queued" && r.Status != "recovering" {
		return agentruntime.Fence{}, agentruntime.ErrLeaseLost
	}
	r.Status = "running"
	r.Owner = o
	r.Epoch++
	s.runs[k.RunID] = r
	s.claims++
	return agentruntime.Fence{RunKey: k, Owner: o, Epoch: r.Epoch}, nil
}
func (s *workerStore) Renew(context.Context, agentruntime.Fence, time.Duration) error { return nil }
func (s *workerStore) Scan(context.Context, int) ([]agentruntime.RunKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agentruntime.RunKey(nil), s.keys...), nil
}
func (s *workerStore) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}
func (s *workerStore) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
}

func TestWorkerConfigRejectsUnsafeLease(t *testing.T) {
	require.Error(t, (WorkerConfig{Lease: time.Minute, Heartbeat: time.Minute, ScanInterval: time.Second, MaxWorkers: 1}).Validate())
}
func TestWorkerSkipsWaitingUser(t *testing.T) {
	s := &workerStore{runs: map[string]agentruntime.Run{"r": {Key: agentruntime.RunKey{TenantID: 1, RunID: "r"}, Status: "waiting_user"}}, keys: []agentruntime.RunKey{{TenantID: 1, RunID: "r"}}}
	n := 0
	w, e := NewAgentRunWorker(s, func(context.Context, agentruntime.Fence) error { n++; return nil }, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: time.Second, ScanInterval: time.Second, MaxWorkers: 1})
	require.NoError(t, e)
	require.NoError(t, w.Tick(context.Background()))
	time.Sleep(20 * time.Millisecond)
	require.Zero(t, n)
	require.Zero(t, s.claims)
}
func TestWorkerTwoTicksDoNotDuplicate(t *testing.T) {
	s := &workerStore{runs: map[string]agentruntime.Run{"r": {Key: agentruntime.RunKey{TenantID: 1, RunID: "r"}, Status: "queued"}}, keys: []agentruntime.RunKey{{TenantID: 1, RunID: "r"}}}
	entered := make(chan struct{})
	release := make(chan struct{})
	w, e := NewAgentRunWorker(s, func(context.Context, agentruntime.Fence) error { close(entered); <-release; return nil }, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: time.Second, ScanInterval: time.Second, MaxWorkers: 1})
	require.NoError(t, e)
	require.NoError(t, w.Tick(context.Background()))
	<-entered
	require.NoError(t, w.Tick(context.Background()))
	require.Equal(t, 1, s.claims)
	close(release)
}
func TestSubmitCopiesSnapshot(t *testing.T) {
	s := &workerStore{runs: map[string]agentruntime.Run{}}
	svc := NewAgentRunService(s)
	raw := json.RawMessage(`{"version":1}`)
	in := agentruntime.Admission{Key: agentruntime.RunKey{TenantID: 1, RunID: "r"}, Snapshot: raw, UserMessage: json.RawMessage(`{"role":"user"}`), AssistantMessage: json.RawMessage(`{"role":"assistant"}`)}
	_, err := svc.Submit(context.Background(), in)
	require.NoError(t, err)
	raw[2] = 'x'
	require.Equal(t, byte('v'), s.runs["r"].Snapshot[2])
}

func (s *workerStore) SetStatus(context.Context, agentruntime.Fence, string, string) error {
	return nil
}
