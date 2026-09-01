import { QueryClient } from "@tanstack/react-query";

let browserQueryClient: QueryClient | undefined = undefined;

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        refetchOnWindowFocus: false,
        retry: false,
      },
    },
  });
}

/**
 * One client per browser tab, a fresh one per server render.
 *
 * The server branch is not optional: a module-level client on the server is
 * shared by every concurrent request, so one visitor's cached answers would
 * be served to the next.
 *
 * In the browser the singleton is what makes a language switch cheap.
 * `Providers` is mounted inside [locale]/layout.tsx, and changing the locale
 * segment remounts that subtree — with a client held in `useState` the whole
 * cache went with it and every panel refetched from scratch.
 */
export function getQueryClient(): QueryClient {
  if (typeof window === "undefined") {
    return createQueryClient();
  }
  if (!browserQueryClient) {
    browserQueryClient = createQueryClient();
  }
  return browserQueryClient;
}
