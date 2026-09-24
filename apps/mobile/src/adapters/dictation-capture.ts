import type { DictationAudio, DictationCapturePort } from '@weknora/mobile-core';

/**
 * 原生听写捕获 Adapter（module-seams §8.3 Audio Device Port）。system audio 属 true
 * external（spec Testing Decisions）：Node 测试链/未安装 expo-audio 的构建 fail closed
 * 返回 undefined（组合根随之隐藏麦克风入口，登录与手打输入完全不受影响）；真机麦克风
 * 验收属 #69/#70。真机构建前置：cd apps/mobile && npx expo install expo-audio。
 * expo-audio 的命令式面（SDK 55，ExpoAudio.ts）：AudioModule.AudioRecorder 构造 +
 * prepareToRecordAsync(RecordingPresets.HIGH_QUALITY) + record()/stop() + uri；
 * HIGH_QUALITY 预设输出 m4a/AAC；AudioRecorder 是 SharedObject，release() 释放。
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

/** 注入式构造（测试注入 fake；与 device-identity 的 SecureStorePort 注入同型）。 */
export function createDictationCaptureFrom(expoAudio: ExpoAudioLike): DictationCapturePort {
  let recorder: AudioRecorderLike | undefined;
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
      const uri = instance.uri;
      if (typeof uri !== 'string' || uri === '') return undefined;
      return { uri, mimeType: 'audio/mp4', fileName: 'dictation.m4a' };
    },
    async cancel() {
      const instance = recorder;
      recorder = undefined;
      if (instance === undefined) return;
      try {
        await instance.stop();
      } catch {
        /* 已经停止的录音机：吞掉，路径仍是丢弃 */
      }
      instance.release?.(); // 原始音频默认删除（CONTEXT.md:339）：释放即弃临时文件
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
    return createDictationCaptureFrom(expoAudio);
  } catch {
    return undefined;
  }
}
