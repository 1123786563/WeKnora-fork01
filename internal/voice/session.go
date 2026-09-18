// Package voice holds the W30 voice-session authorization domain: the pure
// admission rule every charged voice session must pass, and the provider
// boundary that mints short-lived tokens.
//
// Non-negotiable invariants:
//   - A grant NEVER outlives the task deadline (min of the one-minute grant
//     window and the deadline), and an unfunded or already-expired request
//     is refused outright — no silent zero-cost fallback exists.
//   - Provider long-lived keys live ONLY on the server. Providers that
//     cannot mint short-lived tokens get the server-side media-proxy form
//     (an HMAC ticket the product signs itself); the key never reaches the
//     phone in any grant, response, or log line.
//   - A timed-out provider open is UNKNOWN (the provider may have opened a
//     billable session): it surfaces as ErrProviderOpenUnknown so callers
//     record-and-reconcile instead of opening a second charged session.
package voice

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

var (
	// ErrUnfundedVoice reports a voice request whose budget admission
	// produced no positive spendable amount.
	ErrUnfundedVoice = errors.New("voice budget must be positive")
	// ErrVoiceDeadlinePassed reports a voice request whose deadline has
	// already elapsed; expired quota never regains validity.
	ErrVoiceDeadlinePassed = errors.New("voice deadline already passed")
)

// GrantWindow caps how long ONE voice grant lives, independent of the
// task deadline: a session must re-authorize rather than hold a token
// for the whole task.
const GrantWindow = time.Minute

// Grant is the short-lived credential handed to the phone for one voice
// session. Token is the ONLY secret in the structure; it is never the
// provider's long-lived key.
type Grant struct {
	Token      string
	ExpiresAt  time.Time
	MaxSeconds int
}

// AuthorizeVoice is the pure admission rule of one voice session: it
// returns the instant the grant must expire — the earlier of now+GrantWindow
// and the task deadline. budget<=0 or a deadline at/after now refuses;
// the returned expiry is always strictly after now and never after
// deadline.
func AuthorizeVoice(now, deadline time.Time, budget int64) (time.Time, error) {
	if budget <= 0 {
		return time.Time{}, ErrUnfundedVoice
	}
	if !deadline.After(now) {
		return time.Time{}, ErrVoiceDeadlinePassed
	}
	expiry := now.Add(GrantWindow)
	if expiry.After(deadline) {
		expiry = deadline
	}
	return expiry, nil
}

// SessionRef is the stable provider-side mapping identity of one product
// voice session: the provider never learns more than this composed string.
func SessionRef(tenantID uint64, ownerID, sessionID string) string {
	return "tenant/" + strconv.FormatUint(tenantID, 10) + "/owner/" + ownerID + "/voice/" + sessionID
}

// SessionCallID derives the commercial call identity of one voice session's
// consumption: server-derived (tenant-bound), so a settlement replay keys
// idempotently while another tenant reusing the same session id never
// collides.
func SessionCallID(tenantID uint64, sessionID string) string {
	sum := sha256.Sum256([]byte("voice-session|" + strconv.FormatUint(tenantID, 10) + "|" + sessionID))
	return "voice_sess_" + hex.EncodeToString(sum[:12])
}

// TranscriptionCallID derives the call identity of one proxied
// transcription from its client-supplied idempotency key: the same
// request_id replayed settles once; a new request_id is a new charged call.
func TranscriptionCallID(tenantID uint64, requestID string) string {
	sum := sha256.Sum256([]byte("voice-transcribe|" + strconv.FormatUint(tenantID, 10) + "|" + requestID))
	return "voice_tx_" + hex.EncodeToString(sum[:12])
}
