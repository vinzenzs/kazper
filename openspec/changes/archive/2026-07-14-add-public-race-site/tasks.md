# Tasks

## 1. Scaffold `apps/public`

- [x] 1.1 Scaffold an Astro project at `apps/public/` (static output, no framework integration; `.astro` components only), with its own `package.json` and a `README.md` stating what it is and the CI-only build inputs
- [x] 1.2 Build-time feed fetch module: read `FEED_URL` + `FEED_SECRET` from env, `GET` with `X-Feed-Key`, parse `{race, days_remaining}`; throw (failing the build) on network error or non-200; tolerate `{race: null}`
- [x] 1.3 Gitignore the build output (`apps/public/dist`), mirroring `apps/web`

## 2. The page

- [x] 2.1 Hero page: race name, formatted `race_date`, prerendered `days_remaining` as the countdown fallback
- [x] 2.2 Countdown island: tiny vanilla JS computing whole-day difference from embedded `race_date` in the viewer's local timezone, floored at 0, replacing the prerendered number on load
- [x] 2.3 Off-season state: `{race: null}` renders the "between seasons" face (same shell, no countdown, no race name)
- [x] 2.4 Verify shipped assets contain neither `FEED_SECRET`'s value nor the Kazper origin, and the page makes no runtime request to it

## 3. Share card

- [x] 3.1 Build-time OG image generation: race name + build-time countdown; neutral variant for the off-season state
- [x] 3.2 Head metadata: `og:title` / `og:description` / `og:image` (+ `twitter:card`) referencing the generated image

## 4. Pipeline

- [x] 4.1 `.github/workflows/public-site.yml`: triggers = nightly cron (shortly after midnight in the athlete's tz, expressed in UTC), `workflow_dispatch`, `push` filtered to `apps/public/**`; build with `FEED_URL` (repo variable) + `FEED_SECRET` (Actions secret); deploy to GitHub Pages
- [ ] 4.2 Enable GitHub Pages for the repo (Actions deploy source); add `FEED_SECRET` secret + `FEED_URL` variable _**(operator follow-up — GitHub repo settings / live feed / browser; not executable from this environment)**_
- [ ] 4.3 Verify failure semantics: a run with a bad/missing secret fails the build and leaves the previous deploy serving _**(operator follow-up — GitHub repo settings / live feed / browser; not executable from this environment)**_

## 5. Docs & verification

- [x] 5.1 README: public-site topology note (CI-as-shield replacing the Strapi sketch), pointer to `apps/public`
- [ ] 5.2 End-to-end verify against the live feed: real build renders the current A-race and countdown; share a link to confirm the OG card resolves _**(operator follow-up — GitHub repo settings / live feed / browser; not executable from this environment)**_
- [ ] 5.3 Manual check of the stale-build guarantee: build once, view the page across a (simulated) date change with JS on/off _**(operator follow-up — GitHub repo settings / live feed / browser; not executable from this environment)**_
