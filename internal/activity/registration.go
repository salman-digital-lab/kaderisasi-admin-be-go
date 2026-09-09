package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/member"
	"slices"
)

// Upgrade preserves the legacy distinction: explicit status changes upgrade
// levels and badges; the broad bulk endpoint only changes registration status.
func Upgrade(ctx context.Context, tx pgx.Tx, activity database.Object, users []int32, status string) error {
	kind := activity.ID("activity_type")
	if status != "LULUS KEGIATAN" || kind < 2 || kind > 5 {
		return nil
	}
	level, exists := map[int32]int32{2: 3, 3: 6, 5: 10}[kind]
	if !exists {
		return fmt.Errorf("Empty .update() call detected! Update data does not contain any values to update. This will result in a faulty query. Table: profiles. Columns: level.")
	}
	if _, err := tx.Exec(ctx, "UPDATE profiles SET level=$1 WHERE user_id=ANY($2::int[])", level, users); err != nil {
		return err
	}
	if badge := activity.String("badge"); badge != "" {
		q := database.JSONQueries{DB: tx}
		profiles, err := q.All(ctx, "SELECT * FROM profiles WHERE user_id=ANY($1::int[])", users)
		if err != nil {
			return err
		}
		for _, p := range profiles {
			p = member.Profile(p)
			var badges []string
			_ = json.Unmarshal(p["badges"], &badges)
			if !slices.Contains(badges, badge) {
				badges = append(badges, badge)
				change := database.Object{}
				change.Set("badges", badges)
				if _, err = q.Update(ctx, "profiles", p.ID("id"), change); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
