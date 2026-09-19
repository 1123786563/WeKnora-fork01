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

export interface WikiFolderNode {
  id: string;
  parent_id: string;
  name: string;
  path: string;
  depth: number;
  sort_order: number;
  page_count: number;
  has_children: boolean;
  [key: string]: unknown;
}

export interface WikiFolder {
  id: string;
  parent_id: string;
  name: string;
  path: string;
  depth: number;
  sort_order: number;
  [key: string]: unknown;
}

export interface WikiFolderListResponse {
  parent_id: string;
  folders: WikiFolderNode[];
}

export interface WikiIndexEntry {
  slug: string;
  title: string;
  summary: string;
  parent_slug?: string;
  category_path?: string[];
  wiki_path?: string;
  depth?: number;
  sort_order?: number;
}

export interface WikiIndexGroup {
  type: string;
  total: number;
  items: WikiIndexEntry[];
  next_cursor?: string;
}

export interface WikiIndexResponse {
  intro: string;
  version: number;
  groups: WikiIndexGroup[];
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

function folders(value: unknown): WikiFolderListResponse {
  const row = object(value, 'Invalid Wiki folder list response');
  if (typeof row.parent_id !== 'string' || !Array.isArray(row.folders)) throw new Error('Invalid Wiki folder list');
  return {
    parent_id: row.parent_id,
    folders: row.folders.map((item) => {
      const folder = object(item, 'Invalid Wiki folder');
      for (const key of ['id', 'parent_id', 'name', 'path']) if (typeof folder[key] !== 'string') throw new Error(`Invalid Wiki folder field: ${key}`);
      for (const key of ['depth', 'sort_order', 'page_count']) if (typeof folder[key] !== 'number' || !Number.isSafeInteger(folder[key]) || folder[key] < 0) throw new Error(`Invalid Wiki folder field: ${key}`);
      if (typeof folder.has_children !== 'boolean') throw new Error('Invalid Wiki folder field: has_children');
      return folder as WikiFolderNode;
    }),
  };
}

function folder(value: unknown): WikiFolder {
  const row = object(value, 'Invalid Wiki folder response');
  for (const key of ['id', 'parent_id', 'name', 'path']) if (typeof row[key] !== 'string') throw new Error(`Invalid Wiki folder field: ${key}`);
  for (const key of ['depth', 'sort_order']) if (typeof row[key] !== 'number' || !Number.isSafeInteger(row[key]) || row[key] < 0) throw new Error(`Invalid Wiki folder field: ${key}`);
  return row as unknown as WikiFolder;
}

function index(value: unknown): WikiIndexResponse {
  const row = object(value, 'Invalid Wiki index response');
  if (typeof row.intro !== 'string' || typeof row.version !== 'number' || !Number.isSafeInteger(row.version) || row.version < 0 || !Array.isArray(row.groups)) throw new Error('Invalid Wiki index');
  return {
    intro: row.intro,
    version: row.version,
    groups: row.groups.map((item) => {
      const group = object(item, 'Invalid Wiki index group');
      if (typeof group.type !== 'string' || typeof group.total !== 'number' || !Number.isSafeInteger(group.total) || group.total < 0 || !Array.isArray(group.items)) throw new Error('Invalid Wiki index group');
      return {
        type: group.type,
        total: group.total,
        next_cursor: group.next_cursor === undefined ? undefined : String(group.next_cursor),
        items: group.items.map((entry) => {
          const row = object(entry, 'Invalid Wiki index entry');
          for (const key of ['slug', 'title', 'summary']) if (typeof row[key] !== 'string') throw new Error(`Invalid Wiki index entry field: ${key}`);
          return row as unknown as WikiIndexEntry;
        }),
      };
    }),
  };
}

function revisionList(value: unknown): WikiRevisionListResponse {
  const row = object(value, 'Invalid Wiki revision list response');
  if (!Array.isArray(row.revisions)) throw new Error('Invalid Wiki revision list');
  if (!Number.isSafeInteger(row.total) || (row.total as number) < 0 || !Number.isSafeInteger(row.current_version) || (row.current_version as number) < 0) throw new Error('Invalid Wiki revision pagination');
  const revisions = row.revisions.map((item) => {
    const revision = object(item, 'Invalid Wiki revision');
    for (const key of ['id', 'slug', 'title', 'summary']) if (typeof revision[key] !== 'string') throw new Error(`Invalid Wiki revision field: ${key}`);
    if (!Number.isSafeInteger(revision.version) || (revision.version as number) < 1) throw new Error('Invalid Wiki revision version');
    for (const key of ['content', 'edit_source', 'edited_at']) if (revision[key] !== undefined && typeof revision[key] !== 'string') throw new Error(`Invalid Wiki revision field: ${key}`);
    return revision as WikiPageRevision;
  });
  return { revisions, total: row.total as number, current_version: row.current_version as number };
}

function graph(value: unknown): WikiGraphData {
  // Vue parses the graph payload tolerantly: an empty wiki returns a body
  // without nodes/edges and the view degrades to the 暂无图谱数据 empty
  // state. Only present-but-malformed entries are rejected.
  const empty: WikiGraphData = { nodes: [], edges: [], meta: { mode: 'overview', total: 0, returned: 0, truncated: false } };
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return empty;
  const row = value as Record<string, unknown>;
  // The empty-wiki response carries nodes without an edges key.
  if (!Array.isArray(row.nodes) && !Array.isArray(row.edges)) return empty;
  if (!Array.isArray(row.nodes)) throw new Error('Invalid Wiki graph nodes');
  // Vue's renderer crashes on a null/missing edges array (it iterates
  // graph.edges directly), which leaves the 暂无图谱数据 empty state on
  // screen. Nodes without a usable edges array must therefore degrade the
  // whole payload to the empty graph instead of rendering isolated nodes
  // (browser-verified 2026-09-19: backend returns {nodes:[4 pages], edges:null}).
  if (!Array.isArray(row.edges)) return empty;
  const edges = row.edges;
  const nodes = row.nodes.map((item) => {
    const node = object(item, 'Invalid Wiki graph node');
    for (const key of ['slug', 'title', 'page_type']) if (typeof node[key] !== 'string') throw new Error(`Invalid Wiki graph node ${key}`);
    if (typeof node.link_count !== 'number' || !Number.isSafeInteger(node.link_count) || node.link_count < 0) throw new Error('Invalid Wiki graph node link_count');
    if (node.familiar !== undefined && typeof node.familiar !== 'boolean') throw new Error('Invalid Wiki graph node familiar');
    return node as WikiGraphNode;
  });
  const edgeRows = (edges as unknown[]).map((item) => {
    const edge = object(item, 'Invalid Wiki graph edge');
    if (typeof edge.source !== 'string' || typeof edge.target !== 'string') throw new Error('Invalid Wiki graph edge endpoints');
    return edge as WikiGraphEdge;
  });
  const emptyMeta: WikiGraphMeta = { mode: 'overview', total: 0, returned: 0, truncated: false };
  if (typeof row.meta !== 'object' || row.meta === null || Array.isArray(row.meta)) {
    return { nodes, edges, meta: { ...emptyMeta, total: nodes.length, returned: nodes.length } };
  }
  const meta = row.meta as Record<string, unknown>;
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
    // Vue getWikiStats: per-page_type counts drive the sidebar bucket tabs.
    async stats(kbId: string): Promise<{ total_pages: number; pages_by_type: Record<string, number>; pending_issues: number }> {
      const row = object(await request({ method: 'GET', path: `${base(kbId)}/stats` }), 'Invalid Wiki stats response');
      const pages_by_type: Record<string, number> = {};
      if (typeof row.pages_by_type === 'object' && row.pages_by_type !== null && !Array.isArray(row.pages_by_type)) {
        for (const [key, value] of Object.entries(row.pages_by_type as Record<string, unknown>)) {
          pages_by_type[key] = typeof value === 'number' ? value : 0;
        }
      }
      return {
        total_pages: typeof row.total_pages === 'number' ? row.total_pages : 0,
        pages_by_type,
        pending_issues: typeof row.pending_issues === 'number' ? row.pending_issues : 0,
      };
    },
    async search(kbId: string, q: string, limit = 20): Promise<WikiPageListResponse> {
      const suffix = `?q=${encodeURIComponent(q)}&limit=${limit}`;
      return list(await request({ method: 'GET', path: `${base(kbId)}/search${suffix}` }));
    },
    async issues(kbId: string, slug?: string): Promise<unknown[]> {
      const suffix = slug ? `?slug=${encodeURIComponent(slug)}` : '';
      const value = await request({ method: 'GET', path: `${base(kbId)}/issues${suffix}` });
      const root = object(value, 'Invalid Wiki issues response');
      const items = root.issues ?? root.data ?? root;
      return Array.isArray(items) ? items : [];
    },
    async folders(kbId: string, parentId = '', pageTypes: string[] = []): Promise<WikiFolderListResponse> {
      const query = new URLSearchParams();
      if (parentId) query.set('parent_id', parentId);
      if (pageTypes.length > 0) query.set('page_types', pageTypes.join(','));
      const suffix = query.toString();
      return folders(await request({ method: 'GET', path: `${base(kbId)}/folders${suffix ? `?${suffix}` : ''}` }));
    },
    async index(kbId: string, params: { types?: string[]; limit?: number; cursor?: string } = {}): Promise<WikiIndexResponse> {
      const query = new URLSearchParams();
      if (params.types && params.types.length > 0) query.set('types', params.types.join(','));
      if (params.limit !== undefined) query.set('limit', String(params.limit));
      if (params.cursor) query.set('cursor', params.cursor);
      const suffix = query.toString();
      return index(await request({ method: 'GET', path: `${base(kbId)}/index${suffix ? `?${suffix}` : ''}` }));
    },
    async createFolder(kbId: string, parentId: string, name: string): Promise<WikiFolder> {
      return folder(await request({ method: 'POST', path: `${base(kbId)}/folders`, body: { parent_id: parentId, name } }));
    },
    async updateFolder(kbId: string, folderId: string, input: { name?: string; parent_id?: string; move_parent?: boolean }): Promise<WikiFolder> {
      return folder(await request({ method: 'PUT', path: `${base(kbId)}/folders/${encodeURIComponent(folderId)}`, body: input }));
    },
    async removeFolder(kbId: string, folderId: string): Promise<void> {
      await request({ method: 'DELETE', path: `${base(kbId)}/folders/${encodeURIComponent(folderId)}` });
    },
    async movePage(kbId: string, slug: string, folderId: string): Promise<void> {
      await request({ method: 'PUT', path: `${base(kbId)}/move-page`, body: { slug, folder_id: folderId } });
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
      for (const key of ['content', 'edit_source', 'edited_at']) if (value[key] !== undefined && typeof value[key] !== 'string') throw new Error(`Invalid Wiki revision field: ${key}`);
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
