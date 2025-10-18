-include .env

### Execute on local machine
.PHONY: prep build-local build templ notify-templ-proxy air

prep:
	@go get -tool github.com/a-h/templ/cmd/templ@latest
	@go get -tool github.com/air-verse/air@latest
	@npm install
	@cp .env.example .env

build-local:
	@go build -o ./bin/main cmd/server/main.go

build:
	@npm run build
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/main cmd/server/main.go

templ:
	@go tool templ generate --watch --proxy=http://localhost:$(APP_PORT) --proxyport=$(TEMPL_PROXY_PORT) --open-browser=false --proxybind="0.0.0.0"

notify-templ-proxy:
	@go tool templ generate --notify-proxy --proxyport=$(TEMPL_PROXY_PORT)

air:
	@make templ & sleep 1
	@go tool air

### Execute using docker-compose
.PHONY: docker-build docker-up docker-down

docker-build:
	@docker-compose -f docker-compose.yml build --no-cache

docker-up:
	@docker-compose -f docker-compose.yml up --no-recreate

docker-down:
	@docker-compose -f docker-compose.yml down
