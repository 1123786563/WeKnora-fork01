package craftegress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestJournalReplayToleratesTornTail is the OCR medium-finding regression: a
// crash between Write and the newline reaching the disk leaves one torn
// record at the end of the append-only journal; replay must truncate it and
// start instead of refusing (log.Fatal) and blocking the Run's model
// egress. A corrupted line in the MIDDLE still refuses to start.
func TestJournalReplayToleratesTornTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "attempts.jsonl")
	good, err := json.Marshal(CraftEgressAttemptRecord{Ordinal: 1, AttemptID: "aeg-1", RequestDigest: "d1", State: CraftEgressAttemptUnresolved, CreatedNano: 1})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(append([]byte{}, good...), []byte("\n{\"ordinal\":2,\"attempt_id\":\"aeg-2\",\"requ")...), 0o600))

	journal, err := OpenCraftEgressAttemptJournal(path)
	require.NoError(t, err, "a torn trailing record must be truncated, not fatal")
	rec, ok := journal.Reuse("d1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "aeg-1", rec.AttemptID)
	require.NoError(t, journal.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(data), "\n"), "the journal ends with a clean record separator after truncation")

	bad := filepath.Join(dir, "bad.jsonl")
	require.NoError(t, os.WriteFile(bad, []byte("not-json\n"+string(good)+"\n"), 0o600))
	_, err = OpenCraftEgressAttemptJournal(bad)
	require.Error(t, err, "interior corruption must refuse replay, never rewrite history")
}

// TestJournalAllocateIfNotParkedIsAtomic is the OCR medium-finding
// regression: concurrent same-fingerprint requests must never mint two
// identities — the single-lock check-and-mint keeps the "at most one parked
// identity per fingerprint" protocol invariant.
func TestJournalAllocateIfNotParkedIsAtomic(t *testing.T) {
	journal, err := OpenCraftEgressAttemptJournal(filepath.Join(t.TempDir(), "attempts.jsonl"))
	require.NoError(t, err)
	defer func() { _ = journal.Close() }()

	const racers = 16
	var mu sync.Mutex
	ids := map[string]int{}
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, _, err := journal.AllocateIfNotParked("digest-race")
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			ids[record.AttemptID]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.Len(t, ids, 1, "all racers must share one parked identity")
}

// driveAdapterOnce runs one POST through the adapter against a mock gateway
// answering status/body, then reports the LAST journal record.
func driveAdapterOnce(t *testing.T, status int, body string) CraftEgressAttemptRecord {
	t.Helper()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer gateway.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter, err := NewCraftEgressAdapter(CraftEgressAdapterConfig{
		GatewayBaseURL: gateway.URL, Credential: "cred", JournalPath: path,
		AllowPrivateTarget: true, ForwardTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = adapter.Close() }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/x", strings.NewReader(`{"q":1}`))
	adapter.ServeHTTP(rec, req)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "allocate + resolve records exist")
	var last CraftEgressAttemptRecord
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &last))
	return last
}

// TestAdapterParksOnlyOnActivityUnresolvedCode covers the unresolved envelope
// and preserves the non-5xx 409 upstream passthrough behavior. Any 5xx parks
// because its JSON body cannot prove which hop produced the response.
func TestAdapterParksOnlyOnActivityUnresolvedCode(t *testing.T) {
	parked := driveAdapterOnce(t, http.StatusBadGateway, `{"error":{"code":"ACTIVITY_UNRESOLVED"}}`)
	require.Equal(t, CraftEgressAttemptUnresolved, parked.State,
		"a 502 ACTIVITY_UNRESOLVED body parks the attempt")

	definitive := driveAdapterOnce(t, http.StatusConflict, `{"error":"upstream state conflict"}`)
	require.Equal(t, CraftEgressAttemptResolved, definitive.State,
		"a 409 without the ACTIVITY_UNRESOLVED code is a definitive upstream outcome")

	plain502 := driveAdapterOnce(t, http.StatusBadGateway, `{"error":{"code":"UPSTREAM_ERROR"}}`)
	require.Equal(t, CraftEgressAttemptUnresolved, plain502.State,
		"an appFail-shaped 502 cannot authenticate its origin and must keep the attempt parked")
}

var _ = context.Background
