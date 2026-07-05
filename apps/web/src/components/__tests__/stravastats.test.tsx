import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { PersonalRecords } from "../PersonalRecords";
import { Achievements } from "../Achievements";
import { GearPanel } from "../GearPanel";
import { SplitsTable } from "../SplitsTable";
import { ZoneTimeStrip } from "../ZoneTimeStrip";
import {
  populatedAchievements,
  populatedGear,
  populatedRecords,
  populatedWorkoutDetail,
} from "../../test/fixtures";

// The Phase-1 "surface existing data" panels. Each renders against a populated
// fixture and an empty one — the empty-state assertions are load-bearing.

describe("PersonalRecords", () => {
  it("renders each record with its value formatted by unit", () => {
    render(<PersonalRecords records={populatedRecords} />);
    expect(screen.getByText("Fastest 5k")).toBeInTheDocument();
    expect(screen.getByText("18:32")).toBeInTheDocument(); // 1112s → race clock
    expect(screen.getByText("Longest Ride")).toBeInTheDocument();
    expect(screen.getByText("182.0 km")).toBeInTheDocument(); // 182000m → km
  });

  it("shows an empty state with no records", () => {
    render(<PersonalRecords records={[]} />);
    expect(screen.getByText(/no personal records yet/i)).toBeInTheDocument();
  });
});

describe("Achievements", () => {
  it("renders earned and in-progress achievements", () => {
    render(<Achievements achievements={populatedAchievements} />);
    expect(screen.getByText("Century Ride")).toBeInTheDocument();
    expect(screen.getByText("March 200km")).toBeInTheDocument();
    expect(screen.getByText("64%")).toBeInTheDocument();
  });

  it("shows an empty state with no achievements", () => {
    render(<Achievements achievements={[]} />);
    expect(screen.getByText(/no achievements yet/i)).toBeInTheDocument();
  });
});

describe("GearPanel", () => {
  it("renders gear with mileage in km", () => {
    render(<GearPanel gear={populatedGear} />);
    expect(screen.getByText("Vaporfly 3")).toBeInTheDocument();
    expect(screen.getByText("412.0")).toBeInTheDocument(); // 412000m → km
    expect(screen.getByText("Canyon Aeroad")).toBeInTheDocument();
  });

  it("marks retired gear", () => {
    render(<GearPanel gear={populatedGear} />);
    expect(screen.getByText(/bike · retired/i)).toBeInTheDocument();
  });

  it("shows an empty state with no gear", () => {
    render(<GearPanel gear={[]} />);
    expect(screen.getByText(/no gear yet/i)).toBeInTheDocument();
  });
});

describe("SplitsTable", () => {
  it("renders a row per split with distance and pace", () => {
    render(<SplitsTable splits={populatedWorkoutDetail.splits ?? []} />);
    // 1000m → 1.0 km, two splits present
    expect(screen.getAllByText("1.0")).toHaveLength(2);
    // 6.7 m/s → 1000/6.7 ≈ 149s/km → 2:29 /km
    expect(screen.getByText("2:29 /km")).toBeInTheDocument();
  });
});

describe("ZoneTimeStrip", () => {
  it("renders bands for zones with positive time", () => {
    render(
      <ZoneTimeStrip
        label="HR"
        secs={[600, 1800, 900, 1500, 600]}
      />,
    );
    expect(screen.getByText("Z1")).toBeInTheDocument();
    expect(screen.getByText("Z5")).toBeInTheDocument();
  });

  it("shows a placeholder when there is no zone time", () => {
    render(<ZoneTimeStrip label="HR" secs={[null, null, null, null, null]} />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });
});
