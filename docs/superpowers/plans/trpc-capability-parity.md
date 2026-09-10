# tRPC capability parity

The tRPC graph receives the request-scoped capability assembly used by the
builtin agent. The graph checkpoint stores only `CapabilitySnapshot`; provider
clients, registries, sandbox handles, and approval waiters remain live objects
and are rebuilt by the application on recovery.

| Capability | Reuse entry point | Snapshot/evidence | Recovery behavior |
|---|---|---|---|
| RAG scope and rerank | `agentService.registerTools`, existing knowledge tools and reranker | Tool identities plus the request's existing KB/document scope | Rebuild the same authorized scope before resuming; no new scope is inferred from a checkpoint |
| MCP deferred discovery | `registerMCPTools` → `ToolRegistry.RegisterDeferredTool` / `PrepareMCPTools` | Stable tool identities and `DeferredNames`; each session owns a registry | Re-register and prepare through the existing catalog, then compare the advertised set; sessions do not share deferred state |
| MCP approval and OAuth | Existing MCP preflight and approval interfaces | Approval is represented by the durable decision contract, not a live waiter | A pending decision must be resolved by the durable run service; the legacy in-memory Gate remains the builtin path |
| Skills read/install/env | Existing `skills.Manager` and sandbox staging | Metadata-derived `sha256:` skill digests | Rebuild the manager and reject a changed digest instead of silently continuing |
| Shell and file tools | Existing sandbox registration in `prepareAgentCapabilities` | Tool identities | Recreate the session-bound sandbox and tools; an unavailable resource remains a recovery decision |
| Model stream, tool arguments and images | Existing `chat.Chat` through `trpc.NewModel` | Model attempt/usage in `State`; image references in snapshot | Provider credentials and limits are reloaded from current configuration; incomplete streams never become plans |
| VLM fallback | Existing `AgentCapabilities.ImageDescriber` | Image references are serializable; VLM handle is not | Re-resolve the configured VLM model; failure is reported rather than replaced with fabricated text |
| Long-term memory | Existing memory service and `WrapMemoryForPrompt` envelope | `MemoryPrompt` in `CapabilitySnapshot` | The exact recalled envelope is restored; recovery does not perform a second implicit recall |
| System prompt | Existing `BuildSystemPromptWithOptions` result | `SystemPrompt` in `CapabilitySnapshot` | The same prompt is restored before the user message |
| Compaction | Existing compaction policy and durable `CompactionState` | Versioned compaction state plus input cursor | Resume from the committed boundary; the same boundary is not compacted twice |
| Citations and artifacts | Existing tool result and message projections | Tool/message state remains separate from capability metadata | Native tool output and model-facing clipped content retain separate boundaries |
| Model usage | Existing provider response usage mapped by `trpc.NewModel` | Versioned `UsageAttempts` | A committed attempt is reused during recovery; an uncommitted attempt is not reported as success |

The focused verification is:

```text
GOWORK=off go test ./internal/agent/trpc ./internal/application/service \
  -run 'TestCapability|TestAgentCapabilities|TestState|TestNewGraphRunner' -count=1
```

The capability snapshot and graph binding tests pass. Full production tRPC
graph construction, durable OAuth waiter routing, and provider-specific
sandbox reconciliation remain owned by the surrounding recovery tasks; this
document does not claim those paths are enabled by the snapshot alone.
