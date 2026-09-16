import type { NodeRegistrationClient } from '@weknora/paseo-adapter';

export interface RegistrationClientOptions {
  baseURL: string;
  accessToken: () => Promise<string | null>;
  fetcher?: typeof fetch;
}

/** Authenticated control-plane client. The mobile node bearer is never used here. */
export function createRegistrationClient(options: RegistrationClientOptions): NodeRegistrationClient {
  const fetcher = options.fetcher ?? fetch;
  const request = async (path: string, init: RequestInit = {}): Promise<any> => {
    const token = await options.accessToken();
    if (!token?.trim()) throw new Error('AUTH_CREDENTIAL_MISSING');
    const response = await fetcher(new URL(path, options.baseURL), {
      ...init,
      headers: { 'content-type': 'application/json', authorization: `Bearer ${token}`, ...(init.headers ?? {}) },
    });
    if (!response.ok) throw new Error(`REGISTRATION_HTTP_${response.status}`);
    return response.status === 204 ? undefined : response.json();
  };
  return {
    async createChallenge(input) {
      const result = await request('/api/v1/execution-targets/registrations/challenges', { method: 'POST', body: JSON.stringify(input) });
      return result?.data ?? result;
    },
    async complete(input) {
      const result = await request('/api/v1/execution-targets/registrations', { method: 'POST', body: JSON.stringify(input) });
      return result?.data ?? result;
    },
    async revoke(registrationID) {
      await request(`/api/v1/execution-targets/${encodeURIComponent(registrationID)}/revoke`, { method: 'POST' });
    },
  };
}
