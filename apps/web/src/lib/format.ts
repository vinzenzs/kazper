// Formatting helpers. Every input is nullable because every upstream metric is
// nullable; the dashboard shows an em-dash placeholder rather than erroring.

export const PLACEHOLDER = "—";

export function num(value: number | null | undefined, digits = 0): string {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return PLACEHOLDER;
  }
  return value.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

export function duration(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined || Number.isNaN(minutes)) {
    return PLACEHOLDER;
  }
  const h = Math.floor(minutes / 60);
  const m = Math.round(minutes % 60);
  if (h === 0) return `${m}m`;
  return `${h}h ${m.toString().padStart(2, "0")}m`;
}

export function sleep(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds)) {
    return PLACEHOLDER;
  }
  return duration(seconds / 60);
}

export function shortDate(iso: string | null | undefined): string {
  if (!iso) return PLACEHOLDER;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return PLACEHOLDER;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function weekday(iso: string | null | undefined): string {
  if (!iso) return PLACEHOLDER;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return PLACEHOLDER;
  return d.toLocaleDateString(undefined, { weekday: "short" });
}

// titleCase turns "long_run" / "tempo" into "Long Run" / "Tempo" for sport and
// status labels that arrive as snake_case.
export function titleCase(value: string | null | undefined): string {
  if (!value) return PLACEHOLDER;
  return value
    .split(/[_\s]+/)
    .map((w) => (w ? w[0].toUpperCase() + w.slice(1) : w))
    .join(" ");
}
