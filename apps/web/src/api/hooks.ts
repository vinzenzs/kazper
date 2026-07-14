import { useQuery } from "@tanstack/react-query";

import { apiGet } from "./client";
import type {
  AchievementsList,
  FitnessMetricsList,
  GearList,
  PersonalRecordsList,
  IntensityDistribution,
  RecoveryContext,
  TrainingContext,
  PMCSeries,
  TargetTrajectory,
  PowerCurve,
  CPModelResult,
  PowerProfileResult,
  DurabilityResult,
  IntervalsResult,
  QuadrantResult,
  WPrimeBalanceResult,
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

export function usePMC(from: string, to: string) {
  return useQuery({
    queryKey: ["pmc", from, to],
    queryFn: () => apiGet<PMCSeries>(`/performance/pmc?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// The planned-vs-actual CTL trajectory for the active macrocycle. 404s
// (no active macrocycle) surface as an error the PMC panel treats as "no
// overlay"; retry is off so a persistent 404 doesn't spin.
export function useTargetTrajectory() {
  return useQuery({
    queryKey: ["pmc-target-trajectory"],
    queryFn: () =>
      apiGet<TargetTrajectory>(`/performance/pmc/target-trajectory`),
    refetchInterval: SLOW_INTERVAL_MS,
    retry: false,
  });
}

// The critical-power (CP2) model fitted over the window's power best-efforts.
export function useCPModel(from: string, to: string) {
  return useQuery({
    queryKey: ["cp-model", from, to],
    queryFn: () => apiGet<CPModelResult>(`/workouts/cp-model?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// The Coggan power-profile ranking over the window. No weight_kg is sent — the
// endpoint uses the latest stored body weight (or 400 weight_data_missing, which
// surfaces as an error the panel degrades on); sex defaults to male server-side.
export function usePowerProfile(from: string, to: string) {
  return useQuery({
    queryKey: ["power-profile", from, to],
    queryFn: () =>
      apiGet<PowerProfileResult>(`/workouts/power-profile?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
    retry: false,
  });
}

// Detected work intervals for a workout's stored power stream. A 404 (no
// power/no streams) or a `no_distinct_efforts` result both leave the detail-page
// table absent; retry is off so a persistent 404 doesn't spin.
export function useDetectedIntervals(workoutId: string | undefined) {
  return useQuery({
    queryKey: ["detected-intervals", workoutId],
    enabled: !!workoutId,
    queryFn: () => apiGet<IntervalsResult>(`/workouts/${workoutId}/intervals`),
    refetchInterval: SLOW_INTERVAL_MS,
    retry: false,
  });
}

// Per-workout force/velocity quadrant analysis. Disabled until the CP param is
// known (from the cp-model fit); pivot cadence is the UI constant 90 rpm. The
// full scatter is fetched for the chart. A 404 (no cadence stream on pre-bridge
// rides) leaves the detail-page scatter absent.
export function useQuadrant(
  workoutId: string | undefined,
  cpWatts: number | undefined,
  cadenceRpm = 90,
) {
  const enabled = !!workoutId && !!cpWatts && cpWatts > 0;
  return useQuery({
    queryKey: ["quadrant", workoutId, cpWatts, cadenceRpm],
    enabled,
    queryFn: () =>
      apiGet<QuadrantResult>(
        `/workouts/${workoutId}/quadrant?cp_watts=${cpWatts}&cadence_rpm=${cadenceRpm}`,
      ),
    refetchInterval: SLOW_INTERVAL_MS,
    retry: false,
  });
}

// Per-workout W′ balance over the stored power stream. Disabled until the CP/W′
// params are known (from the cp-model fit); the series is downsampled for the
// chart while the exact minimum stays in the summary.
export function useWPrimeBalance(
  workoutId: string | undefined,
  cpWatts: number | undefined,
  wPrimeKj: number | undefined,
) {
  const enabled = !!workoutId && !!cpWatts && cpWatts > 0 && !!wPrimeKj && wPrimeKj > 0;
  return useQuery({
    queryKey: ["w-prime-balance", workoutId, cpWatts, wPrimeKj],
    enabled,
    queryFn: () =>
      apiGet<WPrimeBalanceResult>(
        `/workouts/${workoutId}/w-prime-balance?cp_watts=${cpWatts}&w_prime_kj=${wPrimeKj}&downsample=200`,
      ),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

// The durability (fatigue-resistance) fade table over the window. Empty
// durations / a no_tiered_data reason render the panel's explanatory empty state.
export function useDurability(from: string, to: string) {
  return useQuery({
    queryKey: ["durability", from, to],
    queryFn: () => apiGet<DurabilityResult>(`/workouts/durability?from=${from}&to=${to}`),
    refetchInterval: SLOW_INTERVAL_MS,
  });
}

export function useIntensityDistribution(from: string, to: string) {
  return useQuery({
    queryKey: ["intensity-distribution", from, to],
    queryFn: () =>
      apiGet<IntensityDistribution>(
        `/workouts/intensity-distribution?from=${from}&to=${to}`,
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
