import {
  buildKnowledgeTimeline,
  shouldPollKnowledgeSpans,
  type KnowledgeSpansView,
  type KnowledgeTimelineStep,
} from '@weknora/domain/knowledge/processing';

// Minimal port of the polling core of Vue knowledge-processing-timeline.vue:
// Vue fetches immediately, then keeps ONE interval for the component's whole
// lifetime. The tick callback decides whether to fetch (strict mode — poll
// only while parse_status itself is non-terminal); only explicit cleanup
// clears the interval.

export interface ProcessingTimelineOptions {
  documentId: string;
  getSpans: (documentId: string) => Promise<KnowledgeSpansView>;
  onUpdate?: (steps: KnowledgeTimelineStep[], spans: KnowledgeSpansView) => void;
  onStop?: (spans: KnowledgeSpansView | undefined) => void;
  onError?: (error: unknown) => void;
  intervalMs?: number;
  timer?: {
    setInterval: (handler: () => void, ms: number) => unknown;
    clearInterval: (handle: unknown) => void;
  };
}

export interface ProcessingTimelineSubscription {
  stop: () => void;
  readonly stopped: boolean;
}

export function startProcessingTimeline(options: ProcessingTimelineOptions): ProcessingTimelineSubscription {
  const intervalMs = options.intervalMs ?? 2000;
  const timer = options.timer ?? {
    setInterval: (handler: () => void, ms: number) => window.setInterval(handler, ms),
    clearInterval: (handle: unknown) => window.clearInterval(handle as number),
  };
  let stopped = false;
  let inFlight = false;
  let lastSpans: KnowledgeSpansView | undefined;
  let terminalReported = false;

  async function tick(): Promise<void> {
    if (stopped || inFlight) return;
    if (lastSpans && !shouldPollKnowledgeSpans(typeof lastSpans.parse_status === 'string' ? lastSpans.parse_status : undefined)) return;
    inFlight = true;
    try {
      const spans = await options.getSpans(options.documentId);
      if (stopped) return;
      lastSpans = spans;
      options.onUpdate?.(buildKnowledgeTimeline(spans), spans);
      if (!shouldPollKnowledgeSpans(typeof spans.parse_status === 'string' ? spans.parse_status : undefined)) {
        if (!terminalReported) {
          terminalReported = true;
          options.onStop?.(spans);
        }
      }
    } catch (error) {
      if (!stopped) options.onError?.(error);
    } finally {
      inFlight = false;
    }
  }

  const handle = timer.setInterval(() => tick(), intervalMs);
  void tick();

  function stop(): void {
    if (stopped) return;
    stopped = true;
    timer.clearInterval(handle);
  }

  return {
    stop,
    get stopped() {
      return stopped;
    },
  };
}
