// CFT-S01-T008: the craft command bridge — the single ordering owner for a
// submit intent. The routes assembly keeps its upload/poll/associate worker;
// the bridge freezes the T002 contract around it:
//
//   1. EVERY attachment must be uploaded+associated BEFORE exactly one run
//      submission. An upload failure submits zero runs and the thrown error
//      names the failing attachment.
//   2. The requestId is caller-owned and minted ONCE per intent. The bridge
//      passes it verbatim; a lost submit response retries with the same key
//      and an identical payload, so the server replays the admission
//      (request_id idempotency, Go-side pinned) instead of admitting twice.
//   3. baseVersionId is the editing baseline ONLY. There is no viewVersion
//      concept on this input — a previewed historical version can never
//      leak into the frozen intent.
//
// Reconnects never auto-submit: this module runs ONLY from the composer's
// send path (the controller's SSE recovery stays read-only by design).
import {
  validateCraftSubmitDraftCommand,
  craftDraftWireBody,
} from '@weknora/contracts';

/** One attachment awaiting upload; the file handle itself is opaque here. */
export interface CraftBridgeAttachment {
  name: string;
  sha256: string;
  file: unknown;
}

export interface CraftBridgePorts {
  /** Upload + poll + associate; resolves the canonical resource:// ref. */
  uploadAndAssociate(attachment: CraftBridgeAttachment, signal?: AbortSignal): Promise<string>;
  /** Submit the frozen run command (validated + serialized by the bridge). */
  submitDraft(command: {
    request_id: string;
    prompt: string;
    input_refs: string[];
    knowledge_scope: string;
    base_version_id: string;
  }, signal?: AbortSignal): Promise<unknown>;
}

export interface CraftSubmitDraftIntent {
  sessionId: string;
  prompt: string;
  kind: 'web' | 'document' | 'spreadsheet' | 'slides';
  knowledgeScope: string;
  /** The editing baseline — never the previewed version. */
  baseVersionId: string | null;
  /** Caller-minted, one per intent; retries MUST pass the same key. */
  requestId: string;
  /** Target-only CAS (D003): kept on the intent, not on the wire. */
  expectedWorkspaceRevision: number | null;
  attachments?: readonly CraftBridgeAttachment[];
  signal?: AbortSignal;
}

/**
 * Uploads+associates every attachment in order, then submits exactly one
 * run command. The submit payload is frozen from the SAME intent on every
 * attempt — retries reuse the caller's requestId verbatim.
 */
export async function submitDraftWithAttachments(
  intent: CraftSubmitDraftIntent,
  ports: CraftBridgePorts,
): Promise<unknown> {
  const command = validateCraftSubmitDraftCommand({
    request_id: intent.requestId,
    session_id: intent.sessionId,
    prompt: intent.prompt,
    kind: intent.kind,
    input_refs: [],
    knowledge_scope: intent.knowledgeScope,
    base_version_id: intent.baseVersionId ?? '',
    expected_workspace_revision: intent.expectedWorkspaceRevision,
  });
  const wire = craftDraftWireBody(command);

  const refs: string[] = [];
  for (const attachment of intent.attachments ?? []) {
    refs.push(await ports.uploadAndAssociate(attachment, intent.signal));
  }
  wire.input_refs = refs;
  return ports.submitDraft(wire, intent.signal);
}
