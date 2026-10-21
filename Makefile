BORROW_WORKSPACE ?= --borrow-workspace

.DEFAULT_GOAL := build

.PHONY: prepare-reference check build package run test-unit fixtures test-integration \
	test-browser test-courses test-courses-browser test-shared test-calendar \
	test-activity-courses test-feature-deletion test-short-links test-scoring \
	test-talent-assessment benchmark verify clean-fixtures

# Provides the Node dependencies (pg, jsonwebtoken, Adonis hashing) that the
# fixture harness and the session-transfer checks load. Not an HTTP contract.
prepare-reference:
	node scripts/prepare-reference.mjs

check:
	node scripts/check.mjs

build:
	mkdir -p bin
	go build -o bin/admin-api ./cmd/api
	go build -o bin/admin-jobs ./cmd/jobs

package:
	node scripts/package.mjs

run:
	node scripts/run.mjs api

test-unit:
	go test -race -count=1 ./...

fixtures:
	node scripts/ensure-fixtures.mjs

test-integration: fixtures
	node scripts/test-fixture-disconnect.mjs
	node scripts/source-club-workflows.mjs
	node scripts/test-go.mjs ./...

test-browser: fixtures
	node scripts/browser.mjs $(BORROW_WORKSPACE)
	node scripts/browser.mjs $(BORROW_WORKSPACE) --public

test-courses: fixtures
	node scripts/course-workflows.mjs $(BORROW_WORKSPACE)

test-courses-browser: fixtures
	node scripts/course-workflows.mjs $(BORROW_WORKSPACE) --browser

test-shared: fixtures
	node scripts/session-transfer.mjs $(BORROW_WORKSPACE)
	node scripts/shared-database.mjs $(BORROW_WORKSPACE)

test-calendar: fixtures
	node scripts/calendar.mjs --browser $(BORROW_WORKSPACE)

test-activity-courses: fixtures
	node scripts/activity-courses.mjs --browser $(BORROW_WORKSPACE)

test-feature-deletion: fixtures
	node scripts/feature-deletion.mjs --browser $(BORROW_WORKSPACE)

test-short-links: fixtures
	node scripts/short-links.mjs --browser $(BORROW_WORKSPACE)

test-scoring: fixtures
	node scripts/scoring-workflows.mjs --browser $(BORROW_WORKSPACE)

test-talent-assessment: fixtures
	node scripts/talent-assessment.mjs --browser $(BORROW_WORKSPACE)

benchmark: fixtures
	node scripts/performance.mjs $(BORROW_WORKSPACE)

verify:
	node scripts/verify.mjs $(BORROW_WORKSPACE)

clean-fixtures:
	node scripts/cleanup.mjs
