# Mimosa Security Scan

- Scan: `scan-2026-09-18T11-15-57.437Z-f520b86e55ce`
- Created: 2026-09-18T11:15:57.437Z
- Depth: deep
- Run status: **inconclusive**
- Findings: 219 (0 business-logic candidate)
- 覆盖缺口:
  - 调用图部分不完整:部分调用为动态派发或超出分析规模,跨文件可达性可能不完整
- Verdict effect: `none`

## Findings

### HIGH · 代码注入 (frontend/src/i18n/localeGapScan.ts:164)

static · static-finding

动态执行了外部输入，可被注入代码

### HIGH · 命令注入 (apps/mobile/sources/sync/git-parsers/parseDiff.ts:127)

static · static-finding

外部数据被拼进 shell 命令执行，可注入任意系统命令

### HIGH · 命令注入 (apps/web/vite.config.ts:15)

static · static-finding

外部数据被拼进同步 shell 命令，可注入任意系统命令

### HIGH · 命令注入 (cmd/desktop/update.go:332)

static · static-finding

外部数据进入进程执行接口，可能命令注入。

### HIGH · 命令注入 (cmd/desktop/update.go:402)

static · static-finding

外部数据进入进程执行接口，可能命令注入。

### HIGH · 命令注入 (docs/poc/docker-sandbox/main.go:375)

static · static-finding

外部数据进入进程执行接口，可能命令注入。

### HIGH · 命令注入 (docs/poc/docker-sandbox/main.go:380)

static · static-finding

外部数据进入进程执行接口，可能命令注入。

### HIGH · 命令注入 (docs/poc/docker-sandbox/main.go:383)

static · static-finding

外部数据进入进程执行接口，可能命令注入。

### HIGH · 命令注入 (frontend/src/i18n/localeKeyAudit.ts:709)

static · static-finding

外部数据被拼进同步 shell 命令，可注入任意系统命令

### HIGH · 硬编码凭据 (apps/mobile/sources/text/_default.ts:105)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/_default.ts:106)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/_default.ts:784)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/_default.ts:886)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/ca.ts:106)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/ca.ts:107)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/ca.ts:756)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/ca.ts:858)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/en.ts:121)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/en.ts:122)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/en.ts:770)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/en.ts:872)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/es.ts:106)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/es.ts:107)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/es.ts:756)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/es.ts:858)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/it.ts:105)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/it.ts:106)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/it.ts:754)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/it.ts:856)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pl.ts:117)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pl.ts:118)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pl.ts:772)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pl.ts:874)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pt.ts:106)

static · static-finding

enterSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pt.ts:107)

static · static-finding

invalidSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pt.ts:755)

static · static-finding

secretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/mobile/sources/text/translations/pt.ts:857)

static · static-finding

restoreWithSecretKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:54)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:55)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:56)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:69)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:70)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:71)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:73)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:75)

static · static-finding

confirmPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/auth/auth-state.ts:76)

static · static-finding

confirmPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (apps/web/src/settings/ModelSettingsPanel.tsx:744)

static · static-finding

api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:2317)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:2319)

static · static-finding

confirmPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:2339)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:2340)

static · static-finding

confirmPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:2820)

static · static-finding

webhookSecret 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:4152)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:4183)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:4300)

static · static-finding

missingPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:5172)

static · static-finding

authTypeApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:6470)

static · static-finding

imaApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:6599)

static · static-finding

createApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/en-US.ts:6725)

static · static-finding

playgroundNeedApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:2816)

static · static-finding

webhookSecret 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:4148)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:4179)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:5168)

static · static-finding

authTypeApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:6466)

static · static-finding

imaApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:6595)

static · static-finding

createApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ja-JP.ts:6721)

static · static-finding

playgroundNeedApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:541)

static · static-finding

createApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:667)

static · static-finding

playgroundNeedApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:842)

static · static-finding

imaApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:2103)

static · static-finding

authTypeApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:3122)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:3153)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ko-KR.ts:4496)

static · static-finding

webhookSecret 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:541)

static · static-finding

createApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:667)

static · static-finding

playgroundNeedApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:842)

static · static-finding

imaApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:2103)

static · static-finding

authTypeApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:3122)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:3153)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:4496)

static · static-finding

webhookSecret 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:4942)

static · static-finding

password 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/ru-RU.ts:4944)

static · static-finding

confirmPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:545)

static · static-finding

createApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:671)

static · static-finding

playgroundNeedApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:848)

static · static-finding

imaApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:2109)

static · static-finding

authTypeApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:3128)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:3159)

static · static-finding

auto_create_api_key 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (frontend/src/i18n/locales/zh-CN.ts:4502)

static · static-finding

webhookSecret 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (internal/application/service/user.go:74)

static · static-finding

DetailInvalidOldPassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (internal/application/service/user.go:76)

static · static-finding

DetailSamePassword 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (internal/infrastructure/openmeter/commercial.go:39)

static · static-finding

EnvAPIKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (internal/sandbox/session_binding_redis.go:64)

static · static-finding

traffic_access_token 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (miniprogram/utils/i18n.js:46)

static · static-finding

missingApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (miniprogram/utils/i18n.js:90)

static · static-finding

missingApiKey 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### HIGH · 硬编码凭据 (scripts/migrate.sh:57)

static · static-finding

ENCODED_PASSWORD 的凭据值直接写在源码中，提交后可能造成密钥泄露。

### MEDIUM · 疑似跨文件污点 (apps/miniprogram/src/platform/transport.ts:71)

cross-file · static-finding

HTTP 请求输入 → docparser/mineru_cloud_converter.go:165 的 SSRF 服务端请求

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createMobileTransport 是 ssrf 入口 (apps/mobile/src/runtime.tsx:117)

cross-file · static-finding

不可信数据「环境变量」流入 createMobileTransport() → 污点链：createMobileTransport(sink:ssrf) （定义于 platform/transport.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/mobile/src/runtime.tsx:225)

cross-file · static-finding

环境变量 → auth/endpoints.ts:125 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · oidcUrl 是 ssrf 入口 (apps/mobile/src/runtime.tsx:225)

cross-file · static-finding

不可信数据「环境变量」流入 oidcUrl() → 污点链：oidcUrl(sink:ssrf) （定义于 auth/endpoints.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/auth/AuthPages.tsx:22)

cross-file · static-finding

URL 输入 → auth/api.ts:49 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · lookupInvite 是 ssrf 入口 (apps/web/src/auth/AuthPages.tsx:22)

cross-file · static-finding

不可信数据「URL 输入」流入 lookupInvite() → 污点链：lookupInvite(sink:ssrf) （定义于 auth/api.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/auth/AuthPages.tsx:23)

cross-file · static-finding

URL 输入 → auth/api.ts:53 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · acceptInvitation 是 ssrf 入口 (apps/web/src/auth/AuthPages.tsx:23)

cross-file · static-finding

不可信数据「URL 输入」流入 acceptInvitation() → 污点链：acceptInvitation(sink:ssrf) （定义于 auth/api.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/auth/AuthPages.tsx:30)

cross-file · static-finding

URL 输入 → auth/api.ts:49 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · lookupInvite 是 ssrf 入口 (apps/web/src/auth/AuthPages.tsx:30)

cross-file · static-finding

不可信数据「URL 输入」流入 lookupInvite() → 污点链：lookupInvite(sink:ssrf) （定义于 auth/api.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · stream 经 1 跳到达 ssrf (apps/web/src/chat/ChatRoutePage.tsx:1028)

cross-file · static-finding

不可信数据「URL 输入」流入 stream() → 污点链：stream → consumeChatTransport(sink:ssrf) （定义于 src/client.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · stream 经 1 跳到达 ssrf (apps/web/src/chat/ChatRoutePage.tsx:1035)

cross-file · static-finding

不可信数据「URL 输入」流入 stream() → 污点链：stream → consumeChatTransport(sink:ssrf) （定义于 src/client.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/configuration/ConfigurationEditor.tsx:68)

cross-file · static-finding

URL 输入 → src/configuration.ts:599 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createBrowserTransport 是 ssrf 入口 (apps/web/src/main.tsx:110)

cross-file · static-finding

不可信数据「URL 输入」流入 createBrowserTransport() → 污点链：createBrowserTransport(sink:ssrf) （定义于 platform/http.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/main.tsx:189)

cross-file · static-finding

URL 输入 → auth/endpoints.ts:142 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · switchTenant 是 ssrf 入口 (apps/web/src/main.tsx:189)

cross-file · static-finding

不可信数据「URL 输入」流入 switchTenant() → 污点链：switchTenant(sink:ssrf) （定义于 auth/endpoints.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/main.tsx:302)

cross-file · static-finding

URL 输入 → auth/endpoints.ts:159 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · acceptInvitationByToken 是 ssrf 入口 (apps/web/src/main.tsx:302)

cross-file · static-finding

不可信数据「URL 输入」流入 acceptInvitationByToken() → 污点链：acceptInvitationByToken(sink:ssrf) （定义于 auth/endpoints.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/organizations/OrganizationsPage.tsx:408)

cross-file · static-finding

URL 输入 → identity/organization.ts:63 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · listForOrganization 是 ssrf 入口 (apps/web/src/organizations/OrganizationsPage.tsx:408)

cross-file · static-finding

不可信数据「URL 输入」流入 listForOrganization() → 污点链：listForOrganization(sink:ssrf) （定义于 identity/organization.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/organizations/OrganizationsPage.tsx:409)

cross-file · static-finding

URL 输入 → identity/organization.ts:63 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · listForOrganization 是 ssrf 入口 (apps/web/src/organizations/OrganizationsPage.tsx:409)

cross-file · static-finding

不可信数据「URL 输入」流入 listForOrganization() → 污点链：listForOrganization(sink:ssrf) （定义于 identity/organization.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/organizations/OrganizationsPage.tsx:496)

cross-file · static-finding

URL 输入 → identity/organization.ts:45 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · joinById 是 ssrf 入口 (apps/web/src/organizations/OrganizationsPage.tsx:496)

cross-file · static-finding

不可信数据「URL 输入」流入 joinById() → 污点链：joinById(sink:ssrf) （定义于 identity/organization.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · submitJoinRequest 经 1 跳到达 ssrf (apps/web/src/organizations/OrganizationsPage.tsx:498)

cross-file · static-finding

不可信数据「URL 输入」流入 submitJoinRequest() → 污点链：submitJoinRequest → shareMutation(sink:ssrf) （定义于 identity/organization.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/organizations/OrganizationsPage.tsx:568)

cross-file · static-finding

URL 输入 → identity/tenant.ts:93 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · review 经 1 跳到达 ssrf (apps/web/src/organizations/OrganizationsPage.tsx:587)

cross-file · static-finding

不可信数据「URL 输入」流入 review() → 污点链：review → shareMutation(sink:ssrf) （定义于 identity/organization.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/organizations/OrganizationsPage.tsx:598)

cross-file · static-finding

URL 输入 → identity/tenant.ts:100 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/settings/McpSettingsPanel.tsx:826)

cross-file · static-finding

URL 输入 → src/configuration.ts:599 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (apps/web/src/settings/McpSettingsPanel.tsx:830)

cross-file · static-finding

URL 输入 → src/client.ts:298 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · confirmation 是 xss 入口 (docs/design/weknora-craft-hifi/prototype/app.js:315)

cross-file · static-finding

不可信数据「URL 输入」流入 confirmation() → 污点链：confirmation(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · confirmation 是 xss 入口 (docs/design/weknora-craft-hifi/prototype/app.js:318)

cross-file · static-finding

不可信数据「URL 输入」流入 confirmation() → 污点链：confirmation(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · render 是 xss 入口 (docs/WeKnora-Miniapp-Implementation-Kit/prototype/src/wireframe-base.js:94)

cross-file · static-finding

不可信数据「URL 输入」流入 render() → 污点链：render(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · render 是 xss 入口 (docs/WeKnora-Miniapp-Implementation-Kit/prototype/src/wireframe-base.js:95)

cross-file · static-finding

不可信数据「URL 输入」流入 render() → 污点链：render(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · render 是 xss 入口 (docs/WeKnora-Taro-Design/wireframes/assets-source.js:94)

cross-file · static-finding

不可信数据「URL 输入」流入 render() → 污点链：render(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · render 是 xss 入口 (docs/WeKnora-Taro-Design/wireframes/assets-source.js:95)

cross-file · static-finding

不可信数据「URL 输入」流入 render() → 污点链：render(sink:xss)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · counterCount 是 ssrf 入口 (internal/agent/recoverytest/provider/main.go:162)

cross-file · static-finding

不可信数据「环境变量」流入 counterCount() → 污点链：counterCount(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · buildReport 经 1 跳到达 ssrf (internal/agent/recoverytest/provider/main.go:212)

cross-file · static-finding

不可信数据「环境变量」流入 buildReport() → 污点链：buildReport → counterCount(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · counterCount 是 ssrf 入口 (internal/agent/recoverytest/provider/main.go:494)

cross-file · static-finding

不可信数据「环境变量」流入 counterCount() → 污点链：counterCount(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/agent/recoverytest/provider/main.go:495)

cross-file · static-finding

环境变量 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · buildReport 经 1 跳到达 ssrf (internal/agent/recoverytest/provider/main.go:794)

cross-file · static-finding

不可信数据「环境变量」流入 buildReport() → 污点链：buildReport → counterCount(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · counterCount 是 ssrf 入口 (internal/agent/recoverytest/provider/main.go:799)

cross-file · static-finding

不可信数据「环境变量」流入 counterCount() → 污点链：counterCount(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/application/service/storagebackend.go:208)

cross-file · static-finding

环境变量 → utils/security.go:132 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · SafeJoinUnderBase 是 path-traversal 入口 (internal/application/service/storagebackend.go:208)

cross-file · static-finding

不可信数据「环境变量」流入 SafeJoinUnderBase() → 污点链：SafeJoinUnderBase(sink:path-traversal) （定义于 utils/security.go）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/app_connector_oauth.go:110)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/app_connector_oauth.go:111)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/app_connector_oauth.go:201)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/app_connector_oauth.go:202)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/app_connector_oauth.go:204)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/auth.go:658)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:71 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/custom_agent.go:24)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · UpsertFAQEntries 经 2 跳到达 path-traversal (internal/handler/faq.go:171)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 UpsertFAQEntries() → 污点链：UpsertFAQEntries → faqImportEntriesFileName → SafeFileName(sink:path-traversal) （定义于 service/knowledge_faq_import.go）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/knowledge.go:362)

cross-file · static-finding

HTTP 请求输入 → client/knowledge.go:109 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/sandbox_config.go:310)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/session/qa.go:333)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/session/qa.go:1574)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/session/qa.go:1608)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/session/temporary_document.go:117)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (internal/handler/system.go:2237)

cross-file · static-finding

HTTP 请求输入 → secrets/secrets.go:60 的 文件路径操作

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · dockerCurrentContextName 是 path-traversal 入口 (internal/sandbox/docker_host.go:40)

cross-file · static-finding

不可信数据「环境变量」流入 dockerCurrentContextName() → 污点链：dockerCurrentContextName(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · dockerContextHost 是 path-traversal 入口 (internal/sandbox/docker_host.go:54)

cross-file · static-finding

不可信数据「环境变量」流入 dockerContextHost() → 污点链：dockerContextHost(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · dockerCurrentContextName 是 path-traversal 入口 (internal/sandbox/docker_host.go:65)

cross-file · static-finding

不可信数据「环境变量」流入 dockerCurrentContextName() → 污点链：dockerCurrentContextName(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · send 经 1 跳到达 ssrf (packages/api-client/src/client.ts:186)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 send() → 污点链：send → perform(sink:ssrf) （定义于 platform/transport.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · sendBinary 经 1 跳到达 ssrf (packages/api-client/src/client.ts:224)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 sendBinary() → 污点链：sendBinary → perform(sink:ssrf) （定义于 platform/transport.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createKnowledgeDocumentsApi 是 ssrf 入口 (packages/api-client/src/client.ts:243)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createKnowledgeDocumentsApi() → 污点链：createKnowledgeDocumentsApi(sink:ssrf) （定义于 knowledge/documents.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createKnowledgeFaqApi 是 ssrf 入口 (packages/api-client/src/client.ts:244)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createKnowledgeFaqApi() → 污点链：createKnowledgeFaqApi(sink:ssrf) （定义于 knowledge/faq.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createKnowledgeSettingsApi 是 ssrf 入口 (packages/api-client/src/client.ts:245)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createKnowledgeSettingsApi() → 污点链：createKnowledgeSettingsApi(sink:ssrf) （定义于 knowledge/settings.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createWikiPagesApi 是 ssrf 入口 (packages/api-client/src/client.ts:246)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createWikiPagesApi() → 污点链：createWikiPagesApi(sink:ssrf) （定义于 wiki/pages.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createDataSourcesApi 是 ssrf 入口 (packages/api-client/src/client.ts:247)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createDataSourcesApi() → 污点链：createDataSourcesApi(sink:ssrf) （定义于 src/datasource.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createCommercialApi 是 ssrf 入口 (packages/api-client/src/client.ts:248)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createCommercialApi() → 污点链：createCommercialApi(sink:ssrf) （定义于 src/commercial.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createAppConnectorApi 是 ssrf 入口 (packages/api-client/src/client.ts:249)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createAppConnectorApi() → 污点链：createAppConnectorApi(sink:ssrf) （定义于 src/appconnector.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createAuthApi 是 ssrf 入口 (packages/api-client/src/client.ts:250)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createAuthApi() → 污点链：createAuthApi(sink:ssrf) （定义于 auth/endpoints.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatSessionsApi 是 ssrf 入口 (packages/api-client/src/client.ts:251)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatSessionsApi() → 污点链：createChatSessionsApi(sink:ssrf) （定义于 chat/sessions.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createSandboxTerminalApi 是 ssrf 入口 (packages/api-client/src/client.ts:252)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createSandboxTerminalApi() → 污点链：createSandboxTerminalApi(sink:ssrf) （定义于 sandbox/terminal.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createSandboxSkillInstallApi 是 ssrf 入口 (packages/api-client/src/client.ts:256)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createSandboxSkillInstallApi() → 污点链：createSandboxSkillInstallApi(sink:ssrf) （定义于 sandbox/skill-install.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createSandboxConfigurationsApi 是 ssrf 入口 (packages/api-client/src/client.ts:270)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createSandboxConfigurationsApi() → 污点链：createSandboxConfigurationsApi(sink:ssrf) （定义于 src/sandbox-configurations.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createConfigurationApi 是 ssrf 入口 (packages/api-client/src/client.ts:271)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createConfigurationApi() → 污点链：createConfigurationApi(sink:ssrf) （定义于 src/configuration.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatApprovalsApi 是 ssrf 入口 (packages/api-client/src/client.ts:272)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatApprovalsApi() → 污点链：createChatApprovalsApi(sink:ssrf) （定义于 chat/approvals.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatSteerApi 是 ssrf 入口 (packages/api-client/src/client.ts:273)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatSteerApi() → 污点链：createChatSteerApi(sink:ssrf) （定义于 chat/steer.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatAttachmentsApi 是 ssrf 入口 (packages/api-client/src/client.ts:274)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatAttachmentsApi() → 污点链：createChatAttachmentsApi(sink:ssrf) （定义于 chat/attachments.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatSuggestionsApi 是 ssrf 入口 (packages/api-client/src/client.ts:275)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatSuggestionsApi() → 污点链：createChatSuggestionsApi(sink:ssrf) （定义于 chat/suggestions.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createChatArtifactsApi 是 ssrf 入口 (packages/api-client/src/client.ts:276)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createChatArtifactsApi() → 污点链：createChatArtifactsApi(sink:ssrf) （定义于 chat/artifacts.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createIdentityApi 经 1 跳到达 ssrf (packages/api-client/src/client.ts:277)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createIdentityApi() → 污点链：createIdentityApi → createTenantMembersApi(sink:ssrf) （定义于 identity/index.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createAdministrationApi 是 ssrf 入口 (packages/api-client/src/client.ts:278)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createAdministrationApi() → 污点链：createAdministrationApi(sink:ssrf) （定义于 administration/index.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createSettingsApi 是 ssrf 入口 (packages/api-client/src/client.ts:279)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createSettingsApi() → 污点链：createSettingsApi(sink:ssrf) （定义于 settings/index.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · createEmbedApi 是 ssrf 入口 (packages/api-client/src/client.ts:280)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 createEmbedApi() → 污点链：createEmbedApi(sink:ssrf) （定义于 embed/index.ts）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · request 是 ssrf 入口 (packages/api-client/src/client.ts:328)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 request() → 污点链：request(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · request 是 ssrf 入口 (packages/api-client/src/client.ts:331)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 request() → 污点链：request(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · request 是 ssrf 入口 (packages/api-client/src/client.ts:337)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 request() → 污点链：request(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · request 是 ssrf 入口 (packages/api-client/src/client.ts:353)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 request() → 污点链：request(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · consumeChatTransport 是 ssrf 入口 (packages/api-client/src/client.ts:401)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 consumeChatTransport() → 污点链：consumeChatTransport(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · request 是 ssrf 入口 (packages/api-client/src/client.ts:409)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 request() → 污点链：request(sink:ssrf)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (packages/api-client/src/transport/json.ts:133)

cross-file · static-finding

HTTP 请求输入 → src/EmbedApp.tsx:55 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · fetcher 是 ssrf 入口 (packages/api-client/src/transport/json.ts:133)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 fetcher() → 污点链：fetcher(sink:ssrf) （定义于 src/EmbedApp.tsx）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### MEDIUM · 疑似跨文件污点 (packages/api-client/src/transport/json.ts:175)

cross-file · static-finding

HTTP 请求输入 → src/EmbedApp.tsx:55 的 SSRF 服务端请求伪造 server-side request

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · fetcher 是 ssrf 入口 (packages/api-client/src/transport/json.ts:175)

cross-file · static-finding

不可信数据「HTTP 请求输入」流入 fetcher() → 污点链：fetcher(sink:ssrf) （定义于 src/EmbedApp.tsx）

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · main 是 path-traversal 入口 (scripts/open-connector/check_deployment.py:312)

cross-file · static-finding

不可信数据「命令行参数」流入 main() → 污点链：main(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · main 是 path-traversal 入口 (scripts/open-connector/contract_gate.py:72)

cross-file · static-finding

不可信数据「命令行参数」流入 main() → 污点链：main(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · main 是 path-traversal 入口 (scripts/open-connector/release_gate.py:179)

cross-file · static-finding

不可信数据「命令行参数」流入 main() → 污点链：main(sink:path-traversal)

Proof gaps:

- 静态 advisory 需要人工确认真实数据流和可利用性。

### HIGH · 路径穿越 (docreader/parser/excel_convert.py:59)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/parser/markdown_parser.py:432)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/parser/opendataloader_parser.py:334)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/parser/ppt_convert.py:47)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/parser/pptx_media.py:45)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/scripts/parse_local.py:97)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/scripts/parse_local.py:109)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · 路径穿越 (docreader/splitter/splitter.py:497)

static · static-finding

未校验的路径含 ../ 可访问预期之外的文件

### HIGH · SSRF 服务端请求伪造 (apps/miniprogram/src/platform/transport.ts:54)

static · static-finding

服务端直接按用户提供的 URL 发起请求，可能访问内网、云元数据或本机服务。

### HIGH · SSRF 服务端请求伪造 (apps/miniprogram/src/platform/transport.ts:114)

static · static-finding

服务端直接按用户提供的 URL 发起请求，可能访问内网、云元数据或本机服务。

### HIGH · SSRF 服务端请求伪造 (apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx:71)

static · static-finding

服务端直接按用户提供的 URL 发起请求，可能访问内网、云元数据或本机服务。

### HIGH · SSRF 服务端请求伪造 (deploy/openmeter/smoke.py:26)

static · static-finding

动态 URL 未经协议、目标主机和解析后 IP 边界校验就进入服务端请求，可能访问内网、云元数据或本机服务。

### HIGH · SSRF 服务端请求伪造 (deploy/openmeter/smoke.py:32)

static · static-finding

动态 URL 未经协议、目标主机和解析后 IP 边界校验就进入服务端请求，可能访问内网、云元数据或本机服务。

### HIGH · SSRF 服务端请求伪造 (scripts/saas/probe_case.py:181)

static · static-finding

动态 URL 未经协议、目标主机和解析后 IP 边界校验就进入服务端请求，可能访问内网、云元数据或本机服务。

### HIGH · 弱加密算法 (internal/application/service/knowledge_faq.go:1821)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/application/service/knowledge_util.go:114)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/application/service/knowledge_util.go:128)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/im/wecom/webhook_adapter.go:440)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/models/utils/signer.go:66)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/payment/alipay.go:260)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/payment/alipay.go:568)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### HIGH · 弱加密算法 (internal/searchutil/textutil.go:22)

static · static-finding

使用已被攻破的弱加密/哈希原语（DES/RC4/MD5/ECB 等）。

### MEDIUM · XXE（XML 外部实体） (docreader/parser/docx_merge.py:81)

static · static-finding

解析 XML 时启用外部实体，可读本地文件/发起请求

### HIGH · XML 实体扩展 (docreader/parser/xmind_parser.py:120)

static · static-finding

标准库 ElementTree 会接受内部 DTD 实体扩展；解析不可信 XML 时可能造成资源耗尽。


---

This report is a projection of sealed JSON artifacts. It is not runtime verification.
