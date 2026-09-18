import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";
import { StatusBadge, type BadgeSpec } from "./StatusBadge";

// 工作台/列表任务卡（RunCard 语义）：stale 与 unknown 可分辨，不暴露越权摘要。
export interface TaskCardProps {
  title: string;
  agentName?: string;
  runBadge: BadgeSpec;
  meta?: string; // 如 "更新于 5 分钟前"
  stale?: boolean;
  onPress: () => void;
  testID?: string;
}

export function TaskCard({ title, agentName, runBadge, meta, stale, onPress, testID }: TaskCardProps) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      gap: theme.space[8],
    },
    top: { flexDirection: "row" as const, alignItems: "center", justifyContent: "space-between", gap: theme.space[8] },
    title: {
      flex: 1,
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    agent: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[4],
    },
    agentText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    meta: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: stale ? theme.c.warning : theme.c.subtle,
    },
  });
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`任务 ${title}`}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => [styles.card, pressed && { opacity: 0.8 }]}
    >
      <View style={styles.top}>
        <Text style={styles.title} numberOfLines={2}>
          {title}
        </Text>
        <StatusBadge {...runBadge} />
      </View>
      {agentName && (
        <View style={styles.agent}>
          <Icon name="spark" size={14} color={theme.c.muted} />
          <Text style={styles.agentText}>{agentName}</Text>
        </View>
      )}
      {meta && (
        <Text style={styles.meta}>
          {stale ? "当前显示上次同步结果 · " : ""}
          {meta}
        </Text>
      )}
    </Pressable>
  );
}
