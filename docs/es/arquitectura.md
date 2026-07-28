# Arquitectura

QUANTUM_LOG registra evidencia local y consciente de privacidad sobre actividad de agentes de código con IA. Su flujo central es:

```text
cmd/qlog -> internal/cli -> internal/app -> domain services/resolver -> internal/storage/sqlite
```

[Read in English](../en/architecture.md)

## Capas

| Capa | Responsabilidad |
| --- | --- |
| `cmd/qlog` | Solo entrypoint ejecutable. |
| `internal/cli` | Construcción de comandos Cobra y comportamiento a nivel comando. |
| `internal/app` | Abre servicios y centraliza ciclo de vida de lectura/escritura, bloqueos y checkpoints. |
| `internal/attribution/resolver` | Política pura de resolución de proyectos. |
| `internal/ingest/*` | Normaliza entradas JSONL, OTLP y qlog event. |
| `internal/adapters`, `internal/capture/wrapper` | Integraciones pasivas u orientadas a ciclo de vida. |
| `internal/storage/sqlite` | Migraciones, persistencia, reportes, sanitización y consultas SQLite. |
| `internal/audit` | Verifica cadenas SHA-256 append-only y anchors externos. |
| `internal/tui`, `internal/mcpserver` | Vistas de terminal y MCP sobre mismos servicios de consulta. |

Decisiones de diseño normativas viven en [registros de decisiones de arquitectura](../architecture/), especialmente [ADR-002](../architecture/ADR-002-project-first-attribution.md), [ADR-003](../architecture/ADR-003-local-ledger.md) y [ADR-004](../architecture/ADR-004-cooperative-sqlite-ownership.md).

## Resolución de proyecto

Propiedad de proyecto se resuelve desde evidencia, en este orden:

1. `--project` explícito.
2. `QLOG_PROJECT`.
3. Directorio actual registrado.
4. Raíz Git registrada.
5. Ruta registrada con coincidencia más larga.
6. Señal de proyecto del adaptador.
7. Sin resolver/sin atribuir.

Resolver devuelve método, confianza y valor de evidencia. Proveedor, modelo y nombre de agente nunca establecen propiedad de proyecto. Esto evita atribución conveniente pero no respaldada.

## Ciclo de vida de eventos

1. Importación JSONL, receptor OTLP, plugin, hook o wrapper recibe actividad.
2. Ingestión resuelve evidencia de proyecto y elimina contenido sensible.
3. Eventos raw normalizados se agregan a SQLite.
4. Eventos se encadenan por source y session con hashes SHA-256.
5. Evidencia de model-call puede alimentar consultas de uso, costo, allocation, task, export y anchor.
6. `verify` comprueba integridad de cadena; anchors externos detectan divergencia o truncamiento fuera de libro mayor local.

No todos eventos son registros de uso. Hook de Claude Code o `qlog run` crea evidencia de ciclo de vida y debe permanecer etiquetado de ese modo. Reportes solo muestran totales observados de tokens cuando evidencia ascendente los proporcionó.

## Contrato de calidad de captura

`capture_quality` declara qué puede respaldar un evento. Ejemplos incluyen `otel_reported`, `agent_reported`, `lifecycle_only` y `unavailable`. Se preserva en reportes y exportaciones porque totales de aspecto equivalente pueden tener procedencia diferente.

- `otel_reported`: campos de tokens llegaron mediante evidencia OTLP aceptada.
- `agent_reported`: integración de agente proporcionó campos de tokens.
- `lifecycle_only`: proceso o sesión ocurrió, pero no se afirma conteo de tokens.
- `unavailable`: no existe evidencia de uso soportada.

Nuevas integraciones deben elegir etiqueta veraz. No deben inferir ni estimar conteos de tokens solo para llenar columnas de reporte.

## Bloqueos y diagnósticos

Clientes SQLite oficiales usan protocolo cooperativo multiplataforma:

- Cada cliente toma acceso compartido de quiescencia.
- Writers también toman acceso exclusivo de writer.
- Diagnósticos de solo lectura (`doctor`, `verify`, `anchor check`) toman acceso exclusivo de quiescencia y bloquean mientras WAL activo o actividad de cliente vuelve insegura comprobación estable.

Implicación es operativa: usa comandos qlog, no editores SQLite externos ni aperturas inmutables, para mantenimiento normal. Rechazo diagnóstico por bloqueo es protección intencional contra observaciones inconsistentes.

## Límites locales

Datos permanecen locales por defecto bajo `QLOG_HOME` o valores de plataforma. Proyecto usa `modernc.org/sqlite`, permitiendo builds sin CGo. Migraciones SQLite están embebidas y se aplican en orden léxico.

Collector también prioriza lo local. Su listener por defecto es loopback y exposición fuera de loopback requiere flag deliberado. MCP se ejecuta por stdio. Ninguno de estos diseños reemplaza requisitos de seguridad de red si operador expone servicio más allá de máquina local.

## Límite actual de entrega

M4 sigue `IN_PROGRESS`. Captura de Copilot VS Code es experimental hasta que uso real originado en Copilot con tokens persista en SQLite y evidencia de verificación respalde afirmación. Instalación, dry run o uso genérico importado no alcanzan ese estándar.
