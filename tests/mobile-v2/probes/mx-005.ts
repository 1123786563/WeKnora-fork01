// MX-005 probe · 乐观并发决定观察器
// 运行真实 Go 并发 CAS 测试（真实 SQLite、两 goroutine 同时决定同一 revision），
// 解析其结构化观测行；不做第二套实现。
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
  acceptedCount: number;
  conflictCount: number;
}

export async function runProbe(_input: ProbeInput): Promise<Observation> {
  const { stdout, stderr } = await execFileAsync(
    'go',
    ['test', './internal/application/service/workbench/', '-run', 'TestGormInteractionStoreScopesOwnerAndCASesDecision', '-count=1', '-v'],
    { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 },
  );
  const output = stdout + stderr;
  if (!output.includes('TestGormInteractionStoreScopesOwnerAndCASesDecision') || output.includes('FAIL')) {
    throw new Error(`go CAS test did not pass: ${output.slice(0, 300)}`);
  }
  const marker = output.match(/MX005-OBSERVATION accepted=(\d+) conflict=(\d+)/);
  if (!marker) {
    throw new Error('observation marker missing from go test output (test may be stale or filtered)');
  }
  return { acceptedCount: Number(marker[1]), conflictCount: Number(marker[2]) };
}
