import type { ClientRequest } from '../client.ts';

export interface FAQEntry {
  id: number;
  standard_question: string;
  similar_questions: string[];
  negative_questions: string[];
  answers: string[];
  is_enabled: boolean;
  is_recommended: boolean;
  [key: string]: unknown;
}
export interface FAQEntryListResponse { data: FAQEntry[]; total: number; page: number; page_size: number; }
export interface FAQEntryFieldsUpdate { is_enabled?: boolean; is_recommended?: boolean; tag_id?: number | null; }
export interface FAQEntryFieldsBatchRequest { by_id?: Record<number, FAQEntryFieldsUpdate>; by_tag?: Record<number, FAQEntryFieldsUpdate>; exclude_ids?: number[]; }
export interface FAQEntryPayload { standard_question: string; similar_questions?: string[]; negative_questions?: string[]; answers: string[]; tag_id?: number | null; is_enabled?: boolean; is_recommended?: boolean; }
export interface FAQSearchInput { query_text: string; vector_threshold?: number; match_count?: number; }

function object(value: unknown, message: string): Record<string, unknown> { if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(message); return value as Record<string, unknown>; }
function envelope(value: unknown): Record<string, unknown> { const row = object(value, 'Invalid FAQ response'); if (row.success !== true) throw new Error('Invalid FAQ response success'); return object(row.data, 'Invalid FAQ response data'); }
function integer(value: unknown, field: string): number { if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new Error(`Invalid FAQ field: ${field}`); return value; }
function stringArray(value: unknown, field: string): string[] { if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new Error(`Invalid FAQ field: ${field}`); return value; }
function optionalStringArray(value: unknown, field: string): string[] { return value === null ? [] : stringArray(value, field); }
function parseEntry(value: unknown): FAQEntry { const row = object(value, 'Invalid FAQ entry'); if (typeof row.id !== 'number' || !Number.isSafeInteger(row.id) || row.id < 0) throw new Error('Invalid FAQ entry id'); if (typeof row.standard_question !== 'string' || typeof row.is_enabled !== 'boolean' || typeof row.is_recommended !== 'boolean') throw new Error('Invalid FAQ entry fields'); return { ...row, id: row.id, standard_question: row.standard_question, similar_questions: optionalStringArray(row.similar_questions, 'similar_questions'), negative_questions: optionalStringArray(row.negative_questions, 'negative_questions'), answers: stringArray(row.answers, 'answers'), is_enabled: row.is_enabled, is_recommended: row.is_recommended } as FAQEntry; }

export function createKnowledgeFaqApi(request: (input: ClientRequest) => Promise<unknown>) {
  const base = (kbId: string) => `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/faq`;
  return {
    async list(kbId: string, params: { page?: number; page_size?: number; tag_id?: number; tag_ids?: string; keyword?: string; search_field?: string; sort_order?: string; is_enabled?: boolean } = {}): Promise<FAQEntryListResponse> {
      const query = new URLSearchParams(); for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== '') query.set(key, String(value));
      const row = envelope(await request({ method: 'GET', path: `${base(kbId)}/entries${query.toString() ? `?${query}` : ''}` }));
      if (!Array.isArray(row.data)) throw new Error('Invalid FAQ list data');
      return { data: row.data.map(parseEntry), total: integer(row.total, 'total'), page: integer(row.page, 'page'), page_size: integer(row.page_size, 'page_size') };
    },
    async get(kbId: string, id: number): Promise<FAQEntry> { return parseEntry(envelope(await request({ method: 'GET', path: `${base(kbId)}/entries/${encodeURIComponent(String(id))}` }))); },
    async create(kbId: string, input: FAQEntryPayload): Promise<FAQEntry> { return parseEntry(envelope(await request({ method: 'POST', path: `${base(kbId)}/entry`, body: input }))); },
    async update(kbId: string, id: number, input: Partial<FAQEntryPayload>): Promise<FAQEntry> { return parseEntry(envelope(await request({ method: 'PUT', path: `${base(kbId)}/entries/${encodeURIComponent(String(id))}`, body: input }))); },
    async upsert(kbId: string, input: { entries: FAQEntryPayload[]; mode: 'append' | 'replace' }): Promise<{ task_id: string }> { const row = envelope(await request({ method: 'POST', path: `${base(kbId)}/entries`, body: input })); if (typeof row.task_id !== 'string' || !row.task_id) throw new Error('Invalid FAQ task id'); return { task_id: row.task_id }; },
    async updateFields(kbId: string, input: FAQEntryFieldsBatchRequest): Promise<void> { await request({ method: 'PUT', path: `${base(kbId)}/entries/fields`, body: input }); },
    async updateTags(kbId: string, input: { updates: Record<number, number | null> }): Promise<void> { await request({ method: 'PUT', path: `${base(kbId)}/entries/tags`, body: input }); },
    async removeMany(kbId: string, ids: number[]): Promise<void> { await request({ method: 'DELETE', path: `${base(kbId)}/entries`, body: { ids } }); },
    async search(kbId: string, input: FAQSearchInput): Promise<unknown> { return request({ method: 'POST', path: `${base(kbId)}/search`, body: input }); },
    async exportEntries(kbId: string, format: 'csv' | 'json' = 'csv'): Promise<string> {
      const suffix = format === 'json' ? '?format=json' : '';
      const value = await request({ method: 'GET', path: `${base(kbId)}/entries/export${suffix}` });
      return typeof value === 'string' ? value : JSON.stringify(value);
    },
  };
}
