package achievement

import (
	"context"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

func (s Service) List(ctx context.Context, filters Filters) (Page[Detail], error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountAchievements(ctx, dbgen.CountAchievementsParams{Status: filters.Status, Email: filters.Email, Name: filters.Name, Kind: filters.Type})
	result := Page[Detail]{Meta: database.Meta(total, filters.Page, filters.Size), Data: []Detail{}}
	if err != nil {
		return result, database.LegacyQueryError(err, achievementQuery(filters, true))
	}
	if total == 0 {
		return result, nil
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListAchievements(ctx, dbgen.ListAchievementsParams{Status: filters.Status, Email: filters.Email, Name: filters.Name, Kind: filters.Type, DateOrder: filters.DateOrder, Ascending: filters.Ascending, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, database.LegacyQueryError(err, achievementQuery(filters, false))
	}
	for _, row := range rows {
		detail, err := s.details(row.Achievement, row.PublicUser, row.Profile, row.Approver, true)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, detail)
	}
	return result, nil
}
func (s Service) Show(ctx context.Context, id string) (Detail, error) {
	record, err := s.Existing(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	row, err := dbgen.New(s.Pool).AchievementDetails(ctx, record.ID)
	if err != nil {
		return Detail{}, err
	}
	return s.details(row.Achievement, row.PublicUser, row.Profile, row.Approver, false)
}
func (s Service) Monthly(ctx context.Context, filters LeaderboardFilters) (Page[MonthlyResponse], error) {
	result := Page[MonthlyResponse]{Data: []MonthlyResponse{}}
	params, err := monthFilters(filters)
	if err != nil {
		return result, err
	}
	q := dbgen.New(s.Pool)
	total, err := q.CountMonthlyLeaderboard(ctx, params)
	result.Meta = database.Meta(total, filters.Page, filters.Size)
	if err != nil {
		return result, database.LegacyQueryError(err, leaderboardQuery(filters, &params, true))
	}
	if total == 0 {
		return result, nil
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListMonthlyLeaderboard(ctx, dbgen.ListMonthlyLeaderboardParams{Email: params.Email, Name: params.Name, FilterMonth: params.FilterMonth, Month: params.Month, FilterYear: params.FilterYear, StartDate: params.StartDate, EndDate: params.EndDate, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, database.LegacyQueryError(err, leaderboardQuery(filters, &params, false))
	}
	for _, row := range rows {
		user, err := leaderboardUser(row.PublicUser, row.Profile, row.University)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, MonthlyResponse{MonthlyLeaderboard: row.MonthlyLeaderboard, CreatedAt: domain.ModelTimestamp(row.MonthlyLeaderboard.CreatedAt, s.Location), UpdatedAt: domain.ModelTimestamp(row.MonthlyLeaderboard.UpdatedAt, s.Location), User: user})
	}
	return result, nil
}
func (s Service) Lifetime(ctx context.Context, filters LeaderboardFilters) (Page[LifetimeResponse], error) {
	result := Page[LifetimeResponse]{Data: []LifetimeResponse{}}
	q := dbgen.New(s.Pool)
	total, err := q.CountLifetimeLeaderboard(ctx, dbgen.CountLifetimeLeaderboardParams{Email: filters.Email, Name: filters.Name})
	result.Meta = database.Meta(total, filters.Page, filters.Size)
	if err != nil {
		return result, database.LegacyQueryError(err, leaderboardQuery(filters, nil, true))
	}
	if total == 0 {
		return result, nil
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListLifetimeLeaderboard(ctx, dbgen.ListLifetimeLeaderboardParams{Email: filters.Email, Name: filters.Name, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, database.LegacyQueryError(err, leaderboardQuery(filters, nil, false))
	}
	for _, row := range rows {
		user, err := leaderboardUser(row.PublicUser, row.Profile, row.University)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, LifetimeResponse{LifetimeLeaderboard: row.LifetimeLeaderboard, CreatedAt: domain.ModelTimestamp(row.LifetimeLeaderboard.CreatedAt, s.Location), UpdatedAt: domain.ModelTimestamp(row.LifetimeLeaderboard.UpdatedAt, s.Location), User: user})
	}
	return result, nil
}
