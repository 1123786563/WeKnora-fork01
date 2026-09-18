#!/usr/bin/env node
// 从设计令牌源生成原生主题 TS。
// 源: docs/mobile-rebuild/design-ref/tokens.json（DESIGN_ROOT weknora-expo-hifi-v2/tokens/tokens.json 归档副本）
// 运行: node scripts/gen-theme.mjs（生成后提交；tests/theme.test.ts 会比对源与生成物一致性）
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const tokenSrc = join(root, "..", "..", "docs", "mobile-rebuild", "design-ref", "tokens.json");
const outPath = join(root, "src", "theme", "tokens.generated.ts");

const raw = JSON.parse(readFileSync(tokenSrc, "utf8"));

const pickColors = (theme) => {
  const out = {};
  for (const [k, v] of Object.entries(theme)) {
    if (v && v.$type === "color") out[k] = v.$value;
  }
  return out;
};

const pickScale = (obj) => {
  const out = {};
  for (const [k, v] of Object.entries(obj)) out[k] = v.$value;
  return out;
};

const pickTypo = (t) => {
  const out = {};
  for (const [k, v] of Object.entries(t)) out[k] = { fontSize: v.fontSize, lineHeight: v.lineHeight, fontWeight: String(v.fontWeight) };
  return out;
};

const pickElevation = (e) => {
  const out = {};
  for (const [k, v] of Object.entries(e)) out[k] = { x: v.x, y: v.y, blur: v.blur, opacity: v.opacity };
  return out;
};

const data = {
  colors: { light: pickColors(raw.theme.light), dark: pickColors(raw.theme.dark) },
  spacing: pickScale(raw.space),
  radius: pickScale(raw.radius),
  typography: pickTypo(raw.typography),
  size: raw.size,
  motion: raw.motion,
  elevation: pickElevation(raw.elevation),
};

const header = `// 自动生成：node scripts/gen-theme.mjs ← docs/mobile-rebuild/design-ref/tokens.json
// 禁止手改；令牌源见 DESIGN_ROOT weknora-expo-hifi-v2（SHA 54e02578a6ddbaae4366729285198e3d812df36a）
`;
const body = `export const generatedTokens = ${JSON.stringify(data, null, 2)} as const;
export type ThemeMode = "light" | "dark";
`;

mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(outPath, header + body);
console.log(`written ${outPath}`);
