import { useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';

export interface DeploymentLoginScreenProps {
  officialCloudOrigin?: string;
  deployments?: Array<{ origin: string; label: string }>;
  onSignIn(input: { origin: string; email: string; password: string }): Promise<void>;
  onBeginOidc(input: { origin: string }): Promise<void>;
  onSwitchDeployment?(origin: string): Promise<void>;
}

/** Accepts a deployment only when it can safely become the Runtime origin. */
export function validatedDeploymentOrigin(value: string): string | undefined {
  try {
    const parsed = new URL(value.trim());
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== '/') return undefined;
    return parsed.origin;
  } catch {
    return undefined;
  }
}

/** Collects deployment and credentials, then delegates every auth decision to Runtime. */
export function DeploymentLoginScreen({ officialCloudOrigin, deployments, onSignIn, onBeginOidc, onSwitchDeployment }: DeploymentLoginScreenProps) {
  const [origin, setOrigin] = useState(officialCloudOrigin ?? '');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | undefined>();
  const deploymentOrigin = validatedDeploymentOrigin(origin);

  const requireDeployment = (): string | undefined => {
    if (deploymentOrigin) return deploymentOrigin;
    setError('Enter an HTTPS deployment origin without credentials or a path.');
    return undefined;
  };

  // 切换目标实例（B2-F10）：表单先对齐目标 origin 并清空凭据——无凭据时 runtime 会回到
  // deployment-login 面，组件实例复用而表单已指向目标，Sign in 不会登录旧 origin。
  const switchTo = (deployment: { origin: string }): void => {
    setOrigin(deployment.origin);
    setEmail('');
    setPassword('');
    setError(undefined);
    void onSwitchDeployment?.(deployment.origin);
  };

  return (
    <View>
      <Text>Sign in to WeKnora</Text>
      {deployments && deployments.length > 0 && onSwitchDeployment ? (
        <View>
          <Text>Registered deployments</Text>
          {deployments.map((deployment) => (
            <Button key={deployment.origin} title={deployment.label} onPress={() => { switchTo(deployment); }} />
          ))}
        </View>
      ) : null}
      {officialCloudOrigin ? <Button title="Use WeKnora Cloud" onPress={() => { setOrigin(officialCloudOrigin); setError(undefined); }} /> : null}
      <TextInput value={origin} onChangeText={(value) => { setOrigin(value); setError(undefined); }} placeholder="https://weknora.example.com" autoCapitalize="none" />
      <TextInput value={email} onChangeText={setEmail} placeholder="Email" autoCapitalize="none" keyboardType="email-address" />
      <TextInput value={password} onChangeText={setPassword} placeholder="Password" secureTextEntry />
      {error ? <Text accessibilityRole="alert">{error}</Text> : null}
      <Button title="Sign in" onPress={() => { const safeOrigin = requireDeployment(); if (safeOrigin) void onSignIn({ origin: safeOrigin, email, password }); }} />
      <Button title="Continue with single sign-on" onPress={() => { const safeOrigin = requireDeployment(); if (safeOrigin) void onBeginOidc({ origin: safeOrigin }); }} />
    </View>
  );
}
