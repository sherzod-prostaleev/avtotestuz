import { useState, useEffect, useCallback } from "react";
import { apiGet, ApiError } from "@/lib/api-client";

export interface TicketStatus {
  id: number | string;
  number: number;
  total_questions: number;
  status: "unstarted" | "in_progress" | "completed" | "locked";
  score?: number | null;
  passed?: boolean | null;
  best_correct?: number;
  attempts?: number;
  unlocked?: boolean;
  answered_count?: number;
}

export type TicketItem = TicketStatus;

export function useTickets() {
  const [tickets, setTickets] = useState<TicketStatus[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const fetchTickets = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiGet<TicketStatus[]>("me/variants");
      setTickets(data ?? []);
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
