import { apiPost } from "./api-client";

const MAX_BATCH_SIZE = 100;
const MAX_QUEUE_CAPACITY = 1_000;
const MAX_EVENT_NAME_LENGTH = 64;
const MAX_STRING_PROP_LENGTH = 128;

const SAFE_PROP_KEYS = [
  "category_code",
  "correct",
  "count",
  "duration_ms",
  "error_code",
  "locale",
  "mode",
  "plan_code",
  "position",
  "question_id",
  "score",
  "screen",
  "session_id",
  "sign_code",
  "source",
  "status",
  "stopped_reason",
  "total",
  "variant_number",
] as const;

const SAFE_PROP_KEY_SET: ReadonlySet<string> = new Set(SAFE_PROP_KEYS);
const EVENT_NAME_PATTERN = /^[a-z][a-z0-9_]*$/;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const BEARER_PATTERN = /^bearer\s+/i;
const JWT_PATTERN = /^eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+$/;

export type AnalyticsScalar = string | number | boolean | null;
export type SafeAnalyticsPropKey = (typeof SAFE_PROP_KEYS)[number];
export type SafeAnalyticsProps = Partial<Record<SafeAnalyticsPropKey, AnalyticsScalar>>;

export interface ClientAnalyticsEvent {
  name: string;
  props?: SafeAnalyticsProps;
  ts: string;
}

export interface AnalyticsBatchPayload {
  idempotency_key: string;
  events: ClientAnalyticsEvent[];
}

export interface AnalyticsBatchResponse {
  ok: boolean;
  count: number;
}

export type AnalyticsSend = (payload: AnalyticsBatchPayload) => Promise<void>;

export interface AnalyticsEventQueueOptions {
  send?: AnalyticsSend;
  now?: () => Date;
  flushAt?: number;
  maxQueueSize?: number;
  flushIntervalMs?: number;
  retryBaseMs?: number;
  retryMaxMs?: number;
  lifecycle?: boolean;
}

export interface AnalyticsEventQueue {
  readonly size: number;
  track(name: string, props?: SafeAnalyticsProps): void;
  flush(): Promise<void>;
  dispose(): void;
}

const defaultSend: AnalyticsSend = async (payload) => {
  await apiPost<AnalyticsBatchResponse, AnalyticsBatchPayload>("events", payload);
};

function boundedPositiveInteger(value: number | undefined, fallback: number, maximum: number): number {
  if (value === undefined || !Number.isFinite(value) || value <= 0) {
    return fallback;
  }

  return Math.min(Math.floor(value), maximum);
}

function isLikelyRawPii(value: string): boolean {
  if (EMAIL_PATTERN.test(value) || BEARER_PATTERN.test(value) || JWT_PATTERN.test(value)) {
    return true;
  }

  const phoneCandidate = value.replace(/[\s()+.-]/g, "");
  return /^\d{7,15}$/.test(phoneCandidate);
}

function sanitizeProps(props: SafeAnalyticsProps | undefined): SafeAnalyticsProps | undefined {
  if (!props || typeof props !== "object" || Array.isArray(props)) {
    return undefined;
  }

  const safeProps: SafeAnalyticsProps = {};

  for (const [key, value] of Object.entries(props)) {
    if (!SAFE_PROP_KEY_SET.has(key)) {
      continue;
    }

    if (typeof value === "string") {
      if (value.length === 0 || value.length > MAX_STRING_PROP_LENGTH || isLikelyRawPii(value)) {
        continue;
      }
      safeProps[key as SafeAnalyticsPropKey] = value;
      continue;
    }

    if (typeof value === "number") {
      if (Number.isFinite(value)) {
        safeProps[key as SafeAnalyticsPropKey] = value;
      }
      continue;
    }

    if (typeof value === "boolean" || value === null) {
      safeProps[key as SafeAnalyticsPropKey] = value;
    }
  }

  return Object.keys(safeProps).length > 0 ? safeProps : undefined;
}

class InMemoryAnalyticsEventQueue implements AnalyticsEventQueue {
  private readonly send: AnalyticsSend;
  private readonly now: () => Date;
  private readonly flushAt: number;
  private readonly maxQueueSize: number;
  private readonly flushIntervalMs: number;
  private readonly retryBaseMs: number;
  private readonly retryMaxMs: number;
  private pending: ClientAnalyticsEvent[] = [];
  private inFlight: ClientAnalyticsEvent[] | null = null;
  private activeSend: Promise<void> | null = null;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private nextRetryMs: number;
  private retryIdempotencyKey: string | null = null;
  private disposed = false;
  private lifecycleDocument: Document | null = null;
  private lifecycleWindow: Window | null = null;

  private readonly handleVisibilityChange = (): void => {
    if (this.lifecycleDocument?.visibilityState === "hidden") {
      void this.flush();
    }
  };

  private readonly handlePageHide = (): void => {
    void this.flush();
  };

  constructor(options: AnalyticsEventQueueOptions) {
    this.send = options.send ?? defaultSend;
    this.now = options.now ?? (() => new Date());
    this.maxQueueSize = boundedPositiveInteger(options.maxQueueSize, 200, MAX_QUEUE_CAPACITY);
    this.flushAt = boundedPositiveInteger(options.flushAt, 20, MAX_QUEUE_CAPACITY);
    this.flushIntervalMs = boundedPositiveInteger(options.flushIntervalMs, 10_000, 24 * 60 * 60 * 1_000);
    this.retryBaseMs = boundedPositiveInteger(options.retryBaseMs, 2_000, 24 * 60 * 60 * 1_000);
    this.retryMaxMs = Math.max(
      this.retryBaseMs,
      boundedPositiveInteger(options.retryMaxMs, 60_000, 24 * 60 * 60 * 1_000)
    );
    this.nextRetryMs = this.retryBaseMs;

    if (options.lifecycle !== false) {
      if (typeof document !== "undefined") {
        this.lifecycleDocument = document;
        document.addEventListener("visibilitychange", this.handleVisibilityChange);
      }
      if (typeof window !== "undefined") {
        this.lifecycleWindow = window;
        window.addEventListener("pagehide", this.handlePageHide);
      }
    }
  }

  get size(): number {
    return this.pending.length + (this.inFlight?.length ?? 0);
  }

  track(name: string, props?: SafeAnalyticsProps): void {
    if (this.disposed || !this.isValidEventName(name)) {
      return;
    }

    const timestamp = this.now();
    if (!(timestamp instanceof Date) || !Number.isFinite(timestamp.getTime())) {
      return;
    }

    if (this.size >= this.maxQueueSize) {
      if (this.pending.length === 0) {
        return;
      }
      this.pending.shift();
    }

    const safeProps = sanitizeProps(props);
    this.pending.push({
      name,
      ...(safeProps ? { props: safeProps } : {}),
      ts: timestamp.toISOString(),
    });

    if (!this.activeSend && this.pending.length >= this.flushAt) {
      this.clearTimer();
      void this.beginSend();
      return;
    }

    if (!this.activeSend) {
      this.schedule(this.flushIntervalMs);
    }
  }

  async flush(): Promise<void> {
    if (this.disposed) {
      return;
    }

    this.clearTimer();
    await this.beginSend();
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }

    this.disposed = true;
    this.clearTimer();
    this.lifecycleDocument?.removeEventListener("visibilitychange", this.handleVisibilityChange);
    this.lifecycleWindow?.removeEventListener("pagehide", this.handlePageHide);
    this.lifecycleDocument = null;
    this.lifecycleWindow = null;
  }

  private isValidEventName(name: string): boolean {
    return (
      name.length > 0 &&
      name.length <= MAX_EVENT_NAME_LENGTH &&
      EVENT_NAME_PATTERN.test(name)
    );
  }

  private beginSend(): Promise<void> {
    if (this.disposed || this.pending.length === 0) {
      return this.activeSend ?? Promise.resolve();
    }
    if (this.activeSend) {
      return this.activeSend;
    }

    const batch = this.pending.splice(0, MAX_BATCH_SIZE);
    this.inFlight = batch;
    const idempotencyKey = this.retryIdempotencyKey ?? globalThis.crypto.randomUUID();
    this.retryIdempotencyKey = null;
    const operation = this.performSend(batch, idempotencyKey);
    this.activeSend = operation;
    return operation;
  }

  private async performSend(batch: ClientAnalyticsEvent[], idempotencyKey: string): Promise<void> {
    let succeeded = false;

    try {
      // Keep tracking synchronous and cheap even if a custom sender does work
      // before returning its promise.
      await Promise.resolve();
      await this.send({ idempotency_key: idempotencyKey, events: batch });
      succeeded = true;
    } catch {
      // Analytics is best-effort. Failed events are restored and retried later.
    }

    this.inFlight = null;
    this.activeSend = null;

    if (this.disposed) {
      return;
    }

    if (!succeeded) {
      this.retryIdempotencyKey = idempotencyKey;
      this.pending = batch.concat(this.pending).slice(0, this.maxQueueSize);
      const retryAfterMs = this.nextRetryMs;
      this.nextRetryMs = Math.min(this.nextRetryMs * 2, this.retryMaxMs);
      this.schedule(retryAfterMs);
      return;
    }

    this.nextRetryMs = this.retryBaseMs;
    if (this.pending.length >= this.flushAt) {
      void this.beginSend();
    } else if (this.pending.length > 0) {
      this.schedule(this.flushIntervalMs);
    }
  }

  private schedule(delayMs: number): void {
    if (this.disposed || this.pending.length === 0 || this.timer) {
      return;
    }

    this.timer = setTimeout(() => {
      this.timer = null;
      void this.beginSend();
    }, delayMs);
  }

  private clearTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}

export function createAnalyticsEventQueue(
  options: AnalyticsEventQueueOptions = {}
): AnalyticsEventQueue {
  return new InMemoryAnalyticsEventQueue(options);
}

let defaultQueue: AnalyticsEventQueue | null = null;

function getDefaultQueue(): AnalyticsEventQueue {
  defaultQueue ??= createAnalyticsEventQueue();
  return defaultQueue;
}

export function trackEvent(name: string, props?: SafeAnalyticsProps): void {
  getDefaultQueue().track(name, props);
}

export function flushEvents(): void {
  void getDefaultQueue().flush();
}
