import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { decodeCareerError, decodeCareerReceipt, decodeCareerSource, decodeCareerUpload } from './contracts.ts'
const fixture = JSON.parse(readFileSync(new URL('../testdata/wire-fixtures.json', import.meta.url), 'utf8')) as Record<string, unknown>
test('Go wire fixtures decode proposed and confirmed receipts without type ambiguity', () => {
 const proposed = decodeCareerReceipt(fixture.proposed); assert.equal(proposed.kind, 'proposed')
 if (proposed.kind === 'proposed') assert.equal(proposed.proposal.id, 'proposal-1')
 const confirmed = decodeCareerReceipt(fixture.confirmed); assert.equal(confirmed.kind, 'confirmed')
 if (confirmed.kind === 'confirmed') { assert.equal(confirmed.fact.source.referenceId, 'file-1'); assert.equal(confirmed.proposal?.status, 'confirmed') }
 assert.throws(() => decodeCareerReceipt({ requestId: 'x', revision: 1, kind: 'proposed', fact: {} }))
})
test('Go HTTP error fixtures decode not found, revision conflict, and unknown outcomes', () => {
 assert.equal(decodeCareerError(fixture.not_found).error.code, 'not_found')
 assert.equal(decodeCareerError(fixture.revision_conflict).error.currentRevision, 2)
 const unknown = decodeCareerError(fixture.outcome_unknown)
 assert.equal(unknown.error.code, 'outcome_unknown')
 if (unknown.error.code === 'outcome_unknown') assert.equal(unknown.error.requestId, 'same')
 assert.throws(() => decodeCareerError({ error: { code: 'outcome_unknown', message: 'unknown' } }))
})
test('Career upload source and intake receipt decode without exposing private resource fields', () => {
 const decoded = decodeCareerUpload({ source: { id: 's1', revision: 2, fileName: 'resume.pdf', mimeType: 'application/pdf', size: 44, digest: 'sha256', status: 'ready', missingCategories: ['education.graduation_date'], reviewFlags: ['experience.date_conflict'], createdAt: 'now' }, receipt: { kind: 'intake_completed', requestId: 's1:batch', revision: 2, proposals: [{ id: 'p1', key: 'experience.dates', value: '2024-2025', evidence: 'Exact line', source: { kind: 'resume_extraction', referenceId: 's1' }, status: 'pending', createdAt: 'now' }] } })
 assert.equal(decoded.source.status, 'ready')
 assert.equal(decoded.receipt?.kind, 'intake_completed')
 if (decoded.receipt?.kind === 'intake_completed') assert.equal(decoded.receipt.proposals[0]?.evidence, 'Exact line')
 assert.throws(() => decodeCareerSource({ id: 's1', status: 'ready', extractedText: 'private' }))
 assert.throws(() => decodeCareerUpload({ source: { id: 's1', status: 'unknown' } }))
})
