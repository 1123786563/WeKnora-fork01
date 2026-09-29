import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as nodeModule from 'node:module';

// VoiceRoomScreen.tsx 静态拉入 react-native 源码，esbuild 无法转换——与
// voice-dictation-integration-smoke.test.ts 相同的在库先例：react-native 解析为惰性
// CommonJS stub，再动态导入被测模块（静态导入会被提升，必须动态导入）。
type ResolveNext = (specifier: string, context: unknown) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown }) => void;
};

const NATIVE_MODULE_STUBS: Record<string, string> = {
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView' }",
};
const stubDir = mkdtempSync(join(tmpdir(), 'weknora-voice-room-stub-'));
const stubPath = (name: string): string => join(stubDir, `${name.replaceAll('/', '+')}.cjs`);
for (const [name, source] of Object.entries(NATIVE_MODULE_STUBS)) {
  writeFileSync(stubPath(name), source);
}
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier in NATIVE_MODULE_STUBS ? { shortCircuit: true, url: pathToFileURL(stubPath(specifier)).href } : nextResolve(specifier, context),
  });
}
test.after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

const loadView = () => import('./voice-room-view.ts');
const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

/** 脚本化 VoiceHandle 替身：控制器是被测对象（Voice Room 模块 Interface 已在 mobile-core 全测）。 */
function scriptHandle(): { handle: import('@weknora/mobile-core').VoiceHandle; turnToReady(text: string): Promise<void> } {
  type VoiceRoomState = import('@weknora/mobile-core').VoiceRoomState;
  let listener: ((state: VoiceRoomState) => void) | undefined;
  let seq = 0;
  let state: VoiceRoomState = { phase: 'idle', taskId: 'task-1', turns: [] };
  const publish = (next: VoiceRoomState): void => { state = next; listener?.(state); };
  const handle: import('@weknora/mobile-core').VoiceHandle = {
    state: () => state,
    subscribe(l) { listener = l; return () => { listener = undefined; }; },
    async beginTurn() { publish({ ...state, phase: 'listening' }); },
    async endTurn() {
      const id = `req-${++seq}`;
      publish({ ...state, phase: 'transcribing', pendingTurnId: id });
    },
    editTranscript(id, text) {
      publish({ ...state, turns: state.turns.map((t) => (t.turnId === id && t.state === 'review' ? { ...t, transcript: text } : t)) });
    },
    confirmTranscript(id) {
      if (state.pendingTurnId !== id) return undefined;
      const turn = state.turns.find((t) => t.turnId === id);
      if (turn === undefined) return undefined;
      const text = turn.transcript.trim();
      if (text === '') return undefined;
      publish({ ...state, phase: 'idle', pendingTurnId: undefined, turns: state.turns.map((t) => (t.turnId === id ? { ...t, state: 'confirmed' as const, transcript: text } : t)) });
      return { kind: 'steer' as const, text };
    },
    discardTurn(id) {
      if (state.pendingTurnId !== id) return;
      publish({ ...state, phase: 'idle', pendingTurnId: undefined, turns: state.turns.map((t) => (t.turnId === id ? { ...t, state: 'discarded' as const } : t)) });
    },
    async resume() { publish({ ...state, phase: 'idle' }); },
    async leave() { publish({ ...state, phase: 'ended', pendingTurnId: undefined }); },
    dispose() { listener = undefined; },
  };
  return {
    handle,
    async turnToReady(text: string): Promise<void> {
      const pendingId = state.pendingTurnId;
      if (pendingId === undefined) throw new Error('no turn in flight');
      publish({ ...state, phase: 'ready', turns: [...state.turns, { turnId: pendingId, transcript: text, state: 'review' }], pendingTurnId: pendingId });
    },
  };
}

test('voice room copy covers every phase, notice reason, and never leaks wire detail', async () => {
  const { VOICE_ROOM_NOTICE_COPY, VOICE_ROOM_PHASE_COPY } = await loadView();
  for (const reason of ['session-open-failed', 'transcription-failed', 'permission-denied', 'capture-failed', 'audio-too-large', 'scope-revoked'] as const) {
    assert.ok(VOICE_ROOM_NOTICE_COPY[reason].trim().length > 4, `${reason} 有用户文案`);
  }
  for (const phase of ['idle', 'connecting', 'listening', 'transcribing', 'ready', 'interrupted', 'ended'] as const) {
    assert.ok(VOICE_ROOM_PHASE_COPY[phase].trim().length > 0, `${phase} 有状态文案`);
  }
  assert.ok(VOICE_ROOM_NOTICE_COPY['scope-revoked'].includes('重新'), 'scope 文案引导重新进入而非暴露内部码');
});

test('controller wraps the handle: pending-turn identity never escapes, confirm routes through onConfirmIntent', async () => {
  const { createVoiceRoomController } = await loadView();
  const { handle, turnToReady } = scriptHandle();
  const submitted: Array<{ kind: string; text: string }> = [];
  const controller = createVoiceRoomController({ handle, onConfirmIntent: async (intent) => { submitted.push(intent); } });

  await controller.beginTurn();
  const ending = controller.endTurn();
  await ending;
  await turnToReady('帮我把结论整理成三段');
  assert.equal(controller.state().phase, 'ready');
  controller.editTranscript('帮我把结论整理成三段（校对）');
  await controller.confirmTranscript();
  assert.deepEqual(submitted, [{ kind: 'steer', text: '帮我把结论整理成三段（校对）' }], '确认文字经宿主 act 通道提交');
  assert.equal(controller.state().phase, 'idle');
  assert.equal(controller.state().pendingTurnId, undefined, '确认后屏面无待确认轮次');
  assert.equal(JSON.stringify(controller.state()).includes('audio'), false, '控制器状态不携带音频');
  await controller.leave();
  assert.equal(controller.state().phase, 'ended');
  controller.dispose();
});

test('controller surfaces submit failure honestly without discarding the confirmed intent', async () => {
  const { createVoiceRoomController, VOICE_ROOM_SUBMIT_ERROR_PREFIX } = await loadView();
  const { TaskOfficeError } = await import('@weknora/mobile-core');
  const { handle, turnToReady } = scriptHandle();
  const controller = createVoiceRoomController({
    handle,
    onConfirmIntent: async () => {
      throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
    },
  });
  await controller.beginTurn();
  await controller.endTurn();
  await turnToReady('一句话');
  await controller.confirmTranscript();
  assert.equal(controller.state().lastSubmitError, `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}当前部署未提供运行干预通道。`, 'act 失败以 Task Office 文案如实呈现');
  assert.equal(handle.state().turns[0]!.state, 'confirmed', '确认已发生：文字不回滚（服务端 act 可重试）');
  controller.dispose();
});

test('a failed act keeps a user-reachable retry entry: retrySubmit resubmits the same confirmed text without rollback', async () => {
  const { createVoiceRoomController } = await loadView();
  const { TaskOfficeError } = await import('@weknora/mobile-core');
  const { handle, turnToReady } = scriptHandle();
  const submitted: Array<{ kind: string; text: string }> = [];
  let attempts = 0;
  const controller = createVoiceRoomController({
    handle,
    onConfirmIntent: async (intent) => {
      attempts += 1;
      if (attempts === 1) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
      submitted.push(intent);
    },
  });
  await controller.beginTurn();
  await controller.endTurn();
  await turnToReady('重试前的那句话');
  await controller.confirmTranscript();
  assert.ok(controller.state().lastSubmitError, '首次 act 失败如实呈现');
  await controller.retrySubmit();
  assert.equal(attempts, 2, '重试真实再次调用宿主 act 通道');
  assert.deepEqual(submitted, [{ kind: 'steer', text: '重试前的那句话' }], '重试提交的是同一条已确认文字');
  assert.equal(controller.state().lastSubmitError, undefined, '重试成功后错误清除');
  assert.equal(handle.state().turns[0]!.state, 'confirmed', '重试不改轮次终态（无回滚语义不变）');
  controller.dispose();
});

test('retrySubmit without a prior failure (or after success) is a no-op', async () => {
  const { createVoiceRoomController } = await loadView();
  const { handle, turnToReady } = scriptHandle();
  let attempts = 0;
  const controller = createVoiceRoomController({ handle, onConfirmIntent: async () => { attempts += 1; } });
  await controller.retrySubmit();
  assert.equal(attempts, 0, '无失败意图时不提交');
  await controller.beginTurn();
  await controller.endTurn();
  await turnToReady('正常路径');
  await controller.confirmTranscript();
  assert.equal(attempts, 1);
  await controller.retrySubmit();
  assert.equal(attempts, 1, '成功后重试入口不再重复提交');
  controller.dispose();
});

test('the screen renders state + callbacks only, and the view module never imports wire packages', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const screenSource = readFileSync(join(here, 'screens/VoiceRoomScreen.tsx'), 'utf8');
  assert.equal(/@weknora\/(api-client|contracts)/.test(screenSource), false, 'VoiceRoomScreen 禁止导入 wire 包（module-seams §10）');
  const viewSource = readFileSync(join(here, 'voice-room-view.ts'), 'utf8');
  assert.equal(/@weknora\/(api-client|contracts)/.test(viewSource), false, 'voice-room-view 同样禁止');
  const { VoiceRoomScreen } = await import('./screens/VoiceRoomScreen.tsx');
  assert.equal(typeof VoiceRoomScreen, 'function');
});

type Element = { type?: unknown; props?: Record<string, unknown> };
const treeTexts = (node: unknown): string[] => {
  if (Array.isArray(node)) return node.flatMap(treeTexts);
  if (node === null || node === undefined || typeof node !== 'object') return typeof node === 'string' ? [node] : [];
  const props = (node as Element).props;
  if (typeof props?.children === 'string') return [props.children as string];
  return treeTexts(props?.children);
};
const treeButtons = (node: unknown): Array<{ title: unknown; onPress: unknown }> => {
  if (Array.isArray(node)) return node.flatMap(treeButtons);
  if (!node || typeof node !== 'object') return [];
  const { type, props } = node as Element;
  const here = type === 'Button' ? [{ title: props?.title, onPress: props?.onPress }] : [];
  return [...here, ...treeButtons(props?.children)];
};

test('screen render: missing-taskId deep link copy attributes the parameter, not login; login copy stays undefined-state-only', async () => {
  (globalThis as { React?: unknown }).React ??= {
    createElement: (type: unknown, props: unknown, ...children: unknown[]) => ({ type, props: { ...(props ?? {}), children: children.length === 1 ? children[0] : children } }),
  };
  const { VoiceRoomScreen } = await import('./screens/VoiceRoomScreen.tsx');
  const render = VoiceRoomScreen as (props: unknown) => unknown;
  const loginTexts = treeTexts(render({ state: undefined }));
  assert.ok(loginTexts.some((text) => text.includes('请先登录并激活空间')), 'undefined 状态保留登录兜底文案');
  const paramState = { phase: 'idle', taskId: '', turns: [], lastSubmitError: '链接缺少 taskId：语音房需从任务详情页进入。' };
  const paramTexts = treeTexts(render({ state: paramState }));
  assert.ok(paramTexts.some((text) => text.includes('缺少 taskId')), '缺参文案真实上屏');
  assert.equal(paramTexts.some((text) => text.includes('请先登录并激活空间')), false, '缺参场景不再误报登录问题');
});

test('screen render: lastSubmitError offers a retry button wired to onRetrySubmit', async () => {
  (globalThis as { React?: unknown }).React ??= {
    createElement: (type: unknown, props: unknown, ...children: unknown[]) => ({ type, props: { ...(props ?? {}), children: children.length === 1 ? children[0] : children } }),
  };
  const { VoiceRoomScreen } = await import('./screens/VoiceRoomScreen.tsx');
  const render = VoiceRoomScreen as (props: unknown) => unknown;
  let retried = 0;
  const tree = render({
    state: { phase: 'idle', taskId: 'task-1', turns: [], lastSubmitError: '指令提交失败：当前部署未提供运行干预通道。' },
    onRetrySubmit: () => { retried += 1; },
  });
  const retry = treeButtons(tree).find((button) => button.title === '重试写入任务');
  assert.ok(retry, '提交失败后呈现重试按钮');
  (retry!.onPress as () => void)();
  assert.equal(retried, 1, '重试按钮接到 onRetrySubmit 回调');
  const cleanTree = render({ state: { phase: 'idle', taskId: 'task-1', turns: [] } });
  assert.equal(treeButtons(cleanTree).some((button) => button.title === '重试写入任务'), false, '无错误时不出现重试按钮');
});

test('the voice route attributes a missing taskId deep link to the missing parameter, never to login', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const routeSource = readFileSync(join(here, 'app/tasks/voice.tsx'), 'utf8');
  assert.match(routeSource, /缺少 taskId/, '缺 taskId 深链的文案归因于缺参数');
  assert.match(routeSource, /任务详情页进入/, '文案引导从任务详情页进入');
  assert.doesNotMatch(routeSource, /state=\{undefined\}/, '缺 taskId 分支不再借 undefined 状态触发登录误报');
});

test('the voice route is an Expo Router screen wired through composition only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const routeSource = readFileSync(join(here, 'app/tasks/voice.tsx'), 'utf8');
  assert.match(routeSource, /activeVoiceRoom\(\)/, '路由经 activeVoiceRoom 取模块');
  assert.match(routeSource, /activeTaskOffice\(\)/, '确认文字经既有 Task Office act 通道');
  assert.match(routeSource, /retrySubmit/, 'act 失败的重试入口接入路由（onRetrySubmit → controller.retrySubmit）');
  assert.doesNotMatch(routeSource, /@weknora\/api-client/, '路由不直接导入 wire');
  const detailRoute = readFileSync(join(here, 'app/tasks/detail.tsx'), 'utf8');
  assert.match(detailRoute, /onOpenVoiceRoom/, '详情路由提供语音房入口');
  const detailScreen = readFileSync(join(here, 'screens/TaskDetailScreen.tsx'), 'utf8');
  assert.match(detailScreen, /onOpenVoiceRoom/, '详情屏渲染语音房入口');
});
