"use client";

import { ApiError, apiPost } from "@/lib/api-client";

/**
 * Guest demo answers held until the visitor signs in.
 *
 * On login, POST /me/demo-progress/migrate applies incorrect answers to the
 * mistake bank (FSRS Again). Correct answers are acknowledged but do not
 * inflate mastery / Grand Mock gates. Clear localStorage only after 2xx ack.
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
 * Safe to call repeatedly: no stored progress → no-op. Posts the full guest
 * payload to me/demo-progress/migrate (incorrect → mistakes/Again; correct
 * skipped server-side). Clear local only on 2xx; keep storage on transient
 * failures so DemoProgressCapture can retry.
 */
export async function migrateDemoProgressOnLogin(): Promise<void> {
  if (typeof window === "undefined") return;
  const progress = readDemoProgress();
  if (progress.answers.length === 0) return;

  try {
    await apiPost<{ migrated: number; skipped: number }>("me/demo-progress/migrate", {
      answers: progress.answers.map((a) => ({
        question_id: a.questionId,
        answer_id: a.answerId,
        correct: a.correct,
        answered_at: a.answeredAt,
      })),
    });
    clearDemoProgress();
  } catch (err) {
    if (err instanceof ApiError && (err.status === 400 || err.status === 404)) {
      clearDemoProgress();
      return;
    }
    // Network / 401 / 5xx: keep local progress for a later authenticated retry.
  }
}
