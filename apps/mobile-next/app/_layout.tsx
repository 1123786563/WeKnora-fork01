import React from "react";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { ThemeProvider, useTheme } from "@/theme/ThemeProvider";
import { AppProvider } from "@/host/AppProvider";

const APP_CONFIG = {
  // dev 默认连本地后端；用户可在 M01 覆盖（仅 https 受信，除非显式 dev 开关）
  defaultOrigin: process.env.EXPO_PUBLIC_WEKNORA_ORIGIN ?? "http://localhost:8082",
  allowLocalHttp: __DEV__,
  // 视觉对照开关：EXPO_PUBLIC_VISUAL_FIXTURE=1 npx expo start（dev only；演示数据仅隔离测试用）
  visualFixture: process.env.EXPO_PUBLIC_VISUAL_FIXTURE === "1" && __DEV__,
};

function RootStack() {
  const { theme } = useTheme();
  return (
    <>
      <StatusBar style={theme.mode === "dark" ? "light" : "dark"} />
      <Stack
        screenOptions={{
          headerShown: false,
          contentStyle: { backgroundColor: theme.c.bg },
          animation: "fade_from_bottom",
        }}
      >
        <Stack.Screen name="index" />
        <Stack.Screen name="login" />
        <Stack.Screen name="spaces" />
        <Stack.Screen name="(tabs)" />
      </Stack>
    </>
  );
}

export default function RootLayout() {
  return (
    <ThemeProvider>
      <AppProvider config={APP_CONFIG}>
        <RootStack />
      </AppProvider>
    </ThemeProvider>
  );
}
