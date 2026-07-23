import { useState, useEffect, useCallback } from "react";
import { apiGet, ApiError } from "@/lib/api-client";

export interface TicketStatus {
  number: number;
  total_questions: number;
  status: "unstarted" | "in_progress" | "completed" | "locked";
  best_correct: number;
  attempts: number;
  unlocked: boolean;
  completed_at?: string;
  /** Compatibility field for existing consumers; real responses use best_correct. */
  score?: number;
}

export type TicketItem = TicketStatus;

interface VariantStatusDTO {
  number: number;
  question_count: number;
  unlocked: boolean;
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
    ...(variant.completed_at ? { completed_at: variant.completed_at } : {}),
  };
}

export function useTickets() {
  const [tickets, setTickets] = useState<TicketStatus[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

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

  useEffect(() => {
    fetchTickets();
  }, [fetchTickets]);

  return {
    tickets,
    loading,
    error,
    refetch: fetchTickets,
  };
}
