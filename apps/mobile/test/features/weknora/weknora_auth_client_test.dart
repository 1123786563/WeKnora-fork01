import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:conduit/features/weknora/account/weknora_auth_client.dart';

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
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      final result = await client.login(
        baseUrl: 'http://localhost:8084',
        email: 'a@b.c',
        password: 'pw123',
      );
      expect(result.tokens.accessToken, 'access-1');
      expect(result.tokens.refreshToken, 'refresh-1');
      expect(result.displayName, 'alice');
      expect(http.requests.single.path, 'api/v1/auth/login');
      // The uri getter resolves the raw path against the Dio baseUrl, so
      // this locks the base-url join (path alone would pass either way).
      expect(
        http.requests.single.uri.toString(),
        'http://localhost:8084/api/v1/auth/login',
      );
      expect(
        (http.requests.single.data as Map<String, dynamic>)['email'],
        'a@b.c',
      );
    });

    test('maps 401 to invalidCredentials', () async {
      final http = _QueuedAdapter([
        _Reply.json({'success': false, 'message': 'bad'}, statusCode: 401),
      ]);
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      await expectLater(
        client.login(baseUrl: 'http://x', email: 'a@b.c', password: 'pw'),
        throwsA(
          isA<WeKnoraAuthException>().having(
            (e) => e.invalidCredentials,
            'invalidCredentials',
            isTrue,
          ),
        ),
      );
    });

    test('maps connection error to serverUnreachable', () async {
      final throwing = _QueuedAdapter([]);
      // An empty queue makes the mock adapter throw inside dio, and dio
      // wraps any adapter error in a DioException; the client must map that
      // to WeKnoraAuthException rather than let it escape.
      final client = WeKnoraAuthClient(dioFactory: () => _dio(throwing));
      try {
        await client.login(baseUrl: 'http://x', email: 'a', password: '123456');
        fail('unreachable');
      } on WeKnoraAuthException catch (e) {
        expect(e.serverUnreachable || e.invalidCredentials, isTrue);
      }
    });

    test('maps a non-JSON 200 body to serverUnreachable', () async {
      // A misconfigured base URL or captive portal answers with HTML.
      final http = _QueuedAdapter([
        _Reply.stream([
          utf8.encode('<html>Sign in</html>'),
        ], contentType: 'text/html; charset=utf-8'),
      ]);
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      await expectLater(
        client.login(baseUrl: 'http://x', email: 'a@b.c', password: 'pw'),
        throwsA(
          isA<WeKnoraAuthException>().having(
            (e) => e.serverUnreachable,
            'serverUnreachable',
            isTrue,
          ),
        ),
      );
    });

    test('maps a JSON array 200 body to serverUnreachable', () async {
      final http = _QueuedAdapter([
        _Reply.stream([
          utf8.encode('[1,2]'),
        ], contentType: 'application/json; charset=utf-8'),
      ]);
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      await expectLater(
        client.login(baseUrl: 'http://x', email: 'a@b.c', password: 'pw'),
        throwsA(isA<WeKnoraAuthException>()),
      );
    });

    test('tolerates a non-map user field', () async {
      final http = _QueuedAdapter([
        _Reply.json({
          'success': true,
          'token': 'access-1',
          'refresh_token': 'refresh-1',
          'user': 'oops',
        }),
      ]);
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      final result = await client.login(
        baseUrl: 'http://x',
        email: 'a@b.c',
        password: 'pw',
      );
      expect(result.tokens.accessToken, 'access-1');
      expect(result.userId, '');
      expect(result.displayName, '');
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
      final client = WeKnoraAuthClient(dioFactory: () => _dio(http));
      final pair = await client.refresh(
        baseUrl: 'http://x',
        refreshToken: 'refresh-1',
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

// Copied (behavior-identical) from
// test/features/direct_connections/weknora_adapter_test.dart so this file
// stays self-contained.

Dio _dio(HttpClientAdapter adapter) {
  final dio = Dio();
  dio.httpClientAdapter = adapter;
  return dio;
}

final class _QueuedAdapter implements HttpClientAdapter {
  _QueuedAdapter(this._replies);

  final List<_Reply> _replies;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (_replies.isEmpty) throw StateError('No fake response remains.');
    return _replies.removeAt(0).toBody();
  }

  @override
  void close({bool force = false}) {}
}

final class _Reply {
  const _Reply(this.chunks, this.contentType, this.statusCode);

  factory _Reply.json(Map<String, dynamic> value, {int statusCode = 200}) =>
      _Reply(
        [utf8.encode(jsonEncode(value))],
        'application/json; charset=utf-8',
        statusCode,
      );

  factory _Reply.stream(
    List<List<int>> chunks, {
    required String contentType,
    int statusCode = 200,
  }) => _Reply(chunks, contentType, statusCode);

  final List<List<int>> chunks;
  final String contentType;
  final int statusCode;

  ResponseBody toBody() => ResponseBody(
    Stream<Uint8List>.fromIterable([
      for (final chunk in chunks) Uint8List.fromList(chunk),
    ]),
    statusCode,
    headers: {
      'content-type': [contentType],
    },
  );
}
