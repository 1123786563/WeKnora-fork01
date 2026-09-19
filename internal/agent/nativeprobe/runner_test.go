package nativeprobe

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type scriptedModel struct{}

func (*scriptedModel) Info() model.Info {
	return model.Info{Name: "native-contract-probe"}
}

func (*scriptedModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	msg := model.Message{Role: model.RoleAssistant}
	for _, input := range req.Messages {
		if input.Role == model.RoleTool && input.ToolID == "call-1" {
			msg.Content = "finished"
		}
	}
	if msg.Content == "" {
		msg.ToolCalls = []model.ToolCall{{
			ID: "call-1", Type: "function",
			Function: model.FunctionDefinitionParam{Name: "count", Arguments: []byte(`{}`)},
		}}
	}
	out := make(chan *model.Response, 1)
	out <- &model.Response{
		Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{Message: msg}},
	}
	close(out)
	return out, nil
}

func TestNativeRunnerToolRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var calls atomic.Int32
	count := function.NewFunctionTool(func(context.Context, struct{}) (string, error) {
		calls.Add(1)
		return "counted", nil
	}, function.WithName("count"), function.WithDescription("Count one invocation"))
	ag := llmagent.New("native-probe", llmagent.WithModel(&scriptedModel{}),
		llmagent.WithTools([]tool.Tool{count}))
	svc := inmemory.NewSessionService()
	r := runner.NewRunner("native-probe", ag, runner.WithSessionService(svc))
	defer r.Close()
	defer svc.Close()
	events, err := r.Run(ctx, "user-1", "session-1", model.NewUserMessage("count"))
	require.NoError(t, err)
	finished := false
	for ev := range events {
		if ev == nil || ev.Response == nil {
			continue
		}
		require.Nil(t, ev.Error)
		for _, choice := range ev.Choices {
			if choice.Message.Content == "finished" {
				finished = true
			}
		}
	}
	require.NoError(t, ctx.Err())
	require.True(t, finished)
	require.EqualValues(t, 1, calls.Load())

	sess, err := svc.GetSession(ctx, session.Key{
		AppName: "native-probe", UserID: "user-1", SessionID: "session-1",
	})
	require.NoError(t, err)
	require.NotNil(t, sess)
	require.NotEmpty(t, sess.Events, "Runner must synchronize its emitted events into the Session")
	require.False(t, sess.UpdatedAt.IsZero(), "Runner session mutation must update UpdatedAt")
}

func TestSessionScopeKeysSeparateTenants(t *testing.T) {
	ctx := context.Background()
	svc := inmemory.NewSessionService()
	defer svc.Close()
	a := session.Key{AppName: "weknora/tenant/1", UserID: "same", SessionID: "same"}
	b := session.Key{AppName: "weknora/tenant/2", UserID: "same", SessionID: "same"}
	_, err := svc.CreateSession(ctx, a, session.StateMap{"marker": []byte("a")})
	require.NoError(t, err)
	_, err = svc.CreateSession(ctx, b, session.StateMap{"marker": []byte("b")})
	require.NoError(t, err)
	gotA, err := svc.GetSession(ctx, a)
	require.NoError(t, err)
	gotB, err := svc.GetSession(ctx, b)
	require.NoError(t, err)
	require.Equal(t, []byte("a"), gotA.State["marker"])
	require.Equal(t, []byte("b"), gotB.State["marker"])
}
