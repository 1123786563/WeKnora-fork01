// MX-036 probe · 发布门禁观察器
// frozen 场景：core-pass-remote-blocked-env。
// 真实读取 acceptance-index.json + profile-manifest.json + 证据文件存在性 + 全量回归命令：
// core=releasable（带条件）、remote=blocked-env（能力关闭）、globalAllPassed=false——
// 不以 core 通过冒充全量；任何 accepted 任务的证据文件缺失即失败。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { readFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const execFileAsync = promisify(execFile);
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  core: string;
  remote: string;
  globalAllPassed: boolean;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'core-pass-remote-blocked-env') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }

  // 1) 验收索引（真实文件）
  const index = JSON.parse(await readFile(path.join(repoRoot, 'docs/evidence/mobile-v2/acceptance-index.json'), 'utf8')) as {
    tasks: Record<string, { status: string; evidence: string | null }>;
    release_ruling: { core: string; remote: string; globalAllPassed: boolean };
  };
  // 每个 accepted 任务证据文件必须真实存在
  for (const [id, task] of Object.entries(index.tasks)) {
    if (task.status.startsWith('accepted') && task.evidence) {
      if (!existsSync(path.join(repoRoot, 'docs/evidence/mobile-v2', task.evidence))) {
        throw new Error(`accepted task ${id} missing evidence file ${task.evidence}`);
      }
    }
  }
  // remote 必须为关闭态（MX-027 未激活——manifest 与索引一致）
  const manifest = JSON.parse(await readFile(path.join(repoRoot, 'deploy/mobile-workbench/profile-manifest.json'), 'utf8')) as {
    profiles: { remote: { evidence_required: string[] } };
  };
  if (!manifest.profiles.remote.evidence_required.includes('remote:cancel-evidence')) {
    throw new Error('manifest must gate remote behind cancel evidence');
  }
  if (index.tasks['MX-027']?.status !== 'not-activated') {
    throw new Error('MX-027 must be recorded as not-activated');
  }

  // 2) 全量回归（单元层）真实执行——独立子进程（node:test 递归规避；排除自身防自嵌套）
  const { spawn } = await import('node:child_process');
  const glob = (await import('node:fs/promises')).readdir;
  const files = (await glob(path.join(repoRoot, 'tests/mobile-v2')))
    .filter((name) => name.endsWith('.test.ts') && name !== 'mx-036.test.ts')
    .map((name) => path.join('tests/mobile-v2', name));
  const outcome = await new Promise<{ code: number; output: string }>((resolve) => {
    // NODE_TEST_CONTEXT 必须 delete：置空字符串仍会被子进程 node:test 判定为递归
    // 上下文而静默跳过全部文件（终审实证的空门——exit 0 但零执行）。
    const env = { ...process.env, NODE_OPTIONS: '' };
    delete env.NODE_TEST_CONTEXT;
    const child = spawn(process.execPath, ['--import', 'tsx', '--test', ...files], {
      cwd: repoRoot,
      env,
    });
    let output = '';
    child.stdout.on('data', (chunk: Buffer) => { output += chunk.toString(); });
    child.stderr.on('data', (chunk: Buffer) => { output += chunk.toString(); });
    child.on('close', (code) => resolve({ code: code ?? -1, output }));
  });
  // 双保险：退出码 + 解析通过/失败计数（静默跳过时 pass=0，此处即红）
  // node:test 汇总行形如「ℹ pass 42」「ℹ fail 0」——取最后一次匹配（全局汇总在末尾）
  const summary = /(?:^|\s)(?:pass|fail) (\d+)(?:\s|$)/gm;
  const numbers: Array<{ kind: 'pass' | 'fail'; n: number }> = [];
  for (const line of outcome.output.split('\n')) {
    const m = /^\s*\S*\s*(pass|fail) (\d+)\s*$/.exec(line.trim());
    if (m) numbers.push({ kind: m[1] as 'pass' | 'fail', n: Number(m[2]) });
  }
  void summary;
  const passCount = numbers.filter((x) => x.kind === 'pass').at(-1)?.n ?? 0;
  const failCount = numbers.filter((x) => x.kind === 'fail').at(-1)?.n ?? 99;
  if (outcome.code !== 0 || failCount !== 0 || passCount < 40) {
    throw new Error('mobile-v2 regression must genuinely run green for a release ruling: exit=' + outcome.code + ' pass=' + passCount + ' fail=' + failCount + ' :: ' + outcome.output.slice(0, 240));
  }

  const ruling = index.release_ruling;
  return {
    core: ruling.core.startsWith('releasable') ? 'releasable' : ruling.core,
    remote: ruling.remote.split(' ')[0],
    globalAllPassed: ruling.globalAllPassed,
  };
}
