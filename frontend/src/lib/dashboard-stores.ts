"use client";

import { apiGet } from "@/lib/api-client";
import { createSharedStore } from "@/lib/shared-store";
import type { MockEligibilityResponse } from "@/components/mock/grand-mock-card";

export interface MistakesCountDTO {
  due_count: number;
}

export interface SavedItemDTO {
  question_id: string;
}

export const mockEligibilityStore = createSharedStore<MockEligibilityResponse | null>(
  async () => apiGet<MockEligibilityResponse>("me/mock-eligibility"),
  null,
  { ttlMs: 30_000 }
);

export const mistakesCountStore = createSharedStore<number>(
  async () => {
    const data = await apiGet<MistakesCountDTO>("me/mistakes");
    return Number.isInteger(data.due_count) && data.due_count >= 0 ? data.due_count : 0;
  },
  0,
  { ttlMs: 30_000 }
);

export const savedQuestionsStore = createSharedStore<Set<string>>(
  async () => {
    const items = await apiGet<SavedItemDTO[]>("me/saved");
    return new Set(Array.isArray(items) ? items.map((item) => item.question_id) : []);
  },
  new Set<string>(),
  { ttlMs: 30_000 }
);
