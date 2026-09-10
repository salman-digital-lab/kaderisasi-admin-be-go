# Certificate approval UI review

Scope: the new approval request form, inbox and review dialog, plus the approval
block in existing certificate artwork. Existing unrelated dashboard screens are
outside this review. Synthetic names are confined to explicit test fixtures.

Direction: certificate administration for BMKA staff, using the existing Ant
Design dashboard, responsive table/cards and full-screen mobile dialog. Dials:
ENERGY 1 / RHYTHM 1 / MOTION 1. The workflow prioritizes accurate review and consent.

## Decisions and evidence

- Color: retain the dashboard's blue family, darkening actionable text/surfaces
  within approval components for contrast; neutral status labels carry explicit
  words. Scoped CSS overrides the inherited light button gradient.
- Layout: the existing library/activity containers host the inbox. Desktop rows
  compare recipients; mobile cards and the existing dialog preserve touch access.
- Typography: retain the dashboard typography and the certificate's existing
  serif treatment. The approved block uses ordinary readable text.
- Spacing: existing card padding and form spacing separate identity, preview,
  decision note and consent. No new page shell or navigation is introduced.
- Cards: the request form and inbox represent distinct tasks; recipient cards
  are the existing responsive table presentation.
- Assets: reuse certificate artwork and QR renderer. No signature image or new
  decorative asset is generated.

Evidence: `scripts/test-certificate-approval-ui.mjs`, its
`.artifacts/approval-ui/results.json`, desktop/mobile requests and review
screenshots, and matching desktop/mobile library screenshots. The browser script
checks real components using an isolated adapter, including contrast through
axe-core. Backend authorization and persistence have separate PostgreSQL tests.

## Hard gate

- R-02 PASS: new interface copy contains no em dashes.
- R-03 PASS: 1440px and 390px browser checks and screenshots show contained layouts; overflow assertion passes.
- R-17 PASS: no promotional statistics are introduced.
- R-18 PASS: no testimonials are introduced.
- R-23 PASS: existing artwork/QR components are reused; fixture content is explicitly labeled as test content.
- R-24 PASS: no navigation is added; issued-certificate links use the existing preview route.
- R-25 PASS: axe-core contrast checks cover the request form, review dialog, empty state and error state; primary buttons use a solid accessible blue.
- R-26 PASS: request, refresh, filters, review selection, consent, rejection, cancellation and close controls have working handlers.
- R-27 PASS: loading copy/spinners exist; empty and error states are exercised in both viewport runs.
- R-28 PASS: no FAQ is introduced.
- R-32 PASS: labeled form controls and keyboard select/Tab/Enter/Escape interactions are exercised.
- R-33 PASS: feature changes live in React, TypeScript and CSS source; browser scripts only test them.
- R-34 PASS: no theme toggle is introduced; existing dashboard surfaces are preserved.
- R-35 PASS: production builds pass and the browser script records the new workflow interactions.
- R-36 PASS: the product describes internal approval; documentation explicitly excludes cryptographic PDF signing.
- R-37 PASS: workspace visual-consistency rules and existing certificate pages supply the direction.
- R-38 PASS: product data comes from API records; synthetic examples remain in opt-in browser fixtures.

## Purpose gate

- R-01 PASS: no new gradient or glow; approval buttons override the inherited gradient for contrast.
- R-04 PASS: existing certificate controls and Ant icons retain their document/action meaning.
- R-06 PASS: dashboard and certificate typography are retained for continuity and readability.
- R-07 PASS: no decorative grid, dots or blueprint background is added.
- R-08 PASS: no decorative button arrows are added.
- R-09 PASS: status labels show persisted workflow states without marketing badges.
- R-10 PASS: no glass effect is added.
- R-12 PASS: existing modal elevation separates review from the underlying page.
- R-13 PASS: no glow is added.
- R-14 PASS: responsive recipient cards repeat comparable records using the existing table component.
- R-19 PASS: existing control transitions match MOTION 1; no entrance choreography is added.
- R-22 PASS: no decorative illustration is added.

## Liveliness

- Dials PASS: ENERGY 1 / RHYTHM 1 / MOTION 1 are explicit and suit staff review.
- Consistency PASS: matching desktop/mobile screenshots retain the dashboard visual language.
- Focal point PASS: request submission and review/consent are the primary actions for their respective states.
- Whitespace PASS: spacing separates signer identity, certificate content and decision controls.
- Accent PASS: blue identifies actionable controls, while status text remains neutral.
- Identity PASS: existing BMKA dashboard controls and certificate typography are retained.
- Design read PASS: the workspace's existing-dashboard direction governs the feature, as requested for implementation.

## Craftsmanship and quality locks

- C-1 PASS: the major visual decisions and purposes are recorded above.
- C-2 PASS: the browser workflow exercises the new interactive controls.
- C-3 PASS: each section serves signer choice, request review, consent or decision history.
- C-4 PASS: desktop/mobile, keyboard, empty and error checks pass without page errors.
- C-5 PASS: approved names and dates come from persisted evidence; pending previews identify themselves as previews.
- R-05 PASS: the feature extends an operational inbox and existing certificate renderer.
- R-11 PASS: existing square controls and surfaces are retained.
- R-15 PASS: actions name their outcome, including “Kirim”, “Tinjau”, “Tolak” and “Setujui & terbitkan”.
- R-16 PASS: no marketing buzzwords are added.
- R-20 PASS: the design preserves the product's existing staff workflow and certificate presentation.
- R-21 PASS: no new theme default or theme switch is introduced.
- R-29 PASS: existing blue and neutral surfaces are retained; error styling communicates actionable failures.
- R-30 PASS: no external product is imitated.
- R-31 PASS: color, layout, typography, spacing, cards and assets have written purposes above.
