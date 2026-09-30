import type { MaterialBackendPort, SharePort, BlobFetchPort, MaterialBackendList, MaterialBackendGrant, MaterialBackendTerminalPage, MaterialBackendEvent } from './ports.ts';

export interface ScenarioMaterialHandlers {
  list?(runId: string): Promise<MaterialBackendList> | MaterialBackendList;
  signedUrl?(input: { runId: string; index: number }): Promise<MaterialBackendGrant> | MaterialBackendGrant;
  terminalLog?(input: { runId: string; after: number; limit: number }): Promise<MaterialBackendTerminalPage> | MaterialBackendTerminalPage;
  events?(runId: string): Promise<MaterialBackendEvent[]> | MaterialBackendEvent[];
}

/** in-memory 场景 Adapter（module-seams §12「remote but owned」的测试面）；未编排的调用大声失败，不静默答空。 */
export function createScenarioMaterialRemote(handlers: ScenarioMaterialHandlers = {}): MaterialBackendPort & {
  listCalls: string[];
  grantCalls: Array<{ runId: string; index: number }>;
  terminalCalls: Array<{ runId: string; after: number; limit: number }>;
  eventCalls: string[];
} {
  const listCalls: string[] = [];
  const grantCalls: Array<{ runId: string; index: number }> = [];
  const terminalCalls: Array<{ runId: string; after: number; limit: number }> = [];
  const eventCalls: string[] = [];
  return {
    listCalls, grantCalls, terminalCalls, eventCalls,
    async list(runId) {
      listCalls.push(runId);
      if (handlers.list === undefined) throw new Error(`scenario material list not scripted for ${runId}`);
      return handlers.list(runId);
    },
    async signedUrl(input) {
      grantCalls.push(input);
      if (handlers.signedUrl === undefined) throw new Error(`scenario material grant not scripted for ${input.runId}#${input.index}`);
      return handlers.signedUrl(input);
    },
    async terminalLog(input) {
      terminalCalls.push(input);
      if (handlers.terminalLog === undefined) throw new Error(`scenario terminal log not scripted for ${input.runId}`);
      return handlers.terminalLog(input);
    },
    async events(runId) {
      eventCalls.push(runId);
      if (handlers.events === undefined) throw new Error(`scenario material events not scripted for ${runId}`);
      return handlers.events(runId);
    },
  };
}

/** 按完整 URL 编排的字节源：未编排的 URL 拒绝（不编造字节）。 */
export function createScriptedBlobFetch(script: Record<string, { bytes: Uint8Array; mime: string } | { error: unknown }>): BlobFetchPort & { calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    async fetch(url) {
      calls.push(url);
      const entry = script[url];
      if (entry === undefined) throw new Error(`scenario blob not scripted for ${url}`);
      if ('error' in entry) throw entry.error;
      return entry;
    },
  };
}

/** 记录型分享 Adapter（系统分享 seam 的 scripted test Adapter）。 */
export function createRecordingSharePort(): SharePort & { shared: Array<{ url: string; name: string }> } {
  const shared: Array<{ url: string; name: string }> = [];
  return { shared, async share(input) { shared.push(input); } };
}
