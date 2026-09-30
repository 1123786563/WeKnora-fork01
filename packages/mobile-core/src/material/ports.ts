import type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterialHandle, TerminalLine,
} from './types.ts';

// ── re-export：句柄与视图类型与 Port 同文件可见，方便组合根单点导入 ──
export type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterialHandle, TerminalLine,
};

/** wire 行（api-client Adapter 产出）：消息绑定工件 + 版本身份 + 终端可用性。 */
export interface MaterialBackendArtifact {
  id: string;
  index: number;
  name: string;
  mime: string;
  version: string;
  size: number;
  sourceRun: string;
  createdAt?: string;
  digest?: string;
}

export interface MaterialBackendList {
  runId: string;
  artifacts: MaterialBackendArtifact[];
  terminalAvailable: boolean;
}

export interface MaterialBackendGrant {
  url: string;
  expiresAt: string;
  artifact: MaterialBackendArtifact;
}

export interface MaterialBackendTerminalLine {
  seq: number;
  occurredAt: string;
  stream: 'stdout' | 'stderr';
  text: string;
}

export interface MaterialBackendTerminalPage {
  lines: MaterialBackendTerminalLine[];
  /** 省略 = 末页（游标未推进）。Go 端恒发数字，适配器负责末页转译（B3-F36 合同对齐）。 */
  nextCursor?: number;
}

export interface MaterialBackendEvent {
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

/** Material Backend Port（module-seams §7.3）：WeKnora artifact/terminal-log/events Adapter 或 in-memory 场景 Adapter。 */
export interface MaterialBackendPort {
  list(runId: string): Promise<MaterialBackendList>;
  signedUrl(input: { runId: string; index: number }): Promise<MaterialBackendGrant>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<MaterialBackendTerminalPage>;
  events(runId: string): Promise<MaterialBackendEvent[]>;
}

/** 免凭据字节抓取（签名链接的兑现通道）。Adapter 必须只接受 http/https。 */
export interface BlobFetchPort {
  fetch(url: string): Promise<{ bytes: Uint8Array; mime: string }>;
}

/** 系统分享 seam（module-seams §7.3 Preview / Share Port）：native Adapter 或 scripted test Adapter。 */
export interface SharePort {
  share(input: { url: string; name: string }): Promise<void>;
}

export interface TaskMaterialPorts {
  remote: MaterialBackendPort;
  blob: BlobFetchPort;
  /** 缺省 = 无系统分享通道：act(share) fail closed（MATERIAL_SHARE_UNAVAILABLE）。 */
  share?: SharePort;
}
