package bootstrap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingWorkerSink 记录到达底层的注册调用，用于断言重复注册不触碰底层。
type recordingWorkerSink struct {
	calls []string
}

func (s *recordingWorkerSink) RegisterTaskHandler(taskType string, handler any) {
	s.calls = append(s.calls, taskType)
}

func TestWorkerRegistryRejectsDuplicateTaskType(t *testing.T) {
	sink := &recordingWorkerSink{}
	reg := NewWorkerRegistry("redis", sink)

	require.NoError(t, reg.Register("TypeDocumentProcess", func() {}))

	err := reg.Register("TypeDocumentProcess", func() {})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "already registered"),
		"重复任务类型错误应以 'already registered' 开头，得到: %v", err)
	assert.Equal(t, []string{"TypeDocumentProcess"}, sink.calls,
		"重复注册不得再次触达底层 mux/executor")
}

func TestWorkerRegistryTaskTypesSortedSnapshot(t *testing.T) {
	reg := NewWorkerRegistry("redis", nil)

	require.NoError(t, reg.Register("TypeWikiIngest", nil))
	require.NoError(t, reg.Register("TypeChunkExtract", nil))
	require.NoError(t, reg.Register("TypeKBDelete", nil))

	assert.Equal(t, []string{"TypeChunkExtract", "TypeKBDelete", "TypeWikiIngest"}, reg.TaskTypes())
}

func TestWorkerRegistriesAreIndependent(t *testing.T) {
	redis := NewWorkerRegistry("redis", nil)
	lite := NewWorkerRegistry("lite", nil)

	require.NoError(t, redis.Register("TypeDocumentProcess", nil))
	require.NoError(t, lite.Register("TypeDocumentProcess", nil), "两个 registry 互不影响（无全局状态）")
	assert.Equal(t, "redis", redis.Mode())
	assert.Equal(t, "lite", lite.Mode())
}

func TestVerifyWorkerParityAcceptsIdenticalSets(t *testing.T) {
	redis := NewWorkerRegistry("redis", nil)
	lite := NewWorkerRegistry("lite", nil)
	for _, tt := range []string{"TypeChunkExtract", "TypeDocumentProcess", "TypeWikiIngest"} {
		require.NoError(t, redis.Register(tt, nil))
		require.NoError(t, lite.Register(tt, nil))
	}

	assert.NoError(t, VerifyWorkerParity(redis, lite))
}

func TestVerifyWorkerParityDetectsMismatch(t *testing.T) {
	redis := NewWorkerRegistry("redis", nil)
	lite := NewWorkerRegistry("lite", nil)
	for _, tt := range []string{"TypeChunkExtract", "TypeDocumentProcess"} {
		require.NoError(t, redis.Register(tt, nil))
	}
	require.NoError(t, lite.Register("TypeChunkExtract", nil))
	require.NoError(t, lite.Register("TypeMemoryExtract", nil))

	err := VerifyWorkerParity(redis, lite)
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "TypeDocumentProcess", "错误应指出仅在 Redis 侧的类型")
	assert.Contains(t, msg, "TypeMemoryExtract", "错误应指出仅在 Lite 侧的类型")
}
