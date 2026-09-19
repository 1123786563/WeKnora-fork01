import 'package:conduit/core/providers/backend_mode_providers.dart';
import 'package:conduit/core/services/navigation_service.dart';
import 'package:conduit/features/weknora/account/weknora_account.dart';
import 'package:conduit/features/weknora/account/weknora_account_service.dart';
import 'package:conduit/features/weknora/account/weknora_auth_client.dart';
import 'package:conduit/features/weknora/account/weknora_login_page.dart';
import 'package:conduit/features/weknora/account/weknora_providers.dart';
import 'package:conduit/l10n/app_localizations.dart';
import 'package:conduit/l10n/conduit_localizations.dart';
import 'package:conduit/shared/theme/app_theme.dart';
import 'package:conduit/shared/theme/tweakcn_themes.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:material_ui/material_ui.dart';

import '../../support/test_fonts.dart';

/// Records login/logout calls and lets each test program the outcome.
class _FakeAccountService implements WeKnoraAccountService {
  _FakeAccountService({this.loginError});

  final Object? loginError;
  final List<({String baseUrl, String email, String password})> loginCalls = [];
  int logoutCalls = 0;

  @override
  Future<WeKnoraAccount> login({
    required String baseUrl,
    required String email,
    required String password,
  }) async {
    loginCalls.add((baseUrl: baseUrl, email: email, password: password));
    final error = loginError;
    if (error != null) throw error;
    return WeKnoraAccount(
      baseUrl: baseUrl,
      email: email,
      userId: 'user-1',
      displayName: 'Test User',
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
    );
  }

  @override
  Future<void> logout() async {
    logoutCalls++;
  }

  @override
  WeKnoraAccount? get currentAccount => null;

  @override
  void addAuthExpiredListener(void Function() listener) {}

  @override
  Future<String> accessToken() => throw UnimplementedError();
}

/// Records [set] calls without touching PreferencesStore, so tests can
/// assert what the login flow requested.
class _RecordingBackendController extends PreferredBackendController {
  _RecordingBackendController(this.setCalls);

  final List<PreferredBackend> setCalls;

  @override
  PreferredBackend build() => PreferredBackend.unset;

  @override
  Future<void> set(PreferredBackend backend) async {
    setCalls.add(backend);
    state = backend;
  }
}

void main() {
  setUpAll(loadTestFonts);

  testWidgets('renders fields and submits login', (tester) async {
    final service = _FakeAccountService();
    await _pumpLoginPage(tester, service: service);

    expect(find.byKey(const Key('weknora-server-field')), findsOneWidget);
    expect(find.byKey(const Key('weknora-email-field')), findsOneWidget);
    expect(find.byKey(const Key('weknora-password-field')), findsOneWidget);
    expect(find.byKey(const Key('weknora-login-button')), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('weknora-server-field')),
      'http://localhost:8084',
    );
    await tester.enterText(
      find.byKey(const Key('weknora-email-field')),
      'a@b.c',
    );
    await tester.enterText(
      find.byKey(const Key('weknora-password-field')),
      'pw123456',
    );
    // enterText does not pump; rebuild once so the submit button's
    // enabled state reflects the now-non-empty fields before tapping.
    await tester.pump();
    await tester.tap(find.byKey(const Key('weknora-login-button')));
    await tester.pumpAndSettle();

    expect(service.loginCalls.single.email, 'a@b.c');
    expect(service.loginCalls.single.baseUrl, 'http://localhost:8084');
    expect(service.loginCalls.single.password, 'pw123456');
    // Successful sign-in lands on the chat home route.
    expect(find.byKey(const Key('weknora-chat-home')), findsOneWidget);
    expect(find.byKey(const Key('weknora-login-button')), findsNothing);
  });

  testWidgets('makes Direct the preferred backend after signing in', (
    tester,
  ) async {
    // Without this the router bounces /chat back to the chooser (fresh
    // install, no active server) or to the authentication page (signed-out
    // OpenWebUI user) — see app_router.dart's redirect.
    final service = _FakeAccountService();
    final backend = _RecordingBackendController([]);
    await _pumpLoginPage(tester, service: service, preferredBackend: backend);

    await _fillAndSubmit(tester);

    expect(backend.setCalls, [PreferredBackend.direct]);
    expect(find.byKey(const Key('weknora-chat-home')), findsOneWidget);
  });

  testWidgets('shows inline error on WeKnoraAuthException', (tester) async {
    final service = _FakeAccountService(
      loginError: const WeKnoraAuthException('bad', invalidCredentials: true),
    );
    await _pumpLoginPage(tester, service: service);

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(const Key('weknora-login-button'))),
    )!;
    await _fillAndSubmit(tester);

    expect(find.text(l10n.weknoraLoginInvalidCredentials), findsOneWidget);
    expect(find.text(l10n.weknoraLoginUnreachable), findsNothing);
    // The failure keeps the user on the login page with the form intact.
    expect(find.byKey(const Key('weknora-login-button')), findsOneWidget);
    expect(find.byKey(const Key('weknora-chat-home')), findsNothing);
    expect(service.loginCalls, hasLength(1));
  });

  testWidgets('maps non-auth failures to the unreachable message', (
    tester,
  ) async {
    final service = _FakeAccountService(loginError: StateError('boom'));
    await _pumpLoginPage(tester, service: service);

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(const Key('weknora-login-button'))),
    )!;
    await _fillAndSubmit(tester);

    expect(find.text(l10n.weknoraLoginUnreachable), findsOneWidget);
    expect(find.text(l10n.weknoraLoginInvalidCredentials), findsNothing);
    expect(find.byKey(const Key('weknora-login-button')), findsOneWidget);
  });
}

Future<void> _fillAndSubmit(WidgetTester tester) async {
  await tester.enterText(
    find.byKey(const Key('weknora-server-field')),
    'http://localhost:8084',
  );
  await tester.enterText(find.byKey(const Key('weknora-email-field')), 'a@b.c');
  await tester.enterText(
    find.byKey(const Key('weknora-password-field')),
    'pw123456',
  );
  // enterText does not pump; rebuild so the submit button is enabled.
  await tester.pump();
  await tester.tap(find.byKey(const Key('weknora-login-button')));
  await tester.pumpAndSettle();
}

/// Pumps the login page behind a minimal GoRouter so the page's post-login
/// `context.go(Routes.chat)` resolves like it does in the real app.
Future<void> _pumpLoginPage(
  WidgetTester tester, {
  required _FakeAccountService service,
  _RecordingBackendController? preferredBackend,
}) async {
  final backend = preferredBackend ?? _RecordingBackendController([]);
  final router = GoRouter(
    initialLocation: Routes.weknoraLogin,
    routes: [
      GoRoute(
        path: Routes.weknoraLogin,
        builder: (context, state) => const WeKnoraLoginPage(),
      ),
      GoRoute(
        path: Routes.chat,
        builder: (context, state) =>
            const Scaffold(key: Key('weknora-chat-home')),
      ),
    ],
  );

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        weknoraAccountServiceProvider.overrideWithValue(service),
        // The real bundle touches secure storage and the conversations
        // notifier; the login page only needs it to not throw.
        weknoraSessionRefreshAndHydrateProvider.overrideWithValue(
          () async => const <String>[],
        ),
        preferredBackendProvider.overrideWith(() => backend),
      ],
      child: MaterialApp.router(
        theme: AppTheme.light(TweakcnThemes.t3Chat),
        locale: const Locale('en'),
        localizationsDelegates: conduitLocalizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    ),
  );
  await tester.pumpAndSettle();
}
