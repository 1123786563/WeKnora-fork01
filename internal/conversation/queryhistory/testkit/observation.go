// Package testkit is the old-vs-new differential compatibility gate's
// comparison substrate (Wave 1, Task 9). An Observation captures everything
// one side of a scenario made observable — an HTTP response, worker errors,
// job rows, stored files, enqueued tasks — and Compare proves two stacks
// produce IDENTICAL observations after an explicitly allowlisted set of
// normalizations:
//
//   - JSON object key order, recursively (JSON array order stays significant).
//   - The four transport-only headers: Date, X-Request-Id, X-Trace-Id, and
//     X-Response-Time (generated per response, never business data).
//   - One error-class equivalence: the legacy asynq.SkipRetry sentinel and the
//     module's domain.ErrPermanentPayload both mean "payload not retryable"
//     (NormalizeError maps them to ClassPermanent).
//
// Everything else — CSV bytes, non-JSON bodies, AppError status/code/message,
// user ids, timestamps in domain records, file paths, task options, job
// transitions — stays significant. Compare never ignores a mismatch and names
// the exact field that diverged; the mutation tests in observation_test.go
// (brief Step 8) prove an over-normalizing comparator cannot pass.
package testkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"sort"
	"time"

	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
)

// Observation is one side of a differential scenario: what a stack made
// observable. HTTP scenarios fill Status/Headers/Body; worker scenarios fill
// ErrorClass/Jobs/StoredFiles/Enqueued. Unused fields stay zero on both sides
// and therefore compare equal.
type Observation struct {
	Status      int
	Headers     map[string][]string
	Body        []byte
	ErrorClass  string
	Jobs        []JobObservation
	StoredFiles map[string][]byte
	Enqueued    []TaskObservation
}

// JobObservation is one export-job row as a scenario observed it: the final
// lifecycle state after the scenario ran (status, recorded file path, error
// message), its scope (tenant), and its audit fields. Timestamps are
// deliberately absent: the legacy stack has no injectable clock for job-row
// writes, so the gate compares the transition outcome, which the fixtures
// make time-independent.
type JobObservation struct {
	ID           uint64
	TenantID     uint64
	RequestedBy  string
	Status       string
	FilePath     string
	ErrorMessage string
}

// TaskObservation is one enqueued async task. Type/Queue/MaxRetry/Timeout are
// the task's processing options (significant — the enqueue contract), Payload
// is the exact marshaled payload (byte-significant). The broker-assigned task
// id is not recorded: it is generated per enqueue and carries no business
// meaning.
type TaskObservation struct {
	Type     string
	Queue    string
	MaxRetry int
	Timeout  time.Duration
	Payload  []byte
}

// ClassPermanent is the shared semantic error class of the legacy
// asynq.SkipRetry sentinel and the module's domain.ErrPermanentPayload: a
// task payload that can never succeed on retry. It is the ONE error-class
// equivalence the gate allows.
const ClassPermanent = "permanent"

// transportOnlyHeaders is the complete header normalization allowlist: the
// four headers the transport layer generates per response. Keys are in
// canonical MIME form; NormalizeHTTP canonicalizes before consulting it.
var transportOnlyHeaders = map[string]bool{
	// Date is stamped by the HTTP server per response.
	"Date": true,
	// X-Request-Id is the per-request correlation id (middleware.RequestID).
	"X-Request-Id": true,
	// X-Trace-Id echoes the per-request trace id.
	"X-Trace-Id": true,
	// X-Response-Time is generated transport timing.
	"X-Response-Time": true,
}

// NormalizeError maps an error onto its comparison class. nil is the empty
// class; SkipRetry and ErrPermanentPayload (possibly wrapped — both stacks
// wrap them with context) collapse to ClassPermanent; every other error keeps
// its exact text, so any other divergence stays a mismatch.
func NormalizeError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, asynq.SkipRetry) || errors.Is(err, domain.ErrPermanentPayload) {
		return ClassPermanent
	}
	return err.Error()
}

// ObserveHTTP captures an HTTP response as an Observation.
func ObserveHTTP(status int, header http.Header, body []byte) Observation {
	headers := make(map[string][]string, len(header))
	for key, values := range header {
		headers[key] = append([]string(nil), values...)
	}
	return Observation{Status: status, Headers: headers, Body: append([]byte(nil), body...)}
}

// NormalizeHTTP applies the allowlisted normalizations to an observation:
// drops the four transport-only headers (after key canonicalization), rewrites
// a JSON body with sorted object keys (recursively — array order and number
// literals are preserved), and sorts Jobs by ID and Enqueued tasks by their
// stable type+payload identity. Nothing else is touched; the input is not
// mutated.
func NormalizeHTTP(in Observation) Observation {
	out := in

	headers := make(map[string][]string, len(in.Headers))
	for key, values := range in.Headers {
		canonical := textproto.CanonicalMIMEHeaderKey(key)
		if transportOnlyHeaders[canonical] {
			continue
		}
		sorted := append([]string(nil), values...)
		sort.Strings(sorted)
		headers[canonical] = sorted
	}
	out.Headers = headers

	if normalized, ok := normalizeJSON(in.Body); ok {
		out.Body = normalized
	} else {
		out.Body = append([]byte(nil), in.Body...)
	}

	jobs := append([]JobObservation(nil), in.Jobs...)
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	out.Jobs = jobs

	tasks := append([]TaskObservation(nil), in.Enqueued...)
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Type != tasks[j].Type {
			return tasks[i].Type < tasks[j].Type
		}
		return bytes.Compare(tasks[i].Payload, tasks[j].Payload) < 0
	})
	out.Enqueued = tasks

	files := make(map[string][]byte, len(in.StoredFiles))
	for path, data := range in.StoredFiles {
		files[path] = append([]byte(nil), data...)
	}
	out.StoredFiles = files

	return out
}

// Compare proves two observations are identical after normalization. It
// returns nil only on full equivalence; any divergence comes back as an
// error naming the exact field (Status, Headers[...], Body, ErrorClass,
// Jobs[i].<field>, StoredFiles[path], Enqueued[i].<field>). It never ignores
// a mismatch.
func Compare(want, got Observation) error {
	w := NormalizeHTTP(want)
	g := NormalizeHTTP(got)

	if w.Status != g.Status {
		return fmt.Errorf("Status: want %d, got %d", w.Status, g.Status)
	}

	if err := compareHeaders(w.Headers, g.Headers); err != nil {
		return err
	}

	if !bytes.Equal(w.Body, g.Body) {
		return fmt.Errorf("Body: want %q, got %q", preview(w.Body), preview(g.Body))
	}

	if w.ErrorClass != g.ErrorClass {
		return fmt.Errorf("ErrorClass: want %q, got %q", w.ErrorClass, g.ErrorClass)
	}

	if len(w.Jobs) != len(g.Jobs) {
		return fmt.Errorf("Jobs: want %d entries, got %d", len(w.Jobs), len(g.Jobs))
	}
	for i := range w.Jobs {
		if err := compareJob(i, w.Jobs[i], g.Jobs[i]); err != nil {
			return err
		}
	}

	if err := compareFiles(w.StoredFiles, g.StoredFiles); err != nil {
		return err
	}

	if len(w.Enqueued) != len(g.Enqueued) {
		return fmt.Errorf("Enqueued: want %d tasks, got %d", len(w.Enqueued), len(g.Enqueued))
	}
	for i := range w.Enqueued {
		if err := compareTask(i, w.Enqueued[i], g.Enqueued[i]); err != nil {
			return err
		}
	}

	return nil
}

// normalizeJSON re-renders a valid JSON document with object keys sorted,
// recursively. Array element order, string contents, and number literals
// (json.Number) are preserved exactly; any document that is not a single
// valid JSON value answers ok=false and stays byte-significant.
func normalizeJSON(body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if decoder.More() {
		return nil, false
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, false
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), true
}

// compareHeaders diffs two normalized header maps, naming the header key.
func compareHeaders(want, got map[string][]string) error {
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		wantValues := want[key]
		gotValues, present := got[key]
		if !present {
			return fmt.Errorf("Headers[%s]: want %q, got missing", key, wantValues)
		}
		if fmt.Sprint(wantValues) != fmt.Sprint(gotValues) {
			return fmt.Errorf("Headers[%s]: want %q, got %q", key, wantValues, gotValues)
		}
	}
	extra := make([]string, 0)
	for key := range got {
		if _, ok := want[key]; !ok {
			extra = append(extra, key)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return fmt.Errorf("Headers: want no %q, got %q", extra, got[extra[0]])
	}
	return nil
}

// compareJob diffs one job observation field by field.
func compareJob(index int, want, got JobObservation) error {
	if want.ID != got.ID {
		return fmt.Errorf("Jobs[%d].ID: want %d, got %d", index, want.ID, got.ID)
	}
	if want.TenantID != got.TenantID {
		return fmt.Errorf("Jobs[%d].TenantID: want %d, got %d", index, want.TenantID, got.TenantID)
	}
	if want.RequestedBy != got.RequestedBy {
		return fmt.Errorf("Jobs[%d].RequestedBy: want %q, got %q", index, want.RequestedBy, got.RequestedBy)
	}
	if want.Status != got.Status {
		return fmt.Errorf("Jobs[%d].Status: want %q, got %q", index, want.Status, got.Status)
	}
	if want.FilePath != got.FilePath {
		return fmt.Errorf("Jobs[%d].FilePath: want %q, got %q", index, want.FilePath, got.FilePath)
	}
	if want.ErrorMessage != got.ErrorMessage {
		return fmt.Errorf("Jobs[%d].ErrorMessage: want %q, got %q", index, want.ErrorMessage, got.ErrorMessage)
	}
	return nil
}

// compareTask diffs one enqueued-task observation field by field.
func compareTask(index int, want, got TaskObservation) error {
	if want.Type != got.Type {
		return fmt.Errorf("Enqueued[%d].Type: want %q, got %q", index, want.Type, got.Type)
	}
	if want.Queue != got.Queue {
		return fmt.Errorf("Enqueued[%d].Queue: want %q, got %q", index, want.Queue, got.Queue)
	}
	if want.MaxRetry != got.MaxRetry {
		return fmt.Errorf("Enqueued[%d].MaxRetry: want %d, got %d", index, want.MaxRetry, got.MaxRetry)
	}
	if want.Timeout != got.Timeout {
		return fmt.Errorf("Enqueued[%d].Timeout: want %s, got %s", index, want.Timeout, got.Timeout)
	}
	if !bytes.Equal(want.Payload, got.Payload) {
		return fmt.Errorf("Enqueued[%d].Payload: want %q, got %q",
			index, preview(want.Payload), preview(got.Payload))
	}
	return nil
}

// compareFiles diffs two stored-file maps, naming the path.
func compareFiles(want, got map[string][]byte) error {
	paths := make([]string, 0, len(want))
	for path := range want {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		gotData, present := got[path]
		if !present {
			return fmt.Errorf("StoredFiles[%s]: want %d bytes, got missing", path, len(want[path]))
		}
		if !bytes.Equal(want[path], gotData) {
			return fmt.Errorf("StoredFiles[%s]: want %q, got %q",
				path, preview(want[path]), preview(gotData))
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			return fmt.Errorf("StoredFiles: want no %s, got %d bytes", path, len(got[path]))
		}
	}
	return nil
}

// preview caps a byte dump in error messages at 256 bytes.
func preview(data []byte) []byte {
	const limit = 256
	if len(data) <= limit {
		return data
	}
	return append(append([]byte(nil), data[:limit]...), []byte("...")...)
}
