import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";

// 待你处理卡（M03/M07）：只展示摘要，不暴露审批正文（详细设计 §7）。
export interface PendingCardProps {
  kindLabel: string; // 工具批准 / 预算 / 问题 / 连接授权
  title: string;
  runTitle?: string;
  updatedAt?: string;
  onPress: () => void;
  testID?: string;
}

export function PendingCard({ kindLabel, title, runTitle, updatedAt, onPress, testID }: PendingCardProps) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      borderWidth: 1,
      borderColor: theme.c.line,
      padding: theme.space[16],
      gap: theme.space[4],
    },
    kind: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[6],
    },
    kindText: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      fontWeight: "600",
      color: theme.c.warning,
    },
    title: {
      fontSize: theme.type["body-sm"].fontSize + 2,
      lineHeight: theme.type["body-sm"].lineHeight + 2,
      fontWeight: "600",
      color: theme.c.ink,
    },
    meta: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
    },
  });
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`待处理 ${kindLabel} ${title}`}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => [styles.card, pressed && { opacity: 0.8 }]}
    >
      <View style={styles.kind}>
        <Icon name="clock" size={14} color={theme.c.warning} />
        <Text style={styles.kindText}>需要你处理 · {kindLabel}</Text>
      </View>
      <Text style={styles.title} numberOfLines={2}>
        {title}
      </Text>
      {(runTitle || updatedAt) && (
        <Text style={styles.meta}>
          {[runTitle, updatedAt].filter(Boolean).join(" · ")}
        </Text>
      )}
    </Pressable>
  );
}
