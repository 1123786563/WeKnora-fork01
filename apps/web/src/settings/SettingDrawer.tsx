import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Button as TButton, Drawer as TDrawer } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { formatMessage } from '@weknora/i18n';
import { readInitialLocale } from './PortedSectionsPanel.tsx';

/* SettingDrawer.vue（frontend/src/components/settings/SettingDrawer.vue）同构端口。
 *
 * Vue 端是 t-drawer 包装（placement right / size px / z-index 2500 / attach
 * body / destroy-on-close / footer 条件渲染），自定义 header（monogram 徽章 +
 * 标题 + 副标题）与 footer（footer-left 槽 + 取消/保存 按钮）。可拖宽：body
 * portal 的 setting-drawer-resize-handle + localStorage 记忆宽度。样式在
 * settings.td.css §SettingDrawer（scoped 块类名平移 + unscoped 块原样）。
 *
 * 库间默认差（tdesign-react 1.18.3 ↔ tdesign-vue-next 1.20.7）：react Drawer
 * closeBtn 默认 true（渲染 X），vue-next 默认 false——这里显式 closeBtn={false}
 * 对齐 Vue 端 DOM（SettingDrawer.vue 不传 close-btn，注释明言关闭靠抽屉滑出
 * 与遮罩点击，不要冗余 X）。
 */

interface SettingDrawerProps {
  visible: boolean;
  title: string;
  /** Vue `description` prop：无 #subtitle 槽时的副标题。 */
  description?: string;
  /** Vue `icon` prop：TDesign 图标名，header 徽章内 t-icon（无 #headerIcon 槽时的默认）。 */
  icon?: string;
  /** Vue #headerIcon 槽内容（monogram 徽章等）。 */
  headerIcon?: ReactNode;
  /** Vue #subtitle 槽内容（覆盖 description）。 */
  subtitle?: ReactNode;
  /** Vue #header-extra 槽：header 行下方的整宽区（skills 步骤条 / mcp steps）。 */
  headerExtra?: ReactNode;
  /** 初始宽度（无记忆偏好时）。任意 CSS 长度字符串。 */
  width?: string;
  /** 左缘可见拖宽手柄。 */
  resizable?: boolean;
  minWidth?: number;
  maxWidth?: number;
  /** 记忆宽度的 localStorage key；'' 按标题派生。 */
  storageKey?: string;
  confirmLoading?: boolean;
  confirmDisabled?: boolean;
  confirmText?: string;
  cancelText?: string;
  hideFooter?: boolean;
  zIndex?: number;
  /** Vue :close-on-overlay-click 透传（默认 true；PlatformAPIKeys.vue 传 false）。 */
  closeOnOverlayClick?: boolean;
  /** Vue `:class` 透传（如 `parser-engine-drawer parser-engine-drawer--builtin`）。 */
  drawerClass?: string;
  /** Vue #footer-left 槽。 */
  footerLeft?: ReactNode;
  onVisibleChange: (visible: boolean) => void;
  onConfirm?: () => void;
  onCancel?: () => void;
  children: ReactNode;
}

function blurActiveElementBeforeClose() {
  // Vue blurActiveElementBeforeClose：destroy-on-close 拆除时 textarea autosize
  // 可能对已卸载节点跑 getComputedStyle 抛未捕获 rejection。
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
}

export function SettingDrawer(props: SettingDrawerProps) {
  const {
    visible, title, description = '', icon = '', headerIcon, subtitle, headerExtra,
    width = '560px', resizable = true, minWidth = 480, maxWidth = 1200, storageKey = '',
    confirmLoading = false, confirmDisabled = false, confirmText = '', cancelText = '',
    hideFooter = false, zIndex = 2500, drawerClass, footerLeft, closeOnOverlayClick = true,
    onVisibleChange, onConfirm, onCancel, children,
  } = props;

  const locale = readInitialLocale();
  const tr = (key: string) => formatMessage(locale, key);

  // ---------- width state（storageKey 由标题派生，调用方可用 storageKey 覆盖） ----------
  const resolvedStorageKey = useMemo(
    () => storageKey || `setting-drawer:width:${title || 'default'}`,
    [storageKey, title],
  );
  const [viewportWidth, setViewportWidth] = useState(() =>
    typeof window === 'undefined' ? maxWidth : window.innerWidth);

  const clampWidth = useCallback((n: number) => {
    const cap = Math.min(maxWidth, viewportWidth);
    const floor = Math.min(minWidth, cap);
    return Math.max(floor, Math.min(cap, Math.round(n)));
  }, [maxWidth, viewportWidth, minWidth]);

  const parseWidthToPx = (value: string) => {
    const n = parseInt(value, 10);
    return Number.isFinite(n) ? n : 560;
  };

  const [userWidthPx, setUserWidthPx] = useState<number | null>(() => {
    if (typeof window === 'undefined') return null;
    try {
      const raw = window.localStorage.getItem(resolvedStorageKey);
      if (!raw) return null;
      const n = Number(raw);
      if (!Number.isFinite(n)) return null;
      const cap = Math.min(maxWidth, window.innerWidth);
      const floor = Math.min(minWidth, cap);
      return Math.max(floor, Math.min(cap, Math.round(n)));
    } catch {
      return null;
    }
  });

  const drawerWidthPx = clampWidth(userWidthPx ?? parseWidthToPx(width));
  const effectiveWidth = `${drawerWidthPx}px`;

  useEffect(() => {
    const onWindowResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener('resize', onWindowResize, { passive: true });
    return () => window.removeEventListener('resize', onWindowResize);
  }, []);

  // ---------- custom drag-resize（与 doc-content 同款可见手柄） ----------
  const [drawerResizing, setDrawerResizing] = useState(false);
  const dragStart = useRef({ x: 0, width: 0 });

  const onResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    setDrawerResizing(true);
    const live = { width: drawerWidthPx };
    dragStart.current = { x: e.clientX, width: drawerWidthPx };
    const onResizeMove = (ev: MouseEvent) => {
      const delta = dragStart.current.x - ev.clientX;
      live.width = clampWidth(dragStart.current.width + delta);
      setUserWidthPx(live.width);
    };
    const onResizeEnd = () => {
      document.removeEventListener('mousemove', onResizeMove);
      document.removeEventListener('mouseup', onResizeEnd);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      setDrawerResizing(false);
      try {
        window.localStorage.setItem(resolvedStorageKey, String(live.width));
      } catch { /* quota/private mode */ }
    };
    document.addEventListener('mousemove', onResizeMove);
    document.addEventListener('mouseup', onResizeEnd);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };

  const handleConfirm = () => onConfirm?.();
  const handleCancel = () => {
    blurActiveElementBeforeClose();
    onCancel?.();
    onVisibleChange(false);
  };
  const handleClose = () => {
    blurActiveElementBeforeClose();
    onVisibleChange(false);
  };

  const headerNode = <div className="setting-drawer__header-block">
    <div className="setting-drawer__header">
      {headerIcon || icon ? <div className="setting-drawer__header-icon">{headerIcon ?? <TIcon name={icon} />}</div> : null}
      <div className="setting-drawer__header-text">
        <div className="setting-drawer__title">{title}</div>
        {subtitle || description ? <div className="setting-drawer__subtitle">{subtitle ?? description}</div> : null}
      </div>
      <div className="setting-drawer__header-actions" />
    </div>
    {headerExtra ? <div className="setting-drawer__header-extra">{headerExtra}</div> : null}
  </div>;

  const footerNode = <div className="setting-drawer__footer">
    <div className="setting-drawer__footer-left">{footerLeft}</div>
    <div className="setting-drawer__footer-right">
      <TButton theme="default" variant="outline" onClick={handleCancel}>{cancelText || tr('common.cancel')}</TButton>
      <TButton theme="primary" loading={confirmLoading} disabled={confirmDisabled} onClick={handleConfirm}>{confirmText || tr('common.save')}</TButton>
    </div>
  </div>;

  const resizeHandle = visible && resizable && typeof document !== 'undefined' ? createPortal(
    <div
      className={'setting-drawer-resize-handle' + (drawerResizing ? ' setting-drawer-resize-handle--active' : '')}
      style={{ right: `${drawerWidthPx}px`, '--setting-drawer-travel': `${drawerWidthPx}px` } as CSSProperties}
      role="separator" aria-orientation="vertical" onMouseDown={onResizeStart}
    >
      <div className="setting-drawer-resize-line" />
    </div>,
    document.body,
  ) : null;

  return <>
    {resizeHandle}
    <TDrawer
      visible={visible}
      header={headerNode}
      footer={hideFooter ? false : footerNode}
      closeBtn={false}
      size={effectiveWidth}
      zIndex={zIndex}
      placement="right"
      destroyOnClose
      closeOnOverlayClick={closeOnOverlayClick}
      className={('setting-drawer' + (drawerClass ? ` ${drawerClass}` : '') + (drawerResizing ? ' setting-drawer--resizing' : ''))}
      onClose={handleClose}
    >
      <div className="setting-drawer__body">{children}</div>
    </TDrawer>
  </>;
}
