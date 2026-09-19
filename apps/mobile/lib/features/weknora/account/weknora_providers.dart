import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers/storage_providers.dart';
import '../../../core/services/secure_credential_storage.dart';
import '../../direct_connections/providers/direct_connection_providers.dart';
import 'weknora_account.dart';
import 'weknora_account_service.dart';
import 'weknora_auth_client.dart';

/// App-wide WeKnora account service.
///
/// The account document lives in secure storage under its own versioned key;
/// the live access token is mirrored into the WeKnora Direct connection
/// profile so ordinary requests ride the Direct connection pool.
final weknoraAccountServiceProvider = Provider<WeKnoraAccountService>((ref) {
  final storage = SecureCredentialStorage(
    instance: ref.watch(secureStorageProvider),
  );
  return WeKnoraAccountService(
    // The login screen constructs its own Dio with user-facing options; the
    // service depends only on the authClient abstraction, so this factory is
    // a placeholder for direct construction paths.
    authClient: WeKnoraAuthClient(
      dioFactory: () => Dio(BaseOptions(baseUrl: 'unused')),
    ),
    loadAccount: () async {
      final raw = await storage.readWeKnoraAccount();
      if (raw == null) return null;
      return WeKnoraAccount.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    },
    persistAccount: (account) => account == null
        ? storage.deleteWeKnoraAccount()
        : storage.writeWeKnoraAccount(jsonEncode(account.toJson())),
    listProfiles: () => ref.read(directConnectionProfileStoreProvider).load(),
    upsertProfile: (profile) =>
        ref.read(directConnectionProfilesProvider.notifier).upsert(profile),
  );
});
