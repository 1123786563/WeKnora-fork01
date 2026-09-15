import { createJsonTransport, createWeKnoraClient, type BearerCredential, type HttpRequest, type KnowledgeDocument } from '@weknora/api-client';
import type { MobileHost } from '@/weknora/platform/host';
import { knowledgePaths } from './model';

export type KnowledgeDetail = KnowledgeDocument & Record<string, unknown>;

function authHeaders(credential: BearerCredential | null): Record<string, string> | undefined {
  return credential ? { authorization: `Bearer ${credential.accessToken}` } : undefined;
}

function client(host: MobileHost, credential: BearerCredential | null) {
  const transport = createJsonTransport(fetch);
  return createWeKnoraClient({
    baseURL: host.origin,
    transport: {
      send: (request: HttpRequest) => transport.send({
        ...request,
        headers: { ...request.headers, ...authHeaders(credential) },
      }),
    },
  });
}

function unwrap<T>(value: unknown): T {
  if (typeof value === 'object' && value !== null && 'success' in value && 'data' in value) {
    const response = value as { success?: unknown; data?: unknown };
    if (response.success !== true) throw new Error('REQUEST_REJECTED');
    return response.data as T;
  }
  return value as T;
}

async function binary(host: MobileHost, credential: BearerCredential, path: string): Promise<{ bytes: ArrayBuffer; contentType: string }> {
  const response = await fetch(`${host.origin}${path}`, { headers: { authorization: `Bearer ${credential.accessToken}` } });
  if (!response.ok) throw new Error(`HTTP_${response.status}`);
  return { bytes: await response.arrayBuffer(), contentType: response.headers.get('content-type') ?? 'application/octet-stream' };
}

export function createMobileKnowledgeApi(host: MobileHost, credential: BearerCredential | null) {
  const request = (method: string, path: string) => {
    if (!credential) throw new Error('AUTH_REQUIRED');
    return client(host, credential).request({ method, path });
  };
  return {
    listBases: async () => client(host, credential).knowledgeBases.list(),
    listDocuments: async (kbId: string) => client(host, credential).knowledge.documents.list(kbId, { page: 1, page_size: 100 }),
    detail: async (id: string) => unwrap<KnowledgeDetail>(await request('GET', knowledgePaths.detail(id))),
    preview: async (id: string) => binary(host, credential!, knowledgePaths.preview(id)),
    download: async (id: string) => binary(host, credential!, knowledgePaths.download(id)),
    listWiki: async (kbId: string) => request('GET', knowledgePaths.wikiPages(kbId)),
    listFaq: async (kbId: string) => request('GET', knowledgePaths.faqEntries(kbId)),
  };
}
