import { useQuery } from "@tanstack/react-query";

import { apiGet } from "./client";
import type {
  FitnessMetricsList,
  RecoveryContext,
  TrainingContext,
} from "./types";

// Garmin sync lands data ~daily, so we don't poll aggressively. The query
// client (see queryClient.ts) revalidates on window focus; here we add a slow
// background interval as a backstop so a dashboard left open overnight refreshes
// without a manual reload.
const SLOW_INTERVAL_MS = 5 * 60 * 1000;

export function useTrainingContext() {
  return useQuery({
    queryKey: ["context", "training"],
    queryFn: () => apiGet<TrainingContext>("/context/training"),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

export function useRecoveryContext() {
  return useQuery({
    queryKey: ["context", "recovery"],
    queryFn: () => apiGet<RecoveryContext>("/context/recovery"),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// daysAgo returns a YYYY-MM-DD string n days before today, in local time.
function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().slice(0, 10);
}

// The acute/chronic load trend reads the fitness-metrics history window (an
// existing read endpoint; the training context only carries the latest
// snapshot). 42 days covers the chronic-load (28d) baseline plus context.
export function useFitnessTrend(windowDays = 42) {
  const from = daysAgo(windowDays);
  const to = daysAgo(0);
  return useQuery({
    queryKey: ["fitness-metrics", from, to],
    queryFn: () =>
      apiGet<FitnessMetricsList>(`/fitness-metrics?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}
