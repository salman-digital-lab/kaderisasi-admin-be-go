package perintis

// Pengelola workflows served by kaderisasi-admin-be-go under /v2/perintis.
// Only this repository has this file; permissions are checked by the route
// table (routes.json) before these run.

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/storage"
)

type StaffService struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
	Now     func() time.Time
}

func (s *StaffService) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *StaffService) read(ctx context.Context) (*World, error) {
	return Load(ctx, dbgen.New(s.Pool), s.now())
}

func (s *StaffService) write(ctx context.Context, fn func(*World) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	w, err := Load(ctx, dbgen.New(s.Pool).WithTx(tx), s.now())
	if err != nil {
		return err
	}
	if err := fn(w); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (w *World) staffMonth(month string) (string, error) {
	if month == "" {
		month = CurrentReportMonth(w.Now)
	}
	if month == "" {
		return "", fail(404, "Belum ada bulan laporan.")
	}
	if _, ok := MonthStart(month); !ok || month < FirstMonth {
		return "", fail(404, "Bulan laporan tidak tersedia.")
	}
	return month, nil
}

// ---- dashboard -------------------------------------------------------------------

func (s *StaffService) Dashboard(ctx context.Context, month string) (StaffDashboard, error) {
	w, err := s.read(ctx)
	if err != nil {
		return StaffDashboard{}, err
	}
	if month, err = w.staffMonth(month); err != nil {
		return StaffDashboard{}, err
	}
	due := DeadlinesFor(month)
	journals, err := w.Q.PerintisJournalsInMonth(ctx, DateOf(month))
	if err != nil {
		return StaffDashboard{}, err
	}
	reports, err := w.Q.PerintisReportsInMonth(ctx, DateOf(month))
	if err != nil {
		return StaffDashboard{}, err
	}
	reportOf := map[int32]dbgen.PerintisSessionReport{}
	for _, r := range reports {
		reportOf[r.MentorUserID] = r
	}
	started := map[int32]bool{}
	for _, j := range journals {
		started[j.MenteeUserID] = true
	}
	mentees, withoutMentor := 0, 0
	for _, p := range w.Participants {
		if !p.IsMentee {
			continue
		}
		// A mentee without kategori cannot start a lapbul yet, so they do not
		// count against the on-time rate until the pengelola sets it.
		if p.Status == "active" && p.MentorUserID != nil && (p.Category != nil || started[p.UserID]) {
			mentees++
		}
		if p.Status == "active" && p.MentorUserID == nil {
			withoutMentor++
		}
	}
	mentors := []int32{}
	for _, p := range w.Participants {
		if p.IsMentor && p.Status == "active" && len(w.MenteesOf(p.UserID)) > 0 {
			mentors = append(mentors, p.UserID)
		}
	}
	ontime, sent, respondedOnTime, flags, flagsOnTime := 0, 0, 0, 0, 0
	for _, j := range journals {
		if j.Status != "draft" {
			sent++
			if !j.IsLate {
				ontime++
			}
			if Responded(j.Status) && j.RespondedAt.Valid && !j.RespondedAt.Time.After(due.MentorDue) {
				respondedOnTime++
			}
		}
		if j.FlagTrigger != nil {
			flags++
			if j.FlagHandledAt.Valid && !j.FlagHandledAt.Time.After(j.FlagDueAt.Time) {
				flagsOnTime++
			}
		}
	}
	submittedReports, verifiedReports := 0, 0
	for _, id := range mentors {
		if r, ok := reportOf[id]; ok {
			if r.Status != "draft" {
				submittedReports++
			}
			if r.Status == "verified" {
				verifiedReports++
			}
		}
	}
	out := StaffDashboard{Month: month, ByCoordinator: []CoordinatorRecap{}, WithoutMentor: withoutMentor, Indicators: []Indicator{
		{"ontime", "Lapbul terkirim tepat waktu", ontime, mentees, 0.85},
		{"responded", "Lapbul ditanggapi sesuai tenggat", respondedOnTime, sent, 1},
		{"reports", "Laporan mentoring diajukan", submittedReports, len(mentors), 1},
		{"verified", "Laporan mentoring diverifikasi", verifiedReports, len(mentors), 1},
		{"flags", "Bendera ditindaklanjuti ≤ 3 hari", flagsOnTime, flags, 1},
	}}
	for _, c := range w.All() {
		if !c.IsCoordinator || c.Status != "active" {
			continue
		}
		recap := CoordinatorRecap{Coordinator: w.MustPerson(c.UserID)}
		mine := map[int32]bool{}
		for _, m := range w.MentorsOf(c.UserID) {
			mine[m.UserID] = true
			recap.Mentors++
			recap.Mentees += len(w.MenteesOf(m.UserID))
			if r, ok := reportOf[m.UserID]; ok {
				if r.Status != "draft" {
					recap.ReportsSubmitted++
				}
				if r.Status == "verified" {
					recap.ReportsVerified++
				}
			}
		}
		for _, j := range journals {
			if j.MentorUserID == nil || !mine[*j.MentorUserID] {
				continue
			}
			if j.Status != "draft" {
				recap.Submitted++
			}
			if j.IsLate {
				recap.Late++
			}
			if Responded(j.Status) {
				recap.Responded++
			}
			if f := deref(j.FlagStatus); f == "open" || f == "escalated" {
				recap.ActiveFlags++
			}
		}
		out.ByCoordinator = append(out.ByCoordinator, recap)
	}
	items, err := w.followups(ctx)
	if err != nil {
		return out, err
	}
	for _, f := range items {
		if f.Status == "open" {
			out.FollowupsOpen++
		}
	}
	return out, nil
}

// ---- follow-ups ----------------------------------------------------------------

func (w *World) followup(ctx context.Context, by *int32, at pgtype.Timestamptz, note *string) *Followup {
	if !at.Valid {
		return nil
	}
	return &Followup{By: w.AdminName(ctx, by), At: isoValue(at), Note: deref(note)}
}

func pulseText(v *int16) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(*v)
}

// followups is the 'Perlu tindak lanjut' list (DB_DESIGN §5): lapbul flagged
// to the pengelola, escalated wellbeing flags and cases from reports.
func (w *World) followups(ctx context.Context) ([]FollowupItem, error) {
	journals, err := w.Q.PerintisFollowupJournals(ctx)
	if err != nil {
		return nil, err
	}
	items := []FollowupItem{}
	for _, j := range journals {
		if err := w.loadPeople(ctx, []int32{j.MenteeUserID}); err != nil {
			return nil, err
		}
		subject := w.Name(j.MenteeUserID)
		base := FollowupItem{ID: j.ID, Month: monthOf(j.Month), Subject: subject, MentorName: w.NamePtr(j.MentorUserID),
			CoordinatorName: w.NamePtr(w.CoordinatorOfMentor(j.MentorUserID))}
		if j.FlagReason != nil && (j.Status == "flagged" || j.FollowupAt.Valid) {
			item := base
			item.Source, item.Detail, item.OpenedAt = "journal", *j.FlagReason, isoValue(j.RespondedAt)
			item.Status = "open"
			if j.Status != "flagged" {
				item.Status, item.Handled = "handled", w.followup(ctx, j.FollowupBy, j.FollowupAt, j.FollowupNote)
			}
			items = append(items, item)
		}
		if status := deref(j.FlagStatus); status == "escalated" || status == "resolved" {
			detail := fmt.Sprintf("%s · pulse energi %s, akademik %s, keuangan %s, relasi %s.", FlagTriggerLabel[deref(j.FlagTrigger)],
				pulseText(j.PulseEnergy), pulseText(j.PulseAcademic), pulseText(j.PulseFinance), pulseText(j.PulseRelation))
			if j.FlagPrivateNote != nil {
				detail += " Catatan mentor: " + *j.FlagPrivateNote
			} else {
				detail += " Mentor belum menandai sudah dihubungi."
			}
			item := base
			item.Source, item.Detail, item.Status = "flag", detail, "open"
			item.OpenedAt = isoValue(j.SubmittedAt)
			if status == "resolved" {
				item.Status, item.Handled = "handled", w.followup(ctx, j.FollowupBy, j.FollowupAt, j.FollowupNote)
			}
			items = append(items, item)
		}
	}
	reports, err := w.Q.PerintisCaseReports(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range reports {
		if r.Status == "draft" || r.CaseNote == nil {
			continue
		}
		name := w.Name(r.MentorUserID)
		item := FollowupItem{Source: "report", ID: r.ID, Month: monthOf(r.Month), Subject: "Kelompok " + name,
			MentorName: strPtr(name), CoordinatorName: w.NamePtr(w.Participants[r.MentorUserID].CoordinatorUserID),
			Detail: *r.CaseNote, OpenedAt: isoValue(r.SubmittedAt), Status: "open"}
		if deref(r.CaseStatus) == "resolved" {
			item.Status, item.Handled = "handled", w.followup(ctx, r.FollowupBy, r.FollowupAt, r.FollowupNote)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].Status != items[b].Status {
			return items[a].Status == "open"
		}
		return items[a].OpenedAt < items[b].OpenedAt
	})
	return items, nil
}

func (s *StaffService) Followups(ctx context.Context, source, status string) ([]FollowupItem, error) {
	w, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	all, err := w.followups(ctx)
	if err != nil {
		return nil, err
	}
	items := []FollowupItem{}
	for _, f := range all {
		if (source == "" || f.Source == source) && (status == "" || f.Status == status) {
			items = append(items, f)
		}
	}
	return items, nil
}

func (s *StaffService) ResolveFollowup(ctx context.Context, staff int32, source string, id int32, note string) error {
	text, err := RequireText(note, "Catatan penanganan", 1000)
	if err != nil {
		return err
	}
	return s.write(ctx, func(w *World) error {
		switch source {
		case "journal", "flag":
			j, err := w.Q.PerintisJournalForUpdate(ctx, id)
			if notFound(err) {
				return fail(404, "Catatan Perjalanan tidak ditemukan.")
			}
			if err != nil {
				return err
			}
			if source == "journal" {
				if j.Status != "flagged" {
					return fail(409, "Sudah ditangani.")
				}
				j.Status = "accepted"
			} else {
				if deref(j.FlagStatus) != "escalated" {
					return fail(409, "Sudah ditangani.")
				}
				j.FlagStatus = strPtr("resolved")
			}
			j.FollowupBy, j.FollowupAt, j.FollowupNote = &staff, timestamptz(w.Now), &text
			_, err = SaveJournal(ctx, w.Q, j)
			return err
		case "report":
			r, err := w.Q.PerintisReportForUpdate(ctx, id)
			if notFound(err) {
				return fail(404, "Laporan tidak ditemukan.")
			}
			if err != nil {
				return err
			}
			if deref(r.CaseStatus) != "open" || r.Status == "draft" {
				return fail(409, "Sudah ditangani.")
			}
			r.CaseStatus = strPtr("resolved")
			r.FollowupBy, r.FollowupAt, r.FollowupNote = &staff, timestamptz(w.Now), &text
			_, err = SaveReport(ctx, w.Q, r)
			return err
		}
		return fail(404, "Sumber tidak dikenal.")
	})
}

// ---- participants ----------------------------------------------------------------

func (w *World) row(p dbgen.PerintisParticipant) ParticipantRow {
	person := w.People[p.UserID]
	roleSource := p
	roleSource.Status = "active"
	coordinator := w.CoordinatorOfMentor(p.MentorUserID)
	if p.IsMentor {
		coordinator = p.CoordinatorUserID
	}
	row := ParticipantRow{UserID: p.UserID, Name: person.Name, Email: deref(person.Email), Nim: p.Nim, Roles: Roles(roleSource),
		Category: p.Category, KipK: p.KipK, University: deref(person.University), Tingkat: w.TingkatOf(p.UserID),
		MentorName: w.NamePtr(p.MentorUserID), CoordinatorName: w.NamePtr(coordinator), Status: p.Status,
		HasAccount: w.HasAccount(p.UserID), ClubOK: w.HasClub(p.UserID)}
	if p.IsMentor {
		row.MenteeCount = len(w.MenteesOf(p.UserID))
	}
	return row
}

func (w *World) allIDs() []int32 {
	ids := make([]int32, 0, len(w.Participants))
	for id := range w.Participants {
		ids = append(ids, id)
	}
	return ids
}

// Participants lists participants by role, status and search text. mentor
// "none" keeps only mentees without a mentor.
func (s *StaffService) Participants(ctx context.Context, role, status, mentor, q string) ([]ParticipantRow, error) {
	w, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.LoadDetails(ctx, w.allIDs()); err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	rows := []ParticipantRow{}
	for _, p := range w.All() {
		r := w.row(p)
		hasRole := role == ""
		for _, x := range r.Roles {
			hasRole = hasRole || x == role
		}
		matches := q == "" || strings.Contains(strings.ToLower(r.Name), q) || strings.Contains(strings.ToLower(r.Email), q) || strings.Contains(deref(r.Nim), q)
		noMentor := mentor != "none" || (p.IsMentee && p.MentorUserID == nil)
		if hasRole && (status == "" || r.Status == status) && noMentor && matches {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func (w *World) detail(ctx context.Context, id int32) (ParticipantDetail, error) {
	p, err := w.Find(id)
	if err != nil {
		return ParticipantDetail{}, err
	}
	if err := w.LoadDetails(ctx, []int32{id}); err != nil {
		return ParticipantDetail{}, err
	}
	out := ParticipantDetail{Row: w.row(p), Participant: w.ParticipantView(p), Profile: w.Profile(id),
		Mentees: []PersonSummary{}, Mentors: []PersonSummary{}, Journals: []JournalListItem{}}
	for _, m := range w.MenteesOf(id) {
		out.Mentees = append(out.Mentees, w.MustPerson(m.UserID))
	}
	for _, m := range w.MentorsOf(id) {
		out.Mentors = append(out.Mentors, w.MustPerson(m.UserID))
	}
	if p.IsMentee {
		if out.Journals, err = w.JournalList(ctx, id); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *StaffService) Participant(ctx context.Context, id int32) (ParticipantDetail, error) {
	w, err := s.read(ctx)
	if err != nil {
		return ParticipantDetail{}, err
	}
	return w.detail(ctx, id)
}

// UpdateParticipantInput changes roles and role fields (not the mentor; see MoveMentee).
type UpdateParticipantInput struct {
	IsMentee               bool    `json:"isMentee"`
	IsMentor               bool    `json:"isMentor"`
	IsCoordinator          bool    `json:"isCoordinator"`
	Nim                    *string `json:"nim"`
	KipK                   *bool   `json:"kipK"`
	Category               *string `json:"category"`
	KaderisasiOverride     *string `json:"kaderisasiOverride"`
	KaderisasiOverrideNote *string `json:"kaderisasiOverrideNote"`
	CoordinatorUserID      *int32  `json:"coordinatorUserId"`
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *StaffService) UpdateParticipant(ctx context.Context, staff, id int32, in UpdateParticipantInput) (ParticipantDetail, error) {
	var out ParticipantDetail
	err := s.write(ctx, func(w *World) error {
		p, err := w.Q.PerintisParticipantForUpdate(ctx, id)
		if notFound(err) {
			return fail(404, "Peserta tidak ditemukan.")
		}
		if err != nil {
			return err
		}
		if !in.IsMentee && !in.IsMentor && !in.IsCoordinator {
			return fail(422, "Pilih minimal satu peran.")
		}
		if p.IsMentor && !in.IsMentor && len(w.MenteesOf(id)) > 0 {
			return fail(409, "Peran mentor tidak bisa dicabut selama masih punya mentee aktif.")
		}
		if p.IsCoordinator && !in.IsCoordinator && len(w.MentorsOf(id)) > 0 {
			return fail(409, "Peran koordinator tidak bisa dicabut selama masih membawahi mentor.")
		}
		nim := nilIfEmpty(strings.TrimSpace(deref(in.Nim)))
		if in.IsMentee {
			// NIM, kategori and KIP-K may be filled in later; a mentee without
			// a kategori cannot start a lapbul until it is set.
			if err := checkMenteeData(nim, in.Category); err != nil {
				return err
			}
			for _, other := range w.Participants {
				if nim != nil && other.UserID != id && deref(other.Nim) == *nim {
					return fail(409, "NIM sudah dipakai peserta lain.")
				}
			}
			if o := deref(in.KaderisasiOverride); o != "" && o != "lmd_lama" && o != "lmd_242" && o != "setara" && o != "belum" {
				return fail(422, "Status kaderisasi tidak dikenal.")
			}
		}
		if in.IsMentor && in.CoordinatorUserID != nil {
			c, ok := w.Participants[*in.CoordinatorUserID]
			if !ok || !c.IsCoordinator {
				return fail(422, "Koordinator tidak valid.")
			}
		}
		p.IsMentee, p.IsMentor, p.IsCoordinator = in.IsMentee, in.IsMentor, in.IsCoordinator
		p.Nim, p.KipK, p.Category, p.KaderisasiOverride, p.KaderisasiOverrideNote = nil, nil, nil, nil, nil
		if in.IsMentee {
			p.Nim, p.KipK, p.Category = nim, in.KipK, nilIfEmpty(deref(in.Category))
			p.KaderisasiOverride, p.KaderisasiOverrideNote = nilIfEmpty(deref(in.KaderisasiOverride)), nilIfEmpty(deref(in.KaderisasiOverrideNote))
			if p.UniversityID == nil {
				p.UniversityID = w.People[id].UniversityID
			}
		} else {
			p.MentorUserID = nil
		}
		p.CoordinatorUserID = nil
		if in.IsMentor {
			p.CoordinatorUserID = in.CoordinatorUserID
		}
		p.UpdatedBy = &staff
		saved, err := SaveParticipant(ctx, w.Q, p)
		if uniqueViolation(err) {
			return fail(409, "NIM sudah dipakai peserta lain di kampus yang sama.")
		}
		if err != nil {
			return err
		}
		w.Participants[id] = saved
		out, err = w.detail(ctx, id)
		return err
	})
	return out, err
}

func (s *StaffService) MoveMentee(ctx context.Context, staff, id int32, mentor *int32) (ParticipantDetail, error) {
	var out ParticipantDetail
	err := s.write(ctx, func(w *World) error {
		p, err := w.Q.PerintisParticipantForUpdate(ctx, id)
		if notFound(err) {
			return fail(404, "Peserta tidak ditemukan.")
		}
		if err != nil {
			return err
		}
		if !p.IsMentee {
			return fail(422, "Hanya mentee yang punya mentor.")
		}
		if mentor != nil {
			if *mentor == id {
				return fail(422, "Mentee tidak bisa menjadi mentornya sendiri.")
			}
			m, err := w.Q.PerintisParticipantForUpdate(ctx, *mentor)
			if notFound(err) || (err == nil && (!m.IsMentor || m.Status == "inactive")) {
				return fail(422, "Mentor tidak valid.")
			}
			if err != nil {
				return err
			}
		}
		p.MentorUserID, p.UpdatedBy = mentor, &staff
		if w.Participants[id], err = SaveParticipant(ctx, w.Q, p); err != nil {
			return err
		}
		// The open month's lapbul moves with the mentee until it is answered (PRD §8.7 no. 3).
		if month := CurrentReportMonth(w.Now); month != "" {
			j, err := w.Q.PerintisJournalByMonthForUpdate(ctx, dbgen.PerintisJournalByMonthForUpdateParams{MenteeUserID: id, Month: DateOf(month)})
			if err == nil && !Responded(j.Status) {
				j.MentorUserID = mentor
				if _, err := SaveJournal(ctx, w.Q, j); err != nil {
					return err
				}
			} else if err != nil && !notFound(err) {
				return err
			}
		}
		out, err = w.detail(ctx, id)
		return err
	})
	return out, err
}

func (s *StaffService) SetStatus(ctx context.Context, staff, id int32, status, reason string) (ParticipantDetail, error) {
	var out ParticipantDetail
	err := s.write(ctx, func(w *World) error {
		p, err := w.Q.PerintisParticipantForUpdate(ctx, id)
		if notFound(err) {
			return fail(404, "Peserta tidak ditemukan.")
		}
		if err != nil {
			return err
		}
		switch status {
		case "inactive":
			text, err := RequireText(reason, "Alasan nonaktif", 500)
			if err != nil {
				return err
			}
			if p.IsMentor && len(w.MenteesOf(id)) > 0 {
				return fail(409, "Pindahkan dulu mentee mentor ini ke mentor lain.")
			}
			p.InactiveReason = &text
		case "active":
			p.InactiveReason = nil
		default:
			return fail(422, "Status tidak dikenal.")
		}
		p.Status, p.UpdatedBy = status, &staff
		if w.Participants[id], err = SaveParticipant(ctx, w.Q, p); err != nil {
			return err
		}
		out, err = w.detail(ctx, id)
		return err
	})
	return out, err
}

// ---- lapbul (read + reopen) -----------------------------------------------------------

func (s *StaffService) Journal(ctx context.Context, id int32) (JournalForMentor, error) {
	w, err := s.read(ctx)
	if err != nil {
		return JournalForMentor{}, err
	}
	j, err := w.Q.PerintisJournal(ctx, id)
	if notFound(err) {
		return JournalForMentor{}, fail(404, "Catatan Perjalanan tidak ditemukan.")
	}
	if err != nil {
		return JournalForMentor{}, err
	}
	return w.ReaderView(ctx, j, false, strPtr("Pengelola hanya membaca."))
}

// Reopen lets a mentee fill a locked month (reason required); only a lapbul
// that was never submitted can be reopened.
func (s *StaffService) Reopen(ctx context.Context, staff, mentee int32, month, reason string) error {
	text, err := RequireText(reason, "Alasan", 500)
	if err != nil {
		return err
	}
	return s.write(ctx, func(w *World) error {
		p, ok := w.Participants[mentee]
		if !ok || !p.IsMentee {
			return fail(404, "Mentee tidak ditemukan.")
		}
		valid := false
		for _, m := range ReportMonths(w.Now) {
			valid = valid || m == month
		}
		if !valid {
			return fail(404, "Bulan laporan tidak tersedia.")
		}
		j, err := w.Q.PerintisJournalByMonthForUpdate(ctx, dbgen.PerintisJournalByMonthForUpdateParams{MenteeUserID: mentee, Month: DateOf(month)})
		if notFound(err) {
			category := deref(p.Category)
			if category == "" {
				category = "dasar"
			}
			j, err = w.Q.InsertPerintisJournal(ctx, dbgen.InsertPerintisJournalParams{MenteeUserID: mentee, Month: DateOf(month),
				MentorUserID: p.MentorUserID, Category: category, FormVersion: FormVersion})
		}
		if err != nil {
			return err
		}
		if j.Status != "draft" {
			return fail(409, "Hanya lapbul yang belum terkirim yang bisa dibuka kembali.")
		}
		j.ReopenedBy, j.ReopenReason = &staff, &text
		_, err = SaveJournal(ctx, w.Q, j)
		return err
	})
}

// File streams a lapbul file or group photo to the pengelola.
func (s *StaffService) File(ctx context.Context, key string) ([]byte, string, error) {
	q := dbgen.New(s.Pool)
	denied := fail(404, "Berkas tidak ditemukan.")
	var files []File
	var id, mentor int32
	switch {
	case scanKey(key, "perintis/journals/%d/", &id):
		j, err := q.PerintisJournal(ctx, id)
		if err != nil {
			return nil, "", denied
		}
		files = ParseFiles(j.Files)
	case scanKey(key, "perintis/reports/%d/", &mentor):
		reports, err := q.PerintisReportsOfMentor(ctx, mentor)
		if err != nil {
			return nil, "", denied
		}
		for _, r := range reports {
			files = append(files, ParseFiles(r.Photos)...)
		}
	default:
		return nil, "", denied
	}
	for _, f := range files {
		if f.Key == key {
			data, err := s.Storage.Get(ctx, key)
			if err != nil {
				return nil, "", denied
			}
			return data, f.Mime, nil
		}
	}
	return nil, "", denied
}

func scanKey(key, pattern string, into *int32) bool {
	var n int32
	prefix := strings.SplitN(pattern, "%d", 2)
	if !strings.HasPrefix(key, prefix[0]) {
		return false
	}
	rest := strings.TrimPrefix(key, prefix[0])
	idPart, file, ok := strings.Cut(rest, "/")
	if !ok || file == "" || strings.Contains(file, "/") {
		return false
	}
	if _, err := fmt.Sscanf(idPart, "%d", &n); err != nil || fmt.Sprint(n) != idPart {
		return false
	}
	*into = n
	return true
}

// ---- import ------------------------------------------------------------------------

var importColumns = map[string][]string{
	"mentee":   {"nim", "nama", "email", "kategori", "kip_k"},
	"mentor":   {"email", "email_koordinator"},
	"matching": {"nim", "email_mentor"},
}

func parseCSV(text string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fail(422, "Berkas CSV tidak terbaca.")
	}
	out := [][]string{}
	for _, rec := range records {
		empty := true
		for i := range rec {
			rec[i] = strings.TrimSpace(rec[i])
			empty = empty && rec[i] == ""
		}
		if !empty {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (w *World) participantByEmail(ctx context.Context, email string) (*dbgen.PerintisPublicUserByEmailRow, error) {
	u, err := w.Q.PerintisPublicUserByEmail(ctx, email)
	if notFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (w *World) participantByNIM(nim string) *dbgen.PerintisParticipant {
	for _, p := range w.Participants {
		if deref(p.Nim) == nim {
			p := p
			return &p
		}
	}
	return nil
}

func (w *World) preview(ctx context.Context, kind, text string) (ImportPreview, error) {
	columns, ok := importColumns[kind]
	if !ok {
		return ImportPreview{}, fail(422, "Jenis impor tidak dikenal.")
	}
	records, err := parseCSV(text)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(records) == 0 || strings.ToLower(strings.Join(records[0], ",")) != strings.Join(columns, ",") {
		return ImportPreview{}, fail(422, "Kolom harus persis: %s", strings.Join(columns, ", "))
	}
	if len(records) > 1001 {
		return ImportPreview{}, fail(422, "Paling banyak 1.000 baris sekali impor.")
	}
	seen := map[string]int{}
	rows := []ImportPreviewRow{}
	for i, values := range records[1:] {
		line := i + 2
		row := func(result, message string) ImportPreviewRow {
			return ImportPreviewRow{Line: line, Values: values, Result: result, Message: message}
		}
		if len(values) != len(columns) {
			rows = append(rows, row("error", "Jumlah kolom tidak sesuai."))
			continue
		}
		switch kind {
		case "mentee":
			// NIM, kategori and KIP-K may be left empty and filled in later
			// (as in Tambah peserta); a value that is given must be valid.
			nim, email, category, kip := values[0], strings.ToLower(values[2]), values[3], strings.ToLower(values[4])
			if first, dup := seen[nim]; dup && nim != "" {
				rows = append(rows, row("error", fmt.Sprintf("NIM ganda dengan baris %d", first)))
				continue
			}
			if nim != "" {
				seen[nim] = line
			}
			if values[1] == "" || !strings.Contains(email, "@") {
				rows = append(rows, row("error", "Nama dan email wajib diisi"))
				continue
			}
			if len(nim) > 30 {
				rows = append(rows, row("error", "NIM paling banyak 30 karakter"))
				continue
			}
			if category != "" && CategoryLabel[category] == "" {
				rows = append(rows, row("error", "Kategori tidak dikenal: "+category))
				continue
			}
			if kip != "" && kip != "ya" && kip != "tidak" {
				rows = append(rows, row("error", "kip_k harus ya/tidak atau dikosongkan"))
				continue
			}
			user, err := w.participantByEmail(ctx, email)
			if err != nil {
				return ImportPreview{}, err
			}
			if owner := w.participantByNIM(nim); nim != "" && owner != nil && (user == nil || owner.UserID != user.ID) {
				rows = append(rows, row("error", "NIM sudah dipakai peserta lain"))
				continue
			}
			switch {
			case user == nil:
				rows = append(rows, row("new", "Baru · akun belum aktif (no_account)"))
			case w.Participants[user.ID].IsMentee:
				rows = append(rows, row("warning", "Sudah terdaftar sebagai mentee · data diperbarui"))
			default:
				if err := w.loadPeople(ctx, []int32{user.ID}); err != nil {
					return ImportPreview{}, err
				}
				rows = append(rows, row("match", "Cocok · akun kaderisasi "+w.Name(user.ID)))
			}
		case "mentor":
			email, coordinatorEmail := strings.ToLower(values[0]), strings.ToLower(values[1])
			if !strings.Contains(email, "@") {
				rows = append(rows, row("error", "Email mentor tidak valid"))
				continue
			}
			coordinator, err := w.participantByEmail(ctx, coordinatorEmail)
			if err != nil {
				return ImportPreview{}, err
			}
			if coordinator == nil || !w.Participants[coordinator.ID].IsCoordinator {
				rows = append(rows, row("error", "Email koordinator tidak ditemukan"))
				continue
			}
			user, err := w.participantByEmail(ctx, email)
			if err != nil {
				return ImportPreview{}, err
			}
			if user == nil {
				rows = append(rows, row("new", "Baru · akun mentor dibuat (no_account)"))
			} else {
				if err := w.loadPeople(ctx, []int32{user.ID}); err != nil {
					return ImportPreview{}, err
				}
				rows = append(rows, row("match", "Cocok · "+w.Name(user.ID)))
			}
		case "matching":
			mentee := w.participantByNIM(values[0])
			if mentee == nil || !mentee.IsMentee {
				rows = append(rows, row("error", "NIM tidak ditemukan"))
				continue
			}
			mentor, err := w.participantByEmail(ctx, strings.ToLower(values[1]))
			if err != nil {
				return ImportPreview{}, err
			}
			if mentor == nil || !w.Participants[mentor.ID].IsMentor || w.Participants[mentor.ID].Status == "inactive" {
				rows = append(rows, row("error", "Email mentor tidak ditemukan"))
				continue
			}
			if mentor.ID == mentee.UserID {
				rows = append(rows, row("error", "Mentee tidak bisa menjadi mentornya sendiri"))
				continue
			}
			rows = append(rows, row("match", w.Name(mentee.UserID)+" → "+w.Name(mentor.ID)))
		}
	}
	canApply := len(rows) > 0
	for _, r := range rows {
		canApply = canApply && r.Result != "error"
	}
	return ImportPreview{Kind: kind, Columns: columns, Rows: rows, CanApply: canApply}, nil
}

func (s *StaffService) ImportPreview(ctx context.Context, kind, text string) (ImportPreview, error) {
	w, err := s.read(ctx)
	if err != nil {
		return ImportPreview{}, err
	}
	return w.preview(ctx, kind, text)
}

// ensureUser finds the kaderisasi account by email or creates a no_account
// one (activated on first sign-in) with a profile name.
func (w *World) ensureUser(ctx context.Context, email, name string) (int32, error) {
	user, err := w.participantByEmail(ctx, email)
	if err != nil {
		return 0, err
	}
	if user != nil {
		return user.ID, nil
	}
	id, err := w.Q.InsertPerintisPublicUser(ctx, email)
	if err != nil {
		return 0, err
	}
	if err := w.Q.InsertPerintisProfile(ctx, dbgen.InsertPerintisProfileParams{UserID: id, Name: name}); err != nil {
		return 0, err
	}
	return id, w.loadPeople(ctx, []int32{id})
}

func (w *World) upsert(ctx context.Context, p dbgen.PerintisParticipant, exists bool) error {
	var saved dbgen.PerintisParticipant
	var err error
	if exists {
		saved, err = SaveParticipant(ctx, w.Q, p)
	} else {
		saved, err = w.Q.InsertPerintisParticipant(ctx, dbgen.InsertPerintisParticipantParams{
			UserID: p.UserID, IsMentee: p.IsMentee, IsMentor: p.IsMentor, IsCoordinator: p.IsCoordinator, Nim: p.Nim,
			UniversityID: p.UniversityID, KipK: p.KipK, Category: p.Category, MentorUserID: p.MentorUserID,
			CoordinatorUserID: p.CoordinatorUserID, Status: p.Status, UpdatedBy: p.UpdatedBy})
	}
	if uniqueViolation(err) {
		return fail(409, "NIM %s sudah dipakai peserta lain.", deref(p.Nim))
	}
	if err != nil {
		return err
	}
	w.Participants[p.UserID] = saved
	return nil
}

func (s *StaffService) ImportApply(ctx context.Context, staff int32, kind, text string) (int, error) {
	applied := 0
	err := s.write(ctx, func(w *World) error {
		result, err := w.preview(ctx, kind, text)
		if err != nil {
			return err
		}
		if !result.CanApply {
			return fail(422, "Masih ada baris galat. Perbaiki berkas lalu pratinjau ulang.")
		}
		for _, r := range result.Rows {
			v := r.Values
			switch kind {
			case "mentee":
				id, err := w.ensureUser(ctx, strings.ToLower(v[2]), v[1])
				if err != nil {
					return err
				}
				p, exists := w.Participants[id]
				if !exists {
					p = dbgen.PerintisParticipant{UserID: id, Status: "active"}
				}
				// An empty cell keeps what is stored (nothing, for a new mentee).
				p.IsMentee, p.UpdatedBy = true, &staff
				if v[0] != "" {
					p.Nim = strPtr(v[0])
				}
				if v[3] != "" {
					p.Category = strPtr(v[3])
				}
				if kip := strings.ToLower(v[4]); kip != "" {
					yes := kip == "ya"
					p.KipK = &yes
				}
				if p.UniversityID == nil {
					p.UniversityID = w.People[id].UniversityID
				}
				if err := w.upsert(ctx, p, exists); err != nil {
					return err
				}
			case "mentor":
				email := strings.ToLower(v[0])
				id, err := w.ensureUser(ctx, email, strings.Split(email, "@")[0])
				if err != nil {
					return err
				}
				coordinator, err := w.participantByEmail(ctx, strings.ToLower(v[1]))
				if err != nil {
					return err
				}
				p, exists := w.Participants[id]
				if !exists {
					p = dbgen.PerintisParticipant{UserID: id, Status: "active"}
				}
				p.IsMentor, p.CoordinatorUserID, p.UpdatedBy = true, &coordinator.ID, &staff
				if err := w.upsert(ctx, p, exists); err != nil {
					return err
				}
			case "matching":
				mentee := *w.participantByNIM(v[0])
				mentor, err := w.participantByEmail(ctx, strings.ToLower(v[1]))
				if err != nil {
					return err
				}
				mentee.MentorUserID, mentee.UpdatedBy = &mentor.ID, &staff
				if err := w.upsert(ctx, mentee, true); err != nil {
					return err
				}
			}
			applied++
		}
		return nil
	})
	return applied, err
}

// ---- add from kaderisasi members (Super Admin) ---------------------------------

// MemberCandidate is one kaderisasi member found by the member search. Roles
// lists the Perintis roles they already hold; empty means not yet a participant.
type MemberCandidate struct {
	UserID      int32    `json:"userId"`
	Name        string   `json:"name"`
	Email       *string  `json:"email"`
	MemberID    *string  `json:"memberId"`
	University  *string  `json:"university"`
	Major       *string  `json:"major"`
	IntakeYear  *int32   `json:"intakeYear"`
	HasAccount  bool     `json:"hasAccount"`
	Participant bool     `json:"participant"`
	Roles       []string `json:"roles"`
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchMembers finds up to 20 kaderisasi members by name, email or member ID.
func (s *StaffService) SearchMembers(ctx context.Context, q string) ([]MemberCandidate, error) {
	q = strings.TrimSpace(q)
	if len([]rune(q)) < 3 {
		return nil, fail(422, "Ketik minimal 3 huruf untuk mencari anggota.")
	}
	if len([]rune(q)) > 100 {
		return nil, fail(422, "Kata kunci terlalu panjang.")
	}
	w, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := w.Q.PerintisMemberSearch(ctx, dbgen.PerintisMemberSearchParams{Pattern: likeEscaper.Replace(q), Exact: q})
	if err != nil {
		return nil, err
	}
	out := make([]MemberCandidate, 0, len(rows))
	for _, r := range rows {
		c := MemberCandidate{UserID: r.UserID, Name: r.Name, Email: r.Email, MemberID: r.MemberID,
			University: r.University, Major: r.Major, IntakeYear: r.IntakeYear,
			HasAccount: r.AccountStatus != "no_account", Roles: []string{}}
		if p, ok := w.Participants[r.UserID]; ok {
			c.Participant = true
			c.Roles = Roles(p)
		}
		out = append(out, c)
	}
	return out, nil
}

// checkMenteeData validates the optional mentee fields that were given.
func checkMenteeData(nim *string, category *string) error {
	if nim != nil && len(*nim) > 30 {
		return fail(422, "NIM paling banyak 30 karakter.")
	}
	if c := deref(category); c != "" && CategoryLabel[c] == "" {
		return fail(422, "Kategori tidak dikenal.")
	}
	return nil
}

// AddParticipantInput makes an existing kaderisasi member a participant.
type AddParticipantInput struct {
	UserID int32 `json:"userId"`
	UpdateParticipantInput
	MentorUserID *int32 `json:"mentorUserId"`
}

// AddParticipant enrols a member found by SearchMembers with the chosen roles.
// The checks match UpdateParticipant; NIM, kategori and KIP-K are optional.
// New participants start active.
func (s *StaffService) AddParticipant(ctx context.Context, staff int32, in AddParticipantInput) (ParticipantDetail, error) {
	var out ParticipantDetail
	err := s.write(ctx, func(w *World) error {
		if in.UserID <= 0 {
			return fail(422, "Pilih anggota yang akan ditambahkan.")
		}
		if _, exists := w.Participants[in.UserID]; exists {
			return fail(409, "Anggota ini sudah menjadi peserta. Ubah perannya dari halaman peserta.")
		}
		if err := w.loadPeople(ctx, []int32{in.UserID}); err != nil {
			return err
		}
		if _, ok := w.People[in.UserID]; !ok {
			return fail(404, "Anggota tidak ditemukan.")
		}
		if !in.IsMentee && !in.IsMentor && !in.IsCoordinator {
			return fail(422, "Pilih minimal satu peran.")
		}
		p := dbgen.PerintisParticipant{UserID: in.UserID, IsMentee: in.IsMentee, IsMentor: in.IsMentor,
			IsCoordinator: in.IsCoordinator, Status: "active", UpdatedBy: &staff}
		if in.IsMentee {
			nim := nilIfEmpty(strings.TrimSpace(deref(in.Nim)))
			if err := checkMenteeData(nim, in.Category); err != nil {
				return err
			}
			if nim != nil && w.participantByNIM(*nim) != nil {
				return fail(409, "NIM sudah dipakai peserta lain.")
			}
			p.Nim, p.Category, p.KipK, p.UniversityID = nim, nilIfEmpty(deref(in.Category)), in.KipK, w.People[in.UserID].UniversityID
			if in.MentorUserID != nil {
				m, ok := w.Participants[*in.MentorUserID]
				if !ok || !m.IsMentor || m.Status == "inactive" || m.UserID == in.UserID {
					return fail(422, "Mentor tidak valid.")
				}
				p.MentorUserID = in.MentorUserID
			}
		}
		if in.IsMentor {
			if in.CoordinatorUserID != nil {
				c, ok := w.Participants[*in.CoordinatorUserID]
				if !ok || !c.IsCoordinator || c.Status == "inactive" {
					return fail(422, "Koordinator tidak valid.")
				}
				p.CoordinatorUserID = in.CoordinatorUserID
			}
		}
		if err := w.upsert(ctx, p, false); err != nil {
			return err
		}
		var err error
		out, err = w.detail(ctx, in.UserID)
		return err
	})
	return out, err
}

// ---- export ------------------------------------------------------------------------

// csvCell neutralises spreadsheet formulas and renders booleans as ya/tidak.
// Export labels match the app's wording (content/labels.ts in the frontend).
var (
	exportRoleLabel   = map[string]string{"mentee": "Mentee", "mentor": "Mentor", "coordinator": "Koordinator"}
	exportStatusLabel = map[string]string{"active": "Aktif", "inactive": "Nonaktif"}
)

func labelOr(labels map[string]string, code string) string {
	if label, ok := labels[code]; ok {
		return label
	}
	return code
}

func csvCell(v any) string {
	var s string
	switch x := v.(type) {
	case nil:
		s = ""
	case bool:
		s = map[bool]string{true: "ya", false: "tidak"}[x]
	case *string:
		s = deref(x)
	case *bool:
		if x != nil {
			s = csvCell(*x)
		}
	default:
		s = fmt.Sprint(x)
	}
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		s = "'" + s
	}
	return s
}

func (s *StaffService) Export(ctx context.Context, kind, month string) (string, string, error) {
	w, err := s.read(ctx)
	if err != nil {
		return "", "", err
	}
	if month, err = w.staffMonth(month); err != nil {
		return "", "", err
	}
	label := MonthLabel(month)
	all := w.All()
	if err := w.LoadDetails(ctx, w.allIDs()); err != nil {
		return "", "", err
	}
	journals, err := w.Q.PerintisJournalsInMonth(ctx, DateOf(month))
	if err != nil {
		return "", "", err
	}
	journalOf := map[int32]dbgen.PerintisJournal{}
	for _, j := range journals {
		journalOf[j.MenteeUserID] = j
	}
	reports, err := w.Q.PerintisReportsInMonth(ctx, DateOf(month))
	if err != nil {
		return "", "", err
	}
	reportOf := map[int32]dbgen.PerintisSessionReport{}
	for _, r := range reports {
		reportOf[r.MentorUserID] = r
	}
	coordinatorName := func(mentor *int32) string { return deref(w.NamePtr(w.CoordinatorOfMentor(mentor))) }
	var header []string
	var rows [][]any
	mentees, mentors := []dbgen.PerintisParticipant{}, []dbgen.PerintisParticipant{}
	for _, p := range all {
		if p.IsMentee {
			mentees = append(mentees, p)
		}
		if p.IsMentor && p.Status != "inactive" {
			mentors = append(mentors, p)
		}
	}
	switch kind {
	case "rekap-mentee":
		months := ReportMonths(w.Now)
		for i, j := 0, len(months)-1; i < j; i, j = i+1, j-1 {
			months[i], months[j] = months[j], months[i]
		}
		header = []string{"NIM", "Nama", "Kampus", "Prodi", "Angkatan", "KIP-K", "Kategori", "Mentor", "Koordinator", "Status kaderisasi"}
		for _, m := range months {
			header = append(header, "Lapbul "+MonthLabel(m))
		}
		header = append(header, "Unit/club")
		ids := []int32{}
		for _, p := range mentees {
			ids = append(ids, p.UserID)
		}
		history, err := w.Q.PerintisJournalsOfMentees(ctx, ids)
		if err != nil {
			return "", "", err
		}
		byMentee := map[int32]map[string]dbgen.PerintisJournal{}
		for _, j := range history {
			if byMentee[j.MenteeUserID] == nil {
				byMentee[j.MenteeUserID] = map[string]dbgen.PerintisJournal{}
			}
			byMentee[j.MenteeUserID][monthOf(j.Month)] = j
		}
		for _, p := range mentees {
			profile := w.Profile(p.UserID)
			status := profile.LMDStatus
			if p.KaderisasiOverride != nil {
				status = *p.KaderisasiOverride
			}
			var intake any // blank, not 0, when the profile has no angkatan
			if profile.IntakeYear > 0 {
				intake = profile.IntakeYear
			}
			row := []any{p.Nim, profile.Name, profile.University, profile.Major, intake, p.KipK,
				CategoryLabel[deref(p.Category)], deref(w.NamePtr(p.MentorUserID)), coordinatorName(p.MentorUserID), status}
			for _, m := range months {
				if j, ok := byMentee[p.UserID][m]; ok {
					cell := JournalStatusLabel[j.Status]
					if j.IsLate {
						cell += " (terlambat)"
					}
					row = append(row, cell)
				} else {
					row = append(row, "Belum mengisi")
				}
			}
			rows = append(rows, append(row, w.HasClub(p.UserID)))
		}
	case "tugas-mentor":
		header = []string{"Mentor", "Koordinator", "Kelompok", "Bulan", "T1", "T2", "T3", "Status laporan", "Tanggal verifikasi"}
		for _, m := range mentors {
			mine := []dbgen.PerintisJournal{}
			for _, j := range journals {
				if j.MentorUserID != nil && *j.MentorUserID == m.UserID {
					mine = append(mine, j)
				}
			}
			var report *dbgen.PerintisSessionReport
			status, verified := "not_started", ""
			if r, ok := reportOf[m.UserID]; ok {
				report, status = &r, r.Status
				if r.VerifiedAt.Valid {
					verified = FormatDate(r.VerifiedAt.Time)
				}
			}
			t := Tasks(month, report, mine)
			rows = append(rows, []any{w.Name(m.UserID), deref(w.NamePtr(m.CoordinatorUserID)),
				fmt.Sprintf("%d mentee", len(w.MenteesOf(m.UserID))), label, t.T1, t.T2, t.T3, ReportStatusLabel[status], verified})
		}
	case "status-lapbul":
		header = []string{"NIM", "Nama", "Kelompok (mentor)", "Status", "Dikirim", "Terlambat", "Ditanggapi", "Keputusan"}
		for _, p := range mentees {
			j, ok := journalOf[p.UserID]
			mentor, status, sent, responded, decision, late := p.MentorUserID, "not_started", "", "", "", false
			if ok {
				mentor, status, late = j.MentorUserID, j.Status, j.IsLate
				if j.SubmittedAt.Valid {
					sent = FormatDate(j.SubmittedAt.Time)
				}
				if j.RespondedAt.Valid {
					responded = FormatDate(j.RespondedAt.Time)
				}
				if Responded(j.Status) {
					decision = JournalStatusLabel[j.Status]
				}
			}
			rows = append(rows, []any{p.Nim, w.Name(p.UserID), deref(w.NamePtr(mentor)), JournalStatusLabel[status], sent, late, responded, decision})
		}
	case "laporan-mentoring":
		header = []string{"Kelompok (mentor)", "Mentor", "Koordinator", "Bulan", "Tanggal sesi", "Tema", "Status", "Terlambat", "Tanggal verifikasi"}
		for _, m := range mentors {
			name := w.Name(m.UserID)
			date, theme, status, late, verified := "", "", "not_started", false, ""
			if r, ok := reportOf[m.UserID]; ok {
				status, late = r.Status, r.IsLate
				if r.SessionDate.Valid {
					date = FormatDate(r.SessionDate.Time)
				}
				theme = deref(r.CurriculumCode)
				if r.DeviationReason != nil {
					theme = *r.DeviationReason
				}
				if r.VerifiedAt.Valid {
					verified = FormatDate(r.VerifiedAt.Time)
				}
			}
			rows = append(rows, []any{"Kelompok " + name, name, deref(w.NamePtr(m.CoordinatorUserID)), label, date, theme, ReportStatusLabel[status], late, verified})
		}
	case "data-peserta":
		header = []string{"Nama", "Email", "NIM", "Peran", "Kategori", "Kampus", "Tingkat", "Mentor", "Koordinator", "Jumlah mentee", "Status"}
		for _, p := range all {
			r := w.row(p)
			roles := make([]string, len(r.Roles))
			for i, role := range r.Roles {
				roles[i] = labelOr(exportRoleLabel, role)
			}
			rows = append(rows, []any{r.Name, r.Email, r.Nim, strings.Join(roles, " + "), CategoryLabel[deref(r.Category)], r.University,
				r.Tingkat, r.MentorName, r.CoordinatorName, r.MenteeCount, labelOr(exportStatusLabel, r.Status)})
		}
	default:
		return "", "", fail(404, "Jenis ekspor tidak dikenal.")
	}
	var b bytes.Buffer
	out := csv.NewWriter(&b)
	_ = out.Write(header)
	for _, row := range rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = csvCell(v)
		}
		_ = out.Write(cells)
	}
	out.Flush()
	return fmt.Sprintf("perintis-%s-%s.csv", kind, month[:7]), strings.TrimSuffix(b.String(), "\n"), out.Error()
}
