// W04 app-connector wire contracts.
//
// Every view is an explicit projection: credentials NEVER cross this boundary
// — not the OAuth access/refresh tokens and not even the credential_ref. The
// server responds with the structs below; the parsers below re-validate every
// field so no unvalidated server data reaches the web layer.

/** Connection lifecycle states (internal/appconnector/model.go). */
export type ConnectionState = 'active' | 'revoked' | 'pending_reauthorization';
/** Installation lifecycle states (internal/appconnector/model.go). */
export type InstallationState = 'active' | 'disabled' | 'reauthorization_required';
/** Connection kinds: personal connections are owner-only, space ones shared. */
export type ConnectionKind = 'personal' | 'space';

export interface ConnectionView {
  id: string;
  kind: ConnectionKind;
  state: ConnectionState | string;
  owner_id: string | null;
}

export interface InstallationView {
  id: string;
  app_key: string;
  version: string;
  scopes: string[];
}

/** Sync pause-reason vocabulary (internal/appconnector/sync.go). */
export type SyncPauseReason =
  | 'plan'
  | 'permission'
  | 'conflict'
  | 'rate_limit';

export interface SyncBindingView {
  installation_id: string;
  connection_id: string;
  auth_version: string;
}

export interface SyncStatusView {
  datasource_id: string;
  state: string;
  pause_reason: SyncPauseReason | string | null;
  binding: SyncBindingView | null;
  requires_reauthorization: boolean;
}

export interface CreateInstallationInput {
  app_key: string;
  version: string;
  expected_version: number;
}

export interface UpgradeInstallationInput {
  version: string;
  expected_version: number;
}

export interface CreateConnectionInput {
  installation_id: string;
  kind: ConnectionKind;
  expected_version: number;
}

const connectionKinds: readonly string[] = ['personal', 'space'];

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

function stringArray(value: unknown, field: string, label: string): string[] {
  if (!Array.isArray(value)) {
    throw new Error('invalid ' + label + ' (' + field + ')');
  }
  return value.map((item, i) => {
    if (typeof item !== 'string' || item.trim() === '') {
      throw new Error('invalid ' + label + ' (' + field + '[' + i + '])');
    }
    return item;
  });
}

export function parseConnectionView(value: unknown): ConnectionView {
  const v = asRecord(value, 'connection');
  const id = nonEmptyString(v.id, 'id', 'connection');
  if (!connectionKinds.includes(String(v.kind))) {
    throw new Error('invalid connection (kind)');
  }
  const state = nonEmptyString(v.state, 'state', 'connection');
  if (v.owner_id !== null && typeof v.owner_id !== 'string') {
    throw new Error('invalid connection (owner_id)');
  }
  // Explicit field-by-field construction: any credential-ish field the server
  // might attach is dropped here, never forwarded.
  return { id, kind: v.kind as ConnectionKind, state, owner_id: v.owner_id === null ? null : v.owner_id };
}

export function parseInstallationView(value: unknown): InstallationView {
  const v = asRecord(value, 'installation');
  const id = nonEmptyString(v.id, 'id', 'installation');
  const app_key = nonEmptyString(v.app_key, 'app_key', 'installation');
  const version = nonEmptyString(v.version, 'version', 'installation');
  const scopes = stringArray(v.scopes, 'scopes', 'installation');
  return { id, app_key, version, scopes };
}

function parseSyncBindingView(value: unknown): SyncBindingView | null {
  if (value === null || value === undefined) return null;
  const v = asRecord(value, 'sync binding');
  const installation_id = nonEmptyString(v.installation_id, 'installation_id', 'sync binding');
  const connection_id = nonEmptyString(v.connection_id, 'connection_id', 'sync binding');
  const auth_version = nonEmptyString(v.auth_version, 'auth_version', 'sync binding');
  return { installation_id, connection_id, auth_version };
}

export function parseSyncStatusView(value: unknown): SyncStatusView {
  const v = asRecord(value, 'sync status');
  const datasource_id = nonEmptyString(v.datasource_id, 'datasource_id', 'sync status');
  const state = nonEmptyString(v.state, 'state', 'sync status');
  if (v.pause_reason !== null && v.pause_reason !== undefined && typeof v.pause_reason !== 'string') {
    throw new Error('invalid sync status (pause_reason)');
  }
  const binding = parseSyncBindingView(v.binding);
  if (typeof v.requires_reauthorization !== 'boolean') {
    throw new Error('invalid sync status (requires_reauthorization)');
  }
  return {
    datasource_id,
    state,
    pause_reason: v.pause_reason === undefined ? null : v.pause_reason === null ? null : v.pause_reason as string,
    binding,
    requires_reauthorization: v.requires_reauthorization,
  };
}

// ---------------------------------------------------------------------------
// W05: external action approval (A03) contracts.
// ---------------------------------------------------------------------------

/** Action lifecycle states (internal/appconnector/action.go:30-36). */
export type ActionState =
  | 'awaiting_approval'
  | 'authorized'
  | 'queued'
  | 'dispatched'
  | 'succeeded'
  | 'failed'
  | 'unknown';

export const ACTION_STATES: readonly ActionState[] = [
  'awaiting_approval',
  'authorized',
  'queued',
  'dispatched',
  'succeeded',
  'failed',
  'unknown',
];

export function isActionState(value: unknown): value is ActionState {
  return typeof value === 'string' && (ACTION_STATES as readonly string[]).includes(value);
}

/**
 * Owner-free projection of one action: state (A03 vocabulary only), the
 * digest an approval is bound to, the exact target and content snapshot,
 * and a display connection name. No actor/owner identity crosses the wire.
 */
export interface ActionView {
  id: string;
  state: ActionState;
  digest: string;
  target: string;
  content: string;
  connection_name: string;
}

/** An ActionView plus the fence value an approval must echo back (CAS). */
export interface ActionDetail {
  action: ActionView;
  expected_version: number;
}

/** Action risk categories (internal/appconnector/action.go). */
export type ActionRisk = 'read' | 'write' | 'send' | 'delete';

export interface PrepareActionInput {
  connection_id: string;
  target: string;
  risk: ActionRisk;
  content: string;
  app_version?: string;
}

export interface ApproveActionInput {
  digest: string;
  expected_version: number;
}

export interface ExtendTaskBudgetInput {
  additional_credits: number;
  idempotency_key: string;
}

export interface TaskBudgetExtensionResult {
  task_id: string;
  additional_credits: number;
}

export function parseActionView(value: unknown): ActionView {
  const v = asRecord(value, 'action');
  const id = nonEmptyString(v.id, 'id', 'action');
  const digest = nonEmptyString(v.digest, 'digest', 'action');
  const target = nonEmptyString(v.target, 'target', 'action');
  const content = nonEmptyString(v.content, 'content', 'action');
  const connection_name = nonEmptyString(v.connection_name, 'connection_name', 'action');
  if (!isActionState(v.state)) {
    throw new Error('invalid action (state)');
  }
  // Explicit field-by-field reconstruction: owner/actor fields a server
  // might attach are dropped here — the projection stays owner-free.
  return { id, state: v.state, digest, target, content, connection_name };
}

export function parseActionDetail(value: unknown): ActionDetail {
  const v = asRecord(value, 'action detail');
  const action = parseActionView(v.action);
  if (typeof v.expected_version !== 'number' || !Number.isSafeInteger(v.expected_version) || v.expected_version < 0) {
    throw new Error('invalid action detail (expected_version)');
  }
  return { action, expected_version: v.expected_version };
}

export function parseTaskBudgetExtensionResult(value: unknown): TaskBudgetExtensionResult {
  const v = asRecord(value, 'task budget extension');
  const task_id = nonEmptyString(v.task_id, 'task_id', 'task budget extension');
  if (typeof v.additional_credits !== 'number' || !Number.isSafeInteger(v.additional_credits) || v.additional_credits <= 0) {
    throw new Error('invalid task budget extension (additional_credits)');
  }
  return { task_id, additional_credits: v.additional_credits };
}