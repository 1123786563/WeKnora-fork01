import {
  parseCraftInputView,
  parseCraftPreviewTicket,
  parseCraftRunView,
  parseCraftSessionCreated,
  parseCraftSessionPage,
  parseCraftVersionView,
  parseCraftVersionsPage,
  parseCraftWorkspaceView,
  type CraftInputView,
  type CraftPreviewTicketView,
  type CraftRunView,
  type CraftSessionCreatedView,
  type CraftSessionKind,
  type CraftSessionPageView,
  type CraftVersionView,
  type CraftVersionsPageView,
  type CraftWorkspaceView,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { ApiError } from '../errors.ts';

export interface CraftCreateSessionInput {
  request_id: string;
  title: string;
  kind: CraftSessionKind;
}

export interface CraftListSessionsParams {
  cursor?: string;
  limit?: number;
}

export interface CraftAddInputInput {
  resource_ref: string;
  expected_sha256: string;
}

export interface CraftSubmitRunInput {
  request_id: string;
  prompt: string;
  input_refs?: string[];
  knowledge_scope?: string;
  base_version_id?: string;
}

export type CraftInputDecisionAction = 'continue' | 'cancel';
export interface CraftInputDecisionAcknowledgement {
  ref: string;
  action: CraftInputDecisionAction;
}

export type CraftAccessRole = 'owner' | 'collaborator' | 'viewer';
export type CraftGrantableAccessRole = Exclude<CraftAccessRole, 'owner'>;
export interface CraftAccessMember {
  user_id: string;
  role: CraftAccessRole;
}

function unwrap(value: unknown, label: string): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected a craft ' + label + ' envelope' });
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) {
    const error = typeof envelope.error === 'object' && envelope.error !== null
      ? envelope.error as Record<string, unknown>
      : {};
    throw new ApiError({
      code: typeof error.code === 'string' && error.code !== '' ? error.code : 'CRAFT_ERROR',
      message: typeof error.message === 'string' && error.message !== '' ? error.message : 'Craft request failed',
      requestId: typeof error.requestId === 'string' ? error.requestId : undefined,
    });
  }
  if (envelope.data === undefined) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft ' + label + ' envelope is missing data' });
  }
  return envelope.data;
}

function parseCraftAccessMembers(value: unknown): CraftAccessMember[] {
  const data = unwrap(value, 'access member list');
  if (!Array.isArray(data)) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft access member list must be an array' });
  }
  const ids = new Set<string>();
  return data.map((member): CraftAccessMember => {
    if (typeof member !== 'object' || member === null || Array.isArray(member)) {
      throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft access member must be an object' });
    }
    const row = member as Record<string, unknown>;
    if (typeof row.user_id !== 'string' || row.user_id.trim() === '' ||
      (row.role !== 'owner' && row.role !== 'collaborator' && row.role !== 'viewer') || ids.has(row.user_id)) {
      throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft access member has an invalid identity or role' });
    }
    ids.add(row.user_id);
    return { user_id: row.user_id, role: row.role };
  });
}

function requireCraftActionSuccess(value: unknown, label: string): void {
  if (typeof value === 'object' && value !== null && !Array.isArray(value) && (value as Record<string, unknown>).success === true) return;
  unwrap(value, label); // Throws the server's structured error; successful actions need no data member.
  throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft ' + label + ' response was invalid' });
}

/**
 * The download route is a gin wildcard (files/*file_path, see
 * internal/handler/session/craft.go): the manifest member travels as a
 * version-relative path, each segment URL-encoded, slashes preserved.
 */
export function craftDownloadPath(sessionId: string, versionId: string, filePath: string): string {
  const segments = filePath.split('/').filter((segment) => segment !== '');
  if (segments.length === 0 || segments.some((segment) => segment === '.' || segment === '..' || segment.includes('\\'))) {
    throw new ApiError({ code: 'INVALID_INPUT', message: 'Craft file path must be a non-empty version-relative path' });
  }
  const encoded = segments.map((segment) => encodeURIComponent(segment)).join('/');
  return '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/files/' + encoded;
}

export interface CraftStopStatusView {
  phase: string;
  note: string;
}

function parseStopStatus(value: unknown, label: string): CraftStopStatusView {
  const data = unwrap(value, label) as Record<string, unknown>;
  return {
    phase: typeof data.phase === 'string' ? data.phase : '',
    note: typeof data.note === 'string' ? data.note : '',
  };
}

function withQuery(path: string, params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) if (value !== undefined) query.set(key, String(value));
  const suffix = query.toString();
  return suffix ? path + '?' + suffix : path;
}

export function createCraftApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    /** POST /api/v1/craft/sessions — same request_id replays the same session. */
    async create(input: CraftCreateSessionInput, signal?: AbortSignal): Promise<CraftSessionCreatedView> {
      return parseCraftSessionCreated(unwrap(await request({
        method: 'POST',
        path: '/api/v1/craft/sessions',
        body: { request_id: input.request_id, title: input.title, kind: input.kind },
        signal,
      }), 'session creation'));
    },
    async list(params: CraftListSessionsParams = {}, signal?: AbortSignal): Promise<CraftSessionPageView> {
      // The page parsers read data/next_cursor off the envelope body directly.
      const envelope = await request({ method: 'GET', path: withQuery('/api/v1/craft/sessions', { cursor: params.cursor, limit: params.limit }), signal });
      unwrap(envelope, 'session page');
      return parseCraftSessionPage(envelope);
    },
    /** GET /api/v1/sessions/:session_id/craft — the authoritative workspace snapshot. */
    async get(sessionId: string, signal?: AbortSignal): Promise<CraftWorkspaceView> {
      return parseCraftWorkspaceView(unwrap(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft',
        signal,
      }), 'workspace view'));
    },
    /** GET /sessions/:id/craft/access — role and members from the Task ACL. */
    async accessMembers(sessionId: string, signal?: AbortSignal): Promise<CraftAccessMember[]> {
      return parseCraftAccessMembers(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/access',
        signal,
      }));
    },
    /** POST /sessions/:id/craft/access — owner-only on the server. */
    async grantAccess(sessionId: string, userId: string, role: CraftGrantableAccessRole, signal?: AbortSignal): Promise<void> {
      requireCraftActionSuccess(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/access',
        body: { user_id: userId, role },
        signal,
      }), 'access grant');
    },
    /** POST /sessions/:id/craft/access/revoke — owner-only on the server. */
    async revokeAccess(sessionId: string, userId: string, signal?: AbortSignal): Promise<void> {
      requireCraftActionSuccess(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/access/revoke',
        body: { user_id: userId },
        signal,
      }), 'access revoke');
    },
    async addInput(sessionId: string, input: CraftAddInputInput, signal?: AbortSignal): Promise<CraftInputView> {
      return parseCraftInputView(unwrap(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/inputs',
        body: { resource_ref: input.resource_ref, expected_sha256: input.expected_sha256 },
        signal,
      }), 'input'));
    },
    /** POST /craft/inputs/decision — decision is bound to the exact accepted input ref. */
    async decideInput(
      sessionId: string,
      ref: string,
      action: CraftInputDecisionAction,
      signal?: AbortSignal,
    ): Promise<CraftInputDecisionAcknowledgement> {
      const data = unwrap(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/inputs/decision',
        body: { ref, action },
        signal,
      }), 'input decision');
      if (typeof data !== 'object' || data === null || Array.isArray(data)) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft input decision acknowledgement must be an object' });
      }
      const acknowledgement = data as Record<string, unknown>;
      if (acknowledgement.ref !== ref || acknowledgement.action !== action) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Craft input decision acknowledgement did not match the requested input and action' });
      }
      return { ref, action };
    },
    /** POST /runs — retries must reuse the same request_id (the server replays admission). */
    async submit(sessionId: string, input: CraftSubmitRunInput, signal?: AbortSignal): Promise<CraftRunView> {
      return parseCraftRunView(unwrap(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs',
        body: {
          request_id: input.request_id,
          prompt: input.prompt,
          input_refs: input.input_refs ?? [],
          knowledge_scope: input.knowledge_scope ?? '',
          base_version_id: input.base_version_id ?? '',
        },
        signal,
      }), 'run admission'));
    },
    async versions(sessionId: string, signal?: AbortSignal): Promise<CraftVersionsPageView> {
      const envelope = await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions',
        signal,
      });
      unwrap(envelope, 'versions page');
      return parseCraftVersionsPage(envelope);
    },
    async version(sessionId: string, versionId: string, signal?: AbortSignal): Promise<CraftVersionView> {
      return parseCraftVersionView(unwrap(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId),
        signal,
      }), 'version'));
    },
    async preview(sessionId: string, versionId: string, signal?: AbortSignal): Promise<CraftPreviewTicketView> {
      return parseCraftPreviewTicket(unwrap(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/preview',
        body: {},
        signal,
      }), 'preview ticket'));
    },
    /**
     * Downloads one immutable version member through the platform transport.
     * The response is raw file bytes (content type from the manifest), NOT a
     * JSON envelope and never domain state — callers decide how to persist.
     */
    async download(sessionId: string, versionId: string, filePath: string, signal?: AbortSignal): Promise<unknown> {
      return request({ method: 'GET', path: craftDownloadPath(sessionId, versionId, filePath), signal });
    },
    /**
     * GET /sessions/:id/craft/versions/:version_id/export/consent — the T13
     * (#133) owner-consent surface. The resolved value is the RAW consent
     * wire payload (manifest, decision, state, restricted_derived); the
     * views layer (projectExportConsentView) holds the single fail-closed
     * projection, the same discipline as the run submit seam.
     */
    async exportConsent(sessionId: string, versionId: string, signal?: AbortSignal): Promise<unknown> {
      return unwrap(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/export/consent',
        signal,
      }), 'export consent view');
    },
    /**
     * POST .../export/consent/decision — owner-only on the server; the
     * decision binds to the EXACT manifest digest it was made against (a
     * replay against a changed manifest answers 409). The resolved value is
     * the raw consent view (same shape as exportConsent).
     */
    async decideExportConsent(
      sessionId: string,
      versionId: string,
      decision: 'approved' | 'rejected',
      manifestDigest: string,
      signal?: AbortSignal,
    ): Promise<unknown> {
      return unwrap(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/versions/' + encodeURIComponent(versionId) + '/export/consent/decision',
        body: { decision, manifest_digest: manifestDigest },
        signal,
      }), 'export consent decision');
    },
    /** POST /craft/runs/:run_id/stop — R06 verifiable stop; "stopping" is an honest phase, poll delegationStatus until terminal. */
    async stop(sessionId: string, runId: string, taskId: string, signal?: AbortSignal): Promise<CraftStopStatusView> {
      return parseStopStatus(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs/' + encodeURIComponent(runId) + '/stop',
        body: { task_id: taskId },
        signal,
      }), 'stop');
    },
    /** GET /craft/runs/:run_id/delegations/:task_id/status — read-only poll after a stop answered stopping. */
    async delegationStatus(sessionId: string, runId: string, taskId: string, signal?: AbortSignal): Promise<CraftStopStatusView> {
      return parseStopStatus(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs/' + encodeURIComponent(runId) + '/delegations/' + encodeURIComponent(taskId) + '/status',
        signal,
      }), 'delegation status');
    },
  };
}

export type CraftApi = ReturnType<typeof createCraftApi>;
