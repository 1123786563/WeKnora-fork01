import assert from 'node:assert/strict';
import test from 'node:test';

import { loadStarterQuestions, starterQuestionTarget, type AgentQuestionsApi } from './starter-questions.ts';

function api(questions: string[] | Error): AgentQuestionsApi & { calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    async suggestedQuestions(agentId) { calls.push(agentId); if (questions instanceof Error) throw questions; return questions; },
  };
}

test('agent picker surface: starters are fetched from the selected agent when no session is open', async () => {
  const client = api(['What is WeKnora?', 'How do I upload files?']);
  const questions = await loadStarterQuestions(client, null, 'agent-1');
  assert.deepEqual(questions, ['What is WeKnora?', 'How do I upload files?']);
  assert.deepEqual(client.calls, ['agent-1']);
});

test('no starters without an agent selection (no session-level route on the main API)', async () => {
  const client = api(['never']);
  assert.deepEqual(await loadStarterQuestions(client, null, ''), []);
  assert.equal(client.calls.length, 0);
  assert.deepEqual(await loadStarterQuestions(client, null, undefined), []);
  assert.equal(client.calls.length, 0);
});

test('an open session suppresses starters (message-level suggestions own that state)', async () => {
  const client = api(['never']);
  assert.deepEqual(await loadStarterQuestions(client, 'session-1', 'agent-1'), []);
  assert.equal(client.calls.length, 0);
});

test('backend errors degrade to no starters instead of surfacing a chat error', async () => {
  const client = api(new Error('404 agent not found'));
  assert.deepEqual(await loadStarterQuestions(client, null, 'missing-agent'), []);
});

test('starterQuestionTarget picks the agent id only for the new-conversation view', () => {
  assert.equal(starterQuestionTarget(null, 'agent-1'), 'agent-1');
  assert.equal(starterQuestionTarget(null, '  '), null);
  assert.equal(starterQuestionTarget('session-1', 'agent-1'), null);
});
