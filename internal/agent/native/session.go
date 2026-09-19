// Package native provides controlled facades around the native agent SDK seams.
package native

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const (
	StableAppendInsert = "insert"
	StableAppendReuse  = "reuse"
)

func StableAppendAction(existingHash, incomingHash string, exists bool) (string, error) {
	if !exists {
		return StableAppendInsert, nil
	}
	if existingHash == incomingHash {
		return StableAppendReuse, nil
	}
	return "", &nativecontract.Failure{Code: nativecontract.ErrConflict, Message: "stable event payload changed", Effect: nativecontract.EffectNotDispatched}
}

type scopeContextKey struct{}

func WithScope(ctx context.Context, scope nativecontract.Scope) context.Context {
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

type SessionService struct {
	store  *repository.NativeSessionStore
	scopes nativecontract.ScopeResolver
}

func NewSessionService(store *repository.NativeSessionStore, scopes nativecontract.ScopeResolver) *SessionService {
	return &SessionService{store: store, scopes: scopes}
}
func (s *SessionService) authorize(ctx context.Context, key session.Key) error {
	scope, ok := ctx.Value(scopeContextKey{}).(nativecontract.Scope)
	if !ok {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "missing server scope"}
	}
	actual, err := s.scopes.Recheck(ctx, scope, nil)
	if err != nil {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "scope is no longer authorized"}
	}
	expected, err := nativecontract.SessionKey(actual, key.SessionID)
	if err != nil || expected.AppName != key.AppName || expected.UserID != key.UserID {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "session scope is not authorized"}
	}
	return nil
}
func (s *SessionService) authorizeUser(ctx context.Context, key session.UserKey) error {
	scope, ok := ctx.Value(scopeContextKey{}).(nativecontract.Scope)
	if !ok {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "missing server scope"}
	}
	actual, err := s.scopes.Recheck(ctx, scope, nil)
	if err != nil {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "scope is no longer authorized"}
	}
	expected, err := nativecontract.SessionKey(actual, "list")
	if err != nil || expected.AppName != key.AppName || expected.UserID != key.UserID {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "session scope is not authorized"}
	}
	return nil
}
func (s *SessionService) AppendStable(ctx context.Context, a nativecontract.SessionAppend) error {
	if err := s.authorize(ctx, a.Key); err != nil {
		return err
	}
	err := s.store.AppendStable(ctx, a)
	if errors.Is(err, repository.ErrNativeSessionConflict) {
		_, e := StableAppendAction("stored", a.PayloadHash, true)
		return e
	}
	return err
}
func (s *SessionService) CreateSession(ctx context.Context, key session.Key, state session.StateMap, _ ...session.Option) (*session.Session, error) {
	if err := s.authorize(ctx, key); err != nil {
		return nil, err
	}
	if err := s.store.Create(ctx, key, state); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, key)
}
func (s *SessionService) GetSession(ctx context.Context, key session.Key, _ ...session.Option) (*session.Session, error) {
	if err := s.authorize(ctx, key); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, key)
}
func (s *SessionService) ListSessions(ctx context.Context, key session.UserKey, opts ...session.Option) ([]*session.Session, error) {
	if err := s.authorizeUser(ctx, key); err != nil {
		return nil, err
	}
	options := &session.Options{}
	for _, opt := range opts {
		opt(options)
	}
	if err := session.ValidateListSessionsOptions(options); err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx, key)
	if err != nil {
		return nil, err
	}
	if options.ListSessionPage != nil {
		page := options.ListSessionPage
		if page.Offset >= len(items) {
			return []*session.Session{}, nil
		}
		end := page.Offset + page.Limit
		if end > len(items) {
			end = len(items)
		}
		items = items[page.Offset:end]
	}
	if options.ListSessionOnlyMeta {
		for _, item := range items {
			item.Events = nil
		}
	}
	return items, nil
}
func (s *SessionService) DeleteSession(ctx context.Context, key session.Key, _ ...session.Option) error {
	if err := s.authorize(ctx, key); err != nil {
		return err
	}
	return s.store.Delete(ctx, key)
}
func (s *SessionService) UpdateSessionState(ctx context.Context, key session.Key, state session.StateMap) error {
	if err := s.authorize(ctx, key); err != nil {
		return err
	}
	return s.store.UpdateState(ctx, key, state)
}
func (s *SessionService) AppendEvent(ctx context.Context, sess *session.Session, e *event.Event, _ ...session.Option) error {
	if sess == nil {
		return session.ErrNilSession
	}
	if e == nil || e.ID == "" {
		return &nativecontract.Failure{Code: nativecontract.ErrInvalid, Message: "stable event is required"}
	}
	hash, err := canonicalEventHash(e)
	if err != nil {
		return err
	}
	return s.AppendStable(ctx, nativecontract.SessionAppend{Key: session.Key{AppName: sess.AppName, UserID: sess.UserID, SessionID: sess.ID}, StableEventID: e.ID, PayloadHash: hash, Event: e})
}
func canonicalEventHash(e *event.Event) (string, error) {
	payload, err := json.Marshal(struct {
		Version int          `json:"version"`
		Event   *event.Event `json:"event"`
	}{Version: 1, Event: e})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func (s *SessionService) UpdateAppState(ctx context.Context, app string, state session.StateMap) error {
	if err := s.authorizeApp(ctx, app); err != nil {
		return err
	}
	return s.store.UpdateAppState(ctx, app, state)
}
func (s *SessionService) DeleteAppState(ctx context.Context, app, name string) error {
	if err := s.authorizeApp(ctx, app); err != nil {
		return err
	}
	return s.store.DeleteAppState(ctx, app, name)
}
func (s *SessionService) ListAppStates(ctx context.Context, app string) (session.StateMap, error) {
	if err := s.authorizeApp(ctx, app); err != nil {
		return nil, err
	}
	return s.store.AppState(ctx, app)
}
func (s *SessionService) UpdateUserState(ctx context.Context, key session.UserKey, state session.StateMap) error {
	if err := s.authorizeUser(ctx, key); err != nil {
		return err
	}
	return s.store.UpdateUserState(ctx, key, state)
}
func (s *SessionService) ListUserStates(ctx context.Context, key session.UserKey) (session.StateMap, error) {
	if err := s.authorizeUser(ctx, key); err != nil {
		return nil, err
	}
	return s.store.UserState(ctx, key)
}
func (s *SessionService) DeleteUserState(ctx context.Context, key session.UserKey, name string) error {
	if err := s.authorizeUser(ctx, key); err != nil {
		return err
	}
	return s.store.DeleteUserState(ctx, key, name)
}
func (s *SessionService) authorizeApp(ctx context.Context, app string) error {
	scope, ok := ctx.Value(scopeContextKey{}).(nativecontract.Scope)
	if !ok {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "missing server scope"}
	}
	actual, err := s.scopes.Recheck(ctx, scope, nil)
	if err != nil {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "scope is no longer authorized"}
	}
	expected, err := nativecontract.SessionKey(actual, "app")
	if err != nil || expected.AppName != app {
		return &nativecontract.Failure{Code: nativecontract.ErrForbidden, Message: "app scope is not authorized"}
	}
	return nil
}
func (s *SessionService) CreateSessionSummary(ctx context.Context, sess *session.Session, filter string, force bool) error {
	return s.persistSummary(ctx, sess, filter, force)
}
func (s *SessionService) EnqueueSummaryJob(ctx context.Context, sess *session.Session, filter string, force bool) error {
	return s.persistSummary(ctx, sess, filter, force)
}
func (s *SessionService) persistSummary(ctx context.Context, sess *session.Session, filter string, force bool) error {
	if sess == nil {
		return session.ErrNilSession
	}
	key := session.Key{AppName: sess.AppName, UserID: sess.UserID, SessionID: sess.ID}
	if err := s.authorize(ctx, key); err != nil {
		return err
	}
	text := ""
	if sum := sess.Summaries[filter]; sum != nil {
		text = sum.Summary
	}
	if text == "" {
		return &nativecontract.Failure{Code: nativecontract.ErrStore, Message: "summary text is unavailable", Retryable: true}
	}
	if !force {
		if _, ok, err := s.store.Summary(ctx, key, filter); err != nil {
			return err
		} else if ok {
			return nil
		}
	}
	through := ""
	if len(sess.Events) > 0 {
		through = sess.Events[len(sess.Events)-1].ID
	}
	return s.store.SaveSummary(ctx, key, filter, repository.NativeSessionSummary{Text: text, ThroughEventID: through, Revision: 1})
}
func (s *SessionService) GetSessionSummaryText(ctx context.Context, sess *session.Session, opts ...session.SummaryOption) (string, bool) {
	if sess == nil {
		return "", false
	}
	key := session.Key{AppName: sess.AppName, UserID: sess.UserID, SessionID: sess.ID}
	if s.authorize(ctx, key) != nil {
		return "", false
	}
	options := &session.SummaryOptions{}
	for _, opt := range opts {
		opt(options)
	}
	summary, ok, err := s.store.Summary(ctx, key, options.FilterKey)
	if err != nil || !ok {
		return "", false
	}
	return summary.Text, true
}
func (s *SessionService) Close() error { return nil }

var _ session.Service = (*SessionService)(nil)
