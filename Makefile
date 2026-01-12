.PHONY: all build test dev ui-install ui-dev ui-build example clean

# Build the Go library
build:
	go build ./...

# Run tests
test:
	go test -v ./...

# Run integration tests (requires Docker)
test-integration:
	go test -v -tags=integration ./...

# Install UI dependencies
ui-install:
	cd ui && npm install

# Run UI dev server
ui-dev:
	cd ui && npm run dev

# Build UI for production
ui-build:
	cd ui && npm run build

# Build fullstack example with embedded UI
example-build: ui-build
	mkdir -p _examples/fullstack/ui
	cp -r ui/dist _examples/fullstack/ui/
	cd _examples/fullstack && go build -o ../../bin/rivo-server .

# Run basic example (requires Postgres)
example-basic:
	go run ./_examples/basic

# Run fullstack example (requires Postgres)
example-fullstack:
	go run ./_examples/fullstack

# Development: run API and UI in parallel
dev:
	@echo "Starting development servers..."
	@echo "API will run on :8080"
	@echo "UI will run on :3000 (proxying API)"
	@echo ""
	@echo "Run in separate terminals:"
	@echo "  make example-fullstack"
	@echo "  make ui-dev"

# Clean build artifacts
clean:
	rm -rf bin/ ui/dist ui/node_modules _examples/fullstack/ui

# Database: start postgres with docker
db-start:
	docker run -d --name rivo-postgres \
		-e POSTGRES_USER=rivo \
		-e POSTGRES_PASSWORD=rivo \
		-e POSTGRES_DB=rivo \
		-p 5432:5432 \
		postgres:16-alpine

# Database: stop postgres
db-stop:
	docker stop rivo-postgres && docker rm rivo-postgres

# Generate migration
migrate-new:
	@read -p "Migration name: " name; \
	touch internal/migration/migrations/$$(printf "%03d" $$(($$(ls internal/migration/migrations/*.sql 2>/dev/null | wc -l) + 1)))_$$name.sql

# Help
help:
	@echo "Rivo - Postgres-native workflow and job execution platform"
	@echo ""
	@echo "Usage:"
	@echo "  make build            Build Go library"
	@echo "  make test             Run unit tests"
	@echo "  make test-integration Run integration tests (requires Docker)"
	@echo ""
	@echo "  make ui-install       Install UI dependencies"
	@echo "  make ui-dev           Run UI dev server"
	@echo "  make ui-build         Build UI for production"
	@echo ""
	@echo "  make example-basic    Run basic example"
	@echo "  make example-fullstack Run fullstack example"
	@echo "  make example-build    Build fullstack with embedded UI"
	@echo ""
	@echo "  make db-start         Start Postgres with Docker"
	@echo "  make db-stop          Stop Postgres"
	@echo ""
	@echo "  make clean            Clean build artifacts"
