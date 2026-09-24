.PHONY: up down dev-db run test

up:
	docker compose up --build

down:
	docker compose down

# Starts only postgres, for local backend development against a real db.
dev-db:
	docker compose up -d db

run:
	cd backend && go run ./cmd/server

test:
	cd backend && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...
