export const tokens = {
  color: { brand: '#07c05f', brandHover: '#08dd6e', brandActive: '#06b04d', brandLight: '#e9f8ec', text: '#000000e6', textSecondary: '#00000099', textPlaceholder: '#00000066', textDisabled: '#00000042', surface: '#ffffff', page: '#eeeeee', surfaceHover: '#f3f3f3', surfaceActive: '#e7e7e7', border: '#dcdcdc', component: '#e7e7e7', error: '#e34d59', errorLight: '#fdecee', success: '#00a870', warning: '#ed7b2f' },
  font: { family: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Microsoft YaHei", sans-serif', mono: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace', size: { body: '14px', small: '12px', title: '16px' }, lineHeight: { body: '20px', small: '20px', title: '24px' } },
  radius: { small: '2px', control: '6px', popup: '10px', panel: '12px', round: '999px' },
  shadow: { popup: '0 0 0 0.5px rgba(0,0,0,.03), 0 2px 4px rgba(0,0,0,.04), 0 8px 24px rgba(0,0,0,.1)', panel: '0 6px 30px rgba(0,0,0,.12)' },
  overlay: { menuMinWidth: '148px', menuPadding: '4px', dialogZIndex: 3000, anchoredZIndex: 3500, duration: '180ms', easing: 'cubic-bezier(0.2, 0, 0, 1)' }
} as const;
export const tokenCss = `:root { --wk-color-brand: ${tokens.color.brand}; --wk-color-brand-hover: ${tokens.color.brandHover}; --wk-overlay-menu-min-width: ${tokens.overlay.menuMinWidth}; --wk-overlay-anchored-z: ${tokens.overlay.anchoredZIndex}; }`;
