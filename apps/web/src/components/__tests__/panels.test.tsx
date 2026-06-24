import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { Header } from "../Header";
import { FormGauge } from "../FormGauge";
import { LoadTrend } from "../LoadTrend";
import { RecoverySnapshot } from "../RecoverySnapshot";
import { WorkoutList } from "../WorkoutList";
import {
  emptyRecovery,
  emptyTraining,
  emptyTrend,
  populatedRecovery,
  populatedTraining,
  populatedTrend,
} from "../../test/fixtures";

// Each panel renders against both a populated fixture and a null-heavy one. The
// null-heavy assertions are the load-bearing ones: every metric is nullable, so
// the dashboard must degrade gracefully (placeholder / empty state) not throw.

describe("Header", () => {
  it("renders phase, season and days-to-race when populated", () => {
    render(<Header training={populatedTraining} />);
    expect(screen.getByText("Build 2")).toBeInTheDocument();
    expect(screen.getByText("Alpe d'Huez 2026")).toBeInTheDocument();
    expect(screen.getByText("36")).toBeInTheDocument();
    expect(screen.getByText(/days to race/i)).toBeInTheDocument();
  });

  it("degrades gracefully with no phase or season", () => {
    render(<Header training={emptyTraining} />);
    expect(screen.getByText(/no active phase/i)).toBeInTheDocument();
    expect(screen.getByText(/no race anchored/i)).toBeInTheDocument();
  });
});

describe("FormGauge", () => {
  it("renders the ACWR value and zone", () => {
    render(<FormGauge acwr={populatedTraining.acwr} />);
    expect(screen.getByText("1.08")).toBeInTheDocument();
    expect(screen.getByText(/optimal/i)).toBeInTheDocument();
  });

  it("shows an empty state when ACWR is null", () => {
    render(<FormGauge acwr={null} />);
    expect(screen.getByText(/needs acute \+ chronic load/i)).toBeInTheDocument();
  });
});

describe("LoadTrend", () => {
  it("renders the chart with a legend when there is history", () => {
    render(<LoadTrend metrics={populatedTrend} />);
    expect(screen.getByLabelText(/training load trend/i)).toBeInTheDocument();
    expect(screen.getByText("Acute (7d)")).toBeInTheDocument();
    expect(screen.getByText("Chronic (28d)")).toBeInTheDocument();
  });

  it("shows an empty state with no history", () => {
    render(<LoadTrend metrics={emptyTrend} />);
    expect(screen.getByText(/no load history/i)).toBeInTheDocument();
  });
});

describe("RecoverySnapshot", () => {
  it("renders HRV, sleep and resting HR", () => {
    render(<RecoverySnapshot latest={populatedRecovery.latest} />);
    expect(screen.getByText("HRV")).toBeInTheDocument();
    expect(screen.getByText("78")).toBeInTheDocument();
    expect(screen.getByText("44")).toBeInTheDocument();
  });

  it("shows an empty state with no snapshot", () => {
    render(<RecoverySnapshot latest={emptyRecovery.latest} />);
    expect(screen.getByText(/no recovery snapshot/i)).toBeInTheDocument();
  });
});

describe("WorkoutList", () => {
  it("renders workouts when present", () => {
    render(<WorkoutList title="Recent workouts" workouts={populatedTraining.recent_workouts} />);
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.getByText("120")).toBeInTheDocument();
  });

  it("shows an empty state with no workouts", () => {
    render(
      <WorkoutList
        title="Upcoming workouts"
        workouts={emptyTraining.upcoming_workouts}
        emptyHint="Nothing scheduled"
      />,
    );
    expect(screen.getByText(/nothing scheduled/i)).toBeInTheDocument();
  });
});
