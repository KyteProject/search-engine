.PHONY: test db-migrations-up db-migrations-down

test:
	@echo "[go test] running tests and collecting coverage metrics"
	@go test -v -tags all_tests -race -coverprofile=coverage.txt -covermode=atomic ./...

db-migrations-up:
	migrate -database ${CDB_MIGRATE} -path linkgraph/store/cockroachdb/migrations up

db-migrations-down:
	migrate -database ${CDB_MIGRATE} -path linkgraph/store/cockroachdb/migrations down