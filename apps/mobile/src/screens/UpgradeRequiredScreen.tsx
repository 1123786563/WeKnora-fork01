import { Button, Text, View } from 'react-native';
import type { RuntimeReason } from '@weknora/mobile-core';

export interface UpgradeRequiredScreenProps {
  deploymentLabel?: string;
  reason?: RuntimeReason;
  onSignOut(): Promise<void>;
}

/** A deliberately restricted surface: no authorized workspace controls are rendered here. */
export function UpgradeRequiredScreen({ deploymentLabel, reason, onSignOut }: UpgradeRequiredScreenProps) {
  return (
    <View>
      <Text>Update required</Text>
      <Text>{deploymentLabel ? `${deploymentLabel} cannot open this version of WeKnora.` : 'This deployment cannot open this version of WeKnora.'}</Text>
      <Text>{reason === 'tenant-required' ? 'Choose an active tenant in the web application, then sign in again.' : 'Update the app or contact the deployment administrator.'}</Text>
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
    </View>
  );
}
