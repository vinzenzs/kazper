import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import {
  emptyPMC,
  emptyPowerCurve,
  emptyWorkoutStats,
  populatedPMC,
  populatedPowerCurve,
  populatedWorkoutStats,
} from "../../test/fixtures";

// Shared, hoisted mock state so the vi.mock factory can read it.
const h = vi.hoisted(() => ({
  statsCalls: [] as { from: string; to: string }[],
  curveCalls: [] as { from: string; to: string; sport: string }[],
  pmcCalls: [] as { from: string; to: string }[],
  statsData: undefined as unknown,
  curveData: undefined as unknown,
  pmcData: undefined as unknown,
}));

vi.mock("../../api/hooks", () => ({
  useWorkoutStats: (from: string, to: string) => {
    h.statsCalls.push({ from, to });
    return { data: h.statsData, isLoading: false, isError: false, error: null };
  },
  usePowerCurve: (from: string, to: string, sport: string) => {
    h.curveCalls.push({ from, to, sport });
    return { data: h.curveData, isLoading: false, isError: false, error: null };
  },
  usePMC: (from: string, to: string) => {
    h.pmcCalls.push({ from, to });
    return { data: h.pmcData, isLoading: false, isError: false, error: null };
  },
}));

import { StatsView } from "../StatsView";

beforeEach(() => {
  h.statsCalls.length = 0;
  h.curveCalls.length = 0;
  h.pmcCalls.length = 0;
  h.statsData = populatedWorkoutStats;
  h.curveData = populatedPowerCurve;
  h.pmcData = populatedPMC;
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
    const firstFrom = h.statsCalls[h.statsCalls.length - 1].from;
    fireEvent.click(screen.getByRole("button", { name: "YTD" }));
    const ytdFrom = h.statsCalls[h.statsCalls.length - 1].from;
    expect(ytdFrom).not.toEqual(firstFrom);
    expect(ytdFrom).toMatch(/-01-01$/); // YTD starts on Jan 1
  });

  it("shows an empty state when the window has no workouts", () => {
    h.statsData = emptyWorkoutStats;
    render(<StatsView />);
    expect(
      screen.getByText(/no completed workouts in this period/i),
    ).toBeInTheDocument();
  });

  it("renders the power curve and re-queries when the sport changes", () => {
    render(<StatsView />);
    expect(screen.getByRole("img", { name: /power\/pace curve/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Run" }));
    expect(h.curveCalls[h.curveCalls.length - 1].sport).toEqual("run");
  });

  it("shows a curve empty-state (with the selector still visible)", () => {
    h.curveData = emptyPowerCurve;
    render(<StatsView />);
    expect(screen.getByText(/no effort data for bike in this period/i)).toBeInTheDocument();
    // The sport selector must remain available so the user can switch.
    expect(screen.getByRole("button", { name: "Run" })).toBeInTheDocument();
  });

  it("renders the PMC chart with the fitness/fatigue/form summary and ramp flag", () => {
    render(<StatsView />);
    expect(
      screen.getByRole("img", { name: /performance management chart/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("45.3")).toBeInTheDocument(); // latest CTL
    expect(screen.getByTestId("pmc-missing")).toHaveTextContent(/without TSS/i);
    // The unsafe-ramp week is shaded on the trace.
    expect(screen.getAllByTestId("ramp-band").length).toBeGreaterThan(0);
  });

  it("re-queries the PMC series when the window selector changes", () => {
    render(<StatsView />);
    const first = h.pmcCalls[h.pmcCalls.length - 1].from;
    fireEvent.click(screen.getByRole("button", { name: "365d" }));
    const wider = h.pmcCalls[h.pmcCalls.length - 1].from;
    expect(new Date(wider).getTime()).toBeLessThan(new Date(first).getTime());
  });

  it("shows a PMC empty-state on all-zero history", () => {
    h.pmcData = emptyPMC;
    render(<StatsView />);
    expect(screen.getByText(/no training history to chart yet/i)).toBeInTheDocument();
  });
});
