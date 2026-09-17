import React, { useCallback, useEffect, useRef } from 'react';
import { AccessibilityInfo, BackHandler, KeyboardAvoidingView, Modal, Platform, Pressable, ScrollView, StyleSheet, Text, View, findNodeHandle } from 'react-native';
import { useWeknoraTheme } from './theme.ts';

/**
 * 底部确认 Sheet（MX-008）。
 * 关键合同：
 * - 关闭（返回键/遮罩）不等于决定——onClose 只收起，onDecision 只由显式决定按钮触发；
 *   决定按钮由调用方经 footer 传入（本组件不发明业务决定）。
 * - 关闭后读屏焦点回到触发元素（triggerFocusRef，AccessibilityInfo.setAccessibilityFocus）。
 * - Android 返回键先于系统默认关闭 Sheet（BackHandler）。
 * - 大字体：内容区 ScrollView + maxHeight 比例，不固定高度裁切。
 * - 遮罩是唯一关闭点击区；内容区不整体可点（禁止嵌套点击区域）。
 */
export interface SheetProps {
  visible: boolean;
  title: string;
  /** 触发元素 ref：关闭后焦点回归目标（读屏） */
  triggerFocusRef?: React.RefObject<unknown>;
  onClose: () => void;
  children: React.ReactNode;
  /** 显式决定区（调用方给 Button 等；与本组件的关闭语义分离） */
  footer?: React.ReactNode;
  testID?: string;
}

export function Sheet({ visible, title, triggerFocusRef, onClose, children, footer, testID }: SheetProps) {
  const { theme } = useWeknoraTheme();
  const titleRef = useRef<Text>(null);

  const focusTrigger = useCallback(() => {
    if (!triggerFocusRef?.current) return;
    const handle = findNodeHandle(triggerFocusRef.current as React.Component);
    if (handle !== null && handle !== undefined) {
      AccessibilityInfo.setAccessibilityFocus(handle);
    }
  }, [triggerFocusRef]);

  // 打开时把读屏焦点放到标题；关闭时还给触发元素
  useEffect(() => {
    if (!visible) return;
    const handle = findNodeHandle(titleRef.current);
    if (handle !== null && handle !== undefined) {
      AccessibilityInfo.setAccessibilityFocus(handle);
    }
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      // 返回键与遮罩同语义：先归还焦点再收起（不得绕过 focusTrigger）
      focusTrigger();
      onClose();
      return true;
    });
    return () => sub.remove();
  }, [visible, onClose]);

  const handleClose = useCallback(() => {
    focusTrigger();
    onClose();
  }, [focusTrigger, onClose]);

  if (!visible) return null;
  return (
    <Modal visible transparent animationType="slide" onRequestClose={handleClose} accessibilityViewIsModal testID={testID}>
      <Pressable style={[styles.scrim, { backgroundColor: theme.colors.scrim }]} onPress={handleClose} accessibilityLabel="关闭" accessibilityRole="button">
        {/* 遮罩点击关闭；内容区拦截事件，不嵌套点击区 */}
      </Pressable>
      <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} style={[styles.sheet, { backgroundColor: theme.colors.surface, borderTopLeftRadius: theme.radius.sheet, borderTopRightRadius: theme.radius.sheet, paddingBottom: theme.spacing[20] }]}>
        <View style={[styles.grabber, { backgroundColor: theme.colors['control-line'], borderRadius: theme.spacing[2], marginTop: theme.spacing[8] }]} />
        <Text ref={titleRef} accessibilityRole="header" style={{ color: theme.colors.ink, fontSize: theme.typography.subtitle.fontSize, lineHeight: theme.typography.subtitle.lineHeight, fontWeight: '600', paddingHorizontal: theme.spacing[20], paddingTop: theme.spacing[12] }}>
          {title}
        </Text>
        <ScrollView style={{ maxHeight: '60%' }} contentContainerStyle={{ padding: theme.spacing[20], gap: theme.spacing[12] }}>
          {children}
        </ScrollView>
        {footer ? <View style={{ paddingHorizontal: theme.spacing[20], paddingTop: theme.spacing[8], gap: theme.spacing[12] }}>{footer}</View> : null}
      </KeyboardAvoidingView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  scrim: { flex: 1 },
  sheet: { maxHeight: '85%', width: '100%' },
  grabber: { alignSelf: 'center', width: 36, height: 4 },
});
