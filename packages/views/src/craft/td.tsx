// S5 组件层换 tdesign（craft 域）。packages/views 刻意不带 tdesign-react 与
// react-dom 依赖（package.json 由并行流共享；chat/message-face.tsx 的 t-button
// 同构、composer/agent-selector 的树内渲染同口径先例）：本模块以 tdesign 同构
// 手写 DOM 输出——类名/结构/hook 与 tdesign-react 1.18.3 的 Button/Drawer/
// Dialog 渲染一致（Button.js / drawer/Drawer.js / dialog/DialogCard.js 逐行核对），
// 宿主 app 已全局加载 tdesign.css（apps/web styles.css:2），t-* 类即获得真实
// tdesign 视觉。弹层为 fixed 定位树内渲染（fixed 的 containing block 是视口，
// 不受 shell overflow-x: clip 裁剪），焦点/Escape/焦点归还契约与原 Sheet 实现
// （packages/ui interaction.test.tsx 口径）等价。
//
// 模拟层边界（S5 评审 Important/Minor 回收）：本文件是 craft 域唯一的
// tdesign 手写模拟层，禁止扩散——新增弹层/按钮一律优先引入真实
// tdesign-react 组件（apps/web 有依赖）或在各自的 tdesign 同构判例下评审；
// 当 packages/views 获得 tdesign-react 依赖时应删除本文件并整体切换到真实
// 组件（参照 views-chat message-face 判例的台账口径）。
//
// 差异注记（与真实 tdesign 组件）：
// - Button 保留原生 <button disabled>（台账 #7 的 disabled-渲染-div 是库内部
//   行为，手写层维持原生语义，视觉经 .t-is-disabled 类等价）；
// - Drawer/Dialog 的进入动画类（CSSTransition）与 body portal 未复刻——
//   fixed 树内渲染即可覆盖 craft 用例（无祖先 transform 上下文）；
// - 关闭钮 span/div 附加 role="button" + aria-label（真实库无——手写层维持
//   原 Sheet 的键盘/读屏契约）；CloseIcon 为 tdesign-icons-react 0.6.11
//   close.js 的 SVG 逐属性复刻（stroke=currentColor strokeWidth=2）。
import React, { useEffect, useRef, type ButtonHTMLAttributes, type ReactNode } from 'react';

/** tdesign-icons-react CloseIcon（icons 0.6.11 close.js + icon.js IconBase）
 *  的逐属性复刻：tdesign.css 经 .t-icon 与 1em 尺寸还原视觉。 */
function CloseIcon({ className }: { className?: string }) {
  return (
    <svg
      className={['t-icon', 't-icon-close', className].filter(Boolean).join(' ')}
      fill="none"
      viewBox="0 0 24 24"
      width="1em"
      height="1em"
      style={{ fill: 'none' }}
    >
      <g id="close">
        <path
          id="stroke1"
          stroke="currentColor"
          strokeLinecap="square"
          strokeWidth="2"
          d="M16.9503 7.05029L12.0005 12M12.0005 12L7.05078 16.9498M12.0005 12L16.9503 16.9498M12.0005 12L7.05078 7.05029"
        />
      </g>
    </svg>
  );
}

export interface CraftButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'type'> {
  type?: 'button' | 'submit' | 'reset';
  size?: 'small' | 'medium' | 'large';
  /** tdesign 双轴：theme × variant（craft 旧默认=白底细描边 → default/outline）。 */
  theme?: 'default' | 'primary' | 'danger' | 'warning' | 'success';
  variant?: 'base' | 'outline' | 'dashed' | 'text';
}

/** tdesign Button 同构输出（类族 t-button--theme-x / --variant-x / t-size-x）。
 *  S5 评审 Minor 回收：无调用方使用 loading——死分支已删（真实库的 loading
 *  渲染的是 Loading 图标组件，空 span 同构本就不成立）。 */
export function Button({
  type = 'button',
  size,
  disabled,
  theme = 'default',
  variant = 'outline',
  className,
  children,
  ...rest
}: CraftButtonProps) {
  const classes = [
    't-button',
    `t-button--theme-${theme}`,
    `t-button--variant-${variant}`,
    size === 'small' ? 't-size-s' : '',
    size === 'large' ? 't-size-l' : '',
    disabled ? 't-is-disabled' : '',
    className,
  ].filter(Boolean).join(' ');
  return (
    <button type={type} className={classes} disabled={disabled} {...rest}>
      {children !== null && children !== undefined && children !== false
        ? <span className="t-button__text">{children}</span>
        : null}
    </button>
  );
}

/** 弹层共用：打开时聚焦面板、焦点域内 Escape 关闭、卸载归还触发元素焦点。 */
function useOverlayFocus(open: boolean, onClose: () => void) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (!open) return undefined;
    restoreRef.current = typeof document !== 'undefined' && document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    panelRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      const active = typeof document !== 'undefined' ? document.activeElement : null;
      if (active && panelRef.current && !panelRef.current.contains(active)) return;
      event.preventDefault();
      event.stopPropagation();
      onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      restoreRef.current?.focus();
      restoreRef.current = null;
    };
  }, [onClose, open]);
  return panelRef;
}

function useBodyPortal(open: boolean): boolean {
  // 树内渲染：仅在浏览器（非 SSR 静态标记）且 open 时挂载。
  return open && typeof document !== 'undefined';
}

export interface CraftDrawerProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  /** t-design size：CSS 宽度（右/左侧滑）——craft 抽屉默认 460px。 */
  size?: string;
  placement?: 'left' | 'right';
  className?: string;
  closeLabel?: string;
}

/** tdesign Drawer 同构输出（drawer/Drawer.js DOM 树；footer 恒缺省——
 *  craft 抽屉动作在 body 内，不渲染默认确认/取消栏）。 */
export function Drawer({
  open,
  title,
  children,
  onClose,
  size = '460px',
  placement = 'right',
  className,
  closeLabel = 'Close',
}: CraftDrawerProps) {
  const panelRef = useOverlayFocus(open, onClose);
  const portal = useBodyPortal(open);
  if (!portal) return null;
  const titleText = typeof title === 'string' ? title : undefined;
  return (
    <div
      ref={panelRef}
      role="dialog"
      aria-modal="true"
      aria-label={titleText}
      tabIndex={-1}
      className={['t-drawer', `t-drawer--${placement}`, 't-drawer--open', className].filter(Boolean).join(' ')}
    >
      <div className="t-drawer__mask" onClick={onClose} />
      <div
        className={['t-drawer__content-wrapper', `t-drawer__content-wrapper--${placement}`].join(' ')}
        style={{ width: size }}
      >
        <div className="t-drawer__close-btn" role="button" aria-label={closeLabel} onClick={onClose}><CloseIcon /></div>
        <div className="t-drawer__header">{title}</div>
        <div className="t-drawer__body">{children}</div>
      </div>
    </div>
  );
}

export interface CraftDialogProps {
  open: boolean;
  title: ReactNode;
  children: ReactNode;
  onClose: () => void;
  className?: string;
  closeLabel?: string;
}

/** tdesign Dialog 同构输出（dialog/DialogCard.js DOM 树：卡片根带
 *  t-dialog--default（内边距载体）；header 为 __header-content 包裹文本 +
 *  __close span（CloseIcon），无默认 footer——craft 弹窗动作在 body 内）。 */
export function Dialog({ open, title, children, onClose, className, closeLabel = 'Close' }: CraftDialogProps) {
  const panelRef = useOverlayFocus(open, onClose);
  const portal = useBodyPortal(open);
  if (!portal) return null;
  return (
    <div className="t-dialog__ctx t-dialog__modal t-dialog__ctx--fixed" tabIndex={0}>
      <div className="t-dialog__mask" onClick={onClose} />
      <div className="t-dialog__wrap">
        <div className="t-dialog__position t-dialog--center" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
          <div
            ref={panelRef}
            role="dialog"
            aria-modal="true"
            aria-label={typeof title === 'string' ? title : undefined}
            tabIndex={-1}
            className={['t-dialog', 't-dialog--default', className].filter(Boolean).join(' ')}
          >
            <div className="t-dialog__header">
              <div className="t-dialog__header-content">{title}</div>
              <span className="t-dialog__close" style={{ marginLeft: 'auto' }} role="button" aria-label={closeLabel} onClick={onClose}>
                <CloseIcon />
              </span>
            </div>
            <div className="t-dialog__body">{children}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
