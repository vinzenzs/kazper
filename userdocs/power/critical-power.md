# Critical power & W′

The 2-parameter critical power model summarizes your power-duration curve with two
physiologically meaningful numbers:

- **CP (critical power, W)** — the highest power that is metabolically sustainable; in
  practice ≈ FTP.
- **W′ ("W prime", kJ)** — the finite tank of work you can do *above* CP before exhaustion.

## The model

Total work at exhaustion is linear in time:

$$ W(t) = CP \cdot t + W' $$

Kazper fits this line (ordinary least squares in work–time form) to your **windowed best
efforts** between **2 and 30 minutes** — below 2 min the anaerobic term distorts the fit, above
30 min motivation and fueling cap the data, sprints and 60 m are excluded. CP is the slope, W′
the intercept.

## Quality gates & warnings

| Check | Result |
|---|---|
| fewer than 3 in-band durations | `model: null`, `reason: insufficient_points` |
| longest < 3× shortest duration | `model: null`, `reason: span_too_narrow` |
| fit R² < 0.5 | model returned **with `warning: "poor_fit"`** — treat as unreliable |

The response always includes the exact points used (duration, watts, source workout, date) and
fit quality (`r_squared`, `rmse_w`) — every number is auditable back to a ride.

## The estimate is advisory

CP never writes your config. The intended loop: the coach compares CP against your configured
FTP ("fits 285 W, configured 278 — retest or bump?"), you confirm deliberately, TSS gets
recomputed. The **CP history** endpoint runs the same fit at weekly anchors over a rolling
window (default 90 days) so you can see the estimate *move* across a season next to your
confirmed FTP steps.

!!! tip "Feeding the model"
    The fit is only as good as your best-effort envelope. Occasional honest hard efforts at
    ~3, ~8, and ~20 minutes keep the in-band ladder fresh; a base-season window may legitimately
    gate with `insufficient_points` — that's the model being honest, not broken.
