import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_ui/material_ui.dart';

import '../../../core/services/navigation_service.dart';
import '../../../core/utils/debug_logger.dart';
import '../../../l10n/app_localizations.dart';
import '../../../shared/theme/theme_extensions.dart';
import '../../../shared/widgets/connection_components.dart';
import '../../../shared/widgets/conduit_components.dart';
import '../../../shared/widgets/utility_components.dart';
import 'weknora_auth_client.dart';
import 'weknora_providers.dart';

/// Signs in to a self-hosted WeKnora server with email and password.
///
/// Layout mirrors [ServerConnectionPage]: grouped form fields, an inline
/// attempt banner for errors, and a single primary action pinned by the
/// utility scaffold.
class WeKnoraLoginPage extends ConsumerStatefulWidget {
  const WeKnoraLoginPage({super.key});

  @override
  ConsumerState<WeKnoraLoginPage> createState() => _WeKnoraLoginPageState();
}

class _WeKnoraLoginPageState extends ConsumerState<WeKnoraLoginPage> {
  final _server = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();

  String? _error;
  bool _busy = false;

  bool get _canSubmit =>
      !_busy &&
      _server.text.trim().isNotEmpty &&
      _email.text.trim().isNotEmpty &&
      _password.text.isNotEmpty;

  @override
  void dispose() {
    _server.dispose();
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  void _onFieldChanged(String _) {
    // Typing can both clear a stale error and complete the form, so the
    // submit button's enabled state must be recomputed on every change.
    // (Skipping the rebuild when the form was already complete would keep
    // the button disabled after the last field is filled.)
    setState(() => _error = null);
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    final l10n = AppLocalizations.of(context)!;
    // Resolve every provider seam before the first await: navigating to the
    // chat home after a successful login disposes this widget's ref.
    final accountService = ref.read(weknoraAccountServiceProvider);
    final hydrateSessions = ref.read(weknoraSessionRefreshAndHydrateProvider);
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await accountService.login(
        baseUrl: _server.text.trim(),
        email: _email.text.trim(),
        password: _password.text,
      );
      // Drop the cached account so the settings account section reflects the
      // new session without waiting for its next read.
      ref.invalidate(weknoraAccountProvider);
      unawaited(_hydrateSessionsInBackground(hydrateSessions));
      if (mounted) context.go(Routes.chat);
    } on WeKnoraAuthException catch (error) {
      if (mounted) {
        setState(() {
          // Branch on the typed flags, never on message strings: messages
          // come from the server and are not localized or trusted.
          _error = error.invalidCredentials
              ? l10n.weknoraLoginInvalidCredentials
              : l10n.weknoraLoginUnreachable;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() => _error = l10n.weknoraLoginUnreachable);
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Refreshes the server-side WeKnora session list in the background so the
  /// conversation drawer is populated on arrival. Failures are logged, never
  /// surfaced: the user is already signed in at this point.
  Future<void> _hydrateSessionsInBackground(
    Future<void> Function() hydrateSessions,
  ) async {
    try {
      await hydrateSessions();
    } catch (error, stackTrace) {
      DebugLogger.error(
        'post-login-session-hydrate-failed',
        scope: 'weknora/login',
        error: error,
        stackTrace: stackTrace,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return UtilityPageScaffold.auth(
      title: l10n.weknoraLoginTitle,
      backNavigation: UtilityBackNavigation(
        label: l10n.back,
        buttonKey: const ValueKey<String>('weknora-login-back-button'),
        onPressed: () => context.go(Routes.backendChooser),
      ),
      bottomAction: ConduitButton(
        key: const Key('weknora-login-button'),
        text: l10n.weknoraLoginButton,
        onPressed: _canSubmit ? _submit : null,
        isLoading: _busy,
        isFullWidth: true,
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          InsetGroupedSection(
            flat: true,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                AccessibleFormField(
                  key: const Key('weknora-server-field'),
                  label: l10n.weknoraLoginServerField,
                  hint: 'http://localhost:8084',
                  controller: _server,
                  keyboardType: TextInputType.url,
                  textInputAction: TextInputAction.next,
                  autocorrect: false,
                  onChanged: _onFieldChanged,
                  isRequired: true,
                  autofillHints: const [AutofillHints.url],
                ),
                const SizedBox(height: Spacing.md),
                AccessibleFormField(
                  key: const Key('weknora-email-field'),
                  label: l10n.weknoraLoginEmailField,
                  controller: _email,
                  keyboardType: TextInputType.emailAddress,
                  textInputAction: TextInputAction.next,
                  autocorrect: false,
                  onChanged: _onFieldChanged,
                  isRequired: true,
                  autofillHints: const [AutofillHints.email],
                ),
                const SizedBox(height: Spacing.md),
                AccessibleFormField(
                  key: const Key('weknora-password-field'),
                  label: l10n.weknoraLoginPasswordField,
                  controller: _password,
                  obscureText: true,
                  keyboardType: TextInputType.visiblePassword,
                  textInputAction: TextInputAction.done,
                  autocorrect: false,
                  onChanged: _onFieldChanged,
                  onSubmitted: (_) => _submit(),
                  isRequired: true,
                  autofillHints: const [AutofillHints.password],
                ),
              ],
            ),
          ),
          if (_error != null) ...[
            const SizedBox(height: Spacing.md),
            ConnectionAttemptBanner(
              state: ConnectionAttemptState.failed(_error!),
            ),
          ],
        ],
      ),
    );
  }
}
