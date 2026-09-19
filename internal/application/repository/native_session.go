package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

var ErrNativeSessionConflict = errors.New("native session stable event conflict")

// NativeSessionStore is the business-database persistence boundary for the
// native facade. It writes only the P1 native namespace, never legacy history.
type NativeSessionStore struct{ db *gorm.DB }
type NativeSessionSummary struct {
	Text           string `json:"text"`
	ThroughEventID string `json:"through_event_id"`
	Revision       int64  `json:"revision"`
}

func NewNativeSessionStore(db *gorm.DB) *NativeSessionStore { return &NativeSessionStore{db: db} }

func (s *NativeSessionStore) Create(ctx context.Context, key session.Key, state session.StateMap) error {
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?) ON CONFLICT DO NOTHING", tenant).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", tenant, key.UserID, key.SessionID).Error; err != nil {
			return err
		}
		return s.saveState(ctx, tx, tenant, key, state)
	})
}

func (s *NativeSessionStore) Get(ctx context.Context, key session.Key) (*session.Session, error) {
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.WithContext(ctx).Table("native_agent_sessions").Where("tenant_id = ? AND owner_id = ? AND session_id = ?", tenant, key.UserID, key.SessionID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	state, err := s.loadState(ctx, tenant, key)
	if err != nil {
		return nil, err
	}
	got := session.NewSession(key.AppName, key.UserID, key.SessionID, session.WithSessionState(state))
	var rows []nativeSessionEventRow
	if err := s.db.WithContext(ctx).Table("native_agent_session_events").Select("payload").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", tenant, key.AppName, key.UserID, key.SessionID).Order("ordinal ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		var e event.Event
		if err := json.Unmarshal(row.Payload, &e); err != nil {
			return nil, err
		}
		got.Events = append(got.Events, e)
	}
	return got, nil
}

func (s *NativeSessionStore) List(ctx context.Context, key session.UserKey) ([]*session.Session, error) {
	tenant, err := nativeUserTenant(key)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := s.db.WithContext(ctx).Table("native_agent_sessions").Where("tenant_id = ? AND owner_id = ? AND session_id NOT LIKE ?", tenant, key.UserID, "__native_%").Order("session_id ASC").Pluck("session_id", &ids).Error; err != nil {
		return nil, err
	}
	result := make([]*session.Session, 0, len(ids))
	for _, id := range ids {
		got, err := s.Get(ctx, session.Key{AppName: key.AppName, UserID: key.UserID, SessionID: id})
		if err != nil {
			return nil, err
		}
		result = append(result, got)
	}
	return result, nil
}

func (s *NativeSessionStore) Delete(ctx context.Context, key session.Key) error {
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM native_session_state WHERE tenant_id = ? AND owner_id = ? AND session_id = ?", tenant, key.UserID, key.SessionID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM native_agent_session_events WHERE tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", tenant, key.AppName, key.UserID, key.SessionID).Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM native_agent_sessions WHERE tenant_id = ? AND owner_id = ? AND session_id = ?", tenant, key.UserID, key.SessionID).Error
	})
}
func (s *NativeSessionStore) UpdateState(ctx context.Context, key session.Key, state session.StateMap) error {
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.saveState(ctx, tx, tenant, key, state) })
}
func (s *NativeSessionStore) SaveSummary(ctx context.Context, key session.Key, filter string, summary NativeSessionSummary) error {
	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	return s.UpdateState(ctx, key, session.StateMap{"summary/" + filter: payload})
}
func (s *NativeSessionStore) Summary(ctx context.Context, key session.Key, filter string) (NativeSessionSummary, bool, error) {
	got, err := s.Get(ctx, key)
	if err != nil {
		return NativeSessionSummary{}, false, err
	}
	raw, ok := got.State["summary/"+filter]
	if !ok {
		return NativeSessionSummary{}, false, nil
	}
	var summary NativeSessionSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return NativeSessionSummary{}, false, err
	}
	return summary, true, nil
}
func (s *NativeSessionStore) UpdateAppState(ctx context.Context, app string, state session.StateMap) error {
	tenant, err := nativeUserTenant(session.UserKey{AppName: app, UserID: "app"})
	if err != nil {
		return err
	}
	key := session.Key{AppName: app, UserID: "__native_app__", SessionID: "__native_app_state__"}
	return s.ensureAndState(ctx, tenant, key, state)
}
func (s *NativeSessionStore) AppState(ctx context.Context, app string) (session.StateMap, error) {
	key := session.Key{AppName: app, UserID: "__native_app__", SessionID: "__native_app_state__"}
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return nil, err
	}
	return s.loadState(ctx, tenant, key)
}
func (s *NativeSessionStore) DeleteAppState(ctx context.Context, app, name string) error {
	return s.deleteStateKey(ctx, session.Key{AppName: app, UserID: "__native_app__", SessionID: "__native_app_state__"}, name)
}
func (s *NativeSessionStore) UpdateUserState(ctx context.Context, key session.UserKey, state session.StateMap) error {
	tenant, err := nativeUserTenant(key)
	if err != nil {
		return err
	}
	return s.ensureAndState(ctx, tenant, session.Key{AppName: key.AppName, UserID: key.UserID, SessionID: "__native_user_state__"}, state)
}
func (s *NativeSessionStore) UserState(ctx context.Context, key session.UserKey) (session.StateMap, error) {
	k := session.Key{AppName: key.AppName, UserID: key.UserID, SessionID: "__native_user_state__"}
	tenant, err := nativeSessionTenant(k)
	if err != nil {
		return nil, err
	}
	return s.loadState(ctx, tenant, k)
}
func (s *NativeSessionStore) DeleteUserState(ctx context.Context, key session.UserKey, name string) error {
	return s.deleteStateKey(ctx, session.Key{AppName: key.AppName, UserID: key.UserID, SessionID: "__native_user_state__"}, name)
}
func (s *NativeSessionStore) ensureAndState(ctx context.Context, tenant uint64, key session.Key, state session.StateMap) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?) ON CONFLICT DO NOTHING", tenant).Error; err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", tenant, key.UserID, key.SessionID).Error; err != nil {
			return err
		}
		return s.saveState(ctx, tx, tenant, key, state)
	})
}
func (s *NativeSessionStore) deleteStateKey(ctx context.Context, key session.Key, name string) error {
	tenant, err := nativeSessionTenant(key)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Exec("DELETE FROM native_session_state WHERE tenant_id = ? AND owner_id = ? AND session_id = ? AND state_key = ?", tenant, key.UserID, key.SessionID, name).Error
}

func (s *NativeSessionStore) AppendStable(ctx context.Context, append nativecontract.SessionAppend) error {
	if append.Event == nil || append.StableEventID == "" || append.PayloadHash == "" {
		return fmt.Errorf("native stable append is incomplete")
	}
	tenant, err := nativeSessionTenant(append.Key)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(append.Event)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ { // ordinal contention retries from a fresh transaction.
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var ordinal int64
			if err := tx.Table("native_agent_session_events").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID).Count(&ordinal).Error; err != nil {
				return err
			}
			result := tx.Exec("INSERT INTO native_agent_session_events (tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash, payload, ordinal) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (tenant_id, app_name, user_id, session_id, stable_event_id) DO NOTHING", tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID, append.StableEventID, append.PayloadHash, string(payload), ordinal)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				return nil
			}
			var hash string
			lookup := tx.Table("native_agent_session_events").Select("payload_hash").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ? AND stable_event_id = ?", tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID, append.StableEventID).Row().Scan(&hash)
			if lookup == nil {
				if hash == append.PayloadHash {
					return nil
				}
				return ErrNativeSessionConflict
			}
			return lookup
		})
		if err == nil || errors.Is(err, ErrNativeSessionConflict) {
			return err
		}
	}
	return err
}

type nativeSessionEventRow struct{ Payload []byte }

func (s *NativeSessionStore) saveState(ctx context.Context, tx *gorm.DB, tenant uint64, key session.Key, state session.StateMap) error {
	for name, value := range state {
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err = tx.WithContext(ctx).Exec("INSERT INTO native_session_state (tenant_id, owner_id, session_id, state_key, state_value) VALUES (?, ?, ?, ?, ?) ON CONFLICT (tenant_id, owner_id, session_id, state_key) DO UPDATE SET state_value = excluded.state_value, revision = native_session_state.revision + 1", tenant, key.UserID, key.SessionID, name, string(payload)).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *NativeSessionStore) loadState(ctx context.Context, tenant uint64, key session.Key) (session.StateMap, error) {
	var rows []struct {
		Key   string `gorm:"column:state_key"`
		Value []byte `gorm:"column:state_value"`
	}
	if err := s.db.WithContext(ctx).Table("native_session_state").Select("state_key, state_value").Where("tenant_id = ? AND owner_id = ? AND session_id = ?", tenant, key.UserID, key.SessionID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := session.StateMap{}
	for _, r := range rows {
		var v []byte
		if err := json.Unmarshal(r.Value, &v); err != nil {
			return nil, err
		}
		out[r.Key] = v
	}
	return out, nil
}
func nativeSessionTenant(key session.Key) (uint64, error) {
	if err := key.CheckSessionKey(); err != nil {
		return 0, err
	}
	return nativeUserTenant(session.UserKey{AppName: key.AppName, UserID: key.UserID})
}
func nativeUserTenant(key session.UserKey) (uint64, error) {
	if err := key.CheckUserKey(); err != nil {
		return 0, err
	}
	const prefix = "weknora/native-v1/tenant/"
	if !strings.HasPrefix(key.AppName, prefix) {
		return 0, fmt.Errorf("not a native app scope")
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(key.AppName, prefix), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid native tenant scope")
	}
	return id, nil
}
