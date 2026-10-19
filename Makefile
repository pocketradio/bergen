.PHONY: test unit lint up down migrate

unit:
	go test $(GOFLAGS) ./...

test:
	BERGEN_TEST_DATABASE_URL='postgres://bergen:bergen_local@localhost:5432/bergen?sslmode=disable' BERGEN_TEST_S3_ENDPOINT=localhost:4566 BERGEN_TEST_S3_ACCESS_KEY=test BERGEN_TEST_S3_SECRET_KEY=test go test $(GOFLAGS) ./...

lint:
	go vet ./...
	golangci-lint run ./...

up:
	docker compose pull
	docker compose up -d --wait

down:
	docker compose down

migrate:
	go run ./cmd/server -migrate
