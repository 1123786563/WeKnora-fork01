# Accessibility & Performance · MX-035

## 静态层（已验证，可离线复跑）

- **主操作可达性（M01–M18）**：probe 源级观测各页主操作均带非空 `accessibilityLabel`（读屏可寻址）——`unreachablePrimaryActions=[]`（frozen）。
- **关键 a11y 合同**：全部产品 UI 目录裸 `Pressable` 必带 `accessibilityRole` + 可访问名（tablist/tab/selected 态、button/busy/disabled 态在 Button 组件层统一保证）——`criticalA11yFindings=[]`（frozen）。修复项：VoiceInputScreen 草稿 Pressable 补 role=text。
- **组件级合同**（MX-008 已钉）：44/48 触控区、minHeight（非固定高，大字体不裁切）、危险操作双通道（色+文字+图标位）、busy 不重复触发、状态徽章文字+颜色双通道、Sheet 焦点回归/键盘避让/关闭≠决定。
- **动态字体**：全组件 allowFontScaling 默认开启（未关闭）；文案级 overflow 由 minHeight+flex 承接（无固定高度裁切路径）。
- **减少动态效果**：motion 令牌含 reduced=0（组件未引入动画库——静态页无动画；Sheet/列表动画随 MX-034 设备执行时逐项核验 reduce motion）。

## 设备层（blocked-env，如实）

| 项 | 状态 |
|---|---|
| 18 页×light/dark×320/360/390/430/平板×100%/200% 字体原生截图矩阵 | **pending**（tests/mobile-v2/visual-baseline.json——设备 blocked-env，不伪造 hash；档位与配方已登记） |
| 读屏实测（VoiceOver/TalkBack 逐页朗读） | pending（随 MX-034 设备链） |
| 键盘/返回/焦点/横屏/平板布局实测 | pending（同上） |
| 性能：1000 条消息渲染/长文本输入/滚动耗时 | pending（同上；visual-baseline.json performance 段） |

差异修正原则（冻结）：以明确差异清单修正实现，不以缩小字体消除溢出；不覆盖基准截图掩盖偏差。

## 复跑命令

- `pnpm exec tsx --test tests/mobile-v2/mx-035.test.ts`（静态层 frozen：两组空数组）
