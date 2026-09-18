import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  FlatList,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { StateView } from "@/components/StateView";
import type { SessionWire } from "@/contracts/auth";
import {
  ChatProjection,
  type ChatMessage,
  type ChatMessagePart,
} from "@/features/conversations/chat/ChatService";

// M07 对话（02 规格）：POST /agent-chat/:session_id 真实发送 + SSE 流式渲染（RW-015 接线）。
// 发送失败保留草稿；断流提示可续传（continue-stream 端点在后续恢复控制器接线）。

/** 距底部小于该值视为"接近底部"，此时内容变化才自动滚底 */
const NEAR_BOTTOM_THRESHOLD = 80;

export default function ConversationScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{ sessionId: string }>();
  const sessionId = typeof params.sessionId === "string" ? params.sessionId : "";

  const [session, setSession] = useState<SessionWire | null>(null);
  const [agentName, setAgentName] = useState<string | null>(null);
  const [view, setView] = useState<"loading" | "ready" | "error">("loading");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [streamError, setStreamError] = useState<string | null>(null);
  const [disconnected, setDisconnected] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const projectionRef = useRef<ChatProjection>(new ChatProjection());
  const listRef = useRef<FlatList<ChatMessage> | null>(null);
  const nearBottomRef = useRef(true);

  const load = useCallback(async () => {
    if (!sessionId) {
      setView("ready");
      return;
    }
    setView("loading");
    try {
      const res = await app.api.sessions({ page: 1, page_size: 50 });
      const found = res.sessions.find((s) => s.id === sessionId) ?? null;
      setSession(found);
      setView("ready");
      if (found?.agent_id) {
        try {
          const agents = await app.api.agents();
          setAgentName(agents.find((a) => a.id === found.agent_id)?.name ?? null);
        } catch {
          setAgentName(null);
        }
      }
    } catch {
      setSession(null);
      setView("ready");
    }
  }, [app, sessionId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const { layoutMeasurement, contentOffset, contentSize } = e.nativeEvent;
    const distanceFromBottom = contentSize.height - layoutMeasurement.height - contentOffset.y;
    nearBottomRef.current = distanceFromBottom < NEAR_BOTTOM_THRESHOLD;
  };

  const handleContentSizeChange = () => {
    if (nearBottomRef.current) {
      listRef.current?.scrollToEnd({ animated: true });
    }
  };

  const syncFromProjection = () => setMessages([...projectionRef.current.list]);

  const handleSend = async () => {
    const text = draft.trim();
    if (!text || sending || !sessionId) return;
    setSending(true);
    setStreamError(null);
    setDisconnected(false);
    const keptDraft = draft; // 发送失败恢复草稿
    setDraft("");
    try {
      await app.chat.send(
        sessionId,
        { query: text, agent_id: session?.agent_id ?? undefined },
        projectionRef.current,
        {
          onUpdate: syncFromProjection,
          onDone: syncFromProjection,
          onError: (e) => {
            setStreamError(e.message);
            syncFromProjection();
          },
          onDisconnected: (p) => {
            setDisconnected(true);
            syncFromProjection();
            void p;
          },
        },
      );
    } finally {
      setSending(false);
      // 失败且用户未输入新内容时恢复草稿
      setDraft((current) => (current === "" ? keptDraft : current));
    }
  };

  const styles = StyleSheet.create({
    wrap: { flex: 1, backgroundColor: theme.c.bg },
    header: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[4],
      paddingTop: insets.top + theme.space[12],
      paddingHorizontal: theme.space[8],
      paddingBottom: theme.space[8],
      borderBottomWidth: 1,
      borderBottomColor: theme.c.line,
      backgroundColor: theme.c.bg,
    },
    headerTitle: {
      flex: 1,
      fontSize: theme.type.subtitle.fontSize,
      lineHeight: theme.type.subtitle.lineHeight,
      fontWeight: theme.type.subtitle.fontWeight as "600",
      color: theme.c.ink,
    },
    headerSub: {
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.muted,
      marginTop: 1,
    },
    list: { paddingHorizontal: theme.space[20], paddingVertical: theme.space[16] },
    dateLine: {
      alignSelf: "center",
      fontSize: theme.type.caption.fontSize,
      lineHeight: theme.type.caption.lineHeight,
      color: theme.c.subtle,
      marginBottom: theme.space[12],
    },
    userBubble: {
      alignSelf: "flex-end",
      maxWidth: "85%" as const,
      backgroundColor: theme.c.brand,
      borderRadius: theme.radius.card,
      borderBottomRightRadius: theme.radius.xs,
      paddingHorizontal: theme.space[16],
      paddingVertical: theme.space[12],
      marginBottom: theme.space[12],
    },
    userText: { color: theme.c["on-brand"], fontSize: theme.type.body.fontSize, lineHeight: theme.type.body.lineHeight },
    assistantRow: {
      alignSelf: "flex-start",
      maxWidth: "92%" as const,
      marginBottom: theme.space[16],
      gap: theme.space[8],
    },
    assistantLabel: { flexDirection: "row", alignItems: "center", gap: theme.space[4] },
    assistantLabelText: { fontSize: theme.type.caption.fontSize, color: theme.c.muted, fontWeight: "600" },
    assistantText: {
      color: theme.c.ink,
      fontSize: theme.type.body.fontSize,
      lineHeight: theme.type.body.lineHeight,
    },
    toolCard: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["surface-alt"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    toolCardText: { flex: 1, fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    thinkingText: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.subtle, fontStyle: "italic" },
    errorText: { color: theme.c.danger, fontSize: theme.type["body-sm"].fontSize },
    streamingDot: { color: theme.c.brand, fontSize: theme.type.caption.fontSize },
    composer: {
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
      paddingHorizontal: theme.space[12],
      paddingTop: theme.space[8],
      paddingBottom: insets.bottom + theme.space[8],
      gap: theme.space[4],
    },
    notice: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[8],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["info-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    noticeText: {
      flex: 1,
      fontSize: theme.type["body-sm"].fontSize,
      lineHeight: theme.type["body-sm"].lineHeight,
      color: theme.c.info,
    },
    noticeWarn: { backgroundColor: theme.c["warning-soft"] },
    noticeWarnText: { color: theme.c.warning },
    composerRow: { flexDirection: "row", alignItems: "flex-end", gap: theme.space[4] },
    input: {
      flex: 1,
      minHeight: theme.size["icon-touch"],
      maxHeight: 120,
      borderRadius: theme.radius.control,
      borderWidth: 1,
      borderColor: theme.c["control-line"],
      backgroundColor: theme.c.surface,
      paddingHorizontal: theme.space[16],
      paddingVertical: theme.space[12],
      fontSize: theme.type.body.fontSize,
      lineHeight: theme.type.body.lineHeight,
      color: theme.c.ink,
      textAlignVertical: "center",
    },
  });

  const renderPart = (part: ChatMessagePart, key: string) => {
    switch (part.kind) {
      case "text":
        return (
          <Text key={key} style={styles.assistantText}>
            {part.text}
          </Text>
        );
      case "thinking":
        return (
          <Text key={key} style={styles.thinkingText}>
            {part.text}
          </Text>
        );
      case "tool_call":
      case "tool_result":
        return (
          <View key={key} style={styles.toolCard}>
            <Icon name={part.kind === "tool_call" ? "settings" : "checkcircle"} size={14} color={theme.c.muted} />
            <Text style={styles.toolCardText} numberOfLines={2}>
              {part.label}
            </Text>
          </View>
        );
      case "references":
        return (
          <Pressable key={key} accessibilityRole="button" accessibilityLabel="查看引用" style={styles.toolCard}>
            <Icon name="book" size={14} color={theme.c.brand} />
            <Text style={[styles.toolCardText, { color: theme.c.brand }]} numberOfLines={1}>
              {part.text}
            </Text>
            <Icon name="chevron" size={12} color={theme.c.subtle} />
          </Pressable>
        );
      case "error":
        return (
          <Text key={key} style={styles.errorText} accessibilityRole="alert">
            {part.text}
          </Text>
        );
    }
  };

  const title = session?.title || "会话";
  const today = new Date();
  const dateLine = `${today.getFullYear()} 年 ${today.getMonth() + 1} 月 ${today.getDate()} 日 · 当前空间`;

  return (
    <KeyboardAvoidingView style={styles.wrap} behavior={Platform.OS === "ios" ? "padding" : undefined} keyboardVerticalOffset={0}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <View style={{ flex: 1 }}>
          <Text style={styles.headerTitle} numberOfLines={1} accessibilityRole="header">
            {title}
          </Text>
          {agentName && <Text style={styles.headerSub}>{agentName}</Text>}
        </View>
      </View>

      {view === "loading" ? (
        <StateView state={{ kind: "loading" }} />
      ) : view === "error" ? (
        <StateView state={{ kind: "error", retry: () => void load() }} />
      ) : (
        <FlatList
          ref={listRef}
          data={messages}
          inverted={false}
          keyExtractor={(m) => m.stableId}
          renderItem={({ item }) =>
            item.role === "user" ? (
              <View style={styles.userBubble} accessibilityLabel={`我的消息：${(item.parts[0] as { text: string })?.text ?? ""}`}>
                <Text style={styles.userText}>{(item.parts[0] as { text: string })?.text ?? ""}</Text>
              </View>
            ) : (
              <View style={styles.assistantRow}>
                <View style={styles.assistantLabel}>
                  <Icon name="spark" size={12} color={theme.c.brand} />
                  <Text style={styles.assistantLabelText}>{agentName ?? "Agent"}</Text>
                  {item.streaming && <Text style={styles.streamingDot}> · 正在输入…</Text>}
                </View>
                {item.parts.map((p, i) => renderPart(p, `${item.stableId}_${i}`))}
              </View>
            )
          }
          ListHeaderComponent={<Text style={styles.dateLine}>{dateLine}</Text>}
          ListEmptyComponent={
            <StateView
              state={{
                kind: "empty",
                title: "这里还没有消息",
                description: "输入内容开始这次协作。",
              }}
            />
          }
          onScroll={handleScroll}
          onContentSizeChange={handleContentSizeChange}
          scrollEventThrottle={16}
          contentContainerStyle={styles.list}
          keyboardShouldPersistTaps="handled"
        />
      )}

      <View style={styles.composer}>
        {streamError && (
          <View style={[styles.notice, styles.noticeWarn]} accessibilityRole="alert">
            <Icon name="alert" size={16} color={theme.c.warning} />
            <Text style={[styles.noticeText, styles.noticeWarnText]}>{streamError} · 草稿已保留</Text>
          </View>
        )}
        {disconnected && !streamError && (
          <View style={styles.notice} accessibilityRole="alert">
            <Icon name="wifi" size={16} color={theme.c.info} />
            <Text style={styles.noticeText}>连接中断，回复可能未完成；重新进入会话可继续</Text>
          </View>
        )}
        <View style={styles.composerRow}>
          <TextInput
            value={draft}
            onChangeText={setDraft}
            placeholder="输入消息…"
            placeholderTextColor={theme.c.subtle}
            multiline
            style={styles.input}
            accessibilityLabel="输入消息"
            testID="conversation-input"
          />
          <IconButton name="send" label={sending ? "发送中" : "发送"} onPress={() => void handleSend()} disabled={sending} testID="conversation-send" />
          <IconButton name="mic" label="语音输入" onPress={() => router.push("/voice")} testID="conversation-mic" />
        </View>
      </View>
    </KeyboardAvoidingView>
  );
}
