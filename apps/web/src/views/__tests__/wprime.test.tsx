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
import type { CPModelResult, WPrimeBalanceResult } from "../../api/types";

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
});
