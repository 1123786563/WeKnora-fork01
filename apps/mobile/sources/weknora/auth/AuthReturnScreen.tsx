import * as React from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Stack, useRouter, useLocalSearchParams } from 'expo-router';
import { AUTH_RETURN_REDIRECT, evaluateAuthReturn, type AuthReturnOutcome } from './auth-return-status';

const COPY: Record<AuthReturnOutcome['status'], { title: string; body: string }> = {
  verified: {
    title: 'Provider verified',
    body: 'Your provider signed you in, but this server cannot yet finish native token exchange. Continue with your email and password.',
  },
  missing_state: {
    title: 'Incomplete sign-in link',
    body: 'The sign-in link is missing its one-time verification state. Start sign-in again.',
  },
  unknown_state: {
    title: 'Sign-in link not recognized',
    body: 'This link was not started by this app or has already been used. Start sign-in again.',
  },
  rejected: {
    title: 'Sign-in link rejected',
    body: 'The return link did not match this app exactly or carried credentials in the URL. It was ignored to protect your account.',
  },
};

/**
 * Terminal screen for the OIDC browser callback. It only verifies the return
 * link; it never reads tokens out of the URL and never changes stored
 * credentials, so a forged deep link cannot grant or steal an identity.
 */
export default function AuthReturnScreen() {
  const router = useRouter();
  const params: Record<string, string | string[] | undefined> = useLocalSearchParams();
  const evaluated = React.useRef<AuthReturnOutcome | null>(null);
  const [outcome, setOutcome] = React.useState<AuthReturnOutcome | null>(null);

  React.useEffect(() => {
    if (evaluated.current) return;
    evaluated.current = evaluateAuthReturn(params, AUTH_RETURN_REDIRECT);
    setOutcome(evaluated.current);
  }, [params]);

  const copy = outcome ? COPY[outcome.status] : null;

  return (
    <View style={styles.container}>
      <Stack.Screen options={{ title: 'Finishing sign-in' }} />
      <Text style={styles.title}>{copy?.title ?? 'Checking sign-in link'}</Text>
      <Text style={styles.description}>
        {copy?.body ?? 'Verifying the identity provider response…'}
      </Text>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Back to sign in"
        onPress={() => router.replace('/(app)/login')}
        style={styles.button}
      >
        <Text style={styles.buttonText}>Back to sign in</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, gap: 16, justifyContent: 'center', padding: 24 },
  title: { fontSize: 24, fontWeight: '700' },
  description: { color: '#666', fontSize: 14, lineHeight: 20 },
  button: { alignItems: 'center', backgroundColor: '#1f6feb', borderRadius: 8, padding: 14 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
