package skillhub

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/utils"
)

// newTestServerClient stands up an httptest upstream reachable through the
// SSRF-safe transport (loopback must be whitelisted explicitly, like every
// other connector test in this repository).
func newTestServerClient(t *testing.T, timeout time.Duration, h http.HandlerFunc) (*HTTPClient, *httptest.Server) {
	t.Helper()
	utils.SetSSRFWhitelistFromRaw("127.0.0.1,::1,localhost")
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client, err := New(srv.URL, timeout)
	require.NoError(t, err)
	return client, srv
}

type recordedRequest struct {
	method string
	path   string
	query  string
	accept string
	ua     string
}

func recordRequest(r *http.Request, sink *recordedRequest) {
	*sink = recordedRequest{
		method: r.Method,
		path:   r.URL.Path,
		query:  r.URL.RawQuery,
		accept: r.Header.Get("Accept"),
		ua:     r.Header.Get("User-Agent"),
	}
}

const searchFixture = `{"results":[
  {"slug":"pdf-extract","displayName":"PDF Extract","summary":"Pull pages","version":"1.2.0","author":"tencent"},
  {"name":"No Slug Skill"},
  "not-an-object",
  {"slug":"legacy","summary":"uses summary"},
  {"slug":"","name":"Skipped Empty"},
  {"slug":"display-empty","displayName":"","name":"Fallback Name","downloads":0}
]}`

func TestSearchHappyPath(t *testing.T) {
	var rec recordedRequest
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		recordRequest(r, &rec)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, searchFixture)
	})

	results, err := client.Search(context.Background(), "  pdf  ", 50)
	require.NoError(t, err)

	require.Equal(t, http.MethodGet, rec.method)
	require.Equal(t, "/api/v1/search", rec.path)
	require.Equal(t, "application/json", rec.accept)
	require.Equal(t, "weknora-skillhub-market/1.0", rec.ua)

	// url.Values encodes sorted keys: limit before q.
	require.Equal(t, "limit=50&q=pdf", rec.query)

	require.Len(t, results, 3)

	first := results[0]
	require.Equal(t, "pdf-extract", first.Slug)
	require.Equal(t, "PDF Extract", first.Name)
	require.Equal(t, "Pull pages", first.Description)
	require.Equal(t, "1.2.0", first.Version)
	require.Equal(t, "tencent", first.Raw["author"])
	require.Equal(t, "pdf-extract", first.Raw["slug"])

	// No slug -> entry dropped; non-object entry -> dropped.
	require.Equal(t, "legacy", results[1].Slug)
	require.Equal(t, "legacy", results[1].Name) // falls back to slug
	require.Equal(t, "uses summary", results[1].Description)
	require.Equal(t, "", results[1].Version)

	require.Equal(t, "display-empty", results[2].Slug)
	require.Equal(t, "Fallback Name", results[2].Name) // empty displayName falls to name
	require.Equal(t, "", results[2].Description)
	require.Equal(t, "0", results[2].Raw["downloads"]) // numeric passthrough renders as text
}

func TestSearchNameChainEdgeCases(t *testing.T) {
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[
		  {"slug":"ws-display","displayName":"   ","name":"Blocked Name"},
		  {"slug":"ws-name","name":"   "},
		  {"slug":"num-fallback","displayName":0,"name":"Zero Skip Wins"}
		]}`)
	})
	results, err := client.Search(context.Background(), "q", 10)
	require.NoError(t, err)
	require.Len(t, results, 3)
	// Whitespace displayName wins the `or` chain, strips to empty, and falls
	// back to slug WITHOUT consulting name (Python str(...).strip() or slug).
	require.Equal(t, "ws-display", results[0].Name)
	// Whitespace name likewise collapses to the slug.
	require.Equal(t, "ws-name", results[1].Name)
	// Falsy displayName (0) never wins the chain; name is used.
	require.Equal(t, "Zero Skip Wins", results[2].Name)
}

func TestSearchQueryAndLimitClamping(t *testing.T) {
	var queries []string
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		fmt.Fprint(w, `{"results":[]}`)
	})

	_, err := client.Search(context.Background(), "   ", 0)
	require.NoError(t, err)
	_, err = client.Search(context.Background(), "q", 1000)
	require.NoError(t, err)

	require.Equal(t, []string{"limit=1&q=a", "limit=100&q=q"}, queries)
}

func TestSearchNon200(t *testing.T) {
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	_, err := client.Search(context.Background(), "q", 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMarket)
	require.Contains(t, err.Error(), "500")
}

func TestSearchInvalidJSON(t *testing.T) {
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "this is { not json")
	})
	_, err := client.Search(context.Background(), "q", 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMarket)
	require.Contains(t, err.Error(), "JSON")
}

func TestSearchMissingResultsArray(t *testing.T) {
	for _, body := range []string{`{"items":[]}`, `{"results":{}}`, `[1,2,3]`} {
		client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, body)
		})
		_, err := client.Search(context.Background(), "q", 10)
		require.Error(t, err, "body %q must be rejected", body)
		require.ErrorIs(t, err, ErrMarket)
	}
}

func TestSearchDeclaredContentLengthOversize(t *testing.T) {
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(int(maxJSONBytes+1)))
		w.Write([]byte("x"))
	})
	_, err := client.Search(context.Background(), "q", 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestSearchStreamedOversize(t *testing.T) {
	client, _ := newTestServerClient(t, 30*time.Second, func(w http.ResponseWriter, r *http.Request) {
		// Chunked (no Content-Length): only the streamed cap can catch this.
		chunk := make([]byte, 64*1024)
		for total := 0; total <= int(maxJSONBytes)+len(chunk); total += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return // client hung up once it noticed the violation
			}
		}
	})
	_, err := client.Search(context.Background(), "q", 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestRankingsHappyPathAllKinds(t *testing.T) {
	for _, kind := range RankingKinds() {
		t.Run(kind, func(t *testing.T) {
			var rec recordedRequest
			client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
				recordRequest(r, &rec)
				fmt.Fprintf(w, `{"results":[{"slug":"s-%s","displayName":"S %s"}]}`, kind, kind)
			})
			results, err := client.Rankings(context.Background(), kind)
			require.NoError(t, err)
			require.Equal(t, "/api/v1/showcase/"+kind, rec.path)
			require.Len(t, results, 1)
			require.Equal(t, "s-"+kind, results[0].Slug)
			require.Equal(t, "S "+kind, results[0].Name)
		})
	}
}

func TestRankingsInvalidKindNeverHitsNetwork(t *testing.T) {
	calls := 0
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"results":[]}`)
	})
	_, err := client.Rankings(context.Background(), "bogus")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsupportedRanking)
	require.Equal(t, 0, calls)
}

func TestDownloadHappyPath(t *testing.T) {
	payload := []byte("PK\x03\x04 fake zip bytes")
	var rec recordedRequest
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		recordRequest(r, &rec)
		w.Header().Set("Content-Type", "application/zip")
		w.Write(payload)
	})
	got, err := client.Download(context.Background(), "pdf extract")
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.Equal(t, "/api/v1/download", rec.path)
	require.Equal(t, "slug=pdf+extract", rec.query)
	require.Equal(t, "application/zip,application/octet-stream,*/*", rec.accept)
	require.Equal(t, "weknora-skillhub-market/1.0", rec.ua)
}

func TestDownloadDeclaredOversize(t *testing.T) {
	client, _ := newTestServerClient(t, 5*time.Second, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(int(maxDownloadBytes+1)))
		w.Write([]byte("x"))
	})
	_, err := client.Download(context.Background(), "slug")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestDownloadStreamedOversize(t *testing.T) {
	client, _ := newTestServerClient(t, 60*time.Second, func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 256*1024)
		for total := 0; total <= int(maxDownloadBytes)+len(chunk); total += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
	_, err := client.Download(context.Background(), "slug")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPackageTooLarge)
}

func TestRequestTimeout(t *testing.T) {
	client, _ := newTestServerClient(t, 50*time.Millisecond, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		fmt.Fprint(w, `{"results":[]}`)
	})
	_, err := client.Search(context.Background(), "q", 10)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrMarketTimeout)
	require.ErrorIs(t, err, ErrMarket)
}

func TestNewHostResolution(t *testing.T) {
	_, err := New("", 0)
	require.NoError(t, err)

	_, err = New("  ", time.Second)
	require.NoError(t, err)

	c, err := New(" http://localhost:9999/ ", time.Second)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:9999", c.host)

	for _, bad := range []string{"ftp://api.skillhub.cn", "://missing-scheme", "http://", "skillhub.cn"} {
		_, err := New(bad, time.Second)
		require.Error(t, err, "host %q must be rejected", bad)
		require.ErrorIs(t, err, ErrInvalidHost)
	}
}

func TestDefaultConstants(t *testing.T) {
	require.Equal(t, "https://api.skillhub.cn", DefaultHost)
	require.Equal(t, 30*time.Second, DefaultTimeout)
	require.Equal(t, "weknora-skillhub-market/1.0", UserAgent)
}

func TestHTTPClientImplementsClient(t *testing.T) {
	var _ Client = (*HTTPClient)(nil)
}
