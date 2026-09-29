/**
 * Offline Gate（design spec Implementation Decisions："Offline mode permits approved
 * reads, drafts and annotations. It prohibits Run commands, approval, budget expansion
 * and external Actions."）：离线危险动作在派发前拒绝的结构化判决。
 *
 * 判定语义（AC2 fail closed）：
 * - 状态通道探测失败（抛错）一律视为离线；
 * - 平台无状态通道（status === undefined，如 Node 测试链/未装 expo-network）时透传放行：
 *   物理离线的危险动作由传输层必然失败兜底，fail closed 不变；gate 提供的是提前、
 *   结构化（OFFLINE_ACTION_BLOCKED:kind）的拒绝，而非唯一防线。
 */
export interface NetworkStatusPort {
  online(): Promise<boolean>;
}

export type OfflineActionKind = 'run' | 'approval' | 'budget' | 'external-action';

export const OFFLINE_ACTION_BLOCKED = 'OFFLINE_ACTION_BLOCKED';

export class OfflineGateError extends Error {
  readonly code = OFFLINE_ACTION_BLOCKED;
  constructor(readonly action: OfflineActionKind) {
    super(`${OFFLINE_ACTION_BLOCKED}:${action}`);
  }
}

export interface OfflineGate {
  status(): Promise<'online' | 'offline'>;
  assertOnline(action: OfflineActionKind): Promise<void>;
}

export function createOfflineGate(status: NetworkStatusPort | undefined): OfflineGate {
  const probe = async (): Promise<boolean> => {
    if (status === undefined) return true;
    try {
      return await status.online();
    } catch {
      return false; // fail closed：探测失败 = 离线
    }
  };
  return {
    async status() {
      return (await probe()) ? 'online' : 'offline';
    },
    async assertOnline(action) {
      if (!(await probe())) throw new OfflineGateError(action);
    },
  };
}
