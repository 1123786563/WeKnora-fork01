// MX-017 probe · 产品对话投影观察器
// frozen 场景：same-message-seq42-twice。
// 1) 真实产品 VM（createProductConversationViewModel）重复应用同一 seq42 事件 →
//    渲染消息数 1、文本「你好」（幂等 seq + 稳定 message id）；
// 2) happySyncReads：源级观察——对话四文件 import 图无任何 Happy hooks/sync API。
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { createProductConversationViewModel } from '../../../apps/mobile/sources/weknora/conversations/view-model.ts';
import { createProductScope } from '../../../apps/mobile/sources/weknora/platform/product-session.ts';
import type { ExecutionEvent } from '@weknora/contracts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  renderedMessageCount: number;
  text: string;
  happySyncReads: number;
}

const HAPPY_APIS = ['useSession(', 'useIsDataReady(', 'useSetting(', '@/sync/', 'useSessionMessages(', 'useRealtimeStatus(', 'stopRealtimeSession('];

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'same-message-seq42-twice') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const event: ExecutionEvent = {
    schema_version: 1,
    run_id: 'run-mx017',
    attempt_id: 'attempt-1',
    seq: 42,
    type: 'text.delta',
    occurred_at: '2026-09-18T08:00:00Z',
    payload: { message_id: 'm-42', text: '你好' },
  };
  const scope = createProductScope({ origin: 'https://weknora.example', userId: 'u1', tenantId: 't1' });
  const viewModel = createProductConversationViewModel({
    scope,
    spaceId: null,
    sessionId: 's-1',
    runID: 'run-mx017',
    executions: {
      start: async () => { throw new Error('not used'); },
      lookup: async () => { throw new Error('not used'); },
      command: async () => { throw new Error('not used'); },
    },
  });
  // 同一事件重复投递两次（重放/重复页）
  viewModel.applyEvents([event]);
  const state = viewModel.applyEvents([event]);

  const sources = await Promise.all([
    'apps/mobile/sources/weknora/conversations/view-model.ts',
    'apps/mobile/sources/weknora/conversations/ConversationScreen.tsx',
    'apps/mobile/sources/weknora/conversations/ProductMessageList.tsx',
    'apps/mobile/sources/weknora/conversations/MessageBlock.tsx',
  ].map((relative) => readFile(path.join(repoRoot, relative), 'utf8')));
  const happySyncReads = HAPPY_APIS.reduce((count, api) => count + sources.filter((source) => source.includes(api)).length, 0);

  return {
    renderedMessageCount: state.messages.length,
    text: state.messages[0]?.text ?? '',
    happySyncReads,
  };
}
