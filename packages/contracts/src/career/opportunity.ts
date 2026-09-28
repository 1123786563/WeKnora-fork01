export interface CareerJobSnapshot { revision: number; digest: string; observedAt: string; sourceUrl: string; content: string }
export interface CareerOpportunity { id: string; snapshot: CareerJobSnapshot }
