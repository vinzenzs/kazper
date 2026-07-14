// @ts-check
import { defineConfig } from 'astro/config';

// `site` + `base` come from CI (GitHub Pages URL). They only affect absolute
// URLs (the og:image tag); the page itself is origin-relative. Defaults keep a
// local build working without any env.
const site = process.env.SITE_URL || 'https://example.github.io';
const base = process.env.BASE_PATH || '/';

// https://astro.build/config — static output, zero JS by default (the countdown
// is a deliberate tiny inline island).
export default defineConfig({
  output: 'static',
  site,
  base,
  // No secret, no Kazper origin, and no runtime fetch reaches the browser: the
  // page renders only the build-time feed values embedded at build.
  build: { assets: 'assets' },
});
