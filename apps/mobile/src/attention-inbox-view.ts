import { TaskOfficeError } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, AttentionInboxItem as InboxItem, InteractionActionValue, TaskOffice } from '@weknora/mobile-core';

/** Receipt 四态文案（AC2 语义锚点）：recorded 仅表示决定已记录，绝不解释为外部派发完成。 */
export const ATTENTION_RECEIPT_COPY: Record<AttentionDecisionReceipt['status'], string> = {
  recorded: '决定已记录；执行侧生效情况请以任务详情为准，不代表外部操作已完成。',
  'delivery-unknown': '决定已记录，但外部执行通道状态未知；可重试同步。',
  superseded: '该请求已更新或已由其他设备/会话处理，请刷新收件箱。',
  gone: '该请求已过期、被撤销或不存在，请刷新收件箱。',
};

export const ATTENTION_INBOX_ERROR_COPY: Record<string, string> = {
  TASK_OFFICE_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  TASK_OFFICE_INTERACTIONS_UNAVAILABLE: '当前部署未提供交互决定通道。',
  TASK_OFFICE_INVALID_INPUT: '该动作与请求类型不匹配。',
  TASK_OFFICE_BACKEND: '服务端暂时不可用，请稍后重试。',
};

export interface AttentionReceiptRow {
  key: string;
  copy: string;
}

export interface AttentionInboxViewState {
  loading: boolean;
  items?: InboxItem[];
  error?: string;
  receipts: AttentionReceiptRow[];
}

export interface AttentionInboxController {
  state(): AttentionInboxViewState;
  subscribe(listener: (state: AttentionInboxViewState) => void): () => void;
  refresh(): Promise<void>;
  decide(item: InboxItem, action: InteractionActionValue): Promise<void>;
}

const messageOf = (failure: unknown): string =>
  failure instanceof TaskOfficeError ? (ATTENTION_INBOX_ERROR_COPY[failure.code] ?? failure.code)
    : failure instanceof Error ? failure.message
      : String(failure);

/** 收件箱控制器：load 驱动首帧，decide 追加 receipt 行并自动重读（已决定行应离开 pending）。 */
export function createAttentionInboxController(office: Pick<TaskOffice, 'inbox' | 'decide'>): AttentionInboxController {
  let state: AttentionInboxViewState = { loading: true, receipts: [] };
  let disposed = false;
  const listeners = new Set<(state: AttentionInboxViewState) => void>();
  const publish = (next: AttentionInboxViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const load = (): Promise<void> => office.inbox().then(
    (view) => { if (!disposed) publish({ items: view.items, loading: false, receipts: state.receipts }); },
    (failure: unknown) => { if (!disposed) publish({ loading: false, error: messageOf(failure), receipts: state.receipts }); },
  );
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    refresh(): Promise<void> {
      publish({ ...state, loading: true, error: undefined });
      return load();
    },
    decide(item: InboxItem, action: InteractionActionValue): Promise<void> {
      return office.decide({ item, action }).then(
        async (receipt) => {
          if (!disposed) {
            // recorded 分支的 interactionId 在 record 内（receipt 联合其余三分支在顶层）。
            const interactionId = receipt.status === 'recorded' ? receipt.record.interactionId : receipt.interactionId;
            publish({
              ...state,
              receipts: [...state.receipts, { key: `${interactionId}:${state.receipts.length}`, copy: ATTENTION_RECEIPT_COPY[receipt.status] }],
            });
          }
          await load();
        },
        (failure: unknown) => { if (!disposed) publish({ ...state, loading: false, error: messageOf(failure), receipts: state.receipts }); },
      );
    },
  };
}
