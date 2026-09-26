# Certificate publishing and correction

The fixed document signer is Oktofa Yudha Sudrajad, S.T., M.S.M., Ph.D.,
Ketua Bidang Mahasiswa, Kaderisasi, dan Alumni. The catalog lives in
`internal/certificate/document_signers.go`; it is independent of administrator
accounts. Publishing currently always resolves the `oktofa-yudha-sudrajad` key.

An active administrator with `certificate.issue` (including Asisten Manager
Program) publishes from the activity certificate workflow after confirming the
recipient count and the authority to act for the signer. There is no approver
selection or separate approval step. Salman documents are preflighted for every
recipient before any batch is sent. Existing eligibility, published-score, and
template-version checks still apply.

`issued_by`, `issued_at`, and approval evidence (`approved_by`, document identity,
name/title, timestamp, content hash) identify the publishing administrator and
signed content. Direct publication does not create an approval request; its
legacy `request_id` is zero. Pending requests for the registration are cancelled
in the same publication transaction. The old approval endpoints remain for
compatibility, but are no longer part of the dashboard publishing flow.

Unpublishing uses the existing revocation endpoint and `certificate.revoke`
permission. It marks that specific certificate invalid, blocks participant
downloads, and preserves its code, snapshots, actor, time, and reason. Corrected
publication creates a new record and a new code. The old code stays invalid.
The partial unique index allows only one non-revoked certificate per registration.
Recipient preparation and group editing consider only active certificates;
registration-based owner reads and admin lookups select the latest version.
Activity-wide certificate listings retain all versions for audit.

## Rollout

Deploy compatible Go and public-backend readers first, then run the Ace
`1790403373034_create_allow_certificate_republications_table` migration, then
deploy the dashboard. Go's explicit partial-index conflict target also works
with the previous full unique index during this transition. Republishing a
withdrawn certificate requires the migration. Do not roll back the unique-index
migration after multiple versions exist: restoring global uniqueness must fail
rather than delete history. Prefer forward fixes or compatible application images.

## Verification

Go tests cover direct publishing as the program admin, stored signer/audit,
superseding pending approvals, withdrawal, correction, distinct replacement code,
old snapshot retention, latest-version lookup, concurrent duplicate prevention,
score gating, ineligible recipients, and legacy approval compatibility. A shared
owned-schema run tests participant downloads and public privacy in the Adonis
backend. Desktop/mobile browser tests cover confirmation, cancellation,
unpublishing, and republishing. Migration tests verify the partial index and
retain cleanup evidence. No issued certificate is backfilled or rewritten.
