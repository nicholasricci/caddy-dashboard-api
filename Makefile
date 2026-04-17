.PHONY: run migrate build test lint fmt vuln-install vuln swagger swag-install mcp-build mcp-run docker-dev-build docker-dev-up docker-dev-down docker-dev-logs

run:
	go run ./cmd/server

migrate:
	go run ./cmd/migrate

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

fmt:
	go fmt ./...

vuln-install:
	go install golang.org/x/vuln/cmd/govulncheck@latest

vuln:
	$$(go env GOPATH)/bin/govulncheck ./...

swag-install:
	go install github.com/swaggo/swag/cmd/swag@v1.8.12

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.8.12 init -g cmd/server/main.go -o docs

mcp-build:
	cd tools/mcp-server && npm ci && npm run build

# Avvio locale MCP (stdio): richiede build; imposta il gate dev come da design.
mcp-run:
	cd tools/mcp-server && CADDY_MCP_DEV=1 npm run start

docker-dev-build:
	docker build -f docker/dev/Dockerfile -t caddy-dashboard-api:dev .

docker-dev-up:
	docker compose up --build

docker-dev-down:
	docker compose down -v

docker-dev-logs:
	docker compose logs -f api
