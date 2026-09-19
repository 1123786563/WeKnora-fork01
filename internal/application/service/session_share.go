package service

// Session sharing (SP13 Task 5): an in-tenant logged-in share surface. The
// owner (or an Admin+ member) mints an opaque capability token for one
// session; any Viewer+ member of the SAME tenant can then resolve the token
// into a read-only snapshot. Share-again rotates (the old link dies), DELETE
// revokes (NULL), and the token lookup is tenant-scoped so a leaked link is
// worthless outside the tenant.

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	stderrors "errors"
	"strings"

	"gorm.io/gorm"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// canShareSession reports whether the caller may mint or revoke a session's
// share token: the session owner themself, or any tenant member at Admin
// level or above — the same gate as SP11's canFeedback.
func canShareSession(caller types.Caller, session *types.Session) bool {
	if caller.UserID != "" && session.UserID == caller.UserID {
		return true
	}
	return caller.Role.HasPermission(types.TenantRoleAdmin)
}

// mintShareToken returns 256 bits from crypto/rand, URL-safe (43 chars) —
// the same shape as mintOAuthState. The token is the entire bearer
// capability of the share link, so it is never logged and never accepted
// from the client on any other surface.
func mintShareToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// isShareTokenCollision reports whether err is the partial unique index
// uq_sessions_share_token rejecting a freshly minted token that happens to
// equal another session's. Detection mirrors isDuplicateMembership: the
// gorm sentinel covers translated drivers, the string match covers raw ones
// (Postgres "duplicate key value violates unique constraint", SQLite
// "UNIQUE constraint failed", MySQL "Duplicate entry").
func isShareTokenCollision(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique constraint")
}

// loadSessionForShare loads the session inside the caller's tenant for a
// share mutation. A miss is a 404 AppError (indistinguishable from a
// cross-tenant probe).
func (s *sessionService) loadSessionForShare(
	ctx context.Context, caller types.Caller, sessionID string,
) (*types.Session, error) {
	session, err := s.sessionRepo.GetByID(ctx, caller.TenantID, sessionID)
	if err != nil {
		if stderrors.Is(err, apperrors.ErrSessionNotFound) || stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("session not found")
		}
		return nil, err
	}
	if !canShareSession(caller, session) {
		return nil, apperrors.NewForbiddenError("not allowed to share this session")
	}
	return session, nil
}

// ShareSession mints the session's share token, or rotates it when the
// session is already shared — the previous link stops resolving the moment
// the new token lands. Owner-or-Admin+ only (canShareSession). The unique
// index makes two sessions holding one token impossible; for 256-bit random
// tokens a mint colliding with an existing token is astronomically unlikely,
// and the loop retries exactly once before surfacing the error.
func (s *sessionService) ShareSession(
	ctx context.Context, caller types.Caller, sessionID string,
) (string, error) {
	if strings.TrimSpace(sessionID) == "" {
		return "", apperrors.NewBadRequestError("session id is required")
	}
	if _, err := s.loadSessionForShare(ctx, caller, sessionID); err != nil {
		return "", err
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := mintShareToken()
		if err != nil {
			return "", err
		}
		affected, err := s.sessionRepo.SetShareToken(ctx, caller.TenantID, sessionID, &token)
		if err != nil {
			if isShareTokenCollision(err) && attempt == 0 {
				logger.Warnf(ctx,
					"share token collision for session %s, retrying with a fresh token", sessionID)
				continue
			}
			return "", err
		}
		if affected == 0 {
			// The row vanished between the load and the update.
			return "", apperrors.NewNotFoundError("session not found")
		}
		logger.Infof(ctx, "Session %s shared (token rotated) by user %s", sessionID, caller.UserID)
		return token, nil
	}
	// Unreachable: the loop returns from both arms on its second pass.
	return "", apperrors.NewInternalServerError("failed to mint a unique share token")
}

// UnshareSession revokes the session's share token (NULLs the column), so
// the link stops resolving immediately. Owner-or-Admin+ only; revoking an
// unshared session is still a success (idempotent) as long as the row exists.
func (s *sessionService) UnshareSession(
	ctx context.Context, caller types.Caller, sessionID string,
) error {
	if strings.TrimSpace(sessionID) == "" {
		return apperrors.NewBadRequestError("session id is required")
	}
	if _, err := s.loadSessionForShare(ctx, caller, sessionID); err != nil {
		return err
	}

	affected, err := s.sessionRepo.SetShareToken(ctx, caller.TenantID, sessionID, nil)
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFoundError("session not found")
	}
	logger.Infof(ctx, "Session %s unshared by user %s", sessionID, caller.UserID)
	return nil
}

// GetSharedSession resolves a share token inside the caller's tenant and
// assembles the read-only snapshot it opens: the session row and its most
// recent messages, capped exactly like the Admin+ audit snapshot, with NO
// feedback rows (per-user reaction detail stays out of the first version).
// Session.UserID is intentionally not masked — share readers are logged-in
// members of the same tenant. Every miss (unknown token, revoked link,
// cross-tenant token, soft-deleted session) is one uniform 404 so the read
// reveals nothing about which it was.
func (s *sessionService) GetSharedSession(
	ctx context.Context, caller types.Caller, token string,
) (*types.SharedSessionSnapshot, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, apperrors.NewNotFoundError("shared session not found")
	}

	session, err := s.sessionRepo.GetByShareToken(ctx, caller.TenantID, token)
	if err != nil {
		if stderrors.Is(err, apperrors.ErrSessionNotFound) || stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("shared session not found")
		}
		return nil, err
	}

	messages, truncated, err := s.recentSessionMessagesCapped(
		ctx, session.ID, queryHistorySnapshotMessageLimit)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": session.ID,
			"tenant_id":  caller.TenantID,
		})
		return nil, err
	}

	return &types.SharedSessionSnapshot{
		Session:   *session,
		Messages:  messages,
		Truncated: truncated,
	}, nil
}
