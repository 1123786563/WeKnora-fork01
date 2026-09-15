export const tokens = {
  color: { brand: '#07c05f', brandHover: '#08dd6e', brandActive: '#06b04d', brandLight: '#e9f8ec', text: '#000000e6', textSecondary: '#00000099', textPlaceholder: '#00000066', textDisabled: '#00000042', surface: '#ffffff', page: '#eeeeee', surfaceHover: '#f3f3f3', surfaceActive: '#e7e7e7', border: '#dcdcdc', component: '#e7e7e7', error: '#e34d59', errorLight: '#fdecee', success: '#00a870', warning: '#ed7b2f' },
  font: { family: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Microsoft YaHei", sans-serif', mono: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace', size: { body: '14px', small: '12px', title: '16px' }, lineHeight: { body: '20px', small: '20px', title: '24px' } },
  radius: { small: '2px', control: '6px', popup: '10px', panel: '12px', round: '999px' },
  shadow: { popup: '0 0 0 0.5px rgba(0,0,0,.03), 0 2px 4px rgba(0,0,0,.04), 0 8px 24px rgba(0,0,0,.1)', panel: '0 6px 30px rgba(0,0,0,.12)' },
  overlay: { menuMinWidth: '148px', menuPadding: '4px', dialogZIndex: 3000, anchoredZIndex: 3500, duration: '180ms', easing: 'cubic-bezier(0.2, 0, 0, 1)' }
} as const;
export const tokenCss = `:root {
  --wk-color-brand: ${tokens.color.brand};
  --wk-color-brand-hover: ${tokens.color.brandHover};
  --wk-color-brand-active: ${tokens.color.brandActive};
  --wk-color-brand-light: ${tokens.color.brandLight};
  --wk-color-text: ${tokens.color.text};
  --wk-color-text-secondary: ${tokens.color.textSecondary};
  --wk-color-text-placeholder: ${tokens.color.textPlaceholder};
  --wk-color-text-disabled: ${tokens.color.textDisabled};
  --wk-color-surface: ${tokens.color.surface};
  --wk-color-page: ${tokens.color.page};
  --wk-color-surface-hover: ${tokens.color.surfaceHover};
  --wk-color-surface-active: ${tokens.color.surfaceActive};
  --wk-color-border: ${tokens.color.border};
  --wk-color-component: ${tokens.color.component};
  --wk-color-error: ${tokens.color.error};
  --wk-color-error-light: ${tokens.color.errorLight};
  --wk-color-success: ${tokens.color.success};
  --wk-color-warning: ${tokens.color.warning};
  --wk-font-family: ${tokens.font.family};
  --wk-font-family-mono: ${tokens.font.mono};
  --wk-font-size-body: ${tokens.font.size.body};
  --wk-font-size-small: ${tokens.font.size.small};
  --wk-font-size-title: ${tokens.font.size.title};
  --wk-line-height-body: ${tokens.font.lineHeight.body};
  --wk-line-height-small: ${tokens.font.lineHeight.small};
  --wk-line-height-title: ${tokens.font.lineHeight.title};
  --wk-radius-small: ${tokens.radius.small};
  --wk-radius-control: ${tokens.radius.control};
  --wk-radius-popup: ${tokens.radius.popup};
  --wk-radius-panel: ${tokens.radius.panel};
  --wk-radius-round: ${tokens.radius.round};
  --wk-shadow-popup: ${tokens.shadow.popup};
  --wk-shadow-panel: ${tokens.shadow.panel};
  --wk-overlay-menu-min-width: ${tokens.overlay.menuMinWidth};
  --wk-overlay-dialog-z: ${tokens.overlay.dialogZIndex};
  --wk-overlay-anchored-z: ${tokens.overlay.anchoredZIndex};
  --wk-motion-duration: ${tokens.overlay.duration};
  --wk-motion-easing: ${tokens.overlay.easing};
}`;
