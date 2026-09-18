// MX-004 probe · Go 字节 → 原生 parser 跨语言观察器
// 1) 运行真实 Go emit 测试（用产品 writer 原语重新生成 SSE 字节）；
// 2) 用真实 TS ExecutionSSEParser 逐字节（fault=split-every-byte）消费；
// 3) 用 domain classifyExecutionFrames 分类。全程无第二套实现。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { ExecutionSSEParser } from '../../../apps/mobile/sources/weknora/platform/stream-transport.ts';
import { classifyExecutionFrames, type ExecutionFrameInput } from '../../../packages/domain/src/mobile/execution-frames.ts';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  seqs: number[];
  controlCount: number;
  cursor: number;
}

async function parseGoBytes(file: string, splitEveryByte: boolean): Promise<ExecutionFrameInput[]> {
  const bytes = await readFile(path.join(repoRoot, 'tests', 'mobile-v2', 'fixtures', file));
  const parser = new ExecutionSSEParser();
  const rawFrames = splitEveryByte
    ? (() => {
        const frames = [];
        for (const byte of bytes) frames.push(...parser.push(new Uint8Array([byte])));
        frames.push(...parser.push(new Uint8Array(), true));
        return frames;
      })()
    : (() => {
        const frames = parser.push(new Uint8Array(bytes));
        frames.push(...parser.push(new Uint8Array(), true));
        return frames;
      })();
  return rawFrames.map((frame) =>
    frame.kind === 'control'
      ? { kind: 'control' as const, control: frame.control }
      : { kind: 'business' as const, event: frame.data },
  );
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  // 真实 Go emit 测试重新生成字节（保证消费的是当前 Go writer 输出，非陈旧 fixture）
  const { stdout } = await execFileAsync(
    'go',
    ['test', './internal/handler/session/', '-run', 'TestMX004Emit', '-count=1', '-v'],
    { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 },
  );
  if (!stdout.includes('TestMX004EmitSSEBytesSingle') || stdout.includes('FAIL')) {
    throw new Error(`go emit test did not pass: ${stdout.slice(0, 200)}`);
  }

  if (input.fixture === 'Go-emitted-UTF8-CRLF-control-seq42') {
    const frames = await parseGoBytes('mx-004-stream.bin', input.fault === 'split-every-byte');
    const result = classifyExecutionFrames(frames);
    return {
      seqs: result.business.map((event) => event.seq),
      controlCount: result.controlCount,
      cursor: result.cursor,
    };
  }
  // 附加验收：>256 业务帧无丢失、控制帧不占 seq、cursor 连续到 300
  const frames = await parseGoBytes('mx-004-stream-300.bin', false);
  const result = classifyExecutionFrames(frames);
  return {
    seqs: result.business.map((event) => event.seq),
    controlCount: result.controlCount,
    cursor: result.cursor,
  };
}
