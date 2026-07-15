# Chat, MCP tools & context

The coach is an LLM with the same view of your data you have — every tool it holds is exactly
one API call, forwarded verbatim.

## Context bundles

Two composed reads ground most conversations:

- **`/context/daily`** — today at a glance: nutrition vs goals, hydration, recovery signals,
  your wellness entry, supplements taken, active coach memory.
- **`/context/training`** — the training picture: current phase (with its methodology prose),
  season/macrocycle position and days-to-race, fitness snapshot (VO₂max, load, predictions),
  your physiology config, recent load by sport, recent and upcoming workouts.

## Write confirmation

In chat, any tool that would *change* something (log a meal, update a goal, schedule a workout,
adjust thresholds) pauses for your explicit confirmation before dispatching. Reads never pause.
The principle: **the coach proposes, you decide** — nothing about your plan, goals, or
physiology changes because a model felt confident.

## Coach memory

A dated, queryable store of things worth keeping across sessions: recommendations (windowed),
plus standing facts, preferences, constraints, and observations — each with a review/expiry
lifecycle. Writes are always explicit ("remember that…"); the database is the shared brain
across chat and MCP, while conversation transcripts stay private to their surface.

## What the coach can compute vs what it reads

Everything in this guide — TSS, PMC, CP, W′bal, compliance, EA, correlations — is computed by
the *server* and read by the coach. That's deliberate: the numbers are reproducible,
spec-tested, and identical no matter which surface asks. The coach's job is judgment on top:
sequencing your week, adjusting for how you feel, and turning a fade table into "we're doing
race-pace work at the end of long rides this block."
