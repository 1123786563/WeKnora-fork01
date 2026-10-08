package types

import (
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
)

// Query history visibility modes, stored on tenants.query_history_config.
// All three are enforced at READ time on the query-history audit surfaces
// (the "source=all" admin listing, the admin snapshot, and the CSV export):
// what is recorded never changes — switching modes changes what audit reads
// return, not what was stored.
//
// The mode, config, and export definitions moved to the conversation
// module's domain package (Wave 1, Task 4); these aliases keep every
// existing import of internal/types compiling and behaving byte-identically
// (JSON tags, GORM value/scan, table name, normalization rules). Wave 5
// removes this seam together with the rest of the legacy types.
const (
	// QueryHistoryModeNormal serves audit reads with full owner identities.
	QueryHistoryModeNormal = domain.Normal
	// QueryHistoryModeAnonymized serves audit reads with owner identities
	// masked at read time (user ids replaced by "anonymous", the list rows'
	// IM principal id dropped); stored rows keep their identities, so
	// switching back to normal restores them retroactively.
	QueryHistoryModeAnonymized = domain.Anonymized
	// QueryHistoryModeDisabled rejects audit reads with 403. Recording
	// itself still writes, so re-enabling serves everything that was
	// recorded while the mode was active.
	QueryHistoryModeDisabled = domain.Disabled
)

// QueryHistoryExportJob lifecycle statuses.
const (
	QueryHistoryExportPending = domain.ExportPending
	QueryHistoryExportRunning = domain.ExportRunning
	QueryHistoryExportDone    = domain.ExportDone
	QueryHistoryExportFailed  = domain.ExportFailed
)

// NormalizeQueryHistoryMode returns the supported mode for the raw value,
// falling back to QueryHistoryModeNormal for empty/unknown input.
var NormalizeQueryHistoryMode = domain.NormalizeMode

// QueryHistoryConfig is the workspace-level query history policy, stored in
// the tenants.query_history_config JSONB column. Nil means "not configured";
// callers normalizing a config they are about to enforce get Mode=normal.
type QueryHistoryConfig = domain.Config

// QueryHistorySnapshot is the Admin+ audit snapshot of one session: the
// session row, its most recent messages (capped at 200 — see Truncated), and
// every feedback row recorded on the session. When the tenant's query-history
// mode is anonymized the service masks Session.UserID and each Feedback.UserID
// as "anonymous" before the snapshot is returned. Messages reuse the existing
// message serialization (knowledge_references and agent steps ride along).
//
// Retained locally: it reuses the legacy Session/Message/MessageFeedback
// serializations and is the manifest-recorded Wave 5 compatibility seam the
// module's ports still reference.
type QueryHistorySnapshot struct {
	// Session is the tenant-scoped session row.
	Session Session `json:"session"`
	// Messages are the most recent messages, oldest first. Truncated reports
	// whether older messages were dropped by the cap.
	Messages []*Message `json:"messages"`
	// Feedback holds every like/dislike row of the session, across users.
	Feedback []MessageFeedback `json:"feedback"`
	// Truncated is true when the session holds more messages than the
	// snapshot cap; the snapshot keeps the most recent ones.
	Truncated bool `json:"truncated"`
}

// SharedSessionSnapshot is the read-only view a session share token opens
// (SP13): the shared session row plus its most recent messages, capped at the
// same 200-message limit as the audit snapshot. Unlike QueryHistorySnapshot
// it deliberately carries no feedback rows — per-user reaction detail is
// sensitive and the first version of sharing simply omits it. Session.UserID
// is NOT masked: share readers are logged-in members of the same tenant, and
// the owner/admin who minted the link chose to expose the conversation.
type SharedSessionSnapshot struct {
	// Session is the tenant-scoped session row the token resolves to.
	Session Session `json:"session"`
	// Messages are the most recent messages, oldest first. Truncated reports
	// whether older messages were dropped by the cap.
	Messages []*Message `json:"messages"`
	// Truncated is true when the session holds more messages than the
	// snapshot cap; the snapshot keeps the most recent ones.
	Truncated bool `json:"truncated"`
}

// QueryHistoryExportJob tracks one asynchronous query-history export. The
// worker claims pending jobs, streams the archive to FilePath, and leaves
// either done or failed with ErrorMessage set.
type QueryHistoryExportJob = domain.ExportJob

// QueryHistoryExportRow is one aggregated per-session row of an async export
// CSV: the session header fields plus its message and like/dislike tallies.
// Source reuses the audit listing's origin classification (IM platform /
// embed / api / web).
type QueryHistoryExportRow = domain.ExportRow
