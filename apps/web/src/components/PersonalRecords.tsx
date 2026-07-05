import { Panel } from "./Panel";
import type { PersonalRecord } from "../api/types";
import { PLACEHOLDER, km, num, raceTime, shortDate, titleCase } from "../lib/format";

interface PersonalRecordsProps {
  records: PersonalRecord[] | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// A dense best-efforts table: PR type · value (formatted by its unit) · date.
// `activity_id` is a Garmin external id, not a Kazper workout id, so a PR row is
// display-only — no link into /workouts/:id.
export function PersonalRecords({
  records,
  isLoading,
  isError,
  error,
}: PersonalRecordsProps) {
  const items = records ?? [];
  return (
    <Panel
      title="Personal records"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={items.length === 0}
      emptyHint="No personal records yet"
    >
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-[11px] uppercase tracking-wide text-slate-500">
            <th className="pb-2 font-medium">Record</th>
            <th className="pb-2 text-right font-medium">Value</th>
            <th className="pb-2 text-right font-medium">Date</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-ink-600/50">
          {items.map((r) => (
            <tr key={r.id}>
              <td className="py-2 pr-3 text-slate-200">{titleCase(r.pr_type)}</td>
              <td className="py-2 pl-3 text-right font-semibold tabular-nums text-slate-100">
                {prValue(r.value, r.unit)}
              </td>
              <td className="py-2 pl-3 text-right tabular-nums text-slate-400">
                {shortDate(r.achieved_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Panel>
  );
}

// Format a PR value by its accompanying unit: seconds → race clock, metres →
// km/m, anything else → the number with the unit appended.
function prValue(value: number | null, unit: string): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return PLACEHOLDER;
  }
  switch (unit) {
    case "s":
      return raceTime(value);
    case "m":
      return value >= 1000 ? `${km(value)} km` : `${num(value, 0)} m`;
    default:
      return unit ? `${num(value, 1)} ${unit}` : num(value, 1);
  }
}
