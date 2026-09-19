// 设置卡片左侧 logo 查询表 — ported from the Vue surface
// (frontend/src/views/settings/providerLogos.ts).
//
// 资源被分到两类：
//   color/ —— 厂商官方多色 SVG，<img> 直渲，保留品牌原色
//   mono/  —— 单色 SVG（多取自 simple-icons / 厂商 mark），按卡片品牌色染色
//
// 调用方传入 (category, id) 拿到 { mode, url }；找不到时返回 undefined，
// 卡片回落到原有的首字母 monogram。查表只在 Vite 构建里生效；在纯 Node
// （测试）环境 import.meta.glob 不存在，返回空表，卡片一律走 monogram。

export type ProviderCategory = 'vectorstore' | 'storage' | 'websearch' | 'parser' | 'sandbox';

export type LogoMatch = {
  mode: 'color' | 'mono';
  url: string;
};

type GlobFn = (pattern: string, options: { eager: boolean; query: string; import: string }) => Record<string, string>;

// The direct `import.meta.glob(...)` call is required for Vite's static
// transform. Outside Vite (node test runner) the function is undefined and
// the call throws, which the try/catch folds into an empty lookup — cards
// then fall back to the initial-letter monogram.
const colorModules: Record<string, string> = (() => {
  try { return import.meta.glob('../assets/img/providers/color/*/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>; } catch { return {}; }
})();
const monoModules: Record<string, string> = (() => {
  try { return import.meta.glob('../assets/img/providers/mono/*/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>; } catch { return {}; }
})();

const buildLookup = (modules: Record<string, string>, segment: string) => {
  const map: Partial<Record<ProviderCategory, Record<string, string>>> = {};
  const re = new RegExp(`providers/${segment}/([^/]+)/([^/]+)\\.svg$`);
  for (const [path, url] of Object.entries(modules)) {
    const match = path.match(re);
    if (!match) continue;
    const [, category, id] = match;
    const bucket = (map[category as ProviderCategory] ||= {});
    bucket[id.toLowerCase()] = url;
  }
  return map;
};

const colorLookup = buildLookup(colorModules, 'color');
const monoLookup = buildLookup(monoModules, 'mono');

export function providerLogo(
  category: ProviderCategory,
  id: string | undefined | null,
): LogoMatch | undefined {
  const key = (id ?? '').trim().toLowerCase();
  if (!key) return undefined;
  const color = colorLookup[category]?.[key];
  if (color) return { mode: 'color', url: color };
  const mono = monoLookup[category]?.[key];
  if (mono) return { mode: 'mono', url: mono };
  return undefined;
}
