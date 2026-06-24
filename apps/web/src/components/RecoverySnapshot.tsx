import { Panel } from "./Panel";
import { Stat } from "./Stat";
import type { RecoverySnapshot as Snapshot } from "../api/types";
import { num, sleep } from "../lib/format";

interface RecoverySnapshotProps {
  latest: Snapshot | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Recovery snapshot: HRV · sleep · sleep score · resting HR · stress · body
// battery · readiness. Each metric is nullable; the Stat component shows a
// placeholder for absent values.
export function RecoverySnapshot({ latest, isLoading, isError, error }: RecoverySnapshotProps) {
  const hasAny =
    !!latest &&
    [
      latest.hrv_ms,
      latest.sleep_seconds,
      latest.sleep_score,
      latest.resting_hr,
      latest.stress_avg,
      latest.body_battery_charged,
      latest.body_battery_drained,
      latest.training_readiness,
    ].some((v) => v !== null && v !== undefined);

  return (
    <Panel
      title="Recovery"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={!hasAny}
      emptyHint="No recovery snapshot"
    >
      {hasAny && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat label="HRV" value={num(latest?.hrv_ms, 0)} unit="ms" />
          <Stat label="Sleep" value={sleep(latest?.sleep_seconds)} />
          <Stat label="Sleep score" value={num(latest?.sleep_score, 0)} />
          <Stat label="Resting HR" value={num(latest?.resting_hr, 0)} unit="bpm" />
          <Stat label="Stress" value={num(latest?.stress_avg, 0)} />
          <Stat label="Battery +" value={num(latest?.body_battery_charged, 0)} />
          <Stat label="Battery −" value={num(latest?.body_battery_drained, 0)} />
          <Stat label="Readiness" value={num(latest?.training_readiness, 0)} />
        </div>
      )}
    </Panel>
  );
}
