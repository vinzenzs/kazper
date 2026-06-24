import { Panel } from "./Panel";
import { Stat } from "./Stat";
import type { AthleteConfig } from "../api/types";
import { num, pace, pace100 } from "../lib/format";

interface PowerThresholdsProps {
  config: AthleteConfig | null | undefined;
  wattsPerKg: number | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Power + threshold reference: FTP, watts/kg, threshold/max/lactate HR, and
// threshold run/swim paces. From athlete_config + the derived watts_per_kg, both
// carried by the training context.
export function PowerThresholds({
  config,
  wattsPerKg,
  isLoading,
  isError,
  error,
}: PowerThresholdsProps) {
  const hasAny =
    (!!config &&
      [
        config.ftp_watts,
        config.threshold_hr,
        config.lactate_threshold_hr,
        config.max_hr,
        config.threshold_pace_sec_per_km,
        config.threshold_swim_pace_sec_per_100m,
      ].some((v) => v !== null && v !== undefined)) ||
    (wattsPerKg !== null && wattsPerKg !== undefined);

  return (
    <Panel
      title="Power & thresholds"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasAny}
      emptyHint="No power/threshold config"
    >
      {hasAny && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          <Stat label="FTP" value={num(config?.ftp_watts, 0)} unit="W" />
          <Stat label="W/kg" value={num(wattsPerKg, 1)} />
          <Stat label="Threshold HR" value={num(config?.threshold_hr, 0)} unit="bpm" />
          <Stat label="Max HR" value={num(config?.max_hr, 0)} unit="bpm" />
          <Stat label="LT HR" value={num(config?.lactate_threshold_hr, 0)} unit="bpm" />
          <Stat label="Thr pace" value={pace(config?.threshold_pace_sec_per_km)} />
          <Stat label="Swim pace" value={pace100(config?.threshold_swim_pace_sec_per_100m)} />
        </div>
      )}
    </Panel>
  );
}
