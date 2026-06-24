// Maps Garmin's training_status string to a display label + a Tailwind color
// class for the Header/Fitness badge. Garmin's vocabulary can drift, so any
// unrecognized (or absent) value falls back to a neutral badge rather than
// crashing or rendering nothing.
import { titleCase } from "./format";

export interface StatusBadge {
  label: string;
  // Tailwind classes for the badge (text + subtle background).
  className: string;
}

const NEUTRAL = "text-slate-300 bg-ink-600/70";
const GOOD = "text-accent-good bg-accent-good/10";
const WARN = "text-accent-warn bg-accent-warn/10";
const DANGER = "text-accent-danger bg-accent-danger/10";
const INFO = "text-accent bg-accent/10";

// Keyed by Garmin's snake_case status values.
const STATUS_CLASS: Record<string, string> = {
  productive: GOOD,
  peaking: GOOD,
  maintaining: INFO,
  recovery: INFO,
  unproductive: WARN,
  detraining: WARN,
  overreaching: DANGER,
  no_status: NEUTRAL,
};

export function trainingStatusBadge(
  status: string | null | undefined,
): StatusBadge | null {
  if (!status) return null;
  const className = STATUS_CLASS[status.toLowerCase()] ?? NEUTRAL;
  return { label: titleCase(status), className };
}
