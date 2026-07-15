# Durability

Fresh power is one thing; power **after 2000 kJ of work** decides long races. Durability
re-computes your best efforts *tiered by the work already done* when the effort started.

## How it works

At stream ingest, besides the fresh ladder, Kazper stores the best 1-, 5-, and 20-minute efforts
whose window began **after** cumulative work reached each tier: **500 / 1000 / 1500 / 2000 kJ**
(only tiers the ride actually reached). The durability read then reports, per duration:

- the **fresh** windowed best,
- each tier's best with its **fade**:

$$ \text{fade}_{\%} = \frac{P_{\text{fresh}} - P_{\text{tier}}}{P_{\text{fresh}}} \times 100 $$

Each entry names its source ride. Fresh and tiered bests usually come from *different* rides —
that's inherent to windowed analysis and visible in the response.

## Reading it

- Small 20-min fade at 1500–2000 kJ → strong endurance resilience; race plans can be
  aggressive late.
- Fade concentrated at deep tiers → the limiter is fueling/durability, not threshold — long
  rides with quality late, and race-nutrition rehearsal, beat more FTP work.

## Empty at first

Historical rides only gain tiered data when their streams are re-processed (`no_tiered_data`
tells you the window predates the feature). New syncs tier automatically; a one-time recompute
sweep backfills the archive.
