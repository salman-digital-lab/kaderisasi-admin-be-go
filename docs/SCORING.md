# Activity scoring

Activities optionally define a rubric through the admin **Penilaian** tab. Scores
belong to a registration, including guest registrations. They do not affect
registration status, member levels, leaderboards, or certificate eligibility.

## Storage and calculation

Ace migration `1789268672656_create_create_activity_scorings_table.ts` in the
maintenance repository adds `activity_registrations.scoring_data` (nullable
JSONB), `activity_scoring_rubrics`, and `activity_scoring_publications`.
Existing registrations remain SQL NULL; questionnaire answers and guest data
remain separate. Never update scoring JSON through general registration APIs.

The JSON schema version is 1. It contains `revision`, `state`, `draft` (criterion
ID to score map and participant note), `published` (nullable immutable snapshot),
and editing actor/timestamp. Published snapshots contain their rubric, entered
scores, calculated results, publication revision, actor and timestamp. History
rows append either a publication snapshot or withdrawal event. Activity and
registration deletion is blocked when publication history exists.

Each criterion normalizes `score / maximum * 100`. The total is the weighted
average of the unrounded normalized values. Exact rational arithmetic avoids
binary floating-point boundary errors. Criterion results and totals round to two
decimal places before grade lookup. The largest qualifying minimum threshold
wins. Grades are optional; configured grade scales must include zero. Maximums
and weights must be positive, finite values with at most two decimal places.
Limits: 30 groups, 100 criteria total, 30 grades, 5,000 characters per note, and
1,000,000 for maximums/weights. Null or omitted criterion values mean unscored.

The first non-null score locks the entire rubric, including its shared note.
Activity-row locks serialize rubric changes, scoring saves, imports, and
publication; registration revision comparisons prevent lost updates. Bulk
publication and imports commit all selected changes or none. Draft corrections
do not modify the current published result until republished. Withdrawal hides
the published result and preserves all history.

## HTTP interface

All admin paths begin `/v2/activities/:id/scoring`, require an admin JWT, and use
private/no-store responses. Successful JSON responses use `{ message, data }`.

| Method and suffix | Purpose | Permission |
| --- | --- | --- |
| GET `/rubric` | Rubric or null | `activities.read` |
| PUT `/rubric` | Save `{groups, grades, note, revision}` | `activities.manage` |
| GET root | Entries; `page`, `per_page`, `search`, `state` | `activities.read` |
| PUT `/registrations/:registrationId` | Save `{revision, rubric_revision, draft}` | `activities.registration.manage` |
| POST `/publish` | Publish `{rubric_revision, selections:[{registration_id, revision}]}` | `activities.registration.manage` |
| POST `/withdraw` | Withdraw with the same selection envelope | `activities.registration.manage` |
| GET `/excel?mode=template\|draft\|published` | Download workbook | `activities.read` |
| POST `/import/preview` | Multipart `file`; changes, row/cell errors, hash | `activities.registration.manage` |
| POST `/import/commit` | Same `file` plus `preview_hash` | `activities.registration.manage` |

Initial revisions are zero. Stale revisions return 409; invalid input returns
422; missing activities or registrations outside the activity return 404.
List states are `unscored`, `incomplete`, `complete`, `published`, and `changed`.

Excel templates retain activity ID, rubric revision, registration revisions,
and criterion IDs. Names are informational. Templates have two heading rows;
data starts at row 3. Blank cells preserve saved scores/notes; use the form to
clear a value. Score cells accept decimal points or decimal commas. Formulas,
duplicate registration rows, unknown columns/IDs, and out-of-range values are
rejected. Only template-mode workbooks can be imported. Limits are 5 MiB upload,
32 MiB uncompressed contents, and 10,000 participant rows. The commit reparses
the same bytes against current data and checks the preview hash inside the
transaction. An import never publishes.

The public Adonis registration endpoint returns only `scoring_result`, selected
from the current snapshot for the authenticated owner. General model
serialization hides the entire `scoring_data` column. No guest result links are
provided. Future certificate work can copy a specific publication snapshot;
this phase makes no changes to certificate rendering or issuance.

## Verification and rollout

UI direction follows existing activity setup, participants, and profile activity
pages: ENERGY 1, RHYTHM 1, MOTION 2 (existing control/dialog transitions only).
The existing primary color identifies save/publish actions; green marks actual
publication state. Existing typography and 16px section gaps preserve reading
hierarchy. Rubric group cards keep related fields together; participant tables
support comparison, with horizontal scrolling confined to the table. Published
results use the same bordered card, radius, and heading sizes as profile details.
No decorative illustrations or additional icon system are introduced.

Run `make check`, `make test-unit`, and `make test-scoring`. The scoring target
uses owned test schemas, real Go and Adonis APIs, and desktop/mobile browser
checks. Existing recognized workspace processes are borrowed and restored.
When reusing a separately started local admin Vite server, run
`node scripts/scoring-workflows.mjs --browser --borrow-workspace --reuse-admin-frontend`;
the harness verifies that its API URL is localhost:3334 before reusing it.
Then run `node scripts/coverage.mjs --native=scoring --check`.

Evidence is written to `.artifacts/scoring.json` and
`.artifacts/scoring-browser/`. Fixture records are cleaned in the harness's
finally block; `make clean-fixtures` removes the recorded owned schemas.
Run the maintenance repository's lint/typecheck/build and
`MIGRATION_TEST_ENV=../docs/.env.test.be npm test` for migration validation.

Deploy the additive Ace migration in the selected environment first, then both
APIs, then both frontends. The Go schema file is only a sqlc snapshot and must
not be executed as a migration. Existing activities need no backfill. Rollback
application code without dropping scoring data or publication history.
