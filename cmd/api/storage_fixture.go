//go:build !integration

package main

import (
	"kaderisasi/admin/internal/config"
	"regexp"
)

func storageFixtureAllowed(c config.Config) bool {
	return c.Environment == "test" && regexp.MustCompile(`^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$`).MatchString(c.DBSchema)
}
