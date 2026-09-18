// MX-013 probe · 聚合读模型属主隔离观察器
// 1) 运行真实 Go frozen 测试（真实 SQLite、双用户同租户、真实 handler+service）解析 MX013-OBSERVATION；
// 2) 运行真实挂载套件解析 MX013-OBSERVATION（单次聚合调用/零逐会话补读）。
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
  visibleRunIds: string[];
  perSessionHTTPRequests: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'two-users-one-tenant') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  // 服务端属主隔离（真实 Go 测试输出）
  const go = await execFileAsync('go', ['test', './internal/handler/session/', '-run', 'TestMX013OverviewOwnerScope', '-count=1', '-v'], { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 });
  if (go.stdout.includes('FAIL') || go.stderr.includes('FAIL')) {
    throw new Error(`go overview test failed: ${(go.stdout + go.stderr).slice(0, 300)}`);
  }
  const goMarker = (go.stdout + go.stderr).match(/MX013-OBSERVATION (\{[^\n]+\})/);
  if (!goMarker) throw new Error('MX013-OBSERVATION missing from go test output');

  // 挂载层聚合单次调用（真实 HomeScreen + ProductShell）
  const vit = await execFileAsync('pnpm', ['--filter', '@weknora/mobile', 'exec', 'vitest', 'run', '-c', 'vitest.mobile-v2.config.ts'], { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 });
  const vitOut = vit.stdout + vit.stderr;
  if (/Tests\s+\d+ failed/.test(vitOut)) throw new Error(`mount suite failed: ${vitOut.slice(0, 300)}`);
  const mountMarker = vitOut.match(/MX013-OBSERVATION (\{[^\n]+\})/g);
  if (!mountMarker || mountMarker.length === 0) throw new Error('MX013-OBSERVATION missing from mount suite output');

  const server = JSON.parse(goMarker[1]) as { visibleRunIds: string[]; perSessionHTTPRequests: number };
  const mount = JSON.parse(mountMarker[0].replace('MX013-OBSERVATION ', '')) as { aggregateCalls: number; perSessionHTTPRequests: number };
  if (mount.aggregateCalls !== 1) throw new Error(`mount layer must load the aggregate exactly once, got ${mount.aggregateCalls}`);
  return {
    visibleRunIds: server.visibleRunIds,
    perSessionHTTPRequests: server.perSessionHTTPRequests + mount.perSessionHTTPRequests,
  };
}
