import type { ReactNode } from "react";
import { useMemo } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import type { AppTheme } from "@/theme/theme";
import { PaseoScreenTitle } from "./PaseoScreenTitle";

export type ShellRoute = "home" | "tasks" | "agents" | "me";

export interface PaseoShellProps {
  route: ShellRoute;
  children: ReactNode;
  onNavigate(route: ShellRoute): void;
}

interface PaseoTabBarProps {
  active: ShellRoute;
  onSelect(route: ShellRoute): void;
  styles: PaseoShellStyles;
}

const routeLabels: Readonly<Record<ShellRoute, string>> = {
  home: "首页",
  tasks: "任务",
  agents: "智能体",
  me: "我的",
};

const tabs: readonly ShellRoute[] = ["home", "tasks", "agents", "me"];

function routeLabel(route: ShellRoute): string {
  return routeLabels[route];
}

function PaseoTabBar({ active, onSelect, styles }: PaseoTabBarProps) {
  return (
    <View style={styles.tabBar}>
      {tabs.map((route) => {
        const selected = route === active;
        const label = routeLabel(route);

        return (
          <Pressable
            key={route}
            accessibilityRole="button"
            accessibilityLabel={label}
            accessibilityState={{ selected }}
            onPress={() => onSelect(route)}
            testID={`paseo-tab-${route}`}
            style={({ pressed }) => [styles.tab, selected && styles.tabActive, pressed && styles.tabPressed]}
          >
            <Text style={[styles.tabLabel, selected && styles.tabLabelActive]}>{label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}

export function PaseoShell({ route, children, onNavigate }: PaseoShellProps) {
  const { theme } = useTheme();
  const styles = useMemo(() => createStyles(theme), [theme]);

  return (
    <SafeAreaView testID="paseo-shell" style={styles.safeArea}>
      <PaseoScreenTitle testID="paseo-shell-title">{routeLabel(route)}</PaseoScreenTitle>
      <View style={styles.content}>{children}</View>
      <PaseoTabBar active={route} onSelect={onNavigate} styles={styles} />
    </SafeAreaView>
  );
}

function createStyles(theme: AppTheme) {
  return StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: theme.c.canvas,
  },
  content: {
    flex: 1,
    paddingHorizontal: theme.size["page-padding"],
  },
  tabBar: {
    minHeight: theme.size["tab-height"],
    flexDirection: "row",
    borderTopWidth: 1,
    borderTopColor: theme.c.line,
    backgroundColor: theme.c.surface,
  },
  tab: {
    flex: 1,
    minHeight: theme.size.touch,
    justifyContent: "center",
    alignItems: "center",
  },
  tabActive: {
    backgroundColor: theme.c["brand-soft"],
  },
  tabPressed: {
    opacity: 0.75,
  },
  tabLabel: {
    color: theme.c.muted,
    fontSize: theme.type.caption.fontSize,
    fontWeight: theme.type.label.fontWeight as "600",
  },
  tabLabelActive: {
    color: theme.c.brand,
  },
  });
}

type PaseoShellStyles = ReturnType<typeof createStyles>;
