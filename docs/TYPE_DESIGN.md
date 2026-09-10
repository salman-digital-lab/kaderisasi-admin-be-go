# Typed boundary and query review

This is an implementation ledger, not a completed architecture review.

| Module | Current typed implementation | Remaining work |
|---|---|---|
| Authentication | Explicit login/Google DTOs, claims, sessions, identities, refresh rotation and database queries | Final aggregate verification |
| Reference data | Explicit request DTOs, generated row/response types, university relation/page DTO, sqlc CRUD | Wider path identifier diagnostics |
| Dashboard | sqlc counts and explicit generated response rows | Final aggregate verification |
| Administrators | Create/update/password DTOs, response/identity/page DTOs, sqlc reads/writes | Final aggregate verification |
| Access grants | Typed optional changes and ticket request/response types; separate ticket workflow service; generated locking/count/update queries | Final aggregate verification |
| Members/profiles | Explicit member/profile request and response types; sqlc creation, account, filtering, relations and mutation queries; dedicated profile/credential services | Final aggregate verification and wider path identifier diagnostics |
| Activities/registrations | Typed activity and registration CRUD/list/detail/status requests, response projections, sorting/filters, transaction services, template readiness, image services and Excel records; generated queries | 92 export/workflow comparisons and affected race tests pass; malformed stored JSON, identifier diagnostics and final verification remain |
| Clubs/forms/roles | Typed club and form requests/responses, reads, mutations, attachment transactions, image services and generated queries | Typed registration/role reads, writes, bulk review and Excel exports now use generated queries; 430 affected comparisons pass; final aggregate verification pending |
| Counseling/achievements/leaderboards | Business workflows and real database comparisons | Generic request, response and JSON query adapters remain |
| Certificates | Typed issuance/snapshot responses, generated locking/issuance queries | Template/list/preparation JSON adapters and request DTOs |
| Jobs | Separate typed entrypoint, generated statements and result types | Final aggregate verification |

`domain.Optional[T]` retains omitted, null and concrete PATCH values. Required
reference fields and nullable administrator fields use explicit DTO members.
The existing Vine compatibility validator performs coercion before decoding a
DTO, preserving validation order and the controller's error envelope.

`database.PageNumber` preserves the observed Lucid pagination contract, including
fractional values and non-finite values serialized as JSON null. SQL bounds are
converted separately using Knex's integer conversion, avoiding Go integer overflow.
Certificate endpoints retain their separately validated and bounded pagination.

Adonis sometimes exposes SQL and stack traces in error fields. The production
multipart/query comparisons use production error envelopes. Export diagnostics
retain the error identity and real Go call sites; only validated stack frames are
normalized. The legacy pagination diagnostic adapter still needs profile relation
and expression filters reviewed; it does not claim those cases are complete.

The activity-registration Excel query in the adopted source has no ORDER BY.
Its contract comparison validates contiguous visible numbering, then compares
complete row multisets. This is an identified source nondeterminism, not an
application sorting change. All columns and explicitly ordered exports remain
order-sensitive; normalization integrity is checked by `make check`.
