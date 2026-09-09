package achievement

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"time"
)

type Service struct {
	Pool     *pgxpool.Pool
	Location *time.Location
}

func (s Service) Review(ctx context.Context, existing, payload database.Object, approver int32) (database.Object, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	change := database.Object{}
	change["status"] = payload["status"]
	change.Set("approver_id", approver)
	change.Set("approved_at", time.Now().In(s.Location).Format("2006-01-02"))
	if payload.Has("score") {
		change["score"] = payload["score"]
	}
	if payload.ID("status") == 2 && payload.String("remark") != "" {
		change["remark"] = payload["remark"]
	}
	row, err := q.Update(ctx, "achievements", existing.ID("id"), change)
	if err != nil {
		return nil, err
	}
	if payload.ID("status") == 1 {
		date, err := time.Parse("2006-01-02", row.String("achievement_date"))
		if err != nil {
			return nil, err
		}
		month := date.Format("2006-01") + "-01"
		for _, table := range []string{"monthly_leaderboards", "lifetime_leaderboards"} {
			query := "SELECT * FROM " + table + " WHERE user_id=$1"
			args := []interface{}{row.ID("user_id")}
			if table == "monthly_leaderboards" {
				query += " AND month=$2"
				args = append(args, month)
			}
			query += " LIMIT 1"
			board, err := q.One(ctx, query, args...)
			create := errors.Is(err, pgx.ErrNoRows)
			if err != nil && !create {
				return nil, err
			}
			data := database.Object{}
			if create {
				data.Set("user_id", row.ID("user_id"))
				if table == "monthly_leaderboards" {
					data.Set("month", month)
				}
			}
			for _, key := range []string{"score", "score_academic", "score_competition", "score_organizational"} {
				data.Set(key, board.Number(key))
			}
			switch row.ID("type") {
			case 0:
				data.Set("score_competition", board.Number("score_competition")+row.Number("score"))
			case 1:
				data.Set("score_organizational", board.Number("score_organizational")+row.Number("score"))
			case 2:
				data.Set("score_academic", board.Number("score_academic")+row.Number("score"))
			}
			data.Set("score", board.Number("score")+row.Number("score"))
			if create {
				_, err = q.Insert(ctx, table, data)
			} else {
				_, err = q.Update(ctx, table, board.ID("id"), data)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return row, tx.Commit(ctx)
}
