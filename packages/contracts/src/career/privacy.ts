export interface CareerPrivacyExport { requestId: string; status: 'pending' | 'ready' | 'failed'; downloadUrl?: string }
export interface CareerDeleteReceipt { requestId: string; status: 'accepted' | 'completed'; revision: number }
