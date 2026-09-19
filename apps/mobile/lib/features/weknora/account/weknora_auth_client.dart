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
  const WeKnoraTokenPair({
    required this.accessToken,
    required this.refreshToken,
  });
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
      displayName:
          user['username'] as String? ?? user['email'] as String? ?? '',
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

  Future<void> logout({
    required String baseUrl,
    required String refreshToken,
  }) async {
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
    final dio = _dioFactory();
    // Dio concatenates baseUrl and path verbatim, so the base needs its
    // trailing slash for relative paths such as 'api/v1/auth/login' to
    // resolve (mirrors DirectConnectionProfile.requestBaseUri).
    final trimmed = baseUrl.trim();
    dio.options.baseUrl = trimmed.endsWith('/') ? trimmed : '$trimmed/';
    try {
      final response = await dio.post<dynamic>(
        path,
        data: body,
        options: Options(
          validateStatus: (status) => status != null && status < 500,
        ),
      );
      final data = response.data;
      final map = data is String
          ? jsonDecode(data) as Map<String, dynamic>
          : (data as Map?)?.cast<String, dynamic>() ??
                const <String, dynamic>{};
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
    } on DioException {
      throw WeKnoraAuthException(
        'Could not reach the WeKnora server.',
        serverUnreachable: true,
      ); // error intentionally not surfaced: it may carry the password body.
    }
  }
}
