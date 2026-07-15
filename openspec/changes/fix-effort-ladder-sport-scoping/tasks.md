# Tasks

## 1. Sport predicates

- [x] 1.1 `Repo.Curve`: bind `power` → `w.sport = 'bike'`, `speed` → the caller's run/swim sport (thread the sport through the call sites: power-curve handler, cp-model, cp-model-history, power-profile); multisport matches nothing
- [x] 1.2 Durability query: hard-bind `w.sport = 'bike'`
- [x] 1.3 Regression tests (the bug's shape): seeded running-power workout (450–540 W spikes) beside bike efforts → excluded from curve/CP points/profile anchors/durability tiers; bike speed excluded from run pace curve; emptied-window degradation (CP gates instead of fitting foreign data)

## 2. Fit-quality warning

- [x] 2.1 `warning: "poor_fit"` at `r_squared < 0.5` on cp-model + per history anchor; unit tests both sides of the threshold; MCP tool description notes
- [x] 2.2 `task swag` (new response field)

## 3. Verification

- [x] 3.1 `task vet` + effortanalytics suite green; golden regen only if tool descriptions changed
- [ ] 3.2 **Live validation against the reported case** _(operator/next-session — needs production API access; the regression fixture reproduces the case: fake CP 60.7 W / r² 0.055 vs the live 56.9 / 0.052)_: `cp_model` over the same window no longer sources points from workout `5dca291a` (Schandorf run); CP lands in a plausible band vs configured FTP 278 W (or gates honestly if bike data is thin); `power_profile` anchors are bike-only
