import { Panel } from "./Panel";
import type { Gear } from "../api/types";
import { km, num, titleCase } from "../lib/format";

interface GearPanelProps {
  gear: Gear[] | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// Gear inventory with accumulated mileage. Each row carries a thin muted bar
// sized against the highest-mileage item so relative wear reads at a glance.
// Retired gear is dimmed rather than hidden.
export function GearPanel({ gear, isLoading, isError, error }: GearPanelProps) {
  const items = gear ?? [];
  const maxDistance = items.reduce(
    (m, g) => Math.max(m, g.total_distance_m ?? 0),
    0,
  );
  return (
    <Panel
      title="Gear"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={items.length === 0}
      emptyHint="No gear yet"
    >
      <ul className="flex flex-col gap-3">
        {items.map((g) => (
          <Row key={g.id} gear={g} maxDistance={maxDistance} />
        ))}
      </ul>
    </Panel>
  );
}

function Row({ gear, maxDistance }: { gear: Gear; maxDistance: number }) {
  const distance = gear.total_distance_m ?? 0;
  const pct = maxDistance > 0 ? (distance / maxDistance) * 100 : 0;
  return (
    <li className={gear.retired ? "opacity-40" : undefined}>
      <div className="flex items-baseline justify-between gap-3">
        <div className="min-w-0">
          <span className="truncate text-sm font-medium text-slate-100">
            {gear.display_name}
          </span>
          <span className="ml-2 text-[11px] uppercase tracking-wide text-slate-500">
            {titleCase(gear.gear_type)}
            {gear.retired ? " · retired" : ""}
          </span>
        </div>
        <div className="shrink-0 text-right">
          <span className="text-sm font-semibold tabular-nums text-slate-200">
            {km(gear.total_distance_m)}
          </span>
          <span className="ml-1 text-xs text-slate-400">km</span>
          {gear.total_activities !== null &&
            gear.total_activities !== undefined && (
              <span className="ml-2 text-[11px] text-slate-500">
                {num(gear.total_activities, 0)} act
              </span>
            )}
        </div>
      </div>
      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-ink-700/60">
        <div
          className="h-full rounded-full bg-accent/60"
          style={{ width: `${pct}%` }}
        />
      </div>
    </li>
  );
}
