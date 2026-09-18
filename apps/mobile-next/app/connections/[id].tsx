import React, { useCallback, useEffect, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import * as WebBrowser from "expo-web-browser";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { StatusBadge } from "@/components/StatusBadge";
import { StateView } from "@/components/StateView";
import { DecisionSheet } from "@/components/DecisionSheet";
import { SectionHeader } from "@/components/SearchField";
import type { BadgeSpec } from "@/components/StatusBadge";
import type { ConnectionWire } from "@/contracts/workbench";

// M13 连接详情：Provider、归属（个人/空间）、状态、授权范围与允许的动作；
// 不向移动端下发或显示任何 token/secret；撤销不回滚外部已执行的操作。
const FIXTURE_CONNECTION: ConnectionWire = {
  id: "conn_demo_feishu",
  provider: "飞书连接",
  owner_scope: "tenant",
  status: "active",
  scopes: ["read:wiki", "write:wiki"],
  account_label: "project-bot",
};

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; connection: ConnectionWire }
  | { kind: "empty" }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

export default function ConnectionDetailScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ id?: string }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [revokeVisible, setRevokeVisible] = useState(false);
  const [revokeBusy, setRevokeBusy] = useState(false);
  const [placeholderNotice, setPlaceholderNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) {
      setState({ kind: "empty" });
      return;
    }
    if (app.visualFixture) {
      setState({ kind: "ready", connection: FIXTURE_CONNECTION });
      return;
    }
    setState({ kind: "loading" });
    try {
      const list = await app.api.connections();
      const connection = list.find((c) => c.id === id);
      if (!connection) {
        setState({ kind: "empty" });
        return;
      }
      setState({ kind: "ready", connection });
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") {
        setState({ kind: "forbidden" });
        return;
      }
      setState({ kind: "error", retry: () => void load() });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.visualFixture, id]);

  useEffect(() => {
    void load();
  }, [load]);

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      paddingTop: insets.top + theme.space[12],
      paddingBottom: theme.space[12],
      paddingHorizontal: theme.space[20],
    },
    headerTitle: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    scroll: { paddingHorizontal: theme.space[20], paddingBottom: theme.space[32] },
    card: {
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      gap: theme.space[12],
    },
    headRow: { flexDirection: "row", alignItems: "center", gap: theme.space[12] },
    iconWrap: {
      width: 44,
      height: 44,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["purple-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    providerName: { fontSize: theme.type.title.fontSize, lineHeight: theme.type.title.lineHeight, fontWeight: "600", color: theme.c.ink, flex: 1 },
    providerSub: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: 2 },
    badges: { flexDirection: "row", gap: theme.space[8], flexWrap: "wrap" },
    detailLine: {
      flexDirection: "row",
      justifyContent: "space-between",
      gap: theme.space[12],
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      paddingTop: theme.space[12],
    },
    detailLabel: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    detailValue: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.ink, fontWeight: "600", flexShrink: 1, textAlign: "right" },
    section: { marginTop: theme.space[24] },
    actionRow: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[12],
      backgroundColor: theme.c.surface,
      borderRadius: theme.radius.card,
      padding: theme.space[16],
      marginBottom: theme.space[12],
    },
    actionIconWrap: {
      width: 36,
      height: 36,
      borderRadius: theme.radius.sm,
      backgroundColor: theme.c["surface-alt"],
      alignItems: "center",
      justifyContent: "center",
    },
    actionName: { fontSize: theme.type["body-sm"].fontSize + 1, fontWeight: "600", color: theme.c.ink },
    actionSub: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, marginTop: 2 },
    warningNotice: {
      marginTop: theme.space[20],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    warningNoticeText: { flex: 1, color: theme.c.warning, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    placeholderNotice: {
      marginTop: theme.space[12],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["surface-alt"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    placeholderNoticeText: { color: theme.c.muted, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    footer: {
      padding: theme.space[20],
      paddingBottom: insets.bottom + theme.space[12],
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
      gap: theme.space[12],
    },
  });

  const ownerBadge = (scope: ConnectionWire["owner_scope"]): BadgeSpec =>
    scope === "personal"
      ? { domain: "interaction", label: "个人连接", tone: "neutral" }
      : scope === "tenant"
        ? { domain: "interaction", label: "空间连接", tone: "neutral" }
        : { domain: "interaction", label: "归属待同步", tone: "unknown", unknown: true };

  const statusBadge = (status: string): BadgeSpec => {
    if (status === "active") return { domain: "run", label: "可用", tone: "success" };
    if (status === "expired") return { domain: "run", label: "授权过期", tone: "danger" };
    return { domain: "run", label: "等待同步", tone: "unknown", unknown: true };
  };

  // 允许的动作：从授权范围推导（读取类已授权；写入类需审批），不虚构能力
  const allowedActions = (connection: ConnectionWire): Array<{ name: string; sub: string; badge: BadgeSpec }> => {
    if (app.visualFixture) {
      return [
        { name: "读取项目资料", sub: "仅已授权范围", badge: { domain: "interaction", label: "已授权", tone: "success" } },
        { name: "发布项目更新", sub: "每次写入需明确批准", badge: { domain: "interaction", label: "需审批", tone: "attention" } },
        { name: "删除项目", sub: "当前连接不支持", badge: { domain: "interaction", label: "不可用", tone: "neutral" } },
      ];
    }
    return connection.scopes.map((scope) => {
      const writable = /write|publish|post|send|delete/i.test(scope);
      return {
        name: scope,
        sub: writable ? "写操作需按次审批" : "读取授权范围",
        badge: writable
          ? { domain: "interaction", label: "需审批", tone: "attention" }
          : { domain: "interaction", label: "已授权", tone: "success" },
      };
    });
  };

  const reauthorize = async () => {
    // 占位：真实授权地址由部署的授权服务下发；此处尝试打开系统浏览器并给出提示，不假装成功
    try {
      await WebBrowser.openBrowserAsync(`${app.origin}/api/v1/apps/connections/${id ?? ""}/authorize`);
    } catch {
      // 打开失败不阻塞提示
    }
    setPlaceholderNotice("需要连接部署的授权服务");
  };

  const confirmRevoke = async () => {
    setRevokeBusy(true);
    try {
      // 服务端无撤销端点（能力待接入）：不假装成功，仅提示
      setRevokeVisible(false);
      setPlaceholderNotice("撤销请求需要部署的连接管理服务（能力待接入）");
    } finally {
      setRevokeBusy(false);
    }
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>连接详情</Text>
      </View>

      {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
      {state.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}
      {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
      {state.kind === "empty" && (
        <StateView state={{ kind: "empty", title: "连接不存在", description: "资源不存在或你已无访问权限。重新同步后再尝试。" }} />
      )}

      {state.kind === "ready" && (
        <>
          <ScrollView contentContainerStyle={styles.scroll}>
            <View style={styles.card}>
              <View style={styles.headRow}>
                <View style={styles.iconWrap}>
                  <Icon name="link" size={22} color={theme.c.purple} />
                </View>
                <View style={{ flex: 1 }}>
                  <Text style={styles.providerName} numberOfLines={2}>
                    {state.connection.provider}
                  </Text>
                  <Text style={styles.providerSub}>
                    {state.connection.account_label ? `授权账号 ${state.connection.account_label}` : "授权账号待同步"}
                  </Text>
                </View>
              </View>
              <View style={styles.badges}>
                <StatusBadge {...ownerBadge(state.connection.owner_scope)} />
                <StatusBadge {...statusBadge(state.connection.status)} />
              </View>
              <View style={styles.detailLine}>
                <Text style={styles.detailLabel}>凭据管理</Text>
                <Text style={styles.detailValue}>服务端受控保管</Text>
              </View>
            </View>

            <View style={styles.section}>
              <SectionHeader title="授权范围" />
              <View style={styles.card}>
                {state.connection.scopes.length > 0 ? (
                  state.connection.scopes.map((scope) => (
                    <View key={scope} style={styles.headRow}>
                      <Icon name="checkcircle" size={16} color={theme.c.brand} />
                      <Text style={styles.detailValue}>{scope}</Text>
                    </View>
                  ))
                ) : (
                  <Text style={styles.detailLabel}>暂无已授权范围</Text>
                )}
              </View>
            </View>

            <View style={styles.section}>
              <SectionHeader title="允许的动作" />
              {allowedActions(state.connection).map((action) => (
                <View key={action.name} style={styles.actionRow}>
                  <View style={styles.actionIconWrap}>
                    <Icon name={action.badge.tone === "attention" ? "shield" : action.badge.tone === "neutral" ? "lock" : "book"} size={18} color={theme.c.muted} />
                  </View>
                  <View style={{ flex: 1 }}>
                    <Text style={styles.actionName} numberOfLines={1}>
                      {action.name}
                    </Text>
                    <Text style={styles.actionSub} numberOfLines={1}>
                      {action.sub}
                    </Text>
                  </View>
                  <StatusBadge {...action.badge} />
                </View>
              ))}
            </View>

            <View style={styles.warningNotice} accessibilityRole="alert">
              <Icon name="alert" size={16} color={theme.c.warning} />
              <Text style={styles.warningNoticeText}>撤销不会回滚外部已执行的操作。</Text>
            </View>
            <View style={styles.warningNotice}>
              <Icon name="lock" size={16} color={theme.c.warning} />
              <Text style={styles.warningNoticeText}>移动端不下发 Provider 原始密钥或连接管理 Token。</Text>
            </View>

            {placeholderNotice && (
              <View style={styles.placeholderNotice} accessibilityRole="alert">
                <Text style={styles.placeholderNoticeText}>{placeholderNotice}</Text>
              </View>
            )}
          </ScrollView>

          <View style={styles.footer}>
            <ActionButton label="重新授权" variant="secondary" icon="refresh" block onPress={() => void reauthorize()} />
            <ActionButton
              label="撤销"
              variant="danger"
              icon="link"
              block
              onPress={() => setRevokeVisible(true)}
              accessibilityHint="打开确认层；撤销不会回滚外部已执行的操作"
            />
          </View>

          <DecisionSheet
            visible={revokeVisible}
            title="撤销连接"
            impact="撤销不会回滚外部已执行的操作，也不会删除审计记录。"
            frozenSummary={[
              `连接：${state.connection.provider}`,
              state.connection.owner_scope === "personal" ? "归属：个人连接" : "归属：空间连接",
              "后续调用将重新检查权限",
            ]}
            confirm={{ label: "确认撤销", variant: "danger", onPress: () => void confirmRevoke(), busy: revokeBusy }}
            dismiss={{ label: "返回", variant: "secondary", onPress: () => setRevokeVisible(false) }}
          />
        </>
      )}
    </View>
  );
}
