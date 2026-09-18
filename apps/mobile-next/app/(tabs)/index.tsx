import React from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { useOverview } from "@/features/workbench/useOverview";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { PendingCard } from "@/components/PendingCard";
import { TaskCard } from "@/components/TaskCard";
import { SectionHeader } from "@/components/SearchField";
import { StateView } from "@/components/StateView";

// M03 工作台（02 规格）：Hero 任务入口、指标、需你处理、正在推进、最近成果。
export default function WorkbenchScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const overview = useOverview();

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[16],
    },
    avatar: {
      width: 40,
      height: 40,
      borderRadius: 999,
      backgroundColor: theme.c.brand,
      alignItems: "center",
      justifyContent: "center",
    },
    avatarText: { color: theme.c["on-brand"], fontWeight: "700", fontSize: theme.type.label.fontSize },
    spaceName: { flexDirection: "row", alignItems: "center", gap: theme.space[4] },
    spaceNameText: { fontSize: theme.type.subtitle.fontSize, fontWeight: "600", color: theme.c.ink },
    subline: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 1 },
    hero: {
      backgroundColor: theme.c.hero,
      borderRadius: theme.radius.hero,
      padding: theme.space[24],
      marginBottom: theme.space[20],
    },
    eyebrow: {
      fontSize: theme.type.caption.fontSize,
      letterSpacing: 2,
      color: theme.mode === "light" ? theme.c["hero-ink"] : theme.c.accent,
      fontWeight: "600",
      opacity: 0.7,
      marginBottom: theme.space[8],
    },
    heroTitle: {
      fontSize: theme.type.display.fontSize,
      lineHeight: theme.type.display.lineHeight,
      fontWeight: theme.type.display.fontWeight as "700",
      color: theme.c["hero-ink"],
    },
    heroSub: {
      marginTop: theme.space[8],
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c["hero-ink"],
      opacity: 0.8,
    },
    heroAction: { flexDirection: "row", alignItems: "center", marginTop: theme.space[16], gap: theme.space[12] },
    metrics: { flexDirection: "row", gap: theme.space[12], marginBottom: theme.space[24] },
    metric: {
      flex: 1,
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      alignItems: "center",
      gap: 2,
    },
    metricNum: { fontSize: theme.type.metric.fontSize, lineHeight: theme.type.metric.lineHeight, fontWeight: "700", color: theme.c.ink },
    metricLabel: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    sectionGap: { marginBottom: theme.space[24] },
  });

  const spaceName =
    app.identity?.memberships.find((m) => m.tenantId === app.identity?.selectedTenantId)?.tenantName ?? "未选择空间";

  return (
    <View style={styles.wrap}>
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.header}>
          <View style={styles.avatar} accessibilityLabel="当前用户头像">
            <Text style={styles.avatarText}>{(app.identity?.displayName || "W").slice(0, 1).toUpperCase()}</Text>
          </View>
          <View style={{ flex: 1 }}>
            <Pressable style={styles.spaceName} accessibilityRole="button" accessibilityLabel={`切换空间，当前 ${spaceName}`} onPress={() => router.push("/spaces")}>
              <Text style={styles.spaceNameText} numberOfLines={1}>
                {spaceName}
              </Text>
              <Icon name="down" size={14} color={theme.c.muted} />
            </Pressable>
            <Text style={styles.subline}>你的移动 AI 工作台</Text>
          </View>
          <IconButton name="bell" label="收件箱" onPress={() => router.push("/inbox")} />
        </View>

        <View style={styles.hero}>
          <Text style={styles.eyebrow}>A LITTLE LESS BUSY</Text>
          <Text style={styles.heroTitle}>把想法交给 Agent，{"\n"}把时间留给自己。</Text>
          <Text style={styles.heroSub}>
            提一个目标，随时查看进展。{"\n"}需要决定时，我们会找到你。
          </Text>
          <View style={styles.heroAction}>
            <ActionButton label="新建任务" onPress={() => router.push("/new-task")} icon="plus" />
            <View style={{ flex: 1, alignItems: "flex-end" }}>
              <Icon name="spark" size={34} color={theme.mode === "light" ? theme.c["hero-ink"] : theme.c.accent} />
            </View>
          </View>
        </View>

        {overview.kind === "loading" && <StateView state={{ kind: "loading" }} />}
        {overview.kind === "error" && <StateView state={{ kind: "error", retry: overview.retry }} />}
        {overview.kind === "offline" && <StateView state={{ kind: "error", retry: overview.refresh }} />}
        {overview.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}

        {overview.kind === "ready" && (
          <>
            <View style={styles.metrics}>
              <Pressable style={styles.metric} accessibilityRole="button" accessibilityLabel={`进行中 ${overview.data.counts.running}`} onPress={() => router.push("/(tabs)/sessions")}>
                <Text style={styles.metricNum}>{overview.data.counts.running}</Text>
                <Text style={styles.metricLabel}>进行中</Text>
              </Pressable>
              <Pressable style={styles.metric} accessibilityRole="button" accessibilityLabel={`待你处理 ${overview.data.counts.waitingUser}`} onPress={() => router.push("/inbox")}>
                <Text style={[styles.metricNum, { color: theme.c.warning }]}>{overview.data.counts.waitingUser}</Text>
                <Text style={styles.metricLabel}>待你处理</Text>
              </Pressable>
              <Pressable style={styles.metric} accessibilityRole="button" accessibilityLabel={`已完成 ${overview.data.counts.completed}`} onPress={() => router.push("/(tabs)/sessions")}>
                <Text style={styles.metricNum}>{overview.data.counts.completed}</Text>
                <Text style={styles.metricLabel}>本周完成</Text>
              </Pressable>
            </View>

            {overview.data.pendingInteractions.length > 0 && (
              <View style={styles.sectionGap}>
                <SectionHeader title="需要你决定" actionLabel="查看全部" onAction={() => router.push("/inbox")} />
                {overview.data.pendingInteractions.map((p) => (
                  <PendingCard
                    key={p.id}
                    kindLabel={kindLabel(p.kind)}
                    title={p.title || "一次 Agent 操作需要确认"}
                    onPress={() => router.push(`/interactions/${p.id}?runId=${p.runId}`)}
                  />
                ))}
              </View>
            )}

            {overview.data.inProgress.length > 0 && (
              <View style={styles.sectionGap}>
                <SectionHeader title="正在推进" actionLabel="全部会话" onAction={() => router.push("/(tabs)/sessions")} />
                {overview.data.inProgress.map((r) => (
                  <TaskCard
                    key={r.runId}
                    title={r.title}
                    runBadge={{ domain: "run", label: runStatusLabel(r.status), tone: runStatusTone(r.status), unknown: r.status === "unknown" }}
                    meta={r.asOf ? `更新于 ${r.asOf}` : undefined}
                    onPress={() => router.push(`/executions/${r.runId}`)}
                  />
                ))}
              </View>
            )}

            {overview.data.inProgress.length === 0 && overview.data.pendingInteractions.length === 0 && (
              <StateView state={{ kind: "empty", title: "从第一项任务开始", description: "这里会汇总你的执行、待处理与成果。" }} />
            )}
          </>
        )}
      </ScrollView>
    </View>
  );
}

export function kindLabel(kind: string): string {
  if (kind === "tool_approval") return "工具批准";
  if (kind === "budget") return "预算";
  if (kind === "recovery") return "执行恢复";
  if (kind === "question") return "问题";
  if (kind === "connection") return "连接授权";
  return "需要确认";
}

export function runStatusLabel(status: string): string {
  switch (status) {
    case "queued": return "排队中";
    case "running": return "进行中";
    case "waiting_user": return "等待你处理";
    case "completed": case "succeeded": return "已完成";
    case "failed": return "失败";
    case "cancelled": case "canceled": return "已取消";
    default: return "等待同步";
  }
}

export function runStatusTone(status: string): "neutral" | "progress" | "attention" | "danger" | "success" | "unknown" {
  switch (status) {
    case "queued": return "neutral";
    case "running": return "progress";
    case "waiting_user": return "attention";
    case "completed": case "succeeded": return "success";
    case "failed": return "danger";
    default: return "unknown";
  }
}
