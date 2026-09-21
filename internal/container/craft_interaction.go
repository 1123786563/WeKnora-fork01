package container

// C02 craft interaction assembly: the production interaction store, the
// control service on it, the reliable decision delivery worker with its
// redelivery sweep, and the durable registration hook that intercepts the
// executor's interaction.pending events. Everything here is additive: when
// CRAFT_OPENCODE_BASE_URL is unset the replier is nil and every decision
// stays honestly delivery-unknown instead of being silently approved.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// craftInteractionSweepInterval spaces the outbox redelivery sweeps.
const craftInteractionSweepInterval = 5 * time.Second

// CraftInteractionAssembly bundles the C02 surfaces for the container.
type CraftInteractionAssembly struct {
	Store    *service.GormCraftInteractionStore
	Control  *service.CraftControlService
	Delivery *service.CraftDecisionDelivery
	Runs     service.CraftRunController
	Client   *opencode.Client
}

// newCraftInteractionAssembly assembles the C02 interaction chain. It never
// fails the boot: a missing OpenCode endpoint keeps delivery fail-closed.
// The control service deliberately takes no executor: the R06 stop surface
// keeps its recorded-intent stage here, and taking the executor would create
// a provider cycle (the runtime executor consumes this assembly for the
// interaction.pending registration hook).
func newCraftInteractionAssembly(
	db *gorm.DB,
	store craft.Store,
	runtime *AgentRuntime,
) *CraftInteractionAssembly {
	interactions := service.NewGormCraftInteractionStore(db)
	runs := craftRunsController(runtime)
	assembly := &CraftInteractionAssembly{Store: interactions, Runs: runs}
	if baseURL := strings.TrimSpace(os.Getenv(craftOpenCodeBaseURLEnv)); baseURL != "" {
		if client, err := opencode.NewClient(baseURL, nil); err == nil {
			assembly.Client = client
		} else {
			logger.Warnf(context.Background(), "[CraftInteraction] opencode client unavailable: %v", err)
		}
	}
	assembly.Control = service.NewCraftControlService(runs, store, nil, interactions, assembly.Client)
	assembly.Delivery = service.NewCraftDecisionDelivery(interactions, interactions, assembly.Client, runs)
	return assembly
}

// craftRunsController resolves the run controller the same way the session
// service does: the live runtime's service when present, the registered one
// otherwise.
func craftRunsController(runtime *AgentRuntime) service.CraftRunController {
	if runtime != nil && runtime.Runs != nil {
		return runtime.Runs
	}
	if runs := service.RegisteredAgentRunService(); runs != nil {
		return runs
	}
	return nil
}

// registerCraftInteractionHTTPHandlers installs the handler for routing.
func registerCraftInteractionHTTPHandlers(assembly *CraftInteractionAssembly) {
	if assembly == nil {
		return
	}
	sessionhandler.RegisterCraftInteractionHandler(assembly.Control)
}

// startCraftDecisionDelivery runs the outbox redelivery sweep: every few
// seconds it takes the runs with undelivered decisions and hands them to the
// delivery worker. The sweep holds no run lease and parks no goroutine per
// decision — waiting stays durable state only.
func startCraftDecisionDelivery(assembly *CraftInteractionAssembly, cleaner interfaces.ResourceCleaner) {
	if assembly == nil || assembly.Delivery == nil {
		return
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(craftInteractionSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				keys, err := assembly.Store.PendingDecisionRuns(ctx)
				if err != nil {
					logger.Warnf(ctx, "[CraftDelivery] sweep listing failed: %v", err)
				}
				for _, key := range keys {
					if err := assembly.Delivery.Deliver(ctx, key, ""); err != nil {
						logger.Warnf(ctx, "[CraftDelivery] redelivery for run %s: %v", key.RunID, err)
					}
				}
				cancel()
			case <-stop:
				return
			}
		}
	}()
	cleaner.RegisterWithName("CraftDecisionDelivery", func() error {
		close(stop)
		return nil
	})
}

// craftInteractionRegistrar wraps the run event emitter so an
// interaction.pending event first registers the durable interaction (payload
// extracted best-effort from the OpenCode message snapshot) and parks the
// main run at waiting_user, then emits as before.
func craftInteractionRegistrar(
	client *opencode.Client,
	store craft.Store,
	interactions *service.GormCraftInteractionStore,
	runs service.CraftRunController,
	inner func(context.Context, craft.Task, string, json.RawMessage) error,
) func(context.Context, craft.Task, string, json.RawMessage) error {
	return func(ctx context.Context, task craft.Task, kind string, data json.RawMessage) error {
		if kind == "interaction.pending" {
			registerPendingInteraction(ctx, client, store, interactions, runs, task, data)
		}
		return inner(ctx, task, kind, data)
	}
}

// registerPendingInteraction performs one best-effort durable registration:
// the interaction row carries the structured payload from the start, and the
// main run parks at waiting_user under the executor's live fence. A failure
// is logged and never blocks the executor — the request keeps waiting for a
// human explicitly (fail-closed R06 semantics).
func registerPendingInteraction(
	ctx context.Context,
	client *opencode.Client,
	store craft.Store,
	interactions *service.GormCraftInteractionStore,
	runs service.CraftRunController,
	task craft.Task,
	data json.RawMessage,
) {
	var event struct {
		Kind   string `json:"kind"`
		PartID string `json:"part_id"`
	}
	if err := json.Unmarshal(data, &event); err != nil || event.PartID == "" {
		return
	}
	kind := event.Kind
	if kind != craft.InteractionQuestion && kind != craft.InteractionPermission {
		return
	}
	workspace, err := store.GetWorkspace(ctx, task.Scope)
	if err != nil {
		logger.Warnf(ctx, "[CraftInteraction] registration skipped for %s: workspace: %v", task.ID, err)
		return
	}
	prompt := kind + " request"
	pending := craft.PendingDecision{}
	if client != nil {
		if snapshot, perr := client.Messages(ctx, workspace.OpenCodeSessionID); perr == nil {
			prompt, pending = interactionPayloadOf(snapshot, event.PartID, kind, prompt)
		}
	}
	if pending.Prompt == "" {
		pending.Prompt = prompt
	}
	// The payload carries the full embedded identity so server-side answer
	// validation sees the exact kind it decides on.
	pending.Interaction.ID = craftInteractionIDFor(task.Scope, task.ID, event.PartID)
	pending.Interaction.Kind = kind
	pending.Interaction.ArgsHash = craftInteractionArgsHashFor(kind, prompt, event.PartID)
	pending.Interaction.Prompt = prompt
	interaction := craft.Interaction{
		ID:       craftInteractionIDFor(task.Scope, task.ID, event.PartID),
		Kind:     kind,
		ArgsHash: craftInteractionArgsHashFor(kind, prompt, event.PartID),
		Prompt:   prompt,
	}
	record, err := interactions.PutInteraction(ctx, service.CraftInteractionRecord{
		Interaction:       interaction,
		Scope:             task.Scope,
		RunID:             task.Fence.RunID,
		TaskID:            task.ID,
		ToolCallID:        task.ToolCallID,
		PendingID:         interaction.ID,
		OpenCodeSessionID: workspace.OpenCodeSessionID,
		OpenCodeRequestID: event.PartID,
		Pending:           pending,
	})
	if err != nil {
		logger.Warnf(ctx, "[CraftInteraction] durable registration failed for part %s (the request keeps waiting): %v",
			event.PartID, err)
		return
	}
	if runs != nil {
		if err := runs.WaitForDecision(ctx, task.Fence, record.PendingID); err != nil {
			logger.Warnf(ctx, "[CraftInteraction] park for %s failed (retried on the next registration): %v", record.ID, err)
		}
	}
	logger.Infof(ctx, "[CraftInteraction] registered %s part %s (run %s) — waiting_user",
		kind, event.PartID, task.Fence.RunID)
}

// craftInteractionIDFor mirrors the R06 control service's stable identity
// derivation (scope + delegation + OC request id): stable across retries.
func craftInteractionIDFor(scope craft.Scope, taskID, ocRequestID string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-interaction/%d/%s/%s/%s",
		scope.TenantID, scope.SessionID, taskID, ocRequestID)))
	return "itx_" + hex.EncodeToString(sum[:16])
}

func craftInteractionArgsHashFor(kind, prompt, ocRequestID string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + prompt + "\x00" + ocRequestID))
	return "iargs_" + hex.EncodeToString(sum[:16])
}

// interactionPayloadOf extracts the pending question/permission payload from
// the OpenCode message snapshot, tolerant of the locked runtime's exact
// metadata shape: recognized shapes populate the structured fields, anything
// else degrades to the prompt text only (never fabricated options).
func interactionPayloadOf(messages []opencode.Message, partID, kind, fallbackPrompt string) (string, craft.PendingDecision) {
	pending := craft.PendingDecision{}
	for _, message := range messages {
		for _, raw := range message.Parts {
			var part struct {
				ID       string          `json:"id"`
				Type     string          `json:"type"`
				Tool     string          `json:"tool"`
				Text     string          `json:"text"`
				Metadata json.RawMessage `json:"metadata"`
			}
			if err := json.Unmarshal(raw, &part); err != nil || part.ID != partID {
				continue
			}
			prompt := strings.TrimSpace(part.Text)
			if prompt == "" {
				prompt = fallbackPrompt
			}
			switch kind {
			case craft.InteractionQuestion:
				pending.Options, pending.Multiple = questionOptionsOf(part.Metadata, partID)
			case craft.InteractionPermission:
				pending.Permissions = []craft.PermissionItem{permissionItemOf(part.Metadata, part.Tool)}
			}
			return prompt, pending
		}
	}
	return fallbackPrompt, pending
}

// questionOptionsOf decodes the question metadata tolerantly: either a bare
// list of options or a list of {label,value} pairs; one question per part
// (multi-question decisions arrive as multiple parts, each its own pending).
func questionOptionsOf(metadata json.RawMessage, partID string) (map[string][]string, map[string]bool) {
	options := map[string][]string{}
	multiple := map[string]bool{}
	if len(metadata) == 0 {
		return options, multiple
	}
	var shaped struct {
		Options []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"options"`
		Multiple bool `json:"multiple"`
	}
	if err := json.Unmarshal(metadata, &shaped); err == nil && len(shaped.Options) > 0 {
		for _, option := range shaped.Options {
			value := option.Value
			if value == "" {
				value = option.Label
			}
			if value != "" {
				options[partID] = append(options[partID], value)
			}
		}
		multiple[partID] = shaped.Multiple
		return options, multiple
	}
	var plain []string
	if err := json.Unmarshal(metadata, &plain); err == nil {
		for _, value := range plain {
			if value != "" {
				options[partID] = append(options[partID], value)
			}
		}
	}
	return options, multiple
}

// permissionItemOf decodes the permission metadata tolerantly: the command
// (or pattern), the touched path and the scope it applies to.
func permissionItemOf(metadata json.RawMessage, tool string) craft.PermissionItem {
	item := craft.PermissionItem{Tool: tool}
	if len(metadata) == 0 {
		return item
	}
	var shaped struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Scope   string `json:"scope"`
	}
	if err := json.Unmarshal(metadata, &shaped); err != nil {
		return item
	}
	item.Command = firstNonEmpty(shaped.Command, shaped.Pattern, shaped.Type)
	item.Path = shaped.Path
	item.Scope = shaped.Scope
	return item
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
