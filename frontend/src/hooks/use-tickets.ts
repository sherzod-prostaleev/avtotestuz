import { useState, useEffect, useCallback } from "react";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";

export type TicketLockReason = "vip_required" | "prev_required";

export interface TicketStatus {
  number: number;
  total_questions: number;
  status: "unstarted" | "in_progress" | "completed" | "locked";
  best_correct: number;
  attempts: number;
  unlocked: boolean;
  lock_reason?: TicketLockReason;
  completed_at?: string;
  /** Compatibility field for existing consumers; real responses use best_correct. */
  score?: number;
}

export type TicketItem = TicketStatus;

interface VariantStatusDTO {
  number: number;
  question_count: number;
  unlocked: boolean;
  lock_reason?: TicketLockReason;
  best_correct: number;
  attempts: number;
  completed_at?: string;
}

function toTicketStatus(variant: VariantStatusDTO): TicketStatus {
  const status: TicketStatus["status"] = !variant.unlocked
    ? "locked"
    : variant.completed_at
      ? "completed"
      : variant.attempts > 0
        ? "in_progress"
        : "unstarted";

  return {
    number: variant.number,
    total_questions: variant.question_count,
    status,
    best_correct: variant.best_correct,
    attempts: variant.attempts,
    unlocked: variant.unlocked,
    ...(variant.lock_reason ? { lock_reason: variant.lock_reason } : {}),
    ...(variant.completed_at ? { completed_at: variant.completed_at } : {}),
  };
}

interface VariantResetDTO {
  cleared: number;
  unlock_ceiling: number;
}

export function useTickets() {
  const [tickets, setTickets] = useState<TicketStatus[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [clearing, setClearing] = useState<boolean>(false);
  const [clearError, setClearError] = useState<string | null>(null);
  const [clearedCount, setClearedCount] = useState<number | null>(null);

  const fetchTickets = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiGet<VariantStatusDTO[]>("me/variants");
      setTickets(data.map(toTicketStatus));
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to load tickets");
      }
    } finally {
      setLoading(false);
    }
  }, []);

  // clearProgress backs the "Tozalash" control. The grid is re-read from the
  // server afterwards rather than zeroed locally: the response says how many
  // rows went, but only the server can say which bilets are open now, and
  // that is precisely the part a learner would notice being wrong.
  const clearProgress = useCallback(async (): Promise<boolean> => {
    setClearing(true);
    setClearError(null);
    setClearedCount(null);
    try {
      const res = await apiPost<VariantResetDTO>("me/variants/reset");
      await fetchTickets();
      setClearedCount(res.cleared);
      return true;
    } catch (err: unknown) {
      setClearError(err instanceof ApiError ? err.message : "Failed to clear ticket progress");
      return false;
    } finally {
      setClearing(false);
    }
  }, [fetchTickets]);

  const dismissClearNotice = useCallback(() => {
    setClearedCount(null);
    setClearError(null);
  }, []);

  useEffect(() => {
    fetchTickets();
  }, [fetchTickets]);

  return {
    tickets,
    loading,
    error,
    refetch: fetchTickets,
    clearProgress,
    clearing,
    clearError,
    /** Rows the last clear removed; null until one succeeds. */
    clearedCount,
    dismissClearNotice,
  };
}
