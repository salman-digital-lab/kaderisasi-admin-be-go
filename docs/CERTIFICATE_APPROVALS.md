# Certificate signer approvals

E-sign templates use a `variable-text` element with `variable: "{{approval}}"`
and a visible QR element at least 80 × 80 pixels. The approval block renders the
signer's name, title and approval date from the issued record. New nonblank
starter designs include both elements. Existing image signatures and previously
issued certificates remain supported.

## Using the workflow

1. In the certificate designer, add **Persetujuan elektronik** and **QR verifikasi**.
   For a published design, duplicate it first, make the changes and publish the copy.
   Remove the old signature image if it is no longer needed.
2. Select that design in **Sertifikat kegiatan**, select recipients and review.
3. Choose an active admin signer, enter their title and send approval requests.
4. The signer opens **Persetujuan sertifikat** on the certificate library or activity
   page. They can select up to 20 requests per page, inspect each saved certificate,
   explicitly consent, then approve and issue the batch.
5. A signer can reject requests with a reason. The requester can cancel a pending
   request and submit a corrected one. Approved requests remain in the history;
   certificate revocation uses the existing certificate revocation workflow.

The `certificate.approve` permission is included in the existing roles that can
issue certificates. A permission alone does not authorize signing: the current
admin must also be the request's designated signer. Selecting a signer does not
count as that person's consent. Signers require an active account and display name.

## Data and API behavior

`certificate_approvals` stores a frozen render payload, signer identity/title,
requester, SHA-256 content fingerprint, status, decision actor/time, rejection or
cancellation note, and issued certificate reference. The fingerprint includes
the full payload and designated signer; canonical JSON handles JSONB key ordering.
There can be only one pending request per registration. Identical requests and
repeated decisions are idempotent.

Approval compares the current eligible recipient, activity, published template
version and signer name with the reviewed snapshot. Changes require cancellation
and a fresh review. It creates the issued certificate and its `approval_snapshot`
in the same transaction as the approval decision. Locks follow the existing
registration → activity → template order before locking the approval.

Pending requests have no issued certificate or public certificate code. The
legacy direct-issuance API rejects e-sign templates and registrations with pending
requests. Public rendering, verification and download access reject e-sign
certificates without approval evidence. Public responses expose only signer name,
title and time; internal actor IDs and fingerprints stay in the admin API.

New authenticated endpoints under `/v2/certificates`:

| Endpoint | Purpose |
| --- | --- |
| `GET /signers` | Active admins authorized to approve |
| `GET /approvals` | Paginated requests assigned to or submitted by the current admin |
| `GET /approvals/:id` | Authorized access to the saved review payload |
| `POST /approvals` | Request approval for up to 50 registration IDs with expected activity/template/version |
| `POST /approvals/decide` | Approve, reject or cancel up to 50 requests, binding each decision to its reviewed fingerprint |

Batch endpoints return per-item results. A failed item does not erase successful
items. Clients may retry identical requests safely. Approval requires explicit
`consent: true`; rejection requires a reason.

This is internal electronic approval with online record verification. It does
not add a cryptographic signature to PDF bytes. PDF generation remains in the
frontends, and the QR opens the current official record including revocation.

## Rollout and verification

Apply `1789037184043_create_create_certificate_approvals_table.ts` through Ace in
`kaderisasi-admin-be`, explicitly selecting the intended database environment as
described in the workspace `docs/ENVIRONMENTS.md`. Deploy the Go and public APIs
after the additive migration, then the frontends. No seed or data reset is needed.
Configure the existing public certificate URL for QR generation. Existing
certificates do not need migration to the new approval workflow.

From the Go repository, `node scripts/test-certificate-approvals.mjs` creates its
own marked UUID schema, runs Ace migrations and the approval/legacy issuance
integration tests, then drops only that schema. Evidence is recorded under
`.artifacts/certificate-approvals/`. Its Ace invocation disables the database-wide
advisory lock only for this exclusively owned test schema; ordinary deployments
retain migration locking.

The admin UI's synthetic fixture is `tests/browser/approval.html`, served by the
admin Vite development server. It exercises the real approval components through
an adapter that prevents application API requests. Real PostgreSQL and HTTP
behavior is covered separately by the integration suite. Public certificate
serialization tests cover approval availability and private audit-field removal.

With the admin development server on port 3005 and the Go/public frontend test
dependencies installed, run `node scripts/test-certificate-approval-ui.mjs` from
the Go repository. It checks the workflow at desktop and mobile widths, keyboard
selection, consent, rejection, cancellation, history, empty/error states and text
contrast. Screenshots and results are saved in `.artifacts/approval-ui/`.

Implementation verification on 10 September 2026:

- Go `make check` and `make test-unit` passed.
- Real PostgreSQL/HTTP approval and legacy issuance tests passed. The owned
  schema was removed; evidence is in
  `.artifacts/certificate-approvals/f927ef6b77e09eb0/result.json`.
- Migration lint, typecheck, build and both migration maintenance tests passed.
- Admin lint/build (including TypeScript) and 57 certificate unit tests passed.
- Public frontend lint, typecheck, build and 43 certificate unit tests passed.
- Public API lint, typecheck, build and 5 certificate unit tests passed.
- Desktop/mobile approval browser checks passed. The existing full-service
  upload/PDF/revocation browser test was updated but was not run in this change;
  UI and real API/database behavior were verified separately.

No shared application or production database was migrated during implementation.
See [the UI review](CERTIFICATE_APPROVALS_UI_REVIEW.md) for the design checks.
