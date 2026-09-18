import React, { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme, type ThemePreference } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { ActionButton } from "@/components/ActionButton";
import { DecisionSheet } from "@/components/DecisionSheet";
import { SectionHeader } from "@/components/SearchField";

// M17 我的（tab 页，无返回键）：账户、当前空间、外观/通知偏好、空间用量入口与退出登录。
const APPEARANCE_OPTIONS: Array<{ key: ThemePreference; label: string }> = [
  { key: "system", label: "跟随系统" },
  { key: "light", label: "浅色" },
  { key: "dark", label: "深色" },
];

export default function ProfileScreen() {
  const { theme, preference, setPreference } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [logoutVisible, setLogoutVisible] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[16],
      paddingHorizontal: theme.space[20],
    },
    headerTitle: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    headerSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    userCard: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[16],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[20],
    },
    avatar: {
      width: 52,
      height: 52,
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c.brand,
      alignItems: "center",
      justifyContent: "center",
    },
    avatarText: { color: theme.c["on-brand"], fontWeight: "700", fontSize: theme.type.subtitle.fontSize },
    userName: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    userEmail: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      gap: theme.space[8],
    },
    row: { flexDirection: "row", alignItems: "center", gap: theme.space[12] },
    spaceName: { fontSize: theme.type["body-sm"].fontSize + 1, fontWeight: "600", color: theme.c.ink, flex: 1 },
    spaceSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    rowCard: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      marginBottom: theme.space[12],
    },
    rowIconWrap: {
      width: 36,
      height: 36,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["surface-alt"],
      alignItems: "center",
      justifyContent: "center",
    },
    rowName: { fontSize: theme.type["body-sm"].fontSize + 1, fontWeight: "600", color: theme.c.ink },
    rowSub: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, marginTop: 2 },
    chips: { flexDirection: "row", flexWrap: "wrap", gap: theme.space[8], marginTop: theme.space[12] },
    chip: {
      minHeight: 36,
      borderRadius: theme.radius.pill,
      paddingHorizontal: theme.space[16],
      alignItems: "center",
      justifyContent: "center",
      backgroundColor: theme.c.surface,
      borderWidth: 1,
      borderColor: theme.c.line,
    },
    chipActive: { backgroundColor: theme.c["brand-soft"], borderColor: theme.c["brand-soft"] },
    chipText: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    chipTextActive: { color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontWeight: "600" },
    section: { marginTop: theme.space[24] },
    version: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, textAlign: "center", marginTop: theme.space[24] },
  });

  const identity = app.identity;
  const membership = identity?.memberships.find((m) => m.tenantId === identity?.selectedTenantId) ?? null;
  const spaceName = membership?.tenantName ?? "未选择空间";

  const confirmLogout = async () => {
    setLoggingOut(true);
    try {
      await app.logout();
      router.replace("/login");
    } finally {
      setLoggingOut(false);
      setLogoutVisible(false);
    }
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>我的</Text>
        <Text style={styles.headerSub}>账户、偏好与空间</Text>
      </View>
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.userCard}>
          <View style={styles.avatar} accessibilityLabel="用户头像">
            <Text style={styles.avatarText}>{(identity?.displayName || identity?.email || "W").slice(0, 1).toUpperCase()}</Text>
          </View>
          <View style={{ flex: 1 }}>
            <Text style={styles.userName} numberOfLines={1}>
              {identity?.displayName || identity?.email || "未登录"}
            </Text>
            <Text style={styles.userEmail} numberOfLines={1}>
              {identity?.email ?? "邮箱待同步"}
            </Text>
          </View>
        </View>

        <View style={styles.section}>
          <SectionHeader title="当前空间" />
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={`切换空间，当前 ${spaceName}`}
            onPress={() => router.push("/spaces")}
            style={({ pressed }) => [styles.card, pressed && { opacity: 0.8 }]}
          >
            <View style={styles.row}>
              <Icon name="layers" size={20} color={theme.c.brand} />
              <View style={{ flex: 1 }}>
                <Text style={styles.spaceName}>{spaceName}</Text>
                <Text style={styles.spaceSub}>
                  {membership ? `${membership.role === "owner" ? "Owner" : membership.role} · 空间数据独立` : "角色待同步 · 空间数据独立"}
                </Text>
              </View>
              <Icon name="chevron" size={16} color={theme.c.subtle} />
            </View>
          </Pressable>
        </View>

        <View style={styles.section}>
          <SectionHeader title="偏好" />
          <View style={styles.card}>
            <View style={styles.row}>
              <Icon name={theme.mode === "dark" ? "moon" : "sun"} size={20} color={theme.c.muted} />
              <View style={{ flex: 1 }}>
                <Text style={styles.rowName}>外观</Text>
                <Text style={styles.rowSub}>跟随系统的全局主题</Text>
              </View>
            </View>
            <View style={styles.chips}>
              {APPEARANCE_OPTIONS.map((opt) => {
                const active = preference === opt.key;
                return (
                  <Pressable
                    key={opt.key}
                    accessibilityRole="tab"
                    accessibilityLabel={`外观 ${opt.label}`}
                    accessibilityState={{ selected: active }}
                    onPress={() => setPreference(opt.key)}
                    style={[styles.chip, active && styles.chipActive]}
                  >
                    <Text style={[styles.chipText, active && styles.chipTextActive]}>{opt.label}</Text>
                  </Pressable>
                );
              })}
            </View>
          </View>
          <View style={styles.rowCard}>
            <View style={styles.rowIconWrap}>
              <Icon name="bell" size={18} color={theme.c.muted} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={styles.rowName}>任务通知</Text>
              <Text style={styles.rowSub}>通知仅作提示，不影响任务</Text>
            </View>
          </View>
        </View>

        <View style={styles.section}>
          <SectionHeader title="空间" />
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="查看空间用量"
            onPress={() => router.push("/usage")}
            style={({ pressed }) => [styles.rowCard, pressed && { opacity: 0.8 }]}
          >
            <View style={styles.rowIconWrap}>
              <Icon name="pie" size={18} color={theme.c.muted} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={styles.rowName}>空间用量</Text>
              <Text style={styles.rowSub}>额度、预占与结算</Text>
            </View>
            <Icon name="chevron" size={16} color={theme.c.subtle} />
          </Pressable>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="查看执行目标"
            onPress={() => router.push("/targets")}
            style={({ pressed }) => [styles.rowCard, pressed && { opacity: 0.8 }]}
          >
            <View style={styles.rowIconWrap}>
              <Icon name="monitor" size={18} color={theme.c.muted} />
            </View>
            <View style={{ flex: 1 }}>
              <Text style={styles.rowName}>我的执行目标</Text>
              <Text style={styles.rowSub}>平台与受控远程节点</Text>
            </View>
            <Icon name="chevron" size={16} color={theme.c.subtle} />
          </Pressable>
        </View>

        <View style={{ marginTop: theme.space[24] }}>
          <ActionButton
            label="退出登录"
            variant="danger"
            icon="logout"
            block
            onPress={() => setLogoutVisible(true)}
            accessibilityHint="打开确认层；服务端任务继续运行"
          />
        </View>
        <Text style={styles.version}>WeKnora Mobile</Text>
      </ScrollView>

      <DecisionSheet
        visible={logoutVisible}
        title="退出登录"
        impact="退出登录会清除本机凭证与可见数据；服务端任务继续运行。"
        frozenSummary={[`账户：${identity?.email ?? "待同步"}`, `空间：${spaceName}`]}
        confirm={{ label: "退出登录", variant: "danger", onPress: () => void confirmLogout(), busy: loggingOut }}
        dismiss={{ label: "取消", variant: "secondary", onPress: () => setLogoutVisible(false) }}
      />
    </View>
  );
}
