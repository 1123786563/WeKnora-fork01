import { Button, Text, View } from 'react-native';
import { router } from 'expo-router';

export interface TenantOptionView {
  id: string;
  name?: string;
  active: boolean;
}

export interface AuthorizedLandingScreenProps {
  deploymentLabel: string;
  userId: string;
  tenantId: string;
  tenants: TenantOptionView[];
  onSignOut(): Promise<void>;
  onActivateTenant(tenantId: string): Promise<void>;
}

/** Placeholder handoff point for the Task Office Module; tenant switching stays a pure Runtime callback. */
export function AuthorizedLandingScreen({ deploymentLabel, userId, tenantId, tenants, onSignOut, onActivateTenant }: AuthorizedLandingScreenProps) {
  return (
    <View>
      <Text>WeKnora Task Office</Text>
      <Text>{deploymentLabel}</Text>
      <Text>{`Signed in as ${userId} for tenant ${tenantId}`}</Text>
      <Text>The Task Office will appear here.</Text>
      {tenants.map((tenant) => tenant.active
        ? <Text key={tenant.id}>{`${tenant.name ?? tenant.id} (active)`}</Text>
        : <Button key={tenant.id} title={`Switch to ${tenant.name ?? tenant.id}`} onPress={() => { void onActivateTenant(tenant.id); }} />)}
      <Button title="Open Resources" onPress={() => { router.navigate('/resources'); }} />
      <Button title="Sign out" onPress={() => { void onSignOut(); }} />
    </View>
  );
}
