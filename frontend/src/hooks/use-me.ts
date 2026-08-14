import { useQuery } from "@tanstack/react-query";
import { apiGet } from "@/lib/api-client";

export const ME_QUERY_KEY = ["me"] as const;

export type MeResponseDTO = {
  profile: {
    id: string;
    phone: string;
    name: string;
    region: string;
    district: string;
    birth_date: string | null;
    locale_pref: string;
    theme_pref: string;
    referral_code: string;
    role: string;
    created_at: string;
    must_change_password?: boolean;
    kind?: string;
  };
  vip: {
    active: boolean;
    until: string | null;
  };
};

export function useMeQuery() {
  return useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: ({ signal }) => apiGet<MeResponseDTO>("me", { signal }),
    staleTime: 30_000,
    retry: false,
  });
}
