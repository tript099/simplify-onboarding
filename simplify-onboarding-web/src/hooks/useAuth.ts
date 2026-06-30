import { useQuery, useQueryClient } from "@tanstack/react-query";
import { me, type SessionUser } from "@/lib/api";

export const AUTH_QUERY_KEY = ["me"] as const;

/** Current authenticated user (null when signed out). */
export function useAuth() {
  const query = useQuery({
    queryKey: AUTH_QUERY_KEY,
    queryFn: me,
    staleTime: 30_000,
    retry: false,
    // Re-fetch periodically so the central (persist) session is healed back to
    // no-expiry — a product on the shared Redis (DocFlow) re-arms a sliding TTL on
    // it, and each /me strips that TTL. 5 min is well under DocFlow's 30-min window.
    refetchInterval: 5 * 60_000,
    refetchOnWindowFocus: true,
  });
  return {
    user: (query.data ?? null) as SessionUser | null,
    isLoading: query.isLoading,
    isAuthenticated: !!query.data,
  };
}

/** Imperatively refresh the auth state (call after login / verify / logout). */
export function useRefreshAuth() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
}
