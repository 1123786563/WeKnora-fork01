import assert from 'node:assert/strict'
import test from 'node:test'
import { createCareerApi } from './career.ts'
import { createWeKnoraClient } from './client.ts'
import { createJsonTransport } from './transport/json.ts'

test('career API calls authenticated tenant-scoped routes and preserves action body', async () => {
 const calls: Array<{ url: string; method?: string; body?: string }> = []
 const client = createWeKnoraClient({ baseURL: 'https://example.test', transport: createJsonTransport(async (url, init) => {
  calls.push({ url, method: init?.method, body: init?.body as string })
  const payload = url.includes('/receipt?') ? { kind: 'confirmed', requestId: 'rid', revision: 1, fact: { key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } : url.endsWith('/act') ? { kind: 'confirmed', requestId: 'rid', revision: 1, fact: { key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } : url.includes('/changes?') ? { revision: 0, changes: [] } : { revision: 0, facts: [], proposals: [] }
  return { status: 200, headers: { get: () => 'application/json' }, json: async () => payload, text: async () => '' }
 }) })
 const api = createCareerApi(client.request)
 await api.open()
 await api.list()
 await api.changes(2)
 await api.receipt('rid')
 const action = { action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'rid', expectedRevision: 0 } as const
 await api.act(action)
 assert.deepEqual(calls.map(({ url, method }) => [url, method]), [
  ['https://example.test/api/v1/career/open', 'GET'], ['https://example.test/api/v1/career/list', 'GET'],
  ['https://example.test/api/v1/career/changes?since=2', 'GET'], ['https://example.test/api/v1/career/receipt?requestId=rid', 'GET'],
  ['https://example.test/api/v1/career/act', 'POST'],
 ])
 assert.deepEqual(JSON.parse(calls[4]!.body!), action)
})

test('career upload sends a browser multipart body and lists source versions', async () => {
 const calls: Array<{ url: string; headers: Record<string, string>; body: unknown }> = []
 const client = createWeKnoraClient({ baseURL: 'https://example.test', transport: createJsonTransport(async (url, init) => {
  calls.push({ url, headers: init?.headers ?? {}, body: init?.body })
  const payload = url.endsWith('/sources/upload') ? { source: { id: 's1', revision: 3, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 6, digest: 'd', status: 'processing', createdAt: 'now' } } : { sources: [] }
  return { status: 201, headers: { get: () => 'application/json' }, json: async () => payload, text: async () => '' }
 }) })
 const file = new Blob(['resume'], { type: 'application/pdf' })
 await client.career.upload(file, 'resume.pdf', 'stable-request', 3)
 await client.career.sources()
 assert.equal(calls[0]?.url, 'https://example.test/api/v1/career/sources/upload')
 assert.ok(calls[0]?.body instanceof FormData)
 const form = calls[0]?.body as FormData
 assert.equal(form.get('requestId'), 'stable-request')
 assert.equal(form.get('expectedRevision'), '3')
 assert.equal((form.get('file') as File).name, 'resume.pdf')
 assert.equal(Object.keys(calls[0]?.headers ?? {}).some((key) => key.toLowerCase() === 'content-type'), false)
 assert.equal(calls[1]?.url, 'https://example.test/api/v1/career/sources')
})
