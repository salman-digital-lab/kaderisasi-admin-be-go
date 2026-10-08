# Internal talent assessment

Organization staff enter through **Profil Saya → Asesmen Bakat**. Every active
admin account, including accounts without assigned roles, can complete their own
assessment. Super Admins (including an additional Super Admin role) can read
completed results through **Akun Admin → Lihat Hasil Bakat**. They cannot inspect
or change another account's draft through these endpoints.

The versioned definition in `internal/talent/definition.json` contains the supplied
170 Indonesian statements and the explicit scoring key. It is an internal
reflection instrument, with an unofficial scoring key and 28 reconstructed
statements, not an official Talents Mapping report. The questionnaire API omits
talent mappings and reconstructed-item metadata to avoid suggesting answers.
Questions 11 and 12 deliberately use Positivity and Maximizer, respectively.

Each statement includes a short Indonesian description with everyday examples
from team meetings, community activities, family, and friendships. Descriptions
clarify the original statement without naming its talent or suggesting an answer.
Participants are told the examples are illustrative and to answer from their own
habits. This additive guidance retains `internal-170-v1`: IDs, scoring mappings,
and stored answers are unchanged, so existing drafts resume.
The frontend also accepts a definition without descriptions during API rollout.

Statements are worded in plain, everyday Indonesian: idioms and jargon (for
example "invisible hand", "indra keenam", "introspeksi", the SARA acronym) are
replaced with direct phrasing, and long sentences are shortened. Each rewording
keeps the original item's meaning, intensity words such as "selalu", and its
talent, so it also retains `internal-170-v1`. A change that alters what an item
measures needs a new version instead.

## Workflow and storage

One question per screen, six fully labeled choices, explicit Next/Back, a question
navigator, an all-answers-required review, and server-acknowledged autosave.
Unfilled draft answers are represented by 0; completed answers are integers 1–6.
Navigation position is saved along with answers. Pending edits remain in memory,
with retry/leave controls; browser/device recovery covers acknowledged saves.
No raw answers are persisted in browser storage.

Each admin account owns at most one row in `talent_assessment_drafts` and one in
`talent_assessment_results`. Creation is resumable. All writes lock the participant's
admin-account row in a transaction; draft ID and revision detect stale tabs.
The server computes scores, then replaces the current result and deletes the draft
atomically. A repeated submission for the current submission ID returns the same
result even if a retake draft has since started. No attempt-history table exists.
An unfinished retake leaves the previous result available. Retakes do not remove
previous results until their new submission succeeds.

Scoring: five values per talent, total 5–30, normalized `(total - 5) * 4`.
Descending score, count of 6, count of 5, then stable manual scoring-sheet order.
Equal scores are indicated in the results. Ranks 1–7/8–27/28–34 follow the supplied
three groups. Domain scores are averages of their talent scores, not percentiles.

Theme names stay in English in the scoring definition, API, and stored results;
they are the stable scoring keys. The dashboard's `theme-guide.json` adds an
Indonesian `label` for each of the 34 themes, and the report shows that label
(for example `Communication` → "Komunikasi"). Changing a label never affects
stored results; changing a scoring key would.

## Results report

The dashboard renders an original BMKA report with a participant header, top-seven
summaries, an interactive four-domain map, activity examples, support for ranks
28–34, and a dedicated full-ranking tab. Every theme opens a keyboard-accessible
explanation drawer. Score ties remain explicit; uniform scores do not produce a
claim about a dominant domain. Backend scoring and participant access are unchanged.

The frontend's `theme-guide.json` records the supplied **Penjelasan 34 Tema Bakat
Talents Mapping** PDF title and SHA-256. It contains all 34 theme descriptions,
340 activity examples, lower-theme guidance, and eight distinctions between
commonly confused themes. Domain descriptions use the PDF's Thinking, Influencing,
Relating, and Striving categories, mapped to the existing Indonesian domain names.
The report does not infer the eight-cluster PSP/PSS strength maps in the visual
references because this assessment has no corresponding scoring definitions.

## API

All routes require existing JWT authentication; mutations retain trusted-origin
checks. Ownership comes from the session, never a supplied account ID. Responses
use existing `{ message, data }` envelopes.

| Method | Route | Behavior |
| --- | --- | --- |
| GET | `/v2/talent-assessment/definition` | Version and participant-safe statements |
| GET | `/v2/talent-assessment` | Own draft and current result, either nullable |
| POST | `/v2/talent-assessment/draft` | Resume or create own draft |
| PUT | `/v2/talent-assessment/draft` | Save `{draft_id, revision, answers, current_question}` |
| POST | `/v2/talent-assessment/submit` | Submit `{draft_id, revision}` using stored answers |
| GET | `/v2/talent-assessment/result` | Own completed result |
| GET | `/v2/admin-users/:id/talent-assessment/result` | Super Admin-only read-only result with participant name |
| GET | `/v2/admin-users` | Existing account list; Super Admin rows also carry `talent_assessment_completed` (boolean) so the dashboard shows **Lihat Hasil Bakat** only for accounts with a stored result |

422 rejects malformed/out-of-range/incomplete input. 409 rejects stale draft IDs,
revisions or definition versions. 404 indicates no completed result. Definition
changes require a deliberate version/migration policy before introducing v2;
existing unsupported drafts are blocked rather than rescored against new keys.

## Deployment and verification

1. Run the additive `create_talent_assessments` Ace migration from
   `../kaderisasi-admin-be` against the intended environment.
2. Deploy the Go API, then the admin frontend. No new configuration or scheduler.
3. Verify an active staff account can start/save/resume and a Super Admin can open
   a completed result. Existing public API/models do not reference these new tables.

`make test-talent-assessment` runs isolated PostgreSQL workflow/authorization
checks, desktop/mobile browser scenarios, and native route coverage. It is included
in `make verify`. Finish standalone verification with `make clean-fixtures`.
Migration checks run with `MIGRATION_TEST_ENV=../env/test/admin-be-test-env npm test` in
the migration repository. Fixtures use owned schemas and explicit search paths.

The interface follows the dashboard's BMKA blue, Inter typography, square surfaces
and existing layout. Design review uses ENERGY 1 / RHYTHM 2 / MOTION 1: the statement
is the focal point, answer rows serve touch/keyboard selection, and results use
progressive detail without decorative motion or new visual dependencies.

The report separates Ringkasan, Peta Bakat, Pengembangan, and Semua Skor into keyboard-accessible tabs. Only the active panel is rendered; participant identity remains above the tabs.
