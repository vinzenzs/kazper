import { Panel } from "./Panel";
import { Stat } from "./Stat";
import type { FitnessSnapshot } from "../api/types";
import { raceTime } from "../lib/format";

interface RacePredictionsProps {
  fitness: FitnessSnapshot | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Garmin race-time predictions for the four standard distances, formatted as
// race clocks. All nullable; carried by the training context's fitness snapshot.
export function RacePredictions({ fitness, isLoading, isError, error }: RacePredictionsProps) {
  const hasAny =
    !!fitness &&
    [
      fitness.race_predictor_5k_seconds,
      fitness.race_predictor_10k_seconds,
      fitness.race_predictor_half_seconds,
      fitness.race_predictor_full_seconds,
    ].some((v) => v !== null && v !== undefined);

  return (
    <Panel
      title="Race predictions"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasAny}
      emptyHint="No race predictions"
    >
      {hasAny && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="5K" value={raceTime(fitness?.race_predictor_5k_seconds)} />
          <Stat label="10K" value={raceTime(fitness?.race_predictor_10k_seconds)} />
          <Stat label="Half" value={raceTime(fitness?.race_predictor_half_seconds)} />
          <Stat label="Full" value={raceTime(fitness?.race_predictor_full_seconds)} />
        </div>
      )}
    </Panel>
  );
}
