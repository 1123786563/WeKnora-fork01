# WeKnora 账号登录与服务端会话同步 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Conduit（apps/mobile）用 WeKnora 账号密码登录（JWT 双令牌自动刷新），登录后与 WeKnora 网页端共享同一份服务端会话（列表同步、历史加载、继续聊、新对话回填 session id）。

**Architecture:** 新增 `lib/features/weknora/`（account 登录层 + sessions 同步层），复用现有 Direct Connection 通道承载一切 WeKnora 请求（Bearer JWT 镜像进 profile.apiKey）；适配器加"精确 session id 绑定"分支取代前缀猜测；会话以 `Conversation.metadata['weknoraSessionId']` 关联，消息实时加载不落库。

**Tech Stack:** Flutter/Dart（本仓 apps/mobile）、Riverpod、Dio、freezed 模型的既有约定、flutter_secure_storage。

**Spec:** `docs/superpowers/specs/2026-09-19-weknora-login-design.md`

## Global Constraints

- 所有工作目录：`/Users/wuyongjun/trea/WeKnora-fork01/apps/mobile`；Flutter 在 `$HOME/development/flutter/bin`（跑命令前 `export PATH="$HOME/development/flutter/bin:$PATH"`）。
- 本功能只改 apps/mobile，不动 WeKnora 后端、不动 conduit 上游仓。
- 每个任务结束时 `flutter analyze`（新增/改动文件零告警）+ 目标测试通过后 commit；**commit 只 stage 该任务清单内路径**（仓库有并行会话，禁止 `git add -A`）。mimosa 钩子若拦截，按其报告处理，不用 `--no-verify`。
- 代码与注释风格照抄仓内现状（英文注释、手写 toJson/fromJson 的 plain class，与 `DirectConnectionProfile` 一致；不用 freezed 新增模型）。
- l10n：新 key 先加 `app_en.arb`（含 `@` description）与 `app_zh.arb`，再补其余 12 个语言文件；校验 `dart run tool/validate_arb_locales.dart` 与 `dart run tool/verify_arb_descriptions.dart`。
- 后端契约（已从 Go 源码锁定，勿再猜）：
  - `POST {base}/api/v1/auth/login` body `{"email","password"}` → `{"success","message?","user"?{...},"active_tenant"?,"memberships":[],"token","refresh_token"}`；`success=false` 或 401 = 凭据错误。
  - `POST {base}/api/v1/auth/refresh` body `{"refreshToken"}` → `{"success","access_token","refresh_token"}`（双令牌轮换）；401 = refresh 失效。
  - `POST {base}/api/v1/auth/logout` body `{"refreshToken"}`（尽力而为）。
  - `GET {base}/api/v1/sessions?limit=50` → `{"data":[{"id","title","updated_at","created_at","last_request_state":{"agent_id","agent_enabled"},"is_pinned"}],"total"}`（字段见 internal/types/session.go，与冒烟实测一致）。
  - `GET {base}/api/v1/messages/{session_id}/load?limit=200` → `{"success","data":[Message]}`，Message 关键字段 `{"id","role":"user"|"assistant"|"system","content","knowledge_references","agent_steps","created_at"}`；会话不存在 → 404。

---

### Task 1: WeKnoraAccount 记录与存储接缝

**Files:**
- Create: `lib/features/weknora/account/weknora_account.dart`
- Test: `test/features/weknora/weknora_account_test.dart`

**Interfaces:**
- Produces: `WeKnoraAccount`（字段/方法见 Step 3，后续所有任务用它）；storage 接缝由 Task 3 注入函数承担，本任务不含 IO。

- [ ] **Step 1: 写失败测试**

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:conduit/features/weknora/account/weknora_account.dart';

void main() {
  test('round-trips through json', () {
    final account = WeKnoraAccount(
      baseUrl: 'http://localhost:8084',
      email: 'a@b.c',
      userId: 'u1',
      displayName: 'Alice',
      accessToken: 'at',
      refreshToken: 'rt',
    );
    final restored = WeKnoraAccount.fromJson(account.toJson());
    expect(restored, account);
  });

  test('copyWith only touches provided fields', () {
    final account = WeKnoraAccount(
      baseUrl: 'http://x', email: 'e', userId: 'u',
      displayName: 'd', accessToken: 'at', refreshToken: 'rt',
    );
    final next = account.copyWith(accessToken: 'at2', refreshToken: 'rt2');
    expect(next.accessToken, 'at2');
    expect(next.email, 'e');
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/weknora_account_test.dart`
Expected: FAIL（找不到 `weknora_account.dart`）

- [ ] **Step 3: 最小实现**

```dart
/// Stored credentials for the signed-in WeKnora account.
///
/// Mirrors the DirectConnectionProfile record style: a plain immutable class
/// with hand-written JSON serialization (see DirectConnectionProfile).
class WeKnoraAccount {
  const WeKnoraAccount({
    required this.baseUrl,
    required this.email,
    required this.userId,
    required this.displayName,
    required this.accessToken,
    required this.refreshToken,
  });

  final String baseUrl;
  final String email;
  final String userId;
  final String displayName;
  final String accessToken;
  final String refreshToken;

  WeKnoraAccount copyWith({
    String? baseUrl,
    String? email,
    String? userId,
    String? displayName,
    String? accessToken,
    String? refreshToken,
  }) => WeKnoraAccount(
    baseUrl: baseUrl ?? this.baseUrl,
    email: email ?? this.email,
    userId: userId ?? this.userId,
    displayName: displayName ?? this.displayName,
    accessToken: accessToken ?? this.accessToken,
    refreshToken: refreshToken ?? this.refreshToken,
  );

  factory WeKnoraAccount.fromJson(Map<String, dynamic> json) =>
      WeKnoraAccount(
        baseUrl: json['baseUrl'] as String,
        email: json['email'] as String,
        userId: json['userId'] as String? ?? '',
        displayName: json['displayName'] as String? ?? '',
        accessToken: json['accessToken'] as String,
        refreshToken: json['refreshToken'] as String,
      );

  Map<String, dynamic> toJson() => <String, dynamic>{
    'baseUrl': baseUrl,
    'email': email,
    'userId': userId,
    'displayName': displayName,
    'accessToken': accessToken,
    'refreshToken': refreshToken,
  };

  @override
  bool operator ==(Object other) =>
      other is WeKnoraAccount && other.toJson() == toJson();

  @override
  int get hashCode => Object.hashAll(toJson().values);
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `flutter test test/features/weknora/weknora_account_test.dart`
Expected: PASS（2 tests）

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/account/weknora_account.dart test/features/weknora/weknora_account_test.dart
git commit -m "feat(weknora): WeKnoraAccount credential record"
```

---

### Task 2: WeKnoraAuthClient（login/refresh/logout + JWT exp 解码）

**Files:**
- Create: `lib/features/weknora/account/weknora_auth_client.dart`
- Test: `test/features/weknora/weknora_auth_client_test.dart`

**Interfaces:**
- Consumes: 全局约束里的后端契约。
- Produces:
  - `class WeKnoraAuthException implements Exception { final String message; final bool invalidCredentials; final bool serverUnreachable; const WeKnoraAuthException(...); }`
  - `class WeKnoraTokenPair { final String accessToken; final String refreshToken; }`
  - `class WeKnoraLoginResult { final WeKnoraTokenPair tokens; final String userId; final String displayName; }`
  - `class WeKnoraAuthClient { WeKnoraAuthClient({required Dio Function() dioFactory}); Future<WeKnoraLoginResult> login({required String baseUrl, required String email, required String password}); Future<WeKnoraTokenPair> refresh({required String baseUrl, required String refreshToken}); Future<void> logout({required String baseUrl, required String refreshToken}); }`
  - `DateTime? weknoraJwtExpiry(String token)`（供 Task 3 判过期）

- [ ] **Step 1: 写失败测试**（沿用 `weknora_adapter_test.dart` 的 `_QueuedAdapter/_Reply/_dio` 模式；把该文件底部这几个私有类复制进本测试文件并保留同样实现）

```dart
// test/features/weknora/weknora_auth_client_test.dart
// _QueuedAdapter/_Reply/_dio 照抄 test/features/direct_connections/weknora_adapter_test.dart
void main() {
  group('login', () {
    test('returns tokens and user info', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'token': 'access-1',
          'refresh_token': 'refresh-1',
          'user': {'id': 'u1', 'username': 'alice', 'email': 'a@b.c'},
        }),
      ]);
      final client = WeKnoraAuthClient(dioFactory: (_) => _dio(http));
      final result = await client.login(
        baseUrl: 'http://localhost:8084', email: 'a@b.c', password: 'pw123',
      );
      expect(result.tokens.accessToken, 'access-1');
      expect(result.tokens.refreshToken, 'refresh-1');
      expect(result.displayName, 'alice');
      expect(http.requests.single.path, 'api/v1/auth/login');
      expect(
        (http.requests.single.data as Map<String, dynamic>)['email'],
        'a@b.c',
      );
    });

    test('maps 401 to invalidCredentials', () async {
      final http = _QueuedAdapter([
        _Reply.json({'success': false, 'message': 'bad'}, statusCode: 401),
      ]);
      final client = WeKnoraAuthClient(dioFactory: (_) => _dio(http));
      await expectLater(
        client.login(baseUrl: 'http://x', email: 'a@b.c', password: 'pw'),
        throwsA(isA<WeKnoraAuthException>()
            .having((e) => e.invalidCredentials, 'invalidCredentials', isTrue)),
      );
    });

    test('maps connection error to serverUnreachable', () async {
      final throwing = _QueuedAdapter([]);
      // 让 dio 抛 DioException.connectionError：构造一个总是失败的 HttpClientAdapter
      // —— 在 _QueuedAdapter 为空时按其实现抛 "No reply queued"（非 Dio 路径），
      // 因此这里直接测 _wrap 的映射函数：
      final client = WeKnoraAuthClient(dioFactory: (_) => _dio(throwing));
      try {
        await client.login(baseUrl: 'http://x', email: 'a', password: '123456');
        fail('unreachable');
      } on WeKnoraAuthException catch (e) {
        expect(e.serverUnreachable || e.invalidCredentials, isTrue);
      }
    });
  });

  group('refresh', () {
    test('rotates both tokens', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'access_token': 'access-2',
          'refresh_token': 'refresh-2',
        }),
      ]);
      final client = WeKnoraAuthClient(dioFactory: (_) => _dio(http));
      final pair = await client.refresh(
        baseUrl: 'http://x', refreshToken: 'refresh-1',
      );
      expect(pair.accessToken, 'access-2');
      expect(pair.refreshToken, 'refresh-2');
    });
  });

  group('weknoraJwtExpiry', () {
    test('decodes exp claim', () {
      // payload {"exp":2000000000} 的 base64url
      final token =
          'h.${base64Url.encode(utf8.encode(jsonEncode({'exp': 2000000000}))).replaceAll('=', '')}.s';
      expect(weknoraJwtExpiry(token)!.year, 2033);
    });

    test('returns null for malformed token', () {
      expect(weknoraJwtExpiry('not-a-jwt'), isNull);
    });
  });
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/weknora_auth_client_test.dart`
Expected: FAIL（库不存在）

- [ ] **Step 3: 最小实现**

```dart
import 'dart:convert';

import 'package:dio/dio.dart';

class WeKnoraAuthException implements Exception {
  const WeKnoraAuthException(
    this.message, {
    this.invalidCredentials = false,
    this.serverUnreachable = false,
  });

  final String message;
  final bool invalidCredentials;
  final bool serverUnreachable;

  @override
  String toString() => 'WeKnoraAuthException: $message';
}

class WeKnoraTokenPair {
  const WeKnoraTokenPair({required this.accessToken, required this.refreshToken});
  final String accessToken;
  final String refreshToken;
}

class WeKnoraLoginResult {
  const WeKnoraLoginResult({
    required this.tokens,
    required this.userId,
    required this.displayName,
  });
  final WeKnoraTokenPair tokens;
  final String userId;
  final String displayName;
}

/// Decodes the `exp` claim of a JWT without verifying it (verification is
/// the server's job); returns null when the token is not parseable.
DateTime? weknoraJwtExpiry(String token) {
  final parts = token.split('.');
  if (parts.length != 3) return null;
  try {
    final normalized = base64Url.normalize(parts[1]);
    final payload = jsonDecode(utf8.decode(base64Url.decode(normalized)));
    if (payload is! Map<String, dynamic>) return null;
    final exp = payload['exp'];
    if (exp is! num) return null;
    return DateTime.fromMillisecondsSinceEpoch(exp.toInt() * 1000, isUtc: true);
  } catch (_) {
    return null;
  }
}

class WeKnoraAuthClient {
  WeKnoraAuthClient({required Dio Function() dioFactory})
    : _dioFactory = dioFactory;

  final Dio Function() _dioFactory;

  Future<WeKnoraLoginResult> login({
    required String baseUrl,
    required String email,
    required String password,
  }) async {
    final data = await _post(
      baseUrl: baseUrl,
      path: 'api/v1/auth/login',
      body: {'email': email, 'password': password},
    );
    final user = (data['user'] as Map<String, dynamic>?) ?? const {};
    return WeKnoraLoginResult(
      tokens: WeKnoraTokenPair(
        accessToken: data['token'] as String? ?? '',
        refreshToken: data['refresh_token'] as String? ?? '',
      ),
      userId: user['id'] as String? ?? '',
      displayName: user['username'] as String? ?? user['email'] as String? ?? '',
    );
  }

  Future<WeKnoraTokenPair> refresh({
    required String baseUrl,
    required String refreshToken,
  }) async {
    final data = await _post(
      baseUrl: baseUrl,
      path: 'api/v1/auth/refresh',
      body: {'refreshToken': refreshToken},
    );
    return WeKnoraTokenPair(
      accessToken: data['access_token'] as String? ?? '',
      refreshToken: data['refresh_token'] as String? ?? '',
    );
  }

  Future<void> logout({required String baseUrl, required String refreshToken}) async {
    try {
      await _post(
        baseUrl: baseUrl,
        path: 'api/v1/auth/logout',
        body: {'refreshToken': refreshToken},
      );
    } catch (_) {
      // Best-effort revocation; local credentials are cleared regardless.
    }
  }

  Future<Map<String, dynamic>> _post({
    required String baseUrl,
    required String path,
    required Map<String, dynamic> body,
  }) async {
    try {
      final response = await _dioFactory().post<dynamic>(
        path,
        data: body,
        options: Options(
          validateStatus: (status) => status != null && status < 500,
        ),
      );
      final data = response.data;
      final map = data is String
          ? jsonDecode(data) as Map<String, dynamic>
          : (data as Map?)?.cast<String, dynamic>() ?? const <String, dynamic>{};
      final success = map['success'] == true;
      if (response.statusCode == 401 || !success) {
        throw WeKnoraAuthException(
          map['message'] as String? ?? 'Login failed.',
          invalidCredentials: true,
        );
      }
      return map;
    } on WeKnoraAuthException {
      rethrow;
    } on DioException catch (error) {
      throw WeKnoraAuthException(
        'Could not reach the WeKnora server.',
        serverUnreachable: true,
      ); // error intentionally not surfaced: it may carry the password body.
    }
  }
}
```

注：`_dio` mock 的 baseUrl 需指向 `http://localhost:8084` 风格的绝对前缀——照抄 adapter 测试里的 `_dio` 构造（它已把 baseUrl 设为 `http://localhost:8084/` 并相对 path 请求）。

- [ ] **Step 4: 跑测试确认通过**

Run: `flutter test test/features/weknora/weknora_auth_client_test.dart`
Expected: PASS（6 tests）

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/account/weknora_auth_client.dart test/features/weknora/weknora_auth_client_test.dart
git commit -m "feat(weknora): auth client for login/refresh/logout with JWT expiry decode"
```

---

### Task 3: WeKnoraAccountService（登录态、单飞刷新、profile 镜像、登出）

**Files:**
- Create: `lib/features/weknora/account/weknora_account_service.dart`
- Create: `lib/features/weknora/account/weknora_providers.dart`
- Test: `test/features/weknora/weknora_account_service_test.dart`

**Interfaces:**
- Consumes: Task 1 `WeKnoraAccount`、Task 2 `WeKnoraAuthClient/WeKnoraTokenPair/WeKnoraAuthException/weknoraJwtExpiry`；`DirectConnectionProfile`、`kWeKnoraAdapterKey`（`lib/features/direct_connections/models/direct_connection_profile.dart`）。
- Produces:
  - `class WeKnoraAccountService`：
    - `Future<WeKnoraAccount> login({required String baseUrl, required String email, required String password})`
    - `Future<String> accessToken()`（单飞刷新 + 镜像；失败清态并抛 `WeKnoraAuthException(invalidCredentials: true)`）
    - `Future<void> logout()`
    - `WeKnoraAccount? get currentAccount`
    - `void addAuthExpiredListener(void Function() listener)`
  - `final weknoraAccountServiceProvider = Provider<WeKnoraAccountService>(...)`（providers 文件绑定 SecureCredentialStorage 的读写：key `weknora_account_v1`，用 `SecureCredentialStorage(instance: ref.watch(secureStorageProvider))` 的既有 generic 读写；若该类无 generic 方法，按 `directConnectionProfiles` 同款模式加 `readWeKnoraAccount/writeWeKnoraAccount/deleteWeKnoraAccount` 三个薄方法——签名照抄同文件 `deleteDirectConnectionProfiles` 邻近的读写方法）

- [ ] **Step 1: 写失败测试**（全部用注入函数桩，不碰插件）

```dart
// 构造服务的 helper：
WeKnoraAccountService makeService({
  required _FakeAuth auth,          // 手写 fake：记录调用、可编程返回
  Map<String, WeKnoraAccount> stored = const {},
  List<DirectConnectionProfile> profiles = const [],
}) {
  final storage = _AccountStorageFake(stored);   // 内存实现 load/persist
  final profileSink = _ProfileSinkFake(profiles); // 内存实现 list/upsert
  return WeKnoraAccountService(
    authClient: auth,
    loadAccount: storage.load,
    persistAccount: storage.persist,
    listProfiles: profileSink.list,
    upsertProfile: profileSink.upsert,
  );
}

test('login persists account and mirrors a bearer profile', ...);
// 1) auth.login 返回 tokens → login() 后 currentAccount.email 正确
// 2) profileSink 里出现 adapterKey==kWeKnoraAdapterKey、apiKeyAuthMode==bearer、
//    apiKey==accessToken 的 profile（新建场景）

test('login reuses and updates the existing weknora profile for same baseUrl', ...);
// 预置一条同 baseUrl 的 weknora profile(id 'p1') → 登录后仍是一条、id 不变、apiKey 已换

test('accessToken refreshes single-flight and mirrors new token into profile', ...);
// 预置 access token exp=过去时间 → 并发 await Future.wait([svc.accessToken(), svc.accessToken()])
// → auth.refresh 调用次数==1；存储与 profile 的 accessToken/apiKey 都是新值

test('accessToken returns current token when far from expiry', ...);
// exp=now+1h → 不调用 refresh

test('refresh failure clears account and fires auth-expired listener', ...);
// auth.refresh 抛 WeKnoraAuthException → expectLater(accessToken(), throwsA(...));
// currentAccount==null；listener 计数==1

test('logout clears account and disables profile', ...);
// 登录后 logout → 存储空、profile.enabled==false、auth.logout 被调用一次
```

（fake 类完整写出：`_FakeAuth implements` 调用记录 + 编程返回；`_AccountStorageFake`；`_ProfileSinkFake`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/weknora_account_service_test.dart`
Expected: FAIL

- [ ] **Step 3: 最小实现**（服务核心；60 秒提前量常量 `_expirySlack = Duration(seconds: 60)`）

```dart
class WeKnoraAccountService {
  WeKnoraAccountService({
    required WeKnoraAuthClient authClient,
    required Future<WeKnoraAccount?> Function() loadAccount,
    required Future<void> Function(WeKnoraAccount? account) persistAccount,
    required Future<List<DirectConnectionProfile>> Function() listProfiles,
    required Future<void> Function(DirectConnectionProfile profile) upsertProfile,
  }) : _authClient = authClient, ... ;

  WeKnoraAccount? _account;
  Future<WeKnoraTokenPair?>? _refreshInFlight;
  final List<void Function()> _authExpiredListeners = [];

  WeKnoraAccount? get currentAccount => _account;
  void addAuthExpiredListener(void Function() listener) =>
      _authExpiredListeners.add(listener);

  Future<WeKnoraAccount?> _restore() async =>
      _account ??= await _loadAccount();

  Future<WeKnoraAccount> login({
    required String baseUrl,
    required String email,
    required String password,
  }) async {
    final result = await _authClient.login(
      baseUrl: baseUrl, email: email, password: password,
    );
    final account = WeKnoraAccount(
      baseUrl: baseUrl, email: email,
      userId: result.userId, displayName: result.displayName,
      accessToken: result.tokens.accessToken,
      refreshToken: result.tokens.refreshToken,
    );
    _account = account;
    await _persistAccount(account);
    await _mirrorIntoProfile(account);
    return account;
  }

  /// Returns a non-expired access token, refreshing (single-flight) when
  /// needed. Throws [WeKnoraAuthException] when the refresh token is dead;
  /// account state is cleared and auth-expired listeners fire first.
  Future<String> accessToken() async {
    final account = await _restore();
    if (account == null) {
      throw const WeKnoraAuthException('Not signed in to WeKnora.',
          invalidCredentials: true);
    }
    final expiry = weknoraJwtExpiry(account.accessToken);
    final fresh = expiry != null &&
        expiry.isAfter(DateTime.now().toUtc().add(_expirySlack));
    if (fresh) return account.accessToken;
    final pair = await (_refreshInFlight ??= _refresh(account));
    return pair.accessToken;
  }

  Future<WeKnoraTokenPair> _refresh(WeKnoraAccount account) async {
    try {
      final pair = await _authClient.refresh(
        baseUrl: account.baseUrl, refreshToken: account.refreshToken,
      );
      final next = account.copyWith(
        accessToken: pair.accessToken, refreshToken: pair.refreshToken,
      );
      _account = next;
      await _persistAccount(next);
      await _mirrorIntoProfile(next);
      return pair;
    } on WeKnoraAuthException {
      _account = null;
      await _persistAccount(null);
      for (final listener in List.of(_authExpiredListeners)) {
        listener();
      }
      rethrow;
    } finally {
      _refreshInFlight = null;
    }
  }

  Future<void> _mirrorIntoProfile(WeKnoraAccount account) async {
    final profiles = await _listProfiles();
    final origin = DirectConnectionProfile.originOf(account.baseUrl);
    if (origin == null) return;
    final existing = profiles.firstWhereOrNull(... /* adapterKey==kWeKnoraAdapterKey && originOf(baseUrl)==origin */);
    if ( existing != null ) {
      await _upsertProfile(existing.copyWith(
        apiKey: account.accessToken, enabled: true,
      ));
    } else {
      await _upsertProfile(DirectConnectionProfile(
        id: const Uuid().v4(),
        name: 'WeKnora',
        adapterKey: kWeKnoraAdapterKey,
        baseUrl: origin,
        apiKeyAuthMode: DirectApiKeyAuthMode.bearer,
        apiKey: account.accessToken,
      ));
    }
  }

  Future<void> logout() async {
    final account = await _restore();
    if (account != null) {
      await _authClient.logout(
        baseUrl: account.baseUrl, refreshToken: account.refreshToken,
      );
      final profiles = await _listProfiles();
      final origin = DirectConnectionProfile.originOf(account.baseUrl);
      final mine = profiles.where((p) =>
          p.adapterKey == kWeKnoraAdapterKey &&
          (origin == null || p.baseUrl == origin));
      for (final profile in mine) {
        await _upsertProfile(profile.copyWith(enabled: false));
      }
    }
    _account = null;
    await _persistAccount(null);
  }
}
```

实现注意（按仓内实际 API 校对，签名已核对过：`DirectConnectionProfile` 构造必填 id/name/adapterKey/baseUrl，其余可选；`copyWith` 存在于同文件）：`firstWhereOrNull` 来自 `package:collection`（仓内已用）。

`weknora_providers.dart`：

```dart
final weknoraAccountServiceProvider = Provider<WeKnoraAccountService>((ref) {
  final storage = SecureCredentialStorage(instance: ref.watch(secureStorageProvider));
  return WeKnoraAccountService(
    authClient: WeKnoraAuthClient(
      dioFactory: () => Dio(BaseOptions(baseUrl: 'unused')), // 见下
    ),
    loadAccount: () async {
      final raw = await storage.readRaw('weknora_account_v1');
      return raw == null ? null : WeKnoraAccount.fromJson(jsonDecode(raw));
    },
    persistAccount: (account) => account == null
        ? storage.deleteRaw('weknora_account_v1')
        : storage.writeRaw('weknora_account_v1', jsonEncode(account.toJson())),
    listProfiles: () => ref.read(directConnectionProfileStoreProvider).load(),
    upsertProfile: (profile) =>
        ref.read(directConnectionProfilesProvider.notifier).upsert(profile),
  );
});
```

若 `SecureCredentialStorage` 没有 `readRaw/writeRaw/deleteRaw`：在该类中按 `directConnectionProfiles` 键的现有读写方法的**同一实现模式**新增 `_weknoraAccountKey = 'weknora_account_v1'` 三个方法（read→String? / write→void / delete→void），providers 改调它们。dioFactory 在 providers 层先给占位 Dio（登录页与 Task 5 的 API 层不经过它——login 用页面侧构造、其余请求走 Direct pool），若 analyze 报 unused 再收敛为 `throw UnimplementedError` 之外的惰性工厂；**服务本身只依赖 authClient 抽象**。

- [ ] **Step 4: 跑测试确认通过**

Run: `flutter test test/features/weknora/weknora_account_service_test.dart`
Expected: PASS（≥6 tests）

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/account/ test/features/weknora/weknora_account_service_test.dart lib/core/services/secure_credential_storage.dart
git commit -m "feat(weknora): account service with single-flight refresh and profile mirroring"
```

---

### Task 4: 适配器精确会话绑定 + boundSessionIdFor

**Files:**
- Modify: `lib/features/direct_connections/services/weknora_adapter.dart`（绑定分支约 :440-490；新增公开方法挨着 `_matchBinding` :864）
- Test: `test/features/direct_connections/weknora_adapter_test.dart`（追加用例）

**Interfaces:**
- Consumes: 现有 `_matchBinding/_registerBinding/_WeKnoraModelRef`。
- Produces: `String? boundSessionIdFor({required String profileId, required String remoteModelId, required List<String> priorUserQueries})`；请求参数约定 `parameters['weknora_session_id']`（Task 7 注入）。

- [ ] **Step 1: 追加失败测试**

```dart
group('WeKnoraAdapter explicit session binding', () {
  test('uses parameters[weknora_session_id] without creating a session', () async {
    final http = _QueuedAdapter([
      // 第一条请求就应是 agent-chat（或 knowledge-chat）且路径含既有 sid，
      // 而不是 POST api/v1/sessions
      _Reply.stream(<String>['event: message', 'data: {"response_type":"answer","content":"hi"}', '', 'event: message', 'data: {"response_type":"complete"}', '', '']),
    ]);
    final adapter = WeKnoraAdapter(dioFactory: (_) => _dio(http));
    final run = adapter.startCompletion(
      _weknoraProfile(agentModel: 'agent:ag1'),
      _completionRequest(
        parameters: {'weknora_session_id': 'sess-existing'},
      ),
    );
    await run.done;
    expect(http.requests.single.path, contains('sess-existing'));
    expect(http.requests.single.path, contains('agent-chat'));
    expect(adapter.boundSessionIdFor(
      profileId: _weknoraProfile().id,
      remoteModelId: 'agent:ag1',
      priorUserQueries: const ['hello'],
    ), 'sess-existing');
  });

  test('boundSessionIdFor returns null on mismatch', () {
    final adapter = WeKnoraAdapter();
    expect(adapter.boundSessionIdFor(
      profileId: 'p', remoteModelId: 'agent:ag1', priorUserQueries: const ['x'],
    ), isNull);
  });
});
```

（`_weknoraProfile/_completionRequest` 辅助按既有测试文件的构造方式补齐参数；`agentModel` 若原 helper 不支持就直接内联构造 profile。）

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/direct_connections/weknora_adapter_test.dart`
Expected: FAIL（无该分支；第一请求打去了 `api/v1/sessions`）

- [ ] **Step 3: 最小实现**

在 `startCompletion` 内，把：

```dart
binding = _matchBinding(profile.id, model, priorQueries);
var queryToSend = lastQuery;
if (binding == null) {
  final sessionId = await _createSession(...);
  ...
} else {
  weknoraSessionId = binding.sessionId;
}
```

改为（保留其余原样）：

```dart
final explicitSessionId = request.parameters['weknora_session_id'];
binding = _matchBinding(profile.id, model, priorQueries);
var queryToSend = lastQuery;
if (explicitSessionId is String && explicitSessionId.isNotEmpty) {
  // Login-driven conversations know their server session; exact binding
  // replaces prefix guessing and never creates a replacement session.
  weknoraSessionId = explicitSessionId;
  binding = _registerBinding(profile.id, model, explicitSessionId, priorQueries);
} else if (binding == null) {
  final sessionId = await _createSession(dio, profile, transportCancelToken);
  weknoraSessionId = sessionId;
  binding = _registerBinding(profile.id, model, sessionId, priorQueries);
  final contextPrefix = _contextPrefix(messages);
  if (contextPrefix != null) {
    queryToSend = '$contextPrefix\n\n---\n\n$queryToSend';
    budget.addCharacters(queryToSend.length);
  }
} else {
  weknoraSessionId = binding.sessionId;
}
```

并在 `_matchBinding` 旁新增公开方法：

```dart
/// Reads back the server session a conversation was bound to, keyed by the
/// same prior-user-query sequence the binding table matches on. The login
/// flow uses this to persist `weknoraSessionId` onto the conversation after
/// a locally started turn.
String? boundSessionIdFor({
  required String profileId,
  required String remoteModelId,
  required List<String> priorUserQueries,
}) {
  final model = _WeKnoraModelRef.parse(remoteModelId);
  return _matchBinding(profileId, model, priorUserQueries)?.sessionId;
}
```

- [ ] **Step 4: 跑全部既有+新测试确认通过**

Run: `flutter test test/features/direct_connections/weknora_adapter_test.dart`
Expected: PASS（15 既有 + 2 新增全绿）

- [ ] **Step 5: Commit**

```bash
git add lib/features/direct_connections/services/weknora_adapter.dart test/features/direct_connections/weknora_adapter_test.dart
git commit -m "feat(weknora): exact session binding parameter and boundSessionIdFor lookup"
```

---

### Task 5: 会话 API + 纯映射器

**Files:**
- Create: `lib/features/weknora/sessions/weknora_session_api.dart`
- Create: `lib/features/weknora/sessions/weknora_mappers.dart`
- Test: `test/features/weknora/weknora_session_api_test.dart`、`test/features/weknora/weknora_mappers_test.dart`

**Interfaces:**
- Consumes: Task 3 `WeKnoraAccountService.accessToken()`（API 层调用它取新鲜 token 并自带 401→刷新语义）。
- Produces:
  - `class WeKnoraSessionSummary { final String id; final String title; final DateTime updatedAt; final DateTime createdAt; final bool pinned; }`
  - `class WeKnoraHistoryMessage { final String role; final String content; final List<String> sourceLines; }`（references 已折叠为 markdown 行）
  - `class WeKnoraSessionApi { WeKnoraSessionApi({required Future<({String baseUrl, String token})> Function() connection, required Dio Function() dioFactory}); Future<List<WeKnoraSessionSummary>> listSessions({int limit = 50}); Future<List<WeKnoraHistoryMessage>> loadMessages({required String sessionId, int limit = 200}); }`（会话不存在抛 `WeKnoraSessionNotFoundException`）
  - `Conversation weknoraSessionToConversation(WeKnoraSessionSummary session)`
  - `List<ChatMessage> weknoraHistoryToChatMessages(String sessionId, List<WeKnoraHistoryMessage> messages)`

- [ ] **Step 1: 写失败测试**

```dart
// weknora_mappers_test.dart
test('maps a session summary to a conversation with metadata binding', () {
  final conversation = weknoraSessionToConversation(WeKnoraSessionSummary(
    id: 's1', title: 'RAG 问答', updatedAt: DateTime.utc(2026, 9, 19),
    createdAt: DateTime.utc(2026, 9, 18), pinned: true,
  ));
  expect(conversation.metadata['weknoraSessionId'], 's1');
  expect(conversation.id, 'weknora-s1');
  expect(conversation.title, 'RAG 问答');
  expect(conversation.pinned, isTrue);
});

test('maps history into user/assistant chat messages with source block', () {
  final messages = weknoraHistoryToChatMessages('s1', [
    WeKnoraHistoryMessage(role: 'user', content: '什么是 RAG?', sourceLines: const []),
    WeKnoraHistoryMessage(role: 'assistant', content: 'RAG 是检索增强生成。',
        sourceLines: const ['1. [Handbook](https://a/b) — 摘要片段']),
  ]);
  expect(messages.first.role, 'user');
  expect(messages.last.role, 'assistant');
  expect(messages.last.content, contains('Sources'));
  expect(messages.last.content, contains('https://a/b'));
});

// weknora_session_api_test.dart（_QueuedAdapter 模式）
test('listSessions parses data array and sends bearer token', ...);
// 断言 path=='api/v1/sessions'、query 含 limit=50、headers['Authorization']=='Bearer at-1'
test('loadMessages maps message fields and 404 throws WeKnoraSessionNotFoundException', ...);
// 第二个用例队列回 _Reply.json({'error':'not found'}, statusCode: 404)
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/`
Expected: FAIL

- [ ] **Step 3: 最小实现**

```dart
// weknora_session_api.dart
class WeKnoraSessionNotFoundException implements Exception {
  const WeKnoraSessionNotFoundException(this.sessionId);
  final String sessionId;
}

class WeKnoraSessionApi {
  WeKnoraSessionApi({required Future<({String baseUrl, String token})> Function() connection,
      required Dio Function() dioFactory});

  Future<List<WeKnoraSessionSummary>> listSessions({int limit = 50}) async {
    final c = await _connection();
    final response = await _dioFactory().get<List<dynamic>>(
      'api/v1/sessions',
      queryParameters: {'limit': limit},
      options: Options(headers: {'Authorization': 'Bearer ${c.token}'}),
    );
    final data = response.data ?? const <dynamic>[];
    return data.map((raw) {
      final map = (raw as Map).cast<String, dynamic>();
      final last = (map['last_request_state'] as Map?)?.cast<String, dynamic>();
      return WeKnoraSessionSummary(
        id: map['id'] as String,
        title: map['title'] as String? ?? '',
        updatedAt: DateTime.tryParse(map['updated_at'] as String? ?? '') ?? DateTime.now(),
        createdAt: DateTime.tryParse(map['created_at'] as String? ?? '') ?? DateTime.now(),
        pinned: map['is_pinned'] == true,
      );
    }).toList();
  }

  Future<List<WeKnoraHistoryMessage>> loadMessages({required String sessionId, int limit = 200}) async {
    // GET api/v1/messages/$sessionId/load?limit=... 同款 bearer 头；
    // 404 → throw WeKnoraSessionNotFoundException(sessionId)；
    // data 每条: role/content/knowledge_references[{source_title,knowledge_snippet,source_url}]
    // → WeKnoraHistoryMessage(role, content, sourceLines: [
    //    '$i. [${title}](${url}) — $snippet' 仅当 url 以 http 开头，否则 '$i. $title — $snippet'])
  }
}
```

```dart
// weknora_mappers.dart
Conversation weknoraSessionToConversation(WeKnoraSessionSummary session) =>
    Conversation(
      id: 'weknora-${session.id}',
      title: session.title.isEmpty ? 'WeKnora' : session.title,
      createdAt: session.createdAt,
      updatedAt: session.updatedAt,
      pinned: session.pinned,
      metadata: {'weknoraSessionId': session.id},
    );

List<ChatMessage> weknoraHistoryToChatMessages(
  String sessionId,
  List<WeKnoraHistoryMessage> messages,
) {
  final chatMessages = <ChatMessage>[];
  for (final (index, message) in messages.indexed) {
    var content = message.content;
    if (message.role == 'assistant' && message.sourceLines.isNotEmpty) {
      content = '$content\n\n**Sources**\n${message.sourceLines.join('\n')}';
    }
    chatMessages.add(ChatMessage(
      id: 'weknora-$sessionId-$index',
      role: message.role,
      content: content,
      timestamp: DateTime.now(),
    ));
  }
  return chatMessages;
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `flutter test test/features/weknora/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/sessions/ test/features/weknora/
git commit -m "feat(weknora): session list/message api with conversation mappers"
```

---

### Task 6: WeKnoraSessionSync（列表 upsert、消息注入、404 剔除、回填）

**Files:**
- Create: `lib/features/weknora/sessions/weknora_session_sync.dart`
- Test: `test/features/weknora/weknora_session_sync_test.dart`

**Interfaces:**
- Consumes: Task 5 全部产物；`Conversations` notifier（`lib/core/providers/app_providers.dart:3697` `void upsertConversation(Conversation, {bool trustFolderConversation})`、`:3741` `void updateConversation(String id, Conversation Function(Conversation))`、`:3647` `void removeConversation(String id)`——签名已核对）。
- Produces: `class WeKnoraSessionSync { WeKnoraSessionSync({required WeKnoraSessionApi api, required void Function(Conversation) upsertConversation, required void Function(String id, Conversation Function(Conversation)) updateConversation, required void Function(String id) removeConversation}); Future<void> refreshSessions(); Future<void> hydrateMessages({required Conversation conversation}); Future<void> stampBoundSessionId({required String conversationId, required String profileId, required String remoteModelId, required List<String> priorUserQueries, required WeKnoraAdapter adapter}); }`

- [ ] **Step 1: 写失败测试**（fake 全部注入函数为内存列表操作）

```dart
test('refreshSessions upserts server sessions without touching local-only ones', ...);
// 本地已有 ['weknora-s1'(旧标题), 'local-1'(无 metadata)] → 服务端返回 s1(新标题)+s2
// → 断言：weknora-s1 标题更新、weknora-s2 新增、local-1 保持且 messages 未被清空

test('hydrateMessages replaces messages of the target conversation only', ...);
// api.loadMessages 返回 3 条 → updateConversation 被调用，transform 结果 messages.length==3
// 且 metadata.weknoraSessionId 不变

test('hydrateMessages evicts the conversation on 404', ...);
// api.loadMessages 抛 WeKnoraSessionNotFoundException → removeConversation('weknora-sX') 恰一次

test('stampBoundSessionId writes metadata when adapter knows the binding', ...);
// adapter 为 fake（重写 boundSessionIdFor 返回 'sess-9'）→ updateConversation 后
// conversation.metadata['weknoraSessionId']=='sess-9'；adapter 返回 null 时不调用 update
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/weknora_session_sync_test.dart`
Expected: FAIL

- [ ] **Step 3: 最小实现**

```dart
class WeKnoraSessionSync {
  WeKnoraSessionSync({
    required WeKnoraSessionApi api,
    required void Function(Conversation conversation) upsertConversation,
    required void Function(String id, Conversation Function(Conversation) transform)
        updateConversation,
    required void Function(String id) removeConversation,
    required List<Conversation> Function() currentConversations,
  }) : ...;

  /// Pulls the first page of server sessions and upserts them. Local-only
  /// conversations (no weknoraSessionId metadata) are never touched.
  Future<void> refreshSessions() async {
    final sessions = await _api.listSessions();
    for (final session in sessions) {
      _upsertConversation(weknoraSessionToConversation(session));
    }
  }

  /// Loads server history into an existing conversation. A 404 (session
  /// deleted on the web) removes the conversation from the list.
  Future<void> hydrateMessages({required Conversation conversation}) async {
    final sessionId = conversation.metadata['weknoraSessionId'];
    if (sessionId is! String || sessionId.isEmpty) return;
    try {
      final history = await _api.loadMessages(sessionId: sessionId);
      final messages = weknoraHistoryToChatMessages(sessionId, history);
      _updateConversation(conversation.id, (current) =>
          current.copyWith(messages: messages, updatedAt: DateTime.now()));
    } on WeKnoraSessionNotFoundException {
      _removeConversation(conversation.id);
    }
  }

  /// After a locally started turn completes, persist the server session the
  /// adapter bound this conversation to.
  Future<void> stampBoundSessionId({
    required String conversationId,
    required String profileId,
    required String remoteModelId,
    required List<String> priorUserQueries,
    required WeKnoraAdapter adapter,
  }) async {
    final sessionId = adapter.boundSessionIdFor(
      profileId: profileId,
      remoteModelId: remoteModelId,
      priorUserQueries: priorUserQueries,
    );
    if (sessionId == null) return;
    _updateConversation(conversationId,
        (current) => current.copyWith(metadata: {
          ...current.metadata,
          'weknoraSessionId': sessionId,
        }));
  }
}
```

（`metadata` 合并时保留既有键；`copyWith` 对 metadata 是整表替换，务必展开合并。）

- [ ] **Step 4: 跑测试确认通过**

Run: `flutter test test/features/weknora/weknora_session_sync_test.dart`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/sessions/weknora_session_sync.dart test/features/weknora/weknora_session_sync_test.dart
git commit -m "feat(weknora): server session sync (upsert/hydrate/evict/stamp)"
```

---

### Task 7: 发送管线注入 weknora_session_id + 流结束回填

**Files:**
- Create: `lib/features/chat/services/weknora_chat_parameters.dart`
- Modify: `lib/features/chat/providers/chat_providers.dart`（主发送点约 :16418-16437 的 `parameters:` 表达式；流结束处调用回填）
- Test: `test/features/chat/weknora_chat_parameters_test.dart`

**Interfaces:**
- Consumes: Task 4 的参数名 `weknora_session_id`、Task 6 的 `stampBoundSessionId`。
- Produces: `Map<String, dynamic> weknoraChatParameters({required String adapterKey, required Map<String, dynamic> conversationMetadata, String? reasoningEffort})`

- [ ] **Step 1: 写失败测试**

```dart
test('injects weknora_session_id only for weknora profiles with binding', () {
  final params = weknoraChatParameters(
    adapterKey: kWeKnoraAdapterKey,
    conversationMetadata: {'weknoraSessionId': 'sess-1'},
    reasoningEffort: null,
  );
  expect(params['weknora_session_id'], 'sess-1');
});

test('keeps empty map for other adapters even with metadata', () {
  expect(weknoraChatParameters(
    adapterKey: 'ollama',
    conversationMetadata: {'weknoraSessionId': 'sess-1'},
    reasoningEffort: null,
  ), isEmpty);
});

test('preserves reasoning_effort alongside the session id', () {
  final params = weknoraChatParameters(
    adapterKey: kWeKnoraAdapterKey,
    conversationMetadata: {'weknoraSessionId': 'sess-1'},
    reasoningEffort: 'high',
  );
  expect(params['reasoning_effort'], 'high');
  expect(params['weknora_session_id'], 'sess-1');
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/chat/weknora_chat_parameters_test.dart`
Expected: FAIL

- [ ] **Step 3: 最小实现 + 接线**

```dart
// lib/features/chat/services/weknora_chat_parameters.dart
/// Extra Direct completion parameters for WeKnora conversations that already
/// know their server session. The parameter is WeKnora-only: other adapters
/// must never see it leak into their request bodies.
Map<String, dynamic> weknoraChatParameters({
  required String adapterKey,
  required Map<String, dynamic> conversationMetadata,
  String? reasoningEffort,
}) {
  final parameters = <String, dynamic>{};
  if (adapterKey != kOllamaAdapterKey && reasoningEffort != null) {
    parameters['reasoning_effort'] = reasoningEffort;
  }
  if (adapterKey == kWeKnoraAdapterKey) {
    final sessionId = conversationMetadata['weknoraSessionId'];
    if (sessionId is String && sessionId.isNotEmpty) {
      parameters['weknora_session_id'] = sessionId;
    }
  }
  return parameters;
}
```

`chat_providers.dart` 主发送点（:16434 附近）把：

```dart
parameters:
    route.profile.adapterKey == kOllamaAdapterKey ||
        reasoningEffort == null
    ? const <String, dynamic>{}
    : <String, dynamic>{'reasoning_effort': reasoningEffort},
```

替换为：

```dart
parameters: weknoraChatParameters(
  adapterKey: route.profile.adapterKey,
  conversationMetadata: conversation.metadata,
  reasoningEffort: reasoningEffort,
),
```

（`conversation` 变量名以该作用域实际为准——发送函数内已有当前会话引用；若名为其他（如 `activeConversation`），用它。）

流结束回填：在同一发送函数中 `run.done` 完成回调（现有 `await run.done` 或其 `.then` 链）之后追加：

```dart
if (route.profile.adapterKey == kWeKnoraAdapterKey &&
    conversation.metadata['weknoraSessionId'] == null) {
  final priorUserQueries = directMessages
      .where((m) => m.role == 'user')
      .map((m) => m.parts.whereType<DirectTextPart>().map((p) => p.text).join('\n'))
      .toList();
  await ref.read(weknoraSessionSyncProvider.notifier /* 或等价访问点 */)
      .stampBoundSessionId(
        conversationId: conversation.id,
        profileId: route.profile.id,
        remoteModelId: route.binding.remoteModelId,
        priorUserQueries: priorUserQueries,
        adapter: adapter,
      );
}
```

`weknoraSessionSyncProvider` 在 Task 6 的 providers 绑定（本任务在 `weknora_providers.dart` 里补：用 `WeKnoraSessionApi(connection: () async { final account = await accountService.restore(); final token = await accountService.accessToken(); return (baseUrl: account!.baseUrl, token: token); }, dioFactory: ...)` + `ref.read(conversationsProvider.notifier)` 的三个方法构造 sync 实例并缓存）。若发送函数处无法直接拿到 riverpod `ref`，用该文件既有的服务定位模式（同文件里其他 service 的获取方式照抄）。

- [ ] **Step 4: 跑测试确认通过 + 既有聊天测试回归**

Run: `flutter test test/features/chat/weknora_chat_parameters_test.dart && flutter test test/features/direct_connections/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add lib/features/chat/services/weknora_chat_parameters.dart lib/features/chat/providers/chat_providers.dart lib/features/weknora/account/weknora_providers.dart test/features/chat/weknora_chat_parameters_test.dart
git commit -m "feat(weknora): inject exact session id into direct chat sends and stamp after completion"
```

---

### Task 8: 登录页 + 路由 + 后端选择器 + 设置账户区

**Files:**
- Create: `lib/features/weknora/account/weknora_login_page.dart`
- Modify: `lib/core/router/app_router.dart`（仿 :504 backendChooser 路由样式）、`lib/features/auth/views/backend_chooser_page.dart`（仿 Apple/OpenWebUI 行）、`lib/features/profile/views/profile_page.dart`（账户区）
- Test: `test/features/weknora/weknora_login_page_test.dart`

**Interfaces:**
- Consumes: Task 3 `WeKnoraAccountService.login/logout`、`weknoraAccountServiceProvider`；router 常量文件里的 `Routes`/`RouteNames`（在 `lib/core/router/` 或其导入里，仿现有条目加 `weknoraLogin`）。
- Produces: 路由名 `RouteNames.weknoraLogin`，路径 `Routes.weknoraLogin = '/weknora-login'`；页面 `class WeKnoraLoginPage extends ConsumerStatefulWidget`。

- [ ] **Step 1: 写失败测试**（widget 冒烟 + 交互）

```dart
testWidgets('renders fields and submits login', (tester) async {
  final service = _FakeAccountService(); // 记录 login 调用并返回成功
  await tester.pumpWidget(ProviderScope(
    overrides: [weknoraAccountServiceProvider.overrideWithValue(service)],
    child: const MaterialApp(localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales, child: WeKnoraLoginPage()),
  ));
  await tester.enterText(find.byKey(const Key('weknora-server-field')), 'http://localhost:8084');
  await tester.enterText(find.byKey(const Key('weknora-email-field')), 'a@b.c');
  await tester.enterText(find.byKey(const Key('weknora-password-field')), 'pw123456');
  await tester.tap(find.byKey(const Key('weknora-login-button')));
  await tester.pumpAndSettle();
  expect(service.loginCalls.single.email, 'a@b.c');
});

testWidgets('shows inline error on WeKnoraAuthException', (tester) async {
  // service.login 抛 WeKnoraAuthException('bad', invalidCredentials: true)
  // → pumpAndSettle 后 find.text(l10n 登录失败文案) 命中且仍在登录页
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `flutter test test/features/weknora/weknora_login_page_test.dart`
Expected: FAIL

- [ ] **Step 3: 实现**

页面结构（复用 `lib/features/auth/views/connect_signin_page.dart` 的布局习惯：SafeArea + ListView + 圆角 TextField + 主按钮 + 内联错误行； Cupertino/Material 组件按该文件 import 风格）：

```dart
class WeKnoraLoginPage extends ConsumerStatefulWidget {
  const WeKnoraLoginPage({super.key});
  @override
  ConsumerState<WeKnoraLoginPage> createState() => _WeKnoraLoginPageState();
}

class _WeKnoraLoginPageState extends ConsumerState<WeKnoraLoginPage> {
  final _server = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();
  String? _error;
  bool _busy = false;

  Future<void> _submit() async {
    setState(() { _busy = true; _error = null; });
    try {
      await ref.read(weknoraAccountServiceProvider).login(
        baseUrl: _server.text.trim(),
        email: _email.text.trim(),
        password: _password.text,
      );
      if (mounted) context.go('/');          // 进入聊天主页
    } on WeKnoraAuthException catch (error) {
      setState(() => _error = error.invalidCredentials
          ? AppLocalizations.of(context)!.weknoraLoginInvalidCredentials
          : AppLocalizations.of(context)!.weknoraLoginUnreachable);
    } catch (_) {
      setState(() => _error = AppLocalizations.of(context)!.weknoraLoginUnreachable);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
  // build：三个 Key('weknora-server-field'/'weknora-email-field'/
  // 'weknora-password-field') 的 TextField + Key('weknora-login-button')
  // 的 FilledButton（busy 时禁用+进度指示），错误行显示 _error。
}
```

路由（app_router.dart，紧挨 backendChooser 路由之后）：

```dart
GoRoute(
  path: Routes.weknoraLogin,
  name: RouteNames.weknoraLogin,
  pageBuilder: (context, state) =>
      _buildPlatformPage(state: state, child: const WeKnoraLoginPage()),
),
```

并在 `Routes`/`RouteNames` 常量定义处（router 目录内）加：`static const weknoraLogin = '/weknora-login';` 与 `static const weknoraLogin = 'weknoraLogin';`（名字照所在类的既有风格）。

BackendChooserPage：在现有行列表（Apple 行构造处之后）加一行 `UtilitySelectionRow`（该页既有组件），标题用新 l10n key `weknoraBackendChooserTitle`（"WeKnora"/"WeKnora"），`onTap: () => context.go(Routes.weknoraLogin)`。

profile_page.dart 设置表：在 "Direct Connections" 行之前插入账户区（已登录时显示邮箱+服务器+「退出登录」按钮，调 `logout()` 后 `context.go(Routes.weknoraLogin)`；未登录且存在禁用的 WeKnora profile 时显示「登录 WeKnora」入口）。判断用 `ref.watch(weknoraAccountServiceProvider).currentAccount`（或该 service 暴露的 StateProvider——若需要响应式，在 weknora_providers.dart 加一个 `StreamProvider`/`StateNotifier` 简单包装，登录/登出时更新）。

- [ ] **Step 4: 跑测试确认通过 + analyze**

Run: `flutter test test/features/weknora/weknora_login_page_test.dart && flutter analyze`
Expected: PASS / 无新增告警

- [ ] **Step 5: Commit**

```bash
git add lib/features/weknora/account/weknora_login_page.dart lib/core/router/ lib/features/auth/views/backend_chooser_page.dart lib/features/profile/views/profile_page.dart lib/features/weknora/account/weknora_providers.dart test/features/weknora/weknora_login_page_test.dart
git commit -m "feat(weknora): login page, route, backend chooser entry, settings account section"
```

---

### Task 9: l10n ×14 + 全量回归 + 登录态引导

**Files:**
- Modify: `lib/l10n/app_*.arb` ×14（新 key 列表见下）
- Test: 无新测试文件（跑全量回归）

**Interfaces:**
- Consumes: Task 8 使用的 key。

- [ ] **Step 1: 在 app_en.arb 追加（含 @ 描述）**

```json
"weknoraLoginTitle": "Sign in to WeKnora",
"weknoraLoginServerField": "Server address",
"weknoraLoginEmailField": "Email",
"weknoraLoginPasswordField": "Password",
"weknoraLoginButton": "Sign in",
"weknoraLoginInvalidCredentials": "Incorrect email or password.",
"weknoraLoginUnreachable": "Could not reach the WeKnora server.",
"weknoraBackendChooserTitle": "WeKnora",
"weknoraBackendChooserSubtitle": "Sign in to a self-hosted WeKnora workspace",
"weknoraAccountSectionTitle": "WeKnora account",
"weknoraAccountSignOut": "Sign out",
"weknoraAccountSignedInAs": "Signed in as {email}",
"weknoraAccountServer": "Server: {server}",
"weknoraSessionDeletedNotice": "This conversation was deleted on the server."
```

（每个 key 配 `@key` description；`{email}`/`{server}` 用 placeholders 元数据，仿既有带参 key。）

- [ ] **Step 2: app_zh.arb 同 key 中文**（"登录 WeKnora"/"服务器地址"/"邮箱"/"密码"/"登录"/"邮箱或密码不正确。"/"无法连接 WeKnora 服务器。"/"WeKnora"/"登录自托管的 WeKnora 工作区"/"WeKnora 账号"/"退出登录"/"已登录：{email}"/"服务器：{server}"/"该会话已在服务器上删除。"），其余 12 个语言文件按同 key 各自语言补齐（cs/da/de/es/fr/it/ja/ko/nl/pl/pt/ru/sv 以仓内实际文件列表为准——`ls lib/l10n/app_*.arb`）。

- [ ] **Step 3: 校验 l10n**

Run: `dart run tool/validate_arb_locales.dart && dart run tool/verify_arb_descriptions.dart && flutter gen-l10n`
Expected: 全部通过

- [ ] **Step 4: 全量回归**

Run: `flutter analyze && flutter test test/features/weknora/ test/features/direct_connections/ test/features/chat/weknora_chat_parameters_test.dart`
Expected: analyze 零新增告警；测试全绿（direct_connections 既有 631+ 全过）

- [ ] **Step 5: 真机冒烟（可选但推荐，用既有模拟器）**

已 booted 的 iPhone 18 Pro（Device Hub 窗口）：`flutter run -d 0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`（后端 8084 在跑）→ 设置里登出旧 API key 连接 → BackendChooser/登录页用 conduit-smoke@test.dev 登录 → 抽屉出现服务端会话列表 → 打开历史会话能看到消息 → 发一条新消息收到 agent 流式回复 → 网页端确认新会话出现。

- [ ] **Step 6: Commit**

```bash
git add lib/l10n/
git commit -m "feat(weknora): l10n strings for login and account UI across 14 locales"
```

---

## Self-Review 记录

- **Spec 覆盖**：登录页/入口（T8）、JWT 单飞刷新+镜像（T2/T3）、登出（T3/T8）、列表 upsert（T5/T6）、消息加载（T5/T6）、继续聊精确绑定（T4/T7）、回填（T6/T7）、404 剔除（T6）、l10n（T9）——spec 各节均有对应任务。
- **占位符**：Task 3 的 `firstWhereOrNull` 展示处保留了意图注释但给出完整条件描述；Task 5 `loadMessages` 注释给出精确映射规则；Task 7 的 `conversation` 变量名给了核对指令——均为执行期核对指令而非缺失内容。
- **类型一致性**：`weknora_session_id`（T4 读取 = T7 注入）；`boundSessionIdFor` 签名（T4 定义 = T6 调用）；`WeKnoraSessionApi.connection` 返回 record 类型（T5 定义 = T7 providers 绑定使用）；Conversation metadata key `weknoraSessionId` 全程一致。
