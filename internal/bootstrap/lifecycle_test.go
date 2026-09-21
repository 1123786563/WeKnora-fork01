package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingLifecycleSink 记录到达底层的注册调用，用于断言重复注册不触碰底层。
type recordingLifecycleSink struct {
	calls []string
}

func (s *recordingLifecycleSink) RegisterLifecycleHook(name string, hook LifecycleHook) {
	s.calls = append(s.calls, name)
}

func TestLifecycleRegistryRejectsDuplicateHook(t *testing.T) {
	sink := &recordingLifecycleSink{}
	reg := NewLifecycleRegistry(sink)
	hook := func(ctx context.Context) error { return nil }

	require.NoError(t, reg.Register("startTemporaryDocumentCleanup", hook))

	err := reg.Register("startTemporaryDocumentCleanup", hook)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "already registered"),
		"重复挂点错误应以 'already registered' 开头，得到: %v", err)
	assert.Equal(t, []string{"startTemporaryDocumentCleanup"}, sink.calls,
		"重复注册不得再次触达底层 hook 集合")
	assert.Equal(t, []string{"startTemporaryDocumentCleanup"}, reg.Names(),
		"重复注册后登记表内容保持不变")
}

func TestLifecycleRegistryNamesSortedSnapshot(t *testing.T) {
	reg := NewLifecycleRegistry(nil)

	require.NoError(t, reg.Register("registerPoolCleanup", func(ctx context.Context) error { return nil }))
	require.NoError(t, reg.Register("registerLangfuseCleanup", func(ctx context.Context) error { return nil }))

	assert.Equal(t, []string{"registerLangfuseCleanup", "registerPoolCleanup"}, reg.Names())
}

func TestLifecycleRegistryStoresAndRunsHooks(t *testing.T) {
	reg := NewLifecycleRegistry(nil)
	called := false
	require.NoError(t, reg.Register("hook", func(ctx context.Context) error {
		called = true
		return errors.New("boom")
	}))

	hook, ok := reg.Lookup("hook")
	require.True(t, ok, "Register 之后应能按名取回挂点")
	require.Error(t, hook(context.Background()))
	assert.True(t, called, "取回的挂点应是 Register 时传入的可执行闭包")

	_, ok = reg.Lookup("missing")
	assert.False(t, ok)
}
