import React from "react";
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "./Icon";

export type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";

export function ActionButton({
  label,
  onPress,
  variant = "primary",
  disabled = false,
  busy = false,
  icon,
  block = false,
  testID,
  accessibilityHint,
}: {
  label: string;
  onPress: () => void;
  variant?: ButtonVariant;
  disabled?: boolean;
  busy?: boolean;
  icon?: IconName;
  block?: boolean;
  testID?: string;
  accessibilityHint?: string;
}) {
  const { theme } = useTheme();

  const palette = () => {
    switch (variant) {
      case "primary":
        return { bg: theme.c.brand, fg: theme.c["on-brand"] };
      case "secondary":
        return { bg: theme.c["brand-soft"], fg: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"] };
      case "danger":
        // tokens 未定义 on-danger：浅色白字、深色用 danger-soft 深底色作前景（对照检查通过，见 token-contrast）
        return { bg: theme.c.danger, fg: theme.mode === "light" ? "#FFFFFF" : theme.c["danger-soft"] };
      case "ghost":
        return { bg: "transparent", fg: theme.c.brand };
    }
  };
  const { bg, fg } = palette();
  const inert = disabled || busy;

  const styles = StyleSheet.create({
    base: {
      minHeight: theme.size["button-height"],
      borderRadius: theme.radius.control,
      paddingHorizontal: theme.space[20],
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "center",
      gap: theme.space[8],
      opacity: disabled ? 1 : 1,
      backgroundColor: disabled ? theme.c["disabled-bg"] : bg,
    },
    label: {
      ...({} as object),
      fontSize: theme.type.label.fontSize,
      lineHeight: theme.type.label.lineHeight,
      fontWeight: theme.type.label.fontWeight as "600",
      color: disabled ? theme.c.disabled : fg,
    },
  });

  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityHint={accessibilityHint}
      accessibilityState={{ disabled: inert, busy }}
      disabled={inert}
      onPress={onPress}
      style={({ pressed }) => [styles.base, block && { alignSelf: "stretch" }, pressed && !inert && { opacity: 0.85 }]}
    >
      {busy ? (
        <ActivityIndicator size="small" color={disabled ? theme.c.disabled : fg} />
      ) : (
        icon && <Icon name={icon} color={disabled ? theme.c.disabled : fg} />
      )}
      <Text style={styles.label}>{label}</Text>
    </Pressable>
  );
}
