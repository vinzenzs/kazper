import { Link } from "react-router-dom";

import { Panel } from "./Panel";
import type { WorkoutLite } from "../api/types";
import { duration, num, shortDate, titleCase, weekday } from "../lib/format";

interface WorkoutListProps {
  title: string;
  workouts: WorkoutLite[] | null | undefined;
  // Optional recent-load breakdown (sport → count) shown as chips above the list.
  bySport?: Record<string, number> | null;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  emptyHint?: string;
}

export function WorkoutList({
  title,
  workouts,
  bySport,
  isLoading,
  isError,
  error,
  emptyHint,
}: WorkoutListProps) {
  const items = workouts ?? [];
  const sports = Object.entries(bySport ?? {}).filter(([, n]) => n > 0);
  return (
    <Panel
      title={title}
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={items.length === 0}
      emptyHint={emptyHint ?? "No workouts"}
    >
      {sports.length > 0 && (
        <div className="mb-2 flex flex-wrap gap-1.5">
          {sports.map(([sport, n]) => (
            <span
              key={sport}
              className="rounded-full bg-ink-700/70 px-2 py-0.5 text-[11px] text-slate-300"
            >
              {titleCase(sport)} ×{n}
            </span>
          ))}
        </div>
      )}
      <ul className="divide-y divide-ink-600/50">
        {items.map((w) => (
          <Row key={w.id} workout={w} />
        ))}
      </ul>
    </Panel>
  );
}

function statusColor(status: string): string {
  switch (status) {
    case "completed":
      return "text-accent-good";
    case "planned":
    case "scheduled":
      return "text-accent";
    case "skipped":
    case "missed":
      return "text-accent-danger";
    default:
      return "text-slate-400";
  }
}

function Row({ workout }: { workout: WorkoutLite }) {
  return (
    <li>
      <Link
        to={`/workouts/${workout.id}`}
        className="-mx-1 flex items-center justify-between gap-3 rounded-md px-1 py-2 transition-colors hover:bg-ink-700/40"
      >
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-slate-100">
          {workout.name || titleCase(workout.sport)}
        </div>
        <div className="text-xs text-slate-400">
          {weekday(workout.started_at)} {shortDate(workout.started_at)} ·{" "}
          {titleCase(workout.sport)} · {duration(workout.duration_min)}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-3 text-right">
        <div>
          <div className="text-sm font-semibold tabular-nums text-slate-200">
            {num(workout.tss, 0)}
          </div>
          <div className="text-[10px] uppercase tracking-wide text-slate-500">TSS</div>
        </div>
        <span
          className={`w-16 text-right text-xs font-medium ${statusColor(workout.status)}`}
        >
          {titleCase(workout.status)}
        </span>
      </div>
      </Link>
    </li>
  );
}
