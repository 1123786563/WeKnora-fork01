import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";

export function IconButton({
  name,
  label,
  onPress,
  tone = "plain",
  disabled = false,
  badge,
  testID,
}: {
  name: IconName;
  label: string; // 无障碍标签必填：图标按钮不能没有读屏名称
  onPress: () => void;
  tone?: "plain" | "brand" | "surface";
  disabled?: boolean;
  badge?: number;
  testID?: string;
}) {
  const { theme } = useTheme();
  const fg = tone === "brand" ? theme.c.brand : theme.c.ink;
  const bg = tone === "surface" ? theme.c.surface : "transparent";

  const styles = StyleSheet.create({
    hit: {
      width: theme.size["icon-touch"],
      height: theme.size["icon-touch"],
      borderRadius: theme.radius.pill,
      alignItems: "center",
      justifyContent: "center",
      backgroundColor: bg,
    },
    badge: {
      position: "absolute" as const,
      top: theme.space[6],
      right: theme.space[4],
      minWidth: theme.space[16],
      height: theme.space[16],
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c.danger,
      alignItems: "center",
      justifyContent: "center",
      paddingHorizontal: theme.space[4],
    },
    badgeText: {
      color: theme.mode === "light" ? "#FFFFFF" : "#113525",
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      fontWeight: "600",
    },
  });

  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled }}
      disabled={disabled}
      onPress={onPress}
      style={({ pressed }) => [styles.hit, pressed && { opacity: 0.7 }]}
    >
      <Icon name={name} color={disabled ? theme.c.disabled : fg} />
      {typeof badge === "number" && badge > 0 && (
        <View style={styles.badge}>
          <Text style={styles.badgeText}>{badge > 99 ? "99+" : String(badge)}</Text>
        </View>
      )}
    </Pressable>
  );
}
