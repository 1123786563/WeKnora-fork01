import type {
  CareerApplication, CareerCommand, CareerDeleteReceipt, CareerEnvelope, CareerEvaluation, CareerExportReceipt,
  CareerIntent, CareerMaterialVersion, CareerOpportunity, CareerPage, CareerProfile, CareerProfileFact,
  CareerReminder, CareerRemote, CareerScope, CareerSearchReceipt, CareerSubmission, CareerTimelineEvent, CareerWorkspace,
} from '@weknora/contracts';

export interface CareerRequest { method: string; path: string; scope: CareerScope; requestId?: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal }
export type CareerRequester = (input: CareerRequest) => Promise<unknown>;
export type CareerObserver = (scope: CareerScope, onRevision: (revision: number) => void) => () => void;
export interface CareerWrite { requestId: string; expectedRevision: number }
export interface CareerApi extends CareerRemote<CareerWorkspace, CareerCommand> {
  getProfile(scope: CareerScope): Promise<CareerProfile>;
  updateProfile(scope: CareerScope, input: CareerWrite & { facts: readonly CareerProfileFact[] }): Promise<CareerProfile>;
  search(scope: CareerScope, input: CareerWrite & { query: string }): Promise<CareerSearchReceipt>;
  listOpportunities(scope: CareerScope): Promise<CareerOpportunity[]>;
  getEvaluation(scope: CareerScope, opportunityId: string): Promise<CareerEvaluation>;
  getApplication(scope: CareerScope, applicationId: string): Promise<CareerApplication>;
  createApplication(scope: CareerScope, input: CareerWrite & { opportunityId: string }): Promise<CareerApplication>;
  listApplications(scope: CareerScope): Promise<CareerApplication[]>;
  listMaterials(scope: CareerScope, applicationId: string): Promise<CareerMaterialVersion[]>;
  createMaterial(scope: CareerScope, applicationId: string, input: CareerWrite & { body: Record<string, unknown> }): Promise<CareerMaterialVersion>;
  recordSubmission(scope: CareerScope, applicationId: string, input: CareerWrite & { channel: string; versionUnknown: boolean; materialVersion?: { id: string; version: number; digest: string } | null }): Promise<CareerSubmission>;
  listTimeline(scope: CareerScope, applicationId: string): Promise<CareerTimelineEvent[]>;
  appendTimeline(scope: CareerScope, applicationId: string, input: CareerWrite & { kind: CareerTimelineEvent['kind']; occurredAt?: string }): Promise<CareerTimelineEvent>;
  listReminders(scope: CareerScope): Promise<CareerReminder[]>;
  createReminder(scope: CareerScope, input: CareerWrite & { kind: CareerReminder['kind']; dueAt: string }): Promise<CareerReminder>;
  requestExport(scope: CareerScope, input: { requestId: string }): Promise<CareerExportReceipt>;
  requestDelete(scope: CareerScope, input: CareerWrite): Promise<CareerDeleteReceipt>;
  lookupRequest(scope: CareerScope, requestId: string): Promise<import('@weknora/contracts').CareerReceipt<CareerWorkspace>>;
}
export type { CareerScope, CareerWorkspace, CareerCommand, CareerIntent, CareerPage, CareerEnvelope };
