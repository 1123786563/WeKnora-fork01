import test from 'node:test';
import assert from 'node:assert/strict';
import {
  selectRenderer,
  parseStructuredResultText,
  parseKnowledgeCitation,
  resolveAnalysisResult,
  parseArtifactFile,
  routeStructuredResult,
  ANALYSIS_TABLE_LIMITS,
} from './registry.ts';
test('unknown server component never becomes executable UI',()=>{
 assert.equal(selectRenderer('knowledge.citation'),'citation');
 assert.equal(selectRenderer('javascript:eval'),'text');
 assert.equal(selectRenderer('remote-component'),'text');
});

test('structured result text is only detected from JSON object envelopes', () => {
  const envelope = parseStructuredResultText('{"type":"analysis.table","data":{"columns":["a"],"rows":[[1]]}}');
  assert.equal(envelope?.type, 'analysis.table');
  assert.deepEqual(envelope?.data, { columns: ['a'], rows: [[1]] });
  // Flattened payloads carry the fields next to `type` and become their own data.
  const flat = parseStructuredResultText('{"type":"knowledge.citation","document_id":"d1","chunk_ids":["c1"]}');
  assert.equal(flat?.type, 'knowledge.citation');
  assert.deepEqual(flat?.data, { document_id: 'd1', chunk_ids: ['c1'] });
  // Anything that is not a JSON object with a string type is not structured.
  assert.equal(parseStructuredResultText('plain assistant prose'), null);
  assert.equal(parseStructuredResultText('[1,2,3]'), null);
  assert.equal(parseStructuredResultText('{"no_type":true}'), null);
  assert.equal(parseStructuredResultText('{"type":42}'), null);
  assert.equal(parseStructuredResultText('not json {'), null);
});

test('citation requires authorized knowledge/document/chunk ids and never a source url', () => {
  const citation = parseKnowledgeCitation({
    document_id: 'doc-1', knowledge_id: 'kb-1', chunk_ids: ['c1', 'c2'],
    title: '产品手册', snippet: '长中文摘录'.repeat(3), source_url: 'https://evil.example/doc',
  });
  assert.equal(citation.documentID, 'doc-1');
  assert.equal(citation.knowledgeID, 'kb-1');
  assert.deepEqual(citation.chunkIDs, ['c1', 'c2']);
  // The parsed contract data carries no executable URL — opening always goes
  // back through the injected product resource seam.
  assert.ok(!('sourceURL' in citation));
  // URL-only citations have no authorized id to re-request and must not render.
  assert.throws(() => parseKnowledgeCitation({ source_url: 'https://evil.example', title: 'x' }), /CITATION_REQUIRES_AUTHORIZED_IDS/);
  assert.throws(() => parseKnowledgeCitation({ document_id: '', chunk_ids: [] }), /CITATION_REQUIRES_AUTHORIZED_IDS/);
  assert.throws(() => parseKnowledgeCitation({ document_id: 'd', chunk_ids: [''] }), /CITATION_CHUNK_IDS/);
  assert.throws(() => parseKnowledgeCitation('nope'), /CITATION_PAYLOAD_INVALID/);
});

test('analysis tables validate shape, cell types and fall back to file when over limit', () => {
  const small = resolveAnalysisResult({ columns: ['列A', '列B'], rows: [[1, 'x'], [true, null]] });
  assert.equal(small.renderer, 'table');
  assert.deepEqual(small.renderer === 'table' ? small.table.columns : [], ['列A', '列B']);

  const wide = resolveAnalysisResult({ columns: Array.from({ length: ANALYSIS_TABLE_LIMITS.maxColumns + 1 }, (_, i) => `c${i}`), rows: [[1]] });
  assert.equal(wide.renderer, 'file');
  assert.equal(wide.renderer === 'file' ? wide.reason : '', 'TOO_MANY_COLUMNS');

  const tall = resolveAnalysisResult({ columns: ['a'], rows: Array.from({ length: ANALYSIS_TABLE_LIMITS.maxRows + 1 }, () => [1]) });
  assert.equal(tall.renderer, 'file');
  assert.equal(tall.renderer === 'file' ? tall.reason : '', 'TOO_MANY_ROWS');

  const hugeCell = resolveAnalysisResult({ columns: ['a'], rows: [['x'.repeat(ANALYSIS_TABLE_LIMITS.maxCellLength + 1)]] });
  assert.equal(hugeCell.renderer, 'file');
  assert.equal(hugeCell.renderer === 'file' ? hugeCell.reason : '', 'CELL_TOO_LARGE');

  // Non-scalar cells and ragged rows are not renderable tables: safe text.
  const badCell = resolveAnalysisResult({ columns: ['a'], rows: [[{ evil: true }]] });
  assert.equal(badCell.renderer, 'text');
  const ragged = resolveAnalysisResult({ columns: ['a', 'b'], rows: [[1]] });
  assert.equal(ragged.renderer, 'text');
  assert.equal(resolveAnalysisResult('not a table').renderer, 'text');
  assert.equal(resolveAnalysisResult({ columns: [], rows: [] }).renderer, 'text');
});

test('artifact files validate name, mime, size and a product object reference', () => {
  const file = parseArtifactFile({ name: 'result.csv', mime: 'text/csv', bytes: 12, ref: 'attachment-9' });
  assert.equal(file.name, 'result.csv');
  assert.equal(file.bytes, 12);
  // A URL-shaped ref would invite fetching outside the product surface.
  assert.throws(() => parseArtifactFile({ name: 'x', mime: 'text/csv', bytes: 1, ref: 'https://evil.example/f' }), /ARTIFACT_REF/);
  assert.throws(() => parseArtifactFile({ name: '', mime: 'text/csv', bytes: 1, ref: 'a' }), /ARTIFACT_PAYLOAD_INVALID/);
  assert.throws(() => parseArtifactFile({ name: 'x', mime: 'text/csv', bytes: -1, ref: 'a' }), /ARTIFACT_PAYLOAD_INVALID/);
});

test('routing maps every envelope through the registry switch with safe fallbacks', () => {
  const routed = routeStructuredResult({ type: 'knowledge.citation', data: { document_id: 'd', chunk_ids: ['c'] } });
  assert.equal(routed.renderer, 'citation');
  // Corrupt payload for a known type degrades to safe text, not a shell card.
  const corrupt = routeStructuredResult({ type: 'knowledge.citation', data: { source_url: 'https://x' } });
  assert.equal(corrupt.renderer, 'text');
  const file = routeStructuredResult({ type: 'artifact.file', data: { name: 'a.csv', mime: 'text/csv', bytes: 3, ref: 'r1' } });
  assert.equal(file.renderer, 'file');
  const oversized = routeStructuredResult({ type: 'analysis.table', data: { columns: Array.from({ length: 99 }, (_, i) => `c${i}`), rows: [[]] } });
  assert.equal(oversized.renderer, 'file');
  // Unknown kinds never leave the safe text renderer.
  for (const type of ['javascript:eval', 'remote-component', 'widget.chart.v2']) {
    assert.equal(routeStructuredResult({ type, data: { anything: true } }).renderer, 'text');
    assert.equal(selectRenderer(type), 'text');
  }
});
