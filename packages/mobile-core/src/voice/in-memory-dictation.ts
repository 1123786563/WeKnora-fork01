import type { DictationAudio, DictationCapturePort, DictationTranscriptionPort } from './dictation.ts';

export interface ScriptedDictationCaptureOptions {
  /** start() 的脚本化结果：'recording'（默认）/ 'denied'（权限拒绝）/ Error（捕获失败）。 */
  start?: 'recording' | 'denied' | Error;
  /** stop() 的脚本化结果：音频 / undefined（无可用音频，默认）/ Error。 */
  stop?: DictationAudio | undefined | Error;
  /** true 时 stop() 挂起直至测试调用 resolveStop()（停止/取消竞态用）。 */
  manualStop?: boolean;
}

export interface ScriptedDictationCapture extends DictationCapturePort {
  calls: string[];
  resolveStop(result: DictationAudio | undefined | Error): void;
}

/** 听写捕获的 scripted test Adapter（spec：system audio 以 scripted Adapter 进 Interface 测试）。 */
export function createScriptedDictationCapture(options: ScriptedDictationCaptureOptions = {}): ScriptedDictationCapture {
  const startScript = options.start ?? 'recording';
  const calls: string[] = [];
  let releaseStop: ((result: DictationAudio | undefined | Error) => void) | undefined;
  return {
    calls,
    async start() {
      calls.push('start');
      if (startScript instanceof Error) throw startScript;
      return startScript;
    },
    async stop() {
      calls.push('stop');
      if (options.manualStop === true) {
        return new Promise<DictationAudio | undefined>((resolve, reject) => {
          releaseStop = (result) => { if (result instanceof Error) reject(result); else resolve(result); };
        });
      }
      const scripted = options.stop;
      if (scripted instanceof Error) throw scripted;
      return scripted;
    },
    async cancel() { calls.push('cancel'); },
    resolveStop(result) {
      const release = releaseStop;
      releaseStop = undefined;
      if (release !== undefined) release(result);
    },
  };
}

export interface ScenarioDictationTranscriber extends DictationTranscriptionPort {
  requests: Array<{ requestId: string; audio: DictationAudio }>;
  /** 手动控制转写应答时机（迟到结果/失败重试测试）。 */
  resolve(index: number, result: { text: string; audioSeconds?: number } | Error): void;
  pending(): number;
}

/** 听写转写的手动场景 Adapter：每次 dispatch 记入 requests，由测试按 index 应答。 */
export function createScenarioDictationTranscriber(): ScenarioDictationTranscriber {
  const requests: Array<{ requestId: string; audio: DictationAudio }> = [];
  const resolvers: Array<(result: { text: string; audioSeconds?: number } | Error) => void> = [];
  return {
    requests,
    transcribe(input) {
      requests.push({ requestId: input.requestId, audio: input.audio });
      return new Promise((resolve, reject) => {
        resolvers.push((result) => { if (result instanceof Error) reject(result); else resolve(result); });
      });
    },
    resolve(index, result) {
      const resolve = resolvers[index];
      if (resolve !== undefined) resolve(result);
    },
    pending: () => resolvers.length,
  };
}
