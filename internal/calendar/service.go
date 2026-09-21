package calendar

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

var WIB = time.FixedZone("WIB", 7*60*60)

type Input struct {
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Location    *string   `json:"location"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	AllDay      bool      `json:"all_day"`
	ActivityID  *int32    `json:"activity_id"`
}
type Activity struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	IsPublished bool   `json:"is_published"`
}
type Event struct {
	ID          int32     `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Location    *string   `json:"location"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	AllDay      bool      `json:"all_day"`
	Activity    *Activity `json:"activity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type Service struct{ Queries *dbgen.Queries }

func Range(start, end string) (time.Time, time.Time, error) {
	a, errA := time.Parse(time.RFC3339, start)
	b, errB := time.Parse(time.RFC3339, end)
	if errA != nil || errB != nil || !b.After(a) || b.Sub(a) > 93*24*time.Hour {
		return a, b, domain.Fail(422, "INVALID_CALENDAR_RANGE")
	}
	return a, b, nil
}
func Validate(in *Input) error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 255 || in.StartsAt.IsZero() || in.EndsAt.IsZero() || !in.EndsAt.After(in.StartsAt) || in.StartsAt.Year() < 1 || in.EndsAt.Year() > 9999 {
		return domain.Fail(422, "INVALID_CALENDAR_EVENT")
	}
	for _, field := range []struct {
		value **string
		limit int
	}{{&in.Description, 10000}, {&in.Location, 500}} {
		if *field.value != nil {
			trimmed := strings.TrimSpace(**field.value)
			if utf8.RuneCountInString(trimmed) > field.limit {
				return domain.Fail(422, "INVALID_CALENDAR_EVENT")
			}
			if trimmed == "" {
				*field.value = nil
			} else {
				*field.value = &trimmed
			}
		}
	}
	if in.ActivityID != nil && *in.ActivityID <= 0 {
		return domain.Fail(422, "INVALID_CALENDAR_ACTIVITY")
	}
	if in.AllDay {
		for _, at := range []time.Time{in.StartsAt, in.EndsAt} {
			local := at.In(WIB)
			if local.Hour() != 0 || local.Minute() != 0 || local.Second() != 0 || local.Nanosecond() != 0 {
				return domain.Fail(422, "ALL_DAY_REQUIRES_WIB_MIDNIGHT")
			}
		}
	}
	return nil
}
func project(row dbgen.GetCalendarEventRow, public bool) Event {
	event := Event{ID: row.ID, Title: row.Title, Description: row.Description, Location: row.Location, StartsAt: row.StartsAt.Time.In(WIB), EndsAt: row.EndsAt.Time.In(WIB), AllDay: row.AllDay, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
	published := row.ActivityPublished != nil && *row.ActivityPublished
	if row.ActivityID != nil && row.ActivityName != nil && row.ActivitySlug != nil && (!public || published) {
		event.Activity = &Activity{ID: *row.ActivityID, Name: *row.ActivityName, Slug: *row.ActivitySlug, IsPublished: published}
	}
	return event
}
func (s Service) List(ctx context.Context, start, end time.Time, public bool) ([]Event, error) {
	rows, err := s.Queries.ListCalendarEvents(ctx, dbgen.ListCalendarEventsParams{RangeStart: stamp(start), RangeEnd: stamp(end)})
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, project(dbgen.GetCalendarEventRow(row), public))
	}
	return events, nil
}
func (s Service) Get(ctx context.Context, id int32) (Event, error) {
	row, err := s.Queries.GetCalendarEvent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, domain.Fail(404, "CALENDAR_EVENT_NOT_FOUND")
	}
	return project(row, false), err
}
func stamp(at time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: at, Valid: true} }
func inputError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return domain.Fail(422, "INVALID_CALENDAR_ACTIVITY")
	}
	return err
}
func (s Service) Save(ctx context.Context, id int32, in Input) (Event, error) {
	if err := Validate(&in); err != nil {
		return Event{}, err
	}
	if id == 0 {
		created, err := s.Queries.CreateCalendarEvent(ctx, dbgen.CreateCalendarEventParams{Title: in.Title, Description: in.Description, Location: in.Location, StartsAt: stamp(in.StartsAt), EndsAt: stamp(in.EndsAt), AllDay: in.AllDay, ActivityID: in.ActivityID})
		if err != nil {
			return Event{}, inputError(err)
		}
		id = created
	} else {
		count, err := s.Queries.UpdateCalendarEvent(ctx, dbgen.UpdateCalendarEventParams{ID: id, Title: in.Title, Description: in.Description, Location: in.Location, StartsAt: stamp(in.StartsAt), EndsAt: stamp(in.EndsAt), AllDay: in.AllDay, ActivityID: in.ActivityID})
		if err != nil {
			return Event{}, inputError(err)
		}
		if count == 0 {
			return Event{}, domain.Fail(404, "CALENDAR_EVENT_NOT_FOUND")
		}
	}
	return s.Get(ctx, id)
}
func (s Service) Delete(ctx context.Context, id int32) error {
	count, err := s.Queries.DeleteCalendarEvent(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.Fail(404, "CALENDAR_EVENT_NOT_FOUND")
	}
	return nil
}
