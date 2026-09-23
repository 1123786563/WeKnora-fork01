import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerReceipt, CareerUpload, CareerView } from '../../career-core/src/contracts.ts'
import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload } from '../../career-core/src/contracts.ts'
import type { ClientRequest } from './client.ts'

export type CareerRequest = (input: ClientRequest) => Promise<unknown>
export function createCareerApi(request: CareerRequest) {
 return {
  async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
  async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
  async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
  async receipt(requestId: string, signal?: AbortSignal): Promise<CareerReceipt> { return decodeCareerReceipt(await request({ method: 'GET', path: `/api/v1/career/receipt?requestId=${encodeURIComponent(requestId)}`, ...(signal ? { signal } : {}) })) },
  async act(action: CareerAction, signal?: AbortSignal): Promise<CareerReceipt> { return decodeCareerReceipt(await request({ method: 'POST', path: '/api/v1/career/act', body: action, ...(signal ? { signal } : {}) })) },
  async sources(signal?: AbortSignal): Promise<CareerDocumentSource[]> { return decodeCareerSources(await request({ method: 'GET', path: '/api/v1/career/sources', ...(signal ? { signal } : {}) })) },
  async upload(file: Blob, fileName: string, requestId: string, expectedRevision: number, signal?: AbortSignal): Promise<CareerUpload> {
   if (!requestId.trim()) throw new TypeError('career upload requestId must not be empty')
   const body = new FormData()
   body.append('file', file, fileName)
   body.append('requestId', requestId)
   body.append('expectedRevision', String(expectedRevision))
   return decodeCareerUpload(await request({ method: 'POST', path: '/api/v1/career/sources/upload', body, ...(signal ? { signal } : {}) }))
  },
 }
}
