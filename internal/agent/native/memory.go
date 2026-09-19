// Package native provides controlled SDK facades for the native Agent path.
package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

var (
	ErrMemoryScopeDenied       = errors.New("native memory scope denied")
	ErrMemoryWriteDenied       = errors.New("native memory write denied")
	ErrMemorySearchUnsupported = errors.New("native memory search option unsupported")
)

// MemoryWrite is a bounded extractor output. The job stores only its source
// boundary; it never persists a transcript for a later unrestricted reread.
type MemoryWrite struct {
	ID, Content string
	Metadata    map[string]any
}
type MemoryExtractor func(context.Context, nativecontract.MemoryJob) ([]MemoryWrite, error)

// MemoryService is the selected SDK memory.Service facade. The repository is
// authoritative for permission, generation and tombstone checks; the optional
// backend is a projection only and is never called for a rejected stale job.
type MemoryService struct {
	resolver  nativecontract.ScopeResolver
	repo      *repository.NativeMemoryRepository
	backend   memory.Service
	extractor MemoryExtractor
}

var _ memory.Service = (*MemoryService)(nil)
var _ nativecontract.MemoryGovernance = (*MemoryService)(nil)

func NewMemoryService(resolver nativecontract.ScopeResolver, repo *repository.NativeMemoryRepository, backend memory.Service) *MemoryService {
	return &MemoryService{resolver: resolver, repo: repo, backend: backend}
}
func (s *MemoryService) SetExtractor(extractor MemoryExtractor) { s.extractor = extractor }
func AcceptMemoryWrite(enabled bool, currentGeneration, jobGeneration int64) bool {
	return enabled && currentGeneration == jobGeneration
}

func (s *MemoryService) scope(ctx context.Context) (nativecontract.Scope, error) {
	if s.resolver == nil {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	scope, err := s.resolver.Resolve(ctx)
	if err != nil {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	return s.resolver.Recheck(ctx, scope, nil)
}
func (s *MemoryService) authorize(ctx context.Context, scope nativecontract.Scope) (nativecontract.Scope, error) {
	fresh, err := s.scope(ctx)
	if err != nil {
		return nativecontract.Scope{}, err
	}
	if fresh.TenantID != scope.TenantID || fresh.MemorySubjectID != scope.MemorySubjectID {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	return fresh, nil
}
func matches(scope nativecontract.Scope, key memory.UserKey) bool {
	expected, err := nativecontract.MemoryKey(scope)
	return err == nil && expected == key
}
func (s *MemoryService) keyScope(ctx context.Context, key memory.UserKey) (nativecontract.Scope, error) {
	scope, err := s.scope(ctx)
	if err != nil || !matches(scope, key) {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	return scope, nil
}

func (s *MemoryService) SetEnabled(ctx context.Context, scope nativecontract.Scope, enabled bool) error {
	scope, err := s.authorize(ctx, scope)
	if err != nil {
		return err
	}
	if err := s.repo.EnsureScope(ctx, scope); err != nil {
		return err
	}
	return s.repo.SetEnabled(ctx, scope, enabled)
}
func (s *MemoryService) Delete(ctx context.Context, scope nativecontract.Scope, id string) error {
	scope, err := s.authorize(ctx, scope)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, scope, id)
}
func (s *MemoryService) Clear(ctx context.Context, scope nativecontract.Scope) error {
	scope, err := s.authorize(ctx, scope)
	if err != nil {
		return err
	}
	return s.repo.Clear(ctx, scope)
}
func (s *MemoryService) Enqueue(ctx context.Context, job nativecontract.MemoryJob) error {
	scope, err := s.authorize(ctx, job.Scope)
	if err != nil {
		return err
	}
	job.Scope = scope
	return s.repo.Enqueue(ctx, job)
}
func (s *MemoryService) Execute(ctx context.Context, job nativecontract.MemoryJob) error {
	scope, err := s.authorize(ctx, job.Scope)
	if err != nil {
		return errors.Join(err, s.repo.Discard(ctx, job))
	}
	job.Scope = scope
	state, err := s.repo.State(ctx, scope)
	if err != nil {
		return err
	}
	if !AcceptMemoryWrite(state.Enabled, state.Generation, job.Generation) || state.PolicyRevision != job.PolicyRevision {
		_, err = s.repo.CommitWrites(ctx, job, []repository.NativeMemoryEntry{{ID: "discard", Content: "discard"}})
		return err
	}
	if s.extractor == nil {
		return nil
	}
	writes, err := s.extractor(ctx, job)
	if err != nil {
		return errors.Join(err, s.repo.Fail(ctx, job))
	}
	// Re-resolve after extraction. A worker can spend meaningful time outside
	// the transaction; its initial authorization is never a commit permit.
	fresh, err := s.authorize(ctx, job.Scope)
	if err != nil {
		return errors.Join(err, s.repo.Discard(ctx, job))
	}
	job.Scope = fresh
	entries := make([]repository.NativeMemoryEntry, 0, len(writes))
	for _, write := range writes {
		entries = append(entries, repository.NativeMemoryEntry{ID: write.ID, Content: write.Content, Metadata: write.Metadata})
	}
	_, err = s.repo.CommitWrites(ctx, job, entries)
	if err != nil {
		return errors.Join(err, s.repo.Fail(ctx, job))
	}
	return err
}

func (s *MemoryService) ReadMemories(ctx context.Context, key memory.UserKey, limit int) ([]*memory.Entry, error) {
	scope, err := s.keyScope(ctx, key)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.Read(ctx, scope, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*memory.Entry, 0, len(entries))
	for _, entry := range entries {
		m := &memory.Memory{Memory: entry.Content}
		if topics, ok := entry.Metadata["topics"].([]any); ok {
			for _, topic := range topics {
				if text, ok := topic.(string); ok {
					m.Topics = append(m.Topics, text)
				}
			}
		}
		if kind, ok := entry.Metadata["kind"].(string); ok {
			m.Kind = memory.Kind(kind)
		}
		if location, ok := entry.Metadata["location"].(string); ok {
			m.Location = location
		}
		if participants, ok := entry.Metadata["participants"].([]any); ok {
			for _, person := range participants {
				if text, ok := person.(string); ok {
					m.Participants = append(m.Participants, text)
				}
			}
		}
		if eventTime, ok := entry.Metadata["event_time"].(string); ok {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, eventTime); parseErr == nil {
				m.EventTime = &parsed
			}
		}
		out = append(out, &memory.Entry{ID: entry.ID, AppName: key.AppName, UserID: key.UserID, Memory: m})
	}
	return out, nil
}
func (s *MemoryService) SearchMemories(ctx context.Context, key memory.UserKey, query string, opts ...memory.SearchOption) ([]*memory.Entry, error) {
	options := memory.ResolveSearchOptions(query, opts)
	if options.TimeAfter != nil || options.TimeBefore != nil || options.OrderByEventTime || options.KindFallback || options.Deduplicate || options.HybridSearch || options.SimilarityThreshold != 0 || options.HybridRRFK != 0 {
		return nil, ErrMemorySearchUnsupported
	}
	limit := options.MaxResults
	if limit <= 0 {
		limit = 100
	}
	entries, err := s.ReadMemories(ctx, key, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*memory.Entry, 0, len(entries))
	for _, entry := range entries {
		if (options.Kind == "" || entry.Memory.Kind == options.Kind) && strings.Contains(strings.ToLower(entry.Memory.Memory), strings.ToLower(query)) {
			out = append(out, entry)
		}
	}
	return out, nil
}
func memoryID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func memoryMetadata(topics []string, metadata *memory.Metadata) map[string]any {
	out := map[string]any{"topics": topics}
	if metadata == nil {
		return out
	}
	out["kind"] = string(metadata.Kind)
	out["location"] = metadata.Location
	out["participants"] = metadata.Participants
	if metadata.EventTime != nil {
		out["event_time"] = metadata.EventTime.UTC().Format(time.RFC3339Nano)
	}
	return out
}
func (s *MemoryService) AddMemory(ctx context.Context, key memory.UserKey, value string, topics []string, opts ...memory.AddOption) error {
	scope, err := s.keyScope(ctx, key)
	if err != nil {
		return err
	}
	if err := s.repo.EnsureScope(ctx, scope); err != nil {
		return err
	}
	state, err := s.repo.State(ctx, scope)
	if err != nil {
		return err
	}
	if !AcceptMemoryWrite(state.Enabled, state.Generation, state.Generation) {
		return ErrMemoryWriteDenied
	}
	if err := s.repo.Write(ctx, scope, state.Generation, memoryID(value), value, memoryMetadata(topics, memory.ResolveAddOptions(opts))); err != nil {
		if errors.Is(err, repository.ErrNativeMemoryWriteRejected) {
			return ErrMemoryWriteDenied
		}
		return err
	}
	return nil
}
func (s *MemoryService) UpdateMemory(ctx context.Context, key memory.Key, value string, topics []string, opts ...memory.UpdateOption) error {
	scope, err := s.keyScope(ctx, memory.UserKey{AppName: key.AppName, UserID: key.UserID})
	if err != nil {
		return err
	}
	state, err := s.repo.State(ctx, scope)
	if err != nil {
		return err
	}
	newID := memoryID(value)
	if err := s.repo.Replace(ctx, scope, state.Generation, key.MemoryID, repository.NativeMemoryEntry{ID: newID, Content: value, Metadata: memoryMetadata(topics, memory.ResolveUpdateOptions(opts))}); err != nil {
		if errors.Is(err, repository.ErrNativeMemoryWriteRejected) {
			return ErrMemoryWriteDenied
		}
		return err
	}
	if result := memory.ResolveUpdateResult(opts); result != nil {
		result.MemoryID = newID
	}
	return nil
}
func (s *MemoryService) DeleteMemory(ctx context.Context, key memory.Key) error {
	scope, err := s.keyScope(ctx, memory.UserKey{AppName: key.AppName, UserID: key.UserID})
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, scope, key.MemoryID)
}
func (s *MemoryService) ClearMemories(ctx context.Context, key memory.UserKey) error {
	scope, err := s.keyScope(ctx, key)
	if err != nil {
		return err
	}
	return s.repo.Clear(ctx, scope)
}
func (s *MemoryService) Tools() []tool.Tool {
	// SDK tools do not carry the server-resolved scope required by this
	// facade. Exposing them would bypass authorization, so native wiring must
	// call the scoped methods above instead of passing raw backend tools on.
	return nil
}
func (s *MemoryService) EnqueueAutoMemoryJob(ctx context.Context, sess *session.Session) error {
	if sess == nil {
		return errors.New("session is required")
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return err
	}
	state, err := s.repo.State(ctx, scope)
	if err != nil {
		return err
	}
	return s.Enqueue(ctx, nativecontract.MemoryJob{ID: memoryID(sess.AppName + "\x00" + sess.UserID + "\x00" + sess.ID), Scope: scope, SessionKey: session.Key{AppName: sess.AppName, UserID: sess.UserID, SessionID: sess.ID}, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: sess.ID})
}
func (s *MemoryService) Close() error {
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}
