import { Panel } from "./Panel";
import { Stat } from "./Stat";
import type { FitnessSnapshot } from "../api/types";
import { num } from "../lib/format";
import { trainingStatusBadge } from "../lib/status";

interface FitnessPanelProps {
  fitness: FitnessSnapshot | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Fitness / performance snapshot: VO₂max (run/bike), training status, endurance
// and hill scores, fitness age. All fields are nullable and already carried by
// the training context's `fitness` snapshot.
export function FitnessPanel({ fitness, isLoading, isError, error }: FitnessPanelProps) {
  const badge = trainingStatusBadge(fitness?.training_status);
  const hasAny =
    !!fitness &&
    [
      fitness.vo2max_running,
      fitness.vo2max_cycling,
      fitness.training_status,
      fitness.endurance_score,
      fitness.hill_score,
      fitness.fitness_age,
    ].some((v) => v !== null && v !== undefined);

  return (
    <Panel
      title="Fitness"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasAny}
      emptyHint="No fitness snapshot"
    >
      {hasAny && (
        <>
          {badge && (
            <div className="mb-3">
              <span
                className={`rounded-full px-3 py-0.5 text-xs font-medium uppercase tracking-wide ${badge.className}`}
              >
                {badge.label}
              </span>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <Stat label="VO₂max run" value={num(fitness?.vo2max_running, 1)} />
            <Stat label="VO₂max bike" value={num(fitness?.vo2max_cycling, 1)} />
            <Stat label="Endurance" value={num(fitness?.endurance_score, 0)} />
            <Stat label="Hill" value={num(fitness?.hill_score, 0)} />
            <Stat label="Fitness age" value={num(fitness?.fitness_age, 0)} unit="yr" />
          </div>
        </>
      )}
    </Panel>
  );
}
