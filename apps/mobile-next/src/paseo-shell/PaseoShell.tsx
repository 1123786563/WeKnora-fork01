import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { generatedTokens } from "@/theme/tokens.generated";
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

function PaseoTabBar({ active, onSelect }: PaseoTabBarProps) {
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
  return (
    <SafeAreaView style={styles.safeArea}>
      <PaseoScreenTitle testID="paseo-shell-title">{routeLabel(route)}</PaseoScreenTitle>
      <View style={styles.content}>{children}</View>
      <PaseoTabBar active={route} onSelect={onNavigate} />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: generatedTokens.colors.light.canvas,
  },
  content: {
    flex: 1,
    paddingHorizontal: generatedTokens.size["page-padding"],
  },
  tabBar: {
    minHeight: generatedTokens.size["tab-height"],
    flexDirection: "row",
    borderTopWidth: 1,
    borderTopColor: generatedTokens.colors.light.line,
    backgroundColor: generatedTokens.colors.light.surface,
  },
  tab: {
    flex: 1,
    minHeight: generatedTokens.size.touch,
    justifyContent: "center",
    alignItems: "center",
  },
  tabActive: {
    backgroundColor: generatedTokens.colors.light["brand-soft"],
  },
  tabPressed: {
    opacity: 0.75,
  },
  tabLabel: {
    color: generatedTokens.colors.light.muted,
    fontSize: generatedTokens.typography.caption.fontSize,
    fontWeight: generatedTokens.typography.label.fontWeight as "600",
  },
  tabLabelActive: {
    color: generatedTokens.colors.light.brand,
  },
});
