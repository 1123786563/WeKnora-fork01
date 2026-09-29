import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { OfflineGateError } from '@weknora/mobile-core';
import type {
  ResearchAnnotationDraft, ResearchAnnotationRow, ResearchDelegationRow, ResearchEvent, TaskResearchHandle,
} from '@weknora/mobile-core';
import { createResearchController, RESEARCH_ERROR_COPY, RESEARCH_OFFLINE_REVISION_COPY, type ResearchViewState } from './research-view.ts';

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

/** 结构化 stub：offline=true 时 annotate 返回 drafted 并记账 drafts（模拟离线草稿路径）；
 *  offlineRevision=true 时 requestRevision 上抛真 OfflineGateError（模拟 Run 命令离线判决）。 */
function stubHandle(options: { offline?: boolean; offlineRevision?: boolean } = {}): StubHandle {
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
    async requestRevision(input) {
      if (options.offlineRevision === true) throw new OfflineGateError('run'); // 模块原样上抛的结构化离线判决
      return { intent: 'revision-request', outcome: 'accepted', action: input.action, at: '2026-09-26T00:00:00Z' };
    },
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
  // 终局修复回归（发现 2）：生产装配必须注入 commands 端口——切片以 taskResearchFor
  // 函数体为界（不跨函数匹配 taskOfficeFor 的既有 commands），缺失该注入时生产修订
  // 请求恒命中 RESEARCH_COMMAND_UNAVAILABLE fail closed（计划 Task 7 代码块缺此行）。
  const factoryStart = composition.indexOf('function taskResearchFor');
  const factoryEnd = composition.indexOf('export function activeTaskResearch');
  assert.ok(factoryStart >= 0 && factoryEnd > factoryStart, 'composition 必须保留 taskResearchFor 工厂');
  const researchFactory = composition.slice(factoryStart, factoryEnd);
  assert.ok(researchFactory.includes('commands:'), 'taskResearchFor 必须注入 commands 端口：缺失时修订请求在生产中恒不可用（RESEARCH_COMMAND_UNAVAILABLE）');
  assert.ok(researchFactory.includes('createTaskOfficeRemote'), 'commands 端口必须复用 #37 既有命令通道 adapter（createTaskOfficeRemote，同 taskOfficeFor 先例）');
});

test('controller requestRevision surfaces the structured offline rejection with honest copy', async () => {
  // 终局修复回归（发现 1）：离线修订请求是结构化 OfflineGateError，控制器文案必须是
  // 离线判决文案——绝不得落 RESEARCH_BACKEND 的「服务端暂时不可用」（误导用户以为服务端故障）。
  const controller = createResearchController(stubHandle({ offlineRevision: true }), { runId: 'r-scenario' });
  await controller.requestRevision({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', note: '补齐引用', action: 'steer', expectedRevision: 3 });
  assert.equal(controller.state().error, RESEARCH_OFFLINE_REVISION_COPY);
  assert.notEqual(controller.state().error, RESEARCH_ERROR_COPY.RESEARCH_BACKEND);
  controller.dispose();

  const online = createResearchController(stubHandle(), { runId: 'r-scenario' });
  await online.requestRevision({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', note: '补齐引用', action: 'steer', expectedRevision: 3 });
  assert.match(online.state().notice ?? '', /修订请求已提交/);
  online.dispose();
});
