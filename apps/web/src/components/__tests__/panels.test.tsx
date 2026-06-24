import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { Header } from "../Header";
import { FormGauge } from "../FormGauge";
import { LoadTrend } from "../LoadTrend";
import { FitnessPanel } from "../FitnessPanel";
import { RacePredictions } from "../RacePredictions";
import { PowerThresholds } from "../PowerThresholds";
import { Zones } from "../Zones";
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

  it("shows the training-status badge when present", () => {
    render(<Header training={populatedTraining} />);
    expect(screen.getByText("Productive")).toBeInTheDocument();
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

  it("renders the extended metrics (sleep score, stress, body battery)", () => {
    render(<RecoverySnapshot latest={populatedRecovery.latest} />);
    expect(screen.getByText("Sleep score")).toBeInTheDocument();
    expect(screen.getByText("Stress")).toBeInTheDocument();
    expect(screen.getByText("Battery +")).toBeInTheDocument();
    expect(screen.getByText("Battery −")).toBeInTheDocument();
  });

  it("shows an empty state with no snapshot", () => {
    render(<RecoverySnapshot latest={emptyRecovery.latest} />);
    expect(screen.getByText(/no recovery snapshot/i)).toBeInTheDocument();
  });
});

describe("FitnessPanel", () => {
  it("renders VO₂max, scores and the status badge", () => {
    render(<FitnessPanel fitness={populatedTraining.fitness} />);
    expect(screen.getByText("56.2")).toBeInTheDocument();
    expect(screen.getByText("61.4")).toBeInTheDocument();
    expect(screen.getByText("Productive")).toBeInTheDocument();
    expect(screen.getByText(/fitness age/i)).toBeInTheDocument();
  });

  it("shows an empty state with no fitness snapshot", () => {
    render(<FitnessPanel fitness={emptyTraining.fitness} />);
    expect(screen.getByText(/no fitness snapshot/i)).toBeInTheDocument();
  });
});

describe("RacePredictions", () => {
  it("renders the four distances as race times", () => {
    render(<RacePredictions fitness={populatedTraining.fitness} />);
    expect(screen.getByText("18:32")).toBeInTheDocument(); // 1112s 5k
    expect(screen.getByText("38:30")).toBeInTheDocument(); // 2310s 10k
    expect(screen.getByText("1:24:05")).toBeInTheDocument(); // 5045s half
  });

  it("shows an empty state with no predictions", () => {
    render(<RacePredictions fitness={emptyTraining.fitness} />);
    expect(screen.getByText(/no race predictions/i)).toBeInTheDocument();
  });
});

describe("PowerThresholds", () => {
  it("renders FTP, watts/kg and thresholds", () => {
    render(
      <PowerThresholds
        config={populatedTraining.athlete_config}
        wattsPerKg={populatedTraining.watts_per_kg}
      />,
    );
    expect(screen.getByText("285")).toBeInTheDocument(); // FTP
    expect(screen.getByText("4.1")).toBeInTheDocument(); // W/kg
    expect(screen.getByText("4:05 /km")).toBeInTheDocument(); // 245s/km threshold pace
  });

  it("shows an empty state with no config or watts/kg", () => {
    render(<PowerThresholds config={emptyTraining.athlete_config} wattsPerKg={null} />);
    expect(screen.getByText(/no power\/threshold config/i)).toBeInTheDocument();
  });
});

describe("Zones", () => {
  it("renders HR and power zone bands", () => {
    render(<Zones config={populatedTraining.athlete_config} />);
    expect(screen.getByText("HR")).toBeInTheDocument();
    expect(screen.getByText("Power")).toBeInTheDocument();
    expect(screen.getByText("≤120")).toBeInTheDocument(); // HR Z1 boundary
    expect(screen.getByText("≤360")).toBeInTheDocument(); // Power Z5 boundary
  });

  it("shows an empty state with no zones configured", () => {
    render(<Zones config={emptyTraining.athlete_config} />);
    expect(screen.getByText(/no zones configured/i)).toBeInTheDocument();
  });
});

describe("WorkoutList", () => {
  it("renders workouts when present", () => {
    render(<WorkoutList title="Recent workouts" workouts={populatedTraining.recent_workouts} />);
    expect(screen.getByText("Threshold 4x8")).toBeInTheDocument();
    expect(screen.getByText("120")).toBeInTheDocument();
  });

  it("renders by-sport chips when provided", () => {
    render(
      <WorkoutList
        title="Recent workouts"
        workouts={populatedTraining.recent_workouts}
        bySport={populatedTraining.recent_load.by_sport}
      />,
    );
    expect(screen.getByText("Cycling ×3")).toBeInTheDocument();
    expect(screen.getByText("Running ×2")).toBeInTheDocument();
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
