/**
 * Production DictationPort (W29): expo-audio capture with permission
 * gating and temporary-file cleanup.
 *
 * The app already ships `expo-audio` (SDK 55; `expo-av` is deprecated —
 * see sources/utils/microphonePermissions.ts for the same precedent), so
 * the port is the expo-audio `AudioRecorder` driven directly:
 * `start` requests the microphone permission (a denial raises the typed
 * MIC_PERMISSION_DENIED the controller maps to the text-input fallback),
 * prepares the recorder and begins capturing; `stop` finalizes the file
 * and reports `{uri, durationMs}`; `cancel` stops and deletes the
 * temporary file.
 *
 * `transcribe` is the W30 consumption point: the product-authenticated,
 * budget-metered server-side transcription endpoint. It is injected so
 * W30 plugs in an authSession-transport-backed call without touching
 * this port; until then it fails with a typed error instead of calling
 * an endpoint that does not exist. The client never holds a long-lived
 * model key — auth and budget stay server-side behind the product
 * authSession.
 *
 * Device-level behaviour (real permission prompts, call interruptions,
 * audio hardware) is blocked-env manual evidence; this adapter only
 * translates the recorder into DictationPort values.
 */
import { AudioModule, RecordingPresets, requestRecordingPermissionsAsync, setAudioModeAsync } from 'expo-audio';
import type { AudioRecorder } from 'expo-audio';
import { File } from 'expo-file-system';
import { DictationError, type DictationPort } from './dictation';

/** W30 consumption point: the authenticated transcription call over the product authSession. */
export type DictationTranscriber = (uri: string) => Promise<string>;

export interface ProductDictationPortInput {
  transcribe?: DictationTranscriber;
}

export function createProductDictationPort(input: ProductDictationPortInput = {}): DictationPort {
  let recorder: AudioRecorder | null = null;
  /** The finalized-but-not-yet-cleaned capture of the current session. */
  let pendingUri: string | null = null;

  const deleteFile = (uri: string | null | undefined) => {
    if (!uri) return;
    try {
      const file = new File(uri);
      if (file.exists) file.delete();
    } catch {
      // Temporary-cache cleanup is best-effort; the OS purges the cache dir.
    }
  };

  return {
    async start(): Promise<void> {
      const permission = await requestRecordingPermissionsAsync();
      if (!permission.granted) throw new DictationError('MIC_PERMISSION_DENIED');
      // Matching the app's existing audio-mode precedent so recording works
      // while the device is in silent mode.
      await setAudioModeAsync({ allowsRecording: true, playsInSilentMode: true });
      // A capture abandoned by a superseded flow is removed before a new hold.
      deleteFile(pendingUri);
      pendingUri = null;
      const active = new AudioModule.AudioRecorder({ ...RecordingPresets.HIGH_QUALITY, isMeteringEnabled: false });
      recorder = active;
      await active.prepareToRecordAsync();
      active.record();
    },
    async stop(): Promise<{ uri: string; durationMs: number }> {
      const active = recorder;
      if (!active) throw new DictationError('RECORD_INTERRUPTED', 'DICTATION_NOT_RECORDING');
      await active.stop();
      const uri = active.uri;
      if (uri) pendingUri = uri;
      return { uri: uri ?? '', durationMs: active.getStatus().durationMillis };
    },
    async cancel(): Promise<void> {
      // Grab-and-clear synchronously first: a late cancel arriving from a
      // superseded controller flow can never delete a newer session's file.
      const active = recorder;
      recorder = null;
      const stale = pendingUri;
      pendingUri = null;
      if (active?.isRecording) {
        try { await active.stop(); } catch { /* already stopped or reset */ }
      }
      deleteFile(stale);
      deleteFile(active?.uri);
    },
    async transcribe(uri: string): Promise<string> {
      if (!input.transcribe) throw new DictationError('TRANSCRIBE_FAILED', 'TRANSCRIBE_ENDPOINT_PENDING_W30');
      return input.transcribe(uri);
    },
  };
}
