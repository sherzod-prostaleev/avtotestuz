import { describe, it, expect, vi, beforeEach } from "vitest";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";
import {
  DEMO_PROGRESS_STORAGE_KEY,
  clearDemoProgress,
  demoProgressCount,
  migrateDemoProgressOnLogin,
  readDemoProgress,
  recordDemoAnswer,
} from "./demo-progress-storage";

describe("demo-progress-storage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
  });

  it("starts empty", () => {
    expect(readDemoProgress()).toEqual({ version: 1, answers: [] });
    expect(demoProgressCount()).toBe(0);
  });

  it("records and upserts answers under drivergo:demo-progress", () => {
    recordDemoAnswer({
      questionId: "q1",
      answerId: "a1",
      correct: false,
      answeredAt: "2026-07-25T10:00:00.000Z",
    });
    recordDemoAnswer({
      questionId: "q2",
      answerId: "a2",
      correct: true,
      answeredAt: "2026-07-25T10:01:00.000Z",
    });
    recordDemoAnswer({
      questionId: "q1",
      answerId: "a3",
      correct: true,
      answeredAt: "2026-07-25T10:02:00.000Z",
    });

    const progress = readDemoProgress();
    expect(progress.answers).toHaveLength(2);
    expect(progress.answers.find((a) => a.questionId === "q1")).toEqual({
      questionId: "q1",
      answerId: "a3",
      correct: true,
      answeredAt: "2026-07-25T10:02:00.000Z",
    });
    expect(window.localStorage.getItem(DEMO_PROGRESS_STORAGE_KEY)).toContain("q2");
    expect(demoProgressCount()).toBe(2);
  });

  it("ignores corrupt storage payloads", () => {
    window.localStorage.setItem(DEMO_PROGRESS_STORAGE_KEY, "{not-json");
    expect(readDemoProgress()).toEqual({ version: 1, answers: [] });

    window.localStorage.setItem(
      DEMO_PROGRESS_STORAGE_KEY,
      JSON.stringify({
        version: 1,
        answers: [
          { questionId: 1, answerId: "a", correct: true, answeredAt: "x" },
          {
            questionId: "ok",
            answerId: "a1",
            correct: false,
            answeredAt: "2026-07-25T00:00:00.000Z",
          },
        ],
      })
    );
    expect(readDemoProgress().answers).toEqual([
      {
        questionId: "ok",
        answerId: "a1",
        correct: false,
        answeredAt: "2026-07-25T00:00:00.000Z",
      },
    ]);
  });

  it("clears stored progress", () => {
    recordDemoAnswer({ questionId: "q1", answerId: "a1", correct: true });
    clearDemoProgress();
    expect(window.localStorage.getItem(DEMO_PROGRESS_STORAGE_KEY)).toBeNull();
  });

  it("does nothing when no progress is stored", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true });
    await migrateDemoProgressOnLogin();
    expect(post).not.toHaveBeenCalled();
  });

  it("bookmarks incorrect answers then clears progress", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true });
    recordDemoAnswer({ questionId: "wrong-1", answerId: "a1", correct: false });
    recordDemoAnswer({ questionId: "right-1", answerId: "a2", correct: true });

    await migrateDemoProgressOnLogin();

    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith("me/saved", { question_id: "wrong-1" });
    expect(readDemoProgress().answers).toHaveLength(0);
  });

  it("clears correct-only progress without posting", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true });
    recordDemoAnswer({ questionId: "right-1", answerId: "a2", correct: true });

    await migrateDemoProgressOnLogin();

    expect(post).not.toHaveBeenCalled();
    expect(readDemoProgress().answers).toHaveLength(0);
  });

  it("keeps progress after a transient failure so a later load can retry", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new Error("network down"));
    recordDemoAnswer({ questionId: "wrong-1", answerId: "a1", correct: false });

    await migrateDemoProgressOnLogin();

    expect(demoProgressCount()).toBe(1);
  });

  it("drops progress on definitive client errors", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("gone", "not_found", 404)
    );
    recordDemoAnswer({ questionId: "wrong-1", answerId: "a1", correct: false });

    await migrateDemoProgressOnLogin();

    expect(window.localStorage.getItem(DEMO_PROGRESS_STORAGE_KEY)).toBeNull();
  });
});
