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
