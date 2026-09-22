import { useEffect, useRef, useState } from 'react';
import { Popup, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import type { ChatCopyTable } from '@weknora/views/chat/chat-copy';

/*
 * Vue 事实源：frontend/src/components/ChatHeader.vue（DOM/类名 1:1）。
 * - header.chat-header（毛玻璃 chip）> h1.chat-header__title > span.chat-header__title-text
 *   （is_pinned 时前置 t-icon pin 12px）。
 * - ⋯ 菜单走 t-popup（trigger click / destroyOnClose / bottom-left），内容
 *   .chat-header-menu：pin、rename ─ 分隔 ─ copyId/copyLink/copyMarkdown/
 *   openNewWindow（headerUtilityItems 注入）─ 分隔 ─ clear、delete(is-danger)；
 *   清空/删除在同一弹层内切换 .chat-header-confirm 二次确认。
 * - rename：关闭弹层后标题原位切到 form.chat-header__edit > input。
 * 保留的语义钩点：aria-label（chatHeader.moreActions＝ix-chat-header-menu 点击目标）、
 * data-menu-action、wk-chat-header-menu。
 */

export interface ChatHeaderUtilityItem {
  id: string;
  label: string;
  onActivate(): void;
}

export interface ChatHeaderProps {
  copy: ChatCopyTable;
  title: string;
  isPinned?: boolean;
  disabled?: boolean;
  busy?: boolean;
  onTogglePin?(pinned: boolean): void;
  onRenameSession(title: string): Promise<void>;
  onClearSession?(): Promise<void>;
  onDeleteSession?(): Promise<void>;
  headerUtilityItems?: readonly ChatHeaderUtilityItem[];
  /** Vue menu.newSession fallback already resolved by the host. */
  renameTitle: string;
  renameTitleRequired: string;
  renameTitleFailed: string;
  renameCancel: string;
  renameConfirm: string;
  renameSaving: string;
  clearConfirmTitle: string;
  clearConfirmBody: string;
  clearConfirmAction: string;
  deleteConfirmTitle: string;
  deleteConfirmBody: string;
  deleteConfirmAction: string;
  cancelLabel: string;
  operationFailed: string;
}

export function ChatHeader(props: ChatHeaderProps) {
  const { copy } = props;
  const [menuVisible, setMenuVisible] = useState(false);
  const [menuMode, setMenuMode] = useState<'menu' | 'clear' | 'delete'>('menu');
  const [titleEditing, setTitleEditing] = useState(false);
  const [titleDraft, setTitleDraft] = useState('');
  const [renameBusy, setRenameBusy] = useState(false);
  const [renameError, setRenameError] = useState<string | null>(null);
  const [dangerBusy, setDangerBusy] = useState(false);
  const [dangerError, setDangerError] = useState<string | null>(null);
  const titleInputRef = useRef<HTMLInputElement | null>(null);
  const renameSubmittingRef = useRef(false);

  useEffect(() => {
    if (!titleEditing) return;
    const frame = window.requestAnimationFrame(() => {
      titleInputRef.current?.focus();
      titleInputRef.current?.select();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [titleEditing]);

  function onMenuVisibleChange(visible: boolean): void {
    setMenuVisible(visible);
    if (!visible) setMenuMode('menu');
  }

  function startTitleEdit(): void {
    setTitleDraft(props.renameTitle);
    setRenameError(null);
    setMenuVisible(false);
    setMenuMode('menu');
    setTitleEditing(true);
  }

  function cancelTitleEdit(): void {
    setTitleEditing(false);
    setTitleDraft('');
    setRenameError(null);
  }

  async function submitTitleEdit(): Promise<void> {
    if (!titleEditing || renameSubmittingRef.current) return;
    const title = titleDraft.trim().replace(/\s+/g, ' ').slice(0, 80);
    const current = props.renameTitle.trim().replace(/\s+/g, ' ').slice(0, 80);
    if (!title) {
      setRenameError(props.renameTitleRequired);
      titleInputRef.current?.focus();
      return;
    }
    if (title === current) {
      cancelTitleEdit();
      return;
    }
    renameSubmittingRef.current = true;
    setRenameBusy(true);
    setRenameError(null);
    try {
      setTitleEditing(false);
      setTitleDraft('');
      await props.onRenameSession(title);
    } catch {
      setTitleEditing(false);
      setTitleDraft('');
      setRenameError(props.renameTitleFailed);
    } finally {
      renameSubmittingRef.current = false;
      setRenameBusy(false);
    }
  }

  async function submitDangerAction(): Promise<void> {
    if (!menuMode || dangerBusy) return;
    setDangerBusy(true);
    setDangerError(null);
    try {
      if (menuMode === 'clear') await props.onClearSession?.();
      else await props.onDeleteSession?.();
      setMenuVisible(false);
      setMenuMode('menu');
    } catch (error) {
      setDangerError(error instanceof Error ? error.message : props.operationFailed);
    } finally {
      setDangerBusy(false);
    }
  }

  const busyAction = props.busy || renameBusy || dangerBusy;
  const menuDisabled = Boolean(props.disabled || busyAction);
  const utilityItems = props.headerUtilityItems ?? [];

  return (
    <header className={'chat-header' + (titleEditing ? ' is-editing' : '')}>
      {titleEditing ? (
        <form className="chat-header__edit" onClick={(event) => event.stopPropagation()} onSubmit={(event) => { event.preventDefault(); void submitTitleEdit(); }}>
          <input
            ref={titleInputRef}
            className="chat-header__edit-input"
            value={titleDraft}
            maxLength={80}
            disabled={renameBusy}
            placeholder={props.renameTitle}
            onChange={(event) => setTitleDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') { event.preventDefault(); cancelTitleEdit(); }
              if (event.key === 'Enter') { event.preventDefault(); void submitTitleEdit(); }
            }}
            onBlur={() => { void submitTitleEdit(); }}
            aria-label={props.renameTitle}
          />
          {renameError ? <span role="alert" className="chat-header__edit-error">{renameError}</span> : null}
        </form>
      ) : (
        <h1 className="chat-header__title" title={titleDisplay(props.title, copy)} onDoubleClick={() => { if (!menuDisabled) startTitleEdit(); }}>
          {props.isPinned ? <TIcon name="pin" size="12px" className="chat-header__pin" /> : null}
          <span className="chat-header__title-text">{props.title}</span>
        </h1>
      )}
      {!titleEditing ? (
        <Popup
          visible={menuVisible}
          onVisibleChange={(visible, context) => {
            // trigger click 以外（Esc/外点）的关闭同样回到 menu 态
            if (context?.trigger === 'document') { onMenuVisibleChange(visible); return; }
            onMenuVisibleChange(visible);
          }}
          trigger="click"
          destroyOnClose
          placement="bottom-left"
          disabled={menuDisabled}
          overlayClassName={'chat-header-menu-popup' + (menuMode === 'menu' ? '' : ' is-confirm')}
          content={
            <div className="chat-header-menu" onClick={(event) => event.stopPropagation()}>
              {menuMode === 'menu' ? <>
                {props.onTogglePin ? <button type="button" className="chat-header-menu__item" data-menu-action="pin" onClick={() => { setMenuVisible(false); props.onTogglePin!(Boolean(props.isPinned)); }}>
                  <TIcon className="chat-header-menu__icon" name={props.isPinned ? 'pin-filled' : 'pin'} />
                  <span>{props.isPinned ? copy.unpin : copy.pin}</span>
                </button> : null}
                <button type="button" className="chat-header-menu__item" data-menu-action="rename" onClick={startTitleEdit}>
                  <TIcon className="chat-header-menu__icon" name="edit-1" />
                  <span>{copy.renameSession}</span>
                </button>
                {utilityItems.length > 0 ? <>
                  <div className="chat-header-menu__divider" />
                  {utilityItems.map((item) => (
                    <button key={item.id} type="button" className="chat-header-menu__item" data-menu-action={item.id} onClick={() => { setMenuVisible(false); item.onActivate(); }}>
                      <TIcon className="chat-header-menu__icon" name={utilityIcon(item.id)} />
                      <span>{item.label}</span>
                    </button>
                  ))}
                  <div className="chat-header-menu__divider" />
                </> : null}
                {props.onClearSession ? <button type="button" className="chat-header-menu__item" data-menu-action="clear" onClick={() => { setMenuMode('clear'); setDangerError(null); }}>
                  <TIcon className="chat-header-menu__icon" name="clear" />
                  <span>{copy.clearMessages}</span>
                </button> : null}
                {props.onDeleteSession ? <button type="button" className="chat-header-menu__item is-danger" data-menu-action="delete" onClick={() => { setMenuMode('delete'); setDangerError(null); }}>
                  <TIcon className="chat-header-menu__icon" name="delete" />
                  <span>{copy.deleteSession}</span>
                </button> : null}
              </> : (
                <div className="chat-header-confirm">
                  <div className="chat-header-confirm__title">
                    {menuMode === 'clear' ? props.clearConfirmTitle : props.deleteConfirmTitle}
                  </div>
                  <div className="chat-header-confirm__body">
                    {menuMode === 'clear' ? props.clearConfirmBody : props.deleteConfirmBody}
                  </div>
                  {dangerError ? <p role="alert" className="chat-header-confirm__error">{dangerError}</p> : null}
                  <div className="chat-header-confirm__footer">
                    <button type="button" className="chat-header-confirm__btn" disabled={dangerBusy} onClick={() => setMenuMode('menu')}>
                      {props.cancelLabel}
                    </button>
                    <button type="button" className="chat-header-confirm__btn is-danger" disabled={dangerBusy} onClick={() => void submitDangerAction()}>
                      {menuMode === 'clear' ? props.clearConfirmAction : props.deleteConfirmAction}
                    </button>
                  </div>
                </div>
              )}
            </div>
          }
        >
          <button
            type="button"
            className={'chat-header__menu-btn wk-chat-header-menu' + (busyAction ? ' is-loading' : '')}
            disabled={menuDisabled}
            aria-label={copy.moreActions}
            onClick={(event) => event.stopPropagation()}
          >
            {busyAction ? <TIcon name="loading" size="14px" className="chat-header__menu-loading" /> : <TIcon name="ellipsis" size="16px" />}
          </button>
        </Popup>
      ) : null}
    </header>
  );
}

function titleDisplay(title: string, copy: ChatCopyTable): string {
  return title || copy.newSession;
}

function utilityIcon(id: string): string {
  switch (id) {
    case 'copyId': return 'copy';
    case 'copyLink': return 'link';
    case 'copyMarkdown': return 'file-copy';
    case 'openNewWindow': return 'browse';
    default: return 'copy';
  }
}

/*
 * Vue index.vue 沙箱面板收起态入口（.sandbox-header-toggle，镜像侧栏 toggle 图标）。
 * t-tooltip placement bottom + 纯图标按钮。
 */
export function SandboxHeaderToggle(props: { copy: ChatCopyTable; label: string; onOpen(): void }) {
  return (
    <div className="sandbox-header-toggle">
      <Tooltip placement="bottom" content={props.label}>
        <button type="button" className="sandbox-header-toggle__btn" aria-label={props.label} onClick={props.onOpen}>
          <svg viewBox="0 0 20 20" width="18" height="18" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <rect x="1.5" y="1.5" width="17" height="17" rx="3" stroke="currentColor" strokeWidth="1.2" />
            <line x1="12.5" y1="1.5" x2="12.5" y2="18.5" stroke="currentColor" strokeWidth="1.2" />
            <line x1="16" y1="7.5" x2="16" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
          </svg>
        </button>
      </Tooltip>
    </div>
  );
}
