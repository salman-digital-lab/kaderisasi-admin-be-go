# Form builder and section routing

The admin builder and public activity/club forms share a forward-only routing contract inside `custom_forms.form_schema`. This change needs no migration or new API endpoint. Existing `/v2` routes, registration restrictions, and response envelopes remain in place.

## Schema

```json
{
  "version": 2,
  "fields": [
    { "id": "profile", "section_name": "profile_data", "fields": [] },
    {
      "id": "choice",
      "section_name": "Pilihan peserta",
      "description": "Pilih jalur pendaftaran Anda.",
      "fields": [{
        "key": "track", "label": "Jalur", "type": "radio", "required": false,
        "options": [{ "label": "Lengkapi detail", "value": "detail" }, { "label": "Selesai", "value": "finish" }]
      }],
      "navigation": {
        "defaultTarget": { "type": "next" },
        "questionKey": "track",
        "routes": [{ "optionValue": "finish", "target": { "type": "submit" } }]
      }
    },
    { "id": "detail", "section_name": "Detail", "description": "Petunjuk tambahan.", "fields": [] }
  ]
}
```

- Version is optional for legacy schemas; the current editor explicitly saves version 2. Version 2 requires unique, nonempty section IDs. Field keys are unique across the form.
- A destination is `{"type":"next"}`, `{"type":"submit"}`, or `{"type":"section","sectionId":"a-later-section-id"}`. No backward jump or cycle is accepted.
- An enabled custom `radio` or `select` field in the same section may determine routing. Profile fields cannot determine routes. Each section has at most one routing source.
- Unmapped answers, including an unanswered optional source, use `defaultTarget`. Omitted navigation is sequential; the final section submits.
- Options match the public controls' existing normalized value: the string conversion of a nonempty stored value, otherwise its label. Editing a label preserves its original normalized value, including legacy options without an explicit value.
- Labels and descriptions can change without changing routes. Moving/deleting a target, source, or mapped option leaves a repairable error in the builder. Saving and preview are blocked until repaired.
- Section duplication creates a fresh section ID and fresh question keys without navigation. Question duplication creates a fresh key and is not a routing source. Description-only sections remain present.
- IDs for legacy sections are assigned only in editor memory, then persisted by **Simpan Perubahan**. Reading a form does not update it.

## Respondents and validation

The profile or guest step stays first. Each section validates before advancing. The action reads **Kirim** when the current answer leads to submission. Back follows the visited path, preserves current edits, and removes skipped answers when advancing along a changed route. Progress shows visited sections and the current section, without an assumed total.

Adonis derives the reachable sections from the stored schema for member activity registration, guest registration, club registration, and activity-response updates. It validates reachable enabled questions, rejects invalid choices/unknown keys, and only persists reachable enabled custom answers. Existing response exports therefore receive the pruned records. Legacy activity-response updates without an active custom form retain their existing behavior.

Go preserves routing metadata through input validation and explicit request types, rejects malformed navigation, and checks routing during form readiness. The existing admin validation error envelope is retained, including its HTTP 500 response for caught input-validation failures.

The registration details page links to the existing response editor using `?edit=form`. Its answerable section flow uses the same routing engine as registration. Preview runs entirely in local component state and simulates completion without calling registration or profile mutation APIs.

## Drafts

- Admin recovery: `form-builder:v1:<administrator-id>:<form-id>`, 500 ms debounce, seven-day expiry. Restore and Discard are explicit. A server `updated_at` mismatch is shown before restoring. Failed saves retain recovery; successful saves remove it.
- Respondent recovery: `customForm:v2:<member-id-or-guest>:<form-id>`, 300 ms debounce, two-hour expiry. Recovery includes the full schema fingerprint, section IDs, answers, and visited history. Restored progress is recalculated and prior custom sections are revalidated. Changed/expired schemas return to the first step with an explanation. Successful submissions remove recovery.
- Storage failure leaves the editor/form usable and displays a local-recovery message. Drafts stay on the current browser/device.

## Rollout

1. Deploy the Go admin API and Adonis public API together with their schema/submission validation changes.
2. Deploy the public frontend with routing-aware registration, recovery, and response editing.
3. Deploy the admin frontend with routing controls enabled.

Do not roll an older renderer or validator back over active routed schemas. Keep the upgraded backends and renderer until administrators have explicitly removed navigation from affected forms and saved sequential schemas. Standalone response persistence, profile-based routing, quizzes, and analytics remain outside this change.

## Shared fixtures and verification

The identical `routing.fixtures.json` vectors are copied into `internal/formschema`, admin frontend `src/utils`, public frontend `src/features/customForm`, and public backend `tests/unit`. Their tests cover sequential defaults, explicit forward jumps, early submit, skipped required fields, invalid answers, and pruning. Changes to the routing contract must update all four copies and pass each suite.

Browser suites: `tests/browser/form-builder.spec.mjs` and `tests/public-browser/form-routing.spec.mjs`. Run them through `scripts/browser.mjs --borrow-workspace` (with `--public` for public forms). Both run at 1440px and 390px using owned database fixtures. `history-custom-form.spec.mjs` and `clubs.spec.mjs` guard existing profile and registration behavior.

Database suites must run sequentially through the fixture lease. `make clean-fixtures` removes only recorded owned schemas and storage objects. Verification results and the UI review are recorded in `FORM_BUILDER_VERIFICATION.md`.
