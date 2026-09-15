package workbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxSafeInteger int64 = 9007199254740991

type CapabilityState string

const (
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnavailable CapabilityState = "unavailable"
	CapabilityForbidden   CapabilityState = "forbidden"
)

type Capability struct {
	State  CapabilityState `json:"state"`
	Reason string          `json:"reason"`
}

type ExecutionDTO struct {
	SchemaVersion    int                   `json:"schema_version"`
	RunID            string                `json:"run_id"`
	SessionID        string                `json:"session_id"`
	Revision         int64                 `json:"revision"`
	Driver           string                `json:"driver"`
	RunStatus        string                `json:"run_status"`
	ExecutionStatus  string                `json:"execution_status"`
	SettlementStatus string                `json:"settlement_status"`
	Seq              int64                 `json:"seq"`
	Capabilities     map[string]Capability `json:"capabilities"`
}

type ExecutionEvent struct {
	SchemaVersion int64           `json:"schema_version"`
	RunID         string          `json:"run_id"`
	AttemptID     string          `json:"attempt_id"`
	Seq           int64           `json:"seq"`
	Type          string          `json:"type"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

type ExecutionSnapshot struct {
	Execution ExecutionDTO     `json:"execution"`
	Watermark int64            `json:"watermark"`
	Events    []ExecutionEvent `json:"events"`
}

var iso8601 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`)

func invalid(path, message string) error { return fmt.Errorf("%s: %s", path, message) }

func (c Capability) Validate() error {
	if c.State != CapabilitySupported && c.State != CapabilityUnavailable && c.State != CapabilityForbidden {
		return invalid("capability.state", "unknown capability state")
	}
	if c.State != CapabilitySupported && strings.TrimSpace(c.Reason) == "" {
		return invalid("capability.reason", "reason is required unless supported")
	}
	return nil
}

func validRunStatus(value string) bool {
	switch value {
	case "queued", "running", "waiting_user", "reconciling", "succeeded", "failed", "canceled":
		return true
	default:
		return false
	}
}

func (e ExecutionDTO) Validate() error {
	if e.SchemaVersion != 1 {
		return invalid("schema_version", "expected 1")
	}
	if strings.TrimSpace(e.RunID) == "" {
		return invalid("run_id", "required")
	}
	if strings.TrimSpace(e.SessionID) == "" {
		return invalid("session_id", "required")
	}
	if e.Revision < 0 || e.Revision > MaxSafeInteger {
		return fmt.Errorf("%w: revision", ErrSequenceOverflow)
	}
	if e.Driver != "platform" && e.Driver != "paseo" {
		return invalid("driver", "unknown execution driver")
	}
	if !validRunStatus(e.RunStatus) {
		return invalid("run_status", "unknown run status")
	}
	if strings.TrimSpace(e.ExecutionStatus) == "" {
		return invalid("execution_status", "required")
	}
	if strings.TrimSpace(e.SettlementStatus) == "" {
		return invalid("settlement_status", "required")
	}
	if e.Seq < 0 || e.Seq > MaxSafeInteger {
		return fmt.Errorf("%w: seq", ErrSequenceOverflow)
	}
	if e.Capabilities == nil {
		return invalid("capabilities", "required")
	}
	for name, capability := range e.Capabilities {
		if err := capability.Validate(); err != nil {
			return fmt.Errorf("capabilities.%s: %w", name, err)
		}
	}
	return nil
}

func (e ExecutionEvent) Validate() error {
	if e.SchemaVersion != 1 {
		return invalid("schema_version", "expected 1")
	}
	if strings.TrimSpace(e.RunID) == "" {
		return invalid("run_id", "required")
	}
	if e.Seq < 1 || e.Seq > MaxSafeInteger {
		return fmt.Errorf("%w: seq", ErrSequenceOverflow)
	}
	if strings.TrimSpace(e.Type) == "" {
		return invalid("type", "required")
	}
	if !iso8601.MatchString(e.OccurredAt) {
		return invalid("occurred_at", "expected ISO-8601 timestamp")
	}
	if e.OccurredAt[len(e.OccurredAt)-1] != 'Z' {
		offset := e.OccurredAt[len(e.OccurredAt)-6:]
		offsetHour, hourErr := strconv.Atoi(offset[1:3])
		offsetMinute, minuteErr := strconv.Atoi(offset[4:6])
		if hourErr != nil || minuteErr != nil || offsetHour > 23 || offsetMinute > 59 {
			return invalid("occurred_at", "expected ISO-8601 timestamp")
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, e.OccurredAt); err != nil {
		return invalid("occurred_at", "expected ISO-8601 timestamp")
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return invalid("payload", "invalid JSON")
	}
	trimmed := bytes.TrimSpace(e.Payload)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return invalid("payload", "expected an object")
	}
	return nil
}

func (s ExecutionSnapshot) Validate() error {
	if err := s.Execution.Validate(); err != nil {
		return fmt.Errorf("execution: %w", err)
	}
	if s.Watermark < 0 || s.Watermark > MaxSafeInteger {
		return fmt.Errorf("%w: watermark", ErrSequenceOverflow)
	}
	for index, event := range s.Events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("events[%d]: %w", index, err)
		}
		if event.RunID != s.Execution.RunID {
			return invalid(fmt.Sprintf("events[%d].run_id", index), "must match execution.run_id")
		}
	}
	return nil
}

func ParseExecutionEvent(raw []byte) (ExecutionEvent, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ExecutionEvent{}, err
	}
	attemptRaw, ok := fields["attempt_id"]
	if !ok || bytes.Equal(bytes.TrimSpace(attemptRaw), []byte("null")) {
		return ExecutionEvent{}, invalid("attempt_id", "required")
	}
	if len(attemptRaw) == 0 || attemptRaw[0] != '"' {
		return ExecutionEvent{}, invalid("attempt_id", "expected a string")
	}
	var event ExecutionEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return event, err
	}
	if err := event.Validate(); err != nil {
		return event, err
	}
	return event, nil
}

var ErrSequenceOverflow = errors.New("sequence_overflow")
