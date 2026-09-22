package bootstrap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRouteSink 记录到达底层的注册调用，用于断言重复注册不触碰底层。
type recordingRouteSink struct {
	calls []Route
}

func (s *recordingRouteSink) HandleRoute(method, path string) {
	s.calls = append(s.calls, Route{Method: method, Path: path})
}

func TestRouteRegistryAcceptsDistinctRoutes(t *testing.T) {
	sink := &recordingRouteSink{}
	reg := NewRouteRegistry(sink)

	require.NoError(t, reg.Register("GET", "/api/v1/knowledge-bases"))
	require.NoError(t, reg.Register("POST", "/api/v1/knowledge-bases"))
	require.NoError(t, reg.Register("GET", "/health"))

	assert.Equal(t, []Route{
		{Method: "GET", Path: "/api/v1/knowledge-bases"},
		{Method: "GET", Path: "/health"},
		{Method: "POST", Path: "/api/v1/knowledge-bases"},
	}, reg.Routes(), "Routes() 应返回排序快照")
	assert.Len(t, sink.calls, 3, "每次首次注册都应恰好触达底层一次")
}

func TestRouteRegistryRejectsDuplicateRouteAndKeepsSinkUnmutated(t *testing.T) {
	sink := &recordingRouteSink{}
	reg := NewRouteRegistry(sink)

	require.NoError(t, reg.Register("GET", "/api/v1/session"))

	err := reg.Register("GET", "/api/v1/session")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "already registered"),
		"重复注册错误应以 'already registered' 开头，得到: %v", err)
	assert.Equal(t, []Route{{Method: "GET", Path: "/api/v1/session"}}, sink.calls,
		"重复注册不得再次触达底层 sink")

	// method 不同、path 相同不算重复。
	require.NoError(t, reg.Register("DELETE", "/api/v1/session"))
	assert.Len(t, sink.calls, 2)
}

func TestRouteRegistryNoGlobalMutableState(t *testing.T) {
	a := NewRouteRegistry(nil)
	b := NewRouteRegistry(nil)

	require.NoError(t, a.Register("GET", "/shared"))
	require.NoError(t, b.Register("GET", "/shared"), "两个独立 registry 互不影响（无全局状态）")
}
