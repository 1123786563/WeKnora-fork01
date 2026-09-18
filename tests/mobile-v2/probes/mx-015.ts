// MX-015 probe · 附件未就绪提交观察器
// frozen 场景：valid-draft-scanning-attachment。
// 真实 createTaskForm + evaluateSubmitReadiness/toStartInput：
// 有效草稿 + 扫描中附件 → 提交被阻止（startCount=0）、草稿原样保留；
// 附件就绪后提交派发，StartInput 严格 7 字段（附件绝不进入 body）。
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  createTaskForm,
  evaluateSubmitReadiness,
  toStartInput,
  type NewTaskDraft,
} from '../../../packages/domain/src/mobile/task-form.ts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  startCount: number;
  draft: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'valid-draft-scanning-attachment') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const draft: NewTaskDraft = {
    text: '保留这段文本',
    agentId: 'general',
    budgetUpper: 100,
    attachments: [{ id: 'file-1', name: '需求.pdf', readiness: 'scanning' }],
    knowledgeIds: [],
  };

  // frozen 场景自身的观测计数（未就绪提交必须为 0）
  let startCount = 0;
  const savedDrafts: NewTaskDraft[] = [];
  const form = createTaskForm(draft, {
    submit: async () => {
      startCount += 1;
      return { dispatched: true };
    },
    saveDraft: async (value) => { savedDrafts.push(value); },
    prepareSessionRefs: async () => { /* 会话准备通道：附件引用在此解析（真实 API 由宿主接） */ },
  });

  // 未就绪提交：零网络（start 不被调用）、草稿保留
  const blocked = await form.attemptSubmit({ requestID: 'req-mx015', sessionId: 's-1', targetId: 'platform', workspaceRef: 'ws-1' });
  if (blocked.submitted || blocked.readiness.reason !== 'attachments_not_ready') {
    throw new Error(`scanning attachment must block submit, got ${JSON.stringify(blocked.readiness)}`);
  }
  if (startCount !== 0) throw new Error('no start request may fire while attachments are not ready');
  const preserved = form.draft;
  if (preserved.text !== '保留这段文本' || preserved.attachments.length !== 1) {
    throw new Error('draft must be preserved verbatim on blocked submit');
  }

  // 附件就绪后提交：派发一次，StartInput 严格 7 字段（无附件/知识字段）
  form.update({});
  const readyDraft: NewTaskDraft = { ...draft, attachments: [{ id: 'file-1', name: '需求.pdf', readiness: 'ready' }] };
  // 附加验证（不计入 frozen 观测）：就绪草稿在独立表单上提交一次
  const readySubmitted: string[] = [];
  const readyForm = createTaskForm(readyDraft, {
    submit: async (startInput) => {
      readySubmitted.push(JSON.stringify(startInput));
      return { dispatched: true };
    },
    saveDraft: async () => undefined,
  });
  const submitted = await readyForm.attemptSubmit({ requestID: 'req-mx015', sessionId: 's-1', targetId: 'platform', workspaceRef: 'ws-1' });
  if (!submitted.submitted) throw new Error('ready draft must submit');
  const keys = Object.keys(JSON.parse(readySubmitted[0]!) as Record<string, unknown>).sort();
  if (keys.length !== 7 || keys.join(',') !== 'agent_id,budget_upper,request_id,session_id,target_id,text,workspace_ref') {
    throw new Error(`StartInput must stay exactly the frozen 7 fields, got ${keys.join(',')}`);
  }
  // 纯函数复验：就绪裁决与输入构造一致
  if (!evaluateSubmitReadiness(readyDraft).ready) throw new Error('readiness mismatch');
  toStartInput(readyDraft, { requestID: 'req-mx015', sessionId: 's-1', targetId: 'platform', workspaceRef: 'ws-1' });

  // 源级：NewTaskScreen 无直接 fetch / 不发明 request_id
  const screen = await readFile(path.join(repoRoot, 'apps/mobile/sources/weknora/screens/NewTaskScreen.tsx'), 'utf8');
  if (screen.includes('fetch(') || screen.includes('request_id')) {
    throw new Error('screen must not fetch or fabricate request ids');
  }
  void savedDrafts;

  return { startCount, draft: preserved.text };
}
