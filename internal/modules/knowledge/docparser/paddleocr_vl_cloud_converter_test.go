package docparser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestPaddleOCRVLCloudTimeoutConfig(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		want        time.Duration
	}{
		{"empty", "", paddleOCRVLCloudDefaultTimeout},
		{"whitespace", "  ", paddleOCRVLCloudDefaultTimeout},
		{"minutes", "12m", 12 * time.Minute},
		{"trimmed", " 720s ", 12 * time.Minute},
		{"invalid", "invalid", paddleOCRVLCloudDefaultTimeout},
		{"unitless", "600", paddleOCRVLCloudDefaultTimeout},
		{"zero", "0s", paddleOCRVLCloudDefaultTimeout},
		{"negative", "-1s", paddleOCRVLCloudDefaultTimeout},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WEKNORA_PADDLEOCR_VL_CLOUD_TIMEOUT", tt.value)
			assert.Equal(t, tt.want, NewPaddleOCRVLCloudReader(nil).timeout)
		})
	}
}

func TestPaddleOCRVLCloudPollStopsAtConfiguredTimeout(t *testing.T) {
	allowLoopbackSSRF(t)
	t.Setenv("WEKNORA_PADDLEOCR_VL_CLOUD_TIMEOUT", "200ms")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"state":"running"}}`)
	}))
	defer server.Close()

	reader := NewPaddleOCRVLCloudReader(nil)
	reader.baseURL = server.URL
	reader.pollInterval = 10 * time.Millisecond

	start := time.Now()
	_, err := reader.pollJob(context.Background(), "job-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("timed out after %s", 200*time.Millisecond))
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestPaddleOCRVLCloudReadCompletes(t *testing.T) {
	allowLoopbackSSRF(t)
	var jsonlURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/":
			_, _ = io.WriteString(w, `{"data":{"jobId":"job-42"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/job-42":
			fmt.Fprintf(w, `{"data":{"state":"done","resultUrl":{"jsonUrl":%q}}}`, jsonlURL)
		default:
			_, _ = io.WriteString(w, `{"result":{"layoutParsingResults":[{"markdown":{"text":"cloud parsed document","images":{}}}]}}`+"\n")
		}
	}))
	defer server.Close()
	jsonlURL = server.URL + "/results/job-42.jsonl"

	reader := NewPaddleOCRVLCloudReader(map[string]string{
		"paddleocr_vl_cloud_token":    "test-token",
		"paddleocr_vl_cloud_base_url": server.URL,
	})
	res, err := reader.Read(context.Background(), &types.ReadRequest{
		FileName: "test.pdf", FileType: "pdf", FileContent: []byte("%PDF-test"),
	})
	require.NoError(t, err)
	assert.Empty(t, res.Error)
	assert.Equal(t, "cloud parsed document", res.MarkdownContent)
}
