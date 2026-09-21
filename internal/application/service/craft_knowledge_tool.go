package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
)

// C06: controlled supplemental knowledge retrieval for a running craft
// delegation. The tool is the INTERNAL controlled wrapper the brief allows
// as the alternative to exposing MCP knowledge tools to the sub-execution:
// the sub-execution never receives database, vector-store or library
// credentials — it asks through this backend, which re-authorizes EVERY
// query against the existing ACL entrances and answers with the bounded C01
// material structure only.

// CraftExecutionKnowledgeFlag names the default-off deployment switch. Until
// the deployment explicitly enables it, minting and querying both refuse:
// supplemental retrieval is closed, not best-effort open.
const CraftExecutionKnowledgeFlag = "craft.execution_knowledge"

// Supplemental retrieval budget: one execution credential allows at most
// MaxExecutionKnowledgeCalls queries, and every answer is bounded to
// MaxExecutionKnowledgeBundleBytes of excerpt material (the C01 bundle cap).
const (
	MaxExecutionKnowledgeCalls       = 8
	MaxExecutionKnowledgeBundleBytes = craft.MaxKnowledgeBundleBytes
)

// CraftKnowledgeToolConfig assembles the tool. Every port is required; a nil
// port refuses assembly instead of degrading to a wider scope.
type CraftKnowledgeToolConfig struct {
	// Store resolves the delegated task scope-guarded (R02 store).
	Store craft.Store
	// Access is the existing shared-aware knowledge ACL read (C01 port).
	Access CraftKnowledgeAccess
	// Search is the existing per-library ACL-guarded retrieval (C01 port).
	Search CraftKnowledgeSearch
	// Enabled mirrors the deployment flag craft.execution_knowledge; the
	// zero value (false) is the default: the tool stays closed.
	Enabled bool
	// Now is injectable for tests; defaults to time.Now.
	Now func() time.Time
}

// CraftKnowledgeTool answers controlled supplemental knowledge queries for
// running craft delegations.
type CraftKnowledgeTool struct {
	store  craft.Store
	access CraftKnowledgeAccess
	search CraftKnowledgeSearch
	now    func() time.Time
	// cfgEnabled mirrors craft.execution_knowledge; false is the default.
	cfgEnabled bool

	mu     sync.Mutex
	grants map[string]*executionKnowledgeGrant
}

// NewCraftKnowledgeTool validates the assembly and returns the tool.
func NewCraftKnowledgeTool(cfg CraftKnowledgeToolConfig) (*CraftKnowledgeTool, error) {
	if cfg.Store == nil || cfg.Access == nil || cfg.Search == nil {
		return nil, fmt.Errorf("craft: execution knowledge tool requires store, access and search")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CraftKnowledgeTool{
		store: cfg.Store, access: cfg.Access, search: cfg.Search, now: now,
		cfgEnabled: cfg.Enabled, grants: map[string]*executionKnowledgeGrant{},
	}, nil
}

// CraftKnowledgeGrant is the short-term execution credential view handed to
// the caller of Mint. It carries identity and bounds only — no database,
// vector-store or library credential of any kind.
type CraftKnowledgeGrant struct {
	Token        string
	Scope        craft.Scope
	RunID        string
	TaskID       string
	KnowledgeIDs []string
	Deadline     time.Time
	CallsLeft    int
}

// executionKnowledgeGrant is the server-side grant state.
type executionKnowledgeGrant struct {
	scope        craft.Scope
	runID        string
	taskID       string
	knowledgeIDs []string
	deadline     time.Time
	callsLeft    int
	revoked      bool
}

// craftKnowledgeSnapshotOfTask extracts the task's STORED knowledge
// authorization snapshot: the knowledge coordinates of the craftkb://
// material inputs recorded when the task was prepared. Only these IDs can
// ever be queried — the snapshot, not the model's current wishes, is the
// scope.
func craftKnowledgeSnapshotOfTask(task craft.Task) []string {
	ids := make([]string, 0, len(task.Inputs))
	seen := make(map[string]bool, len(task.Inputs))
	for _, in := range task.Inputs {
		if !strings.HasPrefix(in.Ref, "craftkb://") {
			continue
		}
		id := craftKnowledgeIDOfRef(in.Ref)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// Mint binds one short-term execution credential to the exact tenant/user/
// session of the stored task, the task's OWN run (server-derived from the
// task's persisted fence — never from the caller's claim) and the task's
// stored knowledge snapshot. The expiry is clamped to the delegation's
// deadline: a credential never outlives the run that asked for it.
func (t *CraftKnowledgeTool) Mint(ctx context.Context, scope craft.Scope, taskID string, requested time.Time) (*CraftKnowledgeGrant, error) {
	if t == nil || t.store == nil {
		return nil, fmt.Errorf("%w: execution knowledge tool is not assembled", craft.ErrInvalidInput)
	}
	if !t.enabled() {
		return nil, fmt.Errorf("%w: supplemental retrieval is disabled (flag %s is false)", craft.ErrUnsupported, CraftExecutionKnowledgeFlag)
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("%w: mint requires the task id", craft.ErrInvalidInput)
	}
	task, err := t.store.GetTask(ctx, scope, taskID)
	if err != nil {
		return nil, err
	}
	snapshot := craftKnowledgeSnapshotOfTask(task)
	if len(snapshot) == 0 {
		return nil, fmt.Errorf("%w: task %s stored no knowledge authorization snapshot", craft.ErrInvalidInput, taskID)
	}
	deadline := requested
	if !task.Deadline.IsZero() && (deadline.IsZero() || deadline.After(task.Deadline)) {
		deadline = task.Deadline
	}
	if deadline.IsZero() || !t.now().Before(deadline) {
		return nil, fmt.Errorf("%w: the run deadline has passed; no execution credential may outlive it", craft.ErrInvalidInput)
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("craft: mint execution credential token: %w", err)
	}
	grant := &executionKnowledgeGrant{
		scope:        task.Scope,
		runID:        task.Fence.RunID,
		taskID:       task.ID,
		knowledgeIDs: snapshot,
		deadline:     deadline,
		callsLeft:    MaxExecutionKnowledgeCalls,
	}
	t.mu.Lock()
	t.grants[task.ID] = grant
	t.mu.Unlock()
	return &CraftKnowledgeGrant{
		Token: hex.EncodeToString(token), Scope: grant.scope, RunID: grant.runID,
		TaskID: grant.taskID, KnowledgeIDs: append([]string(nil), grant.knowledgeIDs...),
		Deadline: grant.deadline, CallsLeft: grant.callsLeft,
	}, nil
}

// Revoke cancels the task's execution credential immediately. Cancel and
// terminal transitions both land here; a revoked grant never answers again.
func (t *CraftKnowledgeTool) Revoke(scope craft.Scope, taskID string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if grant, ok := t.grants[taskID]; ok && craft.SameScope(scope, grant.scope) {
		grant.revoked = true
	}
}

// enabled is the default-off flag check.
func (t *CraftKnowledgeTool) enabled() bool {
	return t != nil && t.store != nil && t.access != nil && t.search != nil && t.cfgEnabled
}

// Query answers ONE supplemental retrieval for a running delegation.
//
// Order of refusal, every single call:
//
//  1. the flag must be enabled (default off);
//  2. the caller's context must carry the durable run fence, and the fence's
//     run must be the exact run the credential was minted for — a token
//     minted for another run is refused (cross-run rejection);
//  3. the stored task must exist in the caller's scope and carry a grant
//     that is not revoked, not expired (never past the run deadline) and not
//     out of call budget;
//  4. a task with a STORED terminal result is dead: cancellation and
//     terminal states revoke immediately;
//  5. the grant's stored knowledge snapshot is re-resolved through the
//     CURRENT ACL — the query scope is the snapshot ∩ the ACL of right now.
//     Documents whose authorization was revoked drop out; nothing the model
//     produced meanwhile (new knowledge ids, new libraries) can widen the
//     scope, because the signature carries no ids at all;
//  6. retrieval runs per OWNING library through the existing per-library
//     ACL-guarded search, restricted to the authorized documents of the
//     snapshot, and the answer is bounded to the C01 material structure —
//     excerpts, citation ids, durable refs and digests, at most 64 KiB per
//     query. No credential of the backing stores ever crosses this line.
func (t *CraftKnowledgeTool) Query(ctx context.Context, scope craft.Scope, taskID, query string) (craft.KnowledgeBundle, error) {
	if !t.enabled() {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: supplemental retrieval is disabled (flag %s is false)", craft.ErrUnsupported, CraftExecutionKnowledgeFlag)
	}
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(query) == "" {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: query requires the task id and a retrieval query", craft.ErrInvalidInput)
	}
	fence, ok := agentruntime.RunFenceFromContext(ctx)
	if !ok {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: supplemental retrieval requires the durable run fence", craft.ErrForbidden)
	}
	task, err := t.store.GetTask(ctx, scope, taskID)
	if err != nil {
		return craft.KnowledgeBundle{}, err
	}
	if !craft.SameScope(scope, task.Scope) {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: task %s belongs to another scope", craft.ErrForbidden, taskID)
	}
	// Terminal check: a stored result means the delegation finished; its
	// execution credential is dead even if budget and time remain.
	if result, rerr := t.store.GetResult(ctx, scope, taskID); rerr == nil && result.TaskID == taskID {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: task %s is terminal (%s); its execution credential is revoked", craft.ErrForbidden, taskID, result.Status)
	}

	t.mu.Lock()
	grant := t.grants[taskID]
	if grant != nil {
		defer func() { t.mu.Unlock() }()
	} else {
		t.mu.Unlock()
	}
	if grant == nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: no execution credential was minted for task %s", craft.ErrForbidden, taskID)
	}
	if grant.revoked {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: the execution credential for task %s was revoked", craft.ErrForbidden, taskID)
	}
	if !craft.SameScope(scope, grant.scope) {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: the execution credential for task %s belongs to another scope", craft.ErrForbidden, taskID)
	}
	if fence.RunID != grant.runID {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: the execution credential for task %s is bound to run %s and cannot serve run %s", craft.ErrForbidden, taskID, grant.runID, fence.RunID)
	}
	now := t.now()
	if !now.Before(grant.deadline) {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: the execution credential for task %s expired with the run deadline", craft.ErrForbidden, taskID)
	}
	if grant.callsLeft <= 0 {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: the call budget (%d queries) for task %s is exhausted", craft.ErrForbidden, MaxExecutionKnowledgeCalls, taskID)
	}
	grant.callsLeft--

	bundle, err := t.retrieve(ctx, scope, grant.knowledgeIDs, query)
	if err != nil {
		// The refused query consumed its budget slot (the retrieval ran);
		// fail-closed is returned as-is.
		return craft.KnowledgeBundle{}, err
	}
	return bundle, nil
}

// retrieve re-authorizes the snapshot against the CURRENT ACL and runs the
// bounded per-library retrieval. The intersection drops every document the
// ACL no longer returns; an empty intersection answers an empty bundle —
// narrowed, never widened, never a silent error.
func (t *CraftKnowledgeTool) retrieve(ctx context.Context, scope craft.Scope, snapshot []string, query string) (craft.KnowledgeBundle, error) {
	rows, err := t.access(ctx, scope.TenantID, snapshot)
	if err != nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: re-authorize knowledge snapshot: %w", err)
	}
	rowsByID := make(map[string]*types.Knowledge, len(rows))
	for _, row := range rows {
		if row != nil && row.ID != "" {
			rowsByID[row.ID] = row
		}
	}
	authorized := make([]string, 0, len(snapshot))
	kbOrder := make([]string, 0, len(rows))
	docsInKB := make(map[string][]string)
	kbTenant := make(map[string]uint64)
	for _, id := range snapshot {
		row := rowsByID[id]
		if row == nil {
			continue // revoked in the current ACL: dropped by the intersection
		}
		if _, ok := docsInKB[row.KnowledgeBaseID]; !ok {
			kbOrder = append(kbOrder, row.KnowledgeBaseID)
			kbTenant[row.KnowledgeBaseID] = row.TenantID
		}
		docsInKB[row.KnowledgeBaseID] = append(docsInKB[row.KnowledgeBaseID], row.ID)
		authorized = append(authorized, id)
	}
	if len(authorized) == 0 {
		return craft.KnowledgeBundle{Sources: []craft.Source{}}, nil
	}

	var sources []craft.Source
	seenChunks := make(map[string]bool)
	for _, kbID := range kbOrder {
		results, err := t.search(ctx, kbID, types.SearchParams{
			QueryText:             query,
			KnowledgeIDs:          docsInKB[kbID],
			MatchCount:            craft.MaxKnowledgeSources,
			SkipContextEnrichment: true,
		})
		if err != nil {
			if craftKnowledgeAccessDenied(err) {
				return craft.KnowledgeBundle{}, fmt.Errorf("%w: library %s denied the retrieval", craft.ErrForbidden, kbID)
			}
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: search library %s: %w", kbID, err)
		}
		allowed := make(map[string]bool, len(docsInKB[kbID]))
		for _, id := range docsInKB[kbID] {
			allowed[id] = true
		}
		for _, result := range results {
			if result == nil || result.ID == "" || seenChunks[result.ID] {
				continue
			}
			if !allowed[result.KnowledgeID] || isCraftWebReference(result) {
				continue
			}
			seenChunks[result.ID] = true
			excerpt := craft.ExcerptOf(result.Content, craft.MaxKnowledgeExcerptBytes)
			sources = append(sources, craft.Source{
				ID:       craft.KnowledgeCitationID(kbID, result.KnowledgeID, result.ID),
				Ref:      craft.KnowledgeRef(kbID, result.KnowledgeID, result.ID),
				Excerpt:  excerpt,
				Digest:   craftKnowledgeDigest(excerpt),
				TenantID: kbTenant[kbID],
			})
		}
	}
	return craft.BoundSources(sources, MaxExecutionKnowledgeBundleBytes), nil
}
