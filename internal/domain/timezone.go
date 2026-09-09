package domain

import (
	"time"
	_ "time/tzdata"
)

var jakarta = func() *time.Location {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return location
}()

// Jakarta is the explicit business timezone for club exports and deadlines,
// independent of the API/job process's configured local timezone.
func Jakarta() *time.Location { return jakarta }
