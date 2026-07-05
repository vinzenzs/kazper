import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import { emptyWorkoutStats, populatedWorkoutStats } from "../../test/fixtures";

// Shared, hoisted mock state so the vi.mock factory can read it.
const h = vi.hoisted(() => ({
  calls: [] as { from: string; to: string }[],
  data: undefined as unknown,
}));

vi.mock("../../api/hooks", () => ({
  useWorkoutStats: (from: string, to: string) => {
    h.calls.push({ from, to });
    return { data: h.data, isLoading: false, isError: false, error: null };
  },
}));

// Imported after the mock is declared.
import { StatsView } from "../StatsView";

beforeEach(() => {
  h.calls.length = 0;
  h.data = populatedWorkoutStats;
});
afterEach(() => cleanup());

describe("StatsView", () => {
  it("renders the volume totals for the default (Week) period", () => {
    render(<StatsView />);
    expect(screen.getByText("55.0")).toBeInTheDocument(); // 55000m distance → km
    expect(screen.getByText("2h 15m")).toBeInTheDocument(); // 135 min
    expect(screen.getByText("Cycling ×1")).toBeInTheDocument();
  });

  it("switches the requested window when the period toggle changes", () => {
    render(<StatsView />);
    const firstFrom = h.calls[h.calls.length - 1].from;
    fireEvent.click(screen.getByRole("button", { name: "YTD" }));
    const ytdFrom = h.calls[h.calls.length - 1].from;
    expect(ytdFrom).not.toEqual(firstFrom);
    expect(ytdFrom).toMatch(/-01-01$/); // YTD starts on Jan 1
  });

  it("shows an empty state when the window has no workouts", () => {
    h.data = emptyWorkoutStats;
    render(<StatsView />);
    expect(
      screen.getByText(/no completed workouts in this period/i),
    ).toBeInTheDocument();
  });
});
