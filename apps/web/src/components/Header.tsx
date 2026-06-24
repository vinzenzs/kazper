import type { TrainingContext } from "../api/types";
import { num, titleCase } from "../lib/format";

interface HeaderProps {
  training: TrainingContext | null | undefined;
  isLoading?: boolean;
}

// The top banner: current phase, season name, and days-to-race when the season
// is race-anchored. Each field degrades to a placeholder independently.
export function Header({ training, isLoading }: HeaderProps) {
  const phase = training?.phase ?? null;
  const macro = training?.macrocycle ?? null;
  const daysToRace = macro?.days_to_race ?? null;

  return (
    <header className="flex flex-wrap items-end justify-between gap-4 rounded-xl border border-ink-600/60 bg-gradient-to-r from-ink-800 to-ink-700 px-6 py-5 shadow-lg shadow-black/30">
      <div>
        <div className="text-xs font-semibold uppercase tracking-widest text-accent">
          Kazper · Coach
        </div>
        <div className="mt-1 flex items-baseline gap-3">
          <span className="text-2xl font-bold text-slate-50">
            {isLoading ? "…" : phase ? phase.name : "No active phase"}
          </span>
          {phase && (
            <span className="rounded-full bg-ink-600/70 px-3 py-0.5 text-xs uppercase tracking-wide text-slate-300">
              {titleCase(phase.type)}
            </span>
          )}
        </div>
        {macro && (
          <div className="mt-1 text-sm text-slate-400">
            Season: <span className="text-slate-200">{macro.name}</span>
            {macro.race_name && (
              <>
                {" · "}
                <span className="text-slate-200">{macro.race_name}</span>
              </>
            )}
          </div>
        )}
      </div>

      <div className="text-right">
        {daysToRace !== null ? (
          <>
            <div className="text-4xl font-bold tabular-nums text-accent">
              {num(daysToRace)}
            </div>
            <div className="text-xs uppercase tracking-widest text-slate-400">
              days to race
            </div>
          </>
        ) : (
          <div className="text-sm text-slate-500">No race anchored</div>
        )}
      </div>
    </header>
  );
}
