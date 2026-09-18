import React, { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { StatusBadge } from "@/components/StatusBadge";
import { StateView } from "@/components/StateView";
import type { IconName } from "@/components/Icon";

// M02 空间选择（02 规格）：身份、空间卡（角色/当前标识/独立计费说明）、明确切换操作。
export default function SpacesScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [switching, setSwitching] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const pick = async (tenantId: string) => {
    setError(null);
    setSwitching(tenantId);
    try {
      await app.switchSpace(tenantId);
      router.replace("/(tabs)");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSwitching(null);
    }
  };

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    scroll: { paddingHorizontal: theme.space[20], paddingTop: insets.top + theme.space[24], paddingBottom: theme.space[32] },
    title: {
      fontSize: 26,
      lineHeight: 39,
      fontWeight: "600",
      color: theme.c.ink,
    },
    hint: { color: theme.c.muted, fontSize: theme.type["body-sm"].fontSize, marginTop: theme.space[4], marginBottom: theme.space[20] },
    identity: {
      flexDirection: "row",
      gap: theme.space[12],
      alignItems: "center",
      marginBottom: theme.space[16],
    },
    identityName: { fontSize: theme.type.subtitle.fontSize, fontWeight: "600", color: theme.c.ink },
    identityMail: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      borderWidth: 2,
      borderColor: "transparent",
      padding: theme.space[16],
      gap: theme.space[12],
      marginBottom: theme.space[12],
    },
    cardSelected: { borderColor: theme.c.brand },
    row: { flexDirection: "row", alignItems: "center", gap: theme.space[12] },
    grow: { flex: 1 },
    name: { fontSize: theme.type.subtitle.fontSize, fontWeight: "600", color: theme.c.ink },
    sub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    iconWrap: {
      width: 44,
      height: 44,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    checkRing: {
      width: 26,
      height: 26,
      borderRadius: 999,
      borderWidth: 2,
      borderColor: theme.c.brand,
      alignItems: "center",
      justifyContent: "center",
    },
    notice: {
      marginTop: theme.space[20],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    noticeText: { flex: 1, color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    error: { color: theme.c.danger, marginTop: theme.space[8], fontSize: theme.type["body-sm"].fontSize },
  });

  const memberships = app.identity?.memberships ?? [];
  const selected = app.identity?.selectedTenantId ?? null;

  return (
    <View style={styles.wrap}>
      <ScrollView contentContainerStyle={styles.scroll}>
        <Text style={styles.title}>选择你的工作空间</Text>
        <Text style={styles.hint}>知识、连接与额度，始终属于各自空间。</Text>

        {app.identity && (
          <View style={styles.identity}>
            <Icon name="user" size={18} color={theme.c.muted} />
            <View style={styles.grow}>
              <Text style={styles.identityName}>{app.identity.displayName || app.identity.email}</Text>
              <Text style={styles.identityMail}>{app.identity.email}</Text>
            </View>
          </View>
        )}

        {memberships.length === 0 ? (
          <StateView
            state={{ kind: "empty", title: "还没有可用的空间", description: "请联系管理员邀请你加入团队空间，或创建个人空间。" }}
          />
        ) : (
          memberships.map((m) => {
            const isSelected = m.tenantId === selected;
            const icon: IconName = m.role === "owner" && m.tenantName.includes("个人") ? "user" : "layers";
            return (
              <Pressable
                key={m.tenantId}
                accessibilityRole="button"
                accessibilityLabel={`进入空间 ${m.tenantName}`}
                accessibilityState={{ selected: isSelected, busy: switching === m.tenantId }}
                disabled={switching !== null}
                onPress={() => pick(m.tenantId)}
                style={[styles.card, isSelected && styles.cardSelected]}
              >
                <View style={styles.row}>
                  <View style={styles.iconWrap}>
                    <Icon name={icon} size={22} color={theme.c.brand} />
                  </View>
                  <View style={styles.grow}>
                    <Text style={styles.name}>{m.tenantName}</Text>
                    <Text style={styles.sub}>
                      {m.role === "owner" ? "Owner" : m.role} · 独立计费
                    </Text>
                  </View>
                  {isSelected && (
                    <View style={styles.checkRing}>
                      <Icon name="check" size={14} color={theme.c.brand} />
                    </View>
                  )}
                </View>
                <StatusBadge
                  domain="interaction"
                  label={m.tenantName.includes("个人") ? "个人使用" : "团队工作"}
                  tone="neutral"
                />
              </Pressable>
            );
          })
        )}

        <View style={styles.notice}>
          <Icon name="shield" size={16} color={theme.c.brand} />
          <Text style={styles.noticeText}>切换空间不会停止已提交的任务，也不会合并账单或余额。</Text>
        </View>
        {error && <Text style={styles.error}>{error}</Text>}
      </ScrollView>
    </View>
  );
}
