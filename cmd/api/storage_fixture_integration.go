//go:build integration

package main

import (
	"kaderisasi/admin/internal/config"
	"os"
	"regexp"
)

// Production HTTP error contracts must use the production error envelope while
// retaining storage ownership instrumentation in the isolated test database.
// This exception is excluded from release binaries.
func storageFixtureAllowed(c config.Config) bool {
	fixture := regexp.MustCompile(`^go_rewrite_[a-f0-9]{16}_(baseline|candidate|cross)$`).MatchString(c.DBSchema)
	return fixture && (c.Environment == "test" || (c.Environment == "production" && os.Getenv("GO_REWRITE_PRODUCTION_CONTRACT") == "1"))
}
