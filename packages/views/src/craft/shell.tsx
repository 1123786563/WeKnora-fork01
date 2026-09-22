// CFT-S00-T004: the craft host shell — composed from the craft-local
// tdesign-isomorphic primitives (td.tsx). The td Drawer owns focus trapping,
// Escape and focus restore (t-drawer DOM + the Sheet interaction contract);
// the td Button owns the disabled semantics; nothing here rebuilds a second
// focus or dialog system. This layer only fixes craft's composition defaults:
// the .wk-craft scope, the overflow guard, the 460px drawer, and notices that
// speak state as text (never color-only).
import React, { type ReactNode } from 'react';
import { Button, Drawer } from './td.tsx';

/**
 * Page shell: carries the craft token scope (T003) and clips unexpected
 * horizontal overflow. Real 1440/390 viewport proof runs in the e2e lane
 * (T024/T034); this class is the contract the CSS pins.
 */
export function CraftShell({ children }: { children: ReactNode }) {
  return (
    <div className="wk-craft wk-craft-shell">
      {children}
    </div>
  );
}

export interface CraftPanelAction {
  label: string;
  onAction: () => void;
  /** Disabled actions keep their reason readable — never a dead button. */
  disabled?: boolean;
  disabledReason?: string;
}

/** Panel header: title + secondary line + right-aligned action cluster. */
export function CraftPanelHeader({ title, subtitle, actions = [] }: {
  title: string;
  subtitle?: string;
  actions?: readonly CraftPanelAction[];
}) {
  return (
    <header className="wk-craft-panel-header">
      <div>
        <h3 className="wk-craft-panel-title">{title}</h3>
        {subtitle ? <p className="wk-craft-muted">{subtitle}</p> : null}
      </div>
      {actions.length > 0 ? (
        <div className="wk-craft-actions">
          {actions.map((action) => (
            <Button
              key={action.label}
              size="small"
              disabled={action.disabled}
              title={action.disabled && action.disabledReason ? action.disabledReason : undefined}
              onClick={action.onAction}
            >
              {action.label}
            </Button>
          ))}
          {actions.some((a) => a.disabled && a.disabledReason) ? (
            <span className="wk-craft-muted wk-craft-panel-reason">
              {actions.find((a) => a.disabled && a.disabledReason)?.disabledReason}
            </span>
          ) : null}
        </div>
      ) : null}
    </header>
  );
}

/** Every notice kind speaks: state is never communicated by color alone. */
export const CRAFT_NOTICE_KINDS = [
  'reconnect',
  'failed',
  'canceled',
  'readonly',
  'expired',
  'unknown',
] as const;
export type CraftNoticeKind = (typeof CRAFT_NOTICE_KINDS)[number];

/** An aria-live status line scoped to craft semantics (hifi design §6). */
export function CraftNotice({ kind, children }: { kind: CraftNoticeKind; children: ReactNode }) {
  return (
    <p className="wk-craft-notice" data-kind={kind} aria-live="polite">
      {children}
    </p>
  );
}

/**
 * The craft drawer: the hifi design's 460px side panel (full width under
 * 760px via CSS). Delegates focus/Escape entirely to the tdesign-isomorphic
 * Drawer (td.tsx).
 */
export function CraftDrawer({ open, title, onClose, children }: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer">
      {children}
    </Drawer>
  );
}
