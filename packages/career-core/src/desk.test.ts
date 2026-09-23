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
