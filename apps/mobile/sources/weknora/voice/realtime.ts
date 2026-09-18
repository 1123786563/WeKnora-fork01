/**
 * 实时语音会话域逻辑（MX-029 / voice）。
 * 冻结规则：
 * - 媒体会话经产品 API 授权与限额（**短期媒体令牌**——不下发长期服务密钥；客户端不持 Provider 密钥）；
 * - 媒体会话与产品 Run 绑定（voice session mapping）；账务沿原商业入口（语音不另起钱包）；
 * - 三种中断互不误触发：interrupt_output（停止播报——只停媒体输出）、end_session（结束语音会话——
 *   停令牌与流）、cancel_run（取消任务——走任务取消命令）；frozen：interrupt 不触发任何 run 取消；
 * - 语音结束后迟到用量仍能结算（用量上报与会话结束解耦）；
 * - 不可语音绕过危险写审批（审批交互与语音通道互斥——voice 会话不携带决定权）。
 */

export type VoiceStopAction = 'interrupt_output' | 'end_session' | 'cancel_run';

export interface VoiceSessionState {
  sessionID: string;
  runID: string;
  /** 短期媒体令牌（产品 API 签发；过期需重授权——客户端不保存长期密钥） */
  mediaToken: { value: string; expiresAt: string };
  outputActive: boolean;
  sessionActive: boolean;
}

export interface VoiceSessionPorts {
  /** 产品 API：授权语音会话（短期令牌+限额）。 */
  authorize(runID: string): Promise<VoiceSessionState>;
  /** 停止媒体输出（仅播报）。 */
  stopOutput(session: VoiceSessionState): Promise<void>;
  /** 结束语音会话（令牌失效+流关闭）。 */
  endSession(session: VoiceSessionState): Promise<void>;
  /** 任务取消命令通道（复用 MX-018 语义——独立动作）。 */
  cancelRun(runID: string, expectedRevision: number): Promise<void>;
  /** 迟到用量上报（结算解耦——语音结束后仍可收到并转发结算）。 */
  reportUsage(sessionID: string, units: number): Promise<void>;
}

export function createVoiceSessionController(ports: VoiceSessionPorts) {
  let session: VoiceSessionState | null = null;
  let runCancelCount = 0;

  return {
    runCancelObservations(): number {
      return runCancelCount;
    },
    async start(runID: string): Promise<VoiceSessionState> {
      session = await ports.authorize(runID);
      return session;
    },
    /**
     * 三种停止语义（互不误触发——frozen：interrupt_output 时 runCancelCount 保持 0）：
     * - interrupt_output：停止播报（媒体输出），会话与任务不受影响；
     * - end_session：结束语音会话（令牌失效）；任务不受影响；
     * - cancel_run：取消任务（独立命令通道）；语音会话随后自然终止。
     */
    async stop(action: VoiceStopAction, context: { expectedRevision: number }): Promise<{ mediaOutputStopped: boolean; sessionEnded: boolean; runCancelled: boolean }> {
      if (!session) throw new Error('VOICE_SESSION_NOT_STARTED');
      switch (action) {
        case 'interrupt_output': {
          await ports.stopOutput(session);
          session = { ...session, outputActive: false };
          return { mediaOutputStopped: true, sessionEnded: false, runCancelled: false };
        }
        case 'end_session': {
          await ports.endSession(session);
          const ended = session;
          session = null;
          void ended;
          return { mediaOutputStopped: true, sessionEnded: true, runCancelled: false };
        }
        case 'cancel_run': {
          runCancelCount += 1;
          await ports.cancelRun(session.runID, context.expectedRevision);
          await ports.endSession(session);
          session = null;
          return { mediaOutputStopped: true, sessionEnded: true, runCancelled: true };
        }
      }
    },
    /**
     * 迟到用量：会话结束后的用量仍转发结算（不因会话已结束而丢弃）。
     */
    async handleLateUsage(sessionID: string, units: number): Promise<void> {
      await ports.reportUsage(sessionID, units);
    },
    /** 审批互斥：语音会话不携带决定权（危险写审批只在审批 UI/命令通道）。 */
    canApproveFromVoice(): boolean {
      return false;
    },
  };
}

export type VoiceSessionController = ReturnType<typeof createVoiceSessionController>;
