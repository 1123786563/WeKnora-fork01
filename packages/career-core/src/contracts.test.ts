import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { decodeCareerError, decodeCareerReceipt } from './contracts.ts'
const fixture = JSON.parse(readFileSync(new URL('../testdata/wire-fixtures.json', import.meta.url), 'utf8')) as Record<string, unknown>
test('Go wire fixtures decode proposed and confirmed receipts without type ambiguity', () => {
 const proposed = decodeCareerReceipt(fixture.proposed); assert.equal(proposed.kind, 'proposed')
 if (proposed.kind === 'proposed') assert.equal(proposed.proposal.id, 'proposal-1')
 const confirmed = decodeCareerReceipt(fixture.confirmed); assert.equal(confirmed.kind, 'confirmed')
 if (confirmed.kind === 'confirmed') { assert.equal(confirmed.fact.source.referenceId, 'file-1'); assert.equal(confirmed.proposal?.status, 'confirmed') }
 assert.throws(() => decodeCareerReceipt({ requestId: 'x', revision: 1, kind: 'proposed', fact: {} }))
})
test('Go HTTP error fixtures decode not found and revision conflict responses', () => {
 assert.equal(decodeCareerError(fixture.not_found).error.code, 'not_found')
 assert.equal(decodeCareerError(fixture.revision_conflict).error.currentRevision, 2)
})
