# W′ balance

If W′ is your anaerobic battery, **W′bal** is the battery gauge over one ride: how depleted it
got during each hard effort and how much was left at the end — the difference between "finished
the fifth rep comfortably" and "finished with 2 kJ in the tank".

## The model

The differential (Froncioni–Clarke–Skiba) form over the stored 1 Hz power stream, starting full
at W′:

- **Above CP:** the battery drains at \( (P - CP) \) joules per second.
- **Below CP:** it recharges toward full: \( \Delta = (W' - bal) \cdot \frac{CP - P}{W'} \) per
  second — recovery is fast when you're deeply drained and riding well below CP, slow near full.

## Parameters are explicit

You supply `cp_watts` and `w_prime_kj` with the request — typically straight from the
[CP model](critical-power.md). Nothing is auto-fitted or read from config: the same ride with
the same parameters always gives the same answer, and the response echoes what it was given.

## What you get

A summary — minimum balance (and when it happened), end balance, maximum depletion %, time spent
below 25 % — plus the full series for charting (downsampled on request).

## Negative balance is a message, not an error

The balance is **not clamped at zero**. Going negative means the ride demonstrated more
above-CP work than the supplied W′ allows — i.e. your parameters are stale (CP or W′ too low).
That's exactly the signal to re-run the CP model, so Kazper shows it instead of hiding it;
`max_depletion_pct` can exceed 100 accordingly.

## Reading it for training

- Intervals that end near 0 kJ → the session was truly maximal; progressing it means more
  recovery or fewer reps, not more power.
- Intervals that never dip below ~50 % → headroom; the prescription can grow.
- Time-below-25 % is a decent "how deep did we really go" scalar to track across repeats of the
  same session.
