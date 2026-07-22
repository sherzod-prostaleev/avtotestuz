import { useState, useEffect, useCallback } from "react";
import { apiGet, ApiError } from "@/lib/api-client";
import { fullRoadSignsCatalog } from "@/lib/signs-data";

export interface RoadSign {
  id: string;
  code: string;
  group_code: string;
  group_name: string;
  name: string;
  description: string;
  image_url: string;
}

export type SignItem = RoadSign;

export function useSigns(groupFilter?: string, searchQuery?: string) {
  const [signs, setSigns] = useState<RoadSign[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const fetchSigns = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (groupFilter && groupFilter !== "all") params.append("group", groupFilter);
      if (searchQuery) params.append("q", searchQuery);

      const queryString = params.toString();
      const path = queryString ? `signs?${queryString}` : "signs";

      let data: RoadSign[] | null = null;
      try {
        data = await apiGet<RoadSign[]>(path);
      } catch {
        // Fallback to local catalog if backend DB signs table is unpopulated
        data = null;
      }

      let resultList = data && data.length > 0 ? data : fullRoadSignsCatalog;

      if (groupFilter && groupFilter !== "all") {
        resultList = resultList.filter((s) => s.group_code === groupFilter);
      }
      if (searchQuery) {
        const q = searchQuery.toLowerCase();
        resultList = resultList.filter(
          (s) => s.name.toLowerCase().includes(q) || s.code.toLowerCase().includes(q)
        );
      }

      setSigns(resultList);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to load road signs");
      }
    } finally {
      setLoading(false);
    }
  }, [groupFilter, searchQuery]);

  useEffect(() => {
    fetchSigns();
  }, [fetchSigns]);

  return {
    signs,
    loading,
    error,
    refetch: fetchSigns,
  };
}
