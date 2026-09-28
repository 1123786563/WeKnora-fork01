export type CareerEligibility = 'eligible' | 'ineligible' | 'unknown';
export interface CareerEvaluation { status: CareerEligibility; revision: number; explanation?: string }
export interface CareerSearchRequest { query: string; requestId: string }
