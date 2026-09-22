package native

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

type nativeModelConfigSourceFake struct {
	config NativeModelConfig
	err    error
	calls  int
}

func (s *nativeModelConfigSourceFake) ResolveNativeModel(
	_ context.Context, _ nativecontract.Scope, _ nativecontract.ConfigBinding,
) (NativeModelConfig, error) {
	s.calls++
	return s.config, s.err
}

type nativeCredentialResolverFake struct {
	credential Credential
	err        error
	calls      int
	scopes     []nativecontract.Scope
	refs       []string
	versions   []int64
}

func (r *nativeCredentialResolverFake) ResolveCredential(
	_ context.Context, scope nativecontract.Scope, ref string, version int64,
) (Credential, error) {
	r.calls++
	r.scopes = append(r.scopes, scope)
	r.refs = append(r.refs, ref)
	r.versions = append(r.versions, version)
	return r.credential, r.err
}

func nativeModelBinding() nativecontract.ConfigBinding {
	return nativecontract.ConfigBinding{
		SchemaVersion: 1, SDKVersion: "v1.11.0", GraphVersion: "graph-v1",
		ConfigHash: "config-sha256", ModelID: "model-1", ModelConfigVersion: "model-v3",
		CredentialRef: "credential/model-1", CredentialVersion: 7,
		ToolSetHash: "tools-v1", SkillSetHash: "skills-v1", ExecutionTargetID: "target-1", WorkspaceRef: "workspace-1",
	}
}

func nativeRemoteModelConfig(binding nativecontract.ConfigBinding) NativeModelConfig {
	return NativeModelConfig{
		Binding: binding, Source: types.ModelSourceRemote, Provider: "openai",
		ModelName: "gpt-test", BaseURL: "https://provider.example/v1",
	}
}

func TestNativeModelResolveRejectsMissingOrDriftedConfigBeforeCredentialResolution(t *testing.T) {
	binding := nativeModelBinding()
	for name, sourceBinding := range map[string]nativecontract.ConfigBinding{
		"missing": {},
		"drifted": func() nativecontract.ConfigBinding {
			got := binding
			got.ConfigHash = "different-sha256"
			return got
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			source := &nativeModelConfigSourceFake{config: nativeRemoteModelConfig(sourceBinding)}
			credentials := &nativeCredentialResolverFake{credential: Credential{Value: "secret", Version: 7}}
			resolver := NewModelResolver(source, credentials)

			_, err := resolver.Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
			require.Error(t, err)
			require.Zero(t, credentials.calls, "configuration rejection must not resolve credentials")
		})
	}
}

func TestNativeModelResolveFreezesIdentityAndRejectsCredentialVersionDrift(t *testing.T) {
	binding := nativeModelBinding()
	source := &nativeModelConfigSourceFake{config: nativeRemoteModelConfig(binding)}
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "secret", Version: binding.CredentialVersion + 1}}
	resolver := NewModelResolver(source, credentials)

	resolved, err := resolver.Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
	require.NoError(t, err)
	require.Equal(t, binding, resolved.Config)
	require.NotNil(t, resolved.Model)

	_, err = resolved.Model.GenerateContent(context.Background(), &model.Request{Messages: []model.Message{model.NewUserMessage("hello")}})
	require.Error(t, err)
	require.Equal(t, 1, credentials.calls)
}

func TestNativeModelRejectsCredentialFailureWithoutProviderFallback(t *testing.T) {
	binding := nativeModelBinding()
	source := &nativeModelConfigSourceFake{config: nativeRemoteModelConfig(binding)}
	credentials := &nativeCredentialResolverFake{err: errors.New("credential access denied")}
	resolver := NewModelResolver(source, credentials)

	resolved, err := resolver.Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
	require.NoError(t, err)
	_, err = resolved.Model.GenerateContent(context.Background(), &model.Request{Messages: []model.Message{model.NewUserMessage("hello")}})
	require.ErrorContains(t, err, "credential access denied")
	require.Equal(t, 1, credentials.calls)
}

func TestNativeModelCancellationDoesNotResolveCredential(t *testing.T) {
	binding := nativeModelBinding()
	source := &nativeModelConfigSourceFake{config: nativeRemoteModelConfig(binding)}
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "secret", Version: binding.CredentialVersion}}
	resolved, err := NewModelResolver(source, credentials).Resolve(context.Background(), nativecontract.Scope{TenantID: 9}, binding)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = resolved.Model.GenerateContent(ctx, &model.Request{Messages: []model.Message{model.NewUserMessage("hello")}})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, credentials.calls)
}

func TestNativeModelCredentialResolutionUsesTheAdmittedScopeAndCredentialBinding(t *testing.T) {
	binding := nativeModelBinding()
	scope := nativecontract.Scope{
		TenantID: 9, ActorUserID: "actor-1", SessionOwnerID: "owner-1",
		Principal: nativecontract.Principal{Type: "user", ID: "actor-1"},
	}
	credentials := &nativeCredentialResolverFake{credential: Credential{Value: "secret", Version: binding.CredentialVersion}}
	resolved, err := NewModelResolver(
		&nativeModelConfigSourceFake{config: nativeRemoteModelConfig(binding)}, credentials,
	).Resolve(context.Background(), scope, binding)
	require.NoError(t, err)

	// This invalid combination fails in the SDK before it performs network I/O,
	// but only after the resolver has proved the credential is authorized for
	// this exact admitted identity and version.
	topLogprobs := 1
	_, err = resolved.Model.GenerateContent(context.Background(), &model.Request{
		Messages:         []model.Message{model.NewUserMessage("validate locally")},
		GenerationConfig: model.GenerationConfig{TopLogprobs: &topLogprobs},
	})
	require.Error(t, err)
	require.Equal(t, 1, credentials.calls)
	require.Equal(t, []nativecontract.Scope{scope}, credentials.scopes)
	require.Equal(t, []string{binding.CredentialRef}, credentials.refs)
	require.Equal(t, []int64{binding.CredentialVersion}, credentials.versions)
}
