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

test('opportunity client encodes import, receipt recovery, and fixed evidence IDs', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const receipt = { kind: 'opportunity_imported', requestId: 'request /1', opportunityId: 'opportunity/1', observationId: 'observation-1', snapshotId: 'snapshot ?1', status: 'needs_review', acquiredAt: '2026-09-24T01:02:03Z' }
 const evidence = { opportunityId: 'opportunity/1', observationId: 'observation-1', snapshotId: 'snapshot ?1', rawText: 'JD text', rawSha256: 'a'.repeat(64), extracted: { title: { state: 'unknown' }, company: { state: 'unknown' }, location: { state: 'unknown' }, batch: { state: 'unknown' }, requirements: { state: 'unknown' } }, source: { kind: 'manual_paste' }, acquiredAt: receipt.acquiredAt, status: 'needs_review' }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.path.endsWith('/import') || input.path.includes('/receipt?') ? receipt : evidence
 })
 const input = { requestId: 'request /1', rawText: 'JD text', sourceLabel: 'Board & more', sourceReference: 'listing/1' }
 await api.importOpportunity(input)
 await api.opportunityReceipt(input.requestId)
 await api.opportunityEvidence(receipt.opportunityId, receipt.snapshotId)
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/opportunities/import', body: input },
  { method: 'GET', path: '/api/v1/career/opportunities/receipt?requestId=request%20%2F1' },
  { method: 'GET', path: '/api/v1/career/opportunities/opportunity%2F1?snapshotId=snapshot%20%3F1' },
 ])
})

test('opportunity client refuses blank request and evidence identifiers', async () => {
 const api = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(api.importOpportunity({ requestId: ' ', rawText: 'JD' }), /requestId/)
 await assert.rejects(api.opportunityReceipt(' '), /requestId/)
 await assert.rejects(api.opportunityEvidence('id', ' '), /IDs/)
})

test('evaluation client encodes create, receipt recovery and immutable detail paths', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const receipt = { kind: 'evaluation_created', requestId: 'req /1', evaluationId: 'eval/1', opportunityId: 'opp/1', snapshotId: 'snap ?1', profileRevision: 4, status: 'ineligible' }
 const fact = { factKey: 'education.graduation_year', value: '2026', revision: 4, factRevision: 3, source: { kind: 'manual' }, confirmation: { userId: 'owner-1', confirmedAt: '2026-09-24T01:02:03Z' }, confirmedAt: '2026-09-24T01:02:03Z' }
 const evaluation = { ...receipt, createdAt: '2026-09-24T01:02:03Z', rulesetVersion: 'career-qualification-v1', snapshot: { opportunityId: 'opp/1', observationId: 'obs-1', snapshotId: 'snap ?1', rawText: '仅限2027届', rawSha256: 'a'.repeat(64), source: { kind: 'manual_paste' }, acquiredAt: '2026-09-24T01:02:03Z' }, hard: { overall: 'ineligible', rules: [{ ruleId: 'graduation_year', criterion: 'graduation year', outcome: 'ineligible', reasonCode: 'graduation_year_mismatch', jobEvidence: { snapshotId: 'snap ?1', observationId: 'obs-1', acquiredAt: '2026-09-24T01:02:03Z', rawSha256: 'a'.repeat(64), spanStart: 0, spanEnd: 13, quotedText: '仅限2027届' }, profileEvidence: fact }] }, soft: { matches: [] }, facts: [fact] }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.path.endsWith('/evaluations') && input.method === 'POST' || input.path.includes('/receipt?') ? receipt : evaluation
 })
 const input = { requestId: 'req /1', opportunityId: 'opp/1', snapshotId: 'snap ?1', profileRevision: 4 }
 assert.deepEqual(await api.evaluateOpportunity(input), receipt)
 assert.deepEqual(await api.evaluationReceipt(input.requestId), receipt)
 assert.equal((await api.evaluation(receipt.evaluationId)).snapshot.rawText, '仅限2027届')
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/evaluations', body: input },
  { method: 'GET', path: '/api/v1/career/evaluations/receipt?requestId=req%20%2F1' },
  { method: 'GET', path: '/api/v1/career/evaluations/eval%2F1' },
 ])
})
