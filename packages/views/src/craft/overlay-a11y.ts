import { useEffect } from 'react';

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** Adds the missing modal keyboard contract around TDesign's supported overlay. */
export function useTDesignOverlayA11y(open: boolean, panelSelector: string, accessibleName: string): void {
  useEffect(() => {
    if (!open) return undefined;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    let observer: MutationObserver | undefined;
    const configure = () => {
      const panel = document.querySelector<HTMLElement>(panelSelector);
      if (!panel) return false;
      panel.setAttribute('role', 'dialog');
      panel.setAttribute('aria-modal', 'true');
      panel.setAttribute('aria-label', accessibleName);
      if (!panel.hasAttribute('tabindex')) panel.tabIndex = -1;
      panel.focus();
      return true;
    };
    if (!configure()) {
      observer = new MutationObserver(() => { if (configure()) observer?.disconnect(); });
      observer.observe(document.body, { childList: true, subtree: true });
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return;
      const panel = document.querySelector<HTMLElement>(panelSelector);
      if (!panel) return;
      const controls = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)];
      if (!controls.length) { event.preventDefault(); panel.focus(); return; }
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (event.shiftKey && (document.activeElement === first || !panel.contains(document.activeElement))) {
        event.preventDefault(); last.focus();
      } else if (!event.shiftKey && (document.activeElement === last || !panel.contains(document.activeElement))) {
        event.preventDefault(); first.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      observer?.disconnect();
      document.removeEventListener('keydown', onKeyDown);
      opener?.focus();
    };
  }, [open, panelSelector, accessibleName]);
}
