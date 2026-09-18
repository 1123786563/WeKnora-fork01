// RW-002：主题值必须逐一来自设计令牌源（tokens.json），不允许页面写死。
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { generatedTokens } from "@/theme/tokens.generated";
import { getTheme, lightTheme, darkTheme } from "@/theme/theme";

const tokenSrc = JSON.parse(
  readFileSync(
    join(__dirname, "..", "..", "..", "..", "docs", "mobile-rebuild", "design-ref", "tokens.json"),
    "utf8",
  ),
) as Record<string, any>;

describe("RW-002 theme generated from design tokens", () => {
  it("生成物与令牌源逐值一致（light）", () => {
    const colors = generatedTokens.colors.light as unknown as Record<string, string>;
    for (const [key, def] of Object.entries(tokenSrc.theme.light) as [string, any][]) {
      expect(colors[key]).toBe(def.$value);
    }
  });

  it("生成物与令牌源逐值一致（dark）", () => {
    const colors = generatedTokens.colors.dark as unknown as Record<string, string>;
    for (const [key, def] of Object.entries(tokenSrc.theme.dark) as [string, any][]) {
      expect(colors[key]).toBe(def.$value);
    }
  });

  it("间距与圆角逐值来自令牌源", () => {
    const spacing = generatedTokens.spacing as unknown as Record<string, number>;
    for (const [k, v] of Object.entries(tokenSrc.space) as [string, any][]) {
      expect(spacing[k]).toBe(v.$value);
    }
    const radius = generatedTokens.radius as unknown as Record<string, number>;
    for (const [k, v] of Object.entries(tokenSrc.radius) as [string, any][]) {
      expect(radius[k]).toBe(v.$value);
    }
  });

  it("设计核对锚点值（tokens.json 为权威，decisions D-01）", () => {
    expect(lightTheme.c.brand).toBe("#08766A");
    expect(lightTheme.c.bg).toBe("#F6F7F3");
    expect(lightTheme.c.surface).toBe("#FFFFFF");
    expect(lightTheme.c["hero-ink"]).toBe("#153D30");
    expect(lightTheme.c.accent).toBe("#DDFAAD");
    expect(darkTheme.c.brand).toBe("#94E0BA");
    expect(darkTheme.c.bg).toBe("#111F1B");
    expect(darkTheme.c.surface).toBe("#1A2C25");
    expect(darkTheme.c["hero-ink"]).toBe("#EAFAD4");
  });

  it("尺寸体系：body 16/26、按钮高 50、触控 48、图标区 44、边距 20、Sheet 圆角 28、卡片 20、控件 12", () => {
    expect(generatedTokens.typography.body.fontSize).toBe(16);
    expect(generatedTokens.typography.body.lineHeight).toBe(26);
    expect(generatedTokens.size["button-height"]).toBe(50);
    expect(generatedTokens.size.touch).toBe(48);
    expect(generatedTokens.size["icon-touch"]).toBe(44);
    expect(generatedTokens.size["page-padding"]).toBe(20);
    expect(generatedTokens.radius.sheet).toBe(28);
    expect(generatedTokens.radius.card).toBe(20);
    expect(generatedTokens.radius.control).toBe(12);
  });

  it("getTheme 双主题解析", () => {
    expect(getTheme("light").mode).toBe("light");
    expect(getTheme("dark").mode).toBe("dark");
    expect(getTheme("dark").c.surface).not.toBe(getTheme("light").c.surface);
  });
});
