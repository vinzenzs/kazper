import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";

import {
  emptyDistribution,
  emptyPMC,
  emptyPowerCurve,
  emptyWorkoutStats,
  nullCPModel,
  populatedCPModel,
  populatedDistribution,
  populatedPMC,
  populatedPowerCurve,
  populatedPowerProfile,
  populatedWorkoutStats,
} from "../../test/fixtures";

// Shared, hoisted mock state so the vi.mock factory can read it.
const h = vi.hoisted(() => ({
  statsCalls: [] as { from: string; to: string }[],
  curveCalls: [] as { from: string; to: string; sport: string }[],
  pmcCalls: [] as { from: string; to: string }[],
  cpCalls: [] as { from: string; to: string }[],
  ppCalls: [] as { from: string; to: string }[],
  intensityCalls: [] as { from: string; to: string }[],
  statsData: undefined as unknown,
  curveData: undefined as unknown,
  pmcData: undefined as unknown,
  cpData: undefined as unknown,
  ppData: undefined as unknown,
  ppError: false as boolean,
  intensityData: undefined as unknown,
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
  useCPModel: (from: string, to: string) => {
    h.cpCalls.push({ from, to });
    return { data: h.cpData, isLoading: false, isError: false, error: null };
  },
  usePowerProfile: (from: string, to: string) => {
    h.ppCalls.push({ from, to });
    return {
      data: h.ppError ? undefined : h.ppData,
      isLoading: false,
      isError: h.ppError,
      error: null,
    };
  },
  useIntensityDistribution: (from: string, to: string) => {
    h.intensityCalls.push({ from, to });
    return { data: h.intensityData, isLoading: false, isError: false, error: null };
  },
}));

import { StatsView } from "../StatsView";

beforeEach(() => {
  h.statsCalls.length = 0;
  h.curveCalls.length = 0;
  h.pmcCalls.length = 0;
  h.cpCalls.length = 0;
  h.ppCalls.length = 0;
  h.intensityCalls.length = 0;
  h.statsData = populatedWorkoutStats;
  h.curveData = populatedPowerCurve;
  h.pmcData = populatedPMC;
  h.cpData = populatedCPModel;
  h.ppData = populatedPowerProfile;
  h.ppError = false;
  h.intensityData = populatedDistribution;
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
    // The PMC panel's 90/180/365 toggle is the first on the page (the CP panel
    // adds a second identical one below it).
    fireEvent.click(screen.getAllByRole("button", { name: "365d" })[0]);
    const wider = h.pmcCalls[h.pmcCalls.length - 1].from;
    expect(new Date(wider).getTime()).toBeLessThan(new Date(first).getTime());
  });

  it("shows a PMC empty-state on all-zero history", () => {
    h.pmcData = emptyPMC;
    render(<StatsView />);
    expect(screen.getByText(/no training history to chart yet/i)).toBeInTheDocument();
  });

  it("renders the critical-power model readout and fitted curve", () => {
    render(<StatsView />);
    expect(screen.getByRole("img", { name: /critical-power model/i })).toBeInTheDocument();
    const readout = screen.getByTestId("cp-readout");
    expect(readout).toHaveTextContent("268 W"); // CP
    expect(readout).toHaveTextContent("21.3 kJ"); // W′
    expect(readout).toHaveTextContent("0.99"); // R²
  });

  it("re-queries the CP model when its window selector changes", () => {
    render(<StatsView />);
    const first = h.cpCalls[h.cpCalls.length - 1].from;
    // 90/180/365 toggles, in page order: PMC, CP, Power profile. The CP panel's
    // is the second-to-last 365d button.
    const buttons = screen.getAllByRole("button", { name: "365d" });
    fireEvent.click(buttons[buttons.length - 2]);
    const wider = h.cpCalls[h.cpCalls.length - 1].from;
    expect(new Date(wider).getTime()).toBeLessThan(new Date(first).getTime());
  });

  it("shows the CP degraded state with the gate reason", () => {
    h.cpData = nullCPModel;
    render(<StatsView />);
    expect(screen.getByText(/not enough long efforts/i)).toBeInTheDocument();
    expect(screen.queryByRole("img", { name: /critical-power model/i })).not.toBeInTheDocument();
  });

  it("renders the intensity distribution with classification badge and missing note", () => {
    render(<StatsView />);
    expect(screen.getByTestId("intensity-class")).toHaveTextContent("Polarized");
    expect(screen.getByTestId("intensity-missing")).toHaveTextContent(/without HR zones/i);
    // A window bar plus one weekly bar.
    expect(screen.getAllByTestId("zone-share-bar").length).toBeGreaterThanOrEqual(2);
  });

  it("re-queries the intensity distribution when the period changes", () => {
    render(<StatsView />);
    const first = h.intensityCalls[h.intensityCalls.length - 1].from;
    fireEvent.click(screen.getByRole("button", { name: "YTD" }));
    expect(h.intensityCalls[h.intensityCalls.length - 1].from).not.toEqual(first);
  });

  it("shows an intensity empty-state when the window has no HR-zone data", () => {
    h.intensityData = emptyDistribution;
    render(<StatsView />);
    expect(screen.getByText(/no hr-zone data in this period/i)).toBeInTheDocument();
  });

  it("renders the power-profile panel with anchors, categories and phenotype", () => {
    render(<StatsView />);
    expect(screen.getByText("Power profile (Coggan)")).toBeInTheDocument();
    // Four ranked anchors present.
    const anchors = screen.getByTestId("power-profile-anchors");
    expect(anchors.children.length).toBe(4);
    expect(screen.getByText("Neuromuscular (5 s)")).toBeInTheDocument();
    expect(screen.getByText("Threshold (20 min)")).toBeInTheDocument();
    // W/kg + a category badge.
    expect(screen.getByText("16.3 W/kg")).toBeInTheDocument();
    expect(screen.getByText("Very good")).toBeInTheDocument();
    // Phenotype label.
    expect(screen.getByText("Sprinter")).toBeInTheDocument();
  });

  it("degrades the power-profile panel when weight is missing (fetch error)", () => {
    h.ppError = true;
    render(<StatsView />);
    expect(screen.getByText("Power profile (Coggan)")).toBeInTheDocument();
    expect(screen.getByText(/add a body-weight entry/i)).toBeInTheDocument();
    expect(screen.queryByTestId("power-profile-anchors")).not.toBeInTheDocument();
  });

  it("omits the phenotype label when the profile is incomplete", () => {
    h.ppData = {
      ...populatedPowerProfile,
      anchors: populatedPowerProfile.anchors.slice(0, 2),
      missing_anchors: ["vo2max", "threshold"],
      phenotype: null,
    };
    render(<StatsView />);
    expect(screen.getByText(/phenotype needs all four anchors/i)).toBeInTheDocument();
    expect(screen.queryByText("Sprinter")).not.toBeInTheDocument();
    // Missing anchors are named.
    expect(screen.getByText(/no effort for:/i)).toBeInTheDocument();
  });
});
