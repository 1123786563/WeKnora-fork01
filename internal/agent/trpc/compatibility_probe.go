// Package trpc contains compatibility gates for the pinned recovery SDK.
package trpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	_ "github.com/mattn/go-sqlite3" // Register the SQLite driver used by the checkpoint probe.
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	checkpointsqlite "trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

const (
	probeLineageID     = "compatibility-run"
	probeNamespace     = "compatibility-v1"
	probeToolCallID    = "call-probe-1"
	probeToolName      = "probe_increment"
	stateKeyToolCallID = "probe_tool_call_id"
)

// ProbeReport describes durable counters and the observed SDK terminal state.
type ProbeReport struct {
	ModelCalls      int
	ToolCalls       int
	Completed       bool
	PendingRestored bool
}

// RunCheckpointProbe builds a fresh SDK runner, GraphAgent and SQLite saver.
// It interrupts after the model plan has committed, then resumes the exact
// checkpoint when called again. The database is a disposable probe artifact,
// not an application database. No state survives outside that file.
func RunCheckpointProbe(ctx context.Context, path string, interrupt bool) (report ProbeReport, err error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return report, err
	}
	// SDK List keeps its result cursor open while GetTuple issues another query.
	db.SetMaxOpenConns(2)
	defer func() { err = errors.Join(err, db.Close()) }()
	const createCounts = `CREATE TABLE IF NOT EXISTS probe_counts (name TEXT PRIMARY KEY, value INTEGER NOT NULL)`
	if _, err = db.ExecContext(ctx, createCounts); err != nil {
		return report, err
	}
	saver, err := checkpointsqlite.NewSaver(db)
	if err != nil {
		return report, err
	}
	defer func() { err = errors.Join(err, saver.Close()) }()
	latest, err := graph.NewCheckpointManager(saver).Latest(ctx, probeLineageID, probeNamespace)
	if err != nil {
		return report, err
	}
	state := map[string]any{graph.CfgKeyLineageID: probeLineageID, graph.CfgKeyCheckpointNS: probeNamespace}
	if latest != nil {
		state[graph.CfgKeyCheckpointID] = latest.Checkpoint.ID
		report.PendingRestored = latest.Checkpoint.InterruptState != nil &&
			latest.Checkpoint.InterruptState.TaskID == probeToolCallID
		if !interrupt {
			state[graph.StateKeyCommand] = &graph.Command{ResumeMap: map[string]any{probeToolCallID: true}}
		}
	}
	llm := &probeModel{db: db}
	tools := map[string]tool.Tool{
		probeToolName: function.NewFunctionTool(func(ctx context.Context, _ struct{}) (string, error) {
			return "incremented", incrementProbeCounter(ctx, db, "tool")
		}, function.WithName(probeToolName), function.WithDescription("Increment the durable probe counter.")),
	}
	schema := graph.MessagesStateSchema().AddField(stateKeyToolCallID, graph.StateField{
		Type: reflect.TypeOf(""), Reducer: graph.DefaultReducer,
	})
	g, err := graph.NewStateGraph(schema).
		AddLLMNode("plan", llm, "Plan one tool call.", tools).
		AddNode("approval", func(ctx context.Context, state graph.State) (any, error) {
			// The saver round-trips generic JSON. Decode the message schema,
			// preserving the provider's tool-call ID rather than generating one.
			data, err := json.Marshal(state[graph.StateKeyMessages])
			if err != nil {
				return nil, err
			}
			var messages []model.Message
			if err := json.Unmarshal(data, &messages); err != nil {
				return nil, err
			}
			if len(messages) == 0 || len(messages[len(messages)-1].ToolCalls) != 1 {
				return nil, fmt.Errorf("missing restored tool plan")
			}
			call := messages[len(messages)-1].ToolCalls[0]
			if interrupt || latest != nil {
				payload := map[string]any{"tool_call_id": call.ID, "tool_name": call.Function.Name}
				if _, err := graph.Interrupt(ctx, state, call.ID, payload); err != nil {
					return nil, err
				}
			}
			return graph.State{stateKeyToolCallID: call.ID}, nil
		}).
		AddToolsNode("tool", tools).
		AddLLMNode("answer", llm, "Answer using the tool result.", nil).
		AddEdge("plan", "approval").AddEdge("approval", "tool").AddEdge("tool", "answer").
		SetEntryPoint("plan").SetFinishPoint("answer").Compile()
	if err != nil {
		return report, err
	}
	ag, err := graphagent.New("compatibility", g, graphagent.WithCheckpointSaver(saver))
	if err != nil {
		return report, err
	}
	r := runner.NewRunner("compatibility", ag, runner.WithSessionService(noop.NewService()))
	defer func() { err = errors.Join(err, r.Close()) }()
	message := model.NewUserMessage("Run the probe")
	if latest != nil {
		message = model.NewUserMessage("resume")
	}
	events, err := r.Run(ctx, "probe-user", "probe-session", message, agent.WithRuntimeState(state))
	if err != nil {
		return report, err
	}
	for evt := range events {
		if evt.Response == nil {
			continue
		}
		if evt.Error != nil {
			err = errors.Join(err, fmt.Errorf("SDK event %s: %s", evt.Error.Type, evt.Error.Message))
		}
		if evt.Object == graph.ObjectTypeGraphExecution && evt.Done && len(evt.Choices) == 1 &&
			evt.Choices[0].Message.Content == "probe complete" {
			report.Completed = true
		}
	}
	const readCounts = `SELECT COALESCE((SELECT value FROM probe_counts WHERE name = 'model'), 0),
		COALESCE((SELECT value FROM probe_counts WHERE name = 'tool'), 0)`
	if countErr := db.QueryRowContext(ctx, readCounts).Scan(&report.ModelCalls, &report.ToolCalls); countErr != nil {
		err = errors.Join(err, countErr)
	}
	return report, err
}

func incrementProbeCounter(ctx context.Context, db *sql.DB, name string) error {
	const increment = `INSERT INTO probe_counts(name, value) VALUES (?, 1)
		ON CONFLICT(name) DO UPDATE SET value = value + 1`
	_, err := db.ExecContext(ctx, increment, name)
	return err
}

type probeModel struct{ db *sql.DB }

func (*probeModel) Info() model.Info { return model.Info{Name: "deterministic-checkpoint-probe"} }

func (m *probeModel) GenerateContent(ctx context.Context, request *model.Request) (<-chan *model.Response, error) {
	if err := incrementProbeCounter(ctx, m.db, "model"); err != nil {
		return nil, err
	}
	message := model.Message{Role: model.RoleAssistant}
	for _, input := range request.Messages {
		if input.Role == model.RoleTool {
			if input.ToolID != probeToolCallID || input.Content != `"incremented"` {
				return nil, fmt.Errorf("unexpected tool result: %+v", input)
			}
			message.Content = "probe complete"
		}
	}
	if message.Content == "" {
		message.ToolCalls = []model.ToolCall{{
			ID: probeToolCallID, Type: "function",
			Function: model.FunctionDefinitionParam{Name: probeToolName, Arguments: []byte(`{}`)},
		}}
	}
	responses := make(chan *model.Response, 1)
	responses <- &model.Response{
		Object: model.ObjectTypeChatCompletion, Done: true, Choices: []model.Choice{{Message: message}},
	}
	close(responses)
	return responses, nil
}
