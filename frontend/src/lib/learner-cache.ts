"use client";

import type { QueryClient } from "@tanstack/react-query";
import { clearSessionHistoryCache } from "@/hooks/use-session-history";
import { clearSessionQuestionWarmCache } from "@/hooks/use-session-engine";
import { notificationUnreadCount, supportUnreadCount } from "@/lib/badge-counts";
import { mistakesCountStore, mockEligibilityStore, savedQuestionsStore } from "@/lib/dashboard-stores";
import { getQueryClient } from "@/lib/query-client";

/**
 * Clears all per-learner in-memory caches and React Query caches.
 * Must be called on logout, session expiry, forced password change sign-out,
 * and Telegram account switch so a subsequent learner on the same browser tab
 * never inherits stale user state.
 */
export function resetAllLearnerCaches(queryClient?: QueryClient): void {
  if (queryClient) {
    queryClient.clear();
  }
  getQueryClient().clear();
  clearSessionHistoryCache();
  clearSessionQuestionWarmCache();
  mistakesCountStore.reset();
  savedQuestionsStore.reset();
  mockEligibilityStore.reset();
  notificationUnreadCount.reset();
  supportUnreadCount.reset();
}
