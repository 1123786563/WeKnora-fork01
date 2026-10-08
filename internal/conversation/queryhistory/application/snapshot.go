package application

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
)

// Snapshot assembles the Admin+ audit snapshot of one session: the
// tenant-scoped session row, its most recent messages (capped at 200 by the
// AuditReader port), and every feedback row recorded on the session —
// honoring the tenant's query-history privacy policy. Disabled blocks the
// read before the reader is consulted; anonymized masks Session.UserID and
// every Feedback.UserID as "anonymous" so the snapshot cannot be tied back
// to individual principals. Messages carry no owner field of their own, and
// the snapshot row is a plain types.Session — unlike the list rows
// (SessionListItem) it has no IMUserID, so no IM principal id reaches this
// surface at all; a future IM field on Session must be masked here too.
// Messages pass through unchanged.
func (s *AuditService) Snapshot(
	ctx context.Context, tenantID uint64, sessionID string,
) (*types.QueryHistorySnapshot, error) {
	if tenantID == 0 {
		return nil, errors.New("workspace id is required")
	}
	if sessionID == "" {
		return nil, errors.New("session id is required")
	}

	// Policy first: a disabled or failing workspace never reaches the reader.
	mode, err := s.CheckAccess(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	snapshot, err := s.audit.Snapshot(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}

	if mode == domain.Anonymized {
		snapshot.Session.UserID = "anonymous"
		for i := range snapshot.Feedback {
			snapshot.Feedback[i].UserID = "anonymous"
		}
	}
	return snapshot, nil
}
