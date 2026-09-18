import React from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";

// 状态筛选 chips（M04：全部/进行中/待处理/已完成）
export interface FilterChip {
  key: string;
  label: string;
}

export function FilterChips({
  chips,
  selected,
  onSelect,
  testID,
}: {
  chips: FilterChip[];
  selected: string;
  onSelect: (key: string) => void;
  testID?: string;
}) {
  const { theme } = useTheme();
  const styles = StyleSheet.create({
    row: { flexDirection: "row" as const, gap: theme.space[8] },
    chip: {
      minHeight: 36,
      borderRadius: theme.radius.pill,
      paddingHorizontal: theme.space[16],
      alignItems: "center" as const,
      justifyContent: "center" as const,
      backgroundColor: theme.c.surface,
      borderWidth: 1,
      borderColor: theme.c.line,
    },
    chipActive: { backgroundColor: theme.c["brand-soft"], borderColor: theme.c["brand-soft"] },
    chipText: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
    },
    chipTextActive: { color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontWeight: "600" },
  });
  return (
    <View style={styles.row} testID={testID ?? "filter-chips"}>
      {chips.map((c) => {
        const active = c.key === selected;
        return (
          <Pressable
            key={c.key}
            accessibilityRole="tab"
            accessibilityState={{ selected: active }}
            onPress={() => onSelect(c.key)}
            style={[styles.chip, active && styles.chipActive]}
          >
            <Text style={[styles.chipText, active && styles.chipTextActive]}>{c.label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}
