import type { CareerApplication, CareerEvaluation, CareerOpportunity, CareerProfile } from '@weknora/contracts';
export interface CareerRequest { method: string; path: string; headers?: Record<string, string>; body?: unknown }
export type CareerRequester = (input: CareerRequest) => Promise<unknown>;
export interface CareerApi {
  getProfile(requestId?: string): Promise<CareerProfile>;
  listOpportunities(): Promise<CareerOpportunity[]>;
  getEvaluation(opportunityId: string): Promise<CareerEvaluation>;
  getApplication(applicationId: string): Promise<CareerApplication>;
  updateProfile(input: { requestId: string; expectedRevision: number; body: unknown }): Promise<CareerProfile>;
}
