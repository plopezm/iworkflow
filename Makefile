GOFLAGS ?= -p=2
export GOFLAGS

.PHONY: generate templ sqlc css build dev test lint fmt vet electron electron-build electron-dev electron-test e2e e2e-smoke

generate: templ sqlc css

# One generation per .templ file, run from the root: `-f` keeps the file names recorded in the generated code
# relative to the root, as they are committed. A root-level `templ generate` (or `-path <dir>`) records them
# relative to the directory it reads and rewrites the *_templ.go files of every module.
templ:
	@for f in $$(find . -name '*.templ' -not -path './.atl/*' -not -path './node_modules/*' | sed 's|^\./||' | sort); do templ generate -f $$f || exit 1; done

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

electron: electron-build

electron-build:
	./scripts/electron.sh build

electron-dev:
	./scripts/electron.sh dev

electron-test:
	npm --prefix electron test

e2e:
	./end2end/go/run_go.sh $(FILTER)

e2e-smoke:
	IWF_E2E_SCRIPTS="end2end/dock_smoke.cjs end2end/tab_state_smoke.cjs end2end/ui_smoke.cjs" ./end2end/go/run_go.sh


