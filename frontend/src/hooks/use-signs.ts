import { useCallback, useEffect, useRef, useState } from "react";
import { apiGet } from "@/lib/api-client";

export interface RoadSign {
  code: string;
  group_code: string;
  name: string;
  image_url: string | null;
  question_count: number;
}

export interface RoadSignDetail {
  code: string;
  group_code: string;
  name: string;
  description: string;
  image_url: string | null;
  question_ids: string[];
}

export type SignItem = RoadSign;
export type SignsLoadError = "unavailable";

function isRoadSign(value: unknown): value is RoadSign {
  if (!value || typeof value !== "object") return false;
  const sign = value as Partial<RoadSign>;
  return (
    typeof sign.code === "string" &&
    sign.code.length > 0 &&
    typeof sign.group_code === "string" &&
    sign.group_code.length > 0 &&
    typeof sign.name === "string" &&
    sign.name.length > 0 &&
    (sign.image_url === null || typeof sign.image_url === "string") &&
    Number.isInteger(sign.question_count) &&
    (sign.question_count ?? -1) >= 0
  );
}

function isRoadSignList(value: unknown): value is RoadSign[] {
  return Array.isArray(value) && value.every(isRoadSign);
}

function isRoadSignDetail(value: unknown): value is RoadSignDetail {
  if (!value || typeof value !== "object") return false;
  const sign = value as Partial<RoadSignDetail>;
  return (
    typeof sign.code === "string" &&
    sign.code.length > 0 &&
    typeof sign.group_code === "string" &&
    sign.group_code.length > 0 &&
    typeof sign.name === "string" &&
    sign.name.length > 0 &&
    typeof sign.description === "string" &&
    (sign.image_url === null || typeof sign.image_url === "string") &&
    Array.isArray(sign.question_ids) &&
    sign.question_ids.every((id) => typeof id === "string")
  );
}

export function useSigns(locale: string, groupFilter?: string, searchQuery?: string) {
  const [signs, setSigns] = useState<RoadSign[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<SignsLoadError | null>(null);
  const requestID = useRef(0);

  const fetchSigns = useCallback(async () => {
    const currentRequest = ++requestID.current;
    setLoading(true);
    setError(null);
    setSigns([]);

    try {
      const params = new URLSearchParams({ locale });
      if (groupFilter && groupFilter !== "all") params.set("group", groupFilter);

      const query = searchQuery?.trim();
      if (query) params.set("q", query);

      const data = await apiGet<unknown>(`signs?${params.toString()}`);
      if (!isRoadSignList(data)) throw new Error("Invalid signs response");

      if (currentRequest === requestID.current) setSigns(data);
    } catch {
      if (currentRequest === requestID.current) setError("unavailable");
    } finally {
      if (currentRequest === requestID.current) setLoading(false);
    }
  }, [groupFilter, locale, searchQuery]);

  useEffect(() => {
    void fetchSigns();
  }, [fetchSigns]);

  return { signs, loading, error, refetch: fetchSigns };
}

export function useSignDetail(code: string | null, locale: string) {
  const [sign, setSign] = useState<RoadSignDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<SignsLoadError | null>(null);
  const requestID = useRef(0);

  const fetchSign = useCallback(async () => {
    const currentRequest = ++requestID.current;
    setSign(null);
    setError(null);

    if (!code) {
      setLoading(false);
      return;
    }

    setLoading(true);
    try {
      const params = new URLSearchParams({ locale });
      const data = await apiGet<unknown>(
        `signs/${encodeURIComponent(code)}?${params.toString()}`
      );
      if (!isRoadSignDetail(data) || data.code !== code) {
        throw new Error("Invalid sign detail response");
      }

      if (currentRequest === requestID.current) setSign(data);
    } catch {
      if (currentRequest === requestID.current) setError("unavailable");
    } finally {
      if (currentRequest === requestID.current) setLoading(false);
    }
  }, [code, locale]);

  useEffect(() => {
    void fetchSign();
  }, [fetchSign]);

  return { sign, loading, error, refetch: fetchSign };
}
