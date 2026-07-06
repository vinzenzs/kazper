import { useQuery } from "@tanstack/react-query";

import { apiGet } from "./client";
import type {
  AchievementsList,
  FitnessMetricsList,
  GearList,
  PersonalRecordsList,
  RecoveryContext,
  TrainingContext,
  PowerCurve,
  Workout,
  WorkoutStats,
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

// Personal records (best efforts), gear inventory, and achievements are
// slowly-changing Garmin mirrors — the same slow-revalidate policy as the
// context reads is plenty.
export function usePersonalRecords() {
  return useQuery({
    queryKey: ["personal-records"],
    queryFn: () => apiGet<PersonalRecordsList>("/personal-records"),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

export function useGear() {
  return useQuery({
    queryKey: ["gear"],
    queryFn: () => apiGet<GearList>("/gear"),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

export function useAchievements() {
  return useQuery({
    queryKey: ["achievements"],
    queryFn: () => apiGet<AchievementsList>("/achievements"),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// Workout volume totals over a date range — per-day series + window total.
// The StatsView picks from/to for the Week/Month/YTD toggle.
export function useWorkoutStats(from: string, to: string) {
  return useQuery({
    queryKey: ["workout-stats", from, to],
    queryFn: () =>
      apiGet<WorkoutStats>(`/workouts/summary?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// The mean-maximal power/pace curve over a window. `sport` selects the metric
// (bike → power, run/swim → pace).
export function usePowerCurve(from: string, to: string, sport: string) {
  return useQuery({
    queryKey: ["power-curve", from, to, sport],
    queryFn: () =>
      apiGet<PowerCurve>(
        `/workouts/power-curve?from=${from}&to=${to}&sport=${sport}`,
      ),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// A single workout's detail (splits, sets, HR-zone time) — the list-shaped
// context payloads omit these, so the detail route fetches by id on demand.
// `enabled` guards the missing-param case; the API returns 404 for an unknown id.
export function useWorkout(id: string | undefined) {
  return useQuery({
    queryKey: ["workout", id],
    queryFn: () => apiGet<Workout>(`/workouts/${id}`),
    enabled: !!id,
  });
}
