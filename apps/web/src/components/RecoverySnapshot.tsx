import { Panel } from "./Panel";
import type { RecoverySnapshot as Snapshot } from "../api/types";
import { num, sleep } from "../lib/format";

interface RecoverySnapshotProps {
  latest: Snapshot | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Recovery snapshot: HRV · sleep · resting HR, plus readiness when present. Each
// metric is nullable; the Stat component shows a placeholder for absent values.
export function RecoverySnapshot({ latest, isLoading, isError, error }: RecoverySnapshotProps) {
  const hasAny =
    !!latest &&
    [latest.hrv_ms, latest.sleep_seconds, latest.resting_hr, latest.training_readiness].some(
      (v) => v !== null && v !== undefined,
    );

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
          <Stat label="Resting HR" value={num(latest?.resting_hr, 0)} unit="bpm" />
          <Stat label="Readiness" value={num(latest?.training_readiness, 0)} />
        </div>
      )}
    </Panel>
  );
}

function Stat({ label, value, unit }: { label: string; value: string; unit?: string }) {
  return (
    <div className="rounded-lg bg-ink-700/60 px-3 py-2">
      <div className="text-xs uppercase tracking-wide text-slate-400">{label}</div>
      <div className="mt-0.5 text-xl font-semibold tabular-nums text-slate-100">
        {value}
        {unit && value !== "—" && <span className="ml-1 text-xs text-slate-400">{unit}</span>}
      </div>
    </div>
  );
}
