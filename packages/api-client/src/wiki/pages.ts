import type { ClientRequest } from '../client.ts';

export interface WikiPage {
  id: string;
  slug: string;
  title: string;
  content: string;
  summary: string;
  version: number;
  [key: string]: unknown;
}

export interface WikiPageListResponse {
  pages: WikiPage[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface WikiPageRevision {
  id: string;
  slug: string;
  version: number;
  title: string;
  summary: string;
  content?: string;
  [key: string]: unknown;
}

export interface WikiRevisionListResponse {
  revisions: WikiPageRevision[];
  total: number;
  current_version: number;
}

export interface WikiPageUpdateInput {
  title?: string;
  content?: string;
  summary?: string;
  page_type?: string;
  status?: string;
  aliases?: string[];
  version?: number;
}

function pathSlug(slug: string): string {
  return slug.split('/').map(encodeURIComponent).join('/');
}

function object(value: unknown, message: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(message);
  return value as Record<string, unknown>;
}

function page(value: unknown): WikiPage {
  const row = object(value, 'Invalid Wiki page response');
  for (const key of ['id', 'slug', 'title', 'content', 'summary']) {
    if (typeof row[key] !== 'string') throw new Error(`Invalid Wiki page field: ${key}`);
  }
  if (typeof row.version !== 'number' || !Number.isSafeInteger(row.version) || row.version < 1) throw new Error('Invalid Wiki page version');
  return row as WikiPage;
}

function list(value: unknown): WikiPageListResponse {
  const row = object(value, 'Invalid Wiki page list response');
  if (!Array.isArray(row.pages)) throw new Error('Invalid Wiki page list pages');
  const numbers = ['total', 'page', 'page_size', 'total_pages'];
  for (const key of numbers) if (typeof row[key] !== 'number' || !Number.isSafeInteger(row[key]) || row[key] < 0) throw new Error(`Invalid Wiki page list field: ${key}`);
  return { pages: row.pages.map(page), total: row.total as number, page: row.page as number, page_size: row.page_size as number, total_pages: row.total_pages as number };
}

function revisionList(value: unknown): WikiRevisionListResponse {
  const row = object(value, 'Invalid Wiki revision list response');
  if (!Array.isArray(row.revisions)) throw new Error('Invalid Wiki revision list');
  if (typeof row.total !== 'number' || typeof row.current_version !== 'number') throw new Error('Invalid Wiki revision pagination');
  const revisions = row.revisions.map((item) => {
    const revision = object(item, 'Invalid Wiki revision');
    for (const key of ['id', 'slug', 'title', 'summary']) if (typeof revision[key] !== 'string') throw new Error(`Invalid Wiki revision field: ${key}`);
    if (typeof revision.version !== 'number') throw new Error('Invalid Wiki revision version');
    return revision as WikiPageRevision;
  });
  return { revisions, total: row.total, current_version: row.current_version };
}

export function createWikiPagesApi(request: (input: ClientRequest) => Promise<unknown>) {
  const base = (kbId: string) => `/api/v1/knowledgebase/${encodeURIComponent(kbId)}/wiki`;
  return {
    async list(kbId: string, params: Record<string, string | number | undefined> = {}): Promise<WikiPageListResponse> {
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== '') query.set(key, String(value));
      const suffix = query.toString();
      return list(await request({ method: 'GET', path: `${base(kbId)}/pages${suffix ? `?${suffix}` : ''}` }));
    },
    async get(kbId: string, slug: string): Promise<WikiPage> {
      return page(await request({ method: 'GET', path: `${base(kbId)}/pages/${pathSlug(slug)}` }));
    },
    async create(kbId: string, input: Partial<WikiPage>): Promise<WikiPage> {
      return page(await request({ method: 'POST', path: `${base(kbId)}/pages`, body: input }));
    },
    async update(kbId: string, slug: string, input: WikiPageUpdateInput): Promise<WikiPage> {
      return page(await request({ method: 'PUT', path: `${base(kbId)}/pages/${pathSlug(slug)}`, body: input }));
    },
    async remove(kbId: string, slug: string): Promise<void> {
      await request({ method: 'DELETE', path: `${base(kbId)}/pages/${pathSlug(slug)}` });
    },
    async revisions(kbId: string, slug: string, params: { limit?: number; offset?: number } = {}): Promise<WikiRevisionListResponse> {
      const query = new URLSearchParams();
      if (params.limit !== undefined) query.set('limit', String(params.limit));
      if (params.offset !== undefined) query.set('offset', String(params.offset));
      const suffix = query.toString();
      return revisionList(await request({ method: 'GET', path: `${base(kbId)}/revisions/${pathSlug(slug)}${suffix ? `?${suffix}` : ''}` }));
    },
    async revert(kbId: string, slug: string, version: number): Promise<WikiPage> {
      return page(await request({ method: 'POST', path: `${base(kbId)}/revert`, body: { slug, version } }));
    },
  };
}
