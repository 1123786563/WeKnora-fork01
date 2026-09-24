# T19 activity-ID protocol research

**Research date:** 2026-09-24  
**Scope:** read-only investigation of pinned OpenCode 1.18.4 and the production model egress identity requirement. No application or test source was changed.

## Sources and verified facts

1. `docker/craft/opencode.lock.json` pins OpenCode `1.18.4`, source commit `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e`, and the local probe digest. `docker/craft/runtime-config.json` repeats the version/source pin and the per-architecture Linux release digests. The Dockerfile installs the release binary and starts `opencode serve`; it writes only a basic permission config and contains no provider endpoint, request-header, or plugin configuration.
2. The official OpenCode plugin contract documents a `chat.headers` hook. Its input is session/agent/model/provider/message and its output is a mutable `headers` map: [plugin `Hooks` definition](https://github.com/anomalyco/opencode/blob/49c69c5ed3ccf706b61b3febb43c8aaff7f8325e/packages/plugin/src/index.ts) (the current official page retains this contract). This can add headers to an LLM provider request, so a plugin is technically capable of setting a constant or newly generated header.
3. The official v1.18.4 release notes identify commit `49c69c5` and list only provider-specific fixes; they do not describe a per-attempt identity or retry hook: [v1.18.4 release](https://github.com/anomalyco/opencode/releases/tag/v1.18.4).
4. The OpenCode issue proposing a hook immediately before provider/inference dispatch explicitly describes `chat.params` and `chat.headers` as the existing hooks and proposes a new `chat.request.before` hook between those hooks and `streamText()`: [issue #21240](https://github.com/anomalyco/opencode/issues/21240). That is direct evidence that the existing 1.x plugin surface does not expose a documented pre-dispatch hook with the fully assembled request. The proposal also says the equivalent retry-aware hook would need to fire for each provider call, including retries.
5. The repository’s runtime contract says model access flows through a server-provided gateway and OpenCode never receives tenant/platform credentials (`docker/craft/runtime-config.json`, `credentials.rule`). The WeKnora client’s headers therefore terminate at the OpenCode HTTP server unless OpenCode explicitly copies them; they are not evidence of OpenCode→gateway propagation.

## Result

### Verified capability

OpenCode 1.18.4 has a `chat.headers` plugin hook that can mutate headers on the provider request. That is sufficient for a value known by the plugin at hook time, but the hook input has session/model context rather than a durable physical-attempt record. The pinned image has no checked-in plugin implementing it, no durable attempt store, and no configured egress adapter.

### Not established / cannot satisfy T19

No official 1.18.4 documentation or pinned repository configuration establishes all of the required properties together:

* allocation and durable commit of an opaque ID immediately before each physical provider attempt;
* reuse of that same ID after an unknown send/retry;
* a distinct ID for a deliberately new physical attempt;
* authenticated binding of the ID to tenant/run/delegation and a durable attempt record; and
* a fail-closed guarantee that every model request reaches the Craft gateway with that proof.

`chat.headers` is therefore not enough to close the High finding. A plugin could generate a UUID per hook invocation, but that would be unsafe if the hook runs again during retry or if a request outcome is unknown. A UUID derived from session, prompt, model, timestamp, TCP connection, or body hash has the same ambiguity and cannot prove physical-attempt identity. A WeKnora→OpenCode header alone cannot force forwarding into OpenCode’s provider request.

## Minimal viable protocol if no stronger pinned hook is proven

Use a private per-Run model-egress adapter/sidecar as the only configured provider endpoint:

1. OpenCode is network-denied from direct provider egress and sends provider traffic only to the adapter.
2. The adapter authenticates the container/Run identity, then durably records `(tenant, run, delegation, model-call ordinal, attempt ID)` before forwarding. The ID is opaque and scoped to that record.
3. The adapter injects `X-Craft-Activity-ID` (or an adapter-issued signed attempt proof containing the same identity) on the adapter→Craft gateway request. The gateway validates the proof against the authenticated execution credential and durable attempt record; a caller-selected syntactically valid header is rejected.
4. If forwarding or the provider result is ambiguous, the adapter reuses the existing attempt ID and reconciles/parks. It must not create a new ID and resend the provider request. A deliberately new model attempt allocates a new ordinal/ID.
5. The adapter’s journal must persist across the relevant Run/container restart boundary and correlate to RunView/container generation. `StartBinding` remains the one-shot durable charge-start fence; response-header success is only initiation evidence.

This is an irreducible requirement: a plain header injector or stateless HTTP proxy cannot distinguish retry-after-unknown from a new physical attempt. Keep the gateway fail-closed until a live pinned-image test demonstrates the exact OpenCode provider route, header propagation, unknown retry behavior, and no direct-egress bypass.

## Evidence limits

The lock-specified local probe `/Users/wuyongjun/.opencode/bin/opencode` was rechecked: it reports `1.18.4` and hashes to `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`, matching `opencode.lock.json`. The unqualified PATH command resolves separately to `/opt/homebrew/bin/opencode` version `1.18.7`; that binary was not used as evidence. The pinned Linux image was not built in this read-only investigation, so no claim is made about Linux runtime behavior. A bounded localhost mock-provider test remains the appropriate follow-up once the exact pinned image and gateway endpoint are assembled.
