import React, { createContext, useContext, useMemo, useState } from "react";
import { useColorScheme } from "react-native";
import { getTheme, type AppTheme, type ThemeMode } from "./theme";

export type ThemePreference = ThemeMode | "system";

interface ThemeContextValue {
  theme: AppTheme;
  mode: ThemeMode;
  preference: ThemePreference;
  setPreference: (p: ThemePreference) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({
  children,
  initialPreference = "system",
}: {
  children: React.ReactNode;
  initialPreference?: ThemePreference;
}) {
  const [preference, setPreference] = useState<ThemePreference>(initialPreference);
  const system = useColorScheme();
  const mode: ThemeMode = preference === "system" ? (system === "dark" ? "dark" : "light") : preference;
  const value = useMemo<ThemeContextValue>(
    () => ({ theme: getTheme(mode), mode, preference, setPreference }),
    [mode, preference],
  );
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme 必须在 ThemeProvider 内使用");
  return ctx;
}
