package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/workbench/voice"
	"github.com/stretchr/testify/require"
)

// TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession is the
// server-side evidence for Issue #57 AC1 ("断线有明确结束/恢复"): the explicit
// end after a disconnect is the idempotent stop/settle replay (a late second
// stop answers replay:true and never re-charges), resuming opens a NEW
// session row under the same product session id (only `unknown` rows block
// re-authorization — closed rows never do), and the closed session refuses
// new bound transcriptions while the resumed one serves them.
func TestVoiceRoomDisconnectEndsClearlyThenResumesWithNewSession(t *testing.T) {
	h := newVoiceHarness(t)
	// Room session A (product session "room-1").
	w := h.postSession(t, `{"session_id":"room-1","max_seconds":30,"run_id":"run-1"}`)
	require.Equal(t, http.StatusOK, w.Code)
	idA := voiceSessionIDOf(t, w)
	// The disconnect's explicit end settles the session.
	w = h.stopSession(t, idA, "owner-a")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"settled":true`)
	// A late replay of the same stop (flaky client reconnect) is idempotent.
	w = h.stopSession(t, idA, "owner-a")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"replay":true`)
	// Resuming re-authorizes a NEW session under the same product session id.
	// The server mints the row id from the wall clock (UnixNano + digest), so
	// the harness advances its frozen clock — in production two creates are
	// never the same instant; the frozen-clock replay branch (writeExisting)
	// is the same-(tenant,id) idempotency identity, which a resume never hits.
	h.clock = h.clock.Add(2 * time.Second)
	w = h.postSession(t, `{"session_id":"room-1","max_seconds":30,"run_id":"run-1"}`)
	require.Equal(t, http.StatusOK, w.Code)
	idB := voiceSessionIDOf(t, w)
	require.NotEqual(t, idA, idB, "resume must be a fresh session row, not the closed one")
	// The resumed session serves bound transcriptions; the closed one refuses.
	h.transcriber.result = voice.TranscriptionResult{Text: "resumed turn", AudioSeconds: 1}
	w = h.postTranscription(t, "owner-a", "req-resumed", idB, []byte("audio"))
	require.Equal(t, http.StatusOK, w.Code)
	w = h.postTranscription(t, "owner-a", "req-stale", idA, []byte("audio"))
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), "voice_session_closed")
	// Exactly-once charging across the whole room lifecycle: only the
	// resumed transcription's 1 second settled (session A settled 0 seconds
	// at stop; the stale bound attempt never ran).
	require.Equal(t, int64(1), h.gate.totalSettledAudioSeconds())
}
