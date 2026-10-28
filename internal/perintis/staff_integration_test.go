//go:build integration && perintis

// Pengelola workflows on a real PostgreSQL in an owned perintis_admin_<run>
// schema (explicit search path, no public fallback). Run with:
//
//	node scripts/run.mjs --environment=test -- go test -tags='integration perintis' -count=1 ./internal/perintis/
//
// or with ../env/test/admin-be-test-env exported.
package perintis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
)

type memoryStore map[string][]byte

func (m memoryStore) Put(_ context.Context, key string, data []byte, _ string) error {
	m[key] = data
	return nil
}
func (m memoryStore) Copy(_ context.Context, from, to string) error { m[to] = m[from]; return nil }
func (m memoryStore) Delete(_ context.Context, key string) error    { delete(m, key); return nil }
func (m memoryStore) Get(_ context.Context, key string) ([]byte, error) {
	if data, ok := m[key]; ok {
		return data, nil
	}
	return nil, errors.New("missing")
}

const fixtureTables = `
CREATE TABLE public_users (id serial PRIMARY KEY, email varchar(255), password varchar(255), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz, member_id varchar(255), account_status varchar(20) NOT NULL DEFAULT 'no_account');
CREATE TABLE admin_users (id serial PRIMARY KEY, email varchar(255) NOT NULL, display_name varchar(255));
CREATE TABLE provinces (id integer PRIMARY KEY, name varchar(100));
CREATE TABLE cities (id integer PRIMARY KEY, province_id integer, name varchar(255));
CREATE TABLE universities (id integer PRIMARY KEY, name varchar(100) NOT NULL);
CREATE TABLE profiles (id serial PRIMARY KEY, user_id integer, name varchar(255) NOT NULL, gender varchar(1), whatsapp varchar(35), instagram varchar(50), linkedin varchar(255), province_id integer, city_id integer, university_id integer, major varchar(50), intake_year integer, level integer DEFAULT 0, badges jsonb DEFAULT '[]', created_at timestamptz, updated_at timestamptz);
CREATE TABLE activities (id serial PRIMARY KEY, name varchar(255) NOT NULL, activity_start date, activity_type integer, activity_category integer, certificate_template_id integer);
CREATE TABLE activity_registrations (id serial PRIMARY KEY, user_id integer, activity_id integer, status varchar(50));
CREATE TABLE clubs (id serial PRIMARY KEY, name varchar(255) NOT NULL);
CREATE TABLE club_registrations (id serial PRIMARY KEY, club_id integer, member_id integer, status varchar(50));
CREATE TABLE club_member_roles (id serial PRIMARY KEY, club_registration_id integer NOT NULL, role_name varchar(150) NOT NULL, is_primary boolean NOT NULL DEFAULT false, sort_order integer NOT NULL DEFAULT 0);
CREATE TABLE achievements (id serial PRIMARY KEY, user_id integer, name varchar(255), achievement_date date, type integer, status integer DEFAULT 0);
`

func wantStatus(t *testing.T, err error, status int) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Status != status {
		t.Fatalf("want %d, got %v", status, err)
	}
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func TestStaffWorkflows(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("needs the test database environment (../env/test/admin-be-test-env)")
	}
	ctx := context.Background()
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	schema := "perintis_admin_" + hex.EncodeToString(b)
	c.DBSchema = "public"
	bootstrap, err := database.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		bootstrap.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := bootstrap.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("cleanup of %s failed: %v", schema, err)
		} else {
			t.Logf("cleaned owned schema %s", schema)
		}
		bootstrap.Close()
	})
	c.DBSchema = schema
	pool, err := database.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	raw, err := os.ReadFile("../../database/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.ReplaceAll(string(raw)[strings.Index(string(raw), "CREATE TABLE public.perintis_participants"):], "public.", "")
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(fixtureTables + ddl)
	store := memoryStore{}
	now := time.Date(2027, 1, 8, 10, 0, 0, 0, WIB)
	svc := &StaffService{Pool: pool, Storage: store, Now: func() time.Time { return now }}

	exec(`INSERT INTO universities VALUES (1, 'Institut Teknologi Bandung')`)
	exec(`INSERT INTO admin_users (id, email, display_name) VALUES (9, 'pengelola@example.test', 'Pengelola BMKA')`)
	exec(`INSERT INTO public_users (id, email, account_status) VALUES (1, 'aisyah@example.test', 'active'), (2, 'rizky@example.test', 'active'),
		(3, 'hana@example.test', 'active'), (4, 'bima@example.test', 'active'), (5, 'salsa@example.test', 'active'), (6, 'citra@example.test', 'active')`)
	exec(`SELECT setval('public_users_id_seq', 6)`)
	exec(`INSERT INTO profiles (user_id, name, university_id, major, intake_year) VALUES (1, 'Aisyah', 1, 'Kimia', 2025), (2, 'Rizky', 1, 'IF', 2023),
		(3, 'Hana', 1, 'Bio', 2022), (4, '=Bima', 1, 'Mat', 2025), (5, 'Salsa', 1, 'Fis', 2023), (6, 'Citra', 1, 'Arsi', 2025)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'Salman Media')`)
	exec(`INSERT INTO club_registrations (club_id, member_id, status) VALUES (1, 1, 'APPROVED')`)
	exec(`INSERT INTO perintis_participants (user_id, is_coordinator) VALUES (3, true)`)
	exec(`INSERT INTO perintis_participants (user_id, is_mentor, coordinator_user_id) VALUES (2, true, 3)`)
	exec(`INSERT INTO perintis_participants (user_id, is_mentor, coordinator_user_id, status) VALUES (5, true, 3, 'reserve')`)
	exec(`INSERT INTO perintis_participants (user_id, is_mentee, nim, university_id, kip_k, category, mentor_user_id) VALUES
		(1, true, '10525107', 1, true, 'biaya_hidup', 2), (4, true, '10525114', 1, false, 'dasar', 2)`)
	exec(`INSERT INTO perintis_participants (user_id, is_mentee, nim, university_id, kip_k, category, status) VALUES (6, true, '10525121', 1, false, 'dasar', 'pending')`)
	// Aisyah's December lapbul was flagged to the pengelola; Bima's wellbeing flag escalated.
	exec(`INSERT INTO perintis_journals (id, mentee_user_id, month, mentor_user_id, category, form_version, status, submitted_at, honesty_confirmed_at,
		files, response_appreciation, response_text, flag_reason, responded_by, responded_at, pulse_energy, pulse_academic, pulse_finance, pulse_relation)
		VALUES (1, 1, '2026-12-01', 2, 'biaya_hidup', 1, 'flagged', '2027-01-03T10:00:00+07', now(), '[{"field":"s7khs","key":"perintis/journals/1/a-khs.pdf","name":"khs.pdf","mime":"application/pdf","size":4}]',
		'Terima kasih', 'Saya teruskan', 'Butuh bantuan biaya kos', 2, '2027-01-04T10:00:00+07', 3, 3, 1, 3)`)
	exec(`INSERT INTO perintis_journals (id, mentee_user_id, month, mentor_user_id, category, form_version, status, submitted_at, honesty_confirmed_at,
		is_late, flag_trigger, flag_status, flag_due_at, pulse_energy, pulse_academic, pulse_finance, pulse_relation, wants_contact)
		VALUES (2, 4, '2026-12-01', 2, 'dasar', 1, 'submitted', '2027-01-04T09:00:00+07', now(), false, 'contact_request', 'open', '2027-01-07T09:00:00+07', 4, 4, 4, 4, true)`)
	exec(`SELECT setval('perintis_journals_id_seq', 2)`)
	exec(`INSERT INTO perintis_session_reports (id, mentor_user_id, month, status, submitted_at, session_date, photos, case_note, case_status)
		VALUES (1, 2, '2026-12-01', 'submitted', '2027-01-06T10:00:00+07', '2026-12-20', '[{"key":"perintis/reports/2/p.jpg","name":"p.jpg","mime":"image/jpeg","size":3}]', 'Kasus kos', 'open')`)
	store["perintis/journals/1/a-khs.pdf"] = []byte("%PDF")
	store["perintis/reports/2/p.jpg"] = []byte{0xff, 0xd8, 0xff}

	// dashboard (loading it also escalates Bima's overdue flag)
	dash := must[StaffDashboard](t)(svc.Dashboard(ctx, ""))
	if dash.Month != "2026-12-01" || dash.Indicators[0].Numerator != 2 || dash.Indicators[0].Denominator != 2 || dash.FollowupsOpen != 3 || dash.WithoutMentor != 1 ||
		len(dash.ByCoordinator) != 1 || dash.ByCoordinator[0].Mentors != 2 || dash.ByCoordinator[0].ActiveFlags != 1 {
		t.Fatalf("dashboard: %+v", dash)
	}

	// follow-ups: one per source, closed with a note
	items := must[[]FollowupItem](t)(svc.Followups(ctx, "", "open"))
	sources := map[string]bool{}
	for _, f := range items {
		sources[f.Source] = true
	}
	if len(items) != 3 || !sources["journal"] || !sources["flag"] || !sources["report"] {
		t.Fatalf("followups: %+v", items)
	}
	wantStatus(t, svc.ResolveFollowup(ctx, 9, "journal", 1, " "), 422)
	for _, f := range items {
		if err := svc.ResolveFollowup(ctx, 9, f.Source, f.ID, "Sudah dihubungi"); err != nil {
			t.Fatalf("%s: %v", f.Source, err)
		}
	}
	wantStatus(t, svc.ResolveFollowup(ctx, 9, "journal", 1, "lagi"), 409)
	handled := must[[]FollowupItem](t)(svc.Followups(ctx, "", "handled"))
	if len(handled) != 3 || handled[0].Handled == nil || handled[0].Handled.By != "Pengelola BMKA" {
		t.Fatalf("handled: %+v", handled)
	}
	if j := must[JournalForMentor](t)(svc.Journal(ctx, 1)); j.Journal.Status != "accepted" || j.CanRespond || j.Journal.Followup == nil {
		t.Fatalf("journal after follow-up: %+v", j.Journal)
	}

	// files: only keys recorded on a row are served
	if data, mime, err := svc.File(ctx, "perintis/journals/1/a-khs.pdf"); err != nil || mime != "application/pdf" || string(data) != "%PDF" {
		t.Fatalf("file: %v %s", err, mime)
	}
	_, _, err = svc.File(ctx, "perintis/journals/1/../2/x")
	wantStatus(t, err, 404)
	_, _, err = svc.File(ctx, "perintis/reports/2/other.jpg")
	wantStatus(t, err, 404)

	// participants
	rows := must[[]ParticipantRow](t)(svc.Participants(ctx, "mentee", "", ""))
	if len(rows) != 3 || rows[0].Name != "=Bima" {
		t.Fatalf("mentees: %+v", rows)
	}
	if found := must[[]ParticipantRow](t)(svc.Participants(ctx, "", "", "10525107")); len(found) != 1 || !found[0].ClubOK || found[0].CoordinatorName == nil {
		t.Fatalf("search: %+v", found)
	}
	nim := "10525114"
	cat := "dasar"
	yes := true
	_, err = svc.UpdateParticipant(ctx, 9, 1, UpdateParticipantInput{IsMentee: true, Nim: &nim, Category: &cat, KipK: &yes})
	wantStatus(t, err, 409)
	_, err = svc.UpdateParticipant(ctx, 9, 2, UpdateParticipantInput{IsCoordinator: true})
	wantStatus(t, err, 409)
	updated := must[ParticipantDetail](t)(svc.UpdateParticipant(ctx, 9, 3, UpdateParticipantInput{IsCoordinator: true, IsMentee: true, Nim: strPtr("13522001"), Category: &cat, KipK: &yes}))
	if !updated.Participant.IsMentee || deref(updated.Participant.Nim) != "13522001" {
		t.Fatalf("update: %+v", updated.Participant)
	}
	moved := must[ParticipantDetail](t)(svc.MoveMentee(ctx, 9, 6, ptr32(5)))
	if moved.Participant.Status != "active" || deref32(moved.Participant.MentorUserID) != 5 {
		t.Fatalf("move: %+v", moved.Participant)
	}
	if salsa := must[ParticipantDetail](t)(svc.Participant(ctx, 5)); salsa.Participant.Status != "active" || len(salsa.Mentees) != 1 {
		t.Fatalf("reserve mentor becomes active: %+v", salsa.Participant)
	}
	_, err = svc.MoveMentee(ctx, 9, 6, ptr32(6))
	wantStatus(t, err, 422)
	_, err = svc.SetStatus(ctx, 9, 2, "inactive", "Lulus")
	wantStatus(t, err, 409)
	_, err = svc.SetStatus(ctx, 9, 6, "inactive", "")
	wantStatus(t, err, 422)
	if off := must[ParticipantDetail](t)(svc.SetStatus(ctx, 9, 6, "inactive", "Mengundurkan diri")); off.Participant.Status != "inactive" || deref(off.Participant.InactiveReason) == "" {
		t.Fatalf("status: %+v", off.Participant)
	}

	// reopen a locked month for a mentee who never submitted
	now = time.Date(2027, 1, 14, 10, 0, 0, 0, WIB)
	if err := svc.Reopen(ctx, 9, 4, "2026-11-01", "Sakit, ada surat dokter"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, svc.Reopen(ctx, 9, 1, "2026-12-01", "x"), 409)
	wantStatus(t, svc.Reopen(ctx, 9, 4, "2027-05-01", "x"), 404)
	if list := must[ParticipantDetail](t)(svc.Participant(ctx, 4)).Journals; list[1].Window != "open" {
		t.Fatalf("reopened month: %+v", list)
	}

	// import: new mentee account, new mentor, matching
	preview := must[ImportPreview](t)(svc.ImportPreview(ctx, "mentee", "nim,nama,email,kategori,kip_k\n13525999,Dimas Baru,dimas@example.test,dasar,ya\n10525107,Aisyah,aisyah@example.test,biaya_hidup,tidak\n"))
	if !preview.CanApply || preview.Rows[0].Result != "new" || preview.Rows[1].Result != "warning" {
		t.Fatalf("preview: %+v", preview)
	}
	if bad := must[ImportPreview](t)(svc.ImportPreview(ctx, "mentee", "nim,nama,email,kategori,kip_k\n10525114,X,x@example.test,dasar,ya\n1,Y,y@example.test,lain,ya\n")); bad.CanApply || bad.Rows[0].Result != "error" || bad.Rows[1].Result != "error" {
		t.Fatalf("bad preview: %+v", bad)
	}
	_, err = svc.ImportPreview(ctx, "mentee", "a,b\n")
	wantStatus(t, err, 422)
	if n := must[int](t)(svc.ImportApply(ctx, 9, "mentee", "nim,nama,email,kategori,kip_k\n13525999,Dimas Baru,dimas@example.test,dasar,ya\n")); n != 1 {
		t.Fatalf("applied %d", n)
	}
	must[int](t)(svc.ImportApply(ctx, 9, "mentor", "email,email_koordinator\nmentor.baru@example.test,hana@example.test\n"))
	must[int](t)(svc.ImportApply(ctx, 9, "matching", "nim,email_mentor\n13525999,mentor.baru@example.test\n"))
	dimas := must[[]ParticipantRow](t)(svc.Participants(ctx, "", "", "13525999"))
	if len(dimas) != 1 || dimas[0].HasAccount || dimas[0].Status != "active" || deref(dimas[0].MentorName) != "mentor.baru" {
		t.Fatalf("imported mentee: %+v", dimas)
	}

	// Super Admin: search kaderisasi members, then add them with roles
	exec(`INSERT INTO public_users (id, email, account_status) VALUES (20, 'nadia@example.test', 'active'), (21, 'umar@example.test', 'no_account'), (22, 'nadia.k@example.test', 'active')`)
	exec(`INSERT INTO profiles (user_id, name, university_id) VALUES (20, 'Nadia Kusuma', 1), (21, 'Umar Faruq', 1)`)
	_, err = svc.SearchMembers(ctx, "ab")
	wantStatus(t, err, 422)
	if none := must[[]MemberCandidate](t)(svc.SearchMembers(ctx, "%%%")); len(none) != 0 {
		t.Fatalf("LIKE wildcards must be literal: %+v", none)
	}
	nadia := must[[]MemberCandidate](t)(svc.SearchMembers(ctx, "nadia"))
	if len(nadia) != 2 || nadia[0].UserID != 22 && nadia[1].UserID != 22 {
		t.Fatalf("search by name and email: %+v", nadia)
	}
	if exact := must[[]MemberCandidate](t)(svc.SearchMembers(ctx, "NADIA.K@example.test")); len(exact) == 0 || exact[0].UserID != 22 || exact[0].Name != "nadia.k" {
		t.Fatalf("exact email first: %+v", exact)
	}
	if known := must[[]MemberCandidate](t)(svc.SearchMembers(ctx, "aisyah@")); len(known) != 1 || !known[0].Participant || len(known[0].Roles) != 1 || known[0].Roles[0] != "mentee" {
		t.Fatalf("existing participant: %+v", known)
	}
	if umar := must[[]MemberCandidate](t)(svc.SearchMembers(ctx, "Faruq")); len(umar) != 1 || umar[0].HasAccount || umar[0].Participant {
		t.Fatalf("new member: %+v", umar)
	}
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 1, UpdateParticipantInput: UpdateParticipantInput{IsMentor: true}})
	wantStatus(t, err, 409)
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 999, UpdateParticipantInput: UpdateParticipantInput{IsCoordinator: true}})
	wantStatus(t, err, 404)
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 21})
	wantStatus(t, err, 422)
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 22, UpdateParticipantInput: UpdateParticipantInput{IsMentee: true, Category: &cat, KipK: &yes}})
	wantStatus(t, err, 422)
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 22, UpdateParticipantInput: UpdateParticipantInput{IsMentee: true, Nim: strPtr("10525107"), Category: &cat, KipK: &yes}})
	wantStatus(t, err, 409)
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 20, UpdateParticipantInput: UpdateParticipantInput{IsMentor: true, CoordinatorUserID: ptr32(2)}})
	wantStatus(t, err, 422)
	umar := must[ParticipantDetail](t)(svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 21, UpdateParticipantInput: UpdateParticipantInput{IsCoordinator: true}}))
	if umar.Participant.Status != "active" || !umar.Participant.IsCoordinator || umar.Row.HasAccount {
		t.Fatalf("added coordinator: %+v", umar.Participant)
	}
	added := must[ParticipantDetail](t)(svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 20, UpdateParticipantInput: UpdateParticipantInput{IsMentor: true, CoordinatorUserID: ptr32(21)}}))
	if added.Participant.Status != "reserve" || deref32(added.Participant.CoordinatorUserID) != 21 {
		t.Fatalf("added mentor: %+v", added.Participant)
	}
	_, err = svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 22, MentorUserID: ptr32(21), UpdateParticipantInput: UpdateParticipantInput{IsMentee: true, Nim: strPtr("13525777"), Category: &cat, KipK: &yes}})
	wantStatus(t, err, 422)
	mentee := must[ParticipantDetail](t)(svc.AddParticipant(ctx, 9, AddParticipantInput{UserID: 22, MentorUserID: ptr32(20), UpdateParticipantInput: UpdateParticipantInput{IsMentee: true, Nim: strPtr(" 13525777 "), Category: &cat, KipK: &yes}}))
	if mentee.Participant.Status != "active" || deref(mentee.Participant.Nim) != "13525777" || deref32(mentee.Participant.MentorUserID) != 20 || deref(mentee.Row.CoordinatorName) != "Umar Faruq" {
		t.Fatalf("added mentee: %+v %+v", mentee.Participant, mentee.Row)
	}
	if nadiaNow := must[ParticipantDetail](t)(svc.Participant(ctx, 20)); nadiaNow.Participant.Status != "active" || len(nadiaNow.Mentees) != 1 {
		t.Fatalf("reserve mentor becomes active when a mentee is added: %+v", nadiaNow.Participant)
	}

	// exports
	for _, kind := range []string{"rekap-mentee", "tugas-mentor", "status-lapbul", "laporan-mentoring", "data-peserta"} {
		name, csv, err := svc.Export(ctx, kind, "2026-12-01")
		if err != nil || name != "perintis-"+kind+"-2026-12.csv" || !strings.Contains(csv, "\n") {
			t.Fatalf("%s: %v %s", kind, err, name)
		}
		if strings.Contains(csv, "\n=Bima") || strings.Contains(csv, ",=Bima") {
			t.Fatalf("%s: spreadsheet formula not neutralised", kind)
		}
	}
	_, _, err = svc.Export(ctx, "unknown", "")
	wantStatus(t, err, 404)
}

func ptr32(v int32) *int32 { return &v }
