package opencode

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"time"
)

// Message is the stable subset of the pinned OpenCode 1.18.4 message
// projection consumed by Craft.
type Message struct {
	ID          string
	ParentID    string
	Role        string
	Finish      string
	CompletedAt int64
	Parts       []json.RawMessage
}

var (
	messageIDPattern = regexp.MustCompile(`^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`)
	messageIDMu      sync.Mutex
	lastMessageMS    int64
	messageCounter   uint16
)

// NewMessageID reproduces OpenCode 1.18.4's ascending message ID ordering
// (source commit 49c69c5ed3ccf706b61b3febb43c8aaff7f8325e: 48-bit
// millisecond timestamp plus counter packed into six bytes, hex encoded,
// followed by 14 base62 random characters; IDs sort lexicographically in
// submission order within the same process).
func NewMessageID() (string, error) {
	return newMessageIDAt(time.Now().UnixMilli())
}

func newMessageIDAt(timestampMS int64) (string, error) {
	messageIDMu.Lock()
	defer messageIDMu.Unlock()
	if timestampMS != lastMessageMS {
		lastMessageMS = timestampMS
		messageCounter = 0
	}
	messageCounter++
	value := uint64(timestampMS)*0x1000 + uint64(messageCounter)
	timeBytes := []byte{byte(value >> 40), byte(value >> 32), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
	random := make([]byte, 14)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for index := range random {
		random[index] = base62[int(random[index])%len(base62)]
	}
	return "msg_" + hex.EncodeToString(timeBytes) + string(random), nil
}

func validMessageID(id string) bool {
	return messageIDPattern.MatchString(id)
}

// DecodeParts rejects every shape except a JSON array of non-null objects.
func DecodeParts(raw []byte) ([]json.RawMessage, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, err
	}
	if parts == nil {
		return nil, errors.New("parts must be an array")
	}
	for _, part := range parts {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(part, &object); err != nil || object == nil {
			return nil, errors.New("part must be an object")
		}
		if !bytes.HasPrefix(bytes.TrimSpace(part), []byte("{")) {
			return nil, errors.New("part must be an object")
		}
	}
	return parts, nil
}
