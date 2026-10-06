import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildClaudeCodeCommand,
  buildHttpClientSnippet,
  buildMcpEndpointUrl,
  buildStdioBridgeSnippet,
  groupTools,
  isSharedKbEditable,
  mcpServerKey,
  MCP_TOKEN_PLACEHOLDER,
  mergeKnowledgeBaseOptions,
} from './mcpServerIntegration'

test('buildMcpEndpointUrl strips the api prefix and joins the endpoint path', () => {
  assert.equal(buildMcpEndpointUrl('https://kb.example.com/api/v1', '/mcp/abc'), 'https://kb.example.com/mcp/abc')
  assert.equal(buildMcpEndpointUrl('https://kb.example.com/api/v1/', 'mcp/abc'), 'https://kb.example.com/mcp/abc')
  assert.equal(buildMcpEndpointUrl('http://localhost:8080', '/mcp/x'), 'http://localhost:8080/mcp/x')
})

test('mcpServerKey slugs the endpoint name', () => {
  assert.equal(mcpServerKey('Docs Bot!'), 'weknora-docs-bot')
  assert.equal(mcpServerKey('  '), 'weknora')
  assert.equal(mcpServerKey('产品知识库'), 'weknora')
  assert.equal(mcpServerKey('产品知识库', '3f9a2c1e-aaaa'), 'weknora-3f9a2c1e')
  assert.notEqual(mcpServerKey('知识库A', 'id-one'), mcpServerKey('知识库B', 'id-two'))
})

test('http snippet carries url and bearer header', () => {
  const parsed = JSON.parse(buildHttpClientSnippet('Docs', 'https://h/mcp/1', 'mcp_t'))
  assert.deepEqual(parsed, {
    mcpServers: { 'weknora-docs': { url: 'https://h/mcp/1', headers: { Authorization: 'Bearer mcp_t' } } },
  })
})

test('snippets fall back to a placeholder when no token is known', () => {
  assert.match(buildHttpClientSnippet('a', 'u', ''), new RegExp(MCP_TOKEN_PLACEHOLDER))
  assert.match(buildClaudeCodeCommand('a', 'u', ''), new RegExp(MCP_TOKEN_PLACEHOLDER))
  const bridge = JSON.parse(buildStdioBridgeSnippet('a', 'https://h/mcp/1', ''))
  assert.equal(bridge.mcpServers['weknora-a'].command, 'npx')
  assert.ok(bridge.mcpServers['weknora-a'].args.includes('https://h/mcp/1'))
})

test('claude code command uses the http transport', () => {
  assert.equal(
    buildClaudeCodeCommand('Docs', 'https://h/mcp/1', 'mcp_t'),
    'claude mcp add --transport http weknora-docs https://h/mcp/1 --header "Authorization: Bearer mcp_t"',
  )
})

test('groupTools keeps backend order and drops empty groups', () => {
  const grouped = groupTools(['retrieve', 'chat', 'wiki', 'ingest'], [
    { name: 'ask', group: 'chat', destructive: false },
    { name: 'search_knowledge', group: 'retrieve', destructive: false },
    { name: 'delete_document', group: 'ingest', destructive: true },
  ])
  assert.deepEqual(grouped.map((g) => g.group), ['retrieve', 'chat', 'ingest'])
  assert.deepEqual(grouped[0].tools.map((t) => t.name), ['search_knowledge'])
})

function share(kbId: string | number, permission: string, name?: string) {
  return { knowledge_base: { id: kbId, name }, permission }
}

test('mergeKnowledgeBaseOptions appends shared KBs after own ones (#3828)', () => {
  const merged = mergeKnowledgeBaseOptions(
    [{ id: 1, name: 'Own KB' }],
    [share('s1', 'viewer', 'Shared KB')],
  )
  assert.deepEqual(merged, [
    { id: '1', name: 'Own KB', shared: false },
    { id: 's1', name: 'Shared KB', shared: true, permission: 'viewer' },
  ])
})

test('mergeKnowledgeBaseOptions keeps a shared KB that is also owned exactly once, owned wins', () => {
  const merged = mergeKnowledgeBaseOptions(
    [{ id: 'a', name: 'Owned name' }],
    [share('a', 'editor', 'Shared name'), share('b', 'viewer', 'Other')],
  )
  assert.deepEqual(merged.map((kb) => kb.id), ['a', 'b'])
  const a = merged[0]
  assert.equal(a.shared, false)
  assert.equal(a.name, 'Owned name')
  assert.equal(a.permission, undefined)
})

test('mergeKnowledgeBaseOptions collapses one KB shared through several orgs to the most-privileged grant', () => {
  const merged = mergeKnowledgeBaseOptions([], [
    share('x', 'viewer', 'X'),
    share('x', 'admin', 'X'),
    share('x', 'editor', 'X'),
  ])
  assert.equal(merged.length, 1)
  assert.deepEqual(merged[0], { id: 'x', name: 'X', shared: true, permission: 'admin' })
})

test('mergeKnowledgeBaseOptions orders shared editable grants before view-only, stably', () => {
  const merged = mergeKnowledgeBaseOptions(
    [{ id: 'own2', name: 'B' }, { id: 'own1', name: 'A' }],
    [
      share('v1', 'viewer', 'V1'),
      share('e1', 'editor', 'E1'),
      share('v2', 'viewer', 'V2'),
      share('e2', 'admin', 'E2'),
    ],
  )
  // Own KBs keep API order; editable shared first (share order preserved),
  // then view-only (share order preserved).
  assert.deepEqual(merged.map((kb) => kb.id), ['own2', 'own1', 'e1', 'e2', 'v1', 'v2'])
  assert.deepEqual(merged.map((kb) => kb.shared), [false, false, true, true, true, true])
})

test('mergeKnowledgeBaseOptions skips null rows and falls back to the id for missing names', () => {
  const merged = mergeKnowledgeBaseOptions(
    [null, undefined, { id: 'n1' }],
    [null, { knowledge_base: null, permission: 'viewer' }, share('n2', 'editor')],
  )
  assert.deepEqual(merged, [
    { id: 'n1', name: 'n1', shared: false },
    { id: 'n2', name: 'n2', shared: true, permission: 'editor' },
  ])
})

test('mergeKnowledgeBaseOptions tolerates nullish inputs', () => {
  assert.deepEqual(mergeKnowledgeBaseOptions(null, undefined), [])
})

test('isSharedKbEditable treats admin/editor as editable, viewer/unknown as read-only', () => {
  assert.equal(isSharedKbEditable('admin'), true)
  assert.equal(isSharedKbEditable('editor'), true)
  assert.equal(isSharedKbEditable('viewer'), false)
  assert.equal(isSharedKbEditable(undefined), false)
})
