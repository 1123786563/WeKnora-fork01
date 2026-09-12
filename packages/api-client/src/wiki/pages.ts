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
  edit_source?: string;
  edited_at?: string;
  [key: string]: unknown;
}

export interface WikiRevisionListResponse {
  revisions: WikiPageRevision[];
  total: number;
  current_version: number;
}

export interface WikiGraphQueryParams {
  mode?: 'overview' | 'ego';
  center?: string;
  depth?: number;
  types?: string[];
  limit?: number;
}

export interface WikiGraphNode {
  slug: string;
  title: string;
  page_type: string;
  link_count: number;
  familiar?: boolean;
  [key: string]: unknown;
}

export interface WikiGraphEdge {
  source: string;
  target: string;
  [key: string]: unknown;
}

export interface WikiGraphMeta {
  mode: string;
  total: number;
  returned: number;
  truncated: boolean;
  center?: string;
  depth?: number;
  familiar_count?: number;
  [key: string]: unknown;
}

export interface WikiGraphData {
  nodes: WikiGraphNode[];
  edges: WikiGraphEdge[];
  meta: WikiGraphMeta;
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

function graph(value: unknown): WikiGraphData {
  const row = object(value, 'Invalid Wiki graph response');
  if (!Array.isArray(row.nodes)) throw new Error('Invalid Wiki graph nodes');
  if (!Array.isArray(row.edges)) throw new Error('Invalid Wiki graph edges');
  const nodes = row.nodes.map((item) => {
    const node = object(item, 'Invalid Wiki graph node');
    for (const key of ['slug', 'title', 'page_type']) if (typeof node[key] !== 'string') throw new Error(`Invalid Wiki graph node ${key}`);
    if (typeof node.link_count !== 'number' || !Number.isSafeInteger(node.link_count) || node.link_count < 0) throw new Error('Invalid Wiki graph node link_count');
    if (node.familiar !== undefined && typeof node.familiar !== 'boolean') throw new Error('Invalid Wiki graph node familiar');
    return node as WikiGraphNode;
  });
  const edges = row.edges.map((item) => {
    const edge = object(item, 'Invalid Wiki graph edge');
    if (typeof edge.source !== 'string' || typeof edge.target !== 'string') throw new Error('Invalid Wiki graph edge endpoints');
    return edge as WikiGraphEdge;
  });
  const meta = object(row.meta, 'Invalid Wiki graph meta');
  if (typeof meta.mode !== 'string' || meta.mode.trim() === '') throw new Error('Invalid Wiki graph meta mode');
  for (const key of ['total', 'returned']) if (typeof meta[key] !== 'number' || !Number.isSafeInteger(meta[key]) || meta[key] < 0) throw new Error(`Invalid Wiki graph meta ${key}`);
  if (typeof meta.truncated !== 'boolean') throw new Error('Invalid Wiki graph meta truncated');
  for (const key of ['center']) if (meta[key] !== undefined && typeof meta[key] !== 'string') throw new Error(`Invalid Wiki graph meta ${key}`);
  for (const key of ['depth', 'familiar_count']) if (meta[key] !== undefined && (typeof meta[key] !== 'number' || !Number.isSafeInteger(meta[key]) || meta[key] < 0)) throw new Error(`Invalid Wiki graph meta ${key}`);
  return { nodes, edges, meta: meta as WikiGraphMeta };
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
    async getRevision(kbId: string, slug: string, version: number): Promise<WikiPageRevision> {
      const value = object(await request({ method: 'GET', path: `${base(kbId)}/revisions/${pathSlug(slug)}?version=${encodeURIComponent(String(version))}` }), 'Invalid Wiki revision response');
      for (const key of ['id', 'slug', 'title', 'summary']) if (typeof value[key] !== 'string') throw new Error(`Invalid Wiki revision field: ${key}`);
      if (typeof value.version !== 'number' || !Number.isSafeInteger(value.version) || value.version < 1) throw new Error('Invalid Wiki revision version');
      return value as WikiPageRevision;
    },
    async graph(kbId: string, params: WikiGraphQueryParams = {}): Promise<WikiGraphData> {
      const query = new URLSearchParams();
      if (params.mode !== undefined) query.set('mode', params.mode);
      if (params.center !== undefined && params.center !== '') query.set('center', params.center);
      if (params.depth !== undefined) query.set('depth', String(params.depth));
      if (params.types !== undefined && params.types.length > 0) query.set('types', params.types.join(','));
      if (params.limit !== undefined) query.set('limit', String(params.limit));
      const suffix = query.toString();
      return graph(await request({ method: 'GET', path: `${base(kbId)}/graph${suffix ? `?${suffix}` : ''}` }));
    },
    async revert(kbId: string, slug: string, version: number): Promise<WikiPage> {
      return page(await request({ method: 'POST', path: `${base(kbId)}/revert`, body: { slug, version } }));
    },
  };
}
