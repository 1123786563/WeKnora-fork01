// MX-020 probe · 类型化交互字段观察器
// frozen 场景：budget-question-connection-tool。
// 真实组件行为（挂载级）+ 契约矩阵：工具审批表单不含预算字段（toolBudgetFieldCount=0）；
// 问题回复提交用户编辑后的最新文本；连接授权在浏览器回跳期间展示 authorizing。
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
  toolBudgetFieldCount: number;
  questionAnswer: string;
  connectionState: string;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'budget-question-connection-tool') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }
  // 挂载套件（vitest + rn-mock 基底）真实渲染三类组件并输出观测
  const { stdout, stderr } = await execFileAsync(
    'pnpm',
    ['--filter', '@weknora/mobile', 'exec', 'vitest', 'run', '-c', 'vitest.mobile-v2.config.ts', 'sources/weknora/interactions/'],
    { cwd: repoRoot, maxBuffer: 16 * 1024 * 1024 },
  );
  const output = stdout + stderr;
  if (/Tests\s+\d+ failed/.test(output)) throw new Error(`mount suite failed: ${output.slice(0, 400)}`);
  const marker = output.match(/MX020-OBSERVATION (\{[^\n]+\})/);
  if (!marker) throw new Error('MX020-OBSERVATION missing from mount suite output');
  return JSON.parse(marker[1]) as Observation;
}
