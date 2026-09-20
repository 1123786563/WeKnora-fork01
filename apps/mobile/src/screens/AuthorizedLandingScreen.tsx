import { Button, Text, View } from 'react-native';

export interface AuthorizedLandingScreenProps {
  deploymentLabel: string;
  userId: string;
  tenantId: string;
  onSignOut(): Promise<void>;
}

/** Placeholder handoff point for the Task Office Module; it intentionally makes no Task API call. */
export function AuthorizedLandingScreen({ deploymentLabel, userId, tenantId, onSignOut }: AuthorizedLandingScreenProps) {
  return (
    <View>
      <Text>WeKnora Task Office</Text>
      <Text>{deploymentLabel}</Text>
      <Text>{`Signed in as ${userId} for tenant ${tenantId}`}</Text>
      <Text>The Task Office will appear here.</Text>
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
    </View>
  );
}
