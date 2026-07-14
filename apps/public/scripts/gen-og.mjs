// Build-time Open Graph card generator.
//
// Runs AFTER `astro build` and writes dist/og.png (1200×630) from the same
// build-time feed snapshot the page used, so the share card carries the race
// name + the build-time countdown (or a neutral off-season variant). The head's
// og:image points at this file. It carries the build-time number (it can't run
// JS); the nightly rebuild keeps it fresh.

import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Resvg } from '@resvg/resvg-js';

const here = dirname(fileURLToPath(import.meta.url));
const feedFile = resolve(here, '../src/generated/feed.json');
const outFile = resolve(here, '../dist/og.png');

let feed = { race: null, days_remaining: null };
try {
  feed = JSON.parse(await readFile(feedFile, 'utf8'));
} catch {
  // No snapshot → render the neutral variant rather than fail (the build's
  // fetch step already guards the real failure path).
}

function esc(s) {
  return String(s).replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c]);
}

const race = feed.race;
const days = typeof feed.days_remaining === 'number' ? feed.days_remaining : null;

const svg = race
  ? `<svg width="1200" height="630" viewBox="0 0 1200 630" xmlns="http://www.w3.org/2000/svg">
      <defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#17203c"/><stop offset="1" stop-color="#0b1020"/>
      </linearGradient></defs>
      <rect width="1200" height="630" fill="url(#bg)"/>
      <text x="80" y="130" font-family="sans-serif" font-size="30" letter-spacing="8" fill="#93a1c4">ROAD TO RACE</text>
      <text x="80" y="360" font-family="sans-serif" font-size="240" font-weight="800" fill="#6ea8fe">${days ?? ''}</text>
      <text x="80" y="430" font-family="sans-serif" font-size="40" fill="#93a1c4">days to go</text>
      <text x="80" y="540" font-family="sans-serif" font-size="64" font-weight="700" fill="#f4f6fb">${esc(race.name)}</text>
    </svg>`
  : `<svg width="1200" height="630" viewBox="0 0 1200 630" xmlns="http://www.w3.org/2000/svg">
      <defs><linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0" stop-color="#17203c"/><stop offset="1" stop-color="#0b1020"/>
      </linearGradient></defs>
      <rect width="1200" height="630" fill="url(#bg)"/>
      <text x="80" y="130" font-family="sans-serif" font-size="30" letter-spacing="8" fill="#93a1c4">ROAD TO RACE</text>
      <text x="80" y="340" font-family="sans-serif" font-size="90" font-weight="800" fill="#f4f6fb">Between seasons</text>
      <text x="80" y="420" font-family="sans-serif" font-size="40" fill="#93a1c4">No race on the calendar right now.</text>
    </svg>`;

const png = new Resvg(svg, {
  fitTo: { mode: 'width', value: 1200 },
  font: { loadSystemFonts: true, defaultFontFamily: 'sans-serif' },
})
  .render()
  .asPng();

await mkdir(dirname(outFile), { recursive: true });
await writeFile(outFile, png);
console.log(`gen-og: wrote ${outFile} (${race ? `race "${race.name}"` : 'off-season'})`);
