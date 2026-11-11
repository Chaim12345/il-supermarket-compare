.PHONY: build run ingest test web dev docker clean

build: web/build
	go build -o bin/server ./cmd/server
	go build -o bin/ingest ./cmd/ingest

web/build:
	cd web && npm ci && npm run build

run: build
	./bin/server --db data/prices.db --static web/dist

ingest:
	go run ./cmd/ingest --db data/prices.db

test:
	go test ./...
	cd web && npx tsc --noEmit

dev:
	cd web && npm run dev

docker:
	docker compose up --build

clean:
	rm -rf bin web/dist data/prices.db
