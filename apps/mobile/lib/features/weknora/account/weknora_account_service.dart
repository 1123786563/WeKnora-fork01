import 'package:uuid/uuid.dart';

import '../../direct_connections/models/direct_connection_profile.dart';
import 'weknora_account.dart';
import 'weknora_auth_client.dart';

/// Owns the signed-in WeKnora account: login state, single-flight token
/// refresh, mirroring the session into a Direct connection profile, and
/// logout.
///
/// Storage and profile writes are injected functions so the service never
/// touches plugins directly and stays unit-testable.
class WeKnoraAccountService {
  WeKnoraAccountService({
    required WeKnoraAuthClient authClient,
    required Future<WeKnoraAccount?> Function() loadAccount,
    required Future<void> Function(WeKnoraAccount? account) persistAccount,
    required Future<List<DirectConnectionProfile>> Function() listProfiles,
    required Future<void> Function(DirectConnectionProfile profile)
    upsertProfile,
  }) : _authClient = authClient,
       _loadAccount = loadAccount,
       _persistAccount = persistAccount,
       _listProfiles = listProfiles,
       _upsertProfile = upsertProfile;

  final WeKnoraAuthClient _authClient;
  final Future<WeKnoraAccount?> Function() _loadAccount;
  final Future<void> Function(WeKnoraAccount? account) _persistAccount;
  final Future<List<DirectConnectionProfile>> Function() _listProfiles;
  final Future<void> Function(DirectConnectionProfile profile) _upsertProfile;

  /// Tokens closer than this to their `exp` are treated as expired so a
  /// request never leaves with a token the server would reject mid-flight.
  static const Duration _expirySlack = Duration(seconds: 60);

  WeKnoraAccount? _account;
  Future<WeKnoraTokenPair>? _refreshInFlight;
  final List<void Function()> _authExpiredListeners = [];

  WeKnoraAccount? get currentAccount => _account;

  void addAuthExpiredListener(void Function() listener) =>
      _authExpiredListeners.add(listener);

  Future<WeKnoraAccount?> _restore() async => _account ??= await _loadAccount();

  /// Signs in, persists the account, and mirrors the session as a bearer
  /// Direct connection profile.
  Future<WeKnoraAccount> login({
    required String baseUrl,
    required String email,
    required String password,
  }) async {
    final result = await _authClient.login(
      baseUrl: baseUrl,
      email: email,
      password: password,
    );
    final account = WeKnoraAccount(
      baseUrl: baseUrl,
      email: email,
      userId: result.userId,
      displayName: result.displayName,
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
  /// Transient refresh failures (server unreachable) rethrow with the stored
  /// account intact so a later call retries the refresh.
  Future<String> accessToken() async {
    final account = await _restore();
    if (account == null) {
      throw const WeKnoraAuthException(
        'Not signed in to WeKnora.',
        invalidCredentials: true,
      );
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
        baseUrl: account.baseUrl,
        refreshToken: account.refreshToken,
      );
      final next = account.copyWith(
        accessToken: pair.accessToken,
        refreshToken: pair.refreshToken,
      );
      _account = next;
      await _persistAccount(next);
      await _mirrorIntoProfile(next);
      return pair;
    } on WeKnoraAuthException catch (error) {
      // Only a definitively rejected refresh token ends the session. A
      // transient failure (server unreachable, network blip) must keep the
      // stored account so the next call retries the refresh instead of
      // wiping credentials that may still be perfectly good.
      if (!error.invalidCredentials) rethrow;
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

  /// Keeps the WeKnora Direct profile in sync with the live session: the
  /// profile carries the access token as a bearer API key so ordinary
  /// requests ride the Direct connection pool.
  Future<void> _mirrorIntoProfile(WeKnoraAccount account) async {
    final profiles = await _listProfiles();
    final origin = DirectConnectionProfile.originOf(account.baseUrl);
    if (origin == null) return;
    // `firstOrNull` comes from dart:core's IterableExtensions (Dart 3);
    // package:collection is only a dev_dependency in this repo.
    final existing = profiles
        .where(
          (profile) =>
              profile.adapterKey == kWeKnoraAdapterKey &&
              DirectConnectionProfile.originOf(profile.baseUrl) == origin,
        )
        .firstOrNull;
    if (existing != null) {
      await _upsertProfile(
        existing.copyWith(apiKey: account.accessToken, enabled: true),
      );
    } else {
      await _upsertProfile(
        DirectConnectionProfile(
          id: const Uuid().v4(),
          name: 'WeKnora',
          adapterKey: kWeKnoraAdapterKey,
          baseUrl: origin,
          apiKeyAuthMode: DirectApiKeyAuthMode.bearer,
          apiKey: account.accessToken,
        ),
      );
    }
  }

  /// Revokes the refresh token best-effort, disables the mirrored profile,
  /// and clears all local account state.
  Future<void> logout() async {
    final account = await _restore();
    if (account != null) {
      await _authClient.logout(
        baseUrl: account.baseUrl,
        refreshToken: account.refreshToken,
      );
      final profiles = await _listProfiles();
      final origin = DirectConnectionProfile.originOf(account.baseUrl);
      // Normalize the stored base URL the same way the login mirror does:
      // a profile saved with a trailing slash (or without one) still belongs
      // to this origin and must be disabled on logout.
      final mine = profiles.where(
        (profile) =>
            profile.adapterKey == kWeKnoraAdapterKey &&
            (origin == null ||
                DirectConnectionProfile.originOf(profile.baseUrl) == origin),
      );
      for (final profile in mine) {
        await _upsertProfile(profile.copyWith(enabled: false));
      }
    }
    _account = null;
    await _persistAccount(null);
  }
}
