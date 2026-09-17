// MX-003 probe · 跨语言契约规则观察器
// 从共享 fixture（Go 与 TS 消费同一份）驱动真实 contracts 解析器与命令规则，
// 返回观测值；不做第二套业务实现。
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  parseExecution,
  parseExecutionEvent,
  parseExecutionSnapshot,
  evaluateCommand,
} from '../../../packages/contracts/src/mobile/execution.ts';
import { parseInteractionDecision } from '../../../packages/contracts/src/mobile/interactions.ts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  unknownType: string;
  canCancel: boolean;
}

interface Fixture {
  execution_unavailable_cancel: unknown;
  events_valid: unknown[];
  events_invalid: unknown[];
  decisions_valid: unknown[];
  decisions_invalid: unknown[];
  snapshot_go_shaped: unknown;
}

export async function runProbe(_input: ProbeInput): Promise<Observation> {
  const fixture = JSON.parse(
    await readFile(path.join(repoRoot, 'tests', 'mobile-v2', 'fixtures', 'mx-003-crosslang.json'), 'utf8'),
  ) as Fixture;

  // 1) 未知事件 type：必须可解析且原样保留（未知 type 不丢弃、不改写）
  let unknownType = '';
  for (const raw of fixture.events_valid) {
    const event = parseExecutionEvent(raw);
    if (event.type === 'future.event') unknownType = event.type;
  }

  // 2) 非法事件必须全部被真实解析器拒绝（安全整数上界/缺 run_id/数组 payload/非法日历）
  for (const raw of fixture.events_invalid) {
    let rejected = false;
    try {
      parseExecutionEvent(raw);
    } catch {
      rejected = true;
    }
    if (!rejected) throw new Error(`fixture event must be rejected: ${JSON.stringify(raw).slice(0, 120)}`);
  }

  // 3) 交互决定：合法通过，kind/action 互换与缺 decision_id 必须拒绝
  for (const raw of fixture.decisions_valid) parseInteractionDecision(raw);
  for (const raw of fixture.decisions_invalid) {
    let rejected = false;
    try {
      parseInteractionDecision(raw);
    } catch {
      rejected = true;
    }
    if (!rejected) throw new Error(`fixture decision must be rejected: ${JSON.stringify(raw).slice(0, 120)}`);
  }

  // 4) Go 形快照（含 incomplete/confirmed_watermark）必须完整解析
  const snapshot = parseExecutionSnapshot(fixture.snapshot_go_shaped);
  if (snapshot.incomplete !== false || snapshot.confirmedWatermark !== 33) {
    throw new Error('snapshot drift: incomplete/confirmed_watermark not parsed');
  }

  // 5) 命令规则：cancel capability 不可用 → 禁止
  const execution = parseExecution(fixture.execution_unavailable_cancel);
  const canCancel = evaluateCommand(execution, 'cancel').allowed;

  return { unknownType, canCancel };
}
