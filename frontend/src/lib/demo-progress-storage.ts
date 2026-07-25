"use client";

import { ApiError, apiPost } from "@/lib/api-client";

/**
 * Guest demo answers held until the visitor signs in.
 *
 * There is no dedicated POST /demo/migrate (or guest-progress ingest) API.
 * Anonymous continuity is localStorage across reloads; on login we migrate what
 * is feasible — incorrect answers → POST me/saved bookmarks — then clear.
 * Correct-only progress has no server home outside a real session.
 */
export const DEMO_PROGRESS_STORAGE_KEY = "drivergo:demo-progress";

export const DEMO_PROGRESS_VERSION = 1 as const;

export interface DemoProgressAnswer {
  questionId: string;
  answerId: string;
  correct: boolean;
  answeredAt: string;
}

export interface DemoProgress {
  version: typeof DEMO_PROGRESS_VERSION;
  answers: DemoProgressAnswer[];
}

function emptyProgress(): DemoProgress {
  return { version: DEMO_PROGRESS_VERSION, answers: [] };
}

function parseProgress(raw: string | null): DemoProgress {
  if (!raw) return emptyProgress();
  try {
    const parsed = JSON.parse(raw) as Partial<DemoProgress>;
    if (parsed.version !== DEMO_PROGRESS_VERSION || !Array.isArray(parsed.answers)) {
      return emptyProgress();
    }
    const answers: DemoProgressAnswer[] = [];
    for (const item of parsed.answers) {
      if (
        !item ||
        typeof item.questionId !== "string" ||
        typeof item.answerId !== "string" ||
        typeof item.correct !== "boolean" ||
        typeof item.answeredAt !== "string"
      ) {
        continue;
      }
      answers.push({
        questionId: item.questionId,
        answerId: item.answerId,
        correct: item.correct,
        answeredAt: item.answeredAt,
      });
    }
    return { version: DEMO_PROGRESS_VERSION, answers };
  } catch {
    return emptyProgress();
  }
}

export function readDemoProgress(): DemoProgress {
  if (typeof window === "undefined") return emptyProgress();
  try {
    return parseProgress(window.localStorage.getItem(DEMO_PROGRESS_STORAGE_KEY));
  } catch {
    return emptyProgress();
  }
}

export function writeDemoProgress(progress: DemoProgress): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(DEMO_PROGRESS_STORAGE_KEY, JSON.stringify(progress));
  } catch {
    // Private browsing / storage disabled: guest progress is lost on reload.
  }
}

export function clearDemoProgress(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(DEMO_PROGRESS_STORAGE_KEY);
  } catch {
    // ignore
  }
}

/**
 * Upserts one graded demo answer. Re-answering the same question replaces the
 * prior entry so the count stays honest.
 */
export function recordDemoAnswer(entry: {
  questionId: string;
  answerId: string;
  correct: boolean;
  answeredAt?: string;
}): DemoProgress {
  const next: DemoProgressAnswer = {
    questionId: entry.questionId,
    answerId: entry.answerId,
    correct: entry.correct,
    answeredAt: entry.answeredAt ?? new Date().toISOString(),
  };
  const current = readDemoProgress();
  const without = current.answers.filter((a) => a.questionId !== next.questionId);
  const progress: DemoProgress = {
    version: DEMO_PROGRESS_VERSION,
    answers: [...without, next],
  };
  writeDemoProgress(progress);
  return progress;
}

export function demoProgressCount(progress: DemoProgress = readDemoProgress()): number {
  return progress.answers.length;
}

/**
 * Best-effort migration after OTP verify / authenticated load.
 *
 * Safe to call repeatedly: no stored progress → no-op. Incorrect demo questions
 * are bookmarked via me/saved (idempotent). Transient failures keep storage so
 * the next authenticated page can retry; after a successful pass (or nothing
 * left to migrate) we clear-and-ack.
 */
export async function migrateDemoProgressOnLogin(): Promise<void> {
  if (typeof window === "undefined") return;
  const progress = readDemoProgress();
  if (progress.answers.length === 0) return;

  const incorrect = progress.answers.filter((a) => !a.correct);
  try {
    for (const answer of incorrect) {
      await apiPost<{ ok: boolean }>("me/saved", { question_id: answer.questionId });
    }
    // Ack: no full demo→account session ingest exists; bookmarks are the only
    // feasible server write. Clear so we do not re-POST forever.
    clearDemoProgress();
  } catch (err) {
    if (err instanceof ApiError && (err.status === 400 || err.status === 404)) {
      // Unusable question ids — drop local copy rather than retry forever.
      clearDemoProgress();
      return;
    }
    // Network / 401 / 5xx: keep local progress for a later authenticated retry.
  }
}
