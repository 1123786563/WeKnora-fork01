package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	agentClaimRequestIDMaxBytes = 128
	agentClaimPollInterval      = 100 * time.Millisecond
	agentClaimRenewInterval     = 10 * time.Second // against the 30s store lease
)

// errAgentClaimResponded marks a claim rejection whose HTTP response was
// already recorded on the gin context by admitAgentChatTurn; handlers must
// return without attaching it to c.Errors a second time.
var errAgentClaimResponded = stderrors.New("agent chat turn claim response already written")

// canonicalQARequestHash fingerprints the execution-affecting request fields.
// requestID, tenant ids and generated ids are excluded; image and upload
// payloads are represented by digests; `attachments` carries the pre-uploaded
// session-scoped document IDs, so an identical retry yields the same hash.
func canonicalQARequestHash(req *CreateKnowledgeQARequest, attachments []string) string {
	imageDigests := make([]string, len(req.Images))
	for i, img := range req.Images {
		sum := sha256.Sum256([]byte(img.Data))
		imageDigests[i] = hex.EncodeToString(sum[:])
	}
	uploadDigests := make([][2]string, len(req.AttachmentUploads))
	for i, up := range req.AttachmentUploads {
		sum := sha256.Sum256([]byte(up.Data))
		uploadDigests[i] = [2]string{up.FileName, hex.EncodeToString(sum[:])}
	}
	payload, err := json.Marshal(struct {
		Query            string                 `json:"query"`
		AgentID          string                 `json:"agent_id,omitempty"`
		AgentEnabled     bool                   `json:"agent_enabled"`
		KnowledgeBaseIDs []string               `json:"knowledge_base_ids,omitempty"`
		KnowledgeIDs     []string               `json:"knowledge_ids,omitempty"`
		TagIDs           []string               `json:"tag_ids,omitempty"`
		MCPServiceIDs    []string               `json:"mcp_service_ids,omitempty"`
		SkillNames       []string               `json:"skill_names,omitempty"`
		SummaryModelID   string                 `json:"summary_model_id,omitempty"`
		WebSearchEnabled bool                   `json:"web_search_enabled"`
		ReasoningMode    string                 `json:"reasoning_mode,omitempty"`
		Channel          string                 `json:"channel,omitempty"`
		MentionedItems   []MentionedItemRequest `json:"mentioned_items,omitempty"`
		Images           []string               `json:"images,omitempty"`
		AttachmentUpload [][2]string            `json:"attachment_uploads,omitempty"`
		AttachmentIDs    []string               `json:"attachment_ids,omitempty"`
	}{
		Query:            req.Query,
		AgentID:          req.AgentID,
		AgentEnabled:     req.AgentEnabled,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		KnowledgeIDs:     req.KnowledgeIds,
		TagIDs:           req.TagIDs,
		MCPServiceIDs:    req.MCPServiceIDs,
		SkillNames:       req.SkillNames,
		SummaryModelID:   req.SummaryModelID,
		WebSearchEnabled: req.WebSearchEnabled,
		ReasoningMode:    req.ReasoningMode,
		Channel:          req.Channel,
		MentionedItems:   req.MentionedItems,
		Images:           imageDigests,
		AttachmentUpload: uploadDigests,
		AttachmentIDs:    attachments,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// admitAgentChatTurn resolves the agent WITHOUT writing files/rows, then
// admits the durable claim. Blocked verdict / infra error → 409/500 before
// any attachment persistence. Returns (claim, ReplayNew) only for new turns.
// A nil claim store (claims not assembled) keeps the legacy unfenced path.
func (h *Handler) admitAgentChatTurn(c *gin.Context, reqCtx *qaRequestContext, requestID string) (repository.AgentChatTurnClaim, bool) {
	if h.agentChatTurnClaimStore == nil {
		return repository.AgentChatTurnClaim{}, true
	}
	// The idempotency key is the client's X-Request-ID (echoed by the
	// middleware); the middleware-generated value is only the fallback.
	if raw := strings.TrimSpace(c.GetHeader("X-Request-ID")); raw != "" {
		requestID = raw
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > agentClaimRequestIDMaxBytes {
		c.Error(errors.NewBadRequestError(
			"X-Request-ID must be non-empty and at most 128 bytes for agent turns"))
		return repository.AgentChatTurnClaim{}, false
	}
	sourceTenantID := reqCtx.effectiveTenantID
	if sourceTenantID == 0 {
		sourceTenantID = reqCtx.session.TenantID
	}
	placeholder := *reqCtx.assistantMessage
	placeholder.ID = ""
	claim, replay, err := h.agentChatTurnClaimStore.Admit(reqCtx.ctx, repository.AgentChatTurnClaimInput{
		SourceTenantID:       sourceTenantID,
		SessionTenantID:      reqCtx.session.TenantID,
		SessionID:            reqCtx.sessionID,
		OwnerID:              reqCtx.session.UserID,
		RequestID:            requestID,
		RequestHash:          reqCtx.claimRequestHash,
		LeaseOwner:           uuid.NewString(),
		AgentID:              reqCtx.customAgent.ID,
		AssistantPlaceholder: &placeholder,
	})
	if err != nil {
		if stderrors.Is(err, repository.ErrAgentSecurityReleaseBlocked) ||
			stderrors.Is(err, repository.ErrAgentChatTurnClaimConflict) {
			c.Error(errors.NewConflictError("agent turn refused: " + err.Error()))
			return repository.AgentChatTurnClaim{}, false
		}
		logger.ErrorWithFields(reqCtx.ctx, err, map[string]interface{}{"session_id": reqCtx.sessionID})
		c.Error(errors.NewInternalServerError("agent turn admission failed"))
		return repository.AgentChatTurnClaim{}, false
	}
	if replay != repository.ReplayNew {
		c.Error(errors.NewConflictError("agent turn request already exists").WithDetails(gin.H{
			"assistant_message_id": claim.AssistantMessageID,
			"state":                claim.State,
		}))
		return repository.AgentChatTurnClaim{}, false
	}
	return claim, true
}

// loadClaimAgentSnapshot loads the immutable Version pinned by an adopted
// claim; the frozen snapshot, not the current CustomAgent, drives execution.
func (h *Handler) loadClaimAgentSnapshot(c *gin.Context, reqCtx *qaRequestContext, claim repository.AgentChatTurnClaim) (*types.CustomAgent, bool) {
	if h.agentVersionService == nil {
		c.Error(errors.NewInternalServerError("agent version service is unavailable"))
		return nil, false
	}
	snapshot, err := h.agentVersionService.GetAgentVersion(reqCtx.ctx, claim.SourceTenantID, *claim.LocalAgentVersionID)
	if err != nil || snapshot.Agent == nil || snapshot.Agent.ID != claim.AgentID {
		logger.ErrorWithFields(reqCtx.ctx, err, map[string]interface{}{
			"session_id": reqCtx.sessionID, "claim_id": claim.ID,
		})
		c.Error(errors.NewInternalServerError("pinned agent version is not loadable"))
		return nil, false
	}
	return snapshot.Agent, true
}

// watchAgentChatClaim polls the durable claim state every 100ms and renews the
// 10s heartbeat against the 30s lease. A terminal state, a lost lease, or a
// claim-read error closes the returned channel — local execution must then be
// cancelled (fail closed). The stop func ends the watcher without fencing.
func (h *Handler) watchAgentChatClaim(ctx context.Context, claim repository.AgentChatTurnClaim) (<-chan struct{}, func()) {
	ctx, stop := context.WithCancel(ctx)
	fenced := make(chan struct{})
	var fireOnce sync.Once
	fire := func() { fireOnce.Do(func() { close(fenced) }) }
	go func() {
		poll := time.NewTicker(agentClaimPollInterval)
		renew := time.NewTicker(agentClaimRenewInterval)
		defer poll.Stop()
		defer renew.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-renew.C:
				if err := h.agentChatTurnClaimStore.Renew(ctx, claim.SourceTenantID, claim.ID, claim.Generation, claim.LeaseOwner); err != nil {
					logger.Warnf(ctx, "agent claim renewal fenced, claim %s: %v", claim.ID, err)
					fire()
					return
				}
			case <-poll.C:
				if _, leaseValid, err := h.agentChatTurnClaimStore.GetForHandler(ctx, claim.SourceTenantID, claim.ID, claim.Generation, claim.LeaseOwner); err != nil || !leaseValid {
					logger.Warnf(ctx, "agent claim watcher fail-closed, claim %s: leaseValid=%v err=%v", claim.ID, leaseValid, err)
					fire()
					return
				}
			}
		}
	}()
	return fenced, stop
}

// claimWriteCtx scopes a fenced store write to the claim's source tenant.
func claimWriteCtx(ctx context.Context, claim repository.AgentChatTurnClaim) context.Context {
	return context.WithValue(ctx, types.TenantIDContextKey, claim.SourceTenantID)
}

// finishAgentChatTurnClaim performs the fenced terminal write: the final
// assistant content plus the claim's completed/failed transition in one
// transaction. After a user stop, expiry, or revocation the claim row already
// holds the terminal state, the CAS refuses the write, and history stays as
// fenced — the refusal is expected and only logged.
func (h *Handler) finishAgentChatTurnClaim(ctx context.Context, claim *repository.AgentChatTurnClaim, msg *types.Message, terminalState string) {
	reason := ""
	if terminalState == "failed" {
		reason = "agent execution failed"
	}
	msg.UpdatedAt = time.Now()
	msg.IsCompleted = true
	if _, err := h.agentChatTurnClaimStore.Finish(claimWriteCtx(ctx, *claim), claim.SourceTenantID, claim.ID, claim.Generation, claim.LeaseOwner, msg, terminalState, reason); err != nil {
		logger.Warnf(ctx, "agent claim terminal write fenced for message %s: %v", msg.ID, err)
	}
}
