import assert from 'node:assert/strict';
import test from 'node:test';

import type { KnowledgeChunkSearchHit, WeKnoraClient } from '@weknora/api-client';
import {
  createAgentCache,
  createKbCache,
  ensureAgentsLoaded,
  ensureKnowledgeBasesLoaded,
  groupChunksByKnowledge,
  groupMessagesBySession,
  matchAgentsByName,
  matchKnowledgeBasesByName,
  matchSessionsByTitle,
  parseMessageSearchPayload,
  searchKnowledgeChunks,
  searchMessagesByQuery,
  searchSessionsByKeyword,
} from './command-palette-search.ts';

function chunk(overrides: Partial<KnowledgeChunkSearchHit>): KnowledgeChunkSearchHit {
  return {
    id: 'chunk-1',
    content: 'content',
    matchedContent: 'matched',
    knowledgeId: 'doc-1',
    knowledgeBaseId: 'kb-1',
    knowledgeTitle: 'Doc title',
    knowledgeFilename: 'doc.pdf',
    chunkIndex: 0,
    score: 0.5,
    matchType: 'vector',
    ...overrides,
  };
}

// ─── groupChunksByKnowledge ───

test('groupChunksByKnowledge groups hits by knowledgeId and looks up the KB name', () => {
  const hits = [
    chunk({ id: 'c1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-1', knowledgeTitle: 'Doc A' }),
    chunk({ id: 'c2', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-1', knowledgeTitle: 'Doc A' }),
    chunk({ id: 'c3', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-2', knowledgeTitle: 'Doc B' }),
  ];
  const groups = groupChunksByKnowledge(hits, (kbId) => (kbId === 'kb-1' ? 'Handbook' : 'Other KB'));

  assert.equal(groups.length, 2);
  assert.equal(groups[0]?.knowledgeId, 'doc-1');
  assert.equal(groups[0]?.kbName, 'Handbook');
  assert.equal(groups[0]?.chunks.length, 2);
  assert.equal(groups[1]?.knowledgeId, 'doc-2');
  assert.equal(groups[1]?.chunks.length, 1);
});

test('groupChunksByKnowledge falls back to the filename when no title is present', () => {
  const groups = groupChunksByKnowledge([chunk({ knowledgeTitle: '', knowledgeFilename: 'raw.txt' })], () => '');
  assert.equal(groups[0]?.title, 'raw.txt');
});

// ─── groupMessagesBySession ───

test('groupMessagesBySession groups Q&A pairs by sessionId, preserving order', () => {
  const items = [
    { requestId: 'r1', sessionId: 's1', sessionTitle: 'Alpha', queryContent: 'hi', answerContent: 'hello', score: 1, matchType: 'hybrid', createdAt: '2024-01-01' },
    { requestId: 'r2', sessionId: 's2', sessionTitle: 'Beta', queryContent: 'x', answerContent: 'y', score: 0.9, matchType: 'hybrid', createdAt: '2024-01-02' },
    { requestId: 'r3', sessionId: 's1', sessionTitle: 'Alpha', queryContent: 'z', answerContent: 'w', score: 0.8, matchType: 'keyword', createdAt: '2024-01-03' },
  ];
  const groups = groupMessagesBySession(items);
  assert.equal(groups.length, 2);
  assert.equal(groups[0]?.sessionId, 's1');
  assert.equal(groups[0]?.items.length, 2);
  assert.equal(groups[1]?.sessionId, 's2');
});

// ─── matchKnowledgeBasesByName / matchAgentsByName ───

test('matchKnowledgeBasesByName is a case-insensitive substring filter capped at 4', () => {
  const kbs = Array.from({ length: 6 }, (_, i) => ({ id: `kb-${i}`, name: `Handbook ${i}` }));
  const matches = matchKnowledgeBasesByName(kbs, 'HANDBOOK');
  assert.equal(matches.length, 4);
  assert.equal(matchKnowledgeBasesByName(kbs, 'nomatch').length, 0);
});

test('matchAgentsByName matches on name OR description, capped at 5', () => {
  const agents = [
    { id: 'a1', name: 'Support Bot', description: 'handles tickets' },
    { id: 'a2', name: 'Research', description: 'answers about hello world' },
    { id: 'a3', name: 'Other', description: 'unrelated' },
  ];
  const matches = matchAgentsByName(agents, 'hello');
  assert.deepEqual(matches.map((a) => a.id), ['a2']);
});

// ─── matchSessionsByTitle ───

test('matchSessionsByTitle excludes sessions already covered by a message-group hit', () => {
  const sessions = [
    { id: 's1', title: 'Hello project kickoff' },
    { id: 's2', title: 'Hello again' },
  ];
  const matches = matchSessionsByTitle(sessions, 'hello', new Set(['s1']));
  assert.deepEqual(matches.map((s) => s.id), ['s2']);
});

// ─── parseMessageSearchPayload ───

test('parseMessageSearchPayload converts snake_case fields into typed items', () => {
  const parsed = parseMessageSearchPayload({
    items: [{
      request_id: 'r1', session_id: 's1', session_title: 'Alpha',
      query_content: 'hi', answer_content: 'hello', score: 0.75, match_type: 'hybrid', created_at: '2024-01-01',
    }],
    total: 1,
  });
  assert.equal(parsed.total, 1);
  assert.deepEqual(parsed.items[0], {
    requestId: 'r1', sessionId: 's1', sessionTitle: 'Alpha',
    queryContent: 'hi', answerContent: 'hello', score: 0.75, matchType: 'hybrid', createdAt: '2024-01-01',
  });
});

test('parseMessageSearchPayload tolerates a missing items array', () => {
  const parsed = parseMessageSearchPayload({ total: 0 });
  assert.deepEqual(parsed, { items: [], total: 0 });
});

// ─── searchKnowledgeChunks: skips the network call with no KB scope ───

test('searchKnowledgeChunks returns no results and does not call the backend when the KB scope is empty', async () => {
  let called = false;
  const client = { knowledgeBases: { search: async () => { called = true; return []; } } } as unknown as WeKnoraClient;
  const result = await searchKnowledgeChunks(client, { query: 'hello', knowledgeBaseIds: [] });
  assert.deepEqual(result, []);
  assert.equal(called, false);
});

test('searchKnowledgeChunks forwards the query and KB scope to the client and swallows backend errors as an empty result', async () => {
  const calls: unknown[] = [];
  const client = {
    knowledgeBases: {
      search: async (params: unknown) => { calls.push(params); throw new Error('boom'); },
    },
  } as unknown as WeKnoraClient;
  const result = await searchKnowledgeChunks(client, { query: 'hello', knowledgeBaseIds: ['kb-1'] });
  assert.deepEqual(result, []);
  assert.deepEqual(calls, [{ query: 'hello', knowledgeBaseIds: ['kb-1'] }]);
});

// ─── searchMessagesByQuery ───

test('searchMessagesByQuery calls chatHistory.search with hybrid mode and parses the result', async () => {
  const calls: unknown[] = [];
  const client = {
    settings: {
      chatHistory: {
        search: async (input: unknown) => {
          calls.push(input);
          return {
            items: [{ request_id: 'r1', session_id: 's1', session_title: 'Alpha', query_content: 'hi', answer_content: 'hello', score: 1, match_type: 'hybrid', created_at: '2024-01-01' }],
            total: 1,
          };
        },
      },
    },
  } as unknown as WeKnoraClient;

  const result = await searchMessagesByQuery(client, { query: 'hello', limit: 30 });
  assert.deepEqual(calls, [{ query: 'hello', mode: 'hybrid', limit: 30 }]);
  assert.equal(result.total, 1);
  assert.equal(result.items[0]?.sessionId, 's1');
});

test('searchMessagesByQuery swallows backend errors as an empty result', async () => {
  const client = { settings: { chatHistory: { search: async () => { throw new Error('boom'); } } } } as unknown as WeKnoraClient;
  const result = await searchMessagesByQuery(client, { query: 'hello', limit: 30 });
  assert.deepEqual(result, { items: [], total: 0 });
});

// ─── searchSessionsByKeyword ───

test('searchSessionsByKeyword calls sessions.list with a keyword filter', async () => {
  const calls: unknown[] = [];
  const client = {
    sessions: {
      list: async (params: unknown) => {
        calls.push(params);
        return { data: [{ id: 's1', title: 'Hello project', is_pinned: false }], total: 1, page: 1, page_size: 10 };
      },
    },
  } as unknown as WeKnoraClient;

  const result = await searchSessionsByKeyword(client, { query: 'hello', limit: 10 });
  assert.deepEqual(calls, [{ keyword: 'hello', pageSize: 10 }]);
  assert.deepEqual(result, [{ id: 's1', title: 'Hello project' }]);
});

test('searchSessionsByKeyword swallows backend errors as an empty result', async () => {
  const client = { sessions: { list: async () => { throw new Error('boom'); } } } as unknown as WeKnoraClient;
  const result = await searchSessionsByKeyword(client, { query: 'hello', limit: 10 });
  assert.deepEqual(result, []);
});

// ─── ensureKnowledgeBasesLoaded / ensureAgentsLoaded: load-once caches ───

test('ensureKnowledgeBasesLoaded fetches once and reuses the cached list on subsequent calls', async () => {
  let calls = 0;
  const client = {
    knowledgeBases: {
      list: async () => {
        calls += 1;
        return { data: [{ id: 'kb-1', name: 'Handbook' }], total: 1, page: 1, page_size: 10 };
      },
    },
  } as unknown as WeKnoraClient;

  const cache = createKbCache();
  const first = await ensureKnowledgeBasesLoaded(client, cache);
  const second = await ensureKnowledgeBasesLoaded(client, cache);
  assert.equal(calls, 1);
  assert.deepEqual(first, [{ id: 'kb-1', name: 'Handbook' }]);
  assert.deepEqual(second, first);
});

test('ensureKnowledgeBasesLoaded de-dupes concurrent in-flight calls into a single request', async () => {
  let calls = 0;
  let resolveList: (() => void) | undefined;
  const client = {
    knowledgeBases: {
      list: async () => {
        calls += 1;
        await new Promise<void>((resolve) => { resolveList = resolve; });
        return { data: [{ id: 'kb-1', name: 'Handbook' }], total: 1, page: 1, page_size: 10 };
      },
    },
  } as unknown as WeKnoraClient;

  const cache = createKbCache();
  const p1 = ensureKnowledgeBasesLoaded(client, cache);
  const p2 = ensureKnowledgeBasesLoaded(client, cache);
  resolveList?.();
  await Promise.all([p1, p2]);
  assert.equal(calls, 1);
});

test('ensureKnowledgeBasesLoaded swallows backend errors as an empty list and does not poison the cache as loaded', async () => {
  let calls = 0;
  const client = { knowledgeBases: { list: async () => { calls += 1; throw new Error('boom'); } } } as unknown as WeKnoraClient;
  const cache = createKbCache();
  const result = await ensureKnowledgeBasesLoaded(client, cache);
  assert.deepEqual(result, []);
  await ensureKnowledgeBasesLoaded(client, cache);
  assert.equal(calls, 2);
});

test('ensureAgentsLoaded fetches once and reuses the cached list on subsequent calls', async () => {
  let calls = 0;
  const client = {
    configuration: {
      agents: {
        list: async () => {
          calls += 1;
          return [{ id: 'a1', name: 'Support Bot', description: 'handles tickets' }];
        },
      },
    },
  } as unknown as WeKnoraClient;

  const cache = createAgentCache();
  const first = await ensureAgentsLoaded(client, cache);
  const second = await ensureAgentsLoaded(client, cache);
  assert.equal(calls, 1);
  assert.deepEqual(first, [{ id: 'a1', name: 'Support Bot', description: 'handles tickets' }]);
  assert.deepEqual(second, first);
});
