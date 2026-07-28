# Contribución

Contribuye cambios mediante paquetes Go enfocados, límites explícitos de privacidad y verificación respaldada por evidencia. Ruta ejecutable es deliberadamente delgada:

```text
cmd/qlog -> internal/cli -> internal/app -> domain services/resolver -> internal/storage/sqlite
```

[Read in English](../en/contributing.md)

## Desarrollo local

```bash
go build -o qlog ./cmd/qlog
go test -count=1 ./...
go vet ./...
```

Comandos enfocados opcionales:

```bash
go test -count=1 ./internal/cli
go test -count=1 ./internal/cli -run TestName
make build
make test
make race
make vet
make fmt
```

Proyecto usa `modernc.org/sqlite`, por lo que validación sin CGo está soportada:

```bash
CGO_ENABLED=0 go test -count=1 ./...
```

## Límites de cambio

| Área | Ubica cambios aquí |
| --- | --- |
| Inicio del ejecutable | Solo `cmd/qlog/`. |
| Definición de comando Cobra y pruebas de comandos | `internal/cli/`. |
| Ciclo de vida de contexto de aplicación | `internal/app/`. |
| Atribución pura de proyecto | `internal/attribution/resolver/`. |
| Persistencia, migraciones, reportes, sanitización | `internal/storage/sqlite/`. |
| Verificación de hash-chain/anchor | `internal/audit/`. |
| Normalización JSONL, OTLP, qlog event | `internal/ingest/`. |
| Integración de agentes y captura pasiva | `internal/adapters/`, `internal/capture/wrapper/`. |
| Vistas de terminal y MCP | `internal/tui/`, `internal/mcpserver/`. |

No hagas que comandos CLI alcancen internos SQLite cuando límite de aplicación o store ya posee comportamiento.

## Cambios CLI

Agrega comando con `newXxxCommand(home *string) *cobra.Command` bajo `internal/cli/`, regístralo desde `internal/cli/root.go` y agrega pruebas enfocadas con `--home <tmpdir>`. Mantén comportamiento actual de comandos preciso en [Referencia CLI](referencia-cli.md).

Comandos mutantes usan `app.Open`. Comandos de solo lectura usan `app.OpenReadOnly`. Diagnósticos requieren acceso exclusivo de quiescencia; nunca omitas este protocolo con cliente SQLite externo.

## Almacenamiento y migraciones

Nuevo comportamiento persistente pertenece a `*Store` en `internal/storage/sqlite/`. Cambios de esquema necesitan migración numerada bajo `internal/storage/sqlite/migrations/` y pruebas de almacenamiento. Migraciones se ejecutan en orden léxico.

Usa `t.TempDir()` para homes/bases de datos de prueba aislados. Nunca prepares `qlog.db`, archivos WAL/SHM ni archivos de bloqueo generados.

## Reglas de privacidad y captura

Contenido sensible debe sanitizarse antes de hashing o importación. Mantén cobertura de sanitizador cuando integración introduce otra familia de claves sensibles. No persistas prompts, respuestas, argumentos/resultados de herramientas, secretos, tokens, valores de autorización, API keys, cookies, valores de entorno ni salida de proceso.

Cada ruta de captura debe declarar calidad de captura veraz. No sintetices conteos de tokens. Evidencia solo de ciclo de vida debe permanecer solo de ciclo de vida en reportes, exportaciones, pruebas y documentación.

M4 sigue `IN_PROGRESS`. Captura de Copilot VS Code sigue experimental hasta que trace real persista tokens originados en Copilot en SQLite y evidencia de verificación relevante respalde afirmación.

## Validación y documentación

Antes de solicitar revisión, ejecuta como mínimo:

```bash
go test -count=1 ./...
go vet ./...
git diff --check
```

Para cambios de captura, ejecuta pruebas relevantes de adaptador/collector y valida contrato explícito de calidad de captura. Para cambios CLI, verifica salida `--help` y actualiza docs sin inventar flags ni recuperación no soportada.

## Entrega de release

Mantén evidencia de release separada de afirmaciones públicas. No marques milestone `VERIFIED` sin evidencia completa de aceptación aprobada en [`docs-int/verification/`](../../docs-int/verification/). Mantén ADRs bajo `docs/architecture/` como registros normativos.

Maintainers pueden ejecutar dry run de release:

```bash
goreleaser snapshot --clean
```

Mensajes de commit usan Conventional Commits. No agregues `Co-Authored-By` ni atribución de IA.
