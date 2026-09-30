export type CareerJsonPrimitive = string | number | boolean | null;
export type CareerJsonValue = CareerJsonPrimitive | readonly CareerJsonValue[] | { readonly [key: string]: CareerJsonValue };
export interface CareerFactProvenance { kind: 'user' | 'resume' | 'interview' | 'source'; reference?: string }
export interface CareerFactConfirmation { confirmedBy: 'user'; at: string }
export interface CareerProfileFact { id: string; value: CareerJsonValue; confirmed: boolean; provenance: CareerFactProvenance; confirmation?: CareerFactConfirmation }
export interface CareerProfile { id: string; revision: number; facts: readonly CareerProfileFact[] }
export interface CareerJobSnapshot { revision: number; digest: string; observedAt: string; sourceUrl: string; content: string; completeness: 'complete' | 'partial' | 'unavailable'; failureReason?: string }
export interface CareerOpportunity { id: string; snapshot: CareerJobSnapshot }
export interface CareerJobSnapshotRef { opportunityId: string; revision: number; digest: string }
export type CareerApplicationStage = 'preparing' | 'ready_to_submit' | 'submitted' | 'assessment' | 'interview' | 'offer' | 'closed';
export interface CareerApplication { id: string; revision: number; opportunitySnapshot: CareerJobSnapshotRef; stage: CareerApplicationStage }
export interface CareerMaterialVersionRef { id: string; version: number; digest: string }
export interface CareerMaterialVersion extends CareerMaterialVersionRef { bodyDigest: string; pdfDigest: string; docxDigest: string; publishedAt: string }
export type CareerEligibility = 'eligible' | 'ineligible' | 'unknown';
export type CareerEvidenceResult = 'supports' | 'conflicts' | 'missing';
export interface CareerEvaluationEvidence { claim: string; result: CareerEvidenceResult; source: 'confirmed_profile' | 'job_requirement'; factId?: string; rationale?: string }
export interface CareerEvaluation { status: CareerEligibility; revision: number; profileRevision: number; opportunityRevision: number; modelVersion: string; evidence: readonly CareerEvaluationEvidence[]; explanation?: string }
export interface CareerSearchRequest { query: string; requestId: string }
export interface CareerSearchReceipt { requestId: string; revision: number; status: 'completed' | 'pending' | 'unknown'; opportunityIds: readonly string[] }
export interface CareerSubmission { id: string; channel: string; confirmedAt: string; materialVersion: CareerMaterialVersionRef | null; versionUnknown: boolean }
export type CareerTimelineKind = 'submitted' | 'assessment' | 'interview' | 'offer' | 'rejected' | 'withdrawn' | 'correction';
export interface CareerTimelineEvent { id: string; revision: number; kind: CareerTimelineKind; occurredAt: string; note?: string; correctionOf?: string }
export type CareerReminderKind = 'deadline' | 'assessment' | 'interview' | 'follow_up';
export interface CareerReminder { id: string; revision: number; kind: CareerReminderKind; dueAt: string; status: 'active' | 'paused' | 'completed' }
export interface CareerReminder { id: string; revision: number; kind: CareerReminderKind; dueAt: string; status: 'active' | 'paused' | 'completed' }
export interface CareerExportReceipt { requestId: string; status: 'pending' | 'ready' | 'failed'; revision: number; artifactId?: string; failureReason?: string }
export interface CareerDeleteReceipt { requestId: string; status: 'accepted' | 'completed'; revision: number }
