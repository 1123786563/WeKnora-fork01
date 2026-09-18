import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon } from "./Icon";

// 成果卡（M03/M11/M14）：不可变版本 + 来源 Run。
export interface ArtifactCardProps {
  name: string;
  mime: string;
  version: string;
  sizeLabel?: string;
  sourceRun?: string;
  onPress: () => void;
  testID?: string;
}

export function ArtifactCard({ name, mime, version, sizeLabel, sourceRun, onPress, testID }: ArtifactCardProps) {
  const { theme } = useTheme();
  const iconFor = (): Parameters<typeof Icon>[0]["name"] => {
    if (mime.startsWith("image/")) return "grid";
    if (mime.includes("html")) return "globe";
    if (mime.includes("sheet") || mime.includes("excel")) return "grid";
    return "file";
  };
  const styles = StyleSheet.create({
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      flexDirection: "row" as const,
      alignItems: "center",
      gap: theme.space[12],
    },
    iconWrap: {
      width: 40,
      height: 40,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center" as const,
      justifyContent: "center" as const,
    },
    col: { flex: 1, gap: 2 },
    name: {
      fontSize: theme.type["body-sm"].fontSize + 1,
      lineHeight: theme.type["body-sm"].lineHeight,
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
      accessibilityLabel={`成果 ${name}`}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => [styles.card, pressed && { opacity: 0.8 }]}
    >
      <View style={styles.iconWrap}>
        <Icon name={iconFor()} size={20} color={theme.c.brand} />
      </View>
      <View style={styles.col}>
        <Text style={styles.name} numberOfLines={1} ellipsizeMode="middle">
          {name}
        </Text>
        <Text style={styles.meta}>
          v{version}
          {sizeLabel ? ` · ${sizeLabel}` : ""}
          {sourceRun ? ` · 来源任务 ${sourceRun}` : ""}
        </Text>
      </View>
      <Icon name="chevron" size={16} color={theme.c.subtle} />
    </Pressable>
  );
}
