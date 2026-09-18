import { useEffect } from "react";
import { View } from "react-native";
import { router } from "expo-router";
import { useTheme } from "@/theme/ThemeProvider";
import { StateView } from "@/components/StateView";
import { useApp } from "@/host/AppProvider";

// 入口 gate：按 AuthStage 路由（冷启动恢复在 AppProvider.bootstrap 执行）
export default function Gate() {
  const { theme } = useTheme();
  const app = useApp();

  useEffect(() => {
    if (app.stage.kind === "login") router.replace("/login");
    else if (app.stage.kind === "pick_space") router.replace("/spaces");
    else if (app.stage.kind === "ready") router.replace("/(tabs)");
    else if (app.stage.kind === "server_untrusted") router.replace("/login");
    // booting / restoring 停留 loading
  }, [app.stage.kind]);

  return (
    <View style={{ flex: 1, backgroundColor: theme.c.bg, justifyContent: "center" }}>
      <StateView state={{ kind: "loading" }} />
    </View>
  );
}
