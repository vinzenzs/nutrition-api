# Design: add-workout-reconciliation

## Context

After `add-training-plan` and `add-garmin-bridge`, a training day has a planned
workout (`status='planned'`, `plan_slot_id`, `template_id`, no `external_id`, no
actuals) and — once done — a separate imported activity (`status='completed'`,
`external_id='garmin:<id>'`, actuals, no plan link). They are the same session
expressed twice. `workouts.status` already models a `planned → completed`
lifecycle on one row, so the fix is to let the import *fulfill* the planned row
rather than create a sibling. This change adds that reconciliation to the Garmin
ingestion path and gives explicit fulfill/unfulfill controls for the cases
auto-matching shouldn't decide.

## Goals / Non-Goals

**Goals:**
- Merge a confidently-matched Garmin import into its planned workout, preserving
  the prescription (`template_id`/`plan_slot_id`) and gaining the actuals.
- Stay idempotent across the daily re-sync.
- Never silently mis-merge: ambiguous cases stay two rows, flagged.
- Provide manual fulfill/unfulfill for the ambiguous and mistaken cases.

**Non-Goals:**
- No day-of tolerance window (strict same-local-day v1).
- No reverse-direction matching (materialize → pre-existing activity); deferred.
- No adherence analytics (separate future capability on top of this).
- No bridge change — the bridge keeps POSTing activities as today.

## Decisions

### D1: Merge model — fulfill the planned row in place

On match, the planned row is updated, not duplicated:

```
planned row (status=planned, plan_slot_id=S, template_id=T, external_id=NULL)
   ── merge ──▶
same row     (status=completed, plan_slot_id=S, template_id=T,
              external_id=garmin:123, source=garmin, + kcal/avg_hr/tss/distance/power/…)
```

The prescription is preserved via `template_id` (the template's steps are the
prescription; `workouts` stores no step-level actuals, so nothing is lost by not
keeping a second row). This is why merge is chosen over keep-both-plus-FK: the
link model costs a row and a join for fidelity the table never stored.

### D2: Match only on first sight, in the ingestion service

Reconciliation runs inside the `workouts` create/bulk service, before the insert:

```
incoming activity (source=garmin, external_id=garmin:123)
  ├─ external_id already present in DB → existing external_id UPSERT (idempotent re-sync); DONE
  └─ first sight → candidate query:
        status='planned' AND external_id IS NULL
        AND sport = activity.sport
        AND date(started_at AT TIME ZONE local) = date(activity.start AT TIME ZONE local)
     ├─ exactly 1 candidate → MERGE into it (D1)
     ├─ 0 candidates        → INSERT standalone completed row  (today's behavior)
     └─ ≥2 candidates       → INSERT standalone, set needs_link flag (D4)
```

Keeping this in the backend keeps the bridge dumb (it just forwards activities)
and means any future importer (Strava, Apple Health, manual) gets reconciliation
for free.

### D3: Idempotency and the materialize interaction

- **Re-sync**: once merged, the row owns `external_id=garmin:123`; the next daily
  sync's POST takes the existing `external_id` UPSERT branch and updates actuals
  in place. Reconciliation never re-runs for that activity. ✓
- **Re-materialize**: the merged row is `status='completed'` but still carries
  `plan_slot_id=S`. `add-training-plan`'s materialize upsert is guarded by
  `WHERE status='planned'`, so it skips this row — no revert, no clobber. (That
  guard is already in `add-training-plan`; this change depends on it.) ✓

### D4: Ambiguity is flagged, not guessed

When >1 open planned workout matches (e.g. two easy runs the same day, or a
not-yet-handled brick edge), auto-merge is unsafe. The activity is inserted as a
standalone completed row carrying a nullable `needs_link` marker so the app/agent
can surface "this completed activity may fulfill a planned session — link it?".
This is the one possible schema addition:

```
ALTER TABLE workouts ADD COLUMN needs_link BOOLEAN NOT NULL DEFAULT false;
```

(Alternative considered: derive "needs link" at query time from "completed +
unlinked + a same-day planned of same sport exists". That avoids a column but
pushes a non-trivial correlated check into every read. A stored flag set once at
ingestion is simpler and cheap; chosen.)

### D5: Explicit fulfill / unfulfill

- `POST /workouts/{plannedId}/fulfill {completed_id}` — merge an existing
  completed activity into an existing planned workout: copy the completed row's
  `external_id`/`source`/actuals onto the planned row, flip it to `completed`,
  delete the now-redundant standalone completed row (or vice-versa — see below),
  clear `needs_link`. Used for ambiguous and cross-day cases the auto-path
  declined.
- `POST /workouts/{id}/unfulfill` — reverse a merge: clear `external_id`,
  `source`→prior/`manual`, and the actuals, restore `status='planned'`. Used to
  correct a wrong auto-merge. (The activity is not re-fetched; if needed the next
  Garmin sync re-imports it as a fresh standalone row.)

**Which row survives a merge?** The **planned** row survives (it holds the plan
links); the standalone completed row is removed. This keeps `plan_slot_id`
stable so the slot stays the single addressable identity for that session.

### D6: Local-day comparison and timezone

Matching compares **local calendar days**, not UTC instants — a 21:00 activity
and a planned session both belong to that local date. The service uses the
configured local timezone (same one the daily-context/summary boundaries already
use) for the `date(... AT TIME ZONE local)` comparison. Sport equality is exact
(`run`=`run`); no cross-sport matching.

## Risks / Trade-offs

- **Wrong auto-merge** on a genuinely-ambiguous-but-looks-singular day. Mitigated
  by strict exactly-one matching + `unfulfill`. The flag path covers the rest.
- **Early/late sessions** (planned Tuesday, done Monday) won't auto-match under
  strict same-day. Acceptable v1; `fulfill` handles it manually. A tolerance
  window is a later refinement.
- **Imported-before-planned** ordering leaves two rows that auto-recon won't fix
  (materialize doesn't look back at activities). Documented non-goal; `fulfill`
  is the manual remedy until the reverse direction is built.

## Migration Plan

At most one additive column (`needs_link BOOLEAN NOT NULL DEFAULT false`) — only
if D4's stored-flag option is taken (recommended). Otherwise no migration. No
backfill. Down migration drops the column.

## Open Questions

- Should `unfulfill` trigger an immediate re-import of the activity, or just
  restore the planned row and let the next sync re-create the standalone? Current
  call: the latter (simpler, no bridge coupling).
- Tolerance window (±1 day) for auto-match — defer until the strict version shows
  it misses real sessions.
