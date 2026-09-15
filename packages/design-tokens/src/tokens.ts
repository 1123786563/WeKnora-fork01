export const designTokens = {
  color: {
    text: '#172033',
    muted: '#66758b',
    surface: '#ffffff',
    surfaceMuted: '#f3f3f3',
    canvas: '#f7f9fc',
    border: '#dce3ed',
    lineControl: '#dcdcdc',
    primary: '#2e6de6',
    danger: '#b42318',
    accent: '#07c05f',
    accentHover: '#08dd6e',
    accentActive: '#06b04d',
  },
  radius: { field: '3px', control: '6px', card: '8px', large: '12px', pill: '999px' },
  space: { xs: '0.25rem', sm: '0.5rem', md: '1rem', lg: '1.5rem', xl: '3rem' },
  state: { focusRing: '3px solid rgb(46 109 230 / 35%)', disabledOpacity: 0.6 },
  typography: {
    body: { fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif', lineHeight: 1.5 },
    mono: { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' },
    heading: { weight: 700 },
  },
} as const;

export type DesignTokens = typeof designTokens;
