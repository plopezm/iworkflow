# Desarrollo

> 🇬🇧 [English version](../development.md) · [← README](../../README.es.md)

La guía de arquitectura y de código está en [ARCHITECTURE.es.md](../../ARCHITECTURE.es.md); las reglas del proyecto, en [AGENTS.md](../../AGENTS.md). Ejecuta el servidor y las comprobaciones desde la raíz del repositorio:

```bash
make generate                                  # templ + sqlc + tailwind (tras tocar .templ, SQL o CSS)
make dev                                       # servidor con recarga en http://127.0.0.1:3000
make test                                      # go test -p 2 ./... más los tests de los scripts de empaquetado
make vet                                       # go vet ./...
make lint                                      # golangci-lint run
make e2e                                       # suite completa de navegador con Playwright (o FILTER=<name>)
make e2e-smoke                                 # solo los scripts rápidos de humo de la UI
make model                                     # descarga el modelo del dictado por voz
make dist                                      # paquete de release (dist/)
make electron-build                            # empaqueta la aplicación de escritorio (dist/electron)
make electron-dev                              # ejecuta la ventana de Electron en desarrollo
make electron-test                             # tests unitarios del lanzador (npm test en electron/)
make bump-version VERSION=x.y.z                # fija la versión de la aplicación
```

Usa `go test -p 2 -race ./...` antes de una release para el detector de carreras, y un módulo cada vez mientras iteras (`go test -p 2 ./internal/modules/<feature>/...`).

## Grabar los GIFs de la documentación

Los GIFs del README se generan desde la aplicación real, contra una carpeta de datos desechable y los agentes falsos de la suite e2e, así que no necesitan ningún LLM y son reproducibles. Consulta [media/README.md](../media/README.md).
