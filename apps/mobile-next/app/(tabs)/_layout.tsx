import React from "react";
import { Tabs } from "expo-router";
import { Platform } from "react-native";
import { useTheme } from "@/theme/ThemeProvider";
import { Icon, type IconName } from "@/components/Icon";

// 四个底部入口固定：工作台 / 会话 / 资源 / 我的（02 规格 §1；顺序不动态变换）
export default function TabsLayout() {
  const { theme } = useTheme();
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: theme.mode === "light" ? theme.c.brand : theme.c.accent,
        tabBarInactiveTintColor: theme.c.subtle,
        tabBarStyle: {
          backgroundColor: theme.c.surface,
          borderTopColor: theme.c.line,
          borderTopWidth: 1,
          height: theme.size["tab-height"] + (Platform.OS === "ios" ? 20 : 0),
          paddingBottom: Platform.OS === "ios" ? 20 : 6,
          paddingTop: 6,
        },
        tabBarLabelStyle: {
          fontSize: theme.type.caption.fontSize,
          lineHeight: theme.type.caption.lineHeight,
          fontWeight: "600",
        },
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: "工作台",
          tabBarIcon: ({ color }) => <TabIcon name="home" color={color} />,
        }}
      />
      <Tabs.Screen
        name="sessions"
        options={{
          title: "会话",
          tabBarIcon: ({ color }) => <TabIcon name="chat" color={color} />,
        }}
      />
      <Tabs.Screen
        name="resources"
        options={{
          title: "资源",
          tabBarIcon: ({ color }) => <TabIcon name="grid" color={color} />,
        }}
      />
      <Tabs.Screen
        name="profile"
        options={{
          title: "我的",
          tabBarIcon: ({ color }) => <TabIcon name="user" color={color} />,
        }}
      />
    </Tabs>
  );
}

function TabIcon({ name, color }: { name: IconName; color: string }) {
  return <Icon name={name} size={22} color={color} />;
}
