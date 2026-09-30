export interface CareerScope { deploymentOrigin: string; tenantId: string; actorId: string }
export interface CareerEnvelope<T> { revision: number; value: T }
export interface CareerPage<T> extends CareerEnvelope<readonly T[]> { cursor?: string }
export type CareerCommand =
  | { type: 'updateProfile'; expectedRevision: number; payload: { facts: readonly CareerProfileFact[] } }
  | { type: 'createApplication'; expectedRevision: number; payload: { opportunityId: string } }
  | { type: 'appendTimeline'; expectedRevision: number; payload: { applicationId: string; kind: CareerTimelineKind; occurredAt: string } }
  | { type: 'createReminder'; expectedRevision: number; payload: { kind: CareerReminderKind; dueAt: string } }
  | { type: 'createMaterial'; expectedRevision: number; payload: { applicationId: string; body: Record<string, CareerJsonValue> } }
  | { type: 'confirmSubmission'; expectedRevision: number; payload: { applicationId: string; channel: string; materialVersion: CareerMaterialVersionRef | null } };
export interface CareerIntent<C = CareerCommand> { requestId: string; expectedRevision: number; command: C }
export type CareerReceipt<T> =
  | { kind: 'applied'; requestId: string; envelope: CareerEnvelope<T> }
  | { kind: 'conflict'; requestId: string; envelope: CareerEnvelope<T> }
  | { kind: 'unknown'; requestId: string }
  | { kind: 'forbidden'; requestId: string };
export interface CareerWorkspace { profile?: CareerProfile; opportunities: readonly CareerOpportunity[]; applications: readonly CareerApplication[] }
export interface CareerRemote<T, C = CareerCommand> {
  open(scope: CareerScope, signal?: AbortSignal): Promise<CareerEnvelope<T>>;
  list(scope: CareerScope, cursor?: string, signal?: AbortSignal): Promise<CareerPage<T>>;
  act(scope: CareerScope, intent: CareerIntent<C>, signal?: AbortSignal): Promise<CareerReceipt<T>>;
  lookup(scope: CareerScope, requestId: string, signal?: AbortSignal): Promise<CareerReceipt<T>>;
  observe(scope: CareerScope, onRevision: (revision: number) => void): () => void;
}
export interface CareerIntentStore<C = CareerCommand> {
  save(scope: CareerScope, intent: CareerIntent<C>): Promise<void>;
  list(scope: CareerScope): Promise<readonly CareerIntent<C>[]>;
  remove(scope: CareerScope, requestId: string): Promise<void>;
}
import type { CareerProfile, CareerProfileFact, CareerOpportunity, CareerApplication, CareerTimelineKind, CareerReminderKind, CareerJsonValue, CareerMaterialVersionRef } from './types.ts';
