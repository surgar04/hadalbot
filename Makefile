.PHONY: run fmt check test build migrate-status migrate-up
run:
	go run ./cmd/hadalbot -config configs/development.yaml
fmt:
	go run ./cmd/devtools fmt
check:
	go run ./cmd/devtools check
test:
	go test -count=1 ./...
build:
	go run ./cmd/devtools build
migrate-status:
	go run ./cmd/migrate -command status
migrate-up:
	go run ./cmd/migrate -command up
