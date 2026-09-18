/** Target internal ports, NOT an assertion about current HTTP DTOs.
 * A platform adapter must bind these to the verified existing endpoints.
 * Tenant/user identity is derived by authenticated transport, never by this payload.
 */
export type ArtifactKind = 'web' | 'document' | 'spreadsheet' | 'slides';
export type CheckStatus = 'passed' | 'failed' | 'not_run';
export interface InputRef { readonly ref: string; readonly sha256: string; readonly citationId: string }
export interface KnowledgeSelection { readonly resourceId: string; readonly revision?: string }
export interface DraftIntent {
  readonly requestId: string;
  readonly sessionId: string;
  readonly text: string;
  readonly kind: ArtifactKind;
  readonly inputs: readonly InputRef[];
  readonly knowledge: readonly KnowledgeSelection[];
  readonly baseVersionId: string | null;
  readonly expectedWorkspaceRevision: number;
}
export interface RunReceipt { readonly runId: string; readonly requestId: string; readonly replayed: boolean }
export interface InteractionIdentity {
  readonly sessionId: string; readonly runId: string; readonly interactionId: string;
  readonly requestRevision: number; readonly payloadHash: string;
}
export type Delivery = 'recorded' | 'delivery_pending' | 'delivered' | 'rejected' | 'expired' | 'unknown';
export interface DecisionReceipt { readonly decisionId: string; readonly delivery: Delivery }
export interface RestoreIntent {
  readonly requestId: string; readonly sessionId: string; readonly versionId: string;
  readonly expectedWorkspaceRevision: number;
}
export interface RestoredWorkspace {
  readonly workspaceId: string; readonly generation: string;
  readonly revision: number; readonly baseVersionId: string;
}
export interface PreviewGrant { readonly url: string; readonly expiresAt: string; readonly versionId: string }
export interface CraftCommandPorts {
  submitDraft(intent: DraftIntent, signal: AbortSignal): Promise<RunReceipt>;
  cancelRun(input: {sessionId: string; runId: string; requestId: string}, signal: AbortSignal): Promise<{accepted: boolean}>;
  answerQuestion(input: InteractionIdentity & {requestId: string; answers: readonly string[]}, signal: AbortSignal): Promise<DecisionReceipt>;
  decidePermission(input: InteractionIdentity & {requestId: string; decision: 'allow_once' | 'deny'}, signal: AbortSignal): Promise<DecisionReceipt>;
  restoreVersion(input: RestoreIntent, signal: AbortSignal): Promise<RestoredWorkspace>;
  refreshPreview(input: {sessionId: string; versionId: string}, signal: AbortSignal): Promise<PreviewGrant>;
}
export interface ViewCapabilities {
  readonly canRead: boolean; readonly canWrite: boolean;
  readonly allowedKinds: readonly ArtifactKind[];
  readonly maxInputBytes: number | null;
  readonly disabledReason?: string;
}
