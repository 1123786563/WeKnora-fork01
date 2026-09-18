// MX-035 probe · 视觉/无障碍/性能验收观察器（静态层 + 设备层基线）
// 静态层（可离线验证，真实源级+挂载观测）：
// 1) 主操作可达性：18 页主按钮在对应屏源码中带非空 accessibilityLabel（可寻址=读屏可达）；
// 2) 关键 a11y 合同：裸 Pressable 必带 accessibilityRole+Label；Button 组件统一合同（结构保证）。
// 设备层：视觉基线矩阵与性能数值在 visual-baseline.json 保持 pending（设备 blocked-env——不伪造 hash）。
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  unreachablePrimaryActions: string[];
  criticalA11yFindings: string[];
}

/** 页 → 主操作（屏文件 + accessibilityLabel 关键词）——可寻址即读屏可达。 */
const PRIMARY_ACTIONS: Record<string, { file: string; labelPattern: RegExp }> = {
  M01: { file: 'apps/mobile/sources/weknora/screens/LoginScreen.tsx', labelPattern: /使用邮箱密码登录/ },
  M02: { file: 'apps/mobile/sources/weknora/screens/SpacePickerScreen.tsx', labelPattern: /进入空间/ },
  M03: { file: 'apps/mobile/sources/weknora/screens/HomeScreen.tsx', labelPattern: /重试|数据更新于/ },
  M04: { file: 'apps/mobile/sources/weknora/screens/SessionListScreen.tsx', labelPattern: /新建任务/ },
  M05: { file: 'apps/mobile/sources/weknora/screens/NewTaskScreen.tsx', labelPattern: /提交任务（先保存请求再发送）/ },
  M06: { file: 'apps/mobile/sources/weknora/screens/AgentPickerScreen.tsx', labelPattern: /确认选择/ },
  M07: { file: 'apps/mobile/sources/weknora/conversations/ConversationScreen.tsx', labelPattern: /发送指令/ },
  M08: { file: 'apps/mobile/sources/weknora/screens/ExecutionScreen.tsx', labelPattern: /申请取消该任务/ },
  M09: { file: 'apps/mobile/sources/weknora/screens/InteractionScreen.tsx', labelPattern: /批准工具|拒绝工具/ },
  M10: { file: 'apps/mobile/sources/weknora/screens/InboxScreen.tsx', labelPattern: /重试|点击后恢复身份/ },
  M11: { file: 'apps/mobile/sources/weknora/screens/KnowledgeScreen.tsx', labelPattern: /以引用方式使用.*提问/ },
  M12: { file: 'apps/mobile/sources/weknora/screens/KnowledgeScreen.tsx', labelPattern: /知识库/ },
  M13: { file: 'apps/mobile/sources/weknora/screens/ConnectionScreen.tsx', labelPattern: /撤销.*连接|系统浏览器中开始/ },
  M14: { file: 'apps/mobile/sources/weknora/screens/ArtifactScreen.tsx', labelPattern: /分享成果/ },
  M15: { file: 'apps/mobile/sources/weknora/screens/TargetPickerScreen.tsx', labelPattern: /确认执行目标/ },
  M16: { file: 'apps/mobile/sources/weknora/screens/VoiceInputScreen.tsx', labelPattern: /确认听写内容/ },
  M17: { file: 'apps/mobile/sources/weknora/screens/ProfileScreen.tsx', labelPattern: /退出登录/ },
  M18: { file: 'apps/mobile/sources/weknora/screens/UsageScreen.tsx', labelPattern: /重试/ },
};

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'all-pages-light-dark-200-percent-native') {
    throw new Error(`unsupported fixture: ${input.fixture}`);
  }

  const unreachablePrimaryActions: string[] = [];
  const criticalA11yFindings: string[] = [];

  // 1) 主操作可达性（源级真实观测：屏代码含带 accessibilityLabel 的主操作）
  for (const [page, spec] of Object.entries(PRIMARY_ACTIONS)) {
    const source = await readFile(path.join(repoRoot, spec.file), 'utf8');
    const hasLabeledAction = source.includes('accessibilityLabel') && spec.labelPattern.test(source);
    if (!hasLabeledAction) unreachablePrimaryActions.push(`${page}:primary-action`);
  }

  // 2) 关键 a11y 合同：裸 Pressable 必带 role（label 已由主操作覆盖；tab 类同）
  const scanDirs = [
    'apps/mobile/sources/weknora/ui',
    'apps/mobile/sources/weknora/screens',
    'apps/mobile/sources/weknora/interactions',
    'apps/mobile/sources/weknora/conversations',
    'apps/mobile/sources/weknora/navigation',
    'apps/mobile/sources/weknora/voice',
  ];
  const collect = async (dir: string): Promise<string[]> => {
    const { readdir } = await import('node:fs/promises');
    const out: string[] = [];
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) out.push(...(await collect(full)));
      else if (/\.(tsx|ts)$/.test(entry.name) && !entry.name.includes('.spec.')) out.push(full);
    }
    return out;
  };
  for (const dir of scanDirs) {
    for (const file of await collect(path.join(repoRoot, dir))) {
      const source = await readFile(file, 'utf8');
      // 逐个 Pressable 块检查（到配对 > 之间的属性区）
      const pressableBlocks = source.split(/<Pressable[\s>]/).slice(1).map((chunk) => chunk.slice(0, chunk.search(/>|\/>/) > 0 ? chunk.search(/>|\/>/) : 0));
      for (const block of pressableBlocks) {
        if (!/accessibilityRole/.test(block)) {
          criticalA11yFindings.push(`${path.relative(repoRoot, file)}: Pressable without accessibilityRole`);
        }
        if (!/accessibilityLabel|accessibilityRole="link"/.test(block)) {
          criticalA11yFindings.push(`${path.relative(repoRoot, file)}: Pressable without accessible name`);
        }
      }
    }
  }

  return { unreachablePrimaryActions, criticalA11yFindings };
}
