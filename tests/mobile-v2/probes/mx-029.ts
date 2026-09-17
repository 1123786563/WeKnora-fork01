// MX-029 probe · 语音独立中断语义观察器
// frozen 场景：active-voice-active-run × interrupt-output。
// 真实 createVoiceSessionController：停止播报只停媒体输出（runCancelCount=0——不触发任务取消）；
// 迟到用量在会话结束后仍结算；语音无审批权。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createVoiceSessionController, type VoiceSessionPorts } from '../../../apps/mobile/sources/weknora/voice/realtime.ts';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  mediaOutputStopped: boolean;
  runCancelCount: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'active-voice-active-run' || input.fault !== 'interrupt-output') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  // 服务端：短期令牌/三停/迟到用量（真实 Go 测试）
  const go = await execFileAsync('go', ['test', './internal/application/service/workbench/', '-run', 'TestMX029VoiceSessionLifecycle', '-count=1', '-v'], { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 });
  if ((go.stdout + go.stderr).includes('FAIL')) throw new Error('go voice lifecycle test failed');

  // 客户端：interrupt_output 不触发任务取消
  let outputStops = 0;
  let sessionEnds = 0;
  let runCancels = 0;
  let usageReports = 0;
  const ports: VoiceSessionPorts = {
    authorize: async (runID) => ({
      sessionID: 'vs-1', runID, mediaToken: { value: 'short-lived', expiresAt: '2099-01-01T00:00:00Z' }, outputActive: true, sessionActive: true,
    }),
    stopOutput: async () => { outputStops += 1; },
    endSession: async () => { sessionEnds += 1; },
    cancelRun: async () => { runCancels += 1; },
    reportUsage: async () => { usageReports += 1; },
  };
  const controller = createVoiceSessionController(ports);
  const session = await controller.start('run-1');

  // frozen：停止播报——媒体输出停止，任务取消计数 0
  const interrupted = await controller.stop('interrupt_output', { expectedRevision: 4 });
  if (!interrupted.mediaOutputStopped || interrupted.sessionEnded || interrupted.runCancelled) {
    throw new Error(`interrupt_output must only stop output, got ${JSON.stringify(interrupted)}`);
  }
  if (controller.runCancelObservations() !== 0) throw new Error('interrupt must never cancel the run');
  void session;

  // 对照：结束语音≠取消任务（runCancels 仍 0）
  const ended = await controller.stop('end_session', { expectedRevision: 4 });
  if (!ended.sessionEnded || ended.runCancelled) throw new Error('end_session must not cancel the run');
  // 取消任务：独立通道（计数 +1）
  const controller2 = createVoiceSessionController(ports);
  await controller2.start('run-1');
  const cancelled = await controller2.stop('cancel_run', { expectedRevision: 4 });
  if (!cancelled.runCancelled || controller2.runCancelObservations() !== 1) throw new Error('cancel_run must use the command channel exactly once');

  // 迟到用量：会话结束后仍转发结算
  await controller2.handleLateUsage('vs-1', 80);
  if (usageReports !== 1) throw new Error('late usage must still be reported for settlement');
  // 语音无审批权
  if (controller.canApproveFromVoice() || controller2.canApproveFromVoice()) throw new Error('voice must never carry approval authority');

  return { mediaOutputStopped: interrupted.mediaOutputStopped, runCancelCount: controller.runCancelObservations() };
}
