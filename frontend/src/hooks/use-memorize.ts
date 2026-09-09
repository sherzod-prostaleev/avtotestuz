"use client";

import { useCallback, useEffect, useState } from "react";
import { apiGet } from "@/lib/api-client";
import {
  toQuestionItem,
  toSessionError,
  type QuestionDetailResponse,
  type SessionError,
  type SessionQuestionItem,
} from "@/hooks/use-session-engine";

interface UseMemorizeResult {
  questions: SessionQuestionItem[];
  loading: boolean;
  error: SessionError | null;
}

/**
 * One VIP-gated topic, read-only and fully disclosed — see
 * GET /categories/{code}/memorize. This is not a session: nothing here is
 * answered, scored, or scheduled, so it shares only the one conversion
 * function with useSessionEngine, not the engine itself.
 */
export function useMemorize(categoryCode: string, locale: string): UseMemorizeResult {
  const [questions, setQuestions] = useState<SessionQuestionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<SessionError | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const path = `categories/${encodeURIComponent(categoryCode)}/memorize?locale=${encodeURIComponent(locale)}`;
      const batch = await apiGet<QuestionDetailResponse[]>(path);
      setQuestions(batch.map((detail) => toQuestionItem(detail)));
    } catch (err) {
      setError(toSessionError(err));
      setQuestions([]);
    } finally {
      setLoading(false);
    }
  }, [categoryCode, locale]);

  useEffect(() => {
    void load();
  }, [load]);

  return { questions, loading, error };
}
