# Multiple administrator roles

Apply the new Ace migration from `../kaderisasi-admin-be` before starting this
version of the API. Existing assignments are preserved. The original `role_code`
column stores the first role; `additional_role_codes` stores the remaining roles.
Together they form a set with equal authority. Role order does not affect access.

Administrator create and update endpoints accept `role_codes`, for example
`{"role_codes":["activity_manager","konselor"]}`. On update, omission preserves
assignments and `[]` removes all roles. Null, unknown roles, and requests containing
both `role_code` and `role_codes` are rejected. Duplicate role codes are collapsed.
Legacy `role_code` requests still replace the complete assignment with one role
or clear it with null.

Administrator responses expose `role_codes`, `roles`, and deduplicated
`effective_permissions`. Session users expose `roles`; session `permissions`
contains the same union. The singular `role` field remains for older clients.
Inactive accounts have no effective permissions. The existing JWT reads current
database assignments on each request, so removed permissions take effect without
waiting for token expiry.

Approved access requests add a role without removing current assignments.
Super Admin checks and the last-active-Super-Admin lock consider every role.
The `role_code` list filter matches any assignment. Bootstrap seeding preserves
other roles, and preflight validates additional role codes and counts secondary
Super Admin assignments.

The public Adonis backend does not read administrator RBAC assignments; its
member roles and authentication remain unchanged. Older admin API binaries do
not enforce additional roles and must not be used to manage multi-role accounts.
