import { describe, expect, it } from "vitest";

import { PLACEHOLDER, pace, pace100, raceTime } from "../format";
import { trainingStatusBadge } from "../status";

describe("raceTime", () => {
  it("formats sub-hour durations as m:ss", () => {
    expect(raceTime(1112)).toBe("18:32");
    expect(raceTime(2310)).toBe("38:30");
    expect(raceTime(59)).toBe("0:59");
  });

  it("formats hour-plus durations as h:mm:ss", () => {
    expect(raceTime(5045)).toBe("1:24:05");
    expect(raceTime(10625)).toBe("2:57:05");
  });

  it("returns the placeholder for null/undefined/NaN", () => {
    expect(raceTime(null)).toBe(PLACEHOLDER);
    expect(raceTime(undefined)).toBe(PLACEHOLDER);
    expect(raceTime(Number.NaN)).toBe(PLACEHOLDER);
  });
});

describe("pace", () => {
  it("formats seconds-per-km as m:ss /km", () => {
    expect(pace(245)).toBe("4:05 /km");
    expect(pace(300)).toBe("5:00 /km");
  });

  it("returns the placeholder for absent values", () => {
    expect(pace(null)).toBe(PLACEHOLDER);
  });
});

describe("pace100", () => {
  it("formats seconds-per-100m as m:ss /100m", () => {
    expect(pace100(98)).toBe("1:38 /100m");
  });

  it("returns the placeholder for absent values", () => {
    expect(pace100(undefined)).toBe(PLACEHOLDER);
  });
});

describe("trainingStatusBadge", () => {
  it("maps a known status to a label + class", () => {
    const badge = trainingStatusBadge("productive");
    expect(badge?.label).toBe("Productive");
    expect(badge?.className).toContain("accent-good");
  });

  it("falls back to a neutral badge for an unknown status", () => {
    const badge = trainingStatusBadge("some_new_status");
    expect(badge?.label).toBe("Some New Status");
    expect(badge?.className).toContain("slate");
  });

  it("returns null for an absent status", () => {
    expect(trainingStatusBadge(null)).toBeNull();
    expect(trainingStatusBadge(undefined)).toBeNull();
  });
});
