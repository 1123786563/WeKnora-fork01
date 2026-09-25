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

test('opportunity URL client posts import-url and lists owner-scoped observations with frozen enums', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const urlReceipt = { kind: 'opportunity_url_imported', requestId: 'url request/1', opportunityId: 'opp/1', observationId: 'observation-1', snapshotId: 'snapshot ?1', status: 'needs_review', sourceStatus: 'policy_unverified', completeness: 'unknown', failureCode: 'source_unverified', submittedUrl: 'https://jobs.example.test/1', acquiredAt: '2026-09-25T08:00:00Z', needsUserJD: true }
 const observations = { observations: [
  { observationId: 'observation-1', snapshotId: 'snapshot ?1', source: { kind: 'url', label: 'jobs.example.test', referenceId: 'https://jobs.example.test/1' }, sourceStatus: 'policy_unverified', completeness: 'unknown', failureCode: 'source_unverified', submittedUrl: 'https://jobs.example.test/1', needsUserJD: true, acquiredAt: '2026-09-25T08:00:00Z' },
  { observationId: 'observation-2', snapshotId: 'snapshot-paste-1', source: { kind: 'manual_paste' }, needsUserJD: false, acquiredAt: '2026-09-25T09:00:00Z' },
 ] }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.path.endsWith('/import-url') ? urlReceipt : observations
 })
 const input = { requestId: 'url request/1', url: 'https://jobs.example.test/1' }
 const receipt = await api.importUrl(input)
 assert.equal(receipt.sourceStatus, 'policy_unverified')
 assert.equal(receipt.completeness, 'unknown')
 assert.equal(receipt.failureCode, 'source_unverified')
 assert.equal(receipt.needsUserJD, true)
 assert.equal(receipt.submittedUrl, 'https://jobs.example.test/1')
 const listed = await api.opportunityObservations(receipt.opportunityId)
 assert.equal(listed.observations.length, 2)
 assert.equal(listed.observations[0]?.sourceStatus, 'policy_unverified')
 assert.equal(listed.observations[1]?.source.kind, 'manual_paste')
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/opportunities/import-url', body: input },
  { method: 'GET', path: '/api/v1/career/opportunities/opp%2F1/observations' },
 ])
})

test('importUrl refuses blank identifiers and never decodes invented enum values', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.importUrl({ requestId: ' ', url: 'https://example.test/jd' }), /requestId/)
 await assert.rejects(refusing.importUrl({ requestId: 'r', url: ' ' }), /url/)
 await assert.rejects(refusing.opportunityObservations(' '), /opportunity/)
 const inventing = createCareerApi(async () => ({ kind: 'opportunity_url_imported', requestId: 'r', opportunityId: 'o', observationId: 'b', snapshotId: 's', status: 'needs_review', sourceStatus: 'super_verified', completeness: 'unknown', submittedUrl: 'u', acquiredAt: '2026-09-25T08:00:00Z', needsUserJD: true }))
 await assert.rejects(inventing.importUrl({ requestId: 'r', url: 'https://example.test/jd' }), TypeError)
 const observing = createCareerApi(async () => ({ observations: [{ observationId: 'o', snapshotId: 's', source: { kind: 'url' }, sourceStatus: 'totally_fine', needsUserJD: true, acquiredAt: '2026-09-25T08:00:00Z' }] }))
 await assert.rejects(observing.opportunityObservations('opp-1'), TypeError)
})

test('importOpportunity appends a paste onto a URL observation only with both owner-scoped IDs', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const receipt = { kind: 'opportunity_imported', requestId: 'paste-2', opportunityId: 'opp/1', observationId: 'observation-2', snapshotId: 'snapshot-2', status: 'stored', acquiredAt: '2026-09-25T09:00:00Z' }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return receipt
 })
 await api.importOpportunity({ requestId: 'paste-2', rawText: '完整粘贴的职位描述', opportunityId: 'opp/1', priorObservationId: 'observation-1' })
 assert.deepEqual(calls, [{ method: 'POST', path: '/api/v1/career/opportunities/import', body: { requestId: 'paste-2', rawText: '完整粘贴的职位描述', opportunityId: 'opp/1', priorObservationId: 'observation-1' } }])
 await assert.rejects(api.importOpportunity({ requestId: 'half', rawText: 'jd', opportunityId: 'opp/1' }), /opportunityId and priorObservationId/)
 await assert.rejects(api.importOpportunity({ requestId: 'half', rawText: 'jd', priorObservationId: 'observation-1' }), /opportunityId and priorObservationId/)
})

// T14 Step 0: the T09 backend deliberately stores empty-text snapshots for
// failed URL observations (SHA-256 of zero bytes, needs_review). The api-client
// evidence decode must open those evidence pages: rawText '' is accepted while
// every other field keeps its strict validation.
test('opportunity evidence decode opens the empty-rawText failure snapshot and keeps other fields strict', async () => {
 const emptySnapshot = { opportunityId: 'opp/1', observationId: 'observation-1', snapshotId: 'snapshot ?1', rawText: '', rawSha256: 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', extracted: { title: { state: 'unknown' }, company: { state: 'unknown' }, location: { state: 'unknown' }, batch: { state: 'unknown' }, requirements: { state: 'unknown' } }, source: { kind: 'url' }, acquiredAt: '2026-09-24T01:02:03Z', status: 'needs_review' }
 const api = createCareerApi(async () => emptySnapshot)
 const evidence = await api.opportunityEvidence('opp/1', 'snapshot ?1')
 assert.equal(evidence.rawText, '')
 assert.equal(evidence.rawSha256, 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')
 const blankOnly = createCareerApi(async () => ({ ...emptySnapshot, rawText: ' \n\t ' }))
 await assert.rejects(blankOnly.opportunityEvidence('opp/1', 'snapshot ?1'), TypeError)
 const badDigest = createCareerApi(async () => ({ ...emptySnapshot, rawSha256: 'not-a-sha' }))
 await assert.rejects(badDigest.opportunityEvidence('opp/1', 'snapshot ?1'), TypeError)
 const inventedStatus = createCareerApi(async () => ({ ...emptySnapshot, status: 'super_stored' }))
 await assert.rejects(inventedStatus.opportunityEvidence('opp/1', 'snapshot ?1'), TypeError)
})

// T14: four frozen application routes (POST /applications, GET receipt,
// GET :applicationId, POST link/reconcile) with strict receipt decoding.
test('application client encodes create, receipt, detail and reconcile routes with pinned evidence', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const receipt = { applicationId: 'app/1', requestId: 'apply /1', linkState: 'linking', qualified: true, pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snap ?1', evaluationId: 'eval/1', profileRevision: 4, evaluationStatus: 'eligible', batchIdentity: '2026 autumn campus' } }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return receipt
 })
 const input = { requestId: 'apply /1', opportunityId: 'opp/1', snapshotId: 'snap ?1', evaluationId: 'eval/1', batchIdentity: '2026 Autumn Campus', continueDespiteHardFailure: false, expectedRevision: 4 }
 assert.deepEqual(await api.createApplication(input), receipt)
 assert.deepEqual(await api.applicationReceipt('apply /1'), receipt)
 assert.deepEqual(await api.application('app/1'), receipt)
 assert.deepEqual(await api.reconcileApplicationLink('apply /1'), receipt)
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/applications', body: input },
  { method: 'GET', path: '/api/v1/career/applications/receipt?requestId=apply%20%2F1' },
  { method: 'GET', path: '/api/v1/career/applications/app%2F1' },
  { method: 'POST', path: '/api/v1/career/applications/link/reconcile', body: { requestId: 'apply /1' } },
 ])
})

test('application decode accepts the full frozen receipt shape including warning and ready link', async () => {
 const ready = { applicationId: 'app-2', requestId: 'apply-2', linkState: 'ready', taskId: 'task-9', runId: 'run-9', qualified: false, warning: { evaluationId: 'eval/1', evaluationStatus: 'ineligible', hardRuleId: 'graduation_year', reasonCode: 'graduation_year_mismatch' }, pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snap-1', evaluationId: 'eval/1', profileRevision: 4, evaluationStatus: 'ineligible', batchIdentity: 'autumn' } }
 const api = createCareerApi(async () => ready)
 const decoded = await api.applicationReceipt('apply-2')
 assert.equal(decoded.linkState, 'ready')
 assert.equal(decoded.taskId, 'task-9')
 assert.equal(decoded.qualified, false)
 assert.deepEqual(decoded.warning, ready.warning)
 assert.equal(decoded.pinnedEvidence.profileRevision, 4)
 const failed = createCareerApi(async () => ({ ...ready, linkState: 'link_failed', taskId: undefined, runId: undefined }))
 const failedDecoded = await failed.applicationReceipt('apply-2')
 assert.equal(failedDecoded.linkState, 'link_failed')
 assert.equal(failedDecoded.taskId, undefined)
})

test('application client refuses blank identifiers, invalid revisions and invented enum values', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 const input = { requestId: 'apply-1', opportunityId: 'opp/1', snapshotId: 'snap-1', evaluationId: 'eval/1', batchIdentity: 'batch-1', continueDespiteHardFailure: false, expectedRevision: 3 }
 await assert.rejects(refusing.createApplication({ ...input, requestId: ' ' }), /identifiers/)
 await assert.rejects(refusing.createApplication({ ...input, batchIdentity: ' ' }), /identifiers/)
 await assert.rejects(refusing.createApplication({ ...input, expectedRevision: -1 }), /identifiers/)
 await assert.rejects(refusing.applicationReceipt(' '), /requestId/)
 await assert.rejects(refusing.application(' '), /application ID/)
 await assert.rejects(refusing.reconcileApplicationLink(' '), /requestId/)
 const base = { applicationId: 'app-1', requestId: 'apply-1', qualified: true, pinnedEvidence: { opportunityId: 'opp/1', snapshotId: 'snap-1', evaluationId: 'eval/1', profileRevision: 4, evaluationStatus: 'eligible', batchIdentity: 'batch-1' } }
 const inventors: Array<Record<string, unknown>> = [
  { ...base, linkState: 'connected' },
  { ...base, linkState: 'ready', pinnedEvidence: { ...base.pinnedEvidence, evaluationStatus: 'maybe' } },
  { ...base, pinnedEvidence: { ...base.pinnedEvidence, profileRevision: -2 } },
  { ...base, warning: { evaluationId: 'eval/1' } },
  { ...base, pinnedEvidence: { ...base.pinnedEvidence, batchIdentity: '' } },
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.applicationReceipt('apply-1'), TypeError)
 }
})

// T11: one-shot search client — POST /searches, GET receipt by request ID,
// GET result by search ID — with the frozen coverage/row enums decoded
// strictly (no invented status, qualification, uncertainty, or failure code
// ever reaches the UI).
test('search client posts one-shot searches and recovers by request ID and search ID', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const receipt = { kind: 'search_once', requestId: 'search /1', searchId: 'search-1', status: 'completed', query: '上海 前端 实习', coverage: { sources: [{ sourceId: 'board-1', label: '校招看板', accessMethods: ['public_listing'], cities: ['上海'], available: true }] }, scopeNotes: [], results: [{ resultId: 'result-1', sourceId: 'board-1', link: 'https://jobs.example.test/1', checkedAt: '2026-09-25T08:00:00Z', qualification: 'needs_review', uncertainty: 'low_confidence' }], checkedAt: '2026-09-25T08:00:00Z' }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return receipt
 })
 const input = { requestId: 'search /1', query: '上海 前端 实习', expectedRevision: 3 }
 assert.deepEqual(await api.searchOnce(input), receipt)
 assert.deepEqual(await api.searchReceipt('search /1'), receipt)
 assert.deepEqual(await api.search('search-1'), receipt)
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/searches', body: input },
  { method: 'GET', path: '/api/v1/career/searches/receipt?requestId=search%20%2F1' },
  { method: 'GET', path: '/api/v1/career/searches/search-1' },
 ])
})

test('search decode accepts the empty-coverage failure receipt and keeps failure enums honest', async () => {
 const failed = { kind: 'search_once', requestId: 'r-1', searchId: 's-1', status: 'failed', query: '找岗', coverage: { sources: [] }, scopeNotes: ['no vetted search source is configured; nothing was fetched and no result is fabricated'], failureCode: 'no_vetted_sources', results: [], checkedAt: '2026-09-25T08:00:00Z' }
 const api = createCareerApi(async () => failed)
 const decoded = await api.searchOnce({ requestId: 'r-1', query: '找岗', expectedRevision: 0 })
 assert.equal(decoded.status, 'failed')
 assert.equal(decoded.failureCode, 'no_vetted_sources')
 assert.equal(decoded.coverage.sources.length, 0)
 assert.deepEqual(decoded.scopeNotes, failed.scopeNotes)
 const unavailable = createCareerApi(async () => ({ ...failed, failureCode: 'all_sources_unavailable', coverage: { sources: [{ sourceId: 'board-2', label: '区域看板', accessMethods: ['public_listing'], cities: ['杭州'], available: false, failureCode: 'network_error' }] } }))
 const unavailableDecoded = await unavailable.search('s-1')
 assert.equal(unavailableDecoded.failureCode, 'all_sources_unavailable')
 assert.equal(unavailableDecoded.coverage.sources[0]?.failureCode, 'network_error')
 assert.equal(unavailableDecoded.coverage.sources[0]?.available, false)
})

test('search client refuses blank identifiers, invalid revisions and invented enum values', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.searchOnce({ requestId: ' ', query: '前端', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.searchOnce({ requestId: 'r-1', query: ' ', expectedRevision: 0 }), /query/)
 await assert.rejects(refusing.searchOnce({ requestId: 'r-1', query: '前端', expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.searchOnce({ requestId: 'r-1', query: '前端', expectedRevision: 1.5 }), /revision/)
 await assert.rejects(refusing.searchReceipt(' '), /requestId/)
 await assert.rejects(refusing.search(' '), /search ID/)
 const base = { kind: 'search_once', requestId: 'r-1', searchId: 's-1', status: 'completed', query: '找岗', coverage: { sources: [] }, scopeNotes: [], results: [], checkedAt: '2026-09-25T08:00:00Z' }
 const inventors: Array<Record<string, unknown>> = [
  { ...base, kind: 'search_rule' },
  { ...base, status: 'running' },
  { ...base, failureCode: 'mystery_failure' },
  { ...base, results: [{ resultId: 'r1', sourceId: 'board-1', link: 'https://jobs.example.test/1', checkedAt: '2026-09-25T08:00:00Z', qualification: 'maybe', uncertainty: 'low_confidence' }] },
  { ...base, results: [{ resultId: 'r1', sourceId: 'board-1', link: 'https://jobs.example.test/1', checkedAt: '2026-09-25T08:00:00Z', qualification: 'needs_review', uncertainty: 'high' }] },
  { ...base, results: [{ resultId: 'r1', sourceId: 'board-1', link: 'javascript:alert(1)', checkedAt: '2026-09-25T08:00:00Z', qualification: 'needs_review', uncertainty: 'low_confidence' }] },
  { ...base, results: [{ resultId: 'r1', sourceId: 'board-1', link: 'https://jobs.example.test/1', checkedAt: 'yesterday', qualification: 'needs_review', uncertainty: 'low_confidence' }] },
  { ...base, coverage: { sources: [{ sourceId: 'board-1', label: 'B', accessMethods: [], cities: [], available: false, failureCode: 'invented_code' }] } },
  { ...base, scopeNotes: 'none' },
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.searchOnce({ requestId: 'r-1', query: '找岗', expectedRevision: 0 }), TypeError)
 }
})

// T15: seven frozen material routes (POST /materials, POST /materials/confirm,
// GET receipt, GET :materialId, GET versions, GET versions/:versionId,
// GET versions/:versionId/compare?baseline=N) with strict decoding of the
// structured body, pinned evidence, review risks, and version comparisons.
const materialPin = { opportunityId: 'opp/1', snapshotId: 'snap ?1', snapshotSha256: 'a'.repeat(64), profileRevision: 4 }
const materialBody = { sections: [{ heading: '教育经历', content: '计算机科学与技术本科', claims: [
 { claimId: 'claim-edu', text: '最高学历为本科', factKey: '学历', needsReview: false },
 { claimId: 'claim-intern', text: '实习经历待补充', needsReview: true, reviewNote: '缺少实习证明' },
] }] }
const materialRisks = [{ code: 'missing_placeholder', message: '缺失或待补充信息占位（实习经历待补充）：不得由系统补造', claimId: 'claim-intern' }]
const materialTs = '2026-09-25T08:00:00Z'

test('material client encodes edit, confirm, receipt, detail, versions, version and compare routes', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const editedReceipt = { kind: 'material_edited', requestId: 'edit /1', materialId: 'mat/1', status: 'draft', pinnedEvidence: materialPin, body: materialBody, reviewRisks: materialRisks }
 const confirmedReceipt = { kind: 'material_confirmed', requestId: 'confirm /1', materialId: 'mat/1', status: 'confirmed', version: 2, pinnedEvidence: materialPin, body: materialBody, reviewRisks: materialRisks }
 const view = { materialId: 'mat/1', status: 'confirmed', pinnedEvidence: materialPin, body: materialBody, reviewRisks: materialRisks, failureCode: 'claim_unconfirmed', failureMessage: 'career material claim references an unconfirmed fact: 城市', versionCount: 2, versions: [{ version: 1, createdAt: materialTs }, { version: 2, createdAt: materialTs }], createdAt: materialTs, updatedAt: materialTs }
 const versionList = { materialId: 'mat/1', versions: [{ version: 1, createdAt: materialTs }, { version: 2, createdAt: materialTs }] }
 const versionView = { version: 1, pinnedEvidence: materialPin, factBasisRevision: 4, body: materialBody, reviewRisks: materialRisks, requestId: 'confirm /1', createdAt: materialTs }
 const comparison = { materialId: 'mat/1', baseline: versionView, target: { ...versionView, version: 2 }, changes: [{ kind: 'section_changed', heading: '教育经历', baseline: '计算机科学与技术本科', target: '软件工程硕士' }] }
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  if (input.path === '/api/v1/career/materials' && input.method === 'POST') return editedReceipt
  if (input.path === '/api/v1/career/materials/confirm') return confirmedReceipt
  if (input.path.includes('/materials/receipt?')) return editedReceipt
  if (input.path.endsWith('/versions/2/compare?baseline=1')) return comparison
  if (input.path.endsWith('/versions/2')) return { ...versionView, version: 2 }
  if (input.path.endsWith('/versions')) return versionList
  if (input.path.includes('/materials/mat%2F1')) return view
  throw new Error(`unexpected path ${input.path}`)
 })
 const editInput = { requestId: 'edit /1', opportunityId: 'opp/1', snapshotId: 'snap ?1', body: materialBody, expectedRevision: 4 }
 const confirmInput = { requestId: 'confirm /1', materialId: 'mat/1', expectedRevision: 4 }
 assert.deepEqual(await api.editMaterial(editInput), editedReceipt)
 assert.deepEqual(await api.confirmMaterial(confirmInput), confirmedReceipt)
 assert.deepEqual(await api.materialReceipt('edit /1'), editedReceipt)
 assert.deepEqual(await api.material('mat/1'), view)
 assert.deepEqual(await api.materialVersions('mat/1'), versionList)
 assert.deepEqual(await api.materialVersion('mat/1', 2), { ...versionView, version: 2 })
 assert.deepEqual(await api.compareMaterialVersions('mat/1', 1, 2), comparison)
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/materials', body: editInput },
  { method: 'POST', path: '/api/v1/career/materials/confirm', body: confirmInput },
  { method: 'GET', path: '/api/v1/career/materials/receipt?requestId=edit%20%2F1' },
  { method: 'GET', path: '/api/v1/career/materials/mat%2F1' },
  { method: 'GET', path: '/api/v1/career/materials/mat%2F1/versions' },
  { method: 'GET', path: '/api/v1/career/materials/mat%2F1/versions/2' },
  { method: 'GET', path: '/api/v1/career/materials/mat%2F1/versions/2/compare?baseline=1' },
 ])
})

test('material decode keeps a minimal edit receipt, a risk-free version and a no-change comparison', async () => {
 const minimal = { kind: 'material_edited', requestId: 'e-1', materialId: 'm-1', status: 'draft', pinnedEvidence: materialPin, body: { sections: [{ heading: '自我介绍', content: '', claims: [] }] }, reviewRisks: [] }
 const identical = { materialId: 'm-1', baseline: { version: 1, pinnedEvidence: materialPin, factBasisRevision: 4, body: minimal.body, reviewRisks: [], requestId: 'c-1', createdAt: materialTs }, target: { version: 2, pinnedEvidence: materialPin, factBasisRevision: 4, body: minimal.body, reviewRisks: [], requestId: 'c-2', createdAt: materialTs }, changes: [] }
 const sent: unknown[] = []
 const api = createCareerApi(async () => { const next = sent.length === 0 ? minimal : identical; sent.push(next); return next })
 assert.deepEqual(await api.editMaterial({ requestId: 'e-1', opportunityId: 'opp/1', snapshotId: 'snap ?1', body: minimal.body, expectedRevision: 0 }), minimal)
 assert.deepEqual(await api.compareMaterialVersions('m-1', 1, 2), identical)
})

test('material client refuses blank identifiers, invalid revisions, bodiless creates and invented enums', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 const body = { sections: [{ heading: '教育经历', content: 'x', claims: [] }] }
 await assert.rejects(refusing.editMaterial({ requestId: ' ', opportunityId: 'opp/1', snapshotId: 'snap/1', body, expectedRevision: 1 }), /requestId/)
 await assert.rejects(refusing.editMaterial({ requestId: 'e-1', snapshotId: 'snap/1', body, expectedRevision: 1 }), /opportunity/)
 await assert.rejects(refusing.editMaterial({ requestId: 'e-1', opportunityId: 'opp/1', snapshotId: ' ', body, expectedRevision: 1 }), /snapshot/)
 await assert.rejects(refusing.editMaterial({ requestId: 'e-1', opportunityId: 'opp/1', snapshotId: 'snap/1', body: { sections: [] }, expectedRevision: 1 }), /body/)
 await assert.rejects(refusing.editMaterial({ requestId: 'e-1', opportunityId: 'opp/1', snapshotId: 'snap/1', body, expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.confirmMaterial({ requestId: ' ', materialId: 'm-1', expectedRevision: 1 }), /requestId/)
 await assert.rejects(refusing.confirmMaterial({ requestId: 'c-1', materialId: ' ', expectedRevision: 1 }), /materialId/)
 await assert.rejects(refusing.materialReceipt(' '), /requestId/)
 await assert.rejects(refusing.material(' '), /material ID/)
 await assert.rejects(refusing.materialVersions(' '), /material ID/)
 await assert.rejects(refusing.materialVersion('m-1', 0), /version/)
 await assert.rejects(refusing.compareMaterialVersions('m-1', 0, 2), /version/)
 const base = { kind: 'material_edited', requestId: 'e-1', materialId: 'm-1', status: 'draft', pinnedEvidence: materialPin, body: materialBody, reviewRisks: materialRisks }
 const receiptInventors: Array<Record<string, unknown>> = [
  { ...base, kind: 'material_deleted' },
  { ...base, status: 'published' },
  { ...base, version: 0 },
  { ...base, pinnedEvidence: { ...materialPin, snapshotSha256: 'deadbeef' } },
  { ...base, pinnedEvidence: { ...materialPin, profileRevision: 4.5 } },
  { ...base, reviewRisks: [{ code: 'fabricated_value', message: 'x' }] },
  { ...base, body: { sections: [] } },
  { ...base, body: { sections: [{ heading: ' ', content: 'x', claims: [] }] } },
  { ...base, body: { sections: [{ heading: 'h', content: 'x', claims: [{ claimId: 'c', text: 't', needsReview: 'yes' }] }] } },
  { ...base, body: { sections: [{ heading: 'h', content: 'x', claims: [{ claimId: ' ', text: 't', needsReview: true }] }] } },
  { ...base, failureCode: 7 },
 ]
 for (const payload of receiptInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.materialReceipt('e-1'), TypeError)
 }
 const version = { version: 1, pinnedEvidence: materialPin, factBasisRevision: 4, body: materialBody, reviewRisks: materialRisks, requestId: 'c-1', createdAt: materialTs }
 const versionInventors: Array<Record<string, unknown>> = [
  { ...version, createdAt: 'just now' },
  { ...version, factBasisRevision: -1 },
  { ...version, requestId: '' },
 ]
 for (const payload of versionInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.materialVersion('m-1', 1), TypeError)
 }
 const comparisonInventors: Array<Record<string, unknown>> = [
  { materialId: 'm-1', baseline: version, target: version, changes: [{ kind: 'section_moved', heading: 'h' }] },
  { materialId: 'm-1', baseline: version, target: version, changes: [{ kind: 'claim_changed', claimId: 'c', baseline: 3, target: 't' }] },
  { materialId: ' ', baseline: version, target: version, changes: [] },
 ]
 for (const payload of comparisonInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.compareMaterialVersions('m-1', 1, 2), TypeError)
 }
})

// T17 progress timeline: append/correct/history/receipt follow the frozen
// backend enums (internal/modules/career/progress.go). Decoders reject
// invented kinds, stages, event types, or provenance before they reach the UI.
const progressTs = '2026-09-25T08:00:00Z'
const progressReceipt = (kind: 'progress_appended' | 'progress_corrected' = 'progress_appended') => ({ kind, requestId: 'p-req /1', applicationId: 'app /1', eventId: 'evt-2', seq: 2, revision: 2, ...(kind === 'progress_corrected' ? { correctsEventId: 'evt-1' } : {}), eventType: 'interview', stage: 'interview', note: '一面', occurredAt: progressTs, source: { kind: 'manual' }, confirmer: 'owner-1', createdAt: progressTs })
const progressEvent = (seq: number, corrected = false) => ({ eventId: `evt-${seq}`, seq, kind: 'progress_appended', eventType: seq === 1 ? 'submitted' : 'interview', note: `事件 ${seq}`, occurredAt: progressTs, source: { kind: 'manual' }, confirmer: 'owner-1', corrected, requestId: `req-${seq}`, createdAt: progressTs })
const progressView = () => ({ applicationId: 'app /1', revision: 2, stage: 'interview', events: [progressEvent(1), progressEvent(2)] })

test('progress client encodes append, correct, history and receipt recovery paths', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.method === 'GET' && !input.path.includes('/receipt?') ? progressView() : progressReceipt(input.path.endsWith('/progress/correct') ? 'progress_corrected' : 'progress_appended')
 })
 assert.deepEqual(await api.appendProgress({ requestId: 'p-req /1', applicationId: 'app /1', eventType: 'interview', note: '一面', expectedRevision: 1 }), progressReceipt())
 assert.deepEqual(await api.correctProgress({ requestId: 'p-cor /1', applicationId: 'app /1', correctsEventId: 'evt-1', eventType: 'assessment', note: '实为测评', expectedRevision: 2 }), progressReceipt('progress_corrected'))
 assert.deepEqual(await api.applicationProgress('app /1'), progressView())
 assert.deepEqual(await api.progressReceipt('p-req /1'), progressReceipt())
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/progress', body: { requestId: 'p-req /1', applicationId: 'app /1', eventType: 'interview', note: '一面', source: { kind: 'manual' }, expectedRevision: 1 } },
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/progress/correct', body: { requestId: 'p-cor /1', applicationId: 'app /1', correctsEventId: 'evt-1', eventType: 'assessment', note: '实为测评', source: { kind: 'manual' }, expectedRevision: 2 } },
  { method: 'GET', path: '/api/v1/career/applications/app%20%2F1/progress' },
  { method: 'GET', path: '/api/v1/career/progress/receipt?requestId=p-req%20%2F1' },
 ])
})

test('progress client refuses blank identifiers, invalid revisions and invented enums', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.appendProgress({ requestId: ' ', applicationId: 'app-1', eventType: 'interview', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.appendProgress({ requestId: 'p-1', applicationId: ' ', eventType: 'interview', expectedRevision: 0 }), /applicationId/)
 await assert.rejects(refusing.appendProgress({ requestId: 'p-1', applicationId: 'app-1', eventType: 'ghost', expectedRevision: 0 }), /eventType/)
 await assert.rejects(refusing.appendProgress({ requestId: 'p-1', applicationId: 'app-1', eventType: 'interview', occurredAt: 'just now', expectedRevision: 0 }), /occurredAt/)
 await assert.rejects(refusing.appendProgress({ requestId: 'p-1', applicationId: 'app-1', eventType: 'interview', expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.correctProgress({ requestId: 'p-1', applicationId: 'app-1', correctsEventId: ' ', eventType: 'interview', expectedRevision: 0 }), /correctsEventId/)
 await assert.rejects(refusing.progressReceipt(' '), /requestId/)
 await assert.rejects(refusing.applicationProgress(' '), /applicationId/)
 const receiptInventors: Array<Record<string, unknown>> = [
  { ...progressReceipt(), kind: 'progress_deleted' },
  { ...progressReceipt(), eventType: 'ghost' },
  { ...progressReceipt(), stage: 'ghost' },
  { ...progressReceipt(), seq: 0 },
  { ...progressReceipt(), revision: -1 },
  { ...progressReceipt(), source: { kind: 'email_bot' } },
  { ...progressReceipt(), occurredAt: 'just now' },
  { ...progressReceipt(), confirmer: ' ' },
  { ...progressReceipt(), kind: 'progress_corrected' },
 ]
 for (const payload of receiptInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.progressReceipt('p-req /1'), TypeError)
 }
 const viewInventors: Array<Record<string, unknown>> = [
  { ...progressView(), stage: 'ghost' },
  { ...progressView(), revision: 1.5 },
  { ...progressView(), events: 'none' },
  { ...progressView(), events: [{ ...progressEvent(1), eventType: 'ghost' }] },
  { ...progressView(), events: [{ ...progressEvent(1), kind: 'progress_ghosted' }] },
  { ...progressView(), events: [{ ...progressEvent(1), corrected: 'yes' }] },
  { ...progressView(), events: [{ ...progressEvent(1), source: { kind: 'ghost' } }] },
  { ...progressView(), events: [{ ...progressEvent(1), eventId: ' ' }] },
  { ...progressView(), events: [{ ...progressEvent(1), kind: 'progress_corrected' }] },
 ]
 for (const payload of viewInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.applicationProgress('app /1'), TypeError)
 }
})

// T16 material exports: publish/list/signed-url/authenticated-download/revoke
// follow the frozen backend contract (internal/modules/career/rendering.go).
// One publish renders a same-body PDF/DOCX pair; only the both-verified
// export is submittable; downloads redeem a short-lived grant while still
// carrying authentication through the binary transport.
const exportTs = '2026-09-25T09:00:00Z'
const exportDigest = 'f'.repeat(64)
const publishedExport = () => ({
 kind: 'material_published', requestId: 'pub /1', exportId: 'exp /1', materialId: 'mat /1', version: 2, status: 'submittable', submittable: true, contentDigest: exportDigest,
 files: [
  { format: 'pdf', materialId: 'mat /1', version: 2, contentDigest: exportDigest, objectKey: 'objects/pdf-1', fileDigest: 'a'.repeat(64), size: 2048, verified: true },
  { format: 'docx', materialId: 'mat /1', version: 2, contentDigest: exportDigest, objectKey: 'objects/docx-1', fileDigest: 'b'.repeat(64), size: 4096, verified: true },
 ],
 createdAt: exportTs,
})
const revokedExport = () => ({ ...publishedExport(), kind: 'material_export_revoked', requestId: 'rev /1', status: 'revoked', submittable: false, revokedAt: exportTs })
const exportGrant = () => ({ exportId: 'exp /1', materialId: 'mat /1', version: 2, format: 'pdf', digest: 'a'.repeat(64), size: 2048, expiresAt: 1790000000, signature: 'c0ffee'.repeat(4), url: `/api/v1/career/materials/mat%20%2F1/exports/exp%20%2F1/download?format=pdf&expires=1790000000&signature=${'c0ffee'.repeat(4)}` })

test('material export client walks the five-route surface and decodes strictly', async () => {
 const calls: Array<{ kind: 'json' | 'binary'; method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ kind: 'json', method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  if (input.method === 'POST' && input.path.endsWith('/exports')) return publishedExport()
  if (input.method === 'POST' && input.path.endsWith('/signed-url')) return exportGrant()
  if (input.method === 'DELETE') return revokedExport()
  return { materialId: 'mat /1', exports: [publishedExport(), revokedExport()] }
 }, async (input) => {
  calls.push({ kind: 'binary', method: input.method, path: input.path })
  return { body: new Blob(['%PDF-1.4 career material'], { type: 'application/pdf' }), contentType: 'application/pdf', headers: { 'content-type': 'application/pdf' } }
 })
 const published = await api.publishMaterial({ requestId: 'pub /1', materialId: 'mat /1', version: 2, expectedRevision: 5 })
 assert.equal(published.kind, 'material_published')
 assert.equal(published.submittable, true)
 assert.equal(published.files.length, 2)
 assert.ok(published.files.every((file) => file.verified))
 assert.equal(published.files[0]?.fileDigest, 'a'.repeat(64))
 assert.deepEqual(await api.materialExports('mat /1'), { materialId: 'mat /1', exports: [publishedExport(), revokedExport()] })
 const grant = await api.materialExportSignedURL('mat /1', 'exp /1', 'pdf', 600)
 assert.equal(grant.digest, 'a'.repeat(64))
 const downloaded = await api.materialExportDownload(grant)
 assert.ok(downloaded.body instanceof Blob)
 assert.equal(downloaded.format, 'pdf')
 assert.equal(downloaded.digest, grant.digest)
 const revoked = await api.revokeMaterialExport({ requestId: 'rev /1', materialId: 'mat /1', exportId: 'exp /1', expectedRevision: 5 })
 assert.equal(revoked.status, 'revoked')
 assert.equal(revoked.revokedAt, exportTs)
 assert.deepEqual(calls, [
  { kind: 'json', method: 'POST', path: '/api/v1/career/materials/mat%20%2F1/exports', body: { requestId: 'pub /1', version: 2, expectedRevision: 5 } },
  { kind: 'json', method: 'GET', path: '/api/v1/career/materials/mat%20%2F1/exports' },
  { kind: 'json', method: 'POST', path: '/api/v1/career/materials/mat%20%2F1/exports/exp%20%2F1/signed-url', body: { format: 'pdf', ttlSeconds: 600 } },
  { kind: 'binary', method: 'GET', path: `/api/v1/career/materials/mat%20%2F1/exports/exp%20%2F1/download?format=pdf&expires=1790000000&signature=${'c0ffee'.repeat(4)}` },
  { kind: 'json', method: 'DELETE', path: '/api/v1/career/materials/mat%20%2F1/exports/exp%20%2F1', body: { requestId: 'rev /1', expectedRevision: 5 } },
 ])
})

test('material export decoders reject invented statuses, broken digests and half-verified submittable exports', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.publishMaterial({ requestId: ' ', materialId: 'm-1', version: 1, expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.publishMaterial({ requestId: 'p-1', materialId: ' ', version: 1, expectedRevision: 0 }), /materialId/)
 await assert.rejects(refusing.publishMaterial({ requestId: 'p-1', materialId: 'm-1', version: 0, expectedRevision: 0 }), /version/)
 await assert.rejects(refusing.publishMaterial({ requestId: 'p-1', materialId: 'm-1', version: 1, expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.materialExports(' '), /materialId/)
 await assert.rejects(refusing.materialExportSignedURL('m-1', 'e-1', 'pptx' as 'pdf', 600), /format/)
 await assert.rejects(refusing.materialExportSignedURL('m-1', 'e-1', 'pdf', 0), /ttl/)
 await assert.rejects(refusing.materialExportSignedURL('m-1', 'e-1', 'pdf', 901), /ttl/)
 await assert.rejects(refusing.revokeMaterialExport({ requestId: 'r-1', materialId: 'm-1', exportId: ' ', expectedRevision: 0 }), /exportId/)
 const receiptInventors: Array<Record<string, unknown>> = [
  { ...publishedExport(), kind: 'material_half_published' },
  { ...publishedExport(), status: 'half_published' },
  { ...publishedExport(), submittable: false },
  { ...publishedExport(), contentDigest: 'f'.repeat(63) },
  { ...publishedExport(), files: [{ ...publishedExport().files[0] }] },
  { ...publishedExport(), files: [...publishedExport().files, { format: 'pdf', materialId: 'mat /1', version: 2, contentDigest: exportDigest, verified: false }] },
  { ...publishedExport(), files: publishedExport().files.map((file, index) => index === 1 ? { ...file, verified: false } : file) },
  { ...publishedExport(), files: publishedExport().files.map((file, index) => index === 1 ? { ...file, contentDigest: 'e'.repeat(64) } : file) },
  { ...publishedExport(), files: publishedExport().files.map((file) => ({ ...file, format: 'pptx' })) },
  { ...publishedExport(), files: publishedExport().files.map((file) => ({ ...file, fileDigest: 'zz' })) },
  { ...publishedExport(), createdAt: 'just now' },
  { ...publishedExport(), revokedAt: exportTs },
  { ...revokedExport(), revokedAt: undefined },
  { ...revokedExport(), status: 'submittable', submittable: true },
  { ...revokedExport(), failureCode: 'ghost_code' },
 ]
 for (const payload of receiptInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.materialExports('m-1'), TypeError)
 }
 const grantInventors: Array<Record<string, unknown>> = [
  { ...exportGrant(), format: 'exe' },
  { ...exportGrant(), digest: 'a'.repeat(63) },
  { ...exportGrant(), size: -1 },
  { ...exportGrant(), expiresAt: 0 },
  { ...exportGrant(), signature: ' ' },
  { ...exportGrant(), url: `https://evil.test/api/v1/career/materials/mat%20%2F1/exports/exp%20%2F1/download?format=pdf&expires=1790000000&signature=${'c0ffee'.repeat(4)}` },
 ]
 for (const payload of grantInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.materialExportSignedURL('m-1', 'e-1', 'pdf', 600), TypeError)
 }
 await assert.rejects(createCareerApi(async () => ({ materialId: 'm-1', exports: {} })).materialExports('m-1'), TypeError)
})

// T13: recurring search rule client — POST /rules (create/update), GET
// /rules/receipt by request ID, GET /rules/:ruleId (live view with run
// history and discovery todos) — with the frozen rule/run/todo enums decoded
// strictly; the enable-time estimate is displayed verbatim (backend basis
// text included) and never recomputed client-side.
const ruleEstimate = { triggersPerDay: 1, sourcesPerTrigger: 0, estimatedSearchesPerDay: 0, basis: 'deterministic projection: triggers_per_day = 1440 / interval_minutes; estimated_searches_per_day = triggers_per_day × vetted sources per trigger; this is an estimate from rule parameters, not a quota balance' }
const setRuleReceipt = { kind: 'rule_set', requestId: 'rule /1', ruleId: 'rule-1', query: '上海 前端 实习', intervalMinutes: 1440, status: 'disabled', revision: 1, estimate: ruleEstimate }

test('rule client encodes create, receipt replay and detail routes with strict decoding', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return setRuleReceipt
 })
 const input = { requestId: 'rule /1', query: '上海 前端 实习', intervalMinutes: 1440, status: 'disabled', expectedRevision: 0 }
 assert.deepEqual(await api.setRule(input), setRuleReceipt)
 assert.deepEqual(await api.ruleReceipt('rule /1'), setRuleReceipt)
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/rules', body: { requestId: 'rule /1', query: '上海 前端 实习', intervalMinutes: 1440, status: 'disabled', expectedRevision: 0 } },
  { method: 'GET', path: '/api/v1/career/rules/receipt?requestId=rule%20%2F1' },
 ])
 const update = { requestId: 'rule-2', ruleId: 'rule-1', query: '杭州 后端', intervalMinutes: 60, status: 'enabled', expectedRevision: 0 }
 const updated = { ...setRuleReceipt, requestId: 'rule-2', query: '杭州 后端', intervalMinutes: 60, status: 'enabled', revision: 2, nextDueAt: '2026-09-26T08:00:00Z' }
 const updating = createCareerApi(async () => updated)
 assert.deepEqual(await updating.setRule(update), updated)
 assert.equal(updated.nextDueAt, '2026-09-26T08:00:00Z')
})

test('rule view decode keeps blocked run statuses, todos and the estimate basis verbatim', async () => {
 const view = {
  ruleId: 'rule-1', query: '上海 前端 实习', intervalMinutes: 1440, status: 'paused', revision: 3, lastPeriod: 2,
  estimate: ruleEstimate,
  runs: [
   { kind: 'rule_run', ruleId: 'rule-1', period: 1, requestId: 'rule:rule-1:1', status: 'no_vetted_sources', note: 'no vetted search source is configured; the trigger was not searched and nothing was fabricated', triggeredAt: '2026-09-25T08:00:00Z' },
   { kind: 'rule_run', ruleId: 'rule-1', period: 2, requestId: 'rule:rule-1:2', status: 'blocked_no_quota', note: 'the search quota gate refused admission for this trigger; no search was consumed and nothing was fabricated', triggeredAt: '2026-09-25T22:00:00Z' },
  ],
  todos: [
   { todoId: 'todo-1', ruleId: 'rule-1', runId: 'run-1', searchId: 'search-1', link: 'https://jobs.example.test/1', status: 'open', createdAt: '2026-09-25T08:00:01Z' },
   { todoId: 'todo-2', ruleId: 'rule-1', runId: 'run-2', searchId: 'search-2', sourceId: 'board-1', link: 'https://jobs.example.test/2', status: 'open', createdAt: '2026-09-25T22:00:01Z' },
  ],
  createdAt: '2026-09-24T08:00:00Z', updatedAt: '2026-09-25T22:00:00Z',
 }
 const api = createCareerApi(async () => view)
 const decoded = await api.getRule('rule-1')
 assert.equal(decoded.status, 'paused')
 assert.equal(decoded.nextDueAt, undefined)
 assert.equal(decoded.runs.length, 2)
 assert.equal(decoded.runs[0]?.status, 'no_vetted_sources')
 assert.match(decoded.runs[0]?.note ?? '', /no vetted search source/)
 assert.equal(decoded.runs[1]?.status, 'blocked_no_quota')
 assert.equal(decoded.todos.length, 2)
 assert.equal(decoded.todos[1]?.sourceId, 'board-1')
 assert.equal(decoded.estimate.basis, ruleEstimate.basis)
})

test('rule client refuses blank identifiers, out-of-range intervals and invented enum values', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 const base = { requestId: 'rule-1', query: '前端', intervalMinutes: 60, status: 'enabled', expectedRevision: 2 }
 await assert.rejects(refusing.setRule({ ...base, requestId: ' ' }), /requestId/)
 await assert.rejects(refusing.setRule({ ...base, query: ' ' }), /query/)
 await assert.rejects(refusing.setRule({ ...base, status: 'running' }), /status/)
 await assert.rejects(refusing.setRule({ ...base, intervalMinutes: 0 }), /interval/)
 await assert.rejects(refusing.setRule({ ...base, intervalMinutes: 43201 }), /interval/)
 await assert.rejects(refusing.setRule({ ...base, intervalMinutes: 1.5 }), /interval/)
 await assert.rejects(refusing.setRule({ ...base, expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.ruleReceipt(' '), /requestId/)
 await assert.rejects(refusing.getRule(' '), /rule ID/)
 const inventors: Array<Record<string, unknown>> = [
  { ...setRuleReceipt, kind: 'rule_run' },
  { ...setRuleReceipt, status: 'archived' },
  { ...setRuleReceipt, revision: 0 },
  { ...setRuleReceipt, estimate: { ...ruleEstimate, basis: '' } },
  { ...setRuleReceipt, estimate: { ...ruleEstimate, triggersPerDay: 'daily' } },
  { ...setRuleReceipt, nextDueAt: 'tomorrow' },
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.ruleReceipt('rule /1'), TypeError)
 }
 const runTs = '2026-09-25T08:00:00Z'
 const viewBase = { ruleId: 'rule-1', query: '找岗', intervalMinutes: 60, status: 'enabled', revision: 1, lastPeriod: 0, nextDueAt: '2026-09-26T08:00:00Z', estimate: ruleEstimate, runs: [], todos: [], createdAt: runTs, updatedAt: runTs }
 const viewInventors: Array<Record<string, unknown>> = [
  { ...viewBase, status: 'running' },
  { ...viewBase, runs: [{ kind: 'rule_run', ruleId: 'rule-1', period: 0, requestId: 'r', status: 'completed', triggeredAt: runTs }] },
  { ...viewBase, runs: [{ kind: 'rule_set', ruleId: 'rule-1', period: 1, requestId: 'r', status: 'completed', triggeredAt: runTs }] },
  { ...viewBase, runs: [{ kind: 'rule_run', ruleId: 'rule-1', period: 1, requestId: 'r', status: 'silently_skipped', triggeredAt: runTs }] },
  { ...viewBase, todos: [{ todoId: 'todo-1', ruleId: 'rule-1', runId: 'run-1', searchId: 'search-1', link: 'javascript:alert(1)', status: 'open', createdAt: runTs }] },
  { ...viewBase, todos: [{ todoId: 'todo-1', ruleId: 'rule-1', runId: 'run-1', searchId: 'search-1', link: 'https://jobs.example.test/1', status: 'closed', createdAt: runTs }] },
  { ...viewBase, intervalMinutes: 0 },
 ]
 for (const payload of viewInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.getRule('rule-1'), TypeError)
 }
})

// T18 user-confirmed submissions: record/list/receipt follow the frozen
// backend enums (internal/modules/career/submission.go). One application holds
// at most one submission; the version reference is either an exact submittable
// export binding or the explicit unknown marker — the client never fabricates
// a binding and never lets an invented one reach the UI.
const submissionTs = '2026-09-25T09:15:00Z'
const submissionBinding = { materialId: 'mat /1', exportId: 'exp-1', version: 3, contentDigest: 'b'.repeat(64) }
const submissionReceipt = (extra: Record<string, unknown> = {}) => ({ kind: 'submission_recorded', requestId: 'sub-req /1', applicationId: 'app /1', submissionId: 'sub-1', channel: 'web', occurredAt: submissionTs, versionConfirmed: true, boundVersion: { ...submissionBinding }, note: '官网已投', confirmer: 'owner-1', revision: 2, createdAt: submissionTs, ...extra })
const unknownVersionReceipt = () => submissionReceipt({ versionConfirmed: false, boundVersion: undefined, note: undefined })

test('submission client encodes record, list and receipt recovery paths', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.method === 'POST' ? submissionReceipt() : input.path.includes('/receipt?') ? submissionReceipt() : { submissions: [submissionReceipt()] }
 })
 assert.deepEqual(await api.recordSubmission({ requestId: 'sub-req /1', applicationId: 'app /1', channel: 'web', occurredAt: submissionTs, materialId: 'mat /1', exportId: 'exp-1', versionUnknown: false, note: '官网已投', expectedRevision: 2 }), submissionReceipt())
 assert.deepEqual(await api.recordSubmission({ requestId: 'sub-req-2', applicationId: 'app /1', channel: 'email', versionUnknown: true, expectedRevision: 2 }), submissionReceipt())
 assert.deepEqual(await api.applicationSubmissions('app /1'), { submissions: [submissionReceipt()] })
 assert.deepEqual(await api.submissionReceipt('sub-req /1'), submissionReceipt())
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/submissions', body: { requestId: 'sub-req /1', applicationId: 'app /1', channel: 'web', occurredAt: submissionTs, materialId: 'mat /1', exportId: 'exp-1', versionUnknown: false, note: '官网已投', expectedRevision: 2 } },
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/submissions', body: { requestId: 'sub-req-2', applicationId: 'app /1', channel: 'email', versionUnknown: true, expectedRevision: 2 } },
  { method: 'GET', path: '/api/v1/career/applications/app%20%2F1/submissions' },
  { method: 'GET', path: '/api/v1/career/submissions/receipt?requestId=sub-req%20%2F1' },
 ])
})

test('submission client refuses blank identifiers, mismatched version markers and invented enums', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.recordSubmission({ requestId: ' ', applicationId: 'app-1', channel: 'web', versionUnknown: true, expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: ' ', channel: 'web', versionUnknown: true, expectedRevision: 0 }), /applicationId/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'wechat', versionUnknown: true, expectedRevision: 0 }), /channel/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', occurredAt: 'just now', versionUnknown: true, expectedRevision: 0 }), /occurredAt/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', versionUnknown: true, expectedRevision: -1 }), /revision/)
 // The explicit unknown marker is exclusive; a confirmed binding needs both IDs.
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', materialId: 'mat-1', versionUnknown: true, expectedRevision: 0 }), /versionUnknown/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', exportId: 'exp-1', versionUnknown: true, expectedRevision: 0 }), /versionUnknown/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', versionUnknown: false, expectedRevision: 0 }), /materialId/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', materialId: 'mat-1', versionUnknown: false, expectedRevision: 0 }), /materialId/)
 await assert.rejects(refusing.recordSubmission({ requestId: 's-1', applicationId: 'app-1', channel: 'web', versionUnknown: true, note: 'x'.repeat(4097), expectedRevision: 0 }), /note/)
 await assert.rejects(refusing.applicationSubmissions(' '), /applicationId/)
 await assert.rejects(refusing.submissionReceipt(' '), /requestId/)
 const receiptInventors: Array<Record<string, unknown>> = [
  { ...submissionReceipt(), kind: 'submission_deleted' },
  { ...submissionReceipt(), channel: 'carrier_pigeon' },
  { ...submissionReceipt(), occurredAt: 'yesterday' },
  { ...submissionReceipt(), createdAt: 'soon' },
  { ...submissionReceipt(), submissionId: ' ' },
  { ...submissionReceipt(), confirmer: ' ' },
  { ...submissionReceipt(), revision: -1 },
  { ...submissionReceipt(), versionConfirmed: false },
  { ...submissionReceipt(), boundVersion: undefined },
  { ...submissionReceipt(), versionConfirmed: false, boundVersion: { ...submissionBinding } },
  { ...submissionReceipt(), boundVersion: { ...submissionBinding, version: 0 } },
  { ...submissionReceipt(), boundVersion: { ...submissionBinding, contentDigest: 'not-a-digest' } },
  { ...submissionReceipt(), boundVersion: { ...submissionBinding, materialId: ' ' } },
  { ...unknownVersionReceipt(), versionConfirmed: true },
 ]
 for (const payload of receiptInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.submissionReceipt('sub-req /1'), TypeError)
 }
 // The frozen backend guard is one submission per application: a list
 // claiming two rows for one application is an invented payload.
 const listInventors: Array<Record<string, unknown>> = [
  { submissions: 'none' },
  { submissions: [submissionReceipt(), submissionReceipt({ submissionId: 'sub-2' })] },
  { applications: [submissionReceipt()] },
 ]
 for (const payload of listInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.applicationSubmissions('app /1'), TypeError)
 }
 // An explicit unknown receipt decodes with no binding at all.
 const unknownApi = createCareerApi(async () => unknownVersionReceipt())
 const decoded = await unknownApi.submissionReceipt('sub-req /1')
 assert.equal(decoded.versionConfirmed, false)
 assert.equal('boundVersion' in decoded, false)
})

// T22 whole-space lifecycle (internal/modules/career/career_export.go): the
// export package travels inline with a sha256 digest, and the deletion
// receipt is only "deleted" when every step is done — a partial failure keeps
// a recoverable state and never claims complete deletion. The boundary view
// explains in-space deletions vs non-revocable external platform data and
// the disclosed retention rows before any deletion executes.
const lifecycleTs = '2026-09-25T10:30:00Z'
const exportedFact = (key: string) => ({ key, value: `${key}值`, revision: 2, source: { kind: 'user', label: '本人确认' }, confirmation: { userId: 'owner-1', confirmedAt: lifecycleTs }, confirmedAt: lifecycleTs })
const exportedProgressEvent = { eventId: 'evt-1', applicationId: 'app-1', seq: 1, eventType: 'submitted', note: '官网已投', occurredAt: lifecycleTs, source: { kind: 'user' }, confirmer: 'owner-1' }
const exportReceipt = (extra: Record<string, unknown> = {}) => ({
 kind: 'career_exported', requestId: 'exp-req /1', exportId: 'exp-1', revision: 4, status: 'complete', digest: 'a'.repeat(64), createdAt: lifecycleTs,
 archive: {
  profile: { revision: 4, facts: [exportedFact('学历')], proposals: [] },
  factHistory: [exportedFact('学历'), exportedFact('毕业时间')],
  opportunities: [{ opportunityId: 'opp /1', snapshots: [{ snapshotId: 'snap-1', status: 'needs_review', rawText: 'JD 原文', acquiredAt: lifecycleTs }] }],
  applications: [{ applicationId: 'app-1', opportunityId: 'opp /1', snapshotId: 'snap-1', batchIdentity: 'batch-1', taskId: 'task-9', progressEvents: [exportedProgressEvent] }],
  materials: [{ materialId: 'mat-1', opportunityId: 'opp /1', status: 'confirmed', versions: [{ version: 3, requestId: 'confirm-1', versionBody: '正文', createdAt: lifecycleTs }] }],
  submissions: [{ submissionId: 'sub-1', applicationId: 'app-1', channel: 'web', occurredAt: lifecycleTs, versionConfirmed: true, materialId: 'mat-1', exportId: 'exp-1', version: 3, contentDigest: 'b'.repeat(64), note: '官网已投', confirmer: 'owner-1', createdAt: lifecycleTs }],
 }, ...extra,
})
const boundaryView = () => ({
 inSpace: [
  { section: 'profile', description: '已确认的档案事实与待处理提案', count: 2 },
  { section: 'materials', description: '材料草稿', count: 1 },
 ],
 external: [{ item: 'external_platform_submissions', description: '你在外部招聘平台完成的投递、沟通与账号操作不在本空间控制范围内，本系统无法撤回或修改。', revocable: false }],
 retention: [{ holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'retained' }],
})
const deletionSteps = () => ([
 { name: 'revoke_material_exports', status: 'done' },
 { name: 'purge_career_data', status: 'done' },
 { name: 'remove_workbench_tasks', status: 'done' },
 { name: 'finalize', status: 'done' },
])
const deletionReceipt = (extra: Record<string, unknown> = {}) => ({
 kind: 'career_deleted', requestId: 'del-req /1', status: 'deleted', steps: deletionSteps(),
 retention: [{ holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'retained' }],
 revision: 5, startedAt: lifecycleTs, completedAt: lifecycleTs, ...extra,
})

test('career lifecycle client encodes export, boundary, deletion and both receipt recoveries', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  if (input.path === '/api/v1/career/exports' || input.path.includes('/exports/receipt?')) return exportReceipt()
  if (input.path === '/api/v1/career/deletions/boundary') return boundaryView()
  return deletionReceipt()
 })
 assert.deepEqual(await api.exportCareer({ requestId: 'exp-req /1', expectedRevision: 4 }), exportReceipt())
 assert.deepEqual(await api.careerExportReceipt('exp-req /1'), exportReceipt())
 assert.deepEqual(await api.careerDeletionBoundary(), boundaryView())
 assert.deepEqual(await api.deleteCareer({ requestId: 'del-req /1', expectedRevision: 4 }), deletionReceipt())
 assert.deepEqual(await api.careerDeletionReceipt('del-req /1'), deletionReceipt())
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/exports', body: { requestId: 'exp-req /1', expectedRevision: 4 } },
  { method: 'GET', path: '/api/v1/career/exports/receipt?requestId=exp-req%20%2F1' },
  { method: 'GET', path: '/api/v1/career/deletions/boundary' },
  { method: 'POST', path: '/api/v1/career/deletions', body: { requestId: 'del-req /1', expectedRevision: 4 } },
  { method: 'GET', path: '/api/v1/career/deletions/receipt?requestId=del-req%20%2F1' },
 ])
 // The inline archive keeps its frozen sections: profile, original job
 // snapshots, application progress events, material versions and
 // submission records.
 const decoded = await api.careerExportReceipt('exp-req /1')
 assert.equal(decoded.archive.profile.facts.length, 1)
 assert.equal(decoded.archive.factHistory.length, 2)
 assert.equal(decoded.archive.opportunities[0]?.snapshots.length, 1)
 assert.equal(decoded.archive.applications[0]?.progressEvents.length, 1)
 assert.equal(decoded.archive.materials[0]?.versions.length, 1)
 assert.equal(decoded.archive.submissions.length, 1)
 assert.equal(decoded.archive.applications[0]?.taskId, 'task-9')
})

test('career lifecycle client refuses blank identifiers, bad revisions and invented export payloads', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.exportCareer({ requestId: ' ', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.exportCareer({ requestId: 'e-1', expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.careerExportReceipt(' '), /requestId/)
 await assert.rejects(refusing.deleteCareer({ requestId: ' ', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.deleteCareer({ requestId: 'd-1', expectedRevision: 1.5 }), /revision/)
 await assert.rejects(refusing.careerDeletionReceipt(' '), /requestId/)
 const exported = exportReceipt()
 const exportInventors: Array<Record<string, unknown>> = [
  { ...exported, kind: 'career_deleting' },
  { ...exported, status: 'preparing' },
  { ...exported, digest: 'not-a-digest' },
  { ...exported, exportId: ' ' },
  { ...exported, requestId: '' },
  { ...exported, revision: -1 },
  { ...exported, createdAt: 'soon' },
  { ...exported, archive: 'missing' },
  { ...exported, archive: { ...exported.archive, opportunities: 'none' } },
  { ...exported, archive: { ...exported.archive, opportunities: [{ opportunityId: ' ', snapshots: [] }] } },
  { ...exported, archive: { ...exported.archive, opportunities: [{ opportunityId: 'opp /1', snapshots: [{ snapshotId: 'snap-1', status: 'needs_review', rawText: 'JD', acquiredAt: 'whenever' }] }] } },
  { ...exported, archive: { ...exported.archive, applications: [{ ...exported.archive.applications[0], progressEvents: [{ ...exportedProgressEvent, seq: 0 }] }] } },
  { ...exported, archive: { ...exported.archive, applications: [{ ...exported.archive.applications[0], progressEvents: [{ ...exportedProgressEvent, eventId: ' ' }] }] } },
  { ...exported, archive: { ...exported.archive, materials: [{ ...exported.archive.materials[0], versions: [{ version: 0, versionBody: '正文', createdAt: lifecycleTs }] }] } },
  { ...exported, archive: { ...exported.archive, materials: [{ ...exported.archive.materials[0], materialId: ' ' }] } },
  { ...exported, archive: { ...exported.archive, submissions: [{ ...exported.archive.submissions[0], contentDigest: 'not-a-digest' }] } },
  { ...exported, archive: { ...exported.archive, profile: { ...exported.archive.profile, facts: [{ ...exportedFact('学历'), confirmedAt: 'nope' }] } } },
  { ...exported, archive: { ...exported.archive, profile: { ...exported.archive.profile, facts: [{ ...exportedFact('学历'), source: { kind: '' } }] } } },
  { ...exported, archive: { ...exported.archive, factHistory: [{ ...exportedFact('学历'), confirmation: { userId: ' ', confirmedAt: lifecycleTs } }] } },
 ]
 for (const payload of exportInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.careerExportReceipt('exp-req /1'), TypeError)
 }
})

test('deletion decoders keep the truthful status contract: deleted means every step done, partial keeps a failed step', async () => {
 // A receipt claiming "deleted" while a step failed — or "partial" without
 // any failed step — is an invented payload and never reaches the UI.
 const deletionInventors: Array<Record<string, unknown>> = [
  { ...deletionReceipt(), kind: 'career_export' },
  { ...deletionReceipt(), status: 'mostly_deleted' },
  { ...deletionReceipt(), status: 'deleted', steps: deletionSteps().map((step, index) => index === 1 ? { ...step, status: 'failed' } : step) },
  { ...deletionReceipt(), status: 'deleted', steps: deletionSteps().map((step, index) => index === 3 ? { ...step, status: 'pending' } : step) },
  { ...deletionReceipt(), status: 'deleted', completedAt: undefined },
  { ...deletionReceipt(), status: 'partial' },
  { ...deletionReceipt(), status: 'partial', steps: deletionSteps() },
  { ...deletionReceipt(), status: 'partial', completedAt: lifecycleTs },
  { ...deletionReceipt(), steps: deletionSteps().map((step) => ({ ...step, name: 'format_disk' })) },
  { ...deletionReceipt(), startedAt: 'whenever' },
  { ...deletionReceipt(), revision: -1 },
  { ...deletionReceipt(), retention: [{ holder: 'career_data_deletions', reason: '删除审计与可恢复状态（法定/技术保留）', status: 'purged' }] },
  { ...deletionReceipt(), steps: deletionSteps(), status: 'deleting', completedAt: lifecycleTs },
 ]
 for (const payload of deletionInventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.careerDeletionReceipt('del-req /1'), TypeError)
 }
 // A truthful partial receipt decodes with its failed step and detail.
 const partial = deletionReceipt({ status: 'partial', completedAt: undefined, steps: deletionSteps().map((step, index) => index === 2 ? { ...step, status: 'failed', detail: 'workbench remover unavailable' } : step) })
 const partialApi = createCareerApi(async () => partial)
 const decodedPartial = await partialApi.careerDeletionReceipt('del-req /1')
 assert.equal(decodedPartial.status, 'partial')
 assert.equal(decodedPartial.steps[2]?.detail, 'workbench remover unavailable')
 assert.equal('completedAt' in decodedPartial, false)
})

test('deletion boundary decoder presents in-space sections, non-revocable external data and retained rows', async () => {
 const inventors: Array<Record<string, unknown>> = [
  { ...boundaryView(), inSpace: 'none' },
  { ...boundaryView(), inSpace: [{ section: 'profile', description: '已确认的档案事实与待处理提案', count: -1 }] },
  { ...boundaryView(), inSpace: [{ section: ' ', description: '已确认的档案事实与待处理提案', count: 1 }] },
  { ...boundaryView(), external: [{ item: 'external_platform_submissions', description: '描述', revocable: 'no' }] },
  { ...boundaryView(), external: [{ item: ' ', description: '描述', revocable: false }] },
  { ...boundaryView(), retention: [{ holder: 'career_data_deletions', reason: '删除审计', status: 'maybe' }] },
  { inSpace: [] },
  { external: [] },
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.careerDeletionBoundary(), TypeError)
 }
 const api = createCareerApi(async () => boundaryView())
 const decoded = await api.careerDeletionBoundary()
 assert.equal(decoded.external[0]?.revocable, false)
 assert.equal(decoded.retention[0]?.status, 'retained')
 assert.equal(decoded.inSpace[0]?.count, 2)
})

// T19 sourced preparations: generate/list/receipt follow the frozen backend
// enums (internal/modules/career/preparation.go). The draft anchors to the
// actually submitted version and cites the frozen snapshot plus confirmed
// fact keys; a failed or in-flight generation answers its typed failure
// state with an empty body — never a blank success product. Decoders reject
// invented focuses, statuses or failure codes before they reach the UI.
const preparationTs = '2026-09-26T07:30:00Z'
const preparationAnchor = { submissionId: 'sub-1', materialId: 'mat /1', exportId: 'exp-1', version: 2, contentDigest: 'c'.repeat(64) }
const preparationBody = { sections: [{ heading: '面试准备（草稿）：教育经历', content: '以下要点固定自实际投递版本与岗位快照。', claims: [{ claimId: 'claim-1', text: '本科在读', factKey: '学历', needsReview: false }] }] }
const preparationReceipt = (extra: Record<string, unknown> = {}) => ({
 kind: 'preparation_generated', requestId: 'prep-req /1', applicationId: 'app /1', preparationId: 'prep-1', focus: 'interview_prep', status: 'draft',
 anchor: { ...preparationAnchor }, materialId: 'mat-draft-1', body: preparationBody, reviewRisks: [],
 sources: { submittedVersion: { ...preparationAnchor }, snapshot: { opportunityId: 'opp /1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) }, factKeys: ['学历'], profileRevision: 5 },
 revision: 5, createdAt: preparationTs, ...extra,
})
const failedPreparation = (extra: Record<string, unknown> = {}) => preparationReceipt({
 status: 'failed', materialId: undefined, body: { sections: null }, reviewRisks: [],
 sources: { submittedVersion: { ...preparationAnchor }, snapshot: { opportunityId: '', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) }, factKeys: null, profileRevision: 5 },
 failureCode: 'generation_failed', failureMessage: 'model unavailable', ...extra,
})

test('preparation client encodes generate, list and receipt recovery paths', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  return input.method === 'POST' ? preparationReceipt() : input.path.includes('/receipt?') ? preparationReceipt() : { preparations: [preparationReceipt()] }
 })
 assert.deepEqual(await api.generatePreparation({ requestId: 'prep-req /1', applicationId: 'app /1', focus: 'interview_prep', expectedRevision: 5 }), preparationReceipt())
 assert.deepEqual(await api.generatePreparation({ requestId: 'prep-req-2', applicationId: 'app /1', focus: 'cover_letter', expectedRevision: 5 }), preparationReceipt())
 assert.deepEqual(await api.applicationPreparations('app /1'), { preparations: [preparationReceipt()] })
 assert.deepEqual(await api.preparationReceipt('prep-req /1'), preparationReceipt())
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/preparations', body: { requestId: 'prep-req /1', applicationId: 'app /1', focus: 'interview_prep', expectedRevision: 5 } },
  { method: 'POST', path: '/api/v1/career/applications/app%20%2F1/preparations', body: { requestId: 'prep-req-2', applicationId: 'app /1', focus: 'cover_letter', expectedRevision: 5 } },
  { method: 'GET', path: '/api/v1/career/applications/app%20%2F1/preparations' },
  { method: 'GET', path: '/api/v1/career/preparations/receipt?requestId=prep-req%20%2F1' },
 ])
})

test('preparation client refuses blank identifiers, invented focuses and negative revisions', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.generatePreparation({ requestId: ' ', applicationId: 'app-1', focus: 'interview_prep', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.generatePreparation({ requestId: 'p-1', applicationId: ' ', focus: 'interview_prep', expectedRevision: 0 }), /applicationId/)
 await assert.rejects(refusing.generatePreparation({ requestId: 'p-1', applicationId: 'app-1', focus: 'thank_you_note', expectedRevision: 0 }), /focus/)
 await assert.rejects(refusing.generatePreparation({ requestId: 'p-1', applicationId: 'app-1', focus: 'interview_prep', expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.applicationPreparations(' '), /applicationId/)
 await assert.rejects(refusing.preparationReceipt(' '), /requestId/)
})

test('preparation decoder rejects blank products that disagree with their status', async () => {
 const inventors: Array<Record<string, unknown>> = [
  preparationReceipt({ kind: 'material_edited' }),
  preparationReceipt({ focus: 'thank_you_note' }),
  preparationReceipt({ status: 'published' }),
  // A draft without its materialized draft, an empty body, or a failure code
  // is a blank success product — rejected before the UI can show it.
  { ...preparationReceipt(), materialId: undefined },
  preparationReceipt({ body: { sections: [] } }),
  preparationReceipt({ body: { sections: null } }),
  preparationReceipt({ failureCode: 'generation_failed', failureMessage: 'late failure' }),
  preparationReceipt({ anchor: { ...preparationAnchor, version: 0 } }),
  preparationReceipt({ anchor: { ...preparationAnchor, contentDigest: 'not-a-digest' } }),
  preparationReceipt({ sources: { submittedVersion: { ...preparationAnchor }, snapshot: { opportunityId: '', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) }, factKeys: ['学历'], profileRevision: 5 } }),
  preparationReceipt({ sources: { submittedVersion: { ...preparationAnchor }, snapshot: { opportunityId: 'opp /1', snapshotId: 'snap-1', snapshotSha256: 'zz' }, factKeys: ['学历'], profileRevision: 5 } }),
  preparationReceipt({ sources: { submittedVersion: { ...preparationAnchor }, snapshot: { opportunityId: 'opp /1', snapshotId: 'snap-1', snapshotSha256: 'a'.repeat(64) }, factKeys: [' '], profileRevision: 5 } }),
  preparationReceipt({ revision: -1 }),
  preparationReceipt({ createdAt: 'yesterday' }),
  // A failure state must stay typed: no failure code, an invented one, a
  // materialized draft, a carried body, or review risks are all refused.
  failedPreparation({ failureCode: undefined, failureMessage: undefined }),
  failedPreparation({ failureCode: 'model_sleepy' }),
  failedPreparation({ materialId: 'mat-draft-1' }),
  failedPreparation({ body: preparationBody }),
  failedPreparation({ reviewRisks: [{ code: 'needs_review', message: '缺实习经历' }] }),
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.generatePreparation({ requestId: 'prep-req /1', applicationId: 'app /1', focus: 'interview_prep', expectedRevision: 5 }), TypeError)
 }
})

test('preparation decoder keeps the typed failure state decodable with its empty body and sources', async () => {
 const api = createCareerApi(async () => failedPreparation())
 const decoded = await api.preparationReceipt('prep-req /1')
 assert.equal(decoded.status, 'failed')
 assert.equal(decoded.failureCode, 'generation_failed')
 assert.equal(decoded.failureMessage, 'model unavailable')
 assert.deepEqual(decoded.body, { sections: [] })
 assert.equal('materialId' in decoded, false)
 assert.equal(decoded.anchor.version, 2)
 assert.deepEqual(decoded.sources.factKeys, [])
 // An interrupted generation row answers the in-flight state with no failure
 // code — the request stays recoverable under the same id.
 const inFlight = createCareerApi(async () => failedPreparation({ status: 'generating', failureCode: undefined, failureMessage: undefined }))
 const decodedInFlight = await inFlight.preparationReceipt('prep-req /1')
 assert.equal(decodedInFlight.status, 'generating')
 assert.equal('failureCode' in decodedInFlight, false)
 assert.deepEqual(decodedInFlight.body, { sections: [] })
})

// T20 in-station reminders: the frozen privacy contract owned by the career
// backend (internal/modules/career/reminder.go). One source event holds
// exactly one todo; the push body is drawn exclusively from the frozen
// template table (company, job and interview detail never leave the space),
// and the push report is response-only — a delivery failure never fails the
// durable write and is never a todo state change. Decoders reject invented
// notice keys, interpolated notice bodies, wrong kinds or push reports that
// disagree with their own flags before they reach the UI.
const reminderTs = '2026-09-26T09:15:00Z'
const reminderReceipt = (extra: Record<string, unknown> = {}) => ({
 kind: 'reminder_set', requestId: 'rem-req /1', reminderId: 'rem-1', sourceKind: 'progress_event', sourceId: 'evt /1',
 applicationId: 'app /1', opportunityId: 'opp /1', noticeKey: 'progress_updated', notice: '你有新的求职进展，请登录查看。',
 deduplicated: false, status: 'open', revision: 6, createdAt: reminderTs, ...extra,
})
const reminderView = (extra: Record<string, unknown> = {}) => ({
 reminderId: 'rem-1', sourceKind: 'progress_event', sourceId: 'evt /1', applicationId: 'app /1', opportunityId: 'opp /1',
 noticeKey: 'progress_updated', notice: '你有新的求职进展，请登录查看。', status: 'open', createdAt: reminderTs, ...extra,
})

test('reminder client encodes set, list and receipt recovery paths', async () => {
 const calls: Array<{ method: string; path: string; body?: unknown }> = []
 const api = createCareerApi(async (input) => {
  calls.push({ method: input.method, path: input.path, ...(input.body !== undefined ? { body: input.body } : {}) })
  if (input.method === 'POST') {
   const body = input.body as { sourceKind: string; sourceId: string }
   return body.sourceKind === 'discovery' ? reminderReceipt({ sourceKind: 'discovery', sourceId: 'todo /1', applicationId: undefined, opportunityId: undefined, noticeKey: 'discovery_found', notice: '持续找岗有新发现，请登录查看。' }) : reminderReceipt()
  }
  return input.path.includes('/receipt?') ? reminderReceipt() : { reminders: [reminderView()] }
 })
 assert.deepEqual(await api.setReminder({ requestId: 'rem-req /1', sourceKind: 'progress_event', sourceId: 'evt /1', expectedRevision: 6 }), reminderReceipt())
 assert.deepEqual(await api.setReminder({ requestId: 'rem-req-2', sourceKind: 'discovery', sourceId: 'todo /1', expectedRevision: 6 }), { kind: 'reminder_set', requestId: 'rem-req /1', reminderId: 'rem-1', sourceKind: 'discovery', sourceId: 'todo /1', deduplicated: false, noticeKey: 'discovery_found', notice: '持续找岗有新发现，请登录查看。', status: 'open', revision: 6, createdAt: reminderTs })
 assert.deepEqual(await api.reminders(), { reminders: [reminderView()] })
 assert.deepEqual(await api.reminderReceipt('rem-req /1'), reminderReceipt())
 assert.deepEqual(calls, [
  { method: 'POST', path: '/api/v1/career/reminders', body: { requestId: 'rem-req /1', sourceKind: 'progress_event', sourceId: 'evt /1', expectedRevision: 6 } },
  { method: 'POST', path: '/api/v1/career/reminders', body: { requestId: 'rem-req-2', sourceKind: 'discovery', sourceId: 'todo /1', expectedRevision: 6 } },
  { method: 'GET', path: '/api/v1/career/reminders' },
  { method: 'GET', path: '/api/v1/career/reminders/receipt?requestId=rem-req%20%2F1' },
 ])
})

test('reminder client refuses blank identifiers, invented source kinds and negative revisions', async () => {
 const refusing = createCareerApi(async () => { throw new Error('must not send invalid request') })
 await assert.rejects(refusing.setReminder({ requestId: ' ', sourceKind: 'progress_event', sourceId: 'evt-1', expectedRevision: 0 }), /requestId/)
 await assert.rejects(refusing.setReminder({ requestId: 'r-1', sourceKind: 'progress_event', sourceId: ' ', expectedRevision: 0 }), /sourceId/)
 await assert.rejects(refusing.setReminder({ requestId: 'r-1', sourceKind: 'interview_round', sourceId: 'evt-1', expectedRevision: 0 }), /sourceKind/)
 await assert.rejects(refusing.setReminder({ requestId: 'r-1', sourceKind: 'progress_event', sourceId: 'evt-1', expectedRevision: -1 }), /revision/)
 await assert.rejects(refusing.reminderReceipt(' '), /requestId/)
})

test('reminder decoder keeps only the frozen privacy notice bodies and the closed source vocabulary', async () => {
 const inventors = [
  reminderReceipt({ notice: '字节跳动已为你安排一面，请查看。' }),
  reminderReceipt({ noticeKey: 'interview_scheduled' }),
  reminderReceipt({ noticeKey: 'discovery_found', notice: '你有新的求职进展，请登录查看。' }),
  reminderReceipt({ kind: 'reminder_updated' }),
  reminderReceipt({ status: 'notified' }),
  reminderReceipt({ sourceKind: 'progress_event', applicationId: undefined }),
  reminderView({ notice: '高级后端工程师岗位有更新。' }),
  reminderView({ noticeKey: 'interview_scheduled' }),
  { reminders: [reminderView({ notice: undefined })] },
 ]
 for (const payload of inventors) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.setReminder({ requestId: 'rem-req /1', sourceKind: 'progress_event', sourceId: 'evt /1', expectedRevision: 6 }), TypeError)
 }
})

test('reminder decoder keeps the response-only push report and its frozen reasons', async () => {
 // A delivery failure (or an unsubscribed skip) never fails the write: the
 // receipt stays decodable and the reason comes from the frozen set.
 const delivered = createCareerApi(async () => reminderReceipt({ push: { attempted: true, delivered: true } }))
 assert.deepEqual((await delivered.setReminder({ requestId: 'rem-req /1', sourceKind: 'progress_event', sourceId: 'evt /1', expectedRevision: 6 })).push, { attempted: true, delivered: true })
 const failed = createCareerApi(async () => reminderReceipt({ push: { attempted: true, delivered: false, reason: 'delivery_failed' } }))
 assert.deepEqual((await failed.reminderReceipt('rem-req /1')).push, { attempted: true, delivered: false, reason: 'delivery_failed' })
 const skipped = createCareerApi(async () => reminderReceipt({ push: { attempted: false, delivered: false, reason: 'unsubscribed' } }))
 assert.deepEqual((await skipped.reminderReceipt('rem-req /1')).push, { attempted: false, delivered: false, reason: 'unsubscribed' })
 for (const invented of [
  reminderReceipt({ push: { attempted: true, delivered: true, reason: 'delivery_failed' } }),
  reminderReceipt({ push: { attempted: false, delivered: true } }),
  reminderReceipt({ push: { attempted: false, delivered: false, reason: 'provider_timeout' } }),
  reminderReceipt({ push: { attempted: true, delivered: false } }),
  reminderReceipt({ push: { attempted: true, delivered: false, reason: 'unsubscribed' } }),
 ]) {
  const api = createCareerApi(async () => invented)
  await assert.rejects(api.reminderReceipt('rem-req /1'), TypeError)
 }
})

test('reminder list decoder accepts the empty inbox and the discovery shape', async () => {
 const empty = createCareerApi(async () => ({ reminders: [] }))
 assert.deepEqual(await empty.reminders(), { reminders: [] })
 const discovery = createCareerApi(async () => ({ reminders: [reminderView({ reminderId: 'rem-2', sourceKind: 'discovery', sourceId: 'todo /1', applicationId: undefined, opportunityId: undefined, noticeKey: 'discovery_found', notice: '持续找岗有新发现，请登录查看。' })] }))
 const decoded = await discovery.reminders()
 assert.equal(decoded.reminders[0]?.notice, '持续找岗有新发现，请登录查看。')
 assert.equal('applicationId' in decoded.reminders[0]!, false)
 for (const payload of [{}, { reminders: {} }, { reminders: [null] }]) {
  const api = createCareerApi(async () => payload)
  await assert.rejects(api.reminders(), TypeError)
 }
})
