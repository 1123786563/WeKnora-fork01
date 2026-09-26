import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import type {
  ResearchAnnotationDraft, ResearchAnnotationRow, ResearchDelegationRow, ResearchEvent, TaskResearchHandle,
} from '@weknora/mobile-core';
import { createResearchController, type ResearchViewState } from './research-view.ts';

const here = dirname(fileURLToPath(import.meta.url));

const delegations: ResearchDelegationRow[] = [
  { delegationId: 'd-scenario-1', runId: 'r-scenario', sessionId: 's-scenario', objective: 'survey baselines', sources: ['kb-1'], status: 'assigned', createdAt: '2026-09-26T00:00:00Z' },
];
const annotations: ResearchAnnotationRow[] = [
  { annotationId: 'an1', runId: 'r-scenario', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: 'seed', authorId: 'u3', createdAt: '2026-09-26T00:00:00Z' },
];

interface StubHandle extends TaskResearchHandle {
  annotateCalls: Array<{ materialId: string; baseVersion: string; body: string }>;
  closeReason?: string;
}

/** 结构化 stub：offline=true 时 annotate 返回 drafted 并记账 drafts（模拟离线草稿路径）。 */
function stubHandle(options: { offline?: boolean } = {}): StubHandle {
  const drafts: ResearchAnnotationDraft[] = [];
  let draftSeq = 0;
  const handle: StubHandle = {
    annotateCalls: [],
    closeReason: undefined,
    async delegate(input) { return delegations[0]!; },
    async delegations() { return delegations; },
    async complete(input) { return { ...delegations[0]!, status: 'completed' as const, summary: input.summary }; },
    async annotate(input) {
      handle.annotateCalls.push(input);
      if (options.offline === true) {
        draftSeq += 1;
        const draft: ResearchAnnotationDraft = { draftId: `research-ann-${draftSeq}`, runId: 'r-scenario', materialId: input.materialId, baseVersion: input.baseVersion, body: input.body, draftedAt: new Date().toISOString() };
        drafts.push(draft);
        return { status: 'drafted', draftId: draft.draftId };
      }
      return {
        status: 'recorded',
        annotation: { annotationId: `an-live-${handle.annotateCalls.length}`, runId: 'r-scenario', materialId: input.materialId, baseVersion: input.baseVersion, body: input.body, authorId: 'u1', createdAt: '2026-09-26T00:00:00Z' },
      };
    },
    async annotations() { return annotations; },
    async requestRevision(input) { return { intent: 'revision-request', outcome: 'accepted', action: input.action, at: '2026-09-26T00:00:00Z' }; },
    async flushAnnotationDrafts() {
      const flushed = drafts.splice(0);
      return flushed.map((draft) => ({ draftId: draft.draftId, outcome: 'recorded' as const }));
    },
    async pendingDrafts() { return [...drafts]; },
    subscribe() { return () => undefined; },
    close(reason: string) { handle.closeReason = reason; },
  };
  return handle;
}

test('controller loads delegations, annotations and pending drafts', async () => {
  const controller = createResearchController(stubHandle(), { runId: 'r-scenario' });
  const states: ResearchViewState[] = [];
  const unsubscribe = controller.subscribe((state) => states.push(state));
  await controller.load();
  unsubscribe();
  const settled = controller.state();
  assert.equal(settled.loading, false);
  assert.equal(settled.delegations?.length, 1);
  assert.equal(settled.delegations?.[0]?.objective, 'survey baselines');
  assert.equal(settled.annotations?.length, 1);
  assert.equal(settled.pendingDrafts, 0);
  controller.dispose();
});

test('controller annotate records online and drafts offline with honest copy', async () => {
  const onlineController = createResearchController(stubHandle(), { runId: 'r-scenario' });
  await onlineController.annotate({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', body: '好' });
  assert.match(onlineController.state().notice ?? '', /已记录/);

  const offlineController = createResearchController(stubHandle({ offline: true }), { runId: 'r-scenario' });
  await offlineController.annotate({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', body: '离线批注' });
  assert.equal(offlineController.state().pendingDrafts, 1);
  assert.match(offlineController.state().notice ?? '', /加密草稿/);
  await offlineController.flushDrafts();
  assert.equal(offlineController.state().pendingDrafts, 0);
  assert.match(offlineController.state().notice ?? '', /同步/);
  offlineController.dispose();
});

test('research route and composition wiring stay at the Interface boundary', async () => {
  // 源级断言：Screen 不导入 wire 层；路由只经 composition 取模块。
  const route = readFileSync(join(here, 'app/tasks/research.tsx'), 'utf8');
  assert.ok(!route.includes('@weknora/api-client'), '路由禁止直接导入 api-client（module-seams §10）');
  assert.ok(route.includes('activeTaskResearch'), '路由必须经 composition 工厂取模块');
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.ok(composition.includes('createTaskResearch'), 'composition 必须装配 research 深模块');
  assert.ok(composition.includes('activeTaskResearch'), 'composition 必须导出 activeTaskResearch');
  const screen = readFileSync(join(here, 'screens/ResearchScreen.tsx'), 'utf8');
  assert.ok(!screen.includes('@weknora/contracts') && !screen.includes('@weknora/api-client'), 'Screen 禁止导入 contracts/api-client');
});
