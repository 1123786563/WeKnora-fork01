import { ContractError, parseCareerApplication, parseCareerDeleteReceipt, parseCareerEvaluation, parseCareerExportReceipt, parseCareerMaterialVersion, parseCareerOpportunity, parseCareerProfile, parseCareerReminder, parseCareerSearchReceipt, parseCareerSubmission, parseCareerTimelineEvent } from '@weknora/contracts';
import type { CareerCommand, CareerReceipt, CareerScope, CareerWorkspace } from '@weknora/contracts';
import type { CareerApi, CareerObserver, CareerRequest, CareerRequester } from './types.ts';
import {
  decodeCareerApplication, decodeCareerDeleteReceipt, decodeCareerEnvelope, decodeCareerEnvelopeValue, decodeCareerEvaluation,
  decodeCareerExportReceipt, decodeCareerMaterialVersion, decodeCareerOpportunityList, decodeCareerProfile, decodeCareerReceipt,
  decodeCareerReminders, decodeCareerSearchReceipt, decodeCareerSubmission, decodeCareerTimeline, decodeCareerWorkspace,
} from './decode.ts';
export type { CareerApi, CareerObserver, CareerRequest, CareerRequester, CareerScope, CareerWorkspace, CareerCommand } from './types.ts';
export * from './decode.ts';

function validateScope(scope: CareerScope): CareerScope {
  if (!scope || typeof scope !== 'object' || Array.isArray(scope)) throw new ContractError('scope', 'expected structured CareerScope');
  const keys = Object.keys(scope);
  if (keys.length !== 3 || keys.some(key => !['deploymentOrigin', 'tenantId', 'actorId'].includes(key))) throw new ContractError('scope', 'expected deploymentOrigin, tenantId, actorId');
  for (const key of ['tenantId', 'actorId'] as const) if (typeof scope[key] !== 'string' || !scope[key].trim()) throw new ContractError(`scope.${key}`, 'expected non-empty string');
  if (typeof scope.deploymentOrigin !== 'string' || !scope.deploymentOrigin.trim()) throw new ContractError('scope.deploymentOrigin', 'expected non-empty origin');
  let origin: URL;
  try { origin = new URL(scope.deploymentOrigin); } catch { throw new ContractError('scope.deploymentOrigin', 'expected absolute origin'); }
  if (!['https:', 'http:'].includes(origin.protocol) || origin.origin !== scope.deploymentOrigin.replace(/\/$/, '')) throw new ContractError('scope.deploymentOrigin', 'expected canonical HTTP(S) origin');
  return { deploymentOrigin: origin.origin, tenantId: scope.tenantId, actorId: scope.actorId };
}
function validateWrite(input: { requestId: string; expectedRevision?: number }, body: unknown, requireRevision = true): void {
  if (typeof input.requestId !== 'string' || input.requestId.trim() === '') throw new ContractError('requestId', 'expected non-empty request ID');
  if (requireRevision && (typeof input.expectedRevision !== 'number' || !Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 1)) throw new ContractError('expectedRevision', 'expected positive safe revision');
  validateJson(input, 'input');
  validateJson(body, 'body');
}
function validateJson(value: unknown, path: string): void {
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return;
  if (typeof value === 'number' && Number.isFinite(value)) return;
  if (Array.isArray(value)) { value.forEach((entry, i) => validateJson(entry, `${path}[${i}]`)); return; }
  if (!value || typeof value !== 'object' || Object.getPrototypeOf(value) !== Object.prototype) throw new ContractError(path, 'expected JSON data');
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    if (/^(tenant|owner|actor)(_|[A-Z]|$)/i.test(key)) throw new ContractError(`${path}.${key}`, 'authority fields are not client data');
    validateJson(entry, `${path}.${key}`);
  }
}
function requiredId(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected non-empty ID');
  return value;
}
function array<T>(value: unknown, parse: (v: unknown) => T, path: string): T[] {
  if (!Array.isArray(value)) throw new ContractError(path, 'expected array');
  return value.map(parse);
}
function parseDeskPage(value: unknown) {
  const row = value as Record<string, unknown>;
  if (!row || typeof row !== 'object' || Array.isArray(row) || Object.keys(row).some(key => !['revision', 'value', 'cursor'].includes(key))) throw new ContractError('deskPage', 'invalid page envelope');
  if (!Array.isArray(row.value)) throw new ContractError('deskPage.value', 'expected array');
  const envelope = decodeCareerEnvelopeValue({ revision: row.revision, value: row.value.map(decodeCareerWorkspace) }, value => value as CareerWorkspace[]);
  if (row.cursor !== undefined && typeof row.cursor !== 'string') throw new ContractError('deskPage.cursor', 'expected string');
  return { ...envelope, ...(row.cursor === undefined ? {} : { cursor: row.cursor }) };
}

export function createCareerApi(request: CareerRequester, observeScope: CareerObserver = () => () => undefined): CareerApi {
  async function call<T>(scopeInput: CareerScope, method: string, path: string, parse: (value: unknown) => T, options: { input?: { requestId: string; expectedRevision?: number }; body?: unknown; signal?: AbortSignal; receipt?: boolean; requestId?: string } = {}): Promise<T> {
    const scope = validateScope(scopeInput);
    if (options.input) validateWrite(options.input, options.body, options.input.expectedRevision !== undefined);
    const req: CareerRequest = {
      method, path, scope, ...(options.body === undefined ? {} : { body: options.body }),
      ...(options.signal === undefined ? {} : { signal: options.signal }),
      ...(options.requestId === undefined ? {} : { requestId: options.requestId }),
      ...(options.input ? { requestId: options.input.requestId, headers: { 'X-Request-ID': options.input.requestId, ...(options.input.expectedRevision === undefined ? {} : { 'If-Match': String(options.input.expectedRevision) }) } } : {}),
    };
    const response = await request(req);
    return decodeCareerEnvelope(response, parse, options.requestId ?? options.input?.requestId, options.receipt ?? false).data;
  }
  const desk = {
    open: (scope: CareerScope, signal?: AbortSignal) => call(scope, 'GET', '/api/v1/career/desk', value => {
      const row = value as Record<string, unknown>; return decodeCareerEnvelopeValue(row, decodeCareerWorkspace);
    }, { signal }),
    list: (scope: CareerScope, cursor?: string, signal?: AbortSignal) => call(scope, 'GET', `/api/v1/career/desk${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`, parseDeskPage, { signal }),
    act: (scope: CareerScope, intent: import('@weknora/contracts').CareerIntent<CareerCommand>, signal?: AbortSignal) => call(scope, 'POST', '/api/v1/career/desk/actions', value => decodeCareerReceipt(value, intent.requestId), { input: { requestId: intent.requestId, expectedRevision: intent.expectedRevision }, body: intent, signal, receipt: true }),
    lookup: (scope: CareerScope, requestId: string, signal?: AbortSignal) => call(scope, 'GET', `/api/v1/career/requests/${encodeURIComponent(requiredId(requestId, 'requestId'))}`, value => decodeCareerReceipt(value, requestId), { requestId, receipt: true, signal }),
    observe: (scope: CareerScope, onRevision: (revision: number) => void) => observeScope(validateScope(scope), onRevision),
  };
  return {
    ...desk,
    getProfile: scope => call(scope, 'GET', '/api/v1/career/profile', parseCareerProfile),
    updateProfile: async (scope, input) => {
      validateWrite(input, { facts: input.facts });
      const profile = { id: 'validation', revision: input.expectedRevision, facts: input.facts };
      parseCareerProfile(profile);
      return call(scope, 'PUT', '/api/v1/career/profile', parseCareerProfile, { input, body: { facts: input.facts }, receipt: true });
    },
    search: (scope, input) => {
      validateWrite(input, { query: input.query }, false);
      return call(scope, 'POST', '/api/v1/career/search', value => decodeCareerSearchReceipt(value, input.requestId), { input, body: { query: requiredId(input.query, 'query') }, receipt: true });
    },
    listOpportunities: scope => call(scope, 'GET', '/api/v1/career/opportunities', value => array(value, parseCareerOpportunity, 'opportunities')),
    getEvaluation: (scope, id) => call(scope, 'GET', `/api/v1/career/opportunities/${encodeURIComponent(requiredId(id, 'opportunityId'))}/evaluation`, parseCareerEvaluation),
    getApplication: (scope, id) => call(scope, 'GET', `/api/v1/career/applications/${encodeURIComponent(requiredId(id, 'applicationId'))}`, parseCareerApplication),
    createApplication: (scope, input) => call(scope, 'POST', '/api/v1/career/applications', parseCareerApplication, { input, body: { opportunityId: requiredId(input.opportunityId, 'opportunityId') }, receipt: true }),
    listApplications: scope => call(scope, 'GET', '/api/v1/career/applications', value => array(value, parseCareerApplication, 'applications')),
    listMaterials: (scope, appId) => call(scope, 'GET', `/api/v1/career/applications/${encodeURIComponent(requiredId(appId, 'applicationId'))}/materials`, value => array(value, parseCareerMaterialVersion, 'materials')),
    createMaterial: (scope, appId, input) => call(scope, 'POST', `/api/v1/career/applications/${encodeURIComponent(requiredId(appId, 'applicationId'))}/materials`, parseCareerMaterialVersion, { input, body: { body: input.body, expectedRevision: input.expectedRevision }, receipt: true }),
    recordSubmission: (scope, appId, input) => {
      validateWrite(input, { channel: input.channel, materialVersion: input.materialVersion ?? null, versionUnknown: input.versionUnknown });
      if (typeof input.versionUnknown !== 'boolean' || (input.versionUnknown && input.materialVersion != null) || (!input.versionUnknown && input.materialVersion == null)) throw new ContractError('submission.materialVersion', 'versionUnknown must match the actual version reference');
      const data = parseCareerSubmission({ id: 'validation', channel: input.channel, confirmedAt: '2026-01-01T00:00:00Z', materialVersion: input.materialVersion ?? null, versionUnknown: input.versionUnknown });
      return call(scope, 'POST', `/api/v1/career/applications/${encodeURIComponent(requiredId(appId, 'applicationId'))}/submission`, parseCareerSubmission, { input, body: { channel: data.channel, versionUnknown: data.versionUnknown, materialVersion: data.materialVersion }, receipt: true });
    },
    listTimeline: (scope, appId) => call(scope, 'GET', `/api/v1/career/applications/${encodeURIComponent(requiredId(appId, 'applicationId'))}/timeline`, value => array(value, parseCareerTimelineEvent, 'timeline')),
    appendTimeline: (scope, appId, input) => call(scope, 'POST', `/api/v1/career/applications/${encodeURIComponent(requiredId(appId, 'applicationId'))}/timeline`, parseCareerTimelineEvent, { input, body: { kind: input.kind, ...(input.occurredAt === undefined ? {} : { occurredAt: input.occurredAt }) }, receipt: true }),
    listReminders: scope => call(scope, 'GET', '/api/v1/career/reminders', value => array(value, parseCareerReminder, 'reminders')),
    createReminder: (scope, input) => call(scope, 'POST', '/api/v1/career/reminders', parseCareerReminder, { input, body: { kind: input.kind, dueAt: input.dueAt }, receipt: true }),
    requestExport: (scope, input) => call(scope, 'POST', '/api/v1/career/privacy/export', value => decodeCareerExportReceipt(value, input.requestId), { input, body: {}, receipt: true }),
    requestDelete: (scope, input) => call(scope, 'POST', '/api/v1/career/privacy/delete', value => decodeCareerDeleteReceipt(value, input.requestId), { input, body: {}, receipt: true }),
    lookupRequest: (scope, id) => desk.lookup(scope, id),
  };
}
