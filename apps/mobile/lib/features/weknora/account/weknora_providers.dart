import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/models/conversation.dart';
import '../../../core/providers/app_providers.dart';
import '../../../core/services/navigation_service.dart';
import '../../../core/services/secure_credential_storage.dart';
import '../../../core/utils/debug_logger.dart';
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
    // Spec: an expired WeKnora session routes the user back to the sign-in
    // page. Navigate only from the chat shell (chat home / folder pages,
    // where WeKnora conversations live) so the listener never hijacks the
    // sign-in page itself, onboarding, or any other surface.
    final route = NavigationService.currentRoute;
    final onChatShell =
        route == Routes.chat || route?.startsWith('/folder/') == true;
    if (onChatShell) {
      NavigationService.router.go(Routes.weknoraLogin);
    }
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
/// carry no messages, so after a refresh every WeKnora conversation that was
/// re-upserted holds an empty transcript. Callers (the login flow, the
/// conversation drawer, opening a conversation) must therefore re-hydrate
/// after a refresh; this provider bundles the two steps so the invariant
/// cannot be skipped:
///
/// 1. remember which WeKnora conversations currently hold messages,
/// 2. refresh the server session list,
/// 3. hydrate every WeKnora conversation that either held messages before
///    the refresh (its transcript was just blanked by the re-upsert, or its
///    session vanished from the server list and the load's 404 evicts it)
///    or is now empty (first login: freshly upserted with no history yet).
///
/// One conversation's hydrate failure must not abort the rest: each runs
/// inside its own try/catch and failures are logged. A 404 during hydrate
/// evicts the conversation through the sync layer; the evicted conversation
/// ids are returned so callers can surface the deletion notice.
final weknoraSessionRefreshAndHydrateProvider =
    Provider<Future<List<String>> Function()>((ref) => () async {
      final sync = ref.read(weknoraSessionSyncProvider);
      List<Conversation> conversations() =>
          ref.read(conversationsProvider).asData?.value ??
          const <Conversation>[];
      final hydratedBefore = <String>{
        for (final conversation in conversations())
          if (conversation.metadata['weknoraSessionId'] is String &&
              conversation.messages.isNotEmpty)
            conversation.id,
      };
      await sync.refreshSessions();
      final evicted = <String>[];
      for (final conversation in conversations()) {
        if (conversation.metadata['weknoraSessionId'] is! String) continue;
        // Skip only conversations that never held messages and were not
        // re-upserted by this refresh (they have nothing to restore and no
        // eviction to reconcile).
        if (!hydratedBefore.contains(conversation.id) &&
            conversation.messages.isNotEmpty) {
          continue;
        }
        try {
          await sync.hydrateMessages(conversation: conversation);
        } catch (error, stackTrace) {
          // Best-effort per conversation: keep hydrating the rest.
          DebugLogger.error(
            'weknora-session-hydrate-failed',
            scope: 'weknora/sync',
            error: error,
            stackTrace: stackTrace,
            data: {'conversationId': conversation.id},
          );
          continue;
        }
        if (!conversations().any(
          (current) => current.id == conversation.id,
        )) {
          evicted.add(conversation.id);
        }
      }
      return evicted;
    });

/// Account-guarded variant of the refresh-and-hydrate cycle for UI callers.
///
/// Reads the persisted account document first and does nothing when nobody
/// is signed in, so the drawer / conversation-open wiring never issues
/// authenticated WeKnora requests for unauthenticated users. Refresh
/// failures are logged, not surfaced: the drawer's pull-to-refresh has no
/// per-source error affordance and swallows its own list refresh failures
/// the same way (log-only for this wave). Returns the ids of conversations
/// evicted because the server no longer knows their session (404), so
/// callers can surface the deletion notice.
final weknoraSessionRefreshIfSignedInProvider =
    Provider<Future<List<String>> Function()>((ref) => () async {
      final WeKnoraAccount? account;
      try {
        account = await ref.read(weknoraAccountProvider.future);
      } catch (_) {
        // The account document is unreadable; there is nothing to sync.
        return const <String>[];
      }
      if (account == null) return const <String>[];
      try {
        return await ref.read(weknoraSessionRefreshAndHydrateProvider)();
      } catch (error, stackTrace) {
        DebugLogger.error(
          'weknora-session-refresh-failed',
          scope: 'weknora/sync',
          error: error,
          stackTrace: stackTrace,
        );
        return const <String>[];
      }
    });
