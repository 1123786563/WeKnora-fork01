// CFT-S00-T002 craft command ports — the frozen contract between the design's
// internal command vocabulary and the VERIFIED HTTP DTOs.
//
// Frozen mapping (design example -> existing wire, verified in T001):
//   DraftIntent.text                    -> prompt            (POST /craft/runs)
//   DraftIntent.inputs[].ref            -> input_refs[]
//   DraftIntent.knowledge[]             -> knowledge_scope   (server-encoded scope string)
//   DraftIntent.baseVersionId           -> base_version_id
//   DraftIntent.expectedWorkspaceRevision -> NOT on the wire today: the run DTO
//       has no CAS field (internal/handler/session/craft.go craftRunRequestDTO);
//       admission is protected by request_id replay + the live-run guard. The
//       field stays on the frozen intent (DECISIONS.md D003) so T016 can raise
//       it to a real wire CAS without touching every caller.
//   RestoreIntent.versionId             -> snapshot_id       (POST /craft/restore
//       carries {request_id, snapshot_id, revision} — revision IS the CAS there)
//   decidePermission/answerQuestion     -> POST .../interactions/:iid/decide
//       {action, args_hash, expected_revision, answers|answer}
//
// Identity rule: tenant/user are derived from the authenticated transport.
// Any client-supplied authority fact (sandbox_url, tenant_id, user_id, ...)
// on a command object is a contract error — stripped silently it would look
// like consent; rejected it stays auditable.

import { CRAFT_SESSION_KINDS, type CraftSessionKind } from './index.ts';

/** Actions the decide endpoint actually distinguishes (craft_interaction.go). */
export const CRAFT_DECISION_ACTIONS = ['allow_once', 'deny', 'answer'] as const;
export type CraftDecisionAction = (typeof CRAFT_DECISION_ACTIONS)[number];

/** Fields a client must never supply on a craft command (authority facts). */
const CLIENT_AUTHORITY_FIELDS = [
  'sandbox_url', 'sandbox_id', 'tenant_id', 'user_id', 'owner_id',
  'api_key', 'model', 'model_id', 'admin', 'role', 'budget', 'credits',
] as const;

export interface CraftSubmitDraftCommand {
  /** One user submit intent; same intent reuses it across retries. */
  request_id: string;
  session_id: string;
  /** Design DraftIntent.text — trimmed before freeze, non-empty. */
  prompt: string;
  kind: CraftSessionKind;
  /** Ready input references only (upload completed, associated). */
  input_refs: string[];
  /** Server-encoded knowledge scope; resources are data, not permission. */
  knowledge_scope: string;
  /** Current editing baseline, NOT the previewed historical version. */
  base_version_id: string;
  /** Target-only CAS (D003): kept on the intent, not yet on the wire. */
  expected_workspace_revision: number | null;
}

export interface CraftRestoreCommand {
  request_id: string;
  session_id: string;
  snapshot_id: string;
  /** Mandatory CAS: the workspace revision the user confirmed. */
  revision: number;
}

export interface CraftDecisionAnswer {
  question_id: string;
  choices: string[];
  text: string;
}

export interface CraftDecisionCommand {
  request_id: string;
  session_id: string;
  interaction_id: string;
  action: CraftDecisionAction;
  /** Bind the decision to the exact request payload (permission scope). */
  args_hash: string;
  /** Mandatory CAS: the interaction revision this decision answers. */
  expected_revision: number;
  /** Question answers (action='answer'). */
  answers: CraftDecisionAnswer[];
}

function rejectAuthorityFields(value: object, label: string): void {
  for (const field of CLIENT_AUTHORITY_FIELDS) {
    if (field in value && (value as Record<string, unknown>)[field] !== undefined) {
      throw new Error(`invalid ${label}: client must not supply ${field} (identity/authority comes from the authenticated transport)`);
    }
  }
}

function nonBlank(value: unknown, field: string, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`invalid ${label} (${field})`);
  }
  return value;
}

function positiveRevision(value: unknown, field: string, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) {
    throw new Error(`invalid ${label} (${field}: must be a positive integer)`);
  }
  return value;
}

export function validateCraftSubmitDraftCommand(value: unknown): CraftSubmitDraftCommand {
  const v = value as Record<string, unknown>;
  if (typeof v !== 'object' || v === null) throw new Error('invalid craft draft command');
  rejectAuthorityFields(v, 'craft draft command');
  const kind = v.kind;
  if (typeof kind !== 'string' || !(CRAFT_SESSION_KINDS as readonly string[]).includes(kind)) {
    throw new Error(`invalid craft draft command (kind)`);
  }
  const refs = v.input_refs;
  if (!Array.isArray(refs) || refs.some((ref) => typeof ref !== 'string' || ref.trim() === '')) {
    throw new Error('invalid craft draft command (input_refs: must be non-empty strings)');
  }
  return {
    request_id: nonBlank(v.request_id, 'request_id', 'craft draft command'),
    session_id: nonBlank(v.session_id, 'session_id', 'craft draft command'),
    prompt: nonBlank(v.prompt, 'prompt', 'craft draft command').trim(),
    kind: kind as CraftSessionKind,
    input_refs: refs as string[],
    knowledge_scope: typeof v.knowledge_scope === 'string' ? v.knowledge_scope : '',
    base_version_id: typeof v.base_version_id === 'string' ? v.base_version_id : '',
    expected_workspace_revision:
      v.expected_workspace_revision === undefined || v.expected_workspace_revision === null
        ? null
        : positiveRevision(v.expected_workspace_revision, 'expected_workspace_revision', 'craft draft command'),
  };
}

export function validateCraftRestoreCommand(value: unknown): CraftRestoreCommand {
  const v = value as Record<string, unknown>;
  if (typeof v !== 'object' || v === null) throw new Error('invalid craft restore command');
  rejectAuthorityFields(v, 'craft restore command');
  return {
    request_id: nonBlank(v.request_id, 'request_id', 'craft restore command'),
    session_id: nonBlank(v.session_id, 'session_id', 'craft restore command'),
    snapshot_id: nonBlank(v.snapshot_id, 'snapshot_id', 'craft restore command'),
    revision: positiveRevision(v.revision, 'revision', 'craft restore command'),
  };
}

export function validateCraftDecisionCommand(value: unknown): CraftDecisionCommand {
  const v = value as Record<string, unknown>;
  if (typeof v !== 'object' || v === null) throw new Error('invalid craft decision command');
  rejectAuthorityFields(v, 'craft decision command');
  const action = v.action;
  if (typeof action !== 'string' || !(CRAFT_DECISION_ACTIONS as readonly string[]).includes(action)) {
    throw new Error('invalid craft decision command (action)');
  }
  const answers = v.answers === undefined || v.answers === null ? [] : v.answers;
  if (!Array.isArray(answers)) throw new Error('invalid craft decision command (answers)');
  if (action === 'answer' && answers.length === 0) {
    throw new Error('invalid craft decision command (answers: an answer decision carries at least one answer)');
  }
  return {
    request_id: nonBlank(v.request_id, 'request_id', 'craft decision command'),
    session_id: nonBlank(v.session_id, 'session_id', 'craft decision command'),
    interaction_id: nonBlank(v.interaction_id, 'interaction_id', 'craft decision command'),
    action: action as CraftDecisionAction,
    args_hash: typeof v.args_hash === 'string' ? v.args_hash : '',
    expected_revision: positiveRevision(v.expected_revision, 'expected_revision', 'craft decision command'),
    answers: answers.map((a) => {
      const row = a as Record<string, unknown>;
      return {
        question_id: nonBlank(row.question_id, 'answers.question_id', 'craft decision command'),
        choices: Array.isArray(row.choices) ? (row.choices as string[]) : [],
        text: typeof row.text === 'string' ? row.text : '',
      };
    }),
  };
}

/** Exact POST /sessions/:id/craft/runs body (craftRunRequestDTO). */
export function craftDraftWireBody(cmd: CraftSubmitDraftCommand): {
  request_id: string;
  prompt: string;
  input_refs: string[];
  knowledge_scope: string;
  base_version_id: string;
} {
  return {
    request_id: cmd.request_id,
    prompt: cmd.prompt,
    input_refs: [...cmd.input_refs],
    knowledge_scope: cmd.knowledge_scope,
    base_version_id: cmd.base_version_id,
  };
}

/** Exact POST /sessions/:id/craft/restore body (craftRestoreRequestDTO). */
export function craftRestoreWireBody(cmd: CraftRestoreCommand): {
  request_id: string;
  snapshot_id: string;
  revision: number;
} {
  return { request_id: cmd.request_id, snapshot_id: cmd.snapshot_id, revision: cmd.revision };
}

/** Exact POST .../interactions/:iid/decide body (decideRequestBody). */
export function craftDecisionWireBody(cmd: CraftDecisionCommand): {
  decision_id: string;
  action: string;
  args_hash: string;
  expected_revision: number;
  answers: CraftDecisionAnswer[];
} {
  return {
    decision_id: '',
    action: cmd.action,
    args_hash: cmd.args_hash,
    expected_revision: cmd.expected_revision,
    answers: cmd.answers.map((a) => ({ question_id: a.question_id, choices: [...a.choices], text: a.text })),
  };
}
