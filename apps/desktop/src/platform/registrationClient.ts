import type { NodeRegistrationClient } from '@weknora/paseo-adapter';

export function createDesktopRegistrationClient(input: { baseURL: string; accessToken: () => string | null; fetcher?: typeof fetch }): NodeRegistrationClient {
  const fetcher = input.fetcher ?? fetch;
  const request = async (path: string, init: RequestInit = {}): Promise<any> => {
    const token = input.accessToken();
    if (!token?.trim()) throw new Error('AUTH_CREDENTIAL_MISSING');
    const response = await fetcher(new URL(path, input.baseURL), { ...init, headers: { 'content-type': 'application/json', authorization: `Bearer ${token}`, ...(init.headers ?? {}) } });
    if (!response.ok) throw new Error(`REGISTRATION_HTTP_${response.status}`);
    return response.status === 204 ? undefined : response.json();
  };
  return {
    async createChallenge(value) { const result = await request('/api/v1/execution-targets/registrations/challenges', { method: 'POST', body: JSON.stringify(value) }); return result?.data ?? result; },
    async complete(value) { const result = await request('/api/v1/execution-targets/registrations', { method: 'POST', body: JSON.stringify(value) }); return result?.data ?? result; },
    async revoke(id) { await request(`/api/v1/execution-targets/${encodeURIComponent(id)}/revoke`, { method: 'POST' }); },
  };
}
