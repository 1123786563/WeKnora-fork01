export type CareerApplicationStage = 'preparing' | 'ready_to_submit' | 'submitted' | 'assessment' | 'interview' | 'offer' | 'closed';
export interface CareerApplication { id: string; revision: number; opportunitySnapshot: CareerJobSnapshotRef; stage: CareerApplicationStage }
export interface CareerJobSnapshotRef { opportunityId: string; revision: number; digest: string }
export interface CareerMaterialVersion { id: string; version: number; digest: string; bodyDigest: string; pdfDigest: string; docxDigest: string; publishedAt: string }
