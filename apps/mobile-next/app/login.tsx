import React, { useState } from "react";
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { ActionButton } from "@/components/ActionButton";
import { FormField } from "@/components/FormField";
import { Icon } from "@/components/Icon";

// M01 登录（02 规格）：邮箱/密码、企业 SSO、可信服务器入口。
// 原型模拟文案（“进入演示空间”“演示无需真实密码”）按生产语义替换（decisions D-10）。
export default function LoginScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [serverUrl, setServerUrl] = useState(app.origin);
  const [showServer, setShowServer] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const emailValid = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());

  const submit = async () => {
    setError(null);
    if (!emailValid) {
      setError("请输入有效的邮箱地址");
      return;
    }
    if (!password) {
      setError("请输入密码");
      return;
    }
    setBusy(true);
    try {
      await app.setOrigin(serverUrl.trim());
      await app.auth.loginWithPassword(serverUrl.trim(), email.trim(), password);
      const stage = app.auth.currentIdentity ? "ready" : "pick";
      if (stage === "ready") router.replace("/(tabs)");
      else router.replace("/spaces");
    } catch (e) {
      // 401 通用失败：不暴露账号是否存在
      setError((e as Error).message.includes("Unauthorized") ? "邮箱或密码不正确" : (e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    hero: { alignItems: "center", paddingTop: insets.top + theme.space[48], gap: theme.space[12], marginBottom: theme.space[32] },
    brand: {
      width: 56,
      height: 56,
      borderRadius: theme.radius.card,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    title: {
      fontSize: theme.type.display.fontSize,
      lineHeight: theme.type.display.lineHeight,
      fontWeight: theme.type.display.fontWeight as "700",
      color: theme.c.ink,
      textAlign: "center",
    },
    sub: {
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.muted,
      textAlign: "center",
    },
    divider: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      marginVertical: theme.space[20],
    },
    dividerLine: { flex: 1, height: 1, backgroundColor: theme.c.line },
    dividerText: { color: theme.c.subtle, fontSize: theme.type["body-sm"].fontSize },
    notice: {
      marginTop: theme.space[20],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["info-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    noticeText: { flex: 1, color: theme.c.info, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    serverBtn: { alignItems: "center", paddingVertical: theme.space[12], marginTop: theme.space[8] },
    serverBtnText: { color: theme.c.brand, fontSize: theme.type["body-sm"].fontSize },
    form: { gap: theme.space[16] },
  });

  return (
    <KeyboardAvoidingView style={styles.wrap} behavior={Platform.OS === "ios" ? "padding" : undefined}>
      <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
        <View style={styles.hero}>
          <View style={styles.brand} accessibilityLabel="WeKnora 工作台">
            <Icon name="spark" size={28} color={theme.c.brand} />
          </View>
          <Text style={styles.title}>让想法开始，{"\n"}让工作继续。</Text>
          <Text style={styles.sub}>一个工作台，连接你的 Agent、知识与每一次协作。</Text>
        </View>

        <View style={styles.form}>
          <FormField
            label="邮箱"
            value={email}
            onChangeText={setEmail}
            placeholder="name@company.com"
            inputMode="email"
            autoComplete="email"
            error={error && !emailValid ? error : undefined}
          />
          <FormField
            label="密码"
            value={password}
            onChangeText={setPassword}
            placeholder="请输入密码"
            secure
            autoComplete="password"
            error={error && emailValid && password ? error : undefined}
          />
          {error && !error.includes("邮箱") && !error.includes("密码") && (
            <Text style={{ color: theme.c.danger, fontSize: theme.type["body-sm"].fontSize }}>{error}</Text>
          )}
          <ActionButton label={busy ? "正在登录…" : "登录"} onPress={submit} busy={busy} block icon="arrow" />
        </View>

        <View style={styles.divider}>
          <View style={styles.dividerLine} />
          <Text style={styles.dividerText}>或使用企业账户</Text>
          <View style={styles.dividerLine} />
        </View>
        <ActionButton
          label="企业单点登录"
          variant="secondary"
          block
          icon="shield"
          onPress={() => {
            // SSO：系统浏览器 PKCE + /auth/mobile/exchange（RW-008 后续模拟器实测）
            setError("企业单点登录需要连接部署的 OIDC 服务后使用");
          }}
        />

        <View style={styles.notice} accessibilityRole="alert">
          <Icon name="info" size={16} color={theme.c.info} />
          <Text style={styles.noticeText}>
            登录信息只发送到你指定的 WeKnora 服务器；密码不会保存在普通缓存中。
          </Text>
        </View>

        <Pressable style={styles.serverBtn} onPress={() => setShowServer((v) => !v)} accessibilityRole="button" accessibilityLabel="配置私有部署服务器">
          <Text style={styles.serverBtnText}>{showServer ? "收起服务器设置" : "私有部署服务器"}</Text>
        </Pressable>
        {showServer && (
          <FormField
            label="服务器地址"
            value={serverUrl}
            onChangeText={setServerUrl}
            placeholder="https://weknora.your-company.com"
            hint="仅支持受信任的 https 服务器；本地开发地址需在开发模式使用"
          />
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
