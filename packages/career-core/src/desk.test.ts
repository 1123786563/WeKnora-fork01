import assert from 'node:assert/strict'
import test from 'node:test'
import { CareerDesk, type CareerRemote } from './desk.ts'
import type { CareerAction, CareerReceipt, CareerView } from './contracts.ts'

const view = (revision: number, facts: CareerView['facts'] = [], proposals: CareerView['proposals'] = []): CareerView => ({ revision, facts, proposals })
const pending = { id: 'p1', key: '毕业时间', value: '2027', source: { kind: 'user' }, status: 'pending' as const, createdAt: 'now' }
function remote(overrides: Partial<CareerRemote> = {}): CareerRemote {
 return {
  open: async () => view(0), list: async () => view(0), changes: async () => ({ revision: 0, changes: [] }),
  act: async (action) => ({ kind: 'proposed', requestId: action.requestId, revision: 1, proposal: pending }),
  receipt: async () => { throw Object.assign(new Error('missing'), { code: 'not_found' }) }, ...overrides,
 }
}

test('proposal remains pending until receipt says confirmed', async () => {
 const desk = new CareerDesk(remote({ act: async (action) => ({ kind: 'proposed', requestId: action.requestId, revision: 1, proposal: pending }), list: async () => view(1, [], [pending]) }))
 desk.activate('user-1', 'tenant-1')
 const result = await desk.mutate({ action: 'propose', key: pending.key, value: pending.value, source: pending.source, requestId: 'r1', expectedRevision: 0 })
 assert.equal(result.kind, 'proposed')
 const current = await desk.refresh()
 assert.equal(current.facts.length, 0)
 assert.equal(current.proposals[0]?.status, 'pending')
})

test('unknown act reconciles durable receipt before returning', async () => {
 const confirmed = { kind: 'confirmed', requestId: 'r2', revision: 1, fact: { key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } satisfies CareerReceipt
 const desk = new CareerDesk(remote({ act: async () => { throw Object.assign(new Error('unknown'), { code: 'outcome_unknown' }) }, receipt: async (id) => { assert.equal(id, 'r2'); return confirmed } }))
 desk.activate('u', 't')
 assert.deepEqual(await desk.mutate({ action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'r2', expectedRevision: 0 }), confirmed)
})

test('revision conflict exposes backend current revision', async () => {
 const desk = new CareerDesk(remote({ act: async () => { throw Object.assign(new Error('conflict'), { code: 'revision_conflict', currentRevision: 4 }) } }))
 desk.activate('u', 't')
 await assert.rejects(desk.mutate({ action: 'confirm', key: '城市', value: '上海', source: { kind: 'user' }, requestId: 'r3', expectedRevision: 0 }), (error: unknown) => (error as { currentRevision?: number }).currentRevision === 4)
})

test('late private read is discarded after scope switch', async () => {
 let resolve!: (value: CareerView) => void
 const desk = new CareerDesk(remote({ open: () => new Promise((done) => { resolve = done }) }))
 desk.activate('u', 'tenant-a')
 const opening = desk.open()
 desk.activate('u', 'tenant-b')
 resolve(view(7, [{ key: '秘密', value: 'A', revision: 7, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }]))
 assert.equal(await opening, undefined)
 assert.equal(desk.snapshot, undefined)
})

test('forbidden open stays an error and does not retain prior tenant facts', async () => {
 let forbidden = false
 const desk = new CareerDesk(remote({ open: async () => { if (forbidden) throw Object.assign(new Error('forbidden'), { code: 'forbidden' }); return view(2) } }))
 desk.activate('u', 'tenant-a'); await desk.open(); forbidden = true; desk.activate('u', 'tenant-b')
 await assert.rejects(desk.open(), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.snapshot, undefined)
})

test('dropped act response reconciles committed receipt and never sends a second request id', async () => {
 const fact = { key: '城市', value: '上海', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }
 const saved = { kind: 'confirmed', requestId: 'stable-rid', revision: 1, fact } satisfies CareerReceipt
 let actCalls = 0
 const desk = new CareerDesk(remote({
  act: async () => { actCalls += 1; throw new TypeError('network connection lost after commit') },
  receipt: async (id) => { assert.equal(id, 'stable-rid'); return saved },
 }))
 desk.activate('u', 't'); await desk.open()
 const action: CareerAction = { action: 'confirm', key: '城市', value: '上海', source: { kind: 'user' }, requestId: 'stable-rid', expectedRevision: 0 }
 assert.deepEqual(await desk.mutate(action), saved)
 assert.equal(actCalls, 1)
 assert.equal(desk.snapshot?.facts[0]?.value, '上海')
})

test('unresolved mutation gates new actions; same action can retry only with original id after receipt miss', async () => {
 const sent: CareerAction[] = []
 let receiptExists = false
 const desk = new CareerDesk(remote({
  act: async (action) => { sent.push(action); if (sent.length === 1) throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }); return { kind: 'confirmed', requestId: action.requestId, revision: 1, fact: { key: action.key, value: action.value, revision: 1, source: action.source, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } },
  receipt: async (id) => { if (!receiptExists) throw Object.assign(new Error('not found'), { code: 'not_found' }); return { kind: 'confirmed', requestId: id, revision: 1, fact: { key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' } } },
 }))
 desk.activate('u', 't')
 const action: CareerAction = { action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'same-id', expectedRevision: 0 }
 await assert.rejects(desk.mutate(action), (error: unknown) => (error as { code?: string }).code === 'outcome_unknown')
 assert.deepEqual(desk.pendingAction, action)
 await assert.rejects(desk.mutate({ ...action, key: '城市', value: '深圳', requestId: 'new-id' }), (error: unknown) => (error as { code?: string }).code === 'unresolved_action')
 await assert.rejects(desk.retryUnknown({ ...action, value: '改过的值' }), (error: unknown) => (error as { code?: string }).code === 'retry_payload_mismatch')
 receiptExists = true
 await desk.retryUnknown(action)
 assert.equal(sent.length, 2)
 assert.equal(sent[0]?.requestId, 'same-id')
 assert.equal(sent[1]?.requestId, 'same-id')
})

test('late list response cannot regress same-scope receipt facts or revision', async () => {
 let resolveList!: (value: CareerView) => void
 const confirmed = { key: '学历', value: '本科', revision: 2, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }
 const desk = new CareerDesk(remote({
  list: () => new Promise((resolve) => { resolveList = resolve }),
  act: async (action) => ({ kind: 'confirmed', requestId: action.requestId, revision: 2, fact: confirmed }),
 }))
 desk.activate('u', 't'); await desk.open()
 const refreshing = desk.refresh()
 await desk.mutate({ action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'r', expectedRevision: 0 })
 resolveList(view(1))
 await refreshing
 assert.equal(desk.snapshot?.revision, 2)
 assert.equal(desk.snapshot?.facts[0]?.value, '本科')
})

test('late changes response cannot regress a newer receipt revision', async () => {
 let resolveChanges!: (value: CareerView extends never ? never : { revision: number; changes: CareerView['facts'] extends never ? never : Array<{ revision: number; kind: 'confirmed'; fact: CareerView['facts'][number] }> }) => void
 const latest = { key: '学历', value: '本科', revision: 2, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }
 const desk = new CareerDesk(remote({
  changes: () => new Promise((resolve) => { resolveChanges = resolve }),
  act: async (action) => ({ kind: 'confirmed', requestId: action.requestId, revision: 2, fact: latest }),
 }))
 desk.activate('u', 't'); await desk.open()
 const syncing = desk.syncChanges()
 await desk.mutate({ action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'r', expectedRevision: 0 })
 resolveChanges({ revision: 1, changes: [{ revision: 1, kind: 'confirmed', fact: { ...latest, value: '旧值', revision: 1 } }] })
 await syncing
 assert.equal(desk.snapshot?.revision, 2)
 assert.equal(desk.snapshot?.facts[0]?.value, '本科')
})

test('forbidden same-scope refresh clears cached private view and pending action', async () => {
 let forbidden = false
 const desk = new CareerDesk(remote({
  open: async () => view(1, [{ key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }]),
  list: async () => { if (forbidden) throw Object.assign(new Error('forbidden'), { code: 'forbidden' }); return view(1) },
 }))
 desk.activate('u', 't'); await desk.open(); forbidden = true
 await assert.rejects(desk.refresh(), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.snapshot, undefined)
})


test('forbidden read also discards an unresolved action in the same scope', async () => {
 let forbidden = false
 const desk = new CareerDesk(remote({
  open: async () => view(1, [{ key: '学历', value: '本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }]),
  act: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
  receipt: async () => { throw Object.assign(new Error('not found'), { code: 'not_found' }) },
  list: async () => { if (forbidden) throw Object.assign(new Error('revoked'), { code: 'forbidden' }); return view(1) },
 }))
 desk.activate('u', 't'); await desk.open()
 const action: CareerAction = { action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'pending', expectedRevision: 1 }
 await assert.rejects(desk.mutate(action), (error: unknown) => (error as { code?: string }).code === 'outcome_unknown')
 assert.equal(desk.pendingAction?.requestId, 'pending')
 forbidden = true
 await assert.rejects(desk.refresh(), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.snapshot, undefined)
 assert.equal(desk.pendingAction, undefined)
})

test('same-id retry conflict safely resolves the unknown action and unlocks new edits', async () => {
 let actCalls = 0
 const desk = new CareerDesk(remote({
  act: async () => { actCalls += 1; if (actCalls === 1) throw Object.assign(new Error('lost response'), { code: 'TIMEOUT' }); throw Object.assign(new Error('stale revision'), { code: 'revision_conflict', currentRevision: 3 }) },
  receipt: async () => { throw Object.assign(new Error('not found'), { code: 'not_found' }) },
 }))
 desk.activate('u', 't'); await desk.open()
 const action: CareerAction = { action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'same', expectedRevision: 0 }
 await assert.rejects(desk.mutate(action), (error: unknown) => (error as { code?: string }).code === 'outcome_unknown')
 await assert.rejects(desk.retryUnknown(action), (error: unknown) => (error as { code?: string; currentRevision?: number }).code === 'revision_conflict' && (error as { currentRevision?: number }).currentRevision === 3)
 assert.equal(desk.pendingAction, undefined)
 await assert.rejects(desk.mutate({ ...action, requestId: 'next', expectedRevision: 3, value: '硕士' }), (error: unknown) => (error as { code?: string }).code !== 'unresolved_action')
})

test('forbidden read invalidates concurrent same-scope reads so late success cannot repopulate facts', async () => {
 let resolveLate!: (result: CareerView) => void
 let openCalls = 0
 const privateView = view(1, [{ key: '学历', value: '秘密本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }])
 const desk = new CareerDesk(remote({
  open: async () => { openCalls += 1; return openCalls === 1 ? privateView : new Promise((resolve) => { resolveLate = resolve }) },
  list: async () => { throw Object.assign(new Error('revoked'), { code: 'forbidden' }) },
 }))
 desk.activate('u', 't'); await desk.open()
 const lateRead = desk.open()
 await assert.rejects(desk.refresh(), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.snapshot, undefined)
 resolveLate(privateView)
 assert.equal(await lateRead, undefined)
 assert.equal(desk.snapshot, undefined)
})

test('forbidden receipt after ambiguous act invalidates scope and clears facts and pending action', async () => {
 const privateView = view(1, [{ key: '学历', value: '秘密本科', revision: 1, source: { kind: 'user' }, confirmation: { userId: 'u', confirmedAt: 'now' }, confirmedAt: 'now' }])
 const desk = new CareerDesk(remote({
  open: async () => privateView,
  act: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
  receipt: async () => { throw Object.assign(new Error('revoked'), { code: 'forbidden' }) },
 }))
 desk.activate('u', 't'); await desk.open()
 await assert.rejects(desk.mutate({ action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'same', expectedRevision: 1 }), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.snapshot, undefined)
 assert.equal(desk.pendingAction, undefined)
})


test('retry receipt forbidden stays a forbidden state after a previous receipt miss', async () => {
 let receiptCalls = 0
 const desk = new CareerDesk(remote({
  act: async () => { throw Object.assign(new Error('timeout'), { code: 'TIMEOUT' }) },
  receipt: async () => { receiptCalls += 1; if (receiptCalls === 1) throw Object.assign(new Error('missing'), { code: 'not_found' }); throw Object.assign(new Error('revoked'), { code: 'forbidden' }) },
 }))
 desk.activate('u', 't'); await desk.open()
 const action: CareerAction = { action: 'confirm', key: '学历', value: '本科', source: { kind: 'user' }, requestId: 'r', expectedRevision: 0 }
 await assert.rejects(desk.mutate(action), (error: unknown) => (error as { code?: string }).code === 'outcome_unknown')
 await assert.rejects(desk.retryUnknown(action), (error: unknown) => (error as { code?: string }).code === 'forbidden')
 assert.equal(desk.pendingAction, undefined)
 assert.equal(desk.snapshot, undefined)
})
