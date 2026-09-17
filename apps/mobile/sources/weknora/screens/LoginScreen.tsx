import React, { useState } from 'react';
import { Linking, ScrollView, StyleSheet, Text, View } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import { createOIDCApi } from '@weknora/api-client';
import { useProductAuth } from '../auth/session.tsx';
import { useMobileHost } from '../platform/host.ts';
import { validateLoginCredentials } from '../auth/loginValidation.ts';
import { generatePKCE } from '@/utils/oauth';
import { Button } from '../ui/Button.tsx';
import { Field } from '../ui/Field.tsx';
import { StateView } from '../ui/StateView.tsx';
import { useWeknoraTheme } from '../ui/theme.ts';

/**
 * 产品登录页 M01（MX-010）。视觉走设计令牌组件；
 * 真实命令：密码登录=useProductAuth().login（真实 /auth/login）；SSO=原生 OIDC startNative（PKCE+state 落 SecureStore）。
 * 不模拟成功：失败即错误态（StateView error），无假延迟/Toast。
 */
const NATIVE_PKCE_VERIFIER_KEY = 'weknora:native-oidc:pkce-verifier';
const NATIVE_OIDC_STATE_KEY = 'weknora:native-oidc:state';
const AUTH_RETURN_REDIRECT = 'weknora://auth-return';

export default function LoginScreen({ onAuthenticated }: { onAuthenticated?: () => void }) {
  const host = useMobileHost();
  const auth = useProductAuth();
  const { theme } = useWeknoraTheme();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [ssoBusy, setSsoBusy] = useState(false);

  const submit = async () => {
    if (!host || !email.trim() || !password) {
      setError('请输入邮箱与密码');
      return;
    }
    const validation = validateLoginCredentials(email, password);
    const validationError = validation.email ?? validation.password;
    if (validationError) {
      setError(validationError);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await auth.login(email.trim(), password);
      onAuthenticated?.();
    } catch {
      setError('登录失败，请检查邮箱与密码后重试');
    } finally {
      setPassword('');
      setBusy(false);
    }
  };

  const startNativeOIDC = async () => {
    if (!host) return;
    setSsoBusy(true);
    setError(null);
    try {
      const { verifier, challenge } = await generatePKCE();
      const api = createOIDCApi(async (request) => {
        const response = await fetch(`${host.origin}${request.path}`, {
          method: request.method,
          headers: { 'content-type': 'application/json', ...(request.headers ?? {}) },
          ...(request.body === undefined ? {} : { body: JSON.stringify(request.body) }),
        });
        return (await response.json()) as unknown;
      });
      const state = await api.startNative(AUTH_RETURN_REDIRECT, challenge);
      await SecureStore.setItemAsync(NATIVE_PKCE_VERIFIER_KEY, verifier);
      await SecureStore.setItemAsync(NATIVE_OIDC_STATE_KEY, JSON.stringify(state));
      if (!state.authorization_url) throw new Error('authorization_url missing');
      await Linking.openURL(state.authorization_url);
    } catch {
      setError('无法开始单点登录，请稍后重试或使用邮箱登录');
    } finally {
      setSsoBusy(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={[styles.container, { backgroundColor: theme.colors.bg }]} keyboardShouldPersistTaps="handled">
      <View style={styles.header}>
        <Text style={{ color: theme.colors['hero-ink'], fontSize: theme.typography.display.fontSize, lineHeight: theme.typography.display.lineHeight, fontWeight: '700' }}>WeKnora</Text>
        <Text style={{ color: theme.colors.muted, fontSize: theme.typography.body.fontSize, lineHeight: theme.typography.body.lineHeight, marginTop: theme.spacing[8] }}>
          移动 AI 工作台
        </Text>
      </View>
      <View style={[styles.card, { backgroundColor: theme.colors.surface, borderRadius: theme.radius.card, borderColor: theme.colors.line, padding: theme.spacing[20] }]}>
        {auth.bootstrapError ? (
          <View style={{ marginBottom: theme.spacing[12] }}>
            <StateView kind="error" message="身份恢复失败" detail={auth.bootstrapError} actionLabel="重试" onAction={() => void auth.bootstrapScope()} />
          </View>
        ) : null}
        <Field label="邮箱" value={email} onChangeText={setEmail} placeholder="name@example.com" error={error && !password ? error : undefined} />
        <Field label="密码" value={password} onChangeText={setPassword} placeholder="输入密码" secureTextEntry />
        {error && password ? (
          <Text accessibilityRole="alert" style={{ color: theme.colors.danger, fontSize: theme.typography['body-sm'].fontSize, lineHeight: theme.typography['body-sm'].lineHeight }}>
            {error}
          </Text>
        ) : null}
        <Button label="登录" onPress={() => void submit()} loading={busy} accessibilityLabel="使用邮箱密码登录" />
        <Button label="企业单点登录" variant="secondary" onPress={() => void startNativeOIDC()} loading={ssoBusy} accessibilityLabel="使用企业单点登录" />
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flexGrow: 1, justifyContent: 'center', gap: 24 },
  header: { alignItems: 'center' },
  card: { borderWidth: StyleSheet.hairlineWidth, gap: 16 },
});
