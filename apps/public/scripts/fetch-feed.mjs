// Build-time feed fetch — the ONLY point of contact with Kazper.
//
// Reads FEED_URL + FEED_SECRET from the environment (CI secret store), GETs the
// public race feed once with the X-Feed-Key header, and writes the non-PII
// {race, days_remaining} projection to src/generated/feed.json for the page and
// the OG generator to consume. Nothing here leaks into shipped assets: the
// secret and the URL never enter feed.json, and the browser never sees this
// module.
//
// Failure is loud and safe: a network error or any non-200 status exits
// non-zero, FAILING the build — GitHub Pages then keeps the previous deploy
// serving (a stale build is harmless because the countdown recomputes
// client-side; a broken build never ships). A {race: null} response is a normal
// off-season state and is written through.

import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const outFile = resolve(here, '../src/generated/feed.json');

function fail(msg) {
  console.error(`fetch-feed: ${msg}`);
  process.exit(1);
}

const feedURL = process.env.FEED_URL;
const feedSecret = process.env.FEED_SECRET;
if (!feedURL) fail('FEED_URL is not set');
if (!feedSecret) fail('FEED_SECRET is not set');

let res;
try {
  res = await fetch(feedURL, {
    headers: { 'X-Feed-Key': feedSecret, Accept: 'application/json' },
  });
} catch (err) {
  fail(`feed request failed: ${err?.message ?? err}`);
}

if (!res.ok) {
  // 401/403 = wrong/missing key; 503 = feed disabled (FEED_SECRET unset
  // server-side); anything non-200 must fail the build, not ship stale/empty.
  fail(`feed returned HTTP ${res.status}`);
}

let body;
try {
  body = await res.json();
} catch (err) {
  fail(`feed response was not JSON: ${err?.message ?? err}`);
}

// Normalize to exactly the two fields the site renders. A {race: null} feed is
// the graceful off-season state (rendered as a designed page, not an error).
const race = body?.race ?? null;
const snapshot = {
  race:
    race && typeof race === 'object'
      ? { name: String(race.name ?? ''), race_date: String(race.race_date ?? '') }
      : null,
  days_remaining:
    typeof body?.days_remaining === 'number' ? body.days_remaining : null,
};

await mkdir(dirname(outFile), { recursive: true });
await writeFile(outFile, JSON.stringify(snapshot, null, 2) + '\n', 'utf8');

console.log(
  snapshot.race
    ? `fetch-feed: wrote race "${snapshot.race.name}" (${snapshot.days_remaining} days)`
    : 'fetch-feed: wrote off-season state (race: null)',
);
