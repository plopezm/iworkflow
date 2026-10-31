GOFLAGS ?= -p=2
export GOFLAGS

# Release package (binary + site/) and run settings. Override on the command line, e.g. `make dist DIST_DIR=/opt/iworkflow`.
DIST_DIR ?= dist
GOEXE := $(shell go env GOEXE)
BIN_FILE := iworkflow$(GOEXE)
ELECTRON_PLATFORM := $(strip $(if $(filter linux,$(shell go env GOOS)),--linux,$(if $(filter darwin,$(shell go env GOOS)),--mac,--win)))
SITE_ADDR ?= 0.0.0.0:9000
ARGS ?=
# SKIP_SERVER_BUILD=1 makes electron-build reuse the server already in $(DIST_DIR).
SKIP_SERVER_BUILD ?=

.PHONY: generate templ sqlc css build dist run prod dev test lint fmt vet bump-version electron electron-build electron-dev electron-test e2e e2e-smoke

generate: templ sqlc css

# One generation per .templ file, run from the root: `-f` keeps the file names recorded in the generated code
# relative to the root, as they are committed. A root-level `templ generate` (or `-path <dir>`) records them
# relative to the directory it reads and rewrites the *_templ.go files of every module.
templ:
	@for f in $$(find . \( -name .git -o -name .atl -o -name node_modules -o -name dist \) -prune -o -name '*.templ' -print | sed 's|^\./||' | sort); do templ generate -f $$f || exit 1; done

# Each module owns its sqlc.yaml (internal/modules/<x>/infrastructure/adapters/outbound/persistence/sqlc.yaml).
sqlc:
	@for f in $$(find internal -name sqlc.yaml); do (cd $$(dirname $$f) && sqlc generate) || exit 1; done

css:
	tailwindcss -i web/tailwind.css -o web/static/app.css --minify

build: generate
	go build -o bin/$(BIN_FILE) ./cmd/iworkflow

dev: generate
	go run ./cmd/iworkflow $(ARGS)

# Release package: stripped binary plus the static site, in $(DIST_DIR).
dist: generate
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	go build -ldflags="-s -w" -o $(DIST_DIR)/$(BIN_FILE) ./cmd/iworkflow
	cp -r web/static $(DIST_DIR)/site
	@echo "Packaged into $(DIST_DIR)"

# Start what `make dist` packaged. Pass flags with ARGS="--no-open"; listen address with SITE_ADDR.
run:
	@test -x $(DIST_DIR)/$(BIN_FILE) && test -d $(DIST_DIR)/site || { echo "No complete build in $(DIST_DIR); run 'make dist' first." >&2; exit 1; }
	@echo "Serving on http://$(SITE_ADDR)"
	$(DIST_DIR)/$(BIN_FILE) --addr $(SITE_ADDR) --site-root $(abspath $(DIST_DIR))/site $(ARGS)

prod: dist run

test:
	go test -p 2 ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

lint:
	golangci-lint run

electron: electron-build

# Installer for the current platform, written to $(DIST_DIR)/electron.
electron-build: $(if $(SKIP_SERVER_BUILD),,dist)
	@test -f $(DIST_DIR)/$(BIN_FILE) && test -d $(DIST_DIR)/site || { echo "No server package in $(DIST_DIR); run 'make dist' first." >&2; exit 1; }
	rm -rf electron/resources/server
	mkdir -p electron/resources/server
	cp $(DIST_DIR)/$(BIN_FILE) electron/resources/server/
	cp -r $(DIST_DIR)/site electron/resources/server/site
	cd electron && npm ci && npx electron-builder $(ELECTRON_PLATFORM) --publish never --config.directories.output="$(abspath $(DIST_DIR))/electron"
	@echo "Installer written to $(DIST_DIR)/electron"

# Electron window over the development binary (bin/) staged into $(DIST_DIR).
electron-dev: build
	mkdir -p $(DIST_DIR)/site
	cp bin/$(BIN_FILE) $(DIST_DIR)/$(BIN_FILE)
	cp -r web/static/* $(DIST_DIR)/site/
	cd electron && npm ci && IWF_SERVER_DIR="$(abspath $(DIST_DIR))" npx electron .

electron-test:
	npm --prefix electron test

e2e:
	./end2end/go/run_go.sh $(FILTER)

e2e-smoke:
	IWF_E2E_SCRIPTS="end2end/dock_smoke.cjs end2end/tab_state_smoke.cjs end2end/ui_smoke.cjs" ./end2end/go/run_go.sh

# Set the app version in electron/package.json and its lock file: `make bump-version VERSION=0.18.0`.
bump-version:
	@test -n "$(VERSION)" || { echo "Usage: make bump-version VERSION=MAJOR.MINOR.PATCH" >&2; exit 1; }
	@echo "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$$' || { echo "Invalid version '$(VERSION)' (expected MAJOR.MINOR.PATCH)" >&2; exit 1; }
	cd electron && npm version $(VERSION) --no-git-tag-version --allow-same-version
	git diff --stat -- electron/package.json electron/package-lock.json
