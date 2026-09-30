import type { VoiceSessionEndReceipt, VoiceSessionGrant, VoiceSessionPort } from './voice-room.ts';

export interface ScriptedVoiceSession extends VoiceSessionPort {
  opens: Array<{ productSessionId: string; runId?: string; maxSeconds?: number }>;
  ends: string[];
  setOpenResult(result: Omit<VoiceSessionGrant, 'id'> | Error): void;
  setEndResult(result: Omit<VoiceSessionEndReceipt, 'id'> | Error): void;
}

/** 语音会话的 scripted test Adapter（spec §8.3：Realtime Voice Port 以 scripted Adapter 进
 * Interface 测试）。每次 open 产出一个新的会话 id（与服务端行为一致——每次授权新行），
 * end 回显被结束的会话 id。 */
export function createScriptedVoiceSession(): ScriptedVoiceSession {
  const opens: ScriptedVoiceSession['opens'] = [];
  const ends: string[] = [];
  let openResult: Omit<VoiceSessionGrant, 'id'> | Error = { expiresAt: '2026-09-26T00:10:00.000Z', maxSeconds: 600 };
  let endResult: Omit<VoiceSessionEndReceipt, 'id'> | Error = { state: 'closed', settled: true };
  return {
    opens,
    ends,
    async open(input) {
      opens.push(input);
      if (openResult instanceof Error) throw openResult;
      return { ...openResult, id: `vs-scripted-${opens.length}` };
    },
    async end(sessionId) {
      ends.push(sessionId);
      if (endResult instanceof Error) throw endResult;
      return { ...endResult, id: sessionId };
    },
    setOpenResult(result) {
      openResult = result;
    },
    setEndResult(result) {
      endResult = result;
    },
  };
}
