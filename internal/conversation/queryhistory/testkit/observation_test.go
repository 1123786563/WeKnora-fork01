package testkit

// Differential-gate normalizer tests (Wave 1, Task 9, brief Steps 1-2 and 8).
// They pin the comparison contract BEFORE the testkit exists: the ONLY
// normalizations are JSON object key order (recursive), the four
// transport-only headers, and the single SkipRetry / ErrPermanentPayload
// error-class equivalence. Everything else — CSV bytes, non-JSON bodies,
// AppError payloads, user ids, task options, job transitions — must stay
// significant, and the mutation tests at the bottom prove an
// over-normalizing comparator cannot slip a drift through.

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
)

// httpObs builds a bare HTTP-shaped observation.
func httpObs(status int, headers map[string][]string, body string) Observation {
	return Observation{
		Status:  status,
		Headers: headers,
		Body:    []byte(body),
	}
}

func TestCompareNormalizesJSONObjectKeyOrderOnly(t *testing.T) {
	t.Run("flat object key order is insignificant", func(t *testing.T) {
		want := httpObs(200, nil, `{"success":true,"data":{"job_id":7}}`)
		got := httpObs(200, nil, `{"data":{"job_id":7},"success":true}`)
		require.NoError(t, Compare(want, got))
	})

	t.Run("nested object key order is insignificant recursively", func(t *testing.T) {
		want := httpObs(200, nil,
			`{"a":{"x":1,"y":{"p":"q","r":[1,2]}},"b":[{"z":true,"w":null}]}`)
		got := httpObs(200, nil,
			`{"b":[{"w":null,"z":true}],"a":{"y":{"r":[1,2],"p":"q"},"x":1}}`)
		require.NoError(t, Compare(want, got))
	})

	t.Run("JSON array order is preserved and significant", func(t *testing.T) {
		want := httpObs(200, nil, `{"messages":[{"id":"m1"},{"id":"m2"}]}`)
		got := httpObs(200, nil, `{"messages":[{"id":"m2"},{"id":"m1"}]}`)
		err := Compare(want, got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
	})

	t.Run("JSON number literals are preserved exactly", func(t *testing.T) {
		want := httpObs(200, nil, `{"job_id":1,"big":9007199254740993}`)
		got := httpObs(200, nil, `{"job_id":1.0,"big":9007199254740993}`)
		err := Compare(want, got)
		require.Error(t, err, "1 and 1.0 are different literals and must not collapse")
		require.Contains(t, err.Error(), "Body")
	})
}

func TestCompareDropsOnlyTheFourTransportHeaders(t *testing.T) {
	transportOnly := []string{"Date", "X-Request-Id", "X-Trace-Id", "X-Response-Time"}
	for _, header := range transportOnly {
		t.Run(header+" is dropped", func(t *testing.T) {
			want := Observation{Status: 200, Headers: map[string][]string{
				header: {"value-a"}, "Content-Type": {"application/json; charset=utf-8"},
			}}
			got := Observation{Status: 200, Headers: map[string][]string{
				header: {"a-different-generated-value"}, "Content-Type": {"application/json; charset=utf-8"},
			}}
			require.NoError(t, Compare(want, got))
		})
	}

	t.Run("header keys are canonicalized before the allowlist check", func(t *testing.T) {
		want := Observation{Status: 200, Headers: map[string][]string{
			"x-request-id": {"a"}, "Content-Type": {"application/json"},
		}}
		got := Observation{Status: 200, Headers: map[string][]string{
			"X-Request-ID": {"b"}, "content-type": {"application/json"},
		}}
		require.NoError(t, Compare(want, got))
	})

	t.Run("every other header difference is significant", func(t *testing.T) {
		want := Observation{Status: 200, Headers: map[string][]string{
			"Content-Type": {"text/csv; charset=utf-8"},
		}}
		got := Observation{Status: 200, Headers: map[string][]string{
			"Content-Type": {"application/json"},
		}}
		err := Compare(want, got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Headers[Content-Type]")

		gotMissing := Observation{Status: 200, Headers: map[string][]string{}}
		err = Compare(want, gotMissing)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Headers[Content-Type]")
	})
}

func TestCompareKeepsNonJSONBodiesByteExact(t *testing.T) {
	t.Run("CSV bytes are compared byte for byte", func(t *testing.T) {
		want := httpObs(200, nil, "session_id,title\nexp-a,Alpha\n")
		got := httpObs(200, nil, "session_id,title\nexp-a,Beta\n")
		err := Compare(want, got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
	})

	t.Run("identical CSV passes", func(t *testing.T) {
		want := httpObs(200, nil, "session_id,title\r\nexp-a,Alpha\r\n")
		got := httpObs(200, nil, "session_id,title\r\nexp-a,Alpha\r\n")
		require.NoError(t, Compare(want, got))
	})

	t.Run("empty body vs content is a mismatch", func(t *testing.T) {
		err := Compare(httpObs(204, nil, ""), httpObs(204, nil, "x"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
	})
}

func TestCompareKeepsAppErrorFieldsSignificant(t *testing.T) {
	// The error envelope the platform middleware renders: success=false plus
	// the AppError's status, code, and message. All of it stays significant.
	want := httpObs(403, nil,
		`{"error":{"code":"FORBIDDEN","message":"query history is disabled for this tenant","details":null},"success":false}`)

	t.Run("different message is a mismatch", func(t *testing.T) {
		got := httpObs(403, nil,
			`{"error":{"code":"FORBIDDEN","message":"query history is disabled","details":null},"success":false}`)
		err := Compare(want, got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
	})

	t.Run("different code is a mismatch", func(t *testing.T) {
		got := httpObs(403, nil,
			`{"error":{"code":"BAD_REQUEST","message":"query history is disabled for this tenant","details":null},"success":false}`)
		err := Compare(want, got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
	})
}

func TestNormalizeErrorClasses(t *testing.T) {
	t.Run("nil error is the empty class", func(t *testing.T) {
		require.Equal(t, "", NormalizeError(nil))
	})

	t.Run("legacy SkipRetry and module ErrPermanentPayload share one class", func(t *testing.T) {
		legacy := fmt.Errorf("query history export: unmarshal payload (bad): %w", asynq.SkipRetry)
		modern := fmt.Errorf("query history export: unmarshal payload (bad): %w", domain.ErrPermanentPayload)
		require.Equal(t, ClassPermanent, NormalizeError(legacy))
		require.Equal(t, ClassPermanent, NormalizeError(modern))
		require.Equal(t, NormalizeError(legacy), NormalizeError(modern))
	})

	t.Run("every other error keeps its exact text", func(t *testing.T) {
		storage := errors.New("store export file: disk full")
		require.Equal(t, "store export file: disk full", NormalizeError(storage))
		require.NotEqual(t, NormalizeError(storage),
			NormalizeError(errors.New("store export file: disk FULL")))
	})
}

func TestCompareSortsCollectionsByStableIDOnly(t *testing.T) {
	t.Run("jobs compare by id regardless of slice order", func(t *testing.T) {
		want := Observation{Jobs: []JobObservation{
			{ID: 1, TenantID: 1, Status: "pending"},
			{ID: 2, TenantID: 1, Status: "done", FilePath: "local://temp/x.csv"},
		}}
		got := Observation{Jobs: []JobObservation{
			{ID: 2, TenantID: 1, Status: "done", FilePath: "local://temp/x.csv"},
			{ID: 1, TenantID: 1, Status: "pending"},
		}}
		require.NoError(t, Compare(want, got))
	})

	t.Run("tasks compare by type+payload regardless of slice order", func(t *testing.T) {
		task := func(payload string) TaskObservation {
			return TaskObservation{
				Type: "query:history:export", Queue: "low", MaxRetry: 3,
				Timeout: 10 * time.Minute, Payload: []byte(payload),
			}
		}
		want := Observation{Enqueued: []TaskObservation{
			task(`{"job_id":1}`), task(`{"job_id":2}`),
		}}
		got := Observation{Enqueued: []TaskObservation{
			task(`{"job_id":2}`), task(`{"job_id":1}`),
		}}
		require.NoError(t, Compare(want, got))
	})

	t.Run("stored files compare by path and bytes", func(t *testing.T) {
		want := Observation{StoredFiles: map[string][]byte{
			"local://temp/a.csv": []byte("x"),
		}}
		got := Observation{StoredFiles: map[string][]byte{
			"local://temp/a.csv": []byte("x"),
		}}
		require.NoError(t, Compare(want, got))

		gotOther := Observation{StoredFiles: map[string][]byte{
			"local://temp/a.csv": []byte("y"),
		}}
		err := Compare(want, gotOther)
		require.Error(t, err)
		require.Contains(t, err.Error(), "StoredFiles[local://temp/a.csv]")
	})
}

func TestObserveHTTPCapturesResponse(t *testing.T) {
	obs := ObserveHTTP(http.StatusOK,
		http.Header{"Content-Type": {"text/csv; charset=utf-8"}},
		[]byte("session_id\n"))
	require.Equal(t, http.StatusOK, obs.Status)
	require.Equal(t, map[string][]string{"Content-Type": {"text/csv; charset=utf-8"}}, obs.Headers)
	require.Equal(t, []byte("session_id\n"), obs.Body)
}

// TestCompareDetectsDrift is the anti-false-confidence check (brief Step 8):
// five mutation classes — status, anonymized user id, CSV column order, task
// retry count, and a tenant-scoped job transition — each MUST fail the
// comparison with the mutated field named in the error.
func TestCompareDetectsDrift(t *testing.T) {
	base := func() Observation {
		return Observation{
			Status:  http.StatusOK,
			Headers: map[string][]string{"Content-Type": {"application/json; charset=utf-8"}},
			Body:    []byte(`{"data":{"user_id":"anonymous"},"items":["a","b"]}`),
			Jobs: []JobObservation{{
				ID: 1, TenantID: 7, RequestedBy: "admin-7",
				Status: "done", FilePath: "local://temp/query_history_export_1.csv",
			}},
			StoredFiles: map[string][]byte{
				"local://temp/query_history_export_1.csv": []byte("session_id,title\n"),
			},
			Enqueued: []TaskObservation{{
				Type: "query:history:export", Queue: "low", MaxRetry: 3,
				Timeout: 10 * time.Minute, Payload: []byte(`{"job_id":1,"tenant_id":7}`),
			}},
		}
	}

	t.Run("status mutation is caught", func(t *testing.T) {
		got := base()
		got.Status = http.StatusForbidden
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Status")
	})

	t.Run("anonymized user id mutation is caught", func(t *testing.T) {
		got := base()
		got.Body = []byte(`{"data":{"user_id":"u-real-identity"},"items":["a","b"]}`)
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Body")
		require.True(t, strings.HasPrefix(err.Error(), "Body:"),
			"the comparison error must name the Body field")
	})

	t.Run("CSV column order mutation is caught", func(t *testing.T) {
		want := base()
		want.Body = []byte("session_id,title,user_id\ns1,T,u1\n")
		want.Headers = map[string][]string{"Content-Type": {"text/csv; charset=utf-8"}}
		got := base()
		got.Body = []byte("user_id,session_id,title\nu1,s1,T\n")
		got.Headers = map[string][]string{"Content-Type": {"text/csv; charset=utf-8"}}
		err := Compare(want, got)
		require.Error(t, err, "a CSV column reorder is a contract change")
		require.Contains(t, err.Error(), "Body")
	})

	t.Run("task retry count mutation is caught", func(t *testing.T) {
		got := base()
		got.Enqueued[0].MaxRetry = 25
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Enqueued[0].MaxRetry")
	})

	t.Run("tenant-scoped job transition mutation is caught", func(t *testing.T) {
		got := base()
		got.Jobs[0].Status = "failed"
		got.Jobs[0].ErrorMessage = "boom"
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Jobs[0].Status")

		foreign := base()
		foreign.Jobs[0].TenantID = 8
		err = Compare(base(), foreign)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Jobs[0].TenantID")
	})

	t.Run("the unmutated baseline still passes", func(t *testing.T) {
		// The same observation with reordered JSON keys and fresh transport
		// headers is equivalent: normalizations apply, drift does not.
		got := base()
		got.Body = []byte(`{"items":["a","b"],"data":{"user_id":"anonymous"}}`)
		got.Headers = map[string][]string{
			"Content-Type":    {"application/json; charset=utf-8"},
			"Date":            {"Tue, 01 Sep 2026 00:00:00 GMT"},
			"X-Request-Id":    {"req-2"},
			"X-Trace-Id":      {"trace-2"},
			"X-Response-Time": {"12ms"},
		}
		require.NoError(t, Compare(base(), got))
	})

	t.Run("error class mutation is caught", func(t *testing.T) {
		got := base()
		got.ErrorClass = "permanent"
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ErrorClass")
	})

	t.Run("payload mutation inside the enqueued task is caught", func(t *testing.T) {
		got := base()
		got.Enqueued[0].Payload = []byte(`{"job_id":1,"tenant_id":8}`)
		err := Compare(base(), got)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Enqueued[0].Payload")
	})
}
