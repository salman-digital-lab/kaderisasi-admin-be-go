# Certificate document signers

New approval requests use the fixed catalog in
`internal/certificate/document_signers.go`. The initial profile is Oktofa Yudha
Sudrajad, S.T., M.S.M., Ph.D., Ketua Bidang Mahasiswa, Kaderisasi, dan Alumni.
Add future document identities to that catalog with stable, unique keys.

`GET /v2/certificates/document-signers` exposes the catalog to authorized
certificate readers. `GET /v2/certificates/signers` retains its existing wire
contract and lists eligible administrator approvers. The dashboard labels these
as two separate choices and defaults the document identity to the first profile.

New approval requests require `document_signer_key`. The existing `signer_id`
field identifies the assigned **administrator approver**, retaining its foreign
key and permission checks. Client-provided `signer_title` is ignored: the document
name and title are resolved from the catalog. Missing or unknown profile keys
return `INVALID_DOCUMENT_SIGNER`; older open dashboard tabs must refresh.

The profile is included in the approval snapshot and content hash. Approval
requires the assigned active administrator's permission and consent to act on
behalf of the named document signer. A changed or removed catalog profile blocks
pending approvals until they are cancelled and requested again. The existing
`decided_by` and `decided_at` fields record the administrator's action. Issued
approval evidence also records `approved_by` and the document profile, alongside
the existing fields. Public-backend rendering continues to project only the
snapshot's signer name, title, and approval time.

Snapshots without `document_signer` retain the previous personal-approval checks
and hash representation. Already issued documents continue to render their saved
name/title; neither catalog edits nor admin profile edits rewrite them. No schema
migration or backfill is needed.

Validation: Go unit and certificate workflow tests cover profile hashing,
serialization, unknown/missing profiles, server-owned title, separate approver
identity, unauthorized approval, consent, duplicate prevention, stale content,
legacy pending requests, cancellation, rejection, and revocation. The browser
approval harness covers the new request form and review flow on desktop and mobile.
