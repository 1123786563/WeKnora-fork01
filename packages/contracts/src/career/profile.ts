export interface CareerProfileFact { id: string; value: unknown; confirmed: boolean; source: string }
export interface CareerProfile { id: string; revision: number; facts: CareerProfileFact[] }
