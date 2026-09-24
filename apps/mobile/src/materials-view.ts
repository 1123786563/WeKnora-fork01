import { MaterialError } from '@weknora/mobile-core';
import type { MaterialActResult, MaterialErrorCode, MaterialIndex, MaterialView, TaskMaterialHandle } from '@weknora/mobile-core';

export interface MaterialsViewState {
  index?: MaterialIndex;
  view?: MaterialView;
  loading: boolean;
  error?: string;
  grant?: { name: string; url: string; expiresAt: string };
}

/** MaterialError 错误码 → 用户文案（message 即裸错误码；键类型=MaterialErrorCode，新码缺文案即类型错——B3-F55）。 */
export const MATERIAL_ERROR_COPY: Record<MaterialErrorCode, string> = {
  MATERIAL_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  MATERIAL_CLOSED: '材料视图已关闭。',
  MATERIAL_INVALID_INPUT: '材料参数缺失，请从任务详情重新进入。',
  MATERIAL_NOT_FOUND: '该材料已不存在，请刷新列表。',
  MATERIAL_GRANT_EXPIRED: '下载授权已过期，请重新获取。',
  MATERIAL_GRANT_INVALID: '下载授权校验失败，请重新获取。',
  MATERIAL_SIGNING_DISABLED: '此部署未配置签名密钥，暂无法下载（请联系管理员）。',
  MATERIAL_TERMINAL_READ_ONLY: '终端为只读，不能输入。',
  MATERIAL_GRANT_ORIGIN: '下载链接与当前部署不一致，已拒绝。',
  MATERIAL_GRANT_MISMATCH: '材料凭据校验失败，请刷新列表后重试。',
  MATERIAL_SHARE_UNAVAILABLE: '此设备暂无系统分享通道。',
  MATERIAL_BACKEND: '服务端暂时不可用，请稍后重试。',
};

const messageOf = (failure: unknown): string => {
  if (failure instanceof MaterialError) return MATERIAL_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};

export interface MaterialsController {
  state(): MaterialsViewState;
  subscribe(listener: (state: MaterialsViewState) => void): () => void;
  load(): Promise<void>;
  openMaterial(materialId: string): Promise<void>;
  openTerminal(): Promise<void>;
  openEvidence(): Promise<void>;
  download(materialId: string): Promise<void>;
  share(materialId: string): Promise<void>;
  dispose(): void;
}

/** 材料页控制器：load 驱动索引，open* 驱动视图，download/share 驱动 act；dispose 关闭句柄。 */
export function createMaterialsController(handle: TaskMaterialHandle, input: { runId: string }): MaterialsController {
  let state: MaterialsViewState = { loading: false };
  let disposed = false;
  const listeners = new Set<(state: MaterialsViewState) => void>();
  const publish = (next: MaterialsViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const run = async (action: () => Promise<MaterialsViewState>): Promise<void> => {
    publish({ ...state, loading: true, error: undefined });
    try {
      if (!disposed) publish(await action());
    } catch (failure) {
      if (!disposed) publish({ ...state, loading: false, error: messageOf(failure) });
    }
  };
  const applyGrant = (result: MaterialActResult): MaterialsViewState => {
    if (result.kind !== 'grant') return { ...state, loading: false };
    const entry = state.index?.materials.find((candidate) => candidate.materialId === result.materialId);
    return { ...state, loading: false, grant: { name: entry?.name ?? result.materialId, url: result.url, expiresAt: result.expiresAt } };
  };
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    load(): Promise<void> {
      return run(async () => ({ loading: false, view: undefined, grant: undefined, index: await handle.index({ runId: input.runId }) }));
    },
    openMaterial(materialId): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'artifact', runId: input.runId, materialId }) }));
    },
    openTerminal(): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'terminal', runId: input.runId }) }));
    },
    openEvidence(): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'evidence', runId: input.runId }) }));
    },
    download(materialId): Promise<void> {
      return run(async () => applyGrant(await handle.act({ kind: 'download', runId: input.runId, materialId })));
    },
    share(materialId): Promise<void> {
      return run(async () => applyGrant(await handle.act({ kind: 'share', runId: input.runId, materialId })));
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      listeners.clear();
      handle.close('controller-disposed');
    },
  };
}
