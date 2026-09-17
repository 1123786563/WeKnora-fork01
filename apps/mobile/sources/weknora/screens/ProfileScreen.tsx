import React, { useEffect, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Switch, Text, View } from 'react-native';
import { useWeknoraTheme, resolveThemeMode } from '../ui/theme.ts';
import { Card } from '../ui/Card.tsx';
import { Button } from '../ui/Button.tsx';
import { StateView } from '../ui/StateView.tsx';
import type { AccountPreferences, ThemePreference } from '../preferences/store.ts';

/**
 * 我的页 M17（MX-030）：主题三态（system/light/dark）、通知偏好（与系统权限分离）、
 * 退出（撤销设备/关流/清凭据，经宿主接真实动作）。偏好不含 Provider secrets。
 */
export interface ProfileScreenProps {
  account: { name: string; email?: string };
  preferences: AccountPreferences;
  onChangePreferences?: (next: AccountPreferences) => void;
  onLogout?: () => void;
  onOpenUsage?: () => void;
  onOpenSpaces?: () => void;
  testID?: string;
}

const THEME_OPTIONS: ReadonlyArray<{ id: ThemePreference; label: string }> = [
  { id: 'system', label: '跟随系统' },
  { id: 'light', label: '浅色' },
  { id: 'dark', label: '深色' },
];

export function ProfileScreen({ account, preferences, onChangePreferences, onLogout, onOpenUsage, onOpenSpaces, testID }: ProfileScreenProps) {
  const { theme } = useWeknoraTheme();
  const resolved = resolveThemeMode(preferences.theme === 'system' ? undefined : preferences.theme);
  return (
    <ScrollView testID={testID} style={{ backgroundColor: theme.colors.bg }} contentContainerStyle={{ padding: theme.spacing[16], gap: theme.spacing[16] }}>
      <Card tone="hero">
        <Text accessibilityLabel={`当前账户 ${account.name}${account.email ? `，${account.email}` : ''}`} style={{ color: theme.colors['hero-ink'], fontSize: theme.typography.subtitle.fontSize, lineHeight: theme.typography.subtitle.lineHeight, fontWeight: '600' }}>
          {account.name}
        </Text>
        <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
          {account.email ?? ''}
        </Text>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>外观</Text>
        <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
          当前生效：{resolved === 'dark' ? '深色' : '浅色'}（跟随系统时自动切换）
        </Text>
        <View accessibilityRole="tablist" style={[styles.row, { marginTop: theme.spacing[12], gap: theme.spacing[8] }]}>
          {THEME_OPTIONS.map((option) => {
            const selected = preferences.theme === option.id;
            return (
              <Pressable
                key={option.id}
                accessibilityRole="tab"
                accessibilityLabel={`外观${option.label}`}
                accessibilityState={{ selected }}
                onPress={() => onChangePreferences?.({ ...preferences, theme: option.id })}
                style={[styles.chip, { borderColor: selected ? theme.colors.brand : theme.colors['control-line'], backgroundColor: selected ? theme.colors['brand-soft'] : theme.colors.surface, borderRadius: theme.radius.pill, paddingHorizontal: theme.spacing[12], paddingVertical: theme.spacing[6] }]}
              >
                <Text style={{ color: selected ? theme.colors.brand : theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight }}>{option.label}</Text>
              </Pressable>
            );
          })}
        </View>
      </Card>

      <Card>
        <Text style={{ color: theme.colors.ink, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, fontWeight: '600' }}>通知偏好</Text>
        <Text style={{ color: theme.colors.muted, fontSize: theme.typography.caption.fontSize, lineHeight: theme.typography.caption.lineHeight, marginTop: theme.spacing[4] }}>
          偏好只控制提示；关闭不影响任务执行，系统通知权限在系统设置中管理。
        </Text>
        <View style={{ marginTop: theme.spacing[12] }}>
          {([
            ['approvals', '审批提醒'],
            ['productUpdates', '产品更新'],
            ['usageReports', '用量报告'],
          ] as const).map(([field, label]) => (
            <View key={field} style={[styles.row, { paddingVertical: theme.spacing[8] }]}>
              <Text style={{ color: theme.colors.ink, flex: 1, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>{label}</Text>
              <Switch
                accessibilityLabel={`${label}通知偏好`}
                value={preferences.notificationPreferences[field]}
                onValueChange={(value) => onChangePreferences?.({ ...preferences, notificationPreferences: { ...preferences.notificationPreferences, [field]: value } })}
              />
            </View>
          ))}
        </View>
      </Card>

      <View style={{ gap: theme.spacing[12] }}>
        {onOpenSpaces ? <Button label="切换空间" variant="secondary" onPress={onOpenSpaces} accessibilityLabel="切换到其他空间" /> : null}
        {onOpenUsage ? <Button label="空间用量" variant="secondary" onPress={onOpenUsage} accessibilityLabel="查看当前空间用量" /> : null}
        {onLogout ? (
          <Button
            label="退出登录"
            variant="danger"
            accessibilityLabel="退出登录：撤销本设备注册、关闭活动流与语音、清除本账户在本机的数据"
            onPress={onLogout}
          />
        ) : (
          <StateView kind="empty" message="未登录" />
        )}
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center' },
  chip: { borderWidth: StyleSheet.hairlineWidth },
});

