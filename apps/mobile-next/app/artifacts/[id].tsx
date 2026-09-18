import React, { useCallback, useEffect, useState } from "react";
import { Platform, ScrollView, StyleSheet, Text, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import * as Sharing from "expo-sharing";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { StateView } from "@/components/StateView";
import type { ArtifactWire } from "@/contracts/workbench";

// M14 任务成果：不可变版本 + 来源任务；下载/分享走后端签名链接（真实 bytes 落本地缓存），
// 签名过期自动重授权一次；部署未启用签名时如实提示。禁止 WebView 承载不可信内容。
const FIXTURE_CONTENT = [
  "# 客户反馈分析摘要",
  "",
  "第 38 周 · 12 条反馈 · 产品研发空间",
  "",
  "## 01 / 主要发现",
  "用户最关注任务进度是否清晰，以及离开页面后，是否仍然可以安心等待结果。",
  "",
  "## 02 / 优先处理建议",
  "[P1] 明确登录失败原因，给出可操作的恢复指引。",
  "[P1] 区分执行中、等待审批与停止待确认。",
  "[P2] 完成后提供成果入口，而非只有一条通知。",
  "",
  "## 03 / 依据",
  "客户反馈记录与产品访谈纪要。以上均为原型演示数据，不是真实模型输出。",
].join("\n");

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; artifact: ArtifactWire }
  | { kind: "empty"; description?: string }
  | { kind: "error"; retry: () => void }
  | { kind: "forbidden" };

const TEXTUAL = /text\/|json|xml|csv/;

export default function ArtifactDetailScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ id?: string; runId?: string }>();
  const id = Array.isArray(params.id) ? params.id[0] : params.id;
  const runId = Array.isArray(params.runId) ? params.runId[0] : params.runId;
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [notice, setNotice] = useState<string | null>(null);
  const [noticeTone, setNoticeTone] = useState<"info" | "warn">("warn");
  const [busy, setBusy] = useState<null | "download" | "share">(null);
  const [localUri, setLocalUri] = useState<string | null>(null);
  const [preview, setPreview] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) {
      setState({ kind: "empty", description: "缺少成果编号。" });
      return;
    }
    if (app.visualFixture) {
      setState({
        kind: "ready",
        artifact: {
          id,
          name: "客户反馈分析.md",
          mime: "text/markdown",
          version: "3",
          size: 12 * 1024,
          source_run: runId ?? "run_demo_feedback",
          created_at: "今天 14:32",
          index: 0,
        },
      });
      setPreview(FIXTURE_CONTENT);
      return;
    }
    if (!runId) {
      setState({ kind: "empty", description: "缺少来源任务编号，无法读取成果。" });
      return;
    }
    setState({ kind: "loading" });
    try {
      const list = await app.api.artifacts(runId);
      const artifact = list.find((a) => a.id === id);
      if (!artifact) {
        setState({ kind: "empty", description: "成果不存在或你已无访问权限。" });
        return;
      }
      setState({ kind: "ready", artifact });
    } catch (e) {
      const err = e as { kind?: string };
      if (err.kind === "forbidden") {
        setState({ kind: "forbidden" });
        return;
      }
      setState({ kind: "error", retry: () => void load() });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [app.visualFixture, id, runId]);

  useEffect(() => {
    void load();
  }, [load]);

  const readPreviewIfTextual = async (uri: string, mime: string, name: string) => {
    if (!(TEXTUAL.test(mime) || /\.(md|txt|csv|json)$/i.test(name))) return;
    try {
      const fs = require("expo-file-system") as typeof import("expo-file-system");
      const file = new fs.File(uri);
      const text = await file.text();
      if (text) setPreview(text.slice(0, 4000));
    } catch {
      // 预览尽力而为：读取失败不影响下载结果
    }
  };

  const doDownload = async (artifact: ArtifactWire) => {
    if (!runId) {
      setNoticeTone("warn");
      setNotice("缺少来源任务编号，无法下载。");
      return;
    }
    setBusy("download");
    setNotice(null);
    try {
      const outcome = await app.downloadArtifact(runId, artifact);
      switch (outcome.kind) {
        case "downloaded":
          setLocalUri(outcome.localUri);
          setNoticeTone("info");
          setNotice("已下载到本地缓存，可在此页面预览或分享。");
          await readPreviewIfTextual(outcome.localUri, artifact.mime, artifact.name);
          break;
        case "signing_disabled":
          setNoticeTone("warn");
          setNotice("此部署未启用签名下载（需配置 WEKNORA_ARTIFACT_SIGNING_KEY）。");
          break;
        case "grant_expired":
          setNoticeTone("warn");
          setNotice("下载授权已过期（已自动重新授权一次仍失败），请稍后重试。");
          break;
        case "forbidden":
          setNoticeTone("warn");
          setNotice("没有下载此成果的权限。");
          break;
        case "not_found":
          setNoticeTone("warn");
          setNotice("成果已不存在或已被清理。");
          break;
        case "network":
          setNoticeTone("warn");
          setNotice(outcome.message);
          break;
      }
    } finally {
      setBusy(null);
    }
  };

  const doShare = async (artifact: ArtifactWire) => {
    setBusy("share");
    setNotice(null);
    try {
      let uri = localUri;
      if (!uri) {
        const outcome = await app.downloadArtifact(runId ?? "", artifact);
        if (outcome.kind !== "downloaded") {
          setNoticeTone("warn");
          setNotice(outcome.kind === "signing_disabled"
            ? "此部署未启用签名下载（需配置 WEKNORA_ARTIFACT_SIGNING_KEY）。"
            : "分享前需要先完成下载，请重试。");
          return;
        }
        uri = outcome.localUri;
        setLocalUri(uri);
      }
      // 系统分享面板：只交出本地缓存文件，不携带登录态或长期凭证
      await Sharing.shareAsync(uri, { mimeType: artifact.mime, dialogTitle: artifact.name });
    } catch {
      setNoticeTone("warn");
      setNotice("系统分享不可用或已被取消。");
    } finally {
      setBusy(null);
    }
  };

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
    fileCard: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[16],
    },
    fileIconWrap: {
      width: 56,
      height: 56,
      borderRadius: theme.radius.card,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    fileName: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink, flex: 1 },
    fileMeta: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted, marginTop: theme.space[4] },
    metaLine: {
      marginTop: theme.space[16],
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
    },
    metaText: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, flex: 1 },
    preview: {
      marginTop: theme.space[16],
      backgroundColor: theme.c["surface-alt"],
      borderRadius: theme.radius.card,
      padding: theme.space[16],
    },
    previewText: {
      fontFamily: Platform.select({ ios: "Menlo", default: "monospace" }),
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight + 4,
      color: theme.c.ink,
    },
    placeholderNotice: {
      marginTop: theme.space[16],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    placeholderNoticeText: { color: theme.c.warning, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    infoNotice: {
      marginTop: theme.space[16],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    infoNoticeText: { color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    footer: {
      padding: theme.space[20],
      paddingBottom: insets.bottom + theme.space[12],
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
      gap: theme.space[12],
    },
    footerHint: { fontSize: theme.type.caption.fontSize, color: theme.c.subtle, textAlign: "center" },
  });

  const sizeLabel = (size: number | null): string => {
    if (size == null) return "";
    if (size < 1024) return `${size} B`;
    if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>任务成果</Text>
      </View>

      {state.kind === "loading" && <StateView state={{ kind: "loading" }} />}
      {state.kind === "forbidden" && <StateView state={{ kind: "forbidden" }} />}
      {state.kind === "error" && <StateView state={{ kind: "error", retry: state.retry }} />}
      {state.kind === "empty" && <StateView state={{ kind: "empty", title: "成果不存在", description: state.description ?? "资源不存在或你已无访问权限。" }} />}

      {state.kind === "ready" && (
        <>
          <ScrollView contentContainerStyle={styles.scroll}>
            <View style={styles.fileCard}>
              <View style={styles.fileIconWrap}>
                <Icon name="file" size={28} color={theme.c.brand} />
              </View>
              <View style={{ flex: 1 }}>
                <Text style={styles.fileName} numberOfLines={2}>
                  {state.artifact.name}
                </Text>
                <Text style={styles.fileMeta} numberOfLines={1}>
                  {`v${state.artifact.version}${state.artifact.size != null ? ` · ${sizeLabel(state.artifact.size)}` : ""}${
                    state.artifact.source_run ? ` · 来源任务 ${state.artifact.source_run}` : ""
                  }`}
                </Text>
              </View>
            </View>

            <View style={styles.metaLine}>
              <Icon name="lock" size={14} color={theme.c.subtle} />
              <Text style={styles.metaText}>只读版本 · 内容不可覆盖 · 当前空间内已授权成员</Text>
            </View>

            <View style={styles.preview} accessibilityLabel="成果正文预览">
              <Text style={styles.previewText} selectable>
                {preview ?? "下载后此处显示文本类成果的预览；二进制成果请下载后查看。"}
              </Text>
            </View>

            {notice && (
              <View
                style={noticeTone === "info" ? styles.infoNotice : styles.placeholderNotice}
                accessibilityRole="alert"
              >
                <Text style={noticeTone === "info" ? styles.infoNoticeText : styles.placeholderNoticeText}>{notice}</Text>
              </View>
            )}
          </ScrollView>

          <View style={styles.footer}>
            <View style={{ flexDirection: "row", gap: theme.space[12] }}>
              <View style={{ flex: 1 }}>
                <ActionButton
                  label={busy === "download" ? "下载中…" : "下载"}
                  variant="secondary"
                  icon="download"
                  block
                  busy={busy === "download"}
                  disabled={busy !== null}
                  onPress={() => void doDownload(state.artifact)}
                  testID="download-artifact"
                />
              </View>
              <View style={{ flex: 1 }}>
                <ActionButton
                  label={busy === "share" ? "准备分享…" : "分享"}
                  variant="secondary"
                  icon="share"
                  block
                  busy={busy === "share"}
                  disabled={busy !== null}
                  onPress={() => void doShare(state.artifact)}
                  testID="share-artifact"
                />
              </View>
            </View>
            <Text style={styles.footerHint}>下载与分享使用短时效签名链接，不携带登录态；每次访问重新校验权限</Text>
          </View>
        </>
      )}
    </View>
  );
}
