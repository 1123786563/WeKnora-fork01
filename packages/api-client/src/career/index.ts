import type { CareerRequest, CareerRequester, CareerApi } from './types.ts';
import { decodeCareerApplication, decodeCareerEvaluation, decodeCareerMaterialVersion, decodeCareerOpportunityList, decodeCareerProfile } from './decode.ts';
export type { CareerApi, CareerRequest, CareerRequester } from './types.ts';
export { decodeCareerApplication, decodeCareerEvaluation, decodeCareerMaterialVersion, decodeCareerOpportunityList, decodeCareerProfile } from './decode.ts';

export function createCareerApi(request: CareerRequester): CareerApi {
  return {
    async getProfile(requestId) { return decodeCareerProfile(await request({ method: 'GET', path: '/api/v1/career/profile', ...(requestId ? { headers: { 'X-Request-ID': requestId } } : {}) }), requestId); },
    async listOpportunities() {
      const result = await request({ method: 'GET', path: '/api/v1/career/opportunities' });
      if (!result || typeof result !== 'object' || Array.isArray(result) || !('data' in result)) throw new Error('Career opportunities response must contain data');
      return decodeCareerOpportunityList((result as { data: unknown }).data);
    },
    async getEvaluation(id) { const result = await request({ method: 'GET', path: `/api/v1/career/opportunities/${encodeURIComponent(id)}/evaluation` }); const data = result && typeof result === 'object' && 'data' in result ? (result as { data: unknown }).data : result; return decodeCareerEvaluation(data); },
    async getApplication(id) { const result = await request({ method: 'GET', path: `/api/v1/career/applications/${encodeURIComponent(id)}` }); const data = result && typeof result === 'object' && 'data' in result ? (result as { data: unknown }).data : result; return decodeCareerApplication(data); },
    async updateProfile(input) { return decodeCareerProfile(await request({ method: 'PUT', path: '/api/v1/career/profile', headers: { 'X-Request-ID': input.requestId, 'If-Match': String(input.expectedRevision) }, body: input.body }), input.requestId); },
  };
}
