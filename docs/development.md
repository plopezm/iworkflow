# Development

> 🇪🇸 [Versión en castellano](es/development.md) · [← README](../README.md)

Architecture and code guide in [ARCHITECTURE.md](../ARCHITECTURE.md); project rules in [AGENTS.md](../AGENTS.md). Run the server and the checks from the repository root:

```bash
make generate                                  # templ + sqlc + tailwind (after touching .templ, SQL or CSS)
make dev                                       # server with reload on https://127.0.0.1:3000
make test                                      # go test -p 2 ./... plus the packaging script tests
make vet                                       # go vet ./...
make lint                                      # golangci-lint run
make e2e                                       # full browser Playwright e2e suite (or FILTER=<name>)
make e2e-smoke                                 # quick UI smoke scripts only
make model                                     # download the voice dictation model
make dist                                      # release package (dist/)
make electron-build                            # package desktop app (dist/electron)
make electron-dev                              # run the Electron window in development
make electron-test                             # launcher unit tests (npm test in electron/)
make bump-version VERSION=x.y.z                # set the app version
```

Use `go test -p 2 -race ./...` before a release for the race detector, and one module at a time while iterating (`go test -p 2 ./internal/modules/<feature>/...`).

## Recording the documentation GIFs

The GIFs of the README are produced from the real app, against a throwaway data folder and the fake agents of the e2e suite, so they need no LLM and are reproducible. See [media/README.md](media/README.md).
