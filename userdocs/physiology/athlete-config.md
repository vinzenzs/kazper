# Athlete config & threshold history

## The config singleton

One record holds your physiology: **FTP** (W), **lactate-threshold HR**, **max HR**, **threshold
run pace** (sec/km), **CSS** (critical swim speed, sec/100 m), and your HR- and power-zone
boundaries. Nearly every derived number in this guide reads from it — TSS derivation, zone
resolution for structured workouts, race pacing bands, step compliance.

Updating it is a **full replace**: you (or the coach, with your confirmation) write the complete
new state. There is no partial patch — that's what keeps the history meaningful.

## Threshold history

Every time a config write actually *changes* your physiology, Kazper snapshots the full state
into an append-only history keyed by date. This makes progression a data question: "FTP
240 → 255 → 270 over the season" is a query, not memory. Details worth knowing:

- A write that changes nothing appends nothing (the daily Garmin re-write of an unchanged
  config used to stay silent this way).
- Same-day corrections replace that day's snapshot; a same-day revert deletes it — the history
  never stores two consecutive identical states.
- History answers *"what did I confirm, and when"*. It is not a log of every number Garmin ever
  detected.

## Where the values come from

Garmin auto-detects several of these (cycling FTP, LTHR, threshold pace, max HR) and the bridge
mirrors them daily. **Known issue / in-flight change:** historically the bridge *wrote* these
straight into your config, which could silently overwrite values you'd confirmed and clear
fields Garmin doesn't know (like CSS). The queued `separate-garmin-threshold-detection` change
splits this: Garmin's numbers land in a separate *detected* record, your config stays yours, and
a per-field **source selector** lets you decide — per threshold — whether computations use your
confirmed value or Garmin's latest detection.

!!! tip "Cross-checking your FTP"
    The [critical power model](../power/critical-power.md) gives you a data-derived estimate of
    CP (≈ FTP) from your actual best efforts — the coach's tool for "your configured 278 W looks
    about right / looks stale".
