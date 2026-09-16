import * as React from 'react';
import { View } from 'react-native';
import { MainView } from '@/components/MainView';
import { PendingNotificationCard } from '@/weknora/notifications/NotificationRouter';

/** Product workbench shell around the retained Happy session list. */
export function WorkbenchScreen() {
  return (
    <View style={{ flex: 1 }}>
      <PendingNotificationCard />
      <MainView variant="phone" />
    </View>
  );
}

export default WorkbenchScreen;
