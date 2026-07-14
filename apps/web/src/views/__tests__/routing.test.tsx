import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { AppRoutes } from "../../App";
import {
  populatedAchievements,
  populatedGear,
  populatedRecords,
  populatedRecovery,
  populatedTraining,
  populatedTrend,
  populatedWorkoutDetail,
} from "../../test/fixtures";

// Mock the data layer so the routed views render synchronously without network.
// The routing behaviour — deep-link resolution and nav-link navigation — is what
// these tests exercise, not the fetching.
const ok = <T,>(data: T) => ({ data, isLoading: false, isError: false, error: null });

vi.mock("../../api/hooks", () => ({
  useTrainingContext: () => ok(populatedTraining),
  useRecoveryContext: () => ok(populatedRecovery),
  useFitnessTrend: () => ok({ fitness_metrics: populatedTrend }),
  usePersonalRecords: () => ok({ personal_records: populatedRecords }),
  useAchievements: () => ok({ achievements: populatedAchievements }),
  useGear: () => ok({ gear: populatedGear }),
  useWorkout: () => ok(populatedWorkoutDetail),
  // The workout-detail W′bal strip is gated on a critical-power fit; with no
  // fit the strip is absent, which is the default for these routing tests.
  useCPModel: () => ok(null),
  useWPrimeBalance: () => ok(undefined),
}));

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

describe("routing", () => {
  it("deep-links to the records route", () => {
    renderAt("/records");
    expect(screen.getByText("Personal records")).toBeInTheDocument();
    expect(screen.getByText("Fastest 5k")).toBeInTheDocument();
  });

  it("deep-links to the gear route", () => {
    renderAt("/gear");
    expect(screen.getByText("Vaporfly 3")).toBeInTheDocument();
  });

  it("deep-links to a workout detail route", () => {
    renderAt("/workouts/w1");
    // The detail summary title is the workout name.
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.getByText("Splits")).toBeInTheDocument();
  });

  it("navigates between routes via the header nav", () => {
    renderAt("/");
    // Start on the dashboard (training header phase name).
    expect(screen.getByText("Build 2")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("link", { name: "Gear" }));
    expect(screen.getByText("Canyon Aeroad")).toBeInTheDocument();
  });
});
