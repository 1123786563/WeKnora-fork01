import React, { useState } from "react";
import { FlatList, Pressable, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { FilterChips, type FilterChip } from "@/components/FilterChips";
import { PendingCard } from "@/components/PendingCard";
import { StateView } from "@/components/StateView";
import { StatusBadge } from "@/components/StatusBadge";
import { useOverview } from "@/features/workbench/useOverview";
import { formatRelative, kindLabel } from "@/features/workbench/labels";

// M10 收件箱（02 规格）：聚合待处理交互与任务动态（复用 useOverview 的客户端聚合口径，D-04）；
// 全部已读为本地状态（不向服务端写已读回执——后端暂无该端点）。

const FILTER_CHIPS: FilterChip[] = [
  { key: "all", label: "全部" },
  { key: "pending", label: "待处理" },
  { key: "activity", label: "任务动态" },
];

type InboxItem =
  | { kind: "pending"; id: string; runId: string; itKind: string; title: string }
  | { kind: "activity"; id: string; title: string; updatedAt: string | null };

export default function InboxScreen() {
  const { theme } = useTheme();
  const insets = useSafeAreaInsets();
  const overview = useOverview();

  const [filter, setFilter] = useState("all");
  const [allRead, setAllRead] = useState(false);

  const items: InboxItem[] =
    overview.kind === "ready"
      ? [
          ...overview.data.pendingInteractions.map((p) => ({
            kind: "pending" as const,
            id: p.id,
            runId: p.runId,
            itKind: p.kind,
            title: p.title,
          })),
          ...overview.data.recentSessions.map((s) => ({
            kind: "activity" as const,
            id: s.id,
            title: s.title,
            updatedAt: s.updated_at || null,
          })),
        ]
      : [];

  const filtered = items.filter((it) => filter === "all" || it.kind === filter);

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[4],
      paddingTop: insets.top + theme.space[12],
      paddingHorizontal: theme.space[8],
      paddingBottom: theme.space[8],
    },
    headerTitle: {
      flex: 1,
      fontSize: theme.type.title.fontSize,
      lineHeight: theme.type.title.lineHeight,
      fontWeight: theme.type.title.fontWeight as "700",
      color: theme.c.ink,
    },
    topRow: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      gap: theme.space[8],
      paddingHorizontal: theme.space[20],
      paddingBottom: theme.space[12],
    },
    readAll: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[4],
      minHeight: 36,
      paddingHorizontal: theme.space[8],
    },
    readAllText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      fontWeight: "600",
      color: allRead ? theme.c.disabled : theme.c.brand,
    },
    chips: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[12] },
    list: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    separator: { height: theme.space[12] },
    activityRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
    },
    activityIcon: {
      width: 36,
      height: 36,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["surface-alt"],
      alignItems: "center",
      justifyContent: "center",
    },
    activityCol: { flex: 1, gap: 2 },
    activityTitle: {
      fontSize: theme.type["body-sm"].fontSize + 1,
      lineHeight: theme.type["body-sm"].lineHeight + 2,
      fontWeight: "600",
      color: theme.c.ink,
    },
    activityMeta: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
    },
  });

  const renderBody = () => {
    if (overview.kind === "loading") return <StateView state={{ kind: "loading" }} />;
    if (overview.kind === "forbidden") return <StateView state={{ kind: "forbidden" }} />;
    if (overview.kind === "error" || overview.kind === "offline") {
      return <StateView state={{ kind: "error", retry: overview.kind === "error" ? overview.retry : overview.refresh }} />;
    }
    if (filtered.length === 0) {
      return (
        <StateView
          state={{ kind: "empty", title: "暂时没有新动态", description: "任务需要你决定时，会在这里出现。" }}
        />
      );
    }
    return null;
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle} accessibilityRole="header">
          收件箱
        </Text>
      </View>
      <View style={styles.topRow}>
        <StatusBadge
          domain="interaction"
          label={allRead ? "已读" : `${items.length} 条动态`}
          tone={allRead ? "success" : "attention"}
        />
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="全部标为已读"
          accessibilityState={{ disabled: allRead }}
          disabled={allRead}
          onPress={() => setAllRead(true)}
          style={({ pressed }) => [styles.readAll, pressed && !allRead && { opacity: 0.7 }]}
          testID="read-all-button"
        >
          <Icon name="check" size={16} color={allRead ? theme.c.disabled : theme.c.brand} />
          <Text style={styles.readAllText}>全部已读</Text>
        </Pressable>
      </View>
      <View style={styles.chips}>
        <FilterChips chips={FILTER_CHIPS} selected={filter} onSelect={(k) => setFilter(k)} testID="inbox-filter" />
      </View>
      <FlatList
        data={filtered}
        keyExtractor={(it) => `${it.kind}-${it.id}`}
        renderItem={({ item }) =>
          item.kind === "pending" ? (
            <PendingCard
              kindLabel={kindLabel(item.itKind)}
              title={item.title || "一次 Agent 操作需要确认"}
              onPress={() => router.push(`/interactions/${item.id}?runId=${item.runId}`)}
              testID={`inbox-pending-${item.id}`}
            />
          ) : (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={`任务动态 ${item.title}`}
              onPress={() => router.push(`/sessions/${item.id}`)}
              testID={`inbox-activity-${item.id}`}
              style={({ pressed }) => [styles.activityRow, pressed && { opacity: 0.8 }]}
            >
              <View style={styles.activityIcon}>
                <Icon name="chat" size={18} color={theme.c.muted} />
              </View>
              <View style={styles.activityCol}>
                <Text style={styles.activityTitle} numberOfLines={2}>
                  {item.title}
                </Text>
                <Text style={styles.activityMeta}>
                  {item.updatedAt ? `更新于 ${formatRelative(item.updatedAt)}` : "会话有更新"}
                </Text>
              </View>
              <Icon name="chevron" size={16} color={theme.c.subtle} />
            </Pressable>
          )
        }
        ItemSeparatorComponent={() => <View style={styles.separator} />}
        ListEmptyComponent={renderBody()}
        contentContainerStyle={styles.list}
      />
    </View>
  );
}
