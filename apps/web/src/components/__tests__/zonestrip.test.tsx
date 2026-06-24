import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { ZoneStrip } from "../ZoneStrip";
import { PLACEHOLDER } from "../../lib/format";

describe("ZoneStrip", () => {
  it("renders five bands when all boundaries are present", () => {
    render(<ZoneStrip label="HR" unit="bpm" zones={[120, 140, 160, 175, 190]} />);
    for (const z of ["Z1", "Z2", "Z3", "Z4", "Z5"]) {
      expect(screen.getByText(z)).toBeInTheDocument();
    }
    expect(screen.getByText("≤190")).toBeInTheDocument();
  });

  it("renders only the present bands when zones are sparse", () => {
    // Z2 and Z4 missing → three bands (Z1, Z3, Z5), no inference.
    render(<ZoneStrip label="HR" unit="bpm" zones={[120, null, 160, null, 190]} />);
    expect(screen.getByText("Z1")).toBeInTheDocument();
    expect(screen.getByText("Z3")).toBeInTheDocument();
    expect(screen.getByText("Z5")).toBeInTheDocument();
    expect(screen.queryByText("Z2")).not.toBeInTheDocument();
    expect(screen.queryByText("Z4")).not.toBeInTheDocument();
  });

  it("shows a placeholder when no boundaries are present", () => {
    render(<ZoneStrip label="Power" unit="W" zones={[null, null, null, null, null]} />);
    expect(screen.getByText(PLACEHOLDER)).toBeInTheDocument();
  });
});
