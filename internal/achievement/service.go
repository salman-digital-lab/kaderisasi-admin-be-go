package achievement

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Pool     *pgxpool.Pool
	Location *time.Location
}

func (s Service) Existing(ctx context.Context, id string) (dbgen.Achievement, error) {
	row, err := dbgen.New(s.Pool).AchievementByIdentifier(ctx, id)
	return row, database.LegacyQueryError(err, `select * from "achievements" where "id" = $1 limit $2`)
}
func number(value *int32) *string {
	if value == nil {
		return nil
	}
	text := strconv.FormatInt(int64(*value), 10)
	return &text
}
func inputNumber(value *json.Number) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
func approved(status *int32) bool { return status != nil && *status == 1 }
func approvalDate(location *time.Location) pgtype.Date {
	now := time.Now().In(locationOrLocal(location))
	return pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}
func paramsFor(row dbgen.Achievement) dbgen.UpdateAchievementParams {
	return dbgen.UpdateAchievementParams{ID: row.ID, Name: row.Name, Description: row.Description, Type: number(row.Type), Score: number(row.Score), Proof: row.Proof, Status: number(row.Status), Remark: row.Remark, ApproverID: number(row.ApproverID), ApprovedAt: row.ApprovedAt}
}
func (s Service) Update(ctx context.Context, row dbgen.Achievement, input Input, actor int32) (Response, error) {
	params := paramsFor(row)
	if input.Name != nil {
		params.Name = input.Name
	}
	if input.Description != nil {
		params.Description = input.Description
	}
	if input.Type != nil {
		params.Type = inputNumber(input.Type)
	}
	if input.Score != nil {
		params.Score = inputNumber(input.Score)
	}
	if input.Proof != nil {
		params.Proof = input.Proof
	}
	if input.Status != nil {
		params.Status = inputNumber(input.Status)
	}
	if input.Remark != nil {
		params.Remark = input.Remark
	}
	if input.Status != nil && input.Status.String() == "1" && !approved(row.Status) {
		params.ApproverID = number(&actor)
		params.ApprovedAt = approvalDate(s.Location)
	}
	updated, err := dbgen.New(s.Pool).UpdateAchievement(ctx, params)
	return s.view(updated), database.LegacyQueryError(err, updateStatement(row, params))
}
func (s Service) Review(ctx context.Context, row dbgen.Achievement, input Input, actor int32) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	params := paramsFor(row)
	params.Status = inputNumber(input.Status)
	params.ApproverID = number(&actor)
	params.ApprovedAt = approvalDate(s.Location)
	params.Touch = true
	if input.Score != nil {
		params.Score = inputNumber(input.Score)
	}
	if input.Status.String() == "2" && input.Remark != nil && *input.Remark != "" {
		params.Remark = input.Remark
	}
	updated, err := q.UpdateAchievement(ctx, params)
	if err != nil {
		return Response{}, database.LegacyQueryError(err, updateStatement(row, params))
	}
	if approved(updated.Status) {
		if !updated.AchievementDate.Valid {
			return Response{}, errors.New("Cannot read properties of null (reading 'startOf')")
		}
		date := updated.AchievementDate.Time
		month := pgtype.Date{Time: time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC), Valid: true}
		if err = updateMonthly(ctx, q, updated, month); err != nil {
			return Response{}, err
		}
		if err = updateLifetime(ctx, q, updated); err != nil {
			return Response{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Response{}, err
	}
	return s.view(updated), nil
}
func updateStatement(row dbgen.Achievement, params dbgen.UpdateAchievementParams) string {
	fields := []struct {
		name          string
		before, after interface{}
	}{
		{"name", row.Name, params.Name}, {"description", row.Description, params.Description}, {"type", number(row.Type), params.Type}, {"score", number(row.Score), params.Score}, {"proof", row.Proof, params.Proof}, {"status", number(row.Status), params.Status}, {"remark", row.Remark, params.Remark}, {"approver_id", number(row.ApproverID), params.ApproverID}, {"approved_at", row.ApprovedAt, params.ApprovedAt},
	}
	names := []string{}
	for _, field := range fields {
		if !reflect.DeepEqual(field.before, field.after) || field.name == "approved_at" && params.Touch {
			names = append(names, field.name)
		}
	}
	return dirtyUpdate("achievements", append(names, "updated_at"))
}
func dirtyUpdate(table string, names []string) string {
	setters := []string{}
	for i, name := range names {
		setters = append(setters, `"`+name+`" = $`+strconv.Itoa(i+1))
	}
	return `update "` + table + `" set ` + strings.Join(setters, ", ") + ` where "id" = $` + strconv.Itoa(len(names)+1)
}

// Adonis reads each leaderboard, mutates only the selected category and total,
// then saves it. Preserve nullable untouched categories and its repeated-review
// behavior rather than replacing the workflow with an upsert or increment.
type scores struct{ Academic, Competition, Organizational, Total *string }

func addScore(current, amount *int32) *string {
	total := int64(0)
	if current != nil {
		total += int64(*current)
	}
	if amount != nil {
		total += int64(*amount)
	}
	text := strconv.FormatInt(total, 10)
	return &text
}
func accumulated(academic, competition, organizational, total *int32, row dbgen.Achievement, create bool) scores {
	if create {
		academic, competition, organizational, total = new(int32), new(int32), new(int32), new(int32)
	}
	result := scores{number(academic), number(competition), number(organizational), addScore(total, row.Score)}
	if row.Type != nil {
		switch *row.Type {
		case 0:
			result.Competition = addScore(competition, row.Score)
		case 1:
			result.Organizational = addScore(organizational, row.Score)
		case 2:
			result.Academic = addScore(academic, row.Score)
		}
	}
	return result
}
func updateMonthly(ctx context.Context, q *dbgen.Queries, achievement dbgen.Achievement, month pgtype.Date) error {
	board, err := q.FindMonthlyLeaderboard(ctx, dbgen.FindMonthlyLeaderboardParams{UserID: achievement.UserID, Month: month})
	create := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !create {
		return err
	}
	value := accumulated(board.ScoreAcademic, board.ScoreCompetition, board.ScoreOrganizational, board.Score, achievement, create)
	if create {
		_, err = q.CreateMonthlyLeaderboard(ctx, dbgen.CreateMonthlyLeaderboardParams{UserID: achievement.UserID, Month: month, Score: value.Total, ScoreAcademic: value.Academic, ScoreCompetition: value.Competition, ScoreOrganizational: value.Organizational})
	} else {
		_, err = q.UpdateMonthlyLeaderboard(ctx, dbgen.UpdateMonthlyLeaderboardParams{ID: board.ID, Score: value.Total, ScoreAcademic: value.Academic, ScoreCompetition: value.Competition, ScoreOrganizational: value.Organizational})
	}
	return database.LegacyQueryError(err, boardStatement("monthly_leaderboards", create, board.ScoreAcademic, board.ScoreCompetition, board.ScoreOrganizational, board.Score, value))
}
func updateLifetime(ctx context.Context, q *dbgen.Queries, achievement dbgen.Achievement) error {
	board, err := q.FindLifetimeLeaderboard(ctx, achievement.UserID)
	create := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !create {
		return err
	}
	value := accumulated(board.ScoreAcademic, board.ScoreCompetition, board.ScoreOrganizational, board.Score, achievement, create)
	if create {
		_, err = q.CreateLifetimeLeaderboard(ctx, dbgen.CreateLifetimeLeaderboardParams{UserID: achievement.UserID, Score: value.Total, ScoreAcademic: value.Academic, ScoreCompetition: value.Competition, ScoreOrganizational: value.Organizational})
	} else {
		_, err = q.UpdateLifetimeLeaderboard(ctx, dbgen.UpdateLifetimeLeaderboardParams{ID: board.ID, Score: value.Total, ScoreAcademic: value.Academic, ScoreCompetition: value.Competition, ScoreOrganizational: value.Organizational})
	}
	return database.LegacyQueryError(err, boardStatement("lifetime_leaderboards", create, board.ScoreAcademic, board.ScoreCompetition, board.ScoreOrganizational, board.Score, value))
}
func boardStatement(table string, create bool, academic, competition, organizational, total *int32, value scores) string {
	if create {
		columns := []string{"created_at"}
		if table == "monthly_leaderboards" {
			columns = append(columns, "month")
		}
		columns = append(columns, "score", "score_academic", "score_competition", "score_organizational", "updated_at", "user_id")
		quoted, params := []string{}, []string{}
		for i, column := range columns {
			quoted = append(quoted, `"`+column+`"`)
			params = append(params, "$"+strconv.Itoa(i+1))
		}
		return `insert into "` + table + `" (` + strings.Join(quoted, ", ") + `) values (` + strings.Join(params, ", ") + `) returning "id"`
	}
	columns := []string{}
	for _, field := range []struct {
		name   string
		before *int32
		after  *string
	}{{"score_academic", academic, value.Academic}, {"score_competition", competition, value.Competition}, {"score_organizational", organizational, value.Organizational}, {"score", total, value.Total}} {
		if !reflect.DeepEqual(number(field.before), field.after) {
			columns = append(columns, field.name)
		}
	}
	return dirtyUpdate(table, append(columns, "updated_at"))
}
