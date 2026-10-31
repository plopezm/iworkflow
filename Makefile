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
# Single source of truth for the release version: electron/package.json (set by `make bump-version`).
APP_VERSION ?= $(shell sed -n 's/^  "version": "\(.*\)",/\1/p' electron/package.json | head -n1)

# Audio transcription (sherpa-onnx, Qwen3-ASR). The cgo adapter sits behind the `sherpa` build tag; the default
# GO_TAGS enable it, `make build GO_TAGS=` gives a CGO-free binary where transcription reports "unavailable".
GO_TAGS ?= sherpa
GO_BUILD_TAGS := $(if $(strip $(GO_TAGS)),-tags $(GO_TAGS),)
ASR_MODEL_NAME ?= sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25
ASR_MODELS_DIR ?= $(HOME)/.iworkflow/models
comma := ,
# Where the packaged binary looks for the sherpa-onnx libraries: next to it (Windows needs no rpath).
SHERPA_RPATH := $(if $(filter darwin,$(shell go env GOOS)),-Wl$(comma)-rpath$(comma)@loader_path/lib,$(if $(filter linux,$(shell go env GOOS)),-Wl$(comma)-rpath$(comma)$$ORIGIN/lib,))

.PHONY: model generate templ sqlc css build dist run run-https prod dev dev-https test lint fmt vet bump-version electron electron-build electron-dev electron-test e2e e2e-smoke

generate: templ sqlc css

# Step 1 of the transcription feature: download the pre-converted Qwen3-ASR model (about 840 MB) once. The files are
# checked against the pinned sha256 sums in asr-model.sha256; a download that does not match is removed.
SHA256 := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

model:
	@test -f $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME)/decoder.int8.onnx || { \
		mkdir -p $(ASR_MODELS_DIR) && \
		curl -fL --retry 3 -o $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME).tar.bz2 https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/$(ASR_MODEL_NAME).tar.bz2 && \
		tar xjf $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME).tar.bz2 -C $(ASR_MODELS_DIR) && rm $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME).tar.bz2; }
	@cd $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME) && $(SHA256) -c $(CURDIR)/asr-model.sha256 >/dev/null || { \
		echo "The model in $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME) does not match asr-model.sha256; delete it and run 'make model' again." >&2; exit 1; }
	@echo "Model verified in $(ASR_MODELS_DIR)/$(ASR_MODEL_NAME)"

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
	go build $(GO_BUILD_TAGS) -o bin/$(BIN_FILE) ./cmd/iworkflow

dev: generate
	go run $(GO_BUILD_TAGS) ./cmd/iworkflow $(ARGS)

# Same as dev over HTTPS, which browsers need to expose the microphone of the voice dictation to other devices. The server
# creates a self-signed certificate in ~/.iworkflow/tls on first use and reuses it; browsers ask to accept it once.
dev-https: generate
	IWF_TLS=auto go run $(GO_BUILD_TAGS) ./cmd/iworkflow $(ARGS)

# Release package: stripped binary plus the static site, in $(DIST_DIR).
dist: generate
	@test -n "$(APP_VERSION)" || { echo "Cannot read the version from electron/package.json; pass APP_VERSION=X.Y.Z" >&2; exit 1; }
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	CGO_LDFLAGS='$(SHERPA_RPATH)' go build $(GO_BUILD_TAGS) -ldflags="-s -w -X main.version=$(APP_VERSION)" -o $(DIST_DIR)/$(BIN_FILE) ./cmd/iworkflow
	cp -r web/static $(DIST_DIR)/site
	@$(if $(findstring sherpa,$(GO_TAGS)),./scripts/copy_sherpa_libs.sh $(DIST_DIR),true)
	@echo "Packaged into $(DIST_DIR)"

# Start what `make dist` packaged. Pass flags with ARGS="--no-open"; listen address with SITE_ADDR.
run:
	@test -x $(DIST_DIR)/$(BIN_FILE) && test -d $(DIST_DIR)/site || { echo "No complete build in $(DIST_DIR); run 'make dist' first." >&2; exit 1; }
	@echo "Serving on http://$(SITE_ADDR)"
	$(DIST_DIR)/$(BIN_FILE) --addr $(SITE_ADDR) --site-root $(abspath $(DIST_DIR))/site $(ARGS)

# Same as run over HTTPS (see dev-https).
run-https:
	@test -x $(DIST_DIR)/$(BIN_FILE) && test -d $(DIST_DIR)/site || { echo "No complete build in $(DIST_DIR); run 'make dist' first." >&2; exit 1; }
	@echo "Serving on http://$(SITE_ADDR) and HTTPS on the port +443"
	IWF_TLS=auto $(DIST_DIR)/$(BIN_FILE) --addr $(SITE_ADDR) --site-root $(abspath $(DIST_DIR))/site $(ARGS)

prod: dist run

test:
	go test -p 2 ./...
	./scripts/copy_sherpa_libs_test.sh

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
