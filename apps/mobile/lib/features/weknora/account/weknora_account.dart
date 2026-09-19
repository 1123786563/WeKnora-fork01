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
      other is WeKnoraAccount &&
      other.baseUrl == baseUrl &&
      other.email == email &&
      other.userId == userId &&
      other.displayName == displayName &&
      other.accessToken == accessToken &&
      other.refreshToken == refreshToken;

  @override
  int get hashCode => Object.hashAll(toJson().values);
}
