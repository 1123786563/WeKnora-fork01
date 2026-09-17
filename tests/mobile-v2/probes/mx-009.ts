// MX-009 probe · 产品路由容器观察器
// 挂载真实 ProductShell（rn-mock 基底 + react-test-renderer），观测四固定 Tab 与挂载期网络请求计数。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  visibleTabs: string[];
  happyAuthRequests: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'product-scope-without-happy-sync') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  const { stdout, stderr } = await execFileAsync(
    'pnpm',
    ['--filter', '@weknora/mobile', 'exec', 'vitest', 'run', '-c', 'vitest.mobile-v2.config.ts'],
    { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 },
  );
  const output = stdout + stderr;
  if (/Tests\s+\d+ failed/.test(output)) {
    throw new Error(`mount suite failed: ${output.slice(0, 400)}`);
  }
  const marker = output.match(/MX009-OBSERVATION (\{[^\n]+\})/);
  if (!marker) throw new Error('MX009-OBSERVATION missing from mount suite output');
  return JSON.parse(marker[1]) as Observation;
}
