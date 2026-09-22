# Certificate score sheets

Certificates issued with a participant's published activity scores contain two PDF
pages: the saved certificate artwork and an A4 portrait score sheet. Page two lists
rubric groups, criteria, raw scores and maxima, weights, normalized scores, optional
grades, the weighted final result, and published notes. It uses the published
scoring calculation; it never recalculates grades in the browser.

Publish scores before issuing certificates or requesting signer approval. Draft
corrections remain private until published. The score sheet is saved under
`participant_snapshot.scoring_result`, using the existing JSON snapshot column;
no database migration is needed. Changes to published scores invalidate a pending
approval's content hash. Issued certificates keep their original scores even after
correction, republication, or withdrawal of activity scores.

Existing certificates and certificates issued without published scores keep their
single-page format. The system does not append current grades to historical
certificates. Score sheets are visible in admin issuance and approval previews,
admin downloads, and authenticated owner previews/downloads. Anonymous certificate
pages and verification responses omit scores and notes.

The admin and public frontends are separate applications and maintain matching
`CertificateScoreSheet` components and PDF exporters. Keep their layout changes in
sync. Longer rubrics use two table columns and fit within page two without dropping
rows. Very large rubrics or lengthy notes result in smaller print text.

## Visual direction and validation

Design read: an official participant score sheet using the existing certificate
typography and teal accent. ENERGY 1 / RHYTHM 1 / MOTION 1.

- White paper supports printing; teal `#0f766e` matches the certificate starter
  template and identifies headings and totals.
- A serif title connects the page to the certificate, while Arial keeps numeric
  table columns legible across both applications.
- Grouped tables follow the scoring rubric. The participant's name and final
  result provide the hierarchy; margins separate identity, results, notes, and
  the certificate code. No decorative assets or animations are added.

Run the native checks from `kaderisasi-admin-be-go`:

```sh
make check
make test-unit
make fixtures
node scripts/test-go.mjs ./internal/httpapi -run 'TestCertificate(ScoreSnapshot|ApprovalWorkflow|IssuanceSnapshotsAndRevocation)$'
node scripts/scoring-workflows.mjs --certificate-scores-browser --borrow-workspace
make clean-fixtures
```

`--reuse-public-frontend` may be added when the existing public development server
uses the local test API on port 3333. The browser harness leaves that server running.
It exercises admin and owner downloads, both screen widths, all 100 criteria in a
dense fixture, zero scores, Unicode notes, legacy single-page downloads, and
anonymous score privacy. Generated PDFs and screenshots are recorded under
`.artifacts/certificate-scores/`; `.artifacts/scoring.json` records API and browser
checks and fixture cleanup.

Frontend validation uses each application's lint/build commands, the public
frontend typecheck, and certificate unit suites. The public backend uses lint,
typecheck, and `npm test -- --files=tests/unit/certificate_serialization.spec.ts`.

## Validation record (2026-09-22)

- PASS: Go `make check`, all race-enabled unit tests, and the three targeted
  issuance, score-snapshot, and approval integration tests.
- PASS: 110 native scoring/certificate API and browser checks on the final source.
- PASS: admin lint/build and 18 certificate unit tests; public frontend
  lint/typecheck/build and 43 certificate tests; public backend lint/typecheck
  and five certificate serialization tests.
- PASS: downloaded admin and owner PDFs each have exactly two pages; the legacy
  fixture has one. Poppler-rendered first/second pages and the 100-criterion page
  were visually inspected for clipping, overlap, and Unicode rendering.
- PASS: `make clean-fixtures` removed all owned schemas. Existing public frontend
  on port 3000 was reused and left running.

Visual delivery gate for the added score sheet:

| Gate | Result and evidence |
| --- | --- |
| Hard rules | PASS: 390px/1440px browser assertions found no page overflow; score text has 5.47:1, 7.56:1, or 14.68:1 contrast against white; absent scores omit the sheet. No new interactive controls, navigation, claims, or invented production data. |
| Purpose | PASS: paper, teal, typography, grouping, and spacing follow the reasons above; no decorative imagery or animation. |
| Hierarchy | PASS: participant identity, grouped criteria, weighted total, and supporting notes were checked in both frontends and rendered PDFs. |
| Craft and functionality | PASS: admin and owner download buttons produced the expected PDFs; legacy download produced one page; anonymous pages omitted scores. Both applications built successfully with no browser page errors in the certificate run. |

The browser harness uses synthetic fixture names and grades only. Its PDF files
are verification artifacts, not issued participant documents for distribution.

## Certificate workflow UX

The on-screen preview separates **1. Sertifikat** and **2. Hasil penilaian** with
keyboard-accessible tabs. Screen scores use normal text, grouped criteria and
labelled values that reflow into two columns on phones. The weighted final result
appears first. The public activity result uses this same presentation. The PDF
continues to use the fixed paper layout; the admin keeps its export source outside
the visible and accessible preview, and the owner exporter mounts it on demand.

Downloads are available above the preview, with a stable button label, a live
progress message and a persistent error with a retry action. Owners see the page
count and learn that sharing the certificate link does not expose their scores.
Legacy certificates explain why the download has one page.

The recipient step explains that scores must be published before issuance or
approval and links to the activity's scoring screen. Review identifies the sample
participant and their page count, explicitly avoiding a claim about every selected
participant. A missing published result is shown as a warning. Signer review asks
the signer to check both pages when scores are present.

Design read: certificate review for administrators and participants, using the
existing Ant Design and Mantine page styles. ENERGY 1 / RHYTHM 1 / MOTION 1.

- Use the existing certificate approval theme throughout admin issuance and
  preview so blue controls and secondary text remain readable on white.
- Preserve application fonts, card borders, radii and spacing to match adjacent
  pages; emphasize the final result with size and weight rather than decoration.
- Put actions first and use tabs to avoid scrolling through two scaled paper
  pages. Label/value groups keep all score fields readable at narrow widths.
- Keep paper typography in the PDF while allowing screen content to reflow,
  following [W3C reflow guidance](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html).

The browser harness also checks keyboard navigation, score font sizes and overflow
at 320px, 390px and 1440px, 44px download/tab targets, download retries after a
simulated server failure, axe accessibility checks, issuance guidance, navigation
to scoring, and matching activity/certificate score presentations.

UX validation record (2026-09-22):

- PASS: both frontend lint/build checks, public typecheck, 49 admin certificate
  unit tests and 43 public certificate unit tests.
- PASS: 114 scoring/certificate API and browser checks on stable sources, including
  successful admin/owner retry downloads, keyboard tabs, narrow-screen text and
  touch targets, single-page guidance and anonymous score privacy.
- PASS: axe found no violations in the admin preview main content and the visible
  score panels in both applications for the tested WCAG A/AA rules. This is scoped
  automated evidence, not a platform-wide accessibility certification.
- PASS: desktop/mobile certificate, issuance and activity-result screenshots were
  inspected. Admin and owner PDF pages rendered correctly with Poppler after
  downloading from the score tab; the inactive artwork tab did not affect export.
- PASS: the owned test servers stopped and `make clean-fixtures` removed all owned
  schemas. Port 3000 was free for this follow-up run, so the harness started its
  own public frontend and stopped it afterward.

| UX delivery gate | Evidence |
| --- | --- |
| Hard rules | PASS: no horizontal overflow at tested widths; score text stays at least 14px; tabs and downloads have 44px targets; scoped axe and keyboard checks pass. |
| Purpose | PASS: actions precede previews, result values reflow, and existing certificate theme tokens replace low-contrast controls. Reasons are recorded above. |
| Hierarchy | PASS: the final score leads the details, criteria follow their rubric groups, and notes retain readable application typography at desktop/mobile widths. |
| Craft and functionality | PASS: builds, 92 frontend unit tests and 114 workflow checks; retry, scoring navigation, two-page PDF output and legacy behavior verified. |
