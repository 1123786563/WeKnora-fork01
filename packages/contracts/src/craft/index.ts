// W04 craft wire contracts.
//
// The craft HTTP surface answers `{success, data}` envelopes (api-client
// unwraps them); the parsers below re-validate every field of the `data`
// projections so no unvalidated server data reaches the workbench: a missing
// id, an unknown status/kind, or an illegal seq is a thrown contract error,
// never a type assertion. Downloads are intentionally NOT modelled here —
// file bytes travel through the platform transport, never through domain
// state.

/** Craft artifact kinds (internal/craft/request.go closed set). */
export const CRAFT_SESSION_KINDS = ['web', 'document', 'spreadsheet', 'slides'] as const;
export type CraftSessionKind = (typeof CRAFT_SESSION_KINDS)[number];

/** Main run statuses projected by runView (waiting_user carries pending_id). */
export const CRAFT_RUN_STATUSES = ['queued', 'running', 'stopping', 'waiting_user', 'succeeded', 'failed', 'canceled', 'recovering'] as const;
export type CraftRunStatus = (typeof CRAFT_RUN_STATUSES)[number];

/** Only these run statuses end the main message; child events never do. */
export const CRAFT_TERMINAL_RUN_STATUSES = ['succeeded', 'failed', 'canceled'] as const;
export type CraftTerminalRunStatus = (typeof CRAFT_TERMINAL_RUN_STATUSES)[number];

/** Version check outcomes (internal/craft/contracts.go). */
export const CRAFT_CHECK_STATUSES = ['passed', 'failed', 'not_run'] as const;
export type CraftCheckStatus = (typeof CRAFT_CHECK_STATUSES)[number];

/** RunEvent.payload.kind vocabulary (master plan §5). */
export const CRAFT_EVENT_KINDS = ['delegation.started', 'delegation.text', 'delegation.tool', 'delegation.finished', 'artifact.published', 'interaction.pending', 'interaction.resolved', 'workspace.unavailable'] as const;
export type CraftEventKind = (typeof CRAFT_EVENT_KINDS)[number];

const CRAFT_EVENT_KIND_PREFIXES = ['delegation.', 'artifact.', 'interaction.', 'workspace.'] as const;

export interface CraftSessionCreatedView {
  session_id: string;
  workspace_id: string;
  engine_type: string;
}

export interface CraftSessionSummaryView {
  session_id: string;
  workspace_id: string;
  kind: CraftSessionKind;
  title: string;
  engine_type: string;
  updated_at: string;
}

/** Deployment gate snapshot riding on the list/create/workspace envelopes. */
export interface CraftCapabilitiesView {
  enabled: boolean;
  allowed_kinds: CraftSessionKind[];
}

export interface CraftSessionPageView {
  data: CraftSessionSummaryView[];
  next_cursor: string | null;
  /** Present since CFT-S01-T009 (list carries it for the entry pages). */
  capabilities?: CraftCapabilitiesView | null;
}

export interface CraftRunView {
  run_id: string;
  session_id: string;
  status: CraftRunStatus;
  wait_reason: string;
  revision: number;
  epoch: number;
  /** Server-assigned event watermark; 0 on the POST /runs admission view. */
  seq: number;
  pending_id: string | null;
}

export interface CraftFileVersionView {
  path: string;
  ref: string;
  sha256: string;
  mime: string;
  bytes: number;
}

export interface CraftVersionCheckView {
  name: string;
  status: CraftCheckStatus;
  detail: string;
}

export interface CraftVersionView {
  id: string;
  workspace_id: string;
  run_id: string;
  kind: string;
  files: CraftFileVersionView[];
  checks: CraftVersionCheckView[];
}

export interface CraftVersionsPageView {
  data: CraftVersionView[];
  next_cursor: string | null;
}

/** The workspace row embedded in the workspace view (generation is a server string, not the scope generation). */
export interface CraftWorkspaceRefView {
  id: string;
  session_id: string;
  user_id: string;
  sandbox_id: string;
  generation: string;
  runtime_digest: string;
  revision: number;
}

export interface CraftWorkspaceView {
  session_id: string;
  workspace_id: string;
  kind: CraftSessionKind;
  title: string;
  engine_type: string;
  workspace: CraftWorkspaceRefView;
  active_run_id: string | null;
  pending_id: string | null;
  last_seq: number;
  active_run: CraftRunView | null;
  current_version: CraftVersionView | null;
}

export interface CraftInputView {
  ref: string;
  name: string;
  sha256: string;
  bytes: number;
  citation_id: string;
}

export interface CraftPreviewTicketView {
  url: string;
  expires_at: string;
  version_id: string;
}

/** Wire shape of one persisted RunEvent (outer seq is repository-assigned). */
export interface CraftRunEventView {
  seq: number;
  attempt_id: string | null;
  type: string;
  payload: unknown;
}

/** Craft event envelope inside RunEvent.payload (master plan §5). */
export interface CraftEventPayloadView {
  workspace_id: string;
  delegation_id: string | null;
  tool_call_id: string | null;
  kind: CraftEventKind;
  data: Record<string, unknown>;
}

function asRecord(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('invalid ' + label);
  }
  return value as Record<string, unknown>;
}

function nonEmptyString(value: unknown, field: string, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error('invalid ' + label + ' (' + field + ')');
  }
  return value;
}

function optionalString(value: unknown, field: string, label: string): string | null {
  if (value === undefined || value === null || value === '') return null;
  if (typeof value !== 'string') throw new Error('invalid ' + label + ' (' + field + ')');
  return value;
}

function nonNegativeInt(value: unknown, field: string, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new Error('invalid ' + label + ' (' + field + ')');
  }
  return value;
}

function cursor(value: unknown, label: string): string | null {
  if (value === undefined || value === null) return null;
  if (typeof value !== 'string' || value.trim() === '') return null;
  return value;
}

function oneOf<T extends string>(value: unknown, allowed: readonly T[], field: string, label: string): T {
  if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
    throw new Error('invalid ' + label + ' (' + field + ')');
  }
  return value as T;
}

export function parseCraftSessionCreated(value: unknown): CraftSessionCreatedView {
  const v = asRecord(value, 'craft session creation');
  return {
    session_id: nonEmptyString(v.session_id, 'session_id', 'craft session creation'),
    workspace_id: nonEmptyString(v.workspace_id, 'workspace_id', 'craft session creation'),
    engine_type: nonEmptyString(v.engine_type, 'engine_type', 'craft session creation'),
  };
}

export function parseCraftSessionPage(value: unknown): CraftSessionPageView {
  const v = asRecord(value, 'craft session page');
  if (!Array.isArray(v.data)) throw new Error('invalid craft session page (data)');
  const capabilities = v.capabilities === undefined || v.capabilities === null
    ? null
    : (() => {
        const row = asRecord(v.capabilities, 'craft capabilities');
        const kinds = Array.isArray(row.allowed_kinds) ? row.allowed_kinds : [];
        return {
          enabled: row.enabled === true,
          allowed_kinds: kinds.map((kind) => oneOf(kind, CRAFT_SESSION_KINDS, 'allowed_kinds', 'craft capabilities')),
        };
      })();
  return {
    data: v.data.map((item) => {
      const row = asRecord(item, 'craft session summary');
      return {
        session_id: nonEmptyString(row.session_id, 'session_id', 'craft session summary'),
        workspace_id: nonEmptyString(row.workspace_id, 'workspace_id', 'craft session summary'),
        kind: oneOf(row.kind, CRAFT_SESSION_KINDS, 'kind', 'craft session summary'),
        title: typeof row.title === 'string' ? row.title : '',
        engine_type: nonEmptyString(row.engine_type, 'engine_type', 'craft session summary'),
        updated_at: nonEmptyString(row.updated_at, 'updated_at', 'craft session summary'),
      };
    }),
    next_cursor: cursor(v.next_cursor, 'craft session page'),
    ...(capabilities !== null ? { capabilities } : {}),
  };
}

export function parseCraftRunView(value: unknown): CraftRunView {
  const v = asRecord(value, 'craft run view');
  return {
    run_id: nonEmptyString(v.run_id, 'run_id', 'craft run view'),
    session_id: nonEmptyString(v.session_id, 'session_id', 'craft run view'),
    status: oneOf(v.status, CRAFT_RUN_STATUSES, 'status', 'craft run view'),
    wait_reason: typeof v.wait_reason === 'string' ? v.wait_reason : '',
    revision: nonNegativeInt(v.revision, 'revision', 'craft run view'),
    epoch: nonNegativeInt(v.epoch, 'epoch', 'craft run view'),
    seq: nonNegativeInt(v.seq, 'seq', 'craft run view'),
    pending_id: optionalString(v.pending_id, 'pending_id', 'craft run view'),
  };
}

function parseFileVersion(value: unknown): CraftFileVersionView {
  const v = asRecord(value, 'craft version file');
  return {
    path: nonEmptyString(v.path, 'path', 'craft version file'),
    ref: nonEmptyString(v.ref, 'ref', 'craft version file'),
    sha256: nonEmptyString(v.sha256, 'sha256', 'craft version file'),
    mime: typeof v.mime === 'string' ? v.mime : '',
    bytes: nonNegativeInt(v.bytes, 'bytes', 'craft version file'),
  };
}

function parseVersionCheck(value: unknown): CraftVersionCheckView {
  const v = asRecord(value, 'craft version check');
  return {
    name: nonEmptyString(v.name, 'name', 'craft version check'),
    status: oneOf(v.status, CRAFT_CHECK_STATUSES, 'status', 'craft version check'),
    detail: typeof v.detail === 'string' ? v.detail : '',
  };
}

export function parseCraftVersionView(value: unknown): CraftVersionView {
  const v = asRecord(value, 'craft version');
  if (!Array.isArray(v.files)) throw new Error('invalid craft version (files)');
  if (!Array.isArray(v.checks)) throw new Error('invalid craft version (checks)');
  return {
    id: nonEmptyString(v.id, 'id', 'craft version'),
    workspace_id: nonEmptyString(v.workspace_id, 'workspace_id', 'craft version'),
    run_id: nonEmptyString(v.run_id, 'run_id', 'craft version'),
    kind: nonEmptyString(v.kind, 'kind', 'craft version'),
    files: v.files.map(parseFileVersion),
    checks: v.checks.map(parseVersionCheck),
  };
}

export function parseCraftVersionsPage(value: unknown): CraftVersionsPageView {
  const v = asRecord(value, 'craft versions page');
  if (!Array.isArray(v.data)) throw new Error('invalid craft versions page (data)');
  return {
    data: v.data.map(parseCraftVersionView),
    next_cursor: cursor(v.next_cursor, 'craft versions page'),
  };
}

function parseWorkspaceRef(value: unknown): CraftWorkspaceRefView {
  const v = asRecord(value, 'craft workspace ref');
  return {
    id: nonEmptyString(v.id, 'id', 'craft workspace ref'),
    session_id: nonEmptyString(v.session_id, 'session_id', 'craft workspace ref'),
    user_id: nonEmptyString(v.user_id, 'user_id', 'craft workspace ref'),
    sandbox_id: typeof v.sandbox_id === 'string' ? v.sandbox_id : '',
    generation: typeof v.generation === 'string' ? v.generation : '',
    runtime_digest: typeof v.runtime_digest === 'string' ? v.runtime_digest : '',
    revision: nonNegativeInt(v.revision, 'revision', 'craft workspace ref'),
  };
}

export function parseCraftWorkspaceView(value: unknown): CraftWorkspaceView {
  const v = asRecord(value, 'craft workspace view');
  const activeRun = v.active_run === undefined || v.active_run === null ? null : parseCraftRunView(v.active_run);
  const currentVersion = v.current_version === undefined || v.current_version === null ? null : parseCraftVersionView(v.current_version);
  return {
    session_id: nonEmptyString(v.session_id, 'session_id', 'craft workspace view'),
    workspace_id: nonEmptyString(v.workspace_id, 'workspace_id', 'craft workspace view'),
    kind: oneOf(v.kind, CRAFT_SESSION_KINDS, 'kind', 'craft workspace view'),
    title: typeof v.title === 'string' ? v.title : '',
    engine_type: nonEmptyString(v.engine_type, 'engine_type', 'craft workspace view'),
    workspace: parseWorkspaceRef(v.workspace),
    active_run_id: optionalString(v.active_run_id, 'active_run_id', 'craft workspace view'),
    pending_id: optionalString(v.pending_id, 'pending_id', 'craft workspace view'),
    last_seq: nonNegativeInt(v.last_seq, 'last_seq', 'craft workspace view'),
    active_run: activeRun,
    current_version: currentVersion,
  };
}

export function parseCraftInputView(value: unknown): CraftInputView {
  const v = asRecord(value, 'craft input');
  return {
    ref: nonEmptyString(v.ref, 'ref', 'craft input'),
    name: typeof v.name === 'string' ? v.name : '',
    sha256: typeof v.sha256 === 'string' ? v.sha256 : '',
    bytes: nonNegativeInt(v.bytes, 'bytes', 'craft input'),
    citation_id: typeof v.citation_id === 'string' ? v.citation_id : '',
  };
}

export function parseCraftPreviewTicket(value: unknown): CraftPreviewTicketView {
  const v = asRecord(value, 'craft preview ticket');
  return {
    url: nonEmptyString(v.url, 'url', 'craft preview ticket'),
    expires_at: nonEmptyString(v.expires_at, 'expires_at', 'craft preview ticket'),
    version_id: nonEmptyString(v.version_id, 'version_id', 'craft preview ticket'),
  };
}

export function parseCraftRunEvent(value: unknown): CraftRunEventView {
  const v = asRecord(value, 'craft run event');
  return {
    seq: nonNegativeInt(v.seq, 'seq', 'craft run event'),
    attempt_id: optionalString(v.attempt_id, 'attempt_id', 'craft run event'),
    type: nonEmptyString(v.type, 'type', 'craft run event'),
    payload: v.payload === undefined ? null : v.payload,
  };
}

/**
 * Classifies one RunEvent payload: craft kinds parse strictly (a known-kind
 * payload without workspace identity is an error, not a guess), while
 * payloads outside the craft namespaces return null so the caller advances
 * its seq cursor without inventing craft state (master plan §5).
 */
export function parseCraftEventPayload(value: unknown): CraftEventPayloadView | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  const kind = row.kind;
  if (typeof kind !== 'string') return null;
  const namespaced = CRAFT_EVENT_KIND_PREFIXES.some((prefix) => kind.startsWith(prefix));
  if (!namespaced) return null;
  if (!(CRAFT_EVENT_KINDS as readonly string[]).includes(kind)) {
    throw new Error('invalid craft event payload (kind)');
  }
  const data = row.data === undefined || row.data === null ? {} : asRecord(row.data, 'craft event payload (data)');
  return {
    workspace_id: nonEmptyString(row.workspace_id, 'workspace_id', 'craft event payload'),
    delegation_id: optionalString(row.delegation_id, 'delegation_id', 'craft event payload'),
    tool_call_id: optionalString(row.tool_call_id, 'tool_call_id', 'craft event payload'),
    kind: kind as CraftEventKind,
    data,
  };
}
