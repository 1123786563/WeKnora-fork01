import React, { useEffect, useRef, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { router } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useTheme } from "@/theme/ThemeProvider";
import { useApp } from "@/host/AppProvider";
import { Icon } from "@/components/Icon";
import { IconButton } from "@/components/IconButton";
import { ActionButton } from "@/components/ActionButton";
import { FormField } from "@/components/FormField";

// M16 语音输入：先转文字、由用户确认后才进入草稿并手动发送；
// 不调用 expo-audio 真实录音（原生权限验证在后续任务），visualFixture 开启时模拟状态推进。
type VoicePhase = "idle" | "recording" | "transcribing" | "done";

const DRAFT_ID = "voice-draft";
const DEMO_TRANSCRIPT = "整理本周客户反馈，生成一份优先处理建议。";

/**
 * 迟到转写回填规则：仅当草稿为空（或纯空白）时回填转写结果；
 * 用户已编辑的草稿不被迟到的转写覆盖（RW-024 竞态保护，纯函数以便单测）。
 */
export function transcriptFill(current: string, transcript: string): string {
  return current.trim() ? current : transcript;
}

export default function VoiceScreen() {
  const { theme } = useTheme();
  const app = useApp();
  const insets = useSafeAreaInsets();
  const [phase, setPhase] = useState<VoicePhase>("idle");
  const [text, setText] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const phaseLabel = (): string => {
    switch (phase) {
      case "recording":
        return "正在录音…再次点击结束";
      case "transcribing":
        return "正在转写…";
      case "done":
        return "请确认文字";
      default:
        return "点击开始录音";
    }
  };

  const toggleRecord = () => {
    if (!app.visualFixture) {
      // 真实录音未接入：持久提示，不假装录音成功
      setNotice("录音能力将在原生环境验证后启用（当前为草稿编辑模式）");
      return;
    }
    if (phase === "idle" || phase === "done") {
      setNotice(null);
      setPhase("recording");
      return;
    }
    if (phase === "recording") {
      setPhase("transcribing");
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        setPhase("done");
        setText((current) => transcriptFill(current, DEMO_TRANSCRIPT));
      }, 1200);
    }
  };

  const toDraft = async () => {
    await app.store.saveDraft(app.scopeKey(), DRAFT_ID, text);
    router.back();
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
    center: {
      alignItems: "center",
      gap: theme.space[16],
      paddingVertical: theme.space[32],
    },
    orb: {
      width: 112,
      height: 112,
      borderRadius: theme.radius.pill,
      backgroundColor: theme.c["brand-soft"],
      alignItems: "center",
      justifyContent: "center",
    },
    orbRecording: { backgroundColor: theme.c.danger },
    phaseText: { fontSize: theme.type.subtitle.fontSize, lineHeight: theme.type.subtitle.lineHeight, fontWeight: "600", color: theme.c.ink },
    subText: { fontSize: theme.type["body-sm"].fontSize, color: theme.c.muted },
    waveform: {
      flexDirection: "row",
      alignItems: "center",
      gap: theme.space[4],
      height: 40,
    },
    bar: { width: 4, borderRadius: theme.radius.pill, backgroundColor: theme.c["control-line"] },
    fieldWrap: { marginTop: theme.space[16] },
    notice: {
      marginTop: theme.space[16],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["brand-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[12],
      flexDirection: "row",
      gap: theme.space[8],
      alignItems: "flex-start",
    },
    noticeText: { flex: 1, color: theme.mode === "light" ? theme.c.brand : theme.c["hero-ink"], fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    placeholderNotice: {
      marginTop: theme.space[12],
      borderRadius: theme.radius.control,
      backgroundColor: theme.c["warning-soft"],
      paddingHorizontal: theme.space[12],
      paddingVertical: theme.space[8],
    },
    placeholderNoticeText: { color: theme.c.warning, fontSize: theme.type["body-sm"].fontSize, lineHeight: theme.type["body-sm"].lineHeight },
    footer: {
      padding: theme.space[20],
      paddingBottom: insets.bottom + theme.space[12],
      borderTopWidth: 1,
      borderTopColor: theme.c.line,
      backgroundColor: theme.c.bg,
    },
  });

  const recording = phase === "recording" || phase === "transcribing";
  const barHeights = [12, 20, 32, 16, 28, 40, 24, 44, 30, 46, 34, 40, 22, 32, 18, 24];

  return (
    <View style={styles.wrap}>
      <View style={styles.header}>
        <IconButton name="back" label="返回" onPress={() => router.back()} />
        <Text style={styles.headerTitle}>语音输入</Text>
      </View>

      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.center}>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={phase === "recording" ? "结束录音" : "开始录音"}
            accessibilityState={{ disabled: phase === "transcribing" }}
            disabled={phase === "transcribing"}
            onPress={toggleRecord}
            style={({ pressed }) => [styles.orb, recording && styles.orbRecording, pressed && { opacity: 0.85 }]}
          >
            <Icon name="mic" size={40} color={recording ? (theme.mode === "light" ? "#FFFFFF" : theme.c["danger-soft"]) : theme.c.brand} />
          </Pressable>
          <Text style={styles.phaseText} accessibilityLabel={`录音状态：${phaseLabel()}`}>
            {phaseLabel()}
          </Text>
          <Text style={styles.subText}>语音交互演示 · 不访问麦克风</Text>
          <View style={styles.waveform} accessibilityLabel="录音状态指示" pointerEvents="none">
            {barHeights.map((h, i) => (
              <View
                key={i}
                style={[styles.bar, { height: recording && i % 2 === 0 ? theme.space[32] : h }]}
              />
            ))}
          </View>
        </View>

        <View style={styles.fieldWrap}>
          <FormField
            label="确认或编辑转写内容"
            value={text}
            onChangeText={setText}
            placeholder="录音转写后，你可以在这里修改文字…"
            multiline
          />
        </View>

        {notice && (
          <View style={styles.placeholderNotice} accessibilityRole="alert">
            <Text style={styles.placeholderNoticeText}>{notice}</Text>
          </View>
        )}

        <View style={styles.notice}>
          <Icon name="shield" size={16} color={theme.c.brand} />
          <Text style={styles.noticeText}>语音先转文字，由你确认后才进入草稿并手动发送。</Text>
        </View>
      </ScrollView>

      <View style={styles.footer}>
        <ActionButton
          label="放入草稿"
          icon="check"
          block
          disabled={!text.trim()}
          onPress={() => void toDraft()}
          accessibilityHint="保存到当前空间的新建任务草稿，不会自动发送"
        />
      </View>
    </View>
  );
}
