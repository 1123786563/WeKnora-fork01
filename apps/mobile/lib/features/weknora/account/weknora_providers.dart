import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/models/conversation.dart';
import '../../../core/providers/app_providers.dart';
import '../../../core/services/secure_credential_storage.dart';
import '../../direct_connections/providers/direct_connection_providers.dart';
import '../sessions/weknora_session_api.dart';
import '../sessions/weknora_session_sync.dart';
import 'weknora_account.dart';
import 'weknora_account_service.dart';
import 'weknora_auth_client.dart';

/// Decodes the persisted WeKnora account document; null when absent.
Future<WeKnoraAccount?> _readPersistedWeKnoraAccount(
  SecureCredentialStorage storage,
) async {
  final raw = await storage.readWeKnoraAccount();
  if (raw == null) return null;
  return WeKnoraAccount.fromJson(jsonDecode(raw) as Map<String, dynamic>);
}

/// App-wide WeKnora account service.
///
/// The account document lives in secure storage under its own versioned key;
/// the live access token is mirrored into the WeKnora Direct connection
/// profile so ordinary requests ride the Direct connection pool.
// Explicit type: the auth-expired listener below references
// [weknoraAccountProvider], whose own type is inferred from this provider —
// an inferred type here would be a circularity error.
final Provider<WeKnoraAccountService> weknoraAccountServiceProvider =
    Provider<WeKnoraAccountService>((ref) {
  final storage = SecureCredentialStorage(
    instance: ref.watch(secureStorageProvider),
  );
  final service = WeKnoraAccountService(
    // The login screen constructs its own Dio with user-facing options; the
    // service depends only on the authClient abstraction, so this factory is
    // a placeholder for direct construction paths.
    authClient: WeKnoraAuthClient(
      dioFactory: () => Dio(BaseOptions(baseUrl: 'unused')),
    ),
    loadAccount: () => _readPersistedWeKnoraAccount(storage),
    persistAccount: (account) => account == null
        ? storage.deleteWeKnoraAccount()
        : storage.writeWeKnoraAccount(jsonEncode(account.toJson())),
    listProfiles: () => ref.read(directConnectionProfileStoreProvider).load(),
    upsertProfile: (profile) =>
        ref.read(directConnectionProfilesProvider.notifier).upsert(profile),
  );
  // A failed token refresh clears the persisted account before the service
  // fires this listener; re-read it so reactive UI (the settings account
  // section) flips to the signed-out state instead of showing a stale
  // account whose tokens no longer work.
  service.addAuthExpiredListener(() {
    ref.invalidate(weknoraAccountProvider);
  });
  return service;
});

/// Signed-in WeKnora account for reactive UI (the settings account section).
///
/// The account service is a plain class without change notifications, so this
/// provider reads the same secure-storage document the service persists to.
/// The login page and the settings sign-out flow `ref.invalidate` it after
/// mutating the session; while the read is in flight, watchers see no account
/// and render the signed-out entry.
final weknoraAccountProvider = FutureProvider<WeKnoraAccount?>((ref) {
  // Watching the service keeps this provider in step with test overrides.
  ref.watch(weknoraAccountServiceProvider);
  final storage = SecureCredentialStorage(
    instance: ref.watch(secureStorageProvider),
  );
  return _readPersistedWeKnoraAccount(storage);
});

/// Bridges [WeKnoraSessionSync] onto the app's conversation list.
///
/// The API's connection seam resolves per call through the account service:
/// `accessToken()` restores the account and single-flight refreshes the JWT,
/// and the base URL comes from the restored account. Conversation mutations
/// are the conversations notifier's own methods, matching the sync layer's
/// injected seams.
final weknoraSessionSyncProvider = Provider<WeKnoraSessionSync>((ref) {
  final accountService = ref.watch(weknoraAccountServiceProvider);
  return WeKnoraSessionSync(
    api: WeKnoraSessionApi(
      connection: () async {
        // accessToken() throws WeKnoraAuthException when nobody is signed
        // in; once it returns, the account is restored and non-null.
        final token = await accountService.accessToken();
        final account = accountService.currentAccount;
        if (account == null) {
          throw const WeKnoraAuthException(
            'Not signed in to WeKnora.',
            invalidCredentials: true,
          );
        }
        return (baseUrl: account.baseUrl, token: token);
      },
      // The session API applies its own base URL and auth headers per
      // request; the factory only supplies the transport (same shape as the
      // account service's placeholder factory above).
      dioFactory: () => Dio(BaseOptions(baseUrl: 'unused')),
    ),
    upsertConversation: (conversation) => ref
        .read(conversationsProvider.notifier)
        .upsertConversation(conversation),
    updateConversation: (id, transform) => ref
        .read(conversationsProvider.notifier)
        .updateConversation(id, transform),
    removeConversation: (id) =>
        ref.read(conversationsProvider.notifier).removeConversation(id),
  );
});

/// Refresh-then-hydrate invariant for the WeKnora session list.
///
/// [WeKnoraSessionSync.refreshSessions] upserts mapper-built records that
/// carry no messages, so a refresh would blank the transcript of every
/// WeKnora conversation whose history is already loaded locally. Callers
/// (the drawer / session-list flows) must therefore re-hydrate after a
/// refresh; this provider bundles the two steps so the invariant cannot be
/// skipped:
///
/// 1. remember which WeKnora conversations currently hold messages,
/// 2. refresh the server session list,
/// 3. re-hydrate exactly those conversations from the server.
final weknoraSessionRefreshAndHydrateProvider =
    Provider<Future<void> Function()>((ref) => () async {
      final sync = ref.read(weknoraSessionSyncProvider);
      List<Conversation> conversations() =>
          ref.read(conversationsProvider).asData?.value ??
          const <Conversation>[];
      final hadMessages = <String>{
        for (final conversation in conversations())
          if (conversation.metadata['weknoraSessionId'] is String &&
              conversation.messages.isNotEmpty)
            conversation.id,
      };
      await sync.refreshSessions();
      for (final conversation in conversations()) {
        if (!hadMessages.contains(conversation.id)) continue;
        await sync.hydrateMessages(conversation: conversation);
      }
    });
