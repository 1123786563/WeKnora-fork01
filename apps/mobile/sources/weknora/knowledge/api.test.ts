import { describe, expect, it, vi } from 'vitest';
import { createMobileKnowledgeApi } from './api';

describe('mobile knowledge API adapter', () => {
  it('uses the existing JSON client for list/detail and bearer auth for binary actions', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('preview', { status: 200, headers: { 'content-type': 'text/plain' } }));
    const api = createMobileKnowledgeApi({ backend: 'weknora', origin: 'https://example.test' }, { kind: 'bearer', accessToken: 'token' });
    const preview = await api.preview('doc-1');
    expect(new TextDecoder().decode(preview.bytes)).toBe('preview');
    expect(fetchMock).toHaveBeenCalledWith('https://example.test/api/v1/knowledge/doc-1/preview', { headers: { authorization: 'Bearer token' } });
    fetchMock.mockRestore();
  });

  it('does not make protected requests without a credential', async () => {
    const api = createMobileKnowledgeApi({ backend: 'weknora', origin: 'https://example.test' }, null);
    await expect(api.detail('doc-1')).rejects.toThrow('AUTH_REQUIRED');
  });
});
