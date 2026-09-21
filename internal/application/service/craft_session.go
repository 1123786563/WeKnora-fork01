package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// W03: the Craft workbench's session and HTTP-entrance service. It owns the
// durable craft session state (registration, idempotency keys, associated
// inputs), enforces the existing session permission chain on every entry and
// admits new main runs only through the existing tRPC Submit boundary.

// MaxCraftPromptBytes bounds one craft run prompt (64 KiB). Larger prompts
// are refused before any admission work happens.
const MaxCraftPromptBytes = 64 << 10

// MaxCraftRequestBodyBytes bounds one craft HTTP request body (1 MiB),
// enforced by the handler before decoding.
const MaxCraftRequestBodyBytes = 1 << 20

// CraftFeatureGate is the deployment-owned open/closed switch: craft.enabled
// defaults to false and craft.kinds lists the kinds actually opened. A
// syntactically valid kind is not an open kind.
type CraftFeatureGate struct {
	Enabled bool
	Kinds   []string
}

// Allows reports whether the deployment currently accepts this kind.
func (g CraftFeatureGate) Allows(kind string) bool {
	if !g.Enabled {
		return false
	}
	for _, k := range g.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// CraftGateCapabilities is the deployment gate projected for view
// consumption (CFT-S00-T005): the UI offers exactly the open kinds and can
// explain why a submit entrance is closed. It NEVER relaxes validation —
// every create/run still goes through Allows below.
type CraftGateCapabilities struct {
	Enabled bool
	Kinds   []string
}

// Capabilities projects the gate snapshot (copy, not alias).
func (s *CraftSessionService) Capabilities() CraftGateCapabilities {
	if s == nil {
		return CraftGateCapabilities{}
	}
	return CraftGateCapabilities{Enabled: s.gate.Enabled, Kinds: append([]string(nil), s.gate.Kinds...)}
}

// ActiveRunConflict reports that the session's single run slot is held by a
// live run (queued/running/recovering/waiting_user). The 409 carries the
// existing run id so the client can attach to it instead of retrying blindly.
type ActiveRunConflict struct {
	RunID string
	Note  string
}

func (e *ActiveRunConflict) Error() string {
	return fmt.Sprintf("%v: session already has active run %s: %s", craft.ErrBusy, e.RunID, e.Note)
}

func (e *ActiveRunConflict) Unwrap() error { return craft.ErrBusy }

// CraftSessionConfig assembles the craft session service.
type CraftSessionConfig struct {
	// DB is the migrated business database.
	DB *gorm.DB
	// Sessions is the existing session service: its read path (with the
	// shared/admin fallback) gates every read entry, its owned path gates
	// every write entry.
	Sessions interfaces.SessionService
	// Store is the R02 craft workspace store.
	Store craft.Store
	// Versions is the W01 immutable version store.
	Versions craft.VersionStore
	// Runs admits new main runs through the existing Submit boundary.
	Runs *AgentRunService
	// ActiveRuns is the R05 production active-run query, used as the
	// database-side belt before admission's slot reservation decides.
	ActiveRuns CraftRunActivity
	// TemporaryDocs resolves uploads completed through the existing session
	// attachment entrance.
	TemporaryDocs interfaces.TemporaryDocumentService
	// Files reads stored objects (version file download).
	Files interfaces.FileService
	// Models resolves the chat model identity for admitted run snapshots.
	Models interfaces.ModelService
	// Gate is the deployment feature gate; the zero value keeps craft closed.
	Gate CraftFeatureGate
	// Now is injectable for tests.
	Now func() time.Time
}

// CraftSessionService implements the craft workbench's session surface.
type CraftSessionService struct {
	db            *gorm.DB
	sessions      interfaces.SessionService
	store         craft.Store
	versions      craft.VersionStore
	runs          *AgentRunService
	activeRuns    CraftRunActivity
	temporaryDocs interfaces.TemporaryDocumentService
	files         interfaces.FileService
	models        interfaces.ModelService
	gate          CraftFeatureGate
	now           func() time.Time
}

// NewCraftSessionService validates the assembly and returns the service.
func NewCraftSessionService(cfg CraftSessionConfig) (*CraftSessionService, error) {
	if cfg.DB == nil || cfg.Sessions == nil || cfg.Store == nil || cfg.Versions == nil ||
		cfg.Runs == nil || cfg.ActiveRuns == nil || cfg.TemporaryDocs == nil ||
		cfg.Files == nil || cfg.Models == nil {
		return nil, errors.New("craft: session service requires db, sessions, store, versions, runs, active-runs, temporary docs, files and models")
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CraftSessionService{
		db: cfg.DB, sessions: cfg.Sessions, store: cfg.Store, versions: cfg.Versions,
		runs: cfg.Runs, activeRuns: cfg.ActiveRuns, temporaryDocs: cfg.TemporaryDocs,
		files: cfg.Files, models: cfg.Models, gate: cfg.Gate, now: now,
	}, nil
}

// -----------------------------------------------------------------------------
// Durable rows (migrations 000125_craft_sessions / 000045 sqlite)
// -----------------------------------------------------------------------------

type craftSessionRow struct {
	SessionID string    `gorm:"column:session_id"`
	TenantID  uint64    `gorm:"column:tenant_id"`
	Kind      string    `gorm:"column:kind"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (craftSessionRow) TableName() string { return "craft_sessions" }

type craftSessionRequestRow struct {
	TenantID    uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	UserID      string    `gorm:"column:user_id;primaryKey"`
	Purpose     string    `gorm:"column:purpose;primaryKey"`
	RequestID   string    `gorm:"column:request_id;primaryKey"`
	RequestHash string    `gorm:"column:request_hash"`
	SessionID   string    `gorm:"column:session_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (craftSessionRequestRow) TableName() string { return "craft_session_requests" }

type craftWorkspaceInputRow struct {
	WorkspaceID string    `gorm:"column:workspace_id;primaryKey"`
	TenantID    uint64    `gorm:"column:tenant_id"`
	Ref         string    `gorm:"column:ref"`
	Name        string    `gorm:"column:name"`
	SHA256      string    `gorm:"column:sha256"`
	Bytes       int64     `gorm:"column:bytes"`
	CitationID  string    `gorm:"column:citation_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (craftWorkspaceInputRow) TableName() string { return "craft_workspace_inputs" }

func (r craftWorkspaceInputRow) input() craft.Input {
	return craft.Input{Ref: r.Ref, Name: r.Name, SHA256: r.SHA256, Bytes: r.Bytes, CitationID: r.CitationID}
}

// -----------------------------------------------------------------------------
// Session permission chain
// -----------------------------------------------------------------------------

// craftCreateHash binds the idempotency key to the admitted content: kind and
// title. A retried key with a different request is a conflict, not a replay.
func craftCreateHash(kind, title string) string {
	sum := sha256.Sum256([]byte("craft-create-v1\x00" + kind + "\x00" + title))
	return hex.EncodeToString(sum[:])
}

// readSession loads a session through the existing read ACL (owner scope with
// the shared admin fallback) and enforces the craft tenant scope. Reads of
// another tenant's session answer ErrNotFound: it does not exist here.
func (s *CraftSessionService) readSession(ctx context.Context, scope craft.Scope, sessionID string) (*types.Session, error) {
	if scope.TenantID == 0 || scope.UserID == "" || sessionID == "" {
		return nil, fmt.Errorf("%w: incomplete craft scope", craft.ErrInvalidInput)
	}
	session, err := s.sessions.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, craft.ErrNotFound
	}
	if session.TenantID != scope.TenantID {
		return nil, craft.ErrNotFound
	}
	return session, nil
}

// writeSession loads a session strictly within the caller's owner scope and
// requires the tRPC engine: a builtin session can never become a craft
// session. A readable but not owned session answers ErrForbidden; an
// invisible one stays ErrNotFound.
func (s *CraftSessionService) writeSession(ctx context.Context, scope craft.Scope, sessionID string) (*types.Session, error) {
	if scope.TenantID == 0 || scope.UserID == "" || sessionID == "" {
		return nil, fmt.Errorf("%w: incomplete craft scope", craft.ErrInvalidInput)
	}
	if readable, rerr := s.readSession(ctx, scope, sessionID); rerr == nil && readable.UserID != scope.UserID {
		return nil, fmt.Errorf("%w: session %s belongs to %s", craft.ErrForbidden, sessionID, readable.UserID)
	}
	session, err := s.sessions.GetOwnedSession(ctx, sessionID)
	if err != nil || session == nil {
		return nil, craft.ErrNotFound
	}
	if session.TenantID != scope.TenantID || session.UserID != scope.UserID {
		return nil, craft.ErrNotFound
	}
	if session.EngineType != string(types.AgentEngineTRPC) {
		return nil, fmt.Errorf("%w: session %s uses engine %q; builtin sessions cannot become craft sessions",
			craft.ErrConflict, sessionID, session.EngineType)
	}
	return session, nil
}

// craftRow returns the craft registration of a session in the caller's tenant.
func (s *CraftSessionService) craftRow(ctx context.Context, tenantID uint64, sessionID string) (craftSessionRow, error) {
	var row craftSessionRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftSessionRow{}, fmt.Errorf("%w: session %s is not a craft session", craft.ErrNotFound, sessionID)
	}
	return row, err
}

// ownerScopeOf derives the execution-owner scope from the persisted session
// row: the authenticated viewer may differ (shared read), the workspace
// always belongs to the session owner.
func ownerScopeOf(session *types.Session) craft.Scope {
	return craft.Scope{TenantID: session.TenantID, UserID: session.UserID, SessionID: session.ID}
}

// -----------------------------------------------------------------------------
// Create / Get / List
// -----------------------------------------------------------------------------

// Create registers a new craft session: one transaction creates the sessions
// row with engine_type=trpc, the craft_sessions registration and the
// tenant+user scoped idempotency row. A retried key with identical content
// returns the same session; the same key with different content is a
// conflict. The workspace binding row is ensured after commit so the session
// is addressable as a craft workspace from the first response.
func (s *CraftSessionService) Create(ctx context.Context, scope craft.Scope, in craft.CreateRequest) (craft.Workspace, error) {
	if err := craft.ValidateCreate(in); err != nil {
		return craft.Workspace{}, err
	}
	if !s.gate.Allows(in.Kind) {
		return craft.Workspace{}, fmt.Errorf("%w: craft kind %q is not enabled on this deployment", craft.ErrUnsupported, in.Kind)
	}
	if scope.TenantID == 0 || scope.UserID == "" {
		return craft.Workspace{}, fmt.Errorf("%w: craft create requires an authenticated tenant and user", craft.ErrInvalidInput)
	}
	if scope.SessionID != "" {
		return craft.Workspace{}, fmt.Errorf("%w: craft create derives its own session", craft.ErrInvalidInput)
	}
	requestHash := craftCreateHash(in.Kind, in.Title)

	var sessionID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Idempotent replay check first.
		var existing craftSessionRequestRow
		err := tx.Where("tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ?",
			scope.TenantID, scope.UserID, "create", in.RequestID).Take(&existing).Error
		if err == nil {
			if existing.RequestHash != requestHash {
				return fmt.Errorf("%w: request id %q was already used with different parameters",
					craft.ErrConflict, in.RequestID)
			}
			sessionID = existing.SessionID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		session := &types.Session{
			TenantID:   scope.TenantID,
			UserID:     scope.UserID,
			Title:      strings.TrimSpace(in.Title),
			EngineType: string(types.AgentEngineTRPC),
		}
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		registration := craftSessionRow{SessionID: session.ID, TenantID: scope.TenantID, Kind: in.Kind}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&registration).Error; err != nil {
			return err
		}
		idempotency := craftSessionRequestRow{
			TenantID: scope.TenantID, UserID: scope.UserID, Purpose: "create",
			RequestID: strings.TrimSpace(in.RequestID), RequestHash: requestHash,
			SessionID: session.ID, CreatedAt: s.now(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&idempotency).Error; err != nil {
			return err
		}
		sessionID = session.ID
		return nil
	})
	if err != nil {
		return craft.Workspace{}, err
	}

	owner := craft.Scope{TenantID: scope.TenantID, UserID: scope.UserID, SessionID: sessionID}
	return s.ensureWorkspace(ctx, owner)
}

// ensureWorkspace idempotently guarantees the craft workspace binding row
// exists for the session and returns it. The binding starts empty (no
// sandbox); R03's resolver provisions it on first use.
func (s *CraftSessionService) ensureWorkspace(ctx context.Context, owner craft.Scope) (craft.Workspace, error) {
	stored, err := s.store.PutWorkspace(ctx, craft.Workspace{Scope: owner}, 0)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, craft.ErrConflict) {
		return craft.Workspace{}, err
	}
	return s.store.GetWorkspace(ctx, owner)
}

// Get returns the workspace bound to the session (brief interface). The read
// runs through the existing session read ACL; the workspace is answered in
// the session owner's scope.
func (s *CraftSessionService) Get(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	session, err := s.readSession(ctx, scope, scope.SessionID)
	if err != nil {
		return craft.Workspace{}, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return craft.Workspace{}, err
	}
	return s.store.GetWorkspace(ctx, ownerScopeOf(session))
}

// CraftWorkspaceView is the GET / response body source: the workspace, its
// live run state, the current version and the event watermark.
type CraftWorkspaceView struct {
	SessionID      string
	WorkspaceID    string
	Kind           string
	Title          string
	EngineType     string
	Workspace      craft.Workspace
	ActiveRunID    string
	ActiveRun      *agentruntime.Run
	PendingID      string
	CurrentVersion *craft.Version
	LastSeq        int64
}

// View assembles the workspace view for the session.
func (s *CraftSessionService) View(ctx context.Context, scope craft.Scope) (CraftWorkspaceView, error) {
	session, err := s.readSession(ctx, scope, scope.SessionID)
	if err != nil {
		return CraftWorkspaceView{}, err
	}
	registration, err := s.craftRow(ctx, session.TenantID, session.ID)
	if err != nil {
		return CraftWorkspaceView{}, err
	}
	owner := ownerScopeOf(session)
	workspace, err := s.store.GetWorkspace(ctx, owner)
	if err != nil {
		return CraftWorkspaceView{}, err
	}
	view := CraftWorkspaceView{
		SessionID: session.ID, WorkspaceID: workspace.ID, Kind: registration.Kind,
		Title: session.Title, EngineType: session.EngineType, Workspace: workspace,
	}
	if session.ActiveAgentRunID != nil && *session.ActiveAgentRunID != "" {
		view.ActiveRunID = *session.ActiveAgentRunID
		if run, rerr := s.runs.Get(ctx, agentruntime.RunKey{TenantID: session.TenantID, RunID: view.ActiveRunID}); rerr == nil {
			view.ActiveRun = &run
			if run.Status == "waiting_user" {
				view.PendingID = run.WaitReason
			}
		}
	}
	if versions, verr := s.versions.List(ctx, owner); verr == nil && len(versions) > 0 {
		current := versions[0]
		view.CurrentVersion = &current
	}
	view.LastSeq = s.lastRunSeq(ctx, session.TenantID, view.ActiveRunID)
	return view, nil
}

// lastRunSeq reads the run event watermark through the store's trimmer
// interface; an unavailable trimmer answers 0.
func (s *CraftSessionService) lastRunSeq(ctx context.Context, tenantID uint64, runID string) int64 {
	if runID == "" {
		return 0
	}
	trimmer, ok := s.runs.Store().(agentruntime.RunEventTrimmer)
	if !ok {
		return 0
	}
	seq, err := trimmer.LastEventSeq(ctx, agentruntime.RunKey{TenantID: tenantID, RunID: runID})
	if err != nil {
		return 0
	}
	return seq
}

// CraftSessionSummary is one row of the craft session list.
type CraftSessionSummary struct {
	SessionID   string
	WorkspaceID string
	Kind        string
	Title       string
	EngineType  string
	UpdatedAt   time.Time
}

// List returns the caller's craft sessions newest-first with an opaque
// updated_at+id cursor, following the existing per-user session list scope.
func (s *CraftSessionService) List(ctx context.Context, scope craft.Scope, cursor string, limit int) ([]CraftSessionSummary, string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	afterTime, afterID, err := decodeCraftCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	type listRow struct {
		SessionID   string    `gorm:"column:session_id"`
		WorkspaceID string    `gorm:"column:workspace_id"`
		Kind        string    `gorm:"column:kind"`
		Title       string    `gorm:"column:title"`
		UpdatedAt   time.Time `gorm:"column:updated_at"`
	}
	query := s.db.WithContext(ctx).Table("craft_sessions").
		Select("craft_sessions.session_id, craft_sessions.kind, sessions.title, sessions.updated_at, craft_workspaces.id AS workspace_id").
		Joins("JOIN sessions ON sessions.id = craft_sessions.session_id AND sessions.tenant_id = craft_sessions.tenant_id").
		Joins("LEFT JOIN craft_workspaces ON craft_workspaces.tenant_id = craft_sessions.tenant_id AND craft_workspaces.session_id = craft_sessions.session_id").
		Where("craft_sessions.tenant_id = ? AND sessions.user_id = ? AND sessions.deleted_at IS NULL",
			scope.TenantID, scope.UserID).
		Order("sessions.updated_at DESC, sessions.id DESC").
		Limit(limit + 1)
	if afterTime != nil {
		query = query.Where("(sessions.updated_at < ? OR (sessions.updated_at = ? AND sessions.id < ?))",
			*afterTime, *afterTime, afterID)
	}
	var rows []listRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCraftCursor(last.UpdatedAt, last.SessionID)
	}
	out := make([]CraftSessionSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, CraftSessionSummary{
			SessionID: row.SessionID, WorkspaceID: row.WorkspaceID, Kind: row.Kind,
			Title: row.Title, EngineType: string(types.AgentEngineTRPC), UpdatedAt: row.UpdatedAt,
		})
	}
	return out, next, nil
}

func encodeCraftCursor(updatedAt time.Time, sessionID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(
		updatedAt.UTC().Format(time.RFC3339Nano) + "\x00" + sessionID))
}

func decodeCraftCursor(cursor string) (*time.Time, string, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, "", fmt.Errorf("%w: malformed list cursor", craft.ErrInvalidInput)
	}
	parts := strings.SplitN(string(raw), "\x00", 2)
	if len(parts) != 2 || parts[1] == "" {
		return nil, "", fmt.Errorf("%w: malformed list cursor", craft.ErrInvalidInput)
	}
	when, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, "", fmt.Errorf("%w: malformed list cursor", craft.ErrInvalidInput)
	}
	return &when, parts[1], nil
}

// -----------------------------------------------------------------------------
// Inputs (POST /inputs): associate a completed upload with the workspace
// -----------------------------------------------------------------------------

// AssociateInput links one upload that completed through the existing session
// attachment entrance to the session's craft workspace. The upload must be
// session-scoped (cross-space refs are invisible), parsing must have
// finished, and the caller-declared digest must equal the stored bytes. The
// persisted row is the authorization manifest later runs resolve input_refs
// against; nothing is staged into the sandbox here.
func (s *CraftSessionService) AssociateInput(
	ctx context.Context, scope craft.Scope, resourceRef, expectedSHA256 string,
) (craft.Input, error) {
	if strings.TrimSpace(resourceRef) == "" {
		return craft.Input{}, fmt.Errorf("%w: resource_ref is required", craft.ErrInvalidInput)
	}
	expected := strings.TrimSpace(expectedSHA256)
	if !craft.ValidSHA256(expected) {
		return craft.Input{}, fmt.Errorf("%w: expected_sha256 must be 64 lowercase hex characters", craft.ErrInvalidInput)
	}
	session, err := s.writeSession(ctx, scope, scope.SessionID)
	if err != nil {
		return craft.Input{}, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return craft.Input{}, err
	}
	owner := ownerScopeOf(session)
	workspace, err := s.store.GetWorkspace(ctx, owner)
	if err != nil {
		return craft.Input{}, err
	}

	// The ref addresses an upload of THIS session: another session's or
	// another tenant's upload does not exist for this caller.
	document, err := s.temporaryDocs.Get(ctx, session.TenantID, session.ID, strings.TrimSpace(resourceRef))
	if err != nil || document == nil {
		return craft.Input{}, fmt.Errorf("%w: upload %s was not found in this session", craft.ErrNotFound, resourceRef)
	}
	switch document.Status {
	case types.TemporaryDocumentStatusReady:
	case types.TemporaryDocumentStatusFailed:
		return craft.Input{}, fmt.Errorf("%w: upload %s failed to process: %s",
			craft.ErrConflict, document.FileName, document.ErrorMessage)
	default:
		return craft.Input{}, fmt.Errorf("%w: upload %s is still processing", craft.ErrConflict, document.FileName)
	}
	if err := craft.ValidateInputName(document.FileName); err != nil {
		return craft.Input{}, err
	}
	if document.FileSize <= 0 || document.FileSize > craft.MaxInputBytes {
		return craft.Input{}, fmt.Errorf("%w: upload %s is %d bytes, outside the staged input window",
			craft.ErrInvalidInput, document.FileName, document.FileSize)
	}

	// Verify the declared digest against the stored bytes with a bounded read.
	reader, _, err := s.temporaryDocs.OpenFile(ctx, session.TenantID, session.ID, document.ID)
	if err != nil {
		return craft.Input{}, err
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, craft.MaxInputBytes+1))
	if err != nil {
		return craft.Input{}, err
	}
	if int64(len(content)) != document.FileSize {
		return craft.Input{}, fmt.Errorf("%w: upload %s declares %d bytes but stored %d",
			craft.ErrInvalidInput, document.FileName, document.FileSize, len(content))
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	if digest != expected {
		return craft.Input{}, fmt.Errorf("%w: upload %s digest mismatch: declared %s, stored %s",
			craft.ErrInvalidInput, document.FileName, expected, digest)
	}

	row := craftWorkspaceInputRow{
		WorkspaceID: workspace.ID, TenantID: session.TenantID,
		Ref: document.ResourceRef, Name: document.FileName, SHA256: digest,
		Bytes: document.FileSize, CitationID: document.ID, CreatedAt: s.now(),
	}
	created := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return craft.Input{}, created.Error
	}
	if created.RowsAffected != 1 {
		var stored craftWorkspaceInputRow
		err := s.db.WithContext(ctx).Where("workspace_id = ? AND ref = ?", workspace.ID, document.ResourceRef).
			Take(&stored).Error
		if err != nil {
			return craft.Input{}, err
		}
		if stored.SHA256 != digest {
			return craft.Input{}, fmt.Errorf("%w: input %s is already associated with different content",
				craft.ErrConflict, document.FileName)
		}
		return stored.input(), nil
	}
	return row.input(), nil
}

// WorkspaceInputs lists the inputs associated with the workspace.
func (s *CraftSessionService) WorkspaceInputs(ctx context.Context, scope craft.Scope) ([]craft.Input, error) {
	session, err := s.readSession(ctx, scope, scope.SessionID)
	if err != nil {
		return nil, err
	}
	owner := ownerScopeOf(session)
	workspace, err := s.store.GetWorkspace(ctx, owner)
	if err != nil {
		return nil, err
	}
	var rows []craftWorkspaceInputRow
	if err := s.db.WithContext(ctx).Where("workspace_id = ?", workspace.ID).
		Order("created_at, ref").Find(&rows).Error; err != nil {
		return nil, err
	}
	inputs := make([]craft.Input, 0, len(rows))
	for _, row := range rows {
		inputs = append(inputs, row.input())
	}
	return inputs, nil
}

// -----------------------------------------------------------------------------
// Runs (POST /runs): admit a new main run through the existing Submit
// -----------------------------------------------------------------------------

// CraftRunRequest is the POST /runs body: an idempotency key, the prompt, the
// input references to authorize for this run, an optional knowledge scope and
// an optional base version.
type CraftRunRequest struct {
	RequestID      string
	Prompt         string
	InputRefs      []string
	KnowledgeScope string
	BaseVersionID  string
}

// Validate checks the request shape before any session or admission work.
func (r CraftRunRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return fmt.Errorf("%w: request_id is required", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return fmt.Errorf("%w: prompt is required", craft.ErrInvalidInput)
	}
	if len(r.Prompt) > MaxCraftPromptBytes {
		return fmt.Errorf("%w: prompt is %d bytes over the %d byte limit",
			craft.ErrInvalidInput, len(r.Prompt), MaxCraftPromptBytes)
	}
	if len(r.InputRefs) > craft.MaxInputsPerRound {
		return fmt.Errorf("%w: %d input refs exceed the per-round cap %d",
			craft.ErrInvalidInput, len(r.InputRefs), craft.MaxInputsPerRound)
	}
	if len(r.KnowledgeScope) > 128 {
		return fmt.Errorf("%w: knowledge_scope too long", craft.ErrInvalidInput)
	}
	return nil
}

// StartRun validates the request against the workspace's state and admits a
// new main run through the existing Submit boundary. HTTP never starts a
// goroutine; the durable worker owns execution. A live run (including
// waiting_user) refuses the request with the existing run id, and a retried
// request id reuses the original admission.
func (s *CraftSessionService) StartRun(ctx context.Context, scope craft.Scope, req CraftRunRequest) (agentruntime.Run, error) {
	if err := req.Validate(); err != nil {
		return agentruntime.Run{}, err
	}
	if !s.gate.Enabled {
		return agentruntime.Run{}, fmt.Errorf("%w: craft is not enabled on this deployment", craft.ErrUnsupported)
	}
	session, err := s.writeSession(ctx, scope, scope.SessionID)
	if err != nil {
		return agentruntime.Run{}, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return agentruntime.Run{}, err
	}
	owner := ownerScopeOf(session)
	workspace, err := s.store.GetWorkspace(ctx, owner)
	if err != nil {
		return agentruntime.Run{}, err
	}

	// Every input ref must belong to this workspace's associated manifest.
	authorized, err := s.resolveInputRefs(ctx, workspace.ID, req.InputRefs)
	if err != nil {
		return agentruntime.Run{}, err
	}
	if base := strings.TrimSpace(req.BaseVersionID); base != "" {
		if _, err := s.versions.Get(ctx, owner, base); err != nil {
			return agentruntime.Run{}, err
		}
	}

	requestID := "craft-" + strings.TrimSpace(req.RequestID)

	// Live-run guard (belt) for genuinely fresh keys only: when this key
	// already has an admission, Submit replays it below and must not be
	// blocked by the slot another run holds. Admission's slot reservation
	// remains the arbiter for fresh admissions.
	var priorRun struct{ RunID string }
	priorErr := s.db.WithContext(ctx).Table("agent_runs").Select("run_id").
		Where("tenant_id = ? AND owner_id = ? AND request_id = ?", session.TenantID, session.UserID, requestID).
		Take(&priorRun).Error
	if errors.Is(priorErr, gorm.ErrRecordNotFound) {
		if active, aerr := s.activeRuns(ctx, owner); aerr == nil && active {
			if conflict := s.activeRunConflict(ctx, session); conflict != nil {
				return agentruntime.Run{}, conflict
			}
			return agentruntime.Run{}, fmt.Errorf("%w: session %s has an active run", craft.ErrBusy, session.ID)
		}
	} else if priorErr != nil {
		return agentruntime.Run{}, priorErr
	}

	modelID, err := s.craftChatModelID(ctx)
	if err != nil {
		return agentruntime.Run{}, err
	}
	snapshot, err := craftRunSnapshot(req, authorized, modelID)
	if err != nil {
		return agentruntime.Run{}, err
	}

	runID := uuid.NewString()
	assistantID := craftDeterministicUUID("assistant", session.TenantID, session.UserID, requestID)
	userMessageID := craftDeterministicUUID("user", session.TenantID, session.UserID, requestID)
	digest := sha256.Sum256(append(append([]byte(nil), snapshot...), []byte(assistantID)...))
	user, err := json.Marshal(map[string]any{"role": "user", "content": req.Prompt})
	if err != nil {
		return agentruntime.Run{}, err
	}
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	if err != nil {
		return agentruntime.Run{}, err
	}
	run, err := s.runs.Submit(ctx, agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: session.TenantID, RunID: runID},
		SessionID:          session.ID,
		UserID:             session.UserID,
		UserMessageID:      userMessageID,
		RequestID:          requestID,
		AssistantMessageID: assistantID,
		RequestHash:        hex.EncodeToString(digest[:]),
		Snapshot:           snapshot,
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           s.now().Add(30 * time.Minute),
	})
	if err != nil {
		if errors.Is(err, agentruntime.ErrRunActive) {
			if conflict := s.activeRunConflict(ctx, session); conflict != nil {
				return agentruntime.Run{}, conflict
			}
		}
		if errors.Is(err, agentruntime.ErrConflict) {
			return agentruntime.Run{}, fmt.Errorf("%w: %v", craft.ErrConflict, err)
		}
		return agentruntime.Run{}, err
	}
	return run, nil
}

// activeRunConflict reads the session's run slot and wraps it as a typed
// conflict; nil when the slot is free.
func (s *CraftSessionService) activeRunConflict(ctx context.Context, session *types.Session) *ActiveRunConflict {
	if session.ActiveAgentRunID == nil || *session.ActiveAgentRunID == "" {
		return nil
	}
	note := "the run still holds the session's run slot"
	if run, err := s.runs.Get(ctx, agentruntime.RunKey{TenantID: session.TenantID, RunID: *session.ActiveAgentRunID}); err == nil && run.Status != "" {
		note = "existing run status " + run.Status
	}
	return &ActiveRunConflict{RunID: *session.ActiveAgentRunID, Note: note}
}

// resolveInputRefs loads the workspace's associated inputs and verifies every
// requested ref is part of that manifest.
func (s *CraftSessionService) resolveInputRefs(ctx context.Context, workspaceID string, refs []string) ([]craft.Input, error) {
	var rows []craftWorkspaceInputRow
	if err := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).
		Order("created_at, ref").Find(&rows).Error; err != nil {
		return nil, err
	}
	byRef := make(map[string]craft.Input, len(rows))
	for _, row := range rows {
		byRef[row.Ref] = row.input()
	}
	resolved := make([]craft.Input, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if seen[ref] {
			continue
		}
		seen[ref] = true
		input, ok := byRef[ref]
		if !ok {
			return nil, fmt.Errorf("%w: input ref %q is not associated with this workspace", craft.ErrInvalidInput, ref)
		}
		resolved = append(resolved, input)
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i].Ref < resolved[j].Ref })
	return resolved, nil
}

// craftChatModelID resolves the tenant's default chat-capable model. Without
// any chat model the craft run surface fails closed (503): it never invents a
// model identity.
func (s *CraftSessionService) craftChatModelID(ctx context.Context) (string, error) {
	models, err := s.models.ListModels(ctx)
	if err != nil {
		return "", err
	}
	chatType := func(m *types.Model) bool {
		return m.Type == types.ModelTypeVLLM || m.Type == types.ModelTypeKnowledgeQA
	}
	fallback := ""
	for _, m := range models {
		if m == nil || m.DeletedAt.Valid || !chatType(m) {
			continue
		}
		if m.IsDefault {
			return m.ID, nil
		}
		if fallback == "" {
			fallback = m.ID
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("%w: no chat model is configured for craft runs", craft.ErrUnsupported)
}

// craftRunSnapshot freezes the run's request identity: the prompt, the
// authorized input manifest and the base version, plus the agent config whose
// tool whitelist opens craft_delegate for the craft session. The whitelist is
// server-assembled; the client never supplies tool or model identity.
func craftRunSnapshot(req CraftRunRequest, inputs []craft.Input, modelID string) ([]byte, error) {
	var query strings.Builder
	query.WriteString(req.Prompt)
	if len(inputs) > 0 {
		query.WriteString("\n\n[已授权输入材料]")
		for _, in := range inputs {
			path, err := craft.InputPath(in.SHA256, in.Name)
			if err != nil {
				path = in.Name
			}
			fmt.Fprintf(&query, "\n- %s（%d 字节，引用 %s）", path, in.Bytes, in.Ref)
		}
	}
	if base := strings.TrimSpace(req.BaseVersionID); base != "" {
		fmt.Fprintf(&query, "\n\n[基线版本] %s", base)
	}

	allowed := tools.DefaultAllowedTools()
	allowed = append(allowed, "craft_delegate")
	config := &types.AgentConfig{
		MaxIterations:    15,
		MultiTurnEnabled: true,
		HistoryTurns:     5,
		AllowedTools:     allowed,
	}
	return BuildDurableRunSnapshot(query.String(), nil, modelID, "", config)
}

// craftDeterministicUUID derives a stable uuid-shaped identity so retried
// requests with the same key hash the same request content.
func craftDeterministicUUID(purpose string, tenantID uint64, userID, requestID string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-%s/%d/%s/%s", purpose, tenantID, userID, requestID)))
	id, err := uuid.FromBytes(sum[:16])
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

// -----------------------------------------------------------------------------
// Versions (GET /versions, GET /versions/:id, file download)
// -----------------------------------------------------------------------------

// ListVersions returns the session's workspace versions, newest first.
func (s *CraftSessionService) ListVersions(ctx context.Context, scope craft.Scope) ([]craft.Version, error) {
	session, err := s.readSession(ctx, scope, scope.SessionID)
	if err != nil {
		return nil, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return nil, err
	}
	return s.versions.List(ctx, ownerScopeOf(session))
}

// GetVersion returns one published version of the session's workspace.
func (s *CraftSessionService) GetVersion(ctx context.Context, scope craft.Scope, versionID string) (craft.Version, error) {
	if strings.TrimSpace(versionID) == "" {
		return craft.Version{}, fmt.Errorf("%w: version id is required", craft.ErrInvalidInput)
	}
	session, err := s.readSession(ctx, scope, scope.SessionID)
	if err != nil {
		return craft.Version{}, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return craft.Version{}, err
	}
	return s.versions.Get(ctx, ownerScopeOf(session), strings.TrimSpace(versionID))
}

// OpenVersionFile validates the whole session→workspace→version→file chain
// and opens the version's pinned object for download. filePath is the
// version-relative manifest path; only a byte-exact manifest member answers.
func (s *CraftSessionService) OpenVersionFile(ctx context.Context, scope craft.Scope, versionID, filePath string) (craft.File, io.ReadCloser, error) {
	version, err := s.GetVersion(ctx, scope, versionID)
	if err != nil {
		return craft.File{}, nil, err
	}
	if err := craft.ValidateArtifactPath(filePath); err != nil {
		return craft.File{}, nil, err
	}
	for _, file := range version.Files {
		if file.Path != filePath {
			continue
		}
		reader, err := s.files.GetFile(ctx, file.Ref)
		if err != nil {
			return craft.File{}, nil, err
		}
		return file, reader, nil
	}
	return craft.File{}, nil, fmt.Errorf("%w: file %q is not part of version %s", craft.ErrNotFound, filePath, version.ID)
}
