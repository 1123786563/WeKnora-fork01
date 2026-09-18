// 200% 动态字体纪律（5.2/5.3 可访问性要求的当前环境可验证部分）：
// RN 默认允许字体缩放；本测试静态断言全部页面/组件源码未禁用缩放（无 allowFontScaling={false}），
// 且正文行高来自令牌（随字号缩放），布局不靠缩小字体解决拥挤。
// 真机 200% 渲染截图验证待模拟器环境恢复（runtime 缺失，blocked-env 如实记录）。
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

const ROOT = join(__dirname, "..", "..");

function listFiles(dir: string, exts: string[], acc: string[] = []): string[] {
  const { readdirSync } = require("node:fs") as typeof import("node:fs");
  for (const name of readdirSync(dir)) {
    if (name === "node_modules" || name.startsWith(".")) continue;
    const full = join(dir, name);
    const st = require("node:fs").statSync(full);
    if (st.isDirectory()) listFiles(full, exts, acc);
    else if (exts.some((e) => name.endsWith(e))) acc.push(full);
  }
  return acc;
}

describe("RW-031 可访问性：动态字体不被禁用", () => {
  const dirs = [join(ROOT, "app"), join(ROOT, "src", "components")].filter(existsSync);
  const files = dirs.flatMap((d) => listFiles(d, [".tsx", ".ts"]));

  it("扫描范围非空", () => {
    expect(files.length).toBeGreaterThan(20);
  });

  it("无 allowFontScaling={false} / adjustsFontSizeToFit（禁止靠缩小文字解决拥挤）", () => {
    const offenders: string[] = [];
    for (const f of files) {
      const text = readFileSync(f, "utf8");
      if (/allowFontScaling\s*=\s*\{?\s*false/.test(text)) offenders.push(`${f}: allowFontScaling=false`);
      if (/adjustsFontSizeToFit/.test(text)) offenders.push(`${f}: adjustsFontSizeToFit`);
      if (/maxFontSizeMultiplier\s*=\s*\{?\s*[\d.]+\s*\}?/.test(text) && !/maxFontSizeMultiplier\s*=\s*\{?\s*(undefined|Infinity)/.test(text)) {
        offenders.push(`${f}: maxFontSizeMultiplier 限制`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("正文排版取自主题令牌（核心组件引用 theme.type，而非硬编码 fontSize）", () => {
    const stateView = readFileSync(join(ROOT, "src", "components", "StateView.tsx"), "utf8");
    expect(stateView).toMatch(/theme\.type\./);
    const formField = readFileSync(join(ROOT, "src", "components", "FormField.tsx"), "utf8");
    expect(formField).toMatch(/theme\.type\.body/);
  });
});
