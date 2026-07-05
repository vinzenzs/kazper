import { Panel } from "./Panel";
import type { Achievement } from "../api/types";
import { num, shortDate } from "../lib/format";

interface AchievementsProps {
  achievements: Achievement[] | null | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
}

// A compact chip strip — not a badge wall. Each chip shows the achievement name
// with its earned date, or an in-progress percentage for an unearned challenge.
export function Achievements({
  achievements,
  isLoading,
  isError,
  error,
}: AchievementsProps) {
  const items = achievements ?? [];
  return (
    <Panel
      title="Achievements"
      isLoading={isLoading}
      isError={isError}
      error={error}
      isEmpty={items.length === 0}
      emptyHint="No achievements yet"
    >
      <ul className="flex flex-wrap gap-1.5">
        {items.map((a) => (
          <li
            key={a.id}
            className="rounded-full bg-ink-700/70 px-2.5 py-1 text-xs text-slate-300"
          >
            <span className="font-medium text-slate-100">{a.name}</span>
            <span className="ml-1.5 text-slate-500">{detail(a)}</span>
          </li>
        ))}
      </ul>
    </Panel>
  );
}

function detail(a: Achievement): string {
  if (a.earned_at) return shortDate(a.earned_at);
  if (a.progress_pct !== null && a.progress_pct !== undefined) {
    return `${num(a.progress_pct, 0)}%`;
  }
  return "";
}
