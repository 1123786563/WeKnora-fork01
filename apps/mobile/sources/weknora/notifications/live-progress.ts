/**
 * W31 — background run progress presentation.
 *
 * This module is a pure subscription surface over Run STATUS hints:
 * - It holds no worker: there is no execute/run/start port, nothing that
 *   keeps executing a task, and no background-audio keep-alive (the app
 *   deliberately does not use a background audio mode to keep the run
 *   alive — the run lives server-side, the client only watches).
 * - It never renders conversation content: the notification body is chosen
 *   from a fixed label map keyed by the Run status alone. Message text,
 *   tool payloads and knowledge results structurally cannot reach this
 *   surface.
 * - iOS Live Activity is the preferred surface when ActivityKit is
 *   available; when it is missing (or fails mid-update) the same hint
 *   degrades to a plain notification, so the foreground resume path (the
 *   W12 recovery pass, which owns reconnection to the run) stays available.
 *
 * Native capability provenance (declaration-only, per the W27 source
 * manifest precedent): see `docs/migrations/aws-mobile/source-manifest.json`
 * — the W31 addendum records the AWS sample capability reference and the
 * concrete native dependencies (ActivityKit/WidgetKit on iOS,
 * NotificationCompat progress on Android via expo-notifications). No AWS
 * repository byte is vendored by this module.
 */

/** Status-only hint; the label comes from the fixed map, never from content. */
export interface RunProgressHint {
  runID: string;
  label: string;
}

export interface ProgressNotificationRequest {
  runID: string;
  title: string;
  body: string;
}

export interface LiveProgressPorts {
  /** ActivityKit availability probe (iOS 16.1+); false on Android/older iOS. */
  isLiveActivityAvailable(): boolean;
  /** Live Activity update; absent when the native module is not linked. */
  setLiveActivity?(hint: RunProgressHint): Promise<void>;
  /** Dismisses the Live Activity for a finished run. */
  clearLiveActivity?(runID: string): Promise<void>;
  /** The plain-notification fallback (always present). */
  notify(request: ProgressNotificationRequest): Promise<void>;
}

export interface RunProgressPresenter {
  /** The single entry: one Run status change. Duplicates present once. */
  onRunStatus(runID: string, status: string): Promise<void>;
  /** Clears every surface for the run (terminal or user-dismissed). */
  clear(runID: string): Promise<void>;
  /** The last presented hint, or null once cleared. */
  lastHint(): RunProgressHint | null;
}

/** Fixed status labels — the only strings the background surface may show. */
const STATUS_LABELS: Record<string, string> = {
  pending: '排队中',
  dispatching: '排队中',
  admitted: '运行中',
  running: '运行中',
  active: '运行中',
  succeeded: '已完成',
  failed: '已失败',
  canceled: '已取消',
};

const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'canceled']);

/** Maps a Run status onto its fixed, content-free label. */
export function runProgressStatusLabel(status: string): string {
  return STATUS_LABELS[status.trim().toLowerCase()] ?? '状态更新';
}

function notificationTitle(status: string): string {
  if (status === 'succeeded') return '任务已完成';
  if (status === 'failed') return '任务已失败';
  if (status === 'canceled') return '任务已取消';
  return '任务进行中';
}

/**
 * Builds the presenter. Live Activity first, plain notification as the
 * always-available fallback; every failure of the activity surface degrades
 * instead of surfacing an error to the user.
 */
export function createRunProgressPresenter(ports: LiveProgressPorts): RunProgressPresenter {
  let last: RunProgressHint | null = null;
  const shown = new Map<string, string>();

  const presentViaNotification = async (runID: string, status: string): Promise<void> => {
    await ports.notify({ runID, title: notificationTitle(status), body: `任务${runProgressStatusLabel(status)}` });
  };

  const present = async (runID: string, status: string): Promise<void> => {
    const label = runProgressStatusLabel(status);
    last = { runID, label };
    const useLiveActivity = ports.isLiveActivityAvailable() && typeof ports.setLiveActivity === 'function';
    if (useLiveActivity) {
      try {
        await ports.setLiveActivity!({ runID, label });
        return;
      } catch {
        // ActivityKit refused (missing entitlement, stale activity): degrade
        // to the plain notification rather than losing the hint.
      }
    }
    await presentViaNotification(runID, status).catch(() => undefined);
  };

  return {
    async onRunStatus(runID: string, status: string): Promise<void> {
      if (shown.get(runID) === status) return;
      shown.set(runID, status);
      if (TERMINAL_STATUSES.has(status.trim().toLowerCase())) {
        // Terminal: dismiss the activity and let one plain notification
        // carry the final state; nothing stays resident for a finished run.
        await ports.clearLiveActivity?.(runID).catch(() => undefined);
        await presentViaNotification(runID, status).catch(() => undefined);
        last = null;
        shown.delete(runID);
        return;
      }
      await present(runID, status);
    },
    async clear(runID: string): Promise<void> {
      shown.delete(runID);
      await ports.clearLiveActivity?.(runID).catch(() => undefined);
      last = null;
    },
    lastHint: () => last,
  };
}
