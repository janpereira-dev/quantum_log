# Referencia CLI

Esta referencia documenta definiciones actuales de comandos Cobra. Ejecuta `qlog <command> --help` para ayuda ejecutable de versión instalada. Todos comandos aceptan flag raíz `--home <path>` para sobrescribir directorio local de datos.

[Read in English](../en/cli-reference.md)

## Convenciones de seguridad

- Diagnósticos de solo lectura: `doctor`, `verify` y `anchor check` requieren acceso diagnóstico exclusivo. No modifican libro mayor y pueden fallar cuando cliente oficial mantiene bloqueo cooperativo o existe WAL activo.
- Comandos mutantes usan ciclo de vida normal de aplicación. No omitas protocolo de bloqueo mediante herramientas SQLite externas.
- Calidad de captura es evidencia, no decoración. Reportes deben retener etiquetas como `otel_reported`, `agent_reported` o `lifecycle_only`; totales de tokens no se inventan.
- Comandos de setup, instalación de adaptador y ciclo de vida de collector pueden cambiar configuración o servicios locales. Usa primero rutas dry-run/status cuando existan.

## `init`

**Sintaxis:** `qlog init`
**Propósito:** Inicializar configuración y libro mayor locales.
**Seguridad:** Crea estado local; usa `--home` para libro mayor de prueba aislado.
**Ejemplo:** `qlog init`
**Resultado:** `initialized QUANTUM_LOG at <home>`. Fallos usualmente identifican home inutilizable o problema de inicialización.

## `doctor`

**Sintaxis:** `qlog doctor [--json]`
**Propósito:** Comprobar estado de libro mayor sin modificarlo.
**Seguridad:** Operación diagnóstica exclusiva de solo lectura.
**Ejemplo:** `qlog doctor --json`
**Resultado:** texto `doctor: ok` o JSON con `status`, `database` y posible advertencia. Falla antes de inicialización, por migraciones pendientes, WAL activo o bloqueo de quiescencia retenido.

## `verify`

**Sintaxis:** `qlog verify [--session <id>]`
**Propósito:** Verificar cadenas hash append-only del libro mayor, opcionalmente para una sesión.
**Seguridad:** Operación diagnóstica exclusiva de solo lectura.
**Ejemplo:** `qlog verify --session session-123`
**Resultado:** `ledger: verified`; fallo informa problema de integridad o acceso diagnóstico.

## `maintenance`

**Sintaxis:** `qlog maintenance <status|checkpoint|recover|rebuild-anchor>`
**Propósito:** Gestionar mantenimiento controlado de libro mayor local.
**Seguridad:** Detén clientes oficiales antes de `checkpoint`; funciones de recuperación no están disponibles deliberadamente.
**Ejemplos:**

```bash
qlog maintenance status
qlog maintenance checkpoint
```

**Resultado:** `checkpoint` informa `maintenance checkpoint: WAL cleared`. `recover` y `rebuild-anchor` fallan con error not-implemented.

## `project`

**Sintaxis:**

```text
qlog project register [--path <path>] --name <name> [--slug <slug>]
qlog project current|detect [--project <slug>] [--json]
qlog project list [--json]
qlog project show <slug> [--json]
qlog project tag <key=value> --project <slug>
qlog project tag list --project <slug> [--json]
```

**Propósito:** Gestionar proyectos lógicos, ubicaciones físicas y tags normalizados.
**Seguridad:** Registro y tagging escriben metadatos de libro mayor. No uses nombres de proveedor/modelo como sustitutos de atribución.
**Ejemplo:**

```bash
qlog project register --path . --name MY_PROJECT
qlog project tag environment=work --project my-project
qlog project current --json
```

**Resultado:** registro informa `registered <slug> at <path>`; salida actual incluye método y confianza. `tag` rechaza entrada que no tenga forma `key=value`; comandos que requieren proyecto desconocido fallan claramente.

## `ingest`

**Sintaxis:** `qlog ingest file <path>` o `qlog ingest stdin`
**Propósito:** Importar eventos raw NDJSON normalizados.
**Seguridad:** Entrada se sanitiza antes de importación y hash chaining. Importa solo datos que tienes permiso de procesar.
**Ejemplo:** `qlog ingest file events.ndjson`
**Resultado:** `imported N event(s)`. Archivo ilegible, NDJSON inválido o suplantación de source reservado hacen fallar importación.

## `usage`

**Sintaxis:**

```text
qlog usage today|week|month [--group-by <dimensions>] [--json]
qlog usage project <slug> [--json]
```

**Propósito:** Mostrar uso de tokens observado.
**Seguridad:** Lee totales junto con `capture_quality`; eventos solo de ciclo de vida no son evidencia de tokens.
**Ejemplo:** `qlog usage today --group-by project,agent,provider,model,capture_quality`
**Resultado:** filas usan `project | agent | provider/model | capture_quality | N tokens`, seguidas por `TOTAL | N tokens`; `--json` devuelve estructura de reporte.

## `report`

**Sintaxis:**

```text
qlog report [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>] [--group-by <dimensions>] [--json]
qlog report summary [same flags]
```

**Propósito:** Resumir uso observado y costo asignado. `summary` es subcomando explícito; comando `report` de nivel superior también tiene `summary` como alias.
**Seguridad:** Costo refleja precios y asignaciones persistidos, no factura de cobro.
**Ejemplo:** `qlog report --from 2026-07-01 --to 2026-08-01 --json`
**Resultado:** filas incluyen tokens y micro USD, luego total. Valores de fecha inválidos fallan con guía de formatos aceptados.

## `allocation`

**Sintaxis:**

```text
qlog allocation split <model-call-id> <project=basis-points>...
qlog allocation show <model-call-id> [--json]
qlog allocation repair <model-call-id> --project <slug>
qlog allocation history <model-call-id> [--json]
qlog allocation revert <revision-id> --idempotency-key <key> --reason <text>
```

**Propósito:** Gestionar asignaciones de costo de model-call.
**Seguridad:** `split` agrega una corrección inmutable y actualiza una proyección reconstruible; repara solo cuando se conoce el propietario explícito y correcto. El historial nunca se elimina.
**Ejemplo:** `qlog allocation split call-1 alpha=5000 beta=5000`
**Resultado:** escrituras informan `allocation: updated` o `allocation: repaired`. Sintaxis de asignación inválida o proyectos desconocidos fallan.

## `pricing`

**Sintaxis:**

```text
qlog pricing validate <file>
qlog pricing add <file>
qlog pricing list [--json]
qlog pricing show <provider/model> [--json]
qlog pricing recalculate [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>]
```

**Propósito:** Gestionar registros versionados de precios y costos calculados de model-call.
**Seguridad:** Valida reglas antes de persistir; recalculación actualiza datos de costo calculado usando reglas persistidas.
**Ejemplo:** `qlog pricing validate pricing.json`
**Resultado:** validación imprime `pricing: valid`; add imprime ID de regla; recalculate informa `recalculated N model call(s)`. JSON/reglas, fechas o identidades faltantes inválidos fallan.

## `task`

**Sintaxis:**

```text
qlog task start --project <slug> --title <title> [--type <type>]
qlog task finish <task-id> [--result <result>]
qlog task list [--project <slug>] [--json]
qlog task summary <task-id> [--json]
```

**Propósito:** Asociar uso registrado con tareas de proyecto.
**Seguridad:** Registros de tareas organizan evidencia; no fabrican model calls ni tokens.
**Ejemplo:**

```bash
qlog task start --project my-project --title "Implement import" --type build
qlog task finish <task-id> --result success
```

**Resultado:** start imprime ID; finish imprime resumen de model-call, tokens y costo asignado. Flags project/title requeridos e IDs desconocidos fallan.

## `export`

**Sintaxis:** `qlog export [--format json|csv] [--from <RFC3339|YYYY-MM-DD>] [--to <RFC3339|YYYY-MM-DD>] [--redact-paths]`
**Propósito:** Exportar model calls normalizados como JSON o CSV.
**Seguridad:** Prefiere `--redact-paths` antes de compartir exportaciones. Registros exportados retienen calidad de captura y contexto de asignación.
**Ejemplo:** `qlog export --format csv --redact-paths > qlog-calls.csv`
**Resultado:** array JSON o CSV encabezado por campos de model-call y asignación. Formatos no soportados y fechas inválidas fallan.

## `anchor`

**Sintaxis:** `qlog anchor export` o `qlog anchor check --file <path>`
**Propósito:** Exportar y verificar anchors externos de libro mayor para detección de manipulación y truncamiento.
**Seguridad:** Almacena JSON de anchor exportado fuera del libro mayor y protégelo de modificación.
**Ejemplo:**

```bash
qlog anchor export > anchors.json
qlog anchor check --file anchors.json
```

**Resultado:** check imprime `anchors: ok`, o imprime desajustes/truncamientos y falla. Omitir `--file` falla antes de verificación.

## `setup`

**Sintaxis:** `qlog setup [adapter] [--all] [--yes] [--dry-run] [--json]`
**Propósito:** Planificar o aplicar configuración de integración de auto-captura.
**Seguridad:** Comienza con `--dry-run`; solo `--yes` aplica cambios. Sin adapter ni `--all`, setup selecciona adaptadores disponibles o instalados capaces de configurarse.
**Ejemplo:** `qlog setup opencode --dry-run --json`
**Resultado:** cada plan informa ID de adaptador, estado, calidad de captura y cambios. Adaptadores desconocidos fallan.

## `collector`

**Sintaxis:**

```text
qlog collector status [--listen <address>] [--json]
qlog collector serve [--listen <address>] [--allow-non-loopback]
qlog collector install|start|stop|restart|logs|uninstall
```

**Propósito:** Recibir y gestionar telemetría local. Endpoints de collector son `/v1/traces` para OTLP JSON/protobuf, `/v1/events` para qlog JSON y `/healthz`.
**Seguridad:** Listener por defecto es `127.0.0.1:4318`; vinculación fuera de loopback requiere opt-in explícito.
**Ejemplo:** `qlog collector serve`
**Resultado:** serve anuncia listener y endpoints. `status` informa alcance y estado. Dirección pública sin `--allow-non-loopback` falla.

## `adapter`

**Sintaxis:**

```text
qlog adapter list [--json]
qlog adapter detect [adapter] [--json]
qlog adapter install <adapter> [--dry-run] [--json]
qlog adapter status [adapter] [--json]
qlog adapter test <adapter> [--json]
qlog adapter verify <adapter> [--project <slug>] [--since <duration>] [--json]
qlog adapter uninstall <adapter> [--dry-run] [--json]
```

**Propósito:** Inspeccionar, instalar, probar, verificar y eliminar adaptadores de captura.
**Seguridad:** `install` y `uninstall` pueden cambiar configuración propiedad de qlog; dry-run primero. Verificación distingue configuración de evidencia.
**Ejemplo:** `qlog adapter verify copilot-vscode --project my-project --since 1h --json`
**Resultado:** salida status/test incluye calidad de captura. Copilot sigue sin verificar hasta que exista evidencia local OTLP reciente `model.call` con tokens `otel_reported`; ajustes solos no bastan.

## `hook`

**Sintaxis:** `qlog hook claude-code`
**Propósito:** Recibir payloads de hooks de ciclo de vida de Claude Code en entrada estándar.
**Seguridad:** Entrada del hook se reduce a metadatos de ciclo de vida seguros para privacidad. Prompt, transcript y contenido similar no se persisten.
**Ejemplo:** `qlog hook claude-code < hook-event.json`
**Resultado:** `hook: ingested N` cuando se almacena directamente, o `hook: forwarded` cuando `QLOG_COLLECTOR_URL` está definido. Entrada no JSON o respuestas de collector rechazadas fallan.

## `run`

**Sintaxis:** `qlog run [--project <slug>] [--agent <name>] -- <command> [arguments...]`
**Propósito:** Ejecutar comando y registrar metadatos de ciclo de vida de proceso seguros para privacidad.
**Seguridad:** Argumentos de comando, entorno y salida de proceso no se persisten intencionalmente. Esto es evidencia de ciclo de vida, no captura de uso.
**Ejemplo:** `qlog run --project my-project --agent codex -- codex`
**Resultado:** `recorded process session <id> (exit N)`. Salida no cero del comando envuelto devuelve error después de registrar ciclo de vida.

## `tui`

**Sintaxis:** `qlog tui`
**Propósito:** Abrir dashboard de terminal accesible.
**Seguridad:** Dashboard usa mismos servicios de consulta locales; no reemplaza `verify` para comprobaciones de evidencia.
**Ejemplo:** `qlog tui`
**Resultado:** inicia dashboard interactivo. Ejecutar `qlog` sin argumentos lo abre solo cuando salida es terminal; salida no terminal muestra ayuda.

## `mcp`

**Sintaxis:** `qlog mcp serve`
**Propósito:** Servir integración MCP local de QUANTUM_LOG sobre entrada/salida estándar.
**Seguridad:** Mantén stdio MCP aislado de salida de shell orientada a personas; es para integración de agentes.
**Ejemplo:** `qlog mcp serve`
**Resultado:** servidor MCP se ejecuta hasta que sesión stdio se cierra; errores de inicialización se propagan al invocador.

## `completion`

**Sintaxis:** `qlog completion <bash|fish|powershell|zsh>`
**Propósito:** Generar script de completado de shell para `qlog`.
**Ejemplo:** `qlog completion powershell`
**Resultado:** escribe script de completado solicitado en salida estándar. Ejecutarlo sin shell muestra ayuda de completion; shell desconocido falla como comando desconocido.

## `help`

**Sintaxis:** `qlog help [command] [flags]`
**Propósito:** Mostrar ayuda para `qlog` o ruta de comando.
**Ejemplo:** `qlog help project register`
**Resultado:** imprime uso de comando, comandos disponibles y flags. Ruta de comando desconocida falla con error de comando desconocido.

## Grupos raíz adicionales

CLI actual también expone `unattributed` y `budget`.

| Grupo | Sintaxis | Propósito y seguridad | Ejemplo y salida |
| --- | --- | --- | --- |
| `unattributed` | `list [--json]`; `repair <model-call-id> --project <slug>` | Inspeccionar calls sin asignaciones; reparar solo con evidencia explícita de propiedad. | `qlog unattributed list`; reparación informa `unattributed usage: assigned`. |
| `budget` | `set-project <slug> --monthly-usd-micros <n> [--alert-percent <n>]`; `set-tag <key=value> ...`; `status [--json]` | Configurar alertas mensuales de costo asignado. Budgets no bloquean uso. | `qlog budget status --json`; sintaxis de tag inválida o proyectos desconocidos fallan. |
