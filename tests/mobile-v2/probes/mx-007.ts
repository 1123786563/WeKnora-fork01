// MX-007 probe · 设计令牌奇偶性与 web/native 一致性观察器
// 读取版本化令牌源（packages/design-tokens/src/mobile）做真实比对：
// 1) tokens.json theme.light/dark 扁平键集奇偶；
// 2) tokens.json($value) vs native-tokens.ts 颜色逐键一致 + tokens.css 变量存在；
// 3) 关键 ACTIVE 文本/状态对对比率 ≥4.5（disabled 对按 WCAG 1.4.3 豁免，另行报告）。
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { nativeTokens } from '../../../packages/design-tokens/src/mobile/native-tokens.ts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const mobileTokensDir = path.join(repoRoot, 'packages', 'design-tokens', 'src', 'mobile');

export interface ProbeInput {
  fixture: string;
}

export interface Observation {
  missingThemeKeys: string[];
  webNativeColorDiff: string[];
}

function flatten(record: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(record)) {
    // W3C 令牌节点 {$type, $value} 解包为纯值（键不带 $ 后缀）
    if (typeof value === 'object' && value !== null && !Array.isArray(value) && '$value' in (value as Record<string, unknown>)) {
      out[`${prefix}${key}`] = (value as Record<string, unknown>).$value;
      continue;
    }
    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      Object.assign(out, flatten(value as Record<string, unknown>, `${prefix}${key}.`));
    } else {
      out[`${prefix}${key}`] = value;
    }
  }
  return out;
}

function tokenValue(value: unknown): unknown {
  if (typeof value === 'object' && value !== null && '$value' in (value as Record<string, unknown>)) {
    return (value as Record<string, unknown>).$value;
  }
  return value;
}

function luminance(hex: string): number {
  let h = hex.replace('#', '');
  if (h.length === 8) h = h.slice(0, 6);
  const channels = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255);
  const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrastRatio(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

export async function runProbe(_input: ProbeInput): Promise<Observation> {
  const tokens = JSON.parse(await readFile(path.join(mobileTokensDir, 'tokens.json'), 'utf8')) as {
    theme: Record<'light' | 'dark', Record<string, unknown>>;
  };
  const css = await readFile(path.join(mobileTokensDir, 'tokens.css'), 'utf8');

  // 1) 明暗键集奇偶（相对 $type/$value 展平）
  const lightKeys = new Set(Object.keys(flatten(tokens.theme.light)));
  const darkKeys = new Set(Object.keys(flatten(tokens.theme.dark)));
  const missingThemeKeys: string[] = [];
  for (const key of lightKeys) if (!darkKeys.has(key)) missingThemeKeys.push(`dark missing ${key}`);
  for (const key of darkKeys) if (!lightKeys.has(key)) missingThemeKeys.push(`light missing ${key}`);

  // 2) web(tokens.json/css) vs native(native-tokens.ts) 颜色一致
  const webNativeColorDiff: string[] = [];
  // tokens.css 按 [data-theme="light"]/[data-theme="dark"] 分块定义 --<key>: <value>
  const cssBlocks: Record<'light' | 'dark', string> = { light: '', dark: '' };
  for (const blockMode of ['light', 'dark'] as const) {
    const selector = `[data-theme="${blockMode}"]{`;
    const start = css.indexOf(selector);
    if (start >= 0) {
      const open = css.indexOf('{', start);
      let depth = 0;
      for (let i = open; i < css.length; i++) {
        if (css[i] === '{') depth++;
        else if (css[i] === '}') {
          depth--;
          if (depth === 0) {
            cssBlocks[blockMode] = css.slice(open + 1, i);
            break;
          }
        }
      }
    }
  }
  /** 纯字符串查找 CSS 变量声明值（避免动态正则） */
  function cssVarValue(block: string, key: string): string | undefined {
    const anchor = `--${key}:`;
    const at = block.indexOf(anchor);
    if (at < 0) return undefined;
    const end = block.indexOf(';', at);
    if (end < 0) return undefined;
    return block.slice(at + anchor.length, end).trim();
  }
  for (const mode of ['light', 'dark'] as const) {
    const webColors = flatten(tokens.theme[mode]);
    const nativeColors = nativeTokens.colors[mode] as Record<string, string>;
    for (const [key, raw] of Object.entries(webColors)) {
      const value = tokenValue(raw);
      if (typeof value !== 'string') continue;
      if (!(key in nativeColors)) {
        webNativeColorDiff.push(`${mode}.${key}: missing in native`);
        continue;
      }
      if (String(value).toUpperCase() !== nativeColors[key].toUpperCase()) {
        webNativeColorDiff.push(`${mode}.${key}: ${value} != ${nativeColors[key]}`);
      }
      const declared = cssVarValue(cssBlocks[mode], key);
      if (declared === undefined) {
        webNativeColorDiff.push(`${mode}.${key}: css var --${key} missing in ${mode} block`);
      } else if (declared.toUpperCase() !== String(value).toUpperCase()) {
        webNativeColorDiff.push(`${mode}.${key}: css ${declared} != ${value}`);
      }
    }
    for (const key of Object.keys(nativeColors)) {
      if (!(key in webColors)) webNativeColorDiff.push(`${mode}.${key}: missing in tokens.json`);
    }
  }

  // 3) 对比率验收：ACTIVE 文本/状态对 ≥4.5；disabled 对按 WCAG 豁免单独收集（不作为失败）
  const activePairs: Array<[string, string]> = [
    ['ink', 'bg'], ['ink', 'surface'], ['ink', 'surface-alt'],
    ['muted', 'bg'], ['on-brand', 'brand'], ['hero-ink', 'hero'],
    ['warning', 'warning-soft'], ['danger', 'danger-soft'], ['info', 'info-soft'], ['brand', 'bg'],
  ];
  const exemptPairs: Array<[string, string]> = [['disabled', 'disabled-bg']];
  const contrastFailures: string[] = [];
  const exemptRatios: string[] = [];
  for (const mode of ['light', 'dark'] as const) {
    const colors = nativeTokens.colors[mode] as Record<string, string>;
    for (const [fg, bg] of activePairs) {
      const ratio = contrastRatio(colors[fg], colors[bg]);
      if (ratio < 4.5) contrastFailures.push(`${mode} ${fg}/${bg}=${ratio.toFixed(2)}`);
    }
    for (const [fg, bg] of exemptPairs) {
      exemptRatios.push(`${mode} ${fg}/${bg}=${contrastRatio(colors[fg], colors[bg]).toFixed(2)} (WCAG 1.4.3 inactive-control exemption)`);
    }
  }
  if (contrastFailures.length > 0) {
    throw new Error(`contrast failures (active pairs must be >=4.5): ${contrastFailures.join('; ')} | exempt: ${exemptRatios.join('; ')}`);
  }
  if (exemptRatios.length > 0) console.error('[mx-007 probe] exempt (documented, not asserted):', exemptRatios);

  return { missingThemeKeys: missingThemeKeys.sort(), webNativeColorDiff: webNativeColorDiff.sort() };
}
