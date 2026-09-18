import { generatedTokens, type ThemeMode } from "./tokens.generated";

export type { ThemeMode };

export type ColorToken = keyof typeof generatedTokens.colors.light;
export type SpacingToken = keyof typeof generatedTokens.spacing;
export type RadiusToken = keyof typeof generatedTokens.radius;
export type TypeToken = keyof typeof generatedTokens.typography;

export interface AppTheme {
  mode: ThemeMode;
  c: Readonly<Record<ColorToken, string>>;
  space: Readonly<Record<SpacingToken, number>>;
  radius: Readonly<Record<RadiusToken, number>>;
  type: typeof generatedTokens.typography;
  size: typeof generatedTokens.size;
  motion: typeof generatedTokens.motion;
  elevation: typeof generatedTokens.elevation;
}

const build = (mode: ThemeMode): AppTheme => ({
  mode,
  c: generatedTokens.colors[mode],
  space: generatedTokens.spacing,
  radius: generatedTokens.radius,
  type: generatedTokens.typography,
  size: generatedTokens.size,
  motion: generatedTokens.motion,
  elevation: generatedTokens.elevation,
});

export const lightTheme: AppTheme = build("light");
export const darkTheme: AppTheme = build("dark");

export const getTheme = (mode: ThemeMode): AppTheme => (mode === "dark" ? darkTheme : lightTheme);
