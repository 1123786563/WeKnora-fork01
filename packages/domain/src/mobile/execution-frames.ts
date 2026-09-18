import type { ExecutionEvent } from '@weknora/contracts';

/** v2 控制帧载荷（Go writeWorkbenchControlSSE）：显式 event: control，无业务 id。 */
export interface StreamControl {
  code: string;
  message: string;
}

export interface ExecutionFrameInput {
  kind: 'business' | 'control';
  event?: ExecutionEvent;
  control?: StreamControl;
}

export interface FrameClassification {
  /** 去重（同 seq 保留首个）且升序的业务事件；未知 type 原样保留。 */
  business: ExecutionEvent[];
  controls: StreamControl[];
  controlCount: number;
  /** 业务 watermark：业务事件最大 seq；控制帧绝不推进该值。 */
  cursor: number;
}

export function parseStreamControl(value: unknown): StreamControl {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('invalid stream control payload');
  }
  const row = value as Record<string, unknown>;
  if (typeof row.code !== 'string' || row.code.trim() === '' || typeof row.message !== 'string') {
    throw new Error('invalid stream control payload');
  }
  return { code: row.code, message: row.message };
}

/**
 * 帧分类（MX-004 冻结规则）：
 * - 控制帧（control/error）不进入业务投影、不推进 cursor；
 * - 业务帧按 seq 去重升序（同 seq 保留首个，未知 type 保留）；
 * - cursor 只来自业务 seq。
 */
export function classifyExecutionFrames(frames: ReadonlyArray<ExecutionFrameInput>): FrameClassification {
  const seen = new Set<number>();
  const business: ExecutionEvent[] = [];
  const controls: StreamControl[] = [];
  for (const frame of frames) {
    if (frame.kind === 'control') {
      if (frame.control) controls.push(frame.control);
      continue;
    }
    const event = frame.event;
    if (!event || seen.has(event.seq)) continue;
    seen.add(event.seq);
    business.push(event);
  }
  business.sort((a, b) => a.seq - b.seq);
  const cursor = business.length > 0 ? business[business.length - 1].seq : 0;
  return { business, controls, controlCount: controls.length, cursor };
}
