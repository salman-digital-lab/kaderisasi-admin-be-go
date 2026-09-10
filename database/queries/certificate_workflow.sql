-- name: CertificateRegistrationByIdentifier :one
SELECT * FROM activity_registrations WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockCertificateRegistrationByIdentifier :one
SELECT * FROM activity_registrations WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: CertificateActivityByIdentifier :one
SELECT * FROM activities WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockCertificateActivityByIdentifier :one
SELECT * FROM activities WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;

-- name: CertificateParticipantMember :one
SELECT COALESCE(u.email,'')::text AS email,COALESCE(p.name,'')::text AS name,COALESCE(p.gender,'')::text AS gender,COALESCE(v.name,'')::text AS university
FROM public_users u LEFT JOIN LATERAL (SELECT * FROM profiles WHERE user_id=u.id ORDER BY id LIMIT 1) p ON true LEFT JOIN universities v ON v.id=p.university_id WHERE u.id=$1;

-- name: CertificateGuestUniversity :one
SELECT name FROM universities WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: CountCertificateRecipients :one
SELECT count(*) FROM activity_registrations r LEFT JOIN issued_certificates c ON c.registration_id=r.id WHERE r.activity_id=CAST(CAST(@activity_id AS text) AS integer) AND (sqlc.narg('search')::text IS NULL OR (COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta')) ILIKE '%'||sqlc.narg('search')::text||'%') AND (sqlc.narg('state')::text IS NULL OR (CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END)=sqlc.narg('state')::text) AND (NOT @filter_selected::boolean OR r.id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[])));

-- name: ListCertificateRecipients :many
SELECT r.id AS registration_id,r.created_at,r.status,c.id AS certificate_id,c.certificate_code,(COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta'))::text AS name,(CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END)::text AS state FROM activity_registrations r LEFT JOIN issued_certificates c ON c.registration_id=r.id WHERE r.activity_id=CAST(CAST(@activity_id AS text) AS integer) AND (sqlc.narg('search')::text IS NULL OR (COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta')) ILIKE '%'||sqlc.narg('search')::text||'%') AND (sqlc.narg('state')::text IS NULL OR (CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END)=sqlc.narg('state')::text) AND (NOT @filter_selected::boolean OR r.id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[])))
ORDER BY CASE WHEN @ascending::boolean THEN r.created_at END ASC NULLS LAST,CASE WHEN NOT @ascending::boolean THEN r.created_at END DESC NULLS LAST,CASE WHEN @ascending::boolean THEN r.id END ASC,CASE WHEN NOT @ascending::boolean THEN r.id END DESC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: CertificateRecipientCounts :many
SELECT (CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END)::text AS state,count(*) AS total FROM activity_registrations r LEFT JOIN issued_certificates c ON c.registration_id=r.id WHERE r.activity_id=CAST(CAST(@activity_id AS text) AS integer) GROUP BY (CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END);

-- name: CertificatePreparationRecipients :many
SELECT r.id AS registration_id,(CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END)::text AS state FROM activity_registrations r LEFT JOIN issued_certificates c ON c.registration_id=r.id WHERE r.activity_id=CAST(CAST(@activity_id AS text) AS integer) AND (NOT @filter_selected::boolean OR r.id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[]))) ORDER BY r.id;

-- name: CertificateRecipientNames :many
SELECT r.id,(COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta'))::text AS name FROM activity_registrations r WHERE r.id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[]));

-- name: CertificateBulkRegistrations :many
SELECT * FROM activity_registrations WHERE activity_id=CAST(CAST(@activity_id AS text) AS integer) AND status= @status::text;

-- name: CountIssuedCertificateList :one
SELECT count(*) FROM issued_certificates c WHERE (sqlc.narg('activity_id')::text IS NULL OR c.activity_id=CAST(CAST(sqlc.narg('activity_id') AS text) AS integer)) AND (NOT @filter_selected::boolean OR c.registration_id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[])));

-- name: ListIssuedCertificates :many
SELECT c.id,c.certificate_code,c.registration_id,c.activity_id,c.participant_snapshot,COALESCE(c.template_snapshot->>'name','')::text AS template_name,c.issued_at,c.issued_by,issuer.display_name AS issued_by_name,c.revoked_at,c.revoked_reason,c.revoked_by,revoker.display_name AS revoked_by_name,CASE WHEN c.revoked_at IS NULL THEN 'issued_active' ELSE 'issued_revoked' END::text AS state
FROM issued_certificates c LEFT JOIN admin_users issuer ON issuer.id=c.issued_by LEFT JOIN admin_users revoker ON revoker.id=c.revoked_by WHERE (sqlc.narg('activity_id')::text IS NULL OR c.activity_id=CAST(CAST(sqlc.narg('activity_id') AS text) AS integer)) AND (NOT @filter_selected::boolean OR c.registration_id=ANY(CAST(CAST(@registration_ids AS text[]) AS integer[]))) ORDER BY c.issued_at DESC LIMIT sqlc.narg('page_size')::bigint OFFSET @page_offset::bigint;

-- name: IssuedCertificateByRegistrationIdentifier :one
SELECT * FROM issued_certificates WHERE registration_id=CAST(CAST(@identifier AS text) AS integer);

-- name: IssuedCertificateByIdentifier :one
SELECT * FROM issued_certificates WHERE id=CAST(CAST(@identifier AS text) AS integer);

-- name: LockIssuedCertificateByIdentifier :one
SELECT * FROM issued_certificates WHERE id=CAST(CAST(@identifier AS text) AS integer) FOR UPDATE;
