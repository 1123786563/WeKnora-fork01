const fontFamily = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif';
const fontFamilyMono = 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace';

const lightColor = {
  brand: '#07c05f', brandHover: '#08dd6e', brandActive: '#06b04d', brandLight: '#e9f8ec',
  text: '#000000e6', textSecondary: '#00000099', textPlaceholder: '#00000066', textDisabled: '#00000042',
  surface: '#ffffff', panel: '#ffffff', settingsPanel: '#f9f9f9', page: '#eeeeee', surfaceHover: '#f3f3f3', surfaceActive: '#e7e7e7',
  border: '#dcdcdc', component: '#e7e7e7', error: '#e34d59', errorLight: '#fdecee', success: '#00a870', warning: '#ed7b2f',
} as const;

const darkColor = {
  brand: '#06b04d', brandHover: '#049b38', brandActive: '#038626', brandLight: '#06b04d20',
  text: 'rgba(255, 255, 255, 0.9)', textSecondary: 'rgba(255, 255, 255, 0.55)', textPlaceholder: 'rgba(255, 255, 255, 0.35)', textDisabled: 'rgba(255, 255, 255, 0.22)',
  surface: '#242424', panel: '#181818', settingsPanel: '#181818', page: '#181818', surfaceHover: '#2c2c2c', surfaceActive: '#4b4b4b',
  border: '#5e5e5e', component: '#383838', error: '#c64751', errorLight: '#472324', success: '#059465', warning: '#cf6e2d',
} as const;

export const tokens = {
  color: { ...lightColor, dark: darkColor },
  font: { family: fontFamily, mono: fontFamilyMono, size: { body: '14px', small: '12px', title: '16px' }, lineHeight: { body: '20px', small: '20px', title: '24px' } },
  radius: { small: '2px', control: '6px', popup: '10px', panel: '12px', round: '999px' },
  shadow: { popup: '0 0 0 0.5px rgba(0,0,0,.03), 0 2px 4px rgba(0,0,0,.04), 0 8px 24px rgba(0,0,0,.1)', panel: '0 6px 30px rgba(0,0,0,.12)' },
  overlay: { menuMinWidth: '148px', menuPadding: '4px', settingsZIndex: 1100, dialogZIndex: 3000, anchoredZIndex: 3500, duration: '180ms', easing: 'cubic-bezier(0.2, 0, 0, 1)' },
} as const;

function colorVars(color: typeof lightColor): string {
  return `
  --wk-color-brand: ${color.brand}; --wk-color-brand-hover: ${color.brandHover}; --wk-color-brand-active: ${color.brandActive}; --wk-color-brand-light: ${color.brandLight};
  --wk-color-text: ${color.text}; --wk-color-text-secondary: ${color.textSecondary}; --wk-color-text-placeholder: ${color.textPlaceholder}; --wk-color-text-disabled: ${color.textDisabled};
  --wk-color-surface: ${color.surface}; --wk-color-panel: ${color.panel}; --wk-color-settings-panel: ${color.settingsPanel}; --wk-color-page: ${color.page}; --wk-color-surface-hover: ${color.surfaceHover}; --wk-color-surface-active: ${color.surfaceActive};
  --wk-color-border: ${color.border}; --wk-color-component: ${color.component}; --wk-color-error: ${color.error}; --wk-color-error-light: ${color.errorLight}; --wk-color-success: ${color.success}; --wk-color-warning: ${color.warning};
  --background: var(--wk-color-page); --foreground: var(--wk-color-text); --card: var(--wk-color-surface); --card-foreground: var(--wk-color-text);
  --popover: var(--wk-color-surface); --popover-foreground: var(--wk-color-text); --primary: var(--wk-color-brand); --primary-foreground: #ffffff;
  --secondary: var(--wk-color-surface-hover); --secondary-foreground: var(--wk-color-text); --muted: var(--wk-color-surface-hover); --muted-foreground: var(--wk-color-text-secondary);
  --accent: var(--wk-color-surface-hover); --accent-foreground: var(--wk-color-text); --destructive: var(--wk-color-error); --destructive-foreground: #ffffff;
  --border: var(--wk-color-border); --input: var(--wk-color-border); --ring: var(--wk-color-brand);`;
}

export const tokenCss = `:root {${colorVars(lightColor)}
  --wk-font-family: ${tokens.font.family}; --wk-font-family-mono: ${tokens.font.mono};
  --wk-font-size-body: ${tokens.font.size.body}; --wk-font-size-small: ${tokens.font.size.small}; --wk-font-size-title: ${tokens.font.size.title};
  --wk-line-height-body: ${tokens.font.lineHeight.body}; --wk-line-height-small: ${tokens.font.lineHeight.small}; --wk-line-height-title: ${tokens.font.lineHeight.title};
  --wk-radius-small: ${tokens.radius.small}; --wk-radius-control: ${tokens.radius.control}; --wk-radius-popup: ${tokens.radius.popup}; --wk-radius-panel: ${tokens.radius.panel}; --wk-radius-round: ${tokens.radius.round};
  --wk-shadow-popup: ${tokens.shadow.popup}; --wk-shadow-panel: ${tokens.shadow.panel};
  --wk-overlay-menu-min-width: ${tokens.overlay.menuMinWidth}; --wk-overlay-menu-padding: ${tokens.overlay.menuPadding}; --wk-overlay-settings-z: ${tokens.overlay.settingsZIndex}; --wk-overlay-dialog-z: ${tokens.overlay.dialogZIndex}; --wk-overlay-anchored-z: ${tokens.overlay.anchoredZIndex};
  --wk-motion-duration: ${tokens.overlay.duration}; --wk-motion-easing: ${tokens.overlay.easing};
}
:root[theme-mode="dark"] {${colorVars(darkColor)}}`;
