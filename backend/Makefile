MODULE       := github.com/abdullah-zubair/jobqueue
BIN_DIR      := bin
GOFLAGS      := -trimpath
LDFLAGS      := -s -w
BINARIES     := apiserver worker dispatcher

DB_URL       ?= postgres://jobqueue:jobqueue@localhost:5432/jobqueue?sslmode=disable
MIGRATIONS   := migrations

.PHONY: build $(BINARIES) run-% test test-integration test-race lint fmt tidy \
        migrate-up migrate-down migrate-new dev-up dev-down clean frontend-build

build: $(BINARIES)

# Builds the React dashboard straight into internal/web/dist (see
# frontend/vite.config.ts's outDir), so a subsequent `make build` embeds
# the real app instead of the placeholder page checked into git.
frontend-build:
	cd ../frontend && npm ci && npm run build

$(BINARIES):
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$@ ./cmd/$@

run-%: %
	./$(BIN_DIR)/$*

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .
	goimports -w .

tidy:
	go mod tidy

migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" down 1

migrate-new:
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

dev-up:
	docker compose up -d

dev-down:
	docker compose down

clean:
	rm -rf $(BIN_DIR)
