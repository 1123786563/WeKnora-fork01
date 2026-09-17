// MX-008 probe · 原生组件挂载观察器
// 运行真实 vitest 挂载套件（react-test-renderer + rn-mock 基底，专用配置），
// 解析结构化观测行；不在本文件内复刻组件行为。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  decisionCount: number;
  focusTarget: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'mounted-native-sheet' || input.fault !== 'dismiss-before-decision') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const { stdout, stderr } = await execFileAsync(
    'pnpm',
    ['--filter', '@weknora/mobile', 'exec', 'vitest', 'run', '-c', 'vitest.mobile-v2.config.ts'],
    { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 },
  );
  const output = stdout + stderr;
  if (output.includes('failed') && /Tests\s+\d+ failed/.test(output)) {
    throw new Error(`mount suite failed: ${output.slice(0, 400)}`);
  }
  const marker = output.match(/MX008-OBSERVATION (\{[^\n]+\})/);
  if (!marker) {
    throw new Error('MX008-OBSERVATION missing from mount suite output');
  }
  return JSON.parse(marker[1]) as Observation;
}
