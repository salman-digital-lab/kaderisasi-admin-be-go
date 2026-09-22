package announcement

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type Service struct{ Pool *pgxpool.Pool }
type Record struct {
	dbgen.Announcement
	Audience json.RawMessage `json:"audience"`
}

func Present(row dbgen.Announcement) Record { return Record{row, json.RawMessage(row.Audience)} }
func Missing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ANNOUNCEMENT_NOT_FOUND")
	}
	return err
}
func (s Service) Save(ctx context.Context, id, author int32, in Input) (Record, error) {
	if e := Validate(&in); e != nil {
		return Record{}, e
	}
	raw, _ := json.Marshal(in.Audience)
	q := dbgen.New(s.Pool)
	var row dbgen.Announcement
	var e error
	if id == 0 {
		row, e = q.AnnouncementCreate(ctx, dbgen.AnnouncementCreateParams{Title: in.Title, Body: in.Body, LinkLabel: in.LinkLabel, LinkUrl: in.LinkURL, Audience: raw, AuthorID: &author})
	} else {
		row, e = q.AnnouncementUpdate(ctx, dbgen.AnnouncementUpdateParams{ID: id, Title: in.Title, Body: in.Body, LinkLabel: in.LinkLabel, LinkUrl: in.LinkURL, Audience: raw, Version: in.Version})
		if errors.Is(e, pgx.ErrNoRows) {
			return Record{}, domain.Fail(409, "ANNOUNCEMENT_CHANGED")
		}
	}
	return Present(row), e
}
func resolve(ctx context.Context, q *dbgen.Queries, raw []byte) (Preview, []int32, []int32, error) {
	var a Audience
	if e := json.Unmarshal(raw, &a); e != nil {
		return Preview{}, nil, nil, e
	}
	// Non-nil arrays keep PostgreSQL cardinality and ANY predicates deterministic.
	if a.ActivityStatuses == nil {
		a.ActivityStatuses = []string{}
	}
	rows, e := q.AnnouncementRecipients(ctx, dbgen.AnnouncementRecipientsParams{AllMembers: a.AllMembers, MemberIds: a.MemberIDs, ActivityIds: a.ActivityIDs, ActivityStatuses: a.ActivityStatuses, ClubIds: a.ClubIDs, ClubStatuses: a.ClubStatuses, AllAdmins: a.AllAdmins, AdminIds: a.AdminIDs, RoleCodes: a.RoleCodes})
	p := Preview{}
	members := []int32{}
	admins := []int32{}
	for _, r := range rows {
		if !r.Eligible {
			p.Excluded++
			continue
		}
		p.Eligible++
		if r.Kind == "member" {
			p.Members++
			members = append(members, r.ID)
		} else {
			p.Admins++
			admins = append(admins, r.ID)
		}
	}
	return p, members, admins, e
}
func (s Service) Preview(ctx context.Context, id int32) (Preview, error) {
	q := dbgen.New(s.Pool)
	row, e := q.AnnouncementGet(ctx, id)
	if e != nil {
		return Preview{}, Missing(e)
	}
	p, _, _, e := resolve(ctx, q, row.Audience)
	return p, e
}
func (s Service) Publish(ctx context.Context, id, actor, version int32) (Record, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return Record{}, e
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := dbgen.New(tx)
	row, e := q.AnnouncementLock(ctx, id)
	if e != nil {
		return Record{}, Missing(e)
	}
	if row.State == "published" {
		return Present(row), nil
	}
	if row.State != "draft" || row.Version != version {
		return Record{}, domain.Fail(409, "ANNOUNCEMENT_CHANGED")
	}
	// The same lock used to issue inbox cutoffs ensures unpublished transactions
	// cannot later appear behind an already issued read-all cutoff.
	if _, e = q.AnnouncementClock(ctx); e != nil {
		return Record{}, e
	}
	p, members, admins, e := resolve(ctx, q, row.Audience)
	if e != nil {
		return Record{}, e
	}
	if p.Eligible == 0 {
		return Record{}, domain.Fail(422, "NO_ELIGIBLE_RECIPIENTS")
	}
	if e = q.AnnouncementInsertMembers(ctx, dbgen.AnnouncementInsertMembersParams{AnnouncementID: id, Ids: members}); e != nil {
		return Record{}, e
	}
	if e = q.AnnouncementInsertAdmins(ctx, dbgen.AnnouncementInsertAdminsParams{AnnouncementID: id, Ids: admins}); e != nil {
		return Record{}, e
	}
	row, e = q.AnnouncementPublish(ctx, dbgen.AnnouncementPublishParams{ID: id, PublisherID: &actor, RecipientCount: p.Eligible})
	if e != nil {
		return Record{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Record{}, e
	}
	return Present(row), nil
}
