/**
 * Dictation 深模块（Issue #56 / T26）——module-seams §8.1「Voice Room 拥有……转写草稿」
 * 的听写子面。所有权：录音生命周期、麦克风权限结果、转写草稿（可编辑）与迟到结果丢弃。
 * 不拥有：Task 权限、提交、审批、时间线（AC2 的结构性保障——本模块没有任何提交方法）。
 *
 * 语义要点：
 *   - AC1：cancel() 在任何阶段都只回到 idle——丢音频、丢在途转写结果、绝不触发转写派发，
 *     也绝不触碰宿主（New 屏）的文字草稿；Task 与否只由宿主的既有提交通道决定。
 *   - AC2：转写结果只进入 review（可编辑）；confirmTranscript() 返回确认文本后由宿主写入
 *     目标输入；提交仍是 #36 的显式 Submit。
 *   - 幂等：一次 finish 铸造一个 requestId；失败后 retryTranscription() 复用同一 id
 *     （服务端从结果行幂等重放，mobile_voice.go:465-487）。
 *   - 迟到丢弃：代次守卫——cancel/begin 推进 generation，任何旧代次异步结果整代忽略。
 *   - 原始音频：仅存活于 dispatch 前后（失败重试窗口），成功/取消/丢弃即释放（CONTEXT.md:339）。
 */

/** 服务端单次转写捕获上限（internal/handler/mobile_voice.go:51 voiceMaxAudioBytes = 16 << 20）。 */
export const DICTATION_MAX_AUDIO_BYTES = 16 << 20;
/** 服务端单次转写预算窗（mobile_voice.go:49 voiceTranscribeMaxSeconds = 120）。 */
export const DICTATION_MAX_DURATION_MS = 120_000;

/** 一次捕获的音频源：uri（RN 原生文件源）与 bytes（Node/集成冒烟）恰有其一。 */
export interface DictationAudio {
  uri?: string;
  bytes?: Uint8Array;
  mimeType: string;
  fileName?: string;
}

export type DictationCaptureStart = 'recording' | 'denied';

/** 麦克风捕获 Port（module-seams §8.3 Audio Device Port）：原生 Adapter 与 test Adapter 共用。 */
export interface DictationCapturePort {
  /** 请求麦克风权限并开始捕获；'denied' 表示权限拒绝（零捕获）。其余失败以异常上抛（→ capture-failed）。 */
  start(): Promise<DictationCaptureStart>;
  /** 停止并取回捕获；无可用音频返回 undefined。 */
  stop(): Promise<DictationAudio | undefined>;
  /** 停止并丢弃音频（AC1 取消路径；原生 Adapter 同时释放临时文件）。 */
  cancel(): Promise<void>;
}

export interface DictationTranscriptionInput {
  requestId: string;
  audio: DictationAudio;
}

export interface DictationTranscriptionResult {
  text: string;
  audioSeconds?: number;
}

/** 服务端转写代理 Port（wire 适配在 api-client，经 Runtime 授权通道）。 */
export interface DictationTranscriptionPort {
  transcribe(input: DictationTranscriptionInput): Promise<DictationTranscriptionResult>;
}

export type DictationPhase = 'idle' | 'recording' | 'transcribing' | 'review' | 'denied' | 'failed';
export type DictationFailure = 'capture-failed' | 'audio-too-large' | 'transcription-failed';

export interface DictationState {
  phase: DictationPhase;
  /** review 阶段的可编辑转写草稿（AC2）。 */
  transcript?: string;
  failure?: DictationFailure;
  /** 适配器级错误码（如 VOICE_CHARGING_UNCONFIGURED），仅用于呈现与证据，不参与分支。 */
  failureCode?: string;
}

export interface DictationPorts {
  capture: DictationCapturePort;
  transcribe: DictationTranscriptionPort;
  newRequestId(): string;
  /** 录音时长上限（毫秒）；缺省 DICTATION_MAX_DURATION_MS。到点自动 finish。 */
  maxDurationMs?: number;
  /** 原始音频删除端口（module-seams §8.1 + CONTEXT.md:339「原始音频默认在实时处理后删除」；
   *  #56 延迟项由 #70 闭合）：模块在 dropIntent（音频不再需要的唯一权威时点）对 uri 源音频
   *  尽最大努力删除一次；bytes 源在内存中不调用。缺省不删除（Node/集成冒烟场景）。 */
  audioCleanup?: (audio: DictationAudio) => Promise<void>;
}

export interface Dictation {
  state(): DictationState;
  subscribe(listener: (state: DictationState) => void): () => void;
  /** 开始录音（先请求权限；denied 只影响听写可用性，从不影响宿主文字草稿）。 */
  begin(): Promise<void>;
  /** 停止录音并派发转写；结果进入 review，不写宿主。 */
  finish(): Promise<void>;
  /** AC1：取消——任何阶段回 idle，丢音频/丢在途结果，零转写派发，宿主不受影响。 */
  cancel(): Promise<void>;
  /** 转写失败后显式重试：复用同一 requestId 与同一段音频（幂等重放）。 */
  retryTranscription(): Promise<void>;
  /** review 中编辑转写文本。 */
  editTranscript(text: string): void;
  /** AC2：确认转写——返回（可能已编辑的）文本，模块回 idle；写入宿主由调用方承担。空白返回 undefined 并保持 review。 */
  confirmTranscript(): string | undefined;
  /** 放弃本次转写（review → idle）。 */
  discardTranscript(): void;
  dispose(): void;
}

function failureCodeOf(error: unknown): string | undefined {
  if (error instanceof Error && 'code' in error) {
    const code = (error as { code?: unknown }).code;
    if (typeof code === 'string' && code.trim() !== '') return code;
  }
  return undefined;
}

export function createDictation(ports: DictationPorts): Dictation {
  const maxDurationMs = ports.maxDurationMs ?? DICTATION_MAX_DURATION_MS;
  let state: DictationState = { phase: 'idle' };
  let generation = 0;
  let beginning = false;
  let requestId: string | undefined;
  let pendingAudio: DictationAudio | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const listeners = new Set<(state: DictationState) => void>();
  const publish = (next: DictationState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const clearTimer = (): void => {
    if (timer !== undefined) { clearTimeout(timer); timer = undefined; }
  };
  const dropIntent = (): void => {
    requestId = undefined;
    const audio = pendingAudio;
    pendingAudio = undefined; // 原始音频即刻释放（失败重试窗口结束）
    if (audio?.uri !== undefined && ports.audioCleanup !== undefined) {
      void ports.audioCleanup(audio).catch(() => undefined); // 尽最大努力：清理失败不外泄、不阻塞主流程
    }
  };

  async function dispatchTranscription(attemptGeneration: number, id: string, audio: DictationAudio): Promise<void> {
    try {
      const result = await ports.transcribe.transcribe({ requestId: id, audio });
      if (attemptGeneration !== generation) return; // 迟到结果：整代丢弃
      if (typeof result.text !== 'string' || result.text.trim() === '') {
        dropIntent(); // 空转写无重试价值（同 id 重放仍为空）
        publish({ phase: 'failed', failure: 'transcription-failed' });
        return;
      }
      dropIntent();
      publish({ phase: 'review', transcript: result.text });
    } catch (error) {
      if (attemptGeneration !== generation) return; // 迟到失败同样丢弃
      const code = failureCodeOf(error);
      // 保留 requestId/pendingAudio：失败后的显式重试复用同一幂等身份
      publish({ phase: 'failed', failure: 'transcription-failed', ...(code === undefined ? {} : { failureCode: code }) });
    }
  }

  const dictation: Dictation = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    async begin() {
      if (beginning || state.phase === 'recording' || state.phase === 'transcribing') return;
      beginning = true;
      const attemptGeneration = generation;
      try {
        clearTimer();
        dropIntent();
        let started: DictationCaptureStart;
        try {
          started = await ports.capture.start();
        } catch {
          if (attemptGeneration === generation) publish({ phase: 'failed', failure: 'capture-failed' });
          return;
        }
        if (attemptGeneration !== generation) {
          // begin 在途时被取消：丢弃本次开始（best-effort 停捕获）
          try { await ports.capture.cancel(); } catch { /* 已取消路径不外泄 */ }
          return;
        }
        if (started === 'denied') {
          publish({ phase: 'denied' });
          return;
        }
        publish({ phase: 'recording' });
        timer = setTimeout(() => {
          timer = undefined;
          void dictation.finish(); // 到点自动停止（录音不越过服务端 120s 预算窗）
        }, maxDurationMs);
      } finally {
        beginning = false;
      }
    },
    async finish() {
      if (state.phase !== 'recording') return;
      clearTimer();
      const attemptGeneration = generation;
      let audio: DictationAudio | undefined;
      try {
        audio = await ports.capture.stop();
      } catch {
        audio = undefined;
      }
      if (attemptGeneration !== generation) {
        dropIntent(); // stop 在途时已取消：丢弃，不转写
        return;
      }
      if (audio === undefined || (audio.bytes === undefined && audio.uri === undefined)) {
        dropIntent();
        publish({ phase: 'failed', failure: 'capture-failed' });
        return;
      }
      if (audio.bytes !== undefined && audio.bytes.byteLength > DICTATION_MAX_AUDIO_BYTES) {
        dropIntent();
        publish({ phase: 'failed', failure: 'audio-too-large' });
        return;
      }
      const id = ports.newRequestId();
      requestId = id;
      pendingAudio = audio;
      publish({ phase: 'transcribing' });
      await dispatchTranscription(attemptGeneration, id, audio);
    },
    async cancel() {
      generation += 1; // 任何在途异步（start/stop/转写）迟到即弃
      clearTimer();
      const wasRecording = state.phase === 'recording';
      dropIntent();
      publish({ phase: 'idle' }); // 先回 idle（UI 立即恢复），再收尾捕获
      if (wasRecording) {
        try { await ports.capture.cancel(); } catch { /* 取消失败不影响回 idle */ }
      }
    },
    async retryTranscription() {
      if (state.phase !== 'failed' || state.failure !== 'transcription-failed' || requestId === undefined || pendingAudio === undefined) return;
      const attemptGeneration = generation;
      publish({ phase: 'transcribing' });
      await dispatchTranscription(attemptGeneration, requestId, pendingAudio);
    },
    editTranscript(text) {
      if (state.phase !== 'review') return;
      publish({ ...state, transcript: text });
    },
    confirmTranscript() {
      if (state.phase !== 'review' || typeof state.transcript !== 'string') return undefined;
      const text = state.transcript.trim();
      if (text === '') return undefined;
      publish({ phase: 'idle' });
      return text;
    },
    discardTranscript() {
      if (state.phase !== 'review') return;
      publish({ phase: 'idle' });
    },
    dispose() {
      generation += 1;
      clearTimer();
      dropIntent();
      const wasRecording = state.phase === 'recording';
      listeners.clear();
      if (wasRecording) void ports.capture.cancel().catch(() => undefined);
      state = { phase: 'idle' };
    },
  };
  return dictation;
}
