import { Pressable, ScrollView, Text, View, useColorScheme } from 'react-native';
import { nativeTokens } from '../../../../../packages/design-tokens/src/mobile/native-tokens.ts';
import type { NativeTaskCapabilities } from './task-entry.ts';

export interface ExistingTaskRow {
  taskId: string;
  runId: string;
  title: string;
  runStatus: string;
  updatedAt: string;
}

export interface TaskEntryScreenProps {
  loading: boolean;
  tasks: readonly ExistingTaskRow[];
  error?: string;
  capabilities: NativeTaskCapabilities;
  onRefresh(): void;
  onOpenTask(task: ExistingTaskRow): void;
}

const CAPABILITY_LABELS: Record<keyof NativeTaskCapabilities, string> = {
  navigation: '页面导航',
  fileSelection: '文件选择',
  fileDownload: '文件下载',
  systemShare: '系统分享',
  notifications: '通知',
  secureStorage: '安全存储',
};

/** Existing Task chooser; all business state and authenticated reads remain in Task Office. */
export function TaskEntryScreen({ loading, tasks, error, capabilities, onRefresh, onOpenTask }: TaskEntryScreenProps) {
  const dark = useColorScheme() === 'dark';
  const colors = nativeTokens.colors[dark ? 'dark' : 'light'];
  const spacing = nativeTokens.spacing;
  const typography = nativeTokens.typography;
  const surface = { flex: 1, backgroundColor: colors.bg, paddingHorizontal: spacing['20'], paddingTop: spacing['24'] } as const;
  const heading = { color: colors.ink, fontSize: typography.title.fontSize, lineHeight: typography.title.lineHeight, fontWeight: typography.title.fontWeight } as const;

  return (
    <ScrollView contentContainerStyle={{ ...surface, paddingBottom: spacing['32'], alignItems: 'center' }}>
      <View style={{ width: '100%', maxWidth: 720, gap: spacing['16'] }}>
        <Text accessibilityRole="header" style={heading}>任务工作台</Text>
        {loading ? <Text accessibilityRole="text" accessibilityLiveRegion="polite" style={{ color: colors.muted }}>正在读取已授权任务…</Text> : null}
        {!loading && error !== undefined ? (
          <View accessibilityRole="alert" style={{ backgroundColor: colors['danger-soft'], borderRadius: nativeTokens.radius.card, padding: spacing['16'], gap: spacing['12'] }}>
            <Text style={{ color: colors.danger, fontSize: typography.body.fontSize }}>{error}</Text>
            <ActionButton title="重试读取" onPress={onRefresh} colors={colors} />
          </View>
        ) : null}
        {!loading && error === undefined && tasks.length === 0 ? (
          <View style={{ backgroundColor: colors.surface, borderRadius: nativeTokens.radius.card, padding: spacing['20'], gap: spacing['8'] }}>
            <Text style={{ color: colors.ink, fontSize: typography.subtitle.fontSize, fontWeight: typography.subtitle.fontWeight }}>还没有可显示的任务</Text>
            <Text style={{ color: colors.muted, fontSize: typography['body-sm'].fontSize }}>任务会在获授权的工作空间中显示。</Text>
            <ActionButton title="刷新任务" onPress={onRefresh} colors={colors} />
          </View>
        ) : null}
        {!loading && error === undefined ? tasks.map((task) => (
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={`打开任务 ${task.title || task.taskId}，运行状态 ${task.runStatus}`}
            key={`${task.taskId}:${task.runId}`}
            onPress={() => onOpenTask(task)}
            style={({ pressed }) => ({ backgroundColor: pressed ? colors['brand-soft'] : colors.surface, borderColor: colors.line, borderRadius: nativeTokens.radius.card, borderWidth: 1, minHeight: nativeTokens.size.touch, padding: spacing['16'], gap: spacing['6'] })}
          >
            <Text numberOfLines={2} style={{ color: colors.ink, fontSize: typography.subtitle.fontSize, fontWeight: typography.subtitle.fontWeight }}>{task.title || task.taskId}</Text>
            <Text style={{ color: colors.muted, fontSize: typography['body-sm'].fontSize }}>运行状态：{task.runStatus}</Text>
            <Text style={{ color: colors.subtle, fontSize: typography.caption.fontSize }}>更新于 {task.updatedAt}</Text>
          </Pressable>
        )) : null}
          <View accessibilityLabel="设备能力状态" style={{ backgroundColor: colors['surface-alt'], borderRadius: nativeTokens.radius.card, padding: spacing['16'], gap: spacing['8'] }}>
          <Text style={{ color: colors.ink, fontSize: typography.label.fontSize, fontWeight: typography.label.fontWeight }}>设备能力</Text>
          {(Object.keys(CAPABILITY_LABELS) as Array<keyof NativeTaskCapabilities>).map((key) => {
            const capability = capabilities[key];
            return <Text key={key} style={{ color: capability.status === 'available' ? colors.ink : colors.muted, fontSize: typography['body-sm'].fontSize }}>
              {CAPABILITY_LABELS[key]}：{capability.status === 'available' ? '可用' : `不可用（${capability.reason}）`}
            </Text>;
          })}
        </View>
      </View>
    </ScrollView>
  );
}

function ActionButton({ title, onPress, colors }: { title: string; onPress(): void; colors: typeof nativeTokens.colors[keyof typeof nativeTokens.colors] }) {
  return (
    <Pressable accessibilityRole="button" onPress={onPress} style={({ pressed }) => ({ alignItems: 'center', justifyContent: 'center', backgroundColor: pressed ? colors['brand-hover'] : colors.brand, borderRadius: nativeTokens.radius.control, minHeight: nativeTokens.size['button-height'], paddingHorizontal: nativeTokens.spacing['16'] })}>
      <Text style={{ color: colors['on-brand'], fontSize: nativeTokens.typography.label.fontSize, fontWeight: nativeTokens.typography.label.fontWeight }}>{title}</Text>
    </Pressable>
  );
}
