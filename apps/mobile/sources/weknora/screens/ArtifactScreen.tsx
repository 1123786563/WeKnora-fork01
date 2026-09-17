import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Image, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { Sheet } from '../ui/Sheet.tsx';
import { StatusBadge } from '../ui/StatusBadge.tsx';
import { StateView } from '../ui/StateView.tsx';
import {
  createArtifactViewer,
  type ArtifactVersion,
  type ArtifactAccess,
  type ArtifactAccessPort,
} from '../resources/artifact-viewer.ts';

/**
 * 任务成果页 M14（MX-024）：不可变版本预览——每次打开重新授权；过期→再授权重开；
 * 撤权→清内容提示；文本/图片/表格安全原生渲染；分享经确认 Sheet（不外流签名 URL）。
 */
export interface ArtifactScreenProps {
  version: ArtifactVersion;
  accessPort: ArtifactAccessPort;
  onBack?: () => void;
  testID?: string;
}

export function ArtifactScreen({ version, accessPort, onBack, testID }: ArtifactScreenProps) {
  const { theme } = useWeknoraTheme();
  const viewer = useMemo(() => createArtifactViewer({ access: accessPort }), [accessPort]);
  const [access, setAccess] = useState<ArtifactAccess | null>(null);
  const [phase, setPhase] = useState<'loading' | 'ready' | 'expired' | 'revoked' | 'not_found' | 'error'>('loading');
  const [confirmingShare, setConfirmingShare] = useState(false);
  const [shareNote, setShareNote] = useState<string | null>(null);

  const reopen = useCallback(async () => {
    setPhase('loading');
    setAccess(null);
    try {
      const next = await viewer.open(version.artifactId, version.version);
      setAccess(next);
      setPhase(next.status === 'authorized' ? 'ready' : next.status);
    } catch {
      setPhase('error');
    }
  }, [version.artifactId, version.version, viewer]);

  useEffect(() => {
    void reopen();
    return () => viewer.clearSessionCache(); // 离开页面清会话缓存
  }, [reopen, viewer]);

  const confirmShare = async (confirmed: boolean) => {
    setConfirmingShare(false);
    const outcome = await viewer.share(version, confirmed);
    setShareNote(outcome.scopeDescription + (outcome.shared ? '（已调起系统分享）' : ''));
  };

  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      <Card>
        <View style={styles.row}>
          <StatusBadge tone="brand" label={`v${version.version} 不可变版本`} />
          <Text accessibilityLabel={`成果 ${version.title}，版本 ${version.version}，来源任务 ${version.sourceRunId}`} style={{ color: theme.colors.muted, flex: 1, marginLeft: theme.spacing[8], fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>
            来源任务 {version.sourceRunId}
          </Text>
        </View>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.subtitle.fontSize, lineHeight: theme.typography.subtitle.lineHeight, fontWeight: '600', marginTop: theme.spacing[8] }}>
          {version.title}
        </Text>
      </Card>

      {phase === 'loading' ? <StateView kind="loading" message="正在重新授权并加载成果" /> : null}
      {phase === 'error' ? <StateView kind="error" message="成果加载失败" actionLabel="重试" onAction={() => void reopen()} /> : null}
      {phase === 'not_found' ? <StateView kind="error" message="成果不存在或已不可访问" actionLabel="重试" onAction={() => void reopen()} /> : null}
      {phase === 'revoked' ? (
        <StateView kind="noPermission" message="该成果的访问授权已被撤销" detail="内容已清除；如需访问请联系空间管理员重新授权" actionLabel="重新授权尝试" onAction={() => void reopen()} />
      ) : null}
      {phase === 'expired' ? (
        <StateView kind="error" message="访问授权已过期" detail="签名访问是短时授权——点击重新授权获取新的不可变版本访问" actionLabel="重新授权" onAction={() => void reopen()} />
      ) : null}

      {phase === 'ready' && access?.status === 'authorized' ? (
        <Card>
          <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>
            预览（{version.kind === 'text' ? '文本' : version.kind === 'image' ? '图片' : version.kind === 'table' ? '表格' : '文件'}）
          </Text>
          {version.kind === 'image' ? (
            <Image source={{ uri: access.signedUrl }} style={{ marginTop: theme.spacing[12], height: 220, borderRadius: theme.radius.control }} accessibilityLabel={`成果 ${version.title} 图片预览`} />
          ) : (
            <Text style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight, marginTop: theme.spacing[12] }}>
              {version.kind === 'text' || version.kind === 'table' ? '内容经安全原生渲染（文本/表格通道）；动态 HTML 已静态化处理。' : '文件类型成果将经重新授权后下载到临时目录查看。'}
            </Text>
          )}
          <Text style={{ color: theme.colors.subtle, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[12] }}>
            本此访问授权有效期至 {access.expiresAt}；签名链接不保存、不外流。
          </Text>
        </Card>
      ) : null}

      {shareNote ? (
        <Text accessibilityRole="alert" style={{ color: theme.colors.muted, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{shareNote}</Text>
      ) : null}

      <View style={{ gap: theme.spacing[12] }}>
        <Button label="分享（确认外流范围）" variant="secondary" disabled={phase !== 'ready'} onPress={() => setConfirmingShare(true)} accessibilityLabel={`分享成果 ${version.title}，需先确认外流范围`} />
        {onBack ? <Button label="返回" variant="secondary" onPress={onBack} accessibilityLabel="返回上一页" size="compact" /> : null}
      </View>

      <Sheet
        visible={confirmingShare}
        title="确认分享外流范围"
        onClose={() => setConfirmingShare(false)}
        footer={
          <View style={{ gap: theme.spacing[8] }}>
            <Button label="确认分享此版本" accessibilityLabel={`确认分享 ${version.title} v${version.version}（不含访问链接）`} onPress={() => void confirmShare(true)} />
            <Button label="取消" variant="secondary" accessibilityLabel="取消分享" onPress={() => void confirmShare(false)} size="compact" />
          </View>
        }
      >
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
          将分享「{version.title}」v{version.version}（不可变版本，来源任务 {version.sourceRunId}）。系统分享只携带内容标题与版本说明——单次访问签名链接不会外流。
        </Text>
      </Sheet>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
});
