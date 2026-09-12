# Form builder verification

Verification date: 12 September 2026. The implementation spans the admin frontend, Go admin API, public frontend, and Adonis public API. No migration, API route, or dependency was added. Deploy in the order described in [FORM_BUILDER_ROUTING.md](FORM_BUILDER_ROUTING.md).

## Automated checks

| Area | Result | Evidence |
| --- | --- | --- |
| Go static checks and race-enabled unit tests | PASS | `make check && make test-unit`: formatting, vet, builds, generated queries, harness syntax, normalization integrity, and all Go packages passed. |
| Go affected HTTP integration | PASS | `node scripts/test-go.mjs ./internal/httpapi -run 'Test(FormRoutingRoundTrip\|ClubAndCustomFormLifecycle\|CustomFormAttachmentConcurrency)$'`: metadata create/update/read, invalid references, lifecycle, and concurrent attachment checks passed. |
| Admin lint and production build | PASS | `npm run lint -- --ignore-pattern '.course-ux-preview.local/**'` and `npm run build`. The ignore applies only to another task's local Course snapshot; application source remains checked. |
| Admin affected Vitest | PASS | `npx vitest run src/utils/form-routing.test.ts src/pages/CustomForm/CustomFormEdit/utils/builder-state.test.ts`: 15 tests. |
| Public frontend lint and typecheck | PASS | `npm run lint && npm run typecheck`. |
| Public affected Vitest | PASS | `npx vitest run src/features/customForm`: 15 tests. |
| Adonis lint, typecheck, and build | PASS | `npm run lint && npm run typecheck && npm run build`. |
| Adonis Japa unit suite | PASS with skips | `npm test -- unit`: 42 passed, 15 database/storage-gated tests skipped. The routing and custom-submission cases passed. Skipped tests are not counted as successful integration checks. |
| Shared Go/Adonis database | PASS | `node scripts/shared-database.mjs --borrow-workspace`: 95 checks against both real APIs; [record](../.artifacts/shared-database.json). |
| Admin browser | PASS | Six cases at 1440px and 390px; main/draft cases passed in [full run](../.artifacts/browser/2026-09-12T12-49-44-689Z/report.json); both control cases passed in the [focused rerun](../.artifacts/browser/2026-09-12T12-53-26-419Z/report.json). |
| Public browser and production build | PASS | Ten checks at 1440px and 390px; `form-routing.spec.mjs`, `clubs.spec.mjs`, and `history-custom-form.spec.mjs`; [report](../.artifacts/browser/2026-09-12T12-47-32-592Z/report.json). |
| Public validation contrast follow-up | PASS | Both club browser cases and the production build passed after darkening form error text; [report](../.artifacts/browser/2026-09-12T12-54-20-317Z/report.json). |

All four copies of `form-routing.fixtures.json` / `routing.fixtures.json` have SHA-256 `b4a64dbb8c06145566339710cc49e98a996de4bf3918ff7ea7fcf3923887446d`. The shared API suite consumes the same cases, checking member and guest submission results in PostgreSQL and pruned club responses through Go. It also checks response-edit validation and Go's refusal to reopen a club with malformed stored routes.

The shared fixture originally attempted to create an already-published activity, which the existing Go workflow rejects. Its setup now creates a draft and explicitly sets the publication state on its owned fixture record. Publication readiness remains covered by its existing dedicated suites.

## Historical compatibility results

These comparisons are **not fully passing**. The reference revision and comparison rules were left intact.

- Forms: **89/104 equivalent, 15 differences**, [report](../.artifacts/contracts/forms-report.json). The differences concern the existing activity-attachment guard: malformed/nonexistent activity identifiers, diagnostic SQL text, and subsequent attachment/timestamp states. The relevant guard and actions were already present in `HEAD` and were not changed by this feature.
- Clubs: **61/106 equivalent, 45 differences**, [report](../.artifacts/contracts/clubs-report.json). Every reported difference includes the pre-existing activity fixture's registration flag: historical creation opens registration; the current Go implementation creates a closed draft. The first scenario also differs in the explicit publication/opening response fields. Club routing/readiness introduced no additional response difference in this run.

Earlier runs invalidated by concurrent source edits were discarded. Other tasks were editing Course and Profile features during verification; their files were preserved. Temporary lint/typecheck failures in those files were followed by successful application checks. The public browser harness waited until the other task released port 3000; it did not stop an unowned server.

The last admin full run had a mobile selector failure when clicking Ant Design's overlaid combobox input. The control check now opens that select with its keyboard interaction; both viewport cases pass. It verifies that a custom section ID of `submit` remains a section destination, and a section ID of `profile` cannot steal focus from the profile outline link.

## Browser coverage and visual review

Admin evidence includes edits to titles, labels, descriptions and information-tab values; stable option values and section IDs; keyboard sorting and move menus; cross-section question moves; duplicate questions and sections; validation bounds/pattern/error controls; broken-route repair; description-only sections; explicit saving; failed-save recovery; Restore/Discard; and changed-server warnings. Preview exercises both branches, skipped required questions, Back, early submission, simulated completion, restart, and desktop/mobile widths. Its profile step describes the configured profile fields; its custom question steps are answerable. Preview does not call profile or registration mutation APIs.

Public evidence includes member and guest activity registration, club registration, current education/profile preservation, actual Back history, route changes clearing skipped answers, refresh recovery, changed-schema recovery, default answers determining the submit action, response editing with a removed question, confirmation, and clearing recovery after successful submission. The tests inspect persisted answers and assert that skipped answers are absent.

The admin builder was inspected alongside `/activity/new` at matching 390px and 1440px widths. The public routed form was inspected alongside the existing club form at those widths. Widths, typography, neutral surfaces, radii, and accent usage follow their respective applications. The builder uses the established guided-workflow container, an outline on desktop, and a drawer on mobile. No horizontal overflow was observed; browser assertions also check document width.

Design read: an administrative form editor for BMKA administrators and a registration wizard for members/guests, using the existing guided-workflow visual language. **ENERGY 1 / RHYTHM 1 / MOTION 1**: calm, predictable, restrained movement. Google Forms informs section branching and inline editing; the surrounding UI retains BMKA's components and styling.

## UI delivery gate

This gate applies to the form feature's introduced controls and styling, not an audit of every pre-existing application page. Evidence is the browser actions above, source review, production builds, and matching-width screenshots.

### Hard gate

- R-02 PASS: introduced UI copy contains no decorative em dashes.
- R-03 PASS: mobile screenshots and document-width assertions show no overflow or escaping text.
- R-17 PASS: no statistics or performance claims were introduced.
- R-18 PASS: no testimonials were introduced.
- R-23 PASS: existing logos/navigation were retained; no new visual assets or invented identities were added.
- R-24 PASS: return links and the response-edit link resolve to existing routes in browser checks.
- R-25 PASS: scoped admin secondary text is #595959 on white (7.0:1), hint text #454545 (9.59:1), focus #075d80 (7.28:1), accent #087da7 (4.66:1), and errors #a8071a (7.75:1). Public form errors use #b42318 on white (6.57:1).
- R-26 PASS: Save, Preview, navigation, menus, duplication, ordering, validation controls, and recovery actions have real handlers; browser checks verify their resulting state or persistence.
- R-27 PASS: existing loading/error wrappers remain; empty sections, validation errors, failed saves, malformed routes, and recovery notices have visible states.
- R-28 PASS: no FAQ was introduced.
- R-32 PASS: keyboard drag handles, move alternatives, visible focus, dialog controls, and section-focus navigation are exercised by browser checks.
- R-33 PASS: UI changes were written directly in source; no runtime patching scripts were introduced.
- R-34 PASS: no theme toggle or additional theme was introduced.
- R-35 PASS: both apps were built and the new editing, routing, preview, recovery, and submission interactions were exercised in recorded browser runs.
- R-36 PASS: no security, customer, or performance claims were introduced.
- R-37 PASS: the approved plan supplies the visual direction; the design read and dials above document it.
- R-38 PASS: synthetic content is confined to owned test fixtures; no fictional product content was added.

### Purpose gate

- R-01 PASS: no decorative gradient or glow was added.
- R-04 PASS: existing Ant Design icons indicate dragging, duplication, deletion, preview, and section navigation.
- R-06 PASS: existing application typography is retained for consistency with adjacent workflows.
- R-07 PASS: no background grid or pattern was added.
- R-08 PASS: arrows represent actual movement or return navigation.
- R-09 PASS: no promotional capsule badges were added.
- R-10 PASS: no glass effects were added.
- R-12 PASS: neutral bordered surfaces organize sections; no new repeated large shadows were added.
- R-13 PASS: no glow treatment was added.
- R-14 PASS: compact and expanded question cards reflect editing state, rather than decorative uniform cards.
- R-19 PASS: movement is limited to direct ordering interactions; keyboard scrolling is immediate and predictable.
- R-22 PASS: no illustrations were added.

### Liveliness

- Dials PASS: ENERGY 1 / RHYTHM 1 / MOTION 1 are explicit and match the approved calm direction.
- Focal point PASS: the selected question is expanded; surrounding questions remain compact.
- Whitespace PASS: separate profile, section, question, and routing groups remain readable at both widths.
- Accent PASS: the existing application accent marks the primary save/continue action and selected state.
- Identity PASS: BMKA's Indonesian action language, mandatory data-diri step, and guided-workflow surfaces remain consistent with adjacent pages.
- Design read PASS: the existing product direction, target users, and dials are recorded above.

### Craftsmanship and quality locks

- C-1 PASS: layout and color decisions serve question editing, route visibility, and application consistency.
- C-2 PASS: every introduced control has an implementation; exercised actions produce visible state or database changes.
- C-3 PASS: sections are administrator-authored content; no template marketing sections were added.
- C-4 PASS: mobile, desktop, keyboard, invalid configuration, recovery, and failed-save states are covered.
- C-5 PASS: verification counts come from actual suite output; historical comparison failures and skips are reported separately.
- R-05 PASS: the layout follows editable form structure, not a generic landing-page template.
- R-11 PASS: existing rectangular controls and restrained card radii are retained.
- R-15 PASS: CTAs describe actions: Simpan Perubahan, Pratinjau, Pulihkan draf, Lanjutkan, and Kirim.
- R-16 PASS: no marketing buzzwords were added.
- R-20 PASS: profile restrictions, Indonesian terminology, and BMKA workflow styling preserve product context.
- R-21 PASS: the existing light appearance is retained; no requested theme was deferred.
- R-29 PASS: neutral surfaces and the established accent remain, with semantic error colors.
- R-30 PASS: Google Forms supplies interaction guidance, while BMKA's visual system remains in use.
- R-31 PASS: the reasons for the outline, card expansion, persistent actions, and grouped settings are recorded in this report and the approved plan.

## Fixture cleanup

Database-backed suites ran sequentially using the fixture lease and recorded owned schemas. Browser runs reported zero pending storage objects; the contract runs removed their three recorded storage objects per backend. Final `make clean-fixtures` succeeded after the concurrent Course run released the lease: [cleanup log](../.artifacts/form-builder-cleanup-2026-09-12.log) and [owned schema manifest](../.artifacts/cleaned-schemas-00179968c48b5841.json). The retained manifest has status `cleaned`, and the fixture lease is absent. Shared application tables were not reset.
