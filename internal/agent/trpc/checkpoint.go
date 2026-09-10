package trpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const checkpointSDKVersion = "v1.10.0"

type checkpointSaver struct {
	store agentruntime.CheckpointStore
	fence agentruntime.Fence
}

type checkpointEnvelope struct {
	Version      int                   `json:"version"`
	GraphVersion string                `json:"graph_version"`
	SDKVersion   string                `json:"sdk_version"`
	Tuple        graph.CheckpointTuple `json:"tuple"`
	NewVersions  map[string]int64      `json:"new_versions,omitempty"`
	TaskPaths    map[string]string     `json:"task_paths,omitempty"`
}

// CheckpointNamespace isolates each tenant/run and pins the graph topology.
func CheckpointNamespace(key agentruntime.RunKey) string {
	return fmt.Sprintf("tenant/%d/run/%s/graph/%s", key.TenantID, key.RunID, GraphVersion)
}

// CheckpointConfig builds the exact SDK configurable scope required by the saver.
func CheckpointConfig(key agentruntime.RunKey, id string) map[string]any {
	return graph.CreateCheckpointConfig(key.RunID, id, CheckpointNamespace(key))
}

// NewCheckpointSaver implements the pinned SDK interface on the business store.
// It owns no database connection; Close never closes the shared RunStore.
func NewCheckpointSaver(store agentruntime.RunStore, fence agentruntime.Fence) graph.CheckpointSaver {
	extended, _ := store.(agentruntime.CheckpointStore)
	return &checkpointSaver{store: extended, fence: fence}
}

func (s *checkpointSaver) scope(config map[string]any) (string, error) {
	if s.store == nil {
		return "", fmt.Errorf("RunStore requires durable CheckpointStore operations")
	}
	if s.fence.TenantID == 0 || s.fence.RunID == "" {
		return "", fmt.Errorf("invalid checkpoint run identity")
	}
	values, ok := config[graph.CfgKeyConfigurable].(map[string]any)
	if !ok {
		return "", fmt.Errorf("missing checkpoint configurable scope")
	}
	if values[graph.CfgKeyLineageID] != s.fence.RunID ||
		values[graph.CfgKeyCheckpointNS] != CheckpointNamespace(s.fence.RunKey) {
		return "", fmt.Errorf("checkpoint lineage/namespace or graph version mismatch")
	}
	id, _ := values[graph.CfgKeyCheckpointID].(string)
	return id, nil
}

func (s *checkpointSaver) Get(ctx context.Context, config map[string]any) (*graph.Checkpoint, error) {
	tuple, err := s.GetTuple(ctx, config)
	if err != nil || tuple == nil {
		return nil, err
	}
	return tuple.Checkpoint, nil
}

func (s *checkpointSaver) GetTuple(ctx context.Context, config map[string]any) (*graph.CheckpointTuple, error) {
	id, err := s.scope(config)
	if err != nil {
		return nil, err
	}
	records, err := s.store.ListCheckpoints(ctx, s.fence.RunKey, CheckpointNamespace(s.fence.RunKey))
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if id != "" && record.ID != id {
			continue
		}
		envelope, err := s.decode(record)
		if err != nil {
			return nil, err
		}
		if err := s.validateLogs(ctx, envelope.Tuple.Checkpoint); err != nil {
			return nil, err
		}
		return &envelope.Tuple, nil
	}
	return nil, nil
}

func (s *checkpointSaver) List(ctx context.Context, config map[string]any, filter *graph.CheckpointFilter) (
	[]*graph.CheckpointTuple, error,
) {
	if _, err := s.scope(config); err != nil {
		return nil, err
	}
	records, err := s.store.ListCheckpoints(ctx, s.fence.RunKey, CheckpointNamespace(s.fence.RunKey))
	if err != nil {
		return nil, err
	}
	beforeID := ""
	if filter != nil && filter.Before != nil {
		beforeID, err = s.scope(filter.Before)
		if err != nil {
			return nil, err
		}
	}
	if beforeID != "" {
		found := false
		for i, record := range records {
			if record.ID == beforeID {
				records = records[i+1:]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("before checkpoint not found")
		}
	}
	var result []*graph.CheckpointTuple
	for _, record := range records {
		envelope, err := s.decode(record)
		if err != nil {
			return nil, err
		}
		if filter != nil && !matchesMetadata(envelope.Tuple.Metadata, filter.Metadata) {
			continue
		}
		if err := s.validateLogs(ctx, envelope.Tuple.Checkpoint); err != nil {
			return nil, err
		}
		result = append(result, &envelope.Tuple)
		if filter != nil && filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func matchesMetadata(metadata *graph.CheckpointMetadata, filter map[string]any) bool {
	if len(filter) == 0 {
		return true
	}
	if metadata == nil {
		return false
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return false
	}
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for key, want := range filter {
		got, ok := values[key]
		if !ok {
			got = metadata.Extra[key]
		}
		wantJSON, e1 := json.Marshal(want)
		gotJSON, e2 := json.Marshal(got)
		if e1 != nil || e2 != nil || !bytes.Equal(wantJSON, gotJSON) {
			return false
		}
	}
	return true
}

func (s *checkpointSaver) Put(ctx context.Context, req graph.PutRequest) (map[string]any, error) {
	return s.put(ctx, graph.PutFullRequest{
		Config: req.Config, Checkpoint: req.Checkpoint,
		Metadata: req.Metadata, NewVersions: req.NewVersions,
	}, false)
}

func (s *checkpointSaver) PutFull(ctx context.Context, req graph.PutFullRequest) (map[string]any, error) {
	return s.put(ctx, req, true)
}

func (s *checkpointSaver) put(ctx context.Context, req graph.PutFullRequest,
	replaceWrites bool,
) (map[string]any, error) {
	parent, err := s.scope(req.Config)
	if err != nil {
		return nil, err
	}
	if req.Checkpoint == nil || req.Checkpoint.ID == "" {
		return nil, fmt.Errorf("missing checkpoint")
	}
	if err := validateGraphCheckpoint(req.Checkpoint); err != nil {
		return nil, err
	}
	config := CheckpointConfig(s.fence.RunKey, req.Checkpoint.ID)
	envelope := checkpointEnvelope{
		Version: 1, GraphVersion: GraphVersion, SDKVersion: checkpointSDKVersion,
		NewVersions: req.NewVersions,
		Tuple:       graph.CheckpointTuple{Config: config, Checkpoint: req.Checkpoint, Metadata: req.Metadata},
	}
	if req.Checkpoint.ParentCheckpointID != "" {
		parent = req.Checkpoint.ParentCheckpointID
	}
	if parent == req.Checkpoint.ID {
		parent = ""
	}
	if parent != "" {
		envelope.Tuple.ParentConfig = CheckpointConfig(s.fence.RunKey, parent)
	}
	err = s.store.UpdateCheckpoint(ctx, s.fence, CheckpointNamespace(s.fence.RunKey), req.Checkpoint.ID,
		func(current *agentruntime.CheckpointRecord, nextSeq int64) (*agentruntime.CheckpointRecord, error) {
			writes := req.PendingWrites
			if current != nil {
				nextSeq = current.Seq
				old, err := s.decode(*current)
				if err != nil {
					return nil, err
				}
				envelope.TaskPaths = old.TaskPaths
				if !replaceWrites {
					writes = old.Tuple.PendingWrites
				}
			}
			return encodeCheckpoint(envelope, writes, nextSeq, parent)
		})
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (s *checkpointSaver) PutWrites(ctx context.Context, req graph.PutWritesRequest) error {
	id, err := s.scope(req.Config)
	if err != nil {
		return err
	}
	if id == "" || req.TaskID == "" {
		return fmt.Errorf("pending writes require checkpoint and task IDs")
	}
	return s.store.UpdateCheckpoint(ctx, s.fence, CheckpointNamespace(s.fence.RunKey), id,
		func(current *agentruntime.CheckpointRecord, _ int64) (*agentruntime.CheckpointRecord, error) {
			if current == nil {
				return nil, agentruntime.ErrNotFound
			}
			envelope, err := s.decode(*current)
			if err != nil {
				return nil, err
			}
			writes := envelope.Tuple.PendingWrites
			for i, write := range req.Writes {
				write.TaskID = req.TaskID
				if write.Sequence == 0 {
					write.Sequence = int64(i)
				}
				found := false
				for j, old := range writes {
					if old.TaskID == write.TaskID && old.Sequence == write.Sequence && old.Channel == write.Channel {
						writes[j] = write
						found = true
						break
					}
				}
				if !found {
					writes = append(writes, write)
				}
			}
			sort.SliceStable(writes, func(i, j int) bool { return writes[i].Sequence < writes[j].Sequence })
			if envelope.TaskPaths == nil {
				envelope.TaskPaths = make(map[string]string)
			}
			envelope.TaskPaths[req.TaskID] = req.TaskPath
			return encodeCheckpoint(*envelope, writes, current.Seq, current.ParentID)
		})
}

func (s *checkpointSaver) DeleteLineage(ctx context.Context, lineage string) error {
	if _, err := s.scope(CheckpointConfig(s.fence.RunKey, "")); err != nil {
		return err
	}
	if lineage != s.fence.RunID {
		return fmt.Errorf("checkpoint lineage mismatch")
	}
	return s.store.DeleteCheckpoints(ctx, s.fence, CheckpointNamespace(s.fence.RunKey))
}

func (*checkpointSaver) Close() error { return nil }

func encodeCheckpoint(envelope checkpointEnvelope, writes []graph.PendingWrite, seq int64, parent string) (
	*agentruntime.CheckpointRecord, error,
) {
	// Pending writes have a dedicated column and are never duplicated in state.
	envelope.Tuple.PendingWrites = nil
	state, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	pending, err := json.Marshal(writes)
	if err != nil {
		return nil, err
	}
	values := envelope.Tuple.Config[graph.CfgKeyConfigurable].(map[string]any)
	return &agentruntime.CheckpointRecord{
		Namespace: values[graph.CfgKeyCheckpointNS].(string),
		ID:        envelope.Tuple.Checkpoint.ID, ParentID: parent, Seq: seq, State: state, PendingWrites: pending,
	}, nil
}

func (s *checkpointSaver) decode(record agentruntime.CheckpointRecord) (*checkpointEnvelope, error) {
	var envelope checkpointEnvelope
	d := json.NewDecoder(bytes.NewReader(record.State))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Version != 1 || envelope.GraphVersion != GraphVersion || envelope.SDKVersion != checkpointSDKVersion {
		return nil, fmt.Errorf("incompatible checkpoint graph/schema/SDK version")
	}
	id, err := s.scope(envelope.Tuple.Config)
	if err != nil {
		return nil, err
	}
	if id != record.ID || record.Namespace != CheckpointNamespace(s.fence.RunKey) ||
		envelope.Tuple.Checkpoint == nil || envelope.Tuple.Checkpoint.ID != record.ID {
		return nil, fmt.Errorf("checkpoint identity mismatch")
	}
	if err := validateGraphCheckpoint(envelope.Tuple.Checkpoint); err != nil {
		return nil, err
	}
	// JSON's generic maps must be rehydrated to the graph's declared state types.
	cp := envelope.Tuple.Checkpoint
	if value, ok := cp.ChannelValues[StateKey]; ok {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var state State
		if err := json.Unmarshal(raw, &state); err != nil {
			return nil, err
		}
		cp.ChannelValues[StateKey] = state
	}
	if value, ok := cp.ChannelValues[graph.StateKeyMessages]; ok {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var messages []model.Message
		if err := json.Unmarshal(raw, &messages); err != nil {
			return nil, err
		}
		cp.ChannelValues[graph.StateKeyMessages] = messages
	}
	if err := json.Unmarshal(record.PendingWrites, &envelope.Tuple.PendingWrites); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func validateGraphCheckpoint(cp *graph.Checkpoint) error {
	if cp.Version != graph.CheckpointVersion {
		return fmt.Errorf("unsupported SDK checkpoint version %d", cp.Version)
	}
	if value, ok := cp.ChannelValues[StateKey]; ok {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		var state State
		if err := json.Unmarshal(raw, &state); err != nil {
			return err
		}
	}
	return nil
}

func (s *checkpointSaver) validateLogs(ctx context.Context, cp *graph.Checkpoint) error {
	value, ok := cp.ChannelValues[StateKey]
	if !ok {
		return nil
	}
	state, ok := value.(State)
	if !ok {
		return fmt.Errorf("invalid graph state type %s", reflect.TypeOf(value))
	}
	return s.store.ValidateCheckpointCalls(ctx, s.fence.RunKey, state.PendingCallIDs, state.AppliedCallIDs)
}
