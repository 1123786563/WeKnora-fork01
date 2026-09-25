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
      // uri 指向的临时文件在 stop() 返回后仍须存活：模块要用它发起 multipart 上载；
      // 上传完成后的文件删除依赖平台回收（本适配器不显式 unlink），真机实际删除验收属 #69/#70。
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
      // CONTEXT.md:339「原始音频默认在实时处理后删除」的客户端侧：内存引用由模块 dropIntent
      // 即刻释放，此处 release 释放录音机对象。注意 release 不保证即时删除 uri 指向的临时
      // 文件（显式 unlink 需引入 expo-file-system 原生依赖，超出 #56 计划声明范围）——
      // 平台回收与真机实际删除的最终验收属 #69/#70（spec：「real-device acceptance separately」）。
      instance.release?.();
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
