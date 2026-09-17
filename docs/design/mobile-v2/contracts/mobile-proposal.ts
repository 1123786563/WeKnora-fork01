/** Design proposal only. Freeze against current Go/TS DTOs in MX-003 before production use. */
export type ScopeKey = Readonly<{origin: string; userId: string; tenantId: string}>;
export type Capability = Readonly<{state: 'supported' | 'unavailable' | 'forbidden'; reason: string}>;
export interface StartExecutionInput { // Inherited A-class SDK shape, not a live availability claim.
  request_id: string; session_id: string; agent_id: string; target_id: string;
  workspace_ref: string; text: string; budget_upper: number;
}
export interface ExecutionEvent {
  schema_version: 1; run_id: string; attempt_id: string; seq: number;
  type: string; occurred_at: string; payload: Record<string, unknown>;
}
export type StreamControl =
  | {kind: 'heartbeat'}
  | {kind: 'cursor_expired'; snapshot_required: true}
  | {kind: 'auth_expired'}
  | {kind: 'stream_error'; retryable: boolean; code: string};
export interface MobileRunView {
  runId: string; sessionId: string; runStatus: string; executionStatus: string;
  settlementStatus: string; revision: number; watermark: number;
  asOf: string; stale: boolean; capabilities: Record<string, Capability>;
}
export interface PageResult<T> {items: T[]; next_cursor: string | null; as_of: string;}
export interface OverviewProposal {
  schema_version: 1; as_of: string;
  counts: {running: number; waiting_user: number; completed: number};
  in_progress: MobileRunView[];
  pending_interactions: Array<{id: string; run_id: string; kind: string; title: string}>;
  recent_artifacts: Array<{id: string; version: string; name: string; mime: string; size: number}>;
}
export interface BootstrapProposal {
  schema_version: 1; actor: {id: string; display_name: string};
  memberships: Array<{tenant_id: string; role: string; billing_role: string | null}>;
  selected_tenant_id: string | null; capabilities: Record<string, Capability>;
  limits: {prompt_max_chars: number; upload_max_bytes: number; budget_min: number; budget_max: number};
  protocol: {minimum: number; current: number}; as_of: string;
}
interface DecisionBase {request_id: string; expected_revision: number; content_digest: string;}
/** Proposed closed union, NOT a claim about the current decision handler body. */
export type InteractionDecisionProposal = DecisionBase & (
  | {kind: 'tool'; decision: 'approve' | 'reject'}
  | {kind: 'budget'; decision: 'approve'; authorized_upper: number}
  | {kind: 'budget'; decision: 'reject'}
  | {kind: 'question'; decision: 'answer'; answer: string}
  | {kind: 'connection'; decision: 'authorize'}
  | {kind: 'connection'; decision: 'reject'}
);
export type LocalSubmissionState = 'prepared' | 'sending' | 'uncertain' | 'accepted' | 'rejected';
export interface LocalSubmission {scope: ScopeKey; requestId: string; inputHash: string; state: LocalSubmissionState; runId?: string; createdAt: string;}
export type ExecutionCommand =
  | {action: 'cancel'; expected_revision: number}
  | {action: 'steer'; text: string; expected_revision: number};
export interface NativeHostProposal {
  request<T>(path: string, init: {method: 'GET' | 'POST' | 'PUT' | 'DELETE'; body?: unknown; signal: AbortSignal}): Promise<T>;
  stream(input: {url: string; headers: Record<string,string>; signal: AbortSignal}, onBytes: (bytes: Uint8Array) => Promise<void>): Promise<void>;
  credentials: {read(): Promise<string | null>; write(value: string): Promise<void>; clear(): Promise<void>};
  files: {pick(): Promise<{uri: string; name: string; mime: string; size: number} | null>};
}
export interface ScenarioInput {caseId: string; scope?: 'same' | 'switch'; fault?: string; fixture?: string;}
/** Planned probe output must contain values observed at the real boundary; never hard-code pass flags. */
export interface ScenarioObservation {requestCount?: number; decisionCount?: number; runId?: string; visibleTenant?: string; cursor?: number; status?: string; fields?: string[]; failures?: string[];}
