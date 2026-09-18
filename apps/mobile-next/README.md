# WeKnora 移动工作台（mobile-next）

从零实现的 Expo / React Native 移动 AI SaaS 工作台（Calm Emerald / 静谧翡翠设计 v2.0），对接本仓库 WeKnora Go 后端。不复用旧 `apps/mobile` / Happy / 旧前端业务包（隔离门禁：`npm run check:isolation`）。

## 环境与启动

- Node ≥ 20、npm（独立安装，不加入根 pnpm workspace）
- Expo SDK 55 / React 19.3 / RN 0.83 / expo-router

```bash
cd apps/mobile-next
npm install

# 连本地后端开发（默认 origin http://localhost:8082，仅 __DEV__ 放行本地 http）
EXPO_PUBLIC_WEKNORA_ORIGIN=http://localhost:8082 npx expo start

# iOS / Android（Expo Go 或 dev client）
npx expo start --ios
npx expo start --android
```

后端地址也可在 App 内 M01「私有部署服务器」入口修改；生产仅接受受信 https origin。

## 质量入口

```bash
npm run typecheck          # tsc --noEmit
npm test                   # jest（单元+组件+SSE+状态机）
npm run check:isolation    # 旧代码依赖门禁（import 图/别名/依赖/可达图/metro）
node scripts/gen-theme.mjs # 从设计令牌源重新生成主题（勿手改 tokens.generated.ts）
node tests/sse/gen/main.go > tests/sse/fixtures/go-real-bytes.txt  # 重新生成真实 Go SSE 字节 fixture
```

## 结构

```
app/                 Expo Router 路由（18 页：login/spaces/(tabs)×4/new-task/agents/...）
src/
  app/AppProvider    组合根：装配 platform+api+domain+auth，页面只消费 context
  theme/             tokens.json 生成的双主题（light/dark）
  components/        ActionButton/FormField/StatusBadge/TaskCard/DecisionSheet/RunStatusStack…
  contracts/         wire 类型与运行时解码器（对齐 Go DTO）
  api/               HttpClient（origin 信任边界/401 单飞刷新/错误分类）+ WeKnoraApi
  domain/            ScopeCoordinator（generation 切换隔离）
  platform/          SecureStore 凭证 + SQLite 四表（events/cursors/projections/pending）+ 内存实现
  features/          auth/workbench(提交状态机+SSE parser)/executions/…
tests/               theme/components/api/domain/sse/features
```

## 数据与缓存命名空间

- SQLite 库 `weknora_mobile_next_v1.db`；凭证仅 expo-secure-store；缓存按 `origin+userId+tenantId` scope 隔离；不读取旧 App 任何数据。

## 已知边界（见 docs/evidence/mobile-rebuild/decisions.md）

- 后端暂缺 `/workbench/overview`、通用 inbox、统一四类交互、实时语音端点 → 客户端聚合/专端点/如实展示不可用（D-04~D-07）
- 真实设备链路（录音、推送、SSO 浏览器回跳、双平台 E2E）待模拟器/后端联调环境执行，状态见 progress.md
