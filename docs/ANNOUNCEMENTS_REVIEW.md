# Announcement implementation review

Reviewed 2026-09-22 across the Go API, public Adonis API, Ace migration, and both
frontends. The review included code inspection, real PostgreSQL integration,
desktop/mobile browser flows, keyboard interaction, scoped axe checks, and
screenshots compared with existing application pages.

## Issues fixed

- Background inbox refresh replaced the list with skeletons and removed keyboard
  focus. Both inboxes now retain their current rows during refresh; changing a
  filter or page still shows the initial loading state.
- Failed detail requests had no retry action. Both dialogs now support retry.
  After a successful retry, focus moves to the announcement title so Escape and
  subsequent keyboard navigation remain inside the dialog.
- The composer could discard unsaved changes on close. Closing a dirty editor
  now asks whether to discard changes; tab reload/close also warns while dirty.
  Successful saves clear the dirty state.
- Validation errors in the long composer could be offscreen. Submission now
  scrolls to and focuses the first invalid field.
- Long unbroken titles and action labels could escape the admin preview/detail.
  They now wrap within the dialog, tested at the maximum accepted lengths.
- An unsuccessful management-list request left a loading skeleton and old
  pagination state. It now presents an explicit retry state.
- Zero-recipient previews disabled publishing without explaining the next step.
  They now explain that the audience must change. Club status help explicitly
  describes the APPROVED default when the selection is cleared.
- Admin helper text and placeholders failed contrast checks. Scoped theme tokens
  now use the readable palette already used by dashboard approvals/short links.
- Public inbox detail IDs are validated before requesting a notification, matching
  the admin inbox's unavailable-announcement behavior.

No backend behavior changes were needed during this review. Publication remains
transactional, recipient identities remain authentication-derived, and message
bodies remain plain text with validated HTTPS actions.

## Verification

| Check | Result |
| --- | --- |
| Admin lint, TypeScript/Vite build, unit tests | PASS, 169 tests |
| Public frontend lint, typecheck, production build, unit tests | PASS, 143 tests |
| Admin browser cases on desktop and mobile | PASS, six cases |
| Member browser cases on desktop and mobile | PASS, two cases |
| `TestAnnouncementWorkflow` against owned PostgreSQL fixtures | PASS |
| Scoped axe WCAG 2 A/AA and 2.1 AA checks | PASS, admin composer/inbox and member main content |
| Keyboard refresh retention, Enter, retry focus, Escape | PASS in both inboxes |
| Maximum-length preview and mobile overflow checks | PASS |

The browser suites also exercise compose/save/reopen/preview/publish/withdraw,
targeted member/role selection, deletion, unavailable links, read state, polling
pause/resume, request failures, public session expiry, and private proxy behavior.
The integration suite covers audience overlap, custom statuses, additional roles,
inactive accounts, authorization, concurrent publication, rollback, retry
deduplication, stale edits, pagination, read-all cutoffs, and withdrawal.

## UI and antislop review

- Visual consistency PASS: existing guided admin layout and public PageContainer/
  PageHeader remain in use; screenshots match surrounding desktop/mobile pages.
- Purpose PASS: repeated rows support scanning announcements; badges communicate
  unread counts or lifecycle state; bell icons identify the inbox. No decorative
  gradients, illustrations, marketing sections, or new fonts were introduced.
- Hierarchy PASS: page title and primary task remain the focal points, metadata
  is secondary, and the existing blue accent identifies actions. Restrained
  utility-page direction: ENERGY 1 / RHYTHM 1 / MOTION 1.
- Content PASS: Indonesian task-specific labels, real recipient counts, explicit
  loading/empty/error states, and no invented product claims or social proof.
- Interaction PASS: browser-tested controls, visible keyboard focus, retry paths,
  unsaved-change protection, and responsive maximum-length content.
- Accessibility PASS within the tested scope: automated checks found no remaining
  violations in the audited regions after contrast fixes; keyboard regressions
  are covered. This is not a full-app or manual screen-reader certification.

The public build succeeds with a sitemap fetch warning when the activity API is
stopped. The existing admin bundle-size warning remains unrelated to this change.
No production deployment or shared-database migration was performed. Apply the
additive migration before deploying the APIs and frontends, following
[the rollout notes](ANNOUNCEMENTS.md).

## Composer simplification — September 22, 2026

The admin composer now separates message, recipients, and review within one
bounded dialog. Optional action-link fields and audience-specific controls are
revealed on demand. Footer actions stay visible while long content scrolls.
Drafts preserve their fields across steps and reopen with the saved audience.
Removing a recipient group clears its account/group selections; removing a link
clears both link fields. Failed recipient previews can retry against the saved
draft version without creating another draft.

Validated the refined composer at 1440×1000 and 390×844 alongside the existing
Akses Saya page. Admin lint, TypeScript/Vite build, and 169 unit tests passed.
The updated announcement browser suite passed all eight cases, including
message/link validation, empty recipients, long content, save/reopen, targeted
member/role labels, preview retry, group removal, keyboard navigation, publish,
read state, withdrawal, and permission isolation. Scoped axe checks found no
violations in the exercised composer/inbox regions. All expanded recipient
controls retained visible footer actions on mobile. Owned fixture schemas and
recorded storage cleanup completed.

Browser evidence: `.artifacts/browser/2026-09-22T08-14-16-359Z/`.
Frontend revision: `73188ea4e73ab82b71eb5df3fe28bba653cc20cd`.
