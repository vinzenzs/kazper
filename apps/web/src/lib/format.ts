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

// raceTime formats a whole-second duration as a race clock: "m:ss" under an
// hour, "h:mm:ss" at or above. Used for the race-predictor fields (5k/10k/etc.).
export function raceTime(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds)) {
    return PLACEHOLDER;
  }
  const total = Math.round(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const ss = s.toString().padStart(2, "0");
  if (h === 0) return `${m}:${ss}`;
  return `${h}:${m.toString().padStart(2, "0")}:${ss}`;
}

// pace formats seconds-per-km as "m:ss /km" (threshold running pace).
export function pace(secPerKm: number | null | undefined): string {
  if (secPerKm === null || secPerKm === undefined || Number.isNaN(secPerKm)) {
    return PLACEHOLDER;
  }
  const total = Math.round(secPerKm);
  const m = Math.floor(total / 60);
  const s = (total % 60).toString().padStart(2, "0");
  return `${m}:${s} /km`;
}

// pace100 formats seconds-per-100m as "m:ss /100m" (threshold swim pace).
export function pace100(secPer100m: number | null | undefined): string {
  if (secPer100m === null || secPer100m === undefined || Number.isNaN(secPer100m)) {
    return PLACEHOLDER;
  }
  const total = Math.round(secPer100m);
  const m = Math.floor(total / 60);
  const s = (total % 60).toString().padStart(2, "0");
  return `${m}:${s} /100m`;
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
