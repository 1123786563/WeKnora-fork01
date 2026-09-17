import React, { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useWeknoraTheme } from '../ui/theme.ts';
import { PRODUCT_TABS, type ProductTabId } from './routes.ts';
import { StateView } from '../ui/StateView.tsx';

/**
 * 产品四 Tab 壳（MX-009，关闭 G07 容器面）。
 * - 只承载 工作台/会话/资源/我的；Tab 内容按 screen 插槽注入（页面任务逐个接线）。
 * - 渲染零 Happy 依赖：不 import Happy session/sync/auth——产品链独立展示（G07）；
 *   挂载即做任何网络请求都算违规（probe 以 fetch 计数看守）。
 * - 内容插槽 per-tab：后续任务（MX-013/014/022/030）注入真实 screen；
 *   未接线 Tab 显示诚实的建设中空态（不假装完成）。
 */
export interface ProductShellProps {
  /** 每 Tab 的内容插槽（未提供的 Tab 走建设中空态） */
  tabs?: Partial<Record<ProductTabId, React.ReactNode>>;
  initialTab?: ProductTabId;
  /** 当前 space 代（详情返回栈隔离由路由层按 tab 分组保证，MX-011 接 generation 清栈） */
  testID?: string;
}

export function ProductShell({ tabs, initialTab = 'workbench', testID }: ProductShellProps) {
  const { theme } = useWeknoraTheme();
  const [active, setActive] = useState<ProductTabId>(initialTab);
  const activeIndex = useMemo(() => PRODUCT_TABS.findIndex((tab) => tab.id === active), [active]);
  const go = (index: number) => {
    const next = PRODUCT_TABS[index];
    if (next) setActive(next.id);
  };
  const content = tabs?.[active];
  return (
    <View testID={testID} style={[styles.container, { backgroundColor: theme.colors.bg }]}>
      <View style={styles.content} accessible accessibilityLabel="主要内容区">
        {content ?? (
          <StateView
            kind="empty"
            message={`${PRODUCT_TABS.find((tab) => tab.id === active)?.label ?? ''}页面建设中`}
            detail="该 Tab 的产品页面由后续任务接入（routes.ts 已登记其 screen id 与导航路径）"
          />
        )}
      </View>
      <View
        accessibilityRole="tablist"
        style={[styles.tabbar, { backgroundColor: theme.colors.surface, borderTopColor: theme.colors.line, height: theme.size['tab-height'] }]}
      >
        {PRODUCT_TABS.map((tab, index) => {
          const selected = tab.id === active;
          return (
            <Pressable
              key={tab.id}
              accessibilityRole="tab"
              accessibilityLabel={tab.label}
              accessibilityState={{ selected }}
              onPress={() => go(index)}
              style={styles.tabItem}
            >
              <Text style={{ color: selected ? theme.colors.brand : theme.colors.muted, fontSize: theme.typography.caption.fontSize, fontWeight: selected ? '600' : '400' }}>
                {tab.label}
              </Text>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  content: { flex: 1 },
  tabbar: { flexDirection: 'row', borderTopWidth: StyleSheet.hairlineWidth },
  tabItem: { flex: 1, alignItems: 'center', justifyContent: 'center', minHeight: 48 },
});
