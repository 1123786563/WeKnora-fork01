/**
 * W31 F-3 (review round 1) — the production LiveProgressPorts adapter over
 * expo-notifications (the dependency already ships in apps/mobile; W25
 * picker-adapter precedent).
 *
 * Scope, per the review ruling and the manifest addendum:
 * - The Live Activity native module is NOT linked in this app (declaration
 *   only in docs/migrations/aws-mobile/source-manifest.json), so
 *   `isLiveActivityAvailable()` is honestly false and `setLiveActivity`
 *   stays absent — the presenter degrades to plain notifications.
 * - Plain notifications are real: `notify` posts a local notification with
 *   status-only strings (title/body from the presenter's fixed map plus the
 *   runID). A new status for the same run REPLACES the previous progress
 *   notification instead of stacking.
 * - Permission is read, never requested here (no surprise prompt
 *   mid-conversation; requesting is an app-shell concern). A missing grant
 *   posts nothing — no fake notification surface; the foreground execution
 *   notices (W31) and the W12 recovery pass remain the truthful surfaces.
 * - The expo module is loaded lazily so harness tests inject a structural
 *   double and node:test never touches the native import.
 */
import type { LiveProgressPorts } from './live-progress';

/** Structural subset of expo-notifications this port consumes. */
export interface ExpoNotificationsModule {
  getPermissionsAsync(): Promise<{ granted: boolean }>;
  scheduleNotificationAsync(request: {
    content: { title: string; body: string; data?: Record<string, unknown> };
    trigger: null;
  }): Promise<string>;
  dismissNotificationAsync(identifier: string): Promise<void>;
}

export interface NativeProgressPortInput {
  /** Injectable module seam for harness tests; defaults to lazy expo-notifications. */
  notifications?: ExpoNotificationsModule;
}

export function createExpoNotificationsProgressPort(input: NativeProgressPortInput = {}): LiveProgressPorts {
  let modulePromise: Promise<ExpoNotificationsModule> | null = null;
  const load = (): Promise<ExpoNotificationsModule> => (
    input.notifications ? Promise.resolve(input.notifications) : (modulePromise ??= import('expo-notifications') as Promise<ExpoNotificationsModule>)
  );
  /** runID -> id of the last posted progress notification. */
  const posted = new Map<string, string>();

  return {
    // The Live Activity seam stays declaration-only (manifest addendum);
    // the presenter degrades to the plain notifications below.
    isLiveActivityAvailable: () => false,
    async notify(request): Promise<void> {
      try {
        const notifications = await load();
        const status = await notifications.getPermissionsAsync();
        if (!status.granted) return;
        const previous = posted.get(request.runID);
        if (previous !== undefined) {
          posted.delete(request.runID);
          await notifications.dismissNotificationAsync(previous).catch(() => undefined);
        }
        const id = await notifications.scheduleNotificationAsync({
          content: { title: request.title, body: request.body, data: { runID: request.runID } },
          trigger: null,
        });
        posted.set(request.runID, id);
      } catch {
        // A failed post never rejects the presenter path: the foreground
        // surfaces stay truthful, the run itself is server-side.
      }
    },
    async clearLiveActivity(runID): Promise<void> {
      const id = posted.get(runID);
      if (id === undefined) return;
      posted.delete(runID);
      try {
        const notifications = await load();
        await notifications.dismissNotificationAsync(id);
      } catch {
        // Idempotent clear: an already-gone notification is cleared.
      }
    },
  };
}
