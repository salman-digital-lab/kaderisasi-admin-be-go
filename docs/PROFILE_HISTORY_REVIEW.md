# Education and work history review

Reviewed and fixed on 10 September 2026.

The review covered profile editing, onboarding, public self-submission, custom-form profile and guest sections, admin member lists/details, activity participant lists/details, the two APIs, PostgreSQL JSONB handling, and registration Excel exports. Existing migration/import scripts were inspected to establish the older education format, including missing faculty and nullable intake year.

## Findings and fixes

| Priority | Bug | Result |
| --- | --- | --- |
| P1 | Admin required every education field although the public API and historical imports produced partial entries. | Admin accepts those entries. Missing years remain absent instead of becoming zero; missing text fields normalize to empty strings. |
| P1 | Readers called array methods on JSON strings or objects, and dereferenced null history entries. | Profile forms, onboarding, member tables, and both APIs normalize history values and retain valid sibling entries. |
| P1 | Registrant details passed guest history objects directly to React, and omitted member histories from the profile field map. | Member and guest education/work histories render as readable text. Other structured answers are also safe to render. |
| P1 | Current-education selection and the full history editor maintained separate copies and could overwrite edits. | Both use the same history list; selecting an existing entry preserves edits and moves the selected entry to the final position. |
| P1 | Going back to the profile step reloaded the original profile and could overwrite an earlier successful save. | The form retains the profile returned by the successful update. |
| P1 | Cancelling a new history entry left it in the form. Deleting another entry cleared the active edit snapshot, so cancelling could remove an existing entry. | New entries are removed on cancellation; existing snapshots survive sibling deletion. Adding or opening another edit in the same history is disabled while editing. |
| P1 | A saved activity requirement named `current_education` was treated as a physical profile column, causing a SQL error. | The registration list derives the selected education from the history, with a guest-data fallback. |
| P1 | Null entries could abort the entire Excel export. Guest work history was omitted, and guest education history was not used as a current-education fallback. | Exports tolerate invalid sibling entries and include the available guest histories. |
| P2 | Year handling differed between forms and APIs; fractional/reversed years and whitespace-only jobs could be saved. Clearing a number input could send an empty string. | Shared validators within each application accept optional empty years, reject fractional/out-of-range years, require job title/company, and reject an end year before the start year. |
| P2 | The public profile form displayed unsuccessful server-action results as successful notifications. | Notifications use the returned success state. |

Partial education is supported without inventing a degree or year. Onboarding still asks users to complete its required education fields, and prefills missing years as null. Calendar bounds match the existing profile editors: 1900 through the current year plus 10.

History remains a JSON array of objects using snake_case keys:

```json
{
  "education_history": [
    {
      "degree": "bachelor",
      "institution": "ITB",
      "faculty": "",
      "major": "Physics",
      "intake_year": 2017
    }
  ],
  "work_history": [
    {
      "job_title": "Engineer",
      "company": "Company",
      "start_year": 2021
    }
  ]
}
```

Omitting a history in an update preserves the stored value. Sending an empty array clears it. An absent work end year denotes ongoing work. The final education entry remains the selected/current education; this patch does not reorder records by year.

The Go endpoint uses `memberProfileUpdateValidator` for the corrected rules. The historical Vine schema and its fixtures remain unchanged as a reference, rather than regenerating them to conceal intentional behavior changes.

## Verification

All checks below ran against an isolated checkout containing this patch. Concurrent course and certificate work in the shared workspace produced unrelated compile/lint failures during initial checks. All 44 implementation and regression-test files were compared byte-for-byte with the shared workspace after verification and matched.

| Check | Result |
| --- | --- |
| Public frontend lint and typecheck | Passed |
| Public frontend unit suite | 104 passed |
| Public frontend production build | Passed as part of the browser harness |
| Admin frontend lint, typecheck, production build | Passed |
| Admin frontend unit suite | 100 passed |
| Public backend lint, typecheck, build | Passed |
| Public backend default test suite | 28 passed, 14 unrelated opt-in integration tests skipped |
| Go `make check` | Passed: formatting, vet, builds, dependency verification, sqlc consistency, harness syntax, contract-normalization checks |
| Go `make test-unit` | Passed, with race detection |
| Real Go/public API and PostgreSQL history checks | 21 passed, including cross-backend writes/reads, omitted fields, explicit clearing, validation rejection, historical shapes, virtual current education, and an actual XLSX export |
| Admin browser regression | Desktop and mobile passed, including partial-record editing and guest history display |
| Public profile browser regression | Desktop and mobile passed, including cancellation, validation, saving, reloading, and clearing |
| Custom-form browser regression | Desktop and mobile passed, including preserving edits across selection and back/forward steps |
| Browser runtime errors | None in the six final scenarios |
| Whitespace/diff checks | Passed |

Relevant commands from the Go repository:

```sh
node scripts/ensure-fixtures.mjs
node scripts/profile-history.mjs --borrow-workspace
node scripts/browser.mjs --borrow-workspace profile-history.spec.mjs
node scripts/browser.mjs --borrow-workspace --public history-custom-form.spec.mjs profile-history.spec.mjs
make clean-fixtures
```

API evidence is in [profile-history.json](../.artifacts/profile-history.json).
Admin browser evidence is in [the admin report](../.artifacts/browser/2026-09-10T10-53-19-566Z/report.json).
Final public browser evidence is in [the public report](../.artifacts/browser/2026-09-10T10-57-00-798Z/report.json).

The owned test schemas were cleaned and borrowed workspace services restored. No production data was rewritten, no schema migration was needed, and this patch has not been deployed.
