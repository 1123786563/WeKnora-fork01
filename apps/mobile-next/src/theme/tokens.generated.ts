// 自动生成：node scripts/gen-theme.mjs ← docs/mobile-rebuild/design-ref/tokens.json
// 禁止手改；令牌源见 DESIGN_ROOT weknora-expo-hifi-v2（SHA 54e02578a6ddbaae4366729285198e3d812df36a）
export const generatedTokens = {
  "colors": {
    "light": {
      "canvas": "#EFF1EB",
      "bg": "#F6F7F3",
      "surface": "#FFFFFF",
      "surface-alt": "#EEF2ED",
      "ink": "#1B302B",
      "muted": "#60716B",
      "subtle": "#6B7A74",
      "line": "#DFE6DE",
      "control-line": "#869890",
      "brand": "#08766A",
      "brand-hover": "#065D54",
      "brand-soft": "#E1F1E9",
      "on-brand": "#FFFFFF",
      "accent": "#DDFAAD",
      "hero": "#DDEDE4",
      "hero-ink": "#153D30",
      "warning": "#865200",
      "warning-soft": "#FFF2D8",
      "danger": "#AF3D32",
      "danger-soft": "#FFF0EC",
      "info": "#365B99",
      "info-soft": "#EAF0FC",
      "purple": "#6753A0",
      "purple-soft": "#F0EBFA",
      "disabled": "#748279",
      "disabled-bg": "#E6EAE4",
      "focus": "#08766A",
      "scrim": "#102A237A"
    },
    "dark": {
      "canvas": "#0D1815",
      "bg": "#111F1B",
      "surface": "#1A2C25",
      "surface-alt": "#21382F",
      "ink": "#EAF4EF",
      "muted": "#B4C8BE",
      "subtle": "#A8BEB2",
      "line": "#365246",
      "control-line": "#759A85",
      "brand": "#94E0BA",
      "brand-hover": "#B0EECF",
      "brand-soft": "#244C39",
      "on-brand": "#113525",
      "accent": "#D5F9A9",
      "hero": "#274936",
      "hero-ink": "#EAFAD4",
      "warning": "#FFD087",
      "warning-soft": "#47351F",
      "danger": "#FFB0A5",
      "danger-soft": "#4A2A25",
      "info": "#ADC9FF",
      "info-soft": "#273B55",
      "purple": "#D6C3FF",
      "purple-soft": "#3D3151",
      "disabled": "#9BB0A3",
      "disabled-bg": "#2A3A31",
      "focus": "#94E0BA",
      "scrim": "#000000B8"
    }
  },
  "spacing": {
    "0": 0,
    "2": 2,
    "4": 4,
    "6": 6,
    "8": 8,
    "12": 12,
    "16": 16,
    "20": 20,
    "24": 24,
    "28": 28,
    "32": 32,
    "40": 40,
    "48": 48,
    "64": 64
  },
  "radius": {
    "xs": 6,
    "sm": 10,
    "control": 12,
    "card": 20,
    "hero": 24,
    "sheet": 28,
    "pill": 999
  },
  "typography": {
    "caption": {
      "fontSize": 12,
      "lineHeight": 18,
      "fontWeight": "400"
    },
    "label": {
      "fontSize": 13,
      "lineHeight": 20,
      "fontWeight": "600"
    },
    "body-sm": {
      "fontSize": 14,
      "lineHeight": 22,
      "fontWeight": "400"
    },
    "body": {
      "fontSize": 16,
      "lineHeight": 26,
      "fontWeight": "400"
    },
    "subtitle": {
      "fontSize": 18,
      "lineHeight": 26,
      "fontWeight": "600"
    },
    "title": {
      "fontSize": 22,
      "lineHeight": 30,
      "fontWeight": "650"
    },
    "display": {
      "fontSize": 28,
      "lineHeight": 38,
      "fontWeight": "700"
    },
    "metric": {
      "fontSize": 32,
      "lineHeight": 40,
      "fontWeight": "700"
    }
  },
  "size": {
    "touch": 48,
    "icon-touch": 44,
    "icon": 22,
    "page-padding": 20,
    "phone-width": 390,
    "phone-height": 844,
    "button-height": 50,
    "tab-height": 66
  },
  "motion": {
    "fast": 120,
    "standard": 180,
    "sheet": 240,
    "reduced": 0
  },
  "elevation": {
    "card": {
      "x": 0,
      "y": 2,
      "blur": 8,
      "opacity": 0.025
    },
    "sheet": {
      "x": 0,
      "y": -8,
      "blur": 32,
      "opacity": 0.14
    }
  }
} as const;
export type ThemeMode = "light" | "dark";
