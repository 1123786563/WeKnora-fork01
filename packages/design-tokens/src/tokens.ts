export const designTokens = {
  color: { text: '#172033', muted: '#66758b', surface: '#ffffff', canvas: '#f7f9fc', border: '#dce3ed', primary: '#2e6de6', danger: '#b42318' },
  radius: { control: '6px', card: '8px' },
  space: { xs: '0.25rem', sm: '0.5rem', md: '1rem', lg: '1.5rem', xl: '3rem' },
  state: { focusRing: '3px solid rgb(46 109 230 / 35%)', disabledOpacity: 0.6 },
  typography: { body: { fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif', lineHeight: 1.5 }, heading: { weight: 700 } },
} as const;

export type DesignTokens = typeof designTokens;
