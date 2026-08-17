-include .env

### Execute on local machine
.PHONY: prep build build-local test test-race swag templ-gen templ templ-notify-proxy db-migrate db-migrate-down sqlc air

prep:
	@go get -tool github.com/a-h/templ/cmd/templ@latest
	@go get -tool github.com/air-verse/air@latest
	@go get -tool github.com/swaggo/swag/cmd/swag@latest
	@go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go get -tool github.com/pressly/goose/v3/cmd/goose@latest
	@npm install
	@cp .env.example .env
	@cp config.example.yaml config.yaml

build:
	@npm run build
	@CGO_ENABLED=0 go build -o ./bin/main ./cmd/server/
	@echo "\nBoth NPM and GO builds succeeded"

build-local:
	@go build -o ./bin/main ./cmd/server/

# templ/sqlc/swag output is the bulk of all statements — excluded from the total
COVER_TOTAL = grep -rl --include="*.go" "Code generated .* DO NOT EDIT" . | sed 's|^\./||' | \
	grep -vFf - coverage.out | go tool cover -func=/dev/stdin | tail -1

test:
	@go test ./... -cover -coverprofile=coverage.out
	@echo ""
	@$(COVER_TOTAL)
	@rm -f coverage.out

# Data race detector — needs cgo, so no CGO_ENABLED=0 here
test-race:
	@go test ./... -race -count=1

swag:
	@go tool swag init -g ./internal/embed.go -o ./internal/docs

templ-gen:
	@go tool templ generate

templ:
	@go tool templ generate --watch --proxy=http://localhost:$(SERVER_PORT) --proxyport=$(TEMPL_PROXY_PORT) --open-browser=false --proxybind="0.0.0.0"

templ-notify-proxy:
	@go tool templ generate --notify-proxy --proxyport=$(TEMPL_PROXY_PORT)

# The app runs migrations itself at startup;
db-migrate:
	@make docker-up-infra
	@sleep 1
	@go tool goose up

db-migrate-down:
	@make docker-up-infra
	@sleep 1
	@go tool goose down

sqlc:
	@go tool sqlc generate

air:
	@trap 'make docker-down; exit' INT TERM; \
	make docker-up-infra; \
	make templ & sleep 1; \
	go tool air; \
	make docker-down

### Execute using docker-compose
.PHONY: docker-build docker-up docker-up-infra docker-down

docker-build:
	@docker compose -f docker-compose.yml --profile whole build --no-cache

docker-up:
	@docker compose -f docker-compose.yml --profile whole up --no-recreate

# Postgres, Redis and Jaeger only — what `make air` needs behind a local binary
docker-up-infra:
	@docker compose -f docker-compose.yml up --remove-orphans --detach

docker-down:
	@docker compose -f docker-compose.yml --profile whole down
