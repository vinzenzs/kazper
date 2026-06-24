import { QueryClient } from "@tanstack/react-query";

// Refetch policy (per the design's D5 / open question): revalidate-on-focus plus
// a slow background interval set per-query in hooks.ts. Data changes at most
// daily (Garmin sync), so staleTime is generous and retries are minimal — a
// failed read is more likely an auth/transport problem than a flake.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: true,
      staleTime: 60 * 1000,
      retry: 1,
    },
  },
});
