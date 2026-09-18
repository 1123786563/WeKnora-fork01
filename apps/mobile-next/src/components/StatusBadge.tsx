import React from "react";
import { StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";

// 状态徽章分三个域（详细设计 §4.3）：run（任务进度）/ interaction（待处理）/ settlement（费用最终性）。
// 原始枚举与显示文案分离；未知枚举显示"等待同步"并标记 unknown，不得显示为成功。
export type BadgeDomain = "run" | "interaction" | "settlement";
export type BadgeTone = "neutral" | "progress" | "attention" | "danger" | "success" | "unknown";

export interface BadgeSpec {
  domain: BadgeDomain;
  label: string;
  tone: BadgeTone;
  unknown?: boolean;
}

export function StatusBadge({ domain, label, tone, unknown }: BadgeSpec) {
  const { theme } = useTheme();
  const toneMap: Record<BadgeTone, { fg: string; bg: string; icon?: IconName }> = {
    neutral: { fg: theme.c.muted, bg: theme.c["surface-alt"] },
    progress: { fg: theme.c.info, bg: theme.c["info-soft"], icon: "activity" },
    attention: { fg: theme.c.warning, bg: theme.c["warning-soft"], icon: "clock" },
    danger: { fg: theme.c.danger, bg: theme.c["danger-soft"], icon: "alert" },
    success: { fg: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], bg: theme.c["brand-soft"], icon: "check" },
    unknown: { fg: theme.c.subtle, bg: theme.c["surface-alt"], icon: "refresh" },
  };
  const t = toneMap[unknown ? "unknown" : tone];

  const styles = StyleSheet.create({
    wrap: {
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[4],
      borderRadius: theme.radius.pill,
      paddingHorizontal: theme.space[8],
      paddingVertical: 2,
      backgroundColor: t.bg,
      alignSelf: "flex-start" as const,
    },
    text: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      fontWeight: "600",
      color: t.fg,
    },
  });

  return (
    <View
      style={styles.wrap}
      accessibilityLabel={`${domainName(domain)}：${label}`}
      testID={`badge-${domain}`}
    >
      {t.icon && <Icon name={t.icon} size={12} color={t.fg} />}
      <Text style={styles.text}>{label}</Text>
    </View>
  );
}

function domainName(d: BadgeDomain): string {
  return d === "run" ? "任务状态" : d === "interaction" ? "待处理" : "结算状态";
}
