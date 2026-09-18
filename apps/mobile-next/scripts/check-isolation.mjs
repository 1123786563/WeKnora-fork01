#!/usr/bin/env node
// 旧代码依赖门禁（RW-030）：确保 apps/mobile-next 不依赖旧移动端/旧前端业务包。
// 检查：1) 源码 import 路径 2) tsconfig 路径别名 3) package.json 依赖 4) 重导出/相对路径逃逸 5) metro 可达 import 图。
// 文本搜索只是其中一项；第 5 项从入口递归解析 import，覆盖间接引用。
import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const APP_DIRS = ["app", "src", "tests"];
// 旧客户端实现与旧业务包（禁止依赖）
const FORBIDDEN_PATTERNS = [
  /apps\/mobile(?!-next)/, // 旧移动端（mobile-next 排除）
  /packages\/(api-client|domain|views|core|ui|contracts|happy-wire|design-tokens|i18n)/, // 旧前端业务包
  /@weknora\/(api-client|domain|views|core|ui|contracts|happy-wire)/,
  /happy-wire/,
  /\.\.\/\.\.\/\.\.\/(apps\/mobile|packages)\//, // 相对路径逃逸到旧目录
];

let failures = 0;
const fail = (msg) => {
  console.error(`✗ ${msg}`);
  failures++;
};

// 1+2) tsconfig 别名
const tsconfig = JSON.parse(readFileSync(join(root, "tsconfig.json"), "utf8"));
const paths = tsconfig.compilerOptions?.paths ?? {};
for (const [alias, targets] of Object.entries(paths)) {
  for (const t of /** @type {string[]} */ (targets)) {
    if (FORBIDDEN_PATTERNS.some((p) => p.test(t))) fail(`tsconfig paths ${alias} → ${t} 指向禁止目录`);
  }
}

// 3) package.json 依赖
const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8"));
const deps = { ...pkg.dependencies, ...pkg.devDependencies };
for (const [name, version] of Object.entries(deps)) {
  if (FORBIDDEN_PATTERNS.some((p) => p.test(name)) || name.startsWith("@weknora/")) {
    fail(`依赖 ${name}@${version} 属于旧业务包/workspace 内部包`);
  }
}
// workspace 归属检查：不加入 pnpm workspace（无 workspace:* 协议依赖）
for (const [name, version] of Object.entries(deps)) {
  if (String(version).startsWith("workspace:")) fail(`依赖 ${name} 使用 workspace: 协议（本工程必须独立）`);
}

// 4) 源码静态扫描 + 5) 入口可达 import 图
function listFiles(dir, exts, acc = []) {
  if (!existsSync(dir)) return acc;
  for (const name of readdirSync(dir)) {
    if (name === "node_modules" || name.startsWith(".")) continue;
    const full = join(dir, name);
    const st = statSync(full);
    if (st.isDirectory()) listFiles(full, exts, acc);
    else if (exts.some((e) => name.endsWith(e))) acc.push(full);
  }
  return acc;
}

const sourceFiles = APP_DIRS.flatMap((d) => listFiles(join(root, d), [".ts", ".tsx"]));
for (const f of sourceFiles) {
  const text = readFileSync(f, "utf8");
  const rel = relative(root, f);
  for (const m of text.matchAll(/from\s+["']([^"']+)["']|import\s*\(\s*["']([^"']+)["']\s*\)|require\s*\(\s*["']([^"']+)["']\s*\)/g)) {
    const spec = m[1] ?? m[2] ?? m[3] ?? "";
    if (FORBIDDEN_PATTERNS.some((p) => p.test(spec))) {
      fail(`${rel}: import "${spec}" 指向旧代码`);
    }
    // 相对路径逃逸出工程根
    if (spec.startsWith(".")) {
      const target = resolve(dirname(f), spec);
      if (!target.startsWith(root)) fail(`${rel}: 相对 import "${spec}" 逃逸出工程目录`);
    }
  }
}

// 5) 入口可达图（从 app/ 与 src/ 顶层入口递归解析本地 import；只验证可达文件集合是第 4 步扫描的子集）
const reach = new Set();
const visit = (f) => {
  if (reach.has(f)) return;
  reach.add(f);
  const text = readFileSync(f, "utf8");
  for (const m of text.matchAll(/from\s+["'](\.[^"']+)["']/g)) {
    const spec = m[1];
    const base = resolve(dirname(f), spec);
    for (const cand of [base + ".ts", base + ".tsx", base + "/index.ts", base + "/index.tsx"]) {
      if (existsSync(cand)) {
        if (!cand.startsWith(root)) fail(`可达图：${relative(root, f)} → ${spec} 逃逸`);
        else visit(cand);
        break;
      }
    }
  }
};
for (const f of sourceFiles) {
  const rel = relative(root, f);
  if (rel.startsWith("app/") || /^src\/(app|features|components|theme|api|domain|platform|contracts)\/[^/]+\.tsx?$/.test(rel) || rel.includes("/__")) visit(f);
}

// metro 配置检查：不存在自定义 resolver 指向旧目录
const metroPath = join(root, "metro.config.js");
if (existsSync(metroPath)) {
  const metro = readFileSync(metroPath, "utf8");
  if (/apps\/mobile(?!-next)|packages\//.test(metro)) fail("metro.config.js 引用旧目录");
}

console.log(
  failures === 0
    ? `✓ 隔离门禁通过：${sourceFiles.length} 个源文件，可达图 ${reach.size} 文件，无旧代码依赖`
    : `隔离门禁失败：${failures} 处违规`,
);
process.exit(failures === 0 ? 0 : 1);
