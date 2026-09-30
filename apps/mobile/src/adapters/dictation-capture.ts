import type { DictationAudio, DictationCapturePort } from '@weknora/mobile-core';

/**
 * 原生听写捕获 Adapter（module-seams §8.3 Audio Device Port）。system audio 属 true
 * external（spec Testing Decisions）：Node 测试链/未安装 expo-audio 的构建 fail closed
 * 返回 undefined（组合根随之隐藏麦克风入口，登录与手打输入完全不受影响）；真机麦克风
 * 验收属 #69/#70。真机构建前置：cd apps/mobile && npx expo install expo-audio。
 * expo-audio 的命令式面（SDK 55，ExpoAudio.ts）：AudioModule.AudioRecorder 构造 +
 * prepareToRecordAsync(RecordingPresets.HIGH_QUALITY) + record()/stop() + uri；
 * HIGH_QUALITY 预设输出 m4a/AAC；AudioRecorder 是 SharedObject，release() 释放。
 * #70（文件 URI 工作流客户端侧闭合）：cancel 路径的录音机临时文件由本 Adapter 即刻尽最大
 * 力删除（模块侧 DictationPorts.audioCleanup 覆盖 stop 产物的删除时点——CONTEXT.md:339）。
 */
export interface AudioRecorderLike {
  uri: string | null;
  prepareToRecordAsync(options?: unknown): Promise<void>;
  record(options?: unknown): void;
  stop(): Promise<void>;
  release?(): void;
}

export interface ExpoAudioLike {
  requestRecordingPermissionsAsync(): Promise<{ granted?: boolean }>;
  setAudioModeAsync?(mode: Record<string, unknown>): Promise<void>;
  RecordingPresets?: { HIGH_QUALITY?: unknown };
  AudioModule: { AudioRecorder: new (options?: unknown) => AudioRecorderLike };
}

/** 临时音频文件删除端口（真机：expo-file-system；Node 测试注入 fake）。 */
export interface AudioFileCleanupPort {
  deleteAsync(uri: string): Promise<void>;
}

/** 原生文件删除 Adapter（惰性 require；缺包/缺导出 fail closed → undefined）。 */
export function createNativeAudioFileCleanupIfAvailable(): AudioFileCleanupPort | undefined {
  try {
    const fileSystem = require('expo-file-system') as { deleteAsync?: (uri: string) => Promise<void> };
    if (typeof fileSystem.deleteAsync !== 'function') return undefined;
    return { deleteAsync: (uri) => fileSystem.deleteAsync!(uri) };
  } catch {
    return undefined;
  }
}

/** 注入式构造（测试注入 fake；cleanup 可选——缺省不删，行为与 #56 合并版逐字兼容）。 */
export function createDictationCaptureFrom(expoAudio: ExpoAudioLike, cleanup?: AudioFileCleanupPort): DictationCapturePort {
  let recorder: AudioRecorderLike | undefined;
  const deleteBestEffort = (uri: string | null): void => {
    if (cleanup === undefined || typeof uri !== 'string' || uri === '') return;
    void cleanup.deleteAsync(uri).catch(() => undefined); // 删除失败交还平台回收，绝不外泄
  };
  return {
    async start() {
      const permission = await expoAudio.requestRecordingPermissionsAsync();
      if (permission?.granted !== true) return 'denied';
      await expoAudio.setAudioModeAsync?.({ allowsRecording: true, playsInSilentMode: true });
      const instance = new expoAudio.AudioModule.AudioRecorder();
      await instance.prepareToRecordAsync(expoAudio.RecordingPresets?.HIGH_QUALITY);
      instance.record();
      recorder = instance;
      return 'recording';
    },
    async stop(): Promise<DictationAudio | undefined> {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return undefined;
      try {
        await instance.stop();
      } finally {
        instance.release?.();
      }
      // uri 指向的临时文件在 stop() 返回后仍须存活：模块要用它发起 multipart 上载；
      // 删除时点由模块的 audioCleanup 端口在 dropIntent 权威决定（Task 3 / #70）。
      const uri = instance.uri;
      if (typeof uri !== 'string' || uri === '') return undefined;
      return { uri, mimeType: 'audio/mp4', fileName: 'dictation.m4a' };
    },
    async cancel() {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return;
      const cancelledUri = instance.uri;
      try {
        await instance.stop();
      } catch {
        /* 已经停止的录音机：吞掉，路径仍是丢弃 */
      }
      // CONTEXT.md:339「原始音频默认在实时处理后删除」的客户端侧取消路径（#56 延迟项 → #70）：
      // 取消的音频从未进入模块，release 释放录音机对象后由 Adapter 尽最大努力删除临时文件
      // （release 不保证删盘；删除失败交还平台回收，绝不影响回 idle）。
      instance.release?.();
      deleteBestEffort(cancelledUri);
    },
  };
}

/** 原生组合路径（惰性 require；解析失败/缺导出 fail closed → undefined）。 */
export function createNativeDictationCaptureIfAvailable(): DictationCapturePort | undefined {
  try {
    const expoAudio = require('expo-audio') as ExpoAudioLike;
    if (typeof expoAudio.requestRecordingPermissionsAsync !== 'function' || expoAudio.AudioModule?.AudioRecorder === undefined) {
      return undefined;
    }
    return createDictationCaptureFrom(expoAudio, createNativeAudioFileCleanupIfAvailable());
  } catch {
    return undefined;
  }
}
