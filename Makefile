.PHONY: check build package run test-unit fixtures test-integration test-contract test-browser test-shared test-jobs benchmark verify clean-fixtures

BORROW_WORKSPACE ?= --borrow-workspace

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

test-contract: fixtures
	node scripts/contracts-all.mjs $(BORROW_WORKSPACE)

test-browser: fixtures
	node scripts/browser.mjs $(BORROW_WORKSPACE)
	node scripts/browser.mjs $(BORROW_WORKSPACE) --public

test-shared: fixtures
	node scripts/session-transfer.mjs $(BORROW_WORKSPACE)
	node scripts/shared-database.mjs $(BORROW_WORKSPACE)

test-jobs: fixtures
	node scripts/contract-jobs.mjs

benchmark: fixtures
	node scripts/performance.mjs $(BORROW_WORKSPACE)

verify:
	node scripts/verify.mjs $(BORROW_WORKSPACE)

clean-fixtures:
	node scripts/cleanup.mjs
