GOFLAGS ?= -p=2
export GOFLAGS

.PHONY: generate templ sqlc css build dev test lint fmt vet

generate: templ sqlc css

templ:
	templ generate

# Each module owns its sqlc.yaml (internal/modules/<x>/infrastructure/adapters/outbound/persistence/sqlc.yaml).
sqlc:
	@for f in $$(find internal -name sqlc.yaml); do (cd $$(dirname $$f) && sqlc generate) || exit 1; done

css:
	tailwindcss -i web/tailwind.css -o web/static/app.css --minify

build: generate
	go build -o bin/iworkflow ./cmd/iworkflow

dev: generate
	go run ./cmd/iworkflow

test:
	go test -p 2 ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

lint:
	golangci-lint run
