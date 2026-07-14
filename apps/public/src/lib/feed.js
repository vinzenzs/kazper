import fs from 'node:fs';
import path from 'node:path';

// loadFeed reads the build-time snapshot written by scripts/fetch-feed.mjs.
// Resolved from process.cwd() (the apps/public root during the build) rather
// than import.meta.url — Vite rewrites import.meta.url when it bundles this
// module for the Astro build, so a URL-relative path would miss the file and
// silently fall back to the off-season state. Missing file (e.g. `astro dev`
// without a prior fetch) still degrades to off-season rather than crashing; the
// real build always runs fetch-feed first, and a genuinely failed fetch has
// already exited non-zero by then.
export function loadFeed() {
  const p = path.resolve(process.cwd(), 'src/generated/feed.json');
  try {
    return JSON.parse(fs.readFileSync(p, 'utf8'));
  } catch {
    return { race: null, days_remaining: null };
  }
}

// formatRaceDate renders a YYYY-MM-DD date as e.g. "13 September 2026",
// interpreting the date in UTC so the label never drifts by a day.
export function formatRaceDate(iso) {
  const [y, m, d] = String(iso).split('-').map(Number);
  if (!y || !m || !d) return String(iso);
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  });
}
