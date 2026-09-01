import { describe, it, expect, vi, beforeEach } from "vitest";
import * as apiClient from "@/lib/api-client";
import {
  mockEligibilityStore,
  mistakesCountStore,
  savedQuestionsStore,
} from "@/lib/dashboard-stores";

vi.mock("@/lib/api-client", () => ({
  apiGet: vi.fn(),
}));

describe("dashboard-stores", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEligibilityStore.reset();
    mistakesCountStore.reset();
    savedQuestionsStore.reset();
  });

  describe("mockEligibilityStore", () => {
    it("fetches and parses mock eligibility data", async () => {
      const mockData = {
        eligible: true,
        mastery_percent: 85,
        min_required_percent: 80,
        questions_studied: 1200,
        min_required_questions: 1000,
        is_vip: true,
        reason: null,
      };
      vi.mocked(apiClient.apiGet).mockResolvedValueOnce(mockData);

      const res = await mockEligibilityStore.load();
      expect(res).toEqual(mockData);
      expect(apiClient.apiGet).toHaveBeenCalledWith("me/mock-eligibility");
    });
  });

  describe("mistakesCountStore", () => {
    it("fetches and normalizes due_count", async () => {
      vi.mocked(apiClient.apiGet).mockResolvedValueOnce({ due_count: 5 });

      const count = await mistakesCountStore.load();
      expect(count).toBe(5);
      expect(apiClient.apiGet).toHaveBeenCalledWith("me/mistakes");
    });

    it("falls back to 0 on invalid due_count", async () => {
      vi.mocked(apiClient.apiGet).mockResolvedValueOnce({ due_count: -1 });

      const count = await mistakesCountStore.load();
      expect(count).toBe(0);
    });
  });

  describe("savedQuestionsStore", () => {
    it("fetches and converts saved items into a Set", async () => {
      vi.mocked(apiClient.apiGet).mockResolvedValueOnce([
        { question_id: "q-1" },
        { question_id: "q-2" },
      ]);

      const savedSet = await savedQuestionsStore.load();
      expect(savedSet.has("q-1")).toBe(true);
      expect(savedSet.has("q-2")).toBe(true);
      expect(savedSet.size).toBe(2);
      expect(apiClient.apiGet).toHaveBeenCalledWith("me/saved");
    });
  });
});
