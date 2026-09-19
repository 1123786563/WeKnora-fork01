import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/features/direct_connections/models/direct_connection_profile.dart';
import 'package:conduit/features/weknora/account/weknora_account.dart';
import 'package:conduit/features/weknora/account/weknora_account_service.dart';
import 'package:conduit/features/weknora/account/weknora_auth_client.dart';

/// Minimal unsigned JWT whose payload only carries `exp`, good enough for
/// [weknoraJwtExpiry] (signature is never verified client-side).
String _jwt({required DateTime exp}) {
  String encodeSegment(Map<String, dynamic> payload) =>
      base64Url.encode(utf8.encode(jsonEncode(payload)));
  return '${encodeSegment({'alg': 'HS256', 'typ': 'JWT'})}'
      '.${encodeSegment({'exp': exp.millisecondsSinceEpoch ~/ 1000})}'
      '.sig';
}

WeKnoraLoginResult _loginResult({
  String accessToken = 'access-login',
  String refreshToken = 'refresh-login',
}) => WeKnoraLoginResult(
  tokens: WeKnoraTokenPair(
    accessToken: accessToken,
    refreshToken: refreshToken,
  ),
  userId: 'u1',
  displayName: 'alice',
);

/// Records every auth call and returns programmable results.
class _FakeAuth implements WeKnoraAuthClient {
  _FakeAuth({this.loginResult, this.refreshPair, this.refreshError});

  WeKnoraLoginResult? loginResult;
  WeKnoraTokenPair? refreshPair;
  WeKnoraAuthException? refreshError;

  int loginCalls = 0;
  int refreshCalls = 0;
  int logoutCalls = 0;

  @override
  Future<WeKnoraLoginResult> login({
    required String baseUrl,
    required String email,
    required String password,
  }) async {
    loginCalls++;
    return loginResult!;
  }

  @override
  Future<WeKnoraTokenPair> refresh({
    required String baseUrl,
    required String refreshToken,
  }) async {
    refreshCalls++;
    final error = refreshError;
    if (error != null) throw error;
    return refreshPair!;
  }

  @override
  Future<void> logout({
    required String baseUrl,
    required String refreshToken,
  }) async {
    logoutCalls++;
  }
}

/// In-memory load/persist implementation of the account storage functions.
class _AccountStorageFake {
  _AccountStorageFake([Map<String, WeKnoraAccount> stored = const {}])
    : _accounts = Map.of(stored);

  static const String _key = 'weknora_account_v1';
  final Map<String, WeKnoraAccount> _accounts;
  int writes = 0;

  bool get isEmpty => _accounts.isEmpty;
  WeKnoraAccount? get single => _accounts[_key];

  Future<WeKnoraAccount?> load() async => _accounts[_key];

  Future<void> persist(WeKnoraAccount? account) async {
    writes++;
    if (account == null) {
      _accounts.remove(_key);
    } else {
      _accounts[_key] = account;
    }
  }
}

/// In-memory list/upsert implementation of the profile sink functions.
class _ProfileSinkFake {
  _ProfileSinkFake([List<DirectConnectionProfile> profiles = const []])
    : _profiles = List.of(profiles);

  final List<DirectConnectionProfile> _profiles;

  List<DirectConnectionProfile> get profiles => List.unmodifiable(_profiles);

  Future<List<DirectConnectionProfile>> list() async => List.of(_profiles);

  Future<void> upsert(DirectConnectionProfile profile) async {
    final index = _profiles.indexWhere((item) => item.id == profile.id);
    if (index >= 0) {
      _profiles[index] = profile;
    } else {
      _profiles.add(profile);
    }
  }
}

typedef _Harness = ({
  WeKnoraAccountService service,
  _AccountStorageFake storage,
  _ProfileSinkFake profileSink,
});

_Harness _makeService({
  required _FakeAuth auth,
  Map<String, WeKnoraAccount> stored = const {},
  List<DirectConnectionProfile> profiles = const [],
}) {
  final storage = _AccountStorageFake(stored);
  final profileSink = _ProfileSinkFake(profiles);
  return (
    service: WeKnoraAccountService(
      authClient: auth,
      loadAccount: storage.load,
      persistAccount: storage.persist,
      listProfiles: profileSink.list,
      upsertProfile: profileSink.upsert,
    ),
    storage: storage,
    profileSink: profileSink,
  );
}

void main() {
  test('login persists account and mirrors a bearer profile', () async {
    final auth = _FakeAuth(loginResult: _loginResult());
    final harness = _makeService(auth: auth);

    final account = await harness.service.login(
      baseUrl: 'http://localhost:8080',
      email: 'alice@example.com',
      password: 'pw123',
    );

    expect(account.email, 'alice@example.com');
    expect(harness.service.currentAccount?.email, 'alice@example.com');
    expect(harness.storage.single, account);
    expect(auth.loginCalls, 1);

    final profile = harness.profileSink.profiles.single;
    expect(profile.adapterKey, kWeKnoraAdapterKey);
    expect(profile.apiKeyAuthMode, DirectApiKeyAuthMode.bearer);
    expect(profile.apiKey, 'access-login');
    expect(profile.enabled, isTrue);
    expect(profile.baseUrl, 'http://localhost:8080');
  });

  test('login reuses and updates the existing weknora profile for same baseUrl',
      () async {
    final preset = DirectConnectionProfile(
      id: 'p1',
      name: 'WeKnora',
      adapterKey: kWeKnoraAdapterKey,
      baseUrl: 'http://localhost:8080',
      apiKey: 'stale-key',
    );
    final auth = _FakeAuth(loginResult: _loginResult(accessToken: 'new-key'));
    final harness = _makeService(auth: auth, profiles: [preset]);

    // Trailing slash on purpose: profile matching must use the normalized
    // origin, not the raw string.
    await harness.service.login(
      baseUrl: 'http://localhost:8080/',
      email: 'a@b.c',
      password: 'pw',
    );

    final profiles = harness.profileSink.profiles;
    expect(profiles, hasLength(1));
    expect(profiles.single.id, 'p1');
    expect(profiles.single.apiKey, 'new-key');
    expect(profiles.single.enabled, isTrue);
  });

  test('accessToken refreshes single-flight and mirrors new token into profile',
      () async {
    final now = DateTime.now().toUtc();
    final stored = WeKnoraAccount(
      baseUrl: 'http://localhost:8080',
      email: 'a@b.c',
      userId: 'u1',
      displayName: 'alice',
      accessToken: _jwt(exp: now.subtract(const Duration(minutes: 5))),
      refreshToken: 'refresh-old',
    );
    final newAccess = _jwt(exp: now.add(const Duration(hours: 1)));
    final auth = _FakeAuth(
      refreshPair: WeKnoraTokenPair(
        accessToken: newAccess,
        refreshToken: 'refresh-new',
      ),
    );
    final harness = _makeService(
      auth: auth,
      stored: {'weknora_account_v1': stored},
    );

    final tokens = await Future.wait([
      harness.service.accessToken(),
      harness.service.accessToken(),
    ]);

    expect(tokens, everyElement(newAccess));
    expect(auth.refreshCalls, 1);
    expect(harness.storage.single?.accessToken, newAccess);
    expect(harness.storage.single?.refreshToken, 'refresh-new');
    expect(harness.profileSink.profiles.single.apiKey, newAccess);
  });

  test('accessToken returns current token when far from expiry', () async {
    final now = DateTime.now().toUtc();
    final access = _jwt(exp: now.add(const Duration(hours: 1)));
    final stored = WeKnoraAccount(
      baseUrl: 'http://localhost:8080',
      email: 'a@b.c',
      userId: 'u1',
      displayName: 'alice',
      accessToken: access,
      refreshToken: 'refresh-old',
    );
    final auth = _FakeAuth();
    final harness = _makeService(
      auth: auth,
      stored: {'weknora_account_v1': stored},
    );

    expect(await harness.service.accessToken(), access);
    expect(auth.refreshCalls, 0);
  });

  test('refresh failure clears account and fires auth-expired listener',
      () async {
    final now = DateTime.now().toUtc();
    final stored = WeKnoraAccount(
      baseUrl: 'http://localhost:8080',
      email: 'a@b.c',
      userId: 'u1',
      displayName: 'alice',
      accessToken: _jwt(exp: now.subtract(const Duration(minutes: 5))),
      refreshToken: 'refresh-dead',
    );
    final auth = _FakeAuth(
      refreshError: const WeKnoraAuthException(
        'Session expired.',
        invalidCredentials: true,
      ),
    );
    final harness = _makeService(
      auth: auth,
      stored: {'weknora_account_v1': stored},
    );
    var expiredEvents = 0;
    harness.service.addAuthExpiredListener(() => expiredEvents++);

    await expectLater(
      harness.service.accessToken(),
      throwsA(
        isA<WeKnoraAuthException>().having(
          (error) => error.invalidCredentials,
          'invalidCredentials',
          isTrue,
        ),
      ),
    );

    expect(harness.service.currentAccount, isNull);
    expect(harness.storage.isEmpty, isTrue);
    expect(expiredEvents, 1);
  });

  test('logout clears account and disables profile', () async {
    final auth = _FakeAuth(loginResult: _loginResult());
    final harness = _makeService(auth: auth);

    await harness.service.login(
      baseUrl: 'http://localhost:8080',
      email: 'a@b.c',
      password: 'pw',
    );
    await harness.service.logout();

    expect(harness.service.currentAccount, isNull);
    expect(harness.storage.isEmpty, isTrue);
    expect(harness.profileSink.profiles.single.enabled, isFalse);
    expect(auth.logoutCalls, 1);
  });
}
