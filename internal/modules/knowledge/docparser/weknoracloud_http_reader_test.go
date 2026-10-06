package docparser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestWeKnoraCloudPollTimeoutConfig(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		want        time.Duration
	}{
		{"empty", "", weKnoraCloudDefaultPollTimeout},
		{"whitespace", "  ", weKnoraCloudDefaultPollTimeout},
		{"minutes", "45m", 45 * time.Minute},
		{"trimmed", " 45m ", 45 * time.Minute},
		{"invalid", "invalid", weKnoraCloudDefaultPollTimeout},
		{"unitless", "1200", weKnoraCloudDefaultPollTimeout},
		{"zero", "0s", weKnoraCloudDefaultPollTimeout},
		{"negative", "-1s", weKnoraCloudDefaultPollTimeout},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WEKNORA_WEKNORACLOUD_TIMEOUT", tt.value)
			reader, err := NewWeKnoraCloudSignedDocumentReader("app-id", "api-key")
			require.NoError(t, err)
			assert.Equal(t, tt.want, reader.pollTimeout)
		})
	}
}

func TestWeKnoraCloudPollStopsAtConfiguredTimeout(t *testing.T) {
	allowLoopbackSSRF(t)
	t.Setenv("WEKNORA_WEKNORACLOUD_TIMEOUT", "300ms")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"task_id":"task-1","status":"running","progress":0.1}`)
	}))
	defer server.Close()

	reader, err := NewWeKnoraCloudSignedDocumentReader("app-id", "api-key")
	require.NoError(t, err)
	reader.baseURL = server.URL
	reader.initialPollInterval = 20 * time.Millisecond
	reader.maxPollInterval = 50 * time.Millisecond

	start := time.Now()
	_, err = reader.pollTaskResult(context.Background(), "task-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second, "the configured 300ms timeout, not the 20m default, should stop the poll")
}

func TestWeKnoraCloudReadCompletes(t *testing.T) {
	allowLoopbackSSRF(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/reader":
			_, _ = io.WriteString(w, `{"task_id":"task-7","status":"processing"}`)
		case "/task-7":
			_, _ = io.WriteString(w, `{"task_id":"task-7","status":"completed","result":{"markdown_content":"cloud read ok"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	reader, err := NewWeKnoraCloudSignedDocumentReader("app-id", "api-key")
	require.NoError(t, err)
	reader.baseURL = server.URL

	res, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName: "test.docx", FileType: "docx", FileContent: []byte("doc-bytes"),
	})
	require.NoError(t, err)
	assert.Equal(t, "cloud read ok", res.MarkdownContent)
}
