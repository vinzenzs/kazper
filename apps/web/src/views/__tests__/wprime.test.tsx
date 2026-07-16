import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import {
  populatedAchievements,
  populatedGear,
  populatedRecords,
  populatedRecovery,
  populatedTraining,
  populatedTrend,
  populatedWorkoutDetail,
} from "../../test/fixtures";
import type {
  CPModelResult,
  WPrimeBalanceResult,
  IntervalsResult,
} from "../../api/types";

// The workout-detail W′-balance strip: present only when the critical-power fit
// yields cp/W′ AND the workout has a power stream. These tests drive the two
// gating branches — a live fit + series (strip renders, summary shown) and a
// null fit (strip absent) — through the routed detail view. Hooks are mocked
// per-test with vi.doMock + a fresh dynamic import so each branch is isolated.

const ok = <T,>(data: T) => ({ data, isLoading: false, isError: false, error: null });

const cpFit: CPModelResult = {
  from: "2026-04-15",
  to: "2026-07-14",
  tz: "UTC",
  model: { cp_watts: 262, w_prime_kj: 21.5, r_squared: 0.98, rmse_w: 4.2 },
  points: [],
};

const wpResult: WPrimeBalanceResult = {
  workout_id: "w1",
  params: { cp_watts: 262, w_prime_kj: 21.5 },
  duration_s: 300,
  summary: {
    min_w_prime_kj: 3.4,
    min_at_s: 180,
    end_w_prime_kj: 8.1,
    max_depletion_pct: 84.2,
    time_below_25_pct_s: 45,
  },
  downsample: 200,
  series: [21.5, 18.2, 12.7, 7.9, 5.1, 3.4, 5.0, 6.6, 8.1],
};

const detectedIntervals: IntervalsResult = {
  workout_id: "w1",
  threshold_w: 210,
  intervals: [
    { n: 1, start_s: 120, end_s: 360, duration_s: 240, avg_w: 305, max_w: 320, kj: 73.2 },
    { n: 2, start_s: 480, end_s: 720, duration_s: 240, avg_w: 298, max_w: 315, kj: 71.5 },
  ],
  rests: [{ after_n: 1, duration_s: 120, avg_w: 120 }],
  summary: { count: 2, work_total_s: 480, mean_effort_s: 240, mean_effort_w: 301 },
};

function mockHooks(over: Record<string, unknown>) {
  vi.doMock("../../api/hooks", () => ({
    useTrainingContext: () => ok(populatedTraining),
    useRecoveryContext: () => ok(populatedRecovery),
    useFitnessTrend: () => ok({ fitness_metrics: populatedTrend }),
    usePersonalRecords: () => ok({ personal_records: populatedRecords }),
    useAchievements: () => ok({ achievements: populatedAchievements }),
    useGear: () => ok({ gear: populatedGear }),
    useWorkout: () => ok(populatedWorkoutDetail),
    useCPModel: () => ok(null),
    useWPrimeBalance: () => ok(undefined),
    useDetectedIntervals: () => ok(undefined),
    useQuadrant: () => ok(undefined),
    useStride: () => ok(undefined),
    ...over,
  }));
}

async function renderDetail() {
  const { AppRoutes } = await import("../../App");
  return render(
    <MemoryRouter initialEntries={["/workouts/w1"]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

describe("workout-detail W′ balance strip", () => {
  it("renders the strip and summary when a CP fit and series are present", async () => {
    vi.resetModules();
    mockHooks({
      useCPModel: () => ok(cpFit),
      useWPrimeBalance: () => ok(wpResult),
    });
    await renderDetail();
    expect(screen.getByText("W′ balance")).toBeInTheDocument();
    expect(screen.getByText("Min W′")).toBeInTheDocument();
    // Summary readouts: min kJ and depletion %.
    expect(screen.getByText("3.4 kJ")).toBeInTheDocument();
    expect(screen.getByText("84%")).toBeInTheDocument();
    expect(screen.getByTestId("wprime-min-marker")).toBeInTheDocument();
  });

  it("omits the strip when there is no CP fit", async () => {
    vi.resetModules();
    mockHooks({
      useCPModel: () => ok(null),
      useWPrimeBalance: () => ok(undefined),
    });
    await renderDetail();
    // The detail view still renders (title present) but no W′bal panel.
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.queryByText("W′ balance")).not.toBeInTheDocument();
  });

  it("renders the detected-intervals table when efforts are found", async () => {
    vi.resetModules();
    mockHooks({ useDetectedIntervals: () => ok(detectedIntervals) });
    await renderDetail();
    expect(screen.getByText("Detected intervals")).toBeInTheDocument();
    expect(screen.getByTestId("intervals-table")).toBeInTheDocument();
    // Two effort rows + the header row.
    expect(screen.getByTestId("intervals-table").querySelectorAll("tbody tr")).toHaveLength(2);
    expect(screen.getByText("threshold 210 W")).toBeInTheDocument();
  });

  it("omits the intervals table on a no_distinct_efforts result", async () => {
    vi.resetModules();
    mockHooks({
      useDetectedIntervals: () =>
        ok({
          workout_id: "w1",
          threshold_w: null,
          intervals: [],
          rests: [],
          reason: "no_distinct_efforts",
          summary: { count: 0, work_total_s: 0, mean_effort_s: 0, mean_effort_w: 0 },
        }),
    });
    await renderDetail();
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.queryByText("Detected intervals")).not.toBeInTheDocument();
  });

  it("renders the quadrant scatter when power+cadence and a CP fit are present", async () => {
    vi.resetModules();
    mockHooks({
      useQuadrant: () =>
        ok({
          workout_id: "w1",
          params: { cp_watts: 262, cadence_rpm: 90, crank_mm: 172.5 },
          summary: {
            q1_pct: 10, q2_pct: 55, q3_pct: 20, q4_pct: 15,
            pedaling_s: 3200, excluded_s: 400, aepf_ref_n: 168.4, cpv_ref_mps: 1.56,
          },
          scatter: [
            { aepf_n: 200, cpv_mps: 1.2 },
            { aepf_n: 150, cpv_mps: 1.8 },
          ],
        }),
    });
    await renderDetail();
    expect(screen.getByText("Quadrant analysis")).toBeInTheDocument();
    expect(screen.getByTestId("quadrant-shares")).toBeInTheDocument();
    expect(screen.getByText("55.0%")).toBeInTheDocument(); // Q2 grinding share
  });

  it("omits the quadrant panel without a fit / cadence stream", async () => {
    vi.resetModules();
    mockHooks({ useQuadrant: () => ok(undefined) });
    await renderDetail();
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.queryByText("Quadrant analysis")).not.toBeInTheDocument();
  });
});
