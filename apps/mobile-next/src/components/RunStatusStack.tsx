import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";

// M08 三状态清单（详细设计 §4.3）：run/execution/settlement 三行独立 + 最近同步时间。
// 取消已受理 ≠ 停止已确认 ≠ 退款；未知显示"等待同步"。
export interface RunStatusItem {
  key: "run" | "execution" | "settlement";
  title: string;
  rawValue: string; // 服务端原始枚举
  label: string; // 展示文案（由 domain 层映射，未知→等待同步）
  tone: "neutral" | "progress" | "attention" | "danger" | "success" | "unknown";
  unknown: boolean;
  icon?: IconName;
}

export function RunStatusStack({ items, lastSyncAt, testID }: { items: RunStatusItem[]; lastSyncAt?: string | null; testID?: string }) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      gap: theme.space[8],
    },
    row: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[8],
      minHeight: 32,
    },
    title: {
      width: 92,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    value: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      fontWeight: "600",
      color: theme.c.ink,
    },
    sync: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
    },
  });

  const toneColor = (t: RunStatusItem["tone"]) => {
    switch (t) {
      case "progress": return theme.c.info;
      case "attention": return theme.c.warning;
      case "danger": return theme.c.danger;
      case "success": return theme.mode === "light" ? theme.c.brand : theme.c.accent;
      default: return theme.c.subtle;
    }
  };

  return (
    <View style={styles.card} testID={testID ?? "run-status-stack"}>
      {items.map((it) => (
        <View key={it.key} style={styles.row} accessibilityLabel={`${it.title} ${it.label}`}>
          <Text style={styles.title}>{it.title}</Text>
          <Text style={[styles.value, it.unknown && { color: theme.c.subtle }]}>{it.label}</Text>
          <Icon name={it.icon ?? (it.unknown ? "refresh" : "checkcircle")} size={16} color={toneColor(it.tone)} />
        </View>
      ))}
      <Text style={styles.sync}>{lastSyncAt ? `最近同步 ${lastSyncAt}` : "尚未同步"}</Text>
    </View>
  );
}
