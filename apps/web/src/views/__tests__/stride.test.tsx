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
import type { StrideResult } from "../../api/types";

// The workout-detail cadence-vs-stride view: present for runs with usable bins,
// absent for anything else. The steady-run branch matters most — the view must
// EXPLAIN why there's no split rather than vanish or render an empty chart.
// Hooks are mocked per-test with vi.doMock + a fresh dynamic import.

const ok = <T,>(data: T) => ({ data, isLoading: false, isError: false, error: null });

const strideResult: StrideResult = {
  workout_id: "w1",
  bins: [
    { speed_low_mps: 2.5, speed_high_mps: 2.75, seconds: 300, cadence_spm: 158.2, step_length_m: 0.99 },
    { speed_low_mps: 3.0, speed_high_mps: 3.25, seconds: 420, cadence_spm: 167.4, step_length_m: 1.12 },
    { speed_low_mps: 3.75, speed_high_mps: 4.0, seconds: 180, cadence_spm: 176.1, step_length_m: 1.32 },
    { speed_low_mps: 4.5, speed_high_mps: 4.75, seconds: 90, cadence_spm: 184.0, step_length_m: 1.51 },
  ],
  contribution: { cadence_contribution_pct: 31.4, step_contribution_pct: 68.6 },
  reason: null,
  analyzed_s: 990,
  excluded_s: 45,
};

const steadyResult: StrideResult = {
  workout_id: "w1",
  bins: [
    { speed_low_mps: 3.0, speed_high_mps: 3.25, seconds: 1800, cadence_spm: 168.0, step_length_m: 1.11 },
  ],
  contribution: null,
  reason: "insufficient_speed_range",
  analyzed_s: 1800,
  excluded_s: 12,
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

describe("workout-detail cadence-vs-stride view", () => {
  it("renders the bins and the contribution split for a run with pace variety", async () => {
    vi.resetModules();
    mockHooks({ useStride: () => ok(strideResult) });
    await renderDetail();

    expect(screen.getByText("Cadence vs stride")).toBeInTheDocument();
    expect(screen.getByTestId("stride-view")).toBeInTheDocument();
    expect(screen.getByTestId("stride-split")).toBeInTheDocument();
    // The split is shown decomposed, both halves.
    expect(screen.getByText("31.4%")).toBeInTheDocument();
    expect(screen.getByText("68.6%")).toBeInTheDocument();
    expect(screen.getByText("from turnover")).toBeInTheDocument();
    expect(screen.getByText("from step length")).toBeInTheDocument();
  });

  it("explains itself on a steady run instead of showing an empty split", async () => {
    vi.resetModules();
    mockHooks({ useStride: () => ok(steadyResult) });
    await renderDetail();

    expect(screen.getByText("Cadence vs stride")).toBeInTheDocument();
    // The reason is rendered; no split block.
    expect(screen.getByTestId("stride-reason")).toBeInTheDocument();
    expect(screen.getByTestId("stride-reason").textContent).toMatch(/pace variety/i);
    expect(screen.queryByTestId("stride-split")).not.toBeInTheDocument();
  });

  it("omits the view when the run has no cadence stream", async () => {
    vi.resetModules();
    // A 404 leaves the hook's data undefined.
    mockHooks({ useStride: () => ok(undefined) });
    await renderDetail();

    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument(); // page still renders
    expect(screen.queryByText("Cadence vs stride")).not.toBeInTheDocument();
  });

  it("omits the view when the response carries no bins", async () => {
    vi.resetModules();
    mockHooks({
      useStride: () =>
        ok({ ...steadyResult, bins: [], contribution: null, reason: "insufficient_speed_range" }),
    });
    await renderDetail();

    expect(screen.queryByText("Cadence vs stride")).not.toBeInTheDocument();
  });
});
