# MASTER BUILD PROMPT — QUANTUM_LOG

> Versión del prompt: **1.3 (0.2.0)**  
> Enfoque: **project-first attribution, multi-project usage accounting y ejecución multiagente gobernada**.

## Estado verificable y reglas de evidencia

Release **0.2.0**. Toda la suite `go test ./...` pasa reproduciblemente. M1 cerrado
y verificado; M2-M6 funcional pero no auditado contra matriz AC completa.

| Milestone | Estado de ciclo de vida | Estado de evidencia |
|---|---|---|
| M0 | `VERIFIED` | Init, paths, config. |
| M1 | `VERIFIED` | Resolución, bloqueos cooperativos, doctor/verify read-only, sanitización evidence, anclas externas + detección de truncamiento. |
| M2 | `IMPLEMENTED` | Reporting, allocations, pricing, export. Suite verde. |
| M3 | `IMPLEMENTED` | TUI con servicios de consulta compartidos. |
| M4 | `DETECTION_ONLY` | Modelo de adapter presente; captura verificada no reclamada. |
| M5 | `IMPLEMENTED` | Configs de distribución presentes; installers nativos pendientes de publicación en registros externos. |
| M6 | `IMPLEMENTED` | Servidor MCP stdio y hooks de agente. |

El ciclo de vida de un milestone usa exactamente `NOT_STARTED`, `IN_PROGRESS`,
`IMPLEMENTED`, `VERIFIED`, `BLOCKED` o `DEFERRED`. `DETECTION_ONLY` describe la
madurez de captura de M4, no un séptimo estado del ciclo de vida. Un milestone solo
puede ser `VERIFIED` cuando cada criterio requerido es `PASS` en
`docs/verification/milestone-<n>-evidence.md`; cualquier `FAIL`, `NOT_RUN` o
`BLOCKED` lo impide. Las secciones posteriores que describen entregables son
objetivos de recuperación, no afirmaciones de disponibilidad actual.

## Rol

Actúa como un equipo senior coordinado, compuesto por:

- Principal Go Architect.
- Staff Engineer especializado en CLI y TUI multiplataforma.
- Arquitecto de atribución multi-proyecto y FinOps de IA.
- Ingeniero de observabilidad y OpenTelemetry para sistemas de IA generativa.
- Arquitecto de datos SQLite local-first.
- Ingeniero de seguridad, privacidad y supply chain.
- Release Engineer especializado en binarios Go, GoReleaser, GitHub Releases e instaladores multiplataforma.
- Product Engineer con criterio fuerte de usabilidad, accesibilidad y experiencia de desarrollador.

No te limites a generar un prototipo visual. Construye una base de producto mantenible, auditable, extensible y preparada para distribución pública, de modo que otros desarrolladores puedan instalarla y usarla en sus propios entornos.

## Modelo de ejecución multiagente

Trabaja mediante **un orquestador y seis subagentes especializados**. No todos deben ejecutarse en todos los milestones.

### Orquestador

#### `quantum-log-product-orchestrator`

Responsabilidades:

- Mantener visión de producto, roadmap, ADRs, dependencias y gates.
- Dividir el trabajo en unidades pequeñas y verificables.
- Asignar un único propietario por artefacto para evitar conflictos.
- Consolidar resultados y resolver contradicciones entre subagentes.
- Impedir que una especialidad avance si rompe contratos de otra.
- Exigir evidencia real de build, tests, lint y migraciones.
- Mantener un ledger de decisiones, riesgos, deuda consciente y tareas pendientes.

### Subagentes

#### 1. `project-attribution-architect`

Responsable de:

- Identidad lógica de proyecto.
- Ubicaciones locales, checkouts y worktrees.
- Contextos de trabajo temporales.
- Resolución automática de proyecto.
- Sesiones que cambian entre proyectos.
- Tags, centros de coste y agrupaciones.
- Atribución directa y reparto excepcional entre varios proyectos.

#### 2. `go-sqlite-core-engineer`

Responsable de:

- Dominio Go.
- Persistencia SQLite CGo-free.
- Migraciones.
- Concurrencia, WAL y transacciones.
- Raw events append-only.
- Normalización y cadena hash.

#### 3. `observability-adapters-engineer`

Responsable de:

- OpenTelemetry GenAI.
- Recepción OTLP.
- Hooks, plugins y wrappers.
- Contrato y matriz de capacidades de adaptadores.
- Correlación de sesiones, turns, model calls y tool calls.
- Señales de cambio de directorio, workspace, repositorio y proyecto.

#### 4. `finops-pricing-engineer`

Responsable de:

- Tokens y métricas de consumo.
- Pricing Registry versionado.
- FX.
- Cost snapshots.
- Allocated cost por proyecto y tag.
- Presupuestos, alertas y consultas agregadas.

#### 5. `cli-tui-product-engineer`

Responsable de:

- Cobra.
- Bubble Tea, Lip Gloss y Bubbles.
- Comandos, navegación y UX.
- Vistas de proyectos, consumo, contexto, costes e integridad.
- Salida humana, JSON y NDJSON estable.
- Accesibilidad y degradación correcta en terminales limitados.

#### 6. `security-release-guardian`

Responsable de:

- Privacidad y redacción.
- Integridad del ledger.
- Permisos locales.
- Supply chain.
- GoReleaser, firmas, checksums, SBOM e instaladores.
- Revisión independiente de riesgos y criterios de aceptación.

### Reglas de coordinación

- Cada subagente debe recibir un alcance explícito, contratos de entrada y criterios de salida.
- Ningún subagente debe modificar archivos propiedad de otro sin coordinación del orquestador.
- Los subagentes deben devolver: decisiones, archivos afectados, contratos introducidos, comandos ejecutados, resultados reales y riesgos abiertos.
- El orquestador debe rechazar resultados simulados, stubs engañosos o compatibilidades no demostradas.
- La captura técnica no debe depender de que una skill o subagente esté activo.
- El consumo generado por los propios subagentes debe poder ser atribuido a QUANTUM_LOG cuando el sistema ya pueda medirse a sí mismo.

### Activación por milestone

Para `Milestone 0` y `Milestone 1`, activar únicamente:

```text
quantum-log-product-orchestrator
project-attribution-architect
go-sqlite-core-engineer
security-release-guardian
```

Activar `observability-adapters-engineer`, `finops-pricing-engineer` y `cli-tui-product-engineer` cuando sus contratos base existan y estén aprobados.

---

# 1. Nombre y contrato de marca

La marca del producto es:

```text
QUANTUM_LOG
```

Reglas obligatorias:

- En comunicación, documentación, encabezados y marca, escribir siempre `QUANTUM_LOG`, en mayúsculas y con underscore.
- El nombre público del proyecto, paquetes y artefactos debe ser `quantum-log` cuando el ecosistema no admita underscore.
- El repositorio fuente actual es `github.com/janpereira-dev/quantum_log`; no renombrarlo automáticamente sin un plan explícito de migración.
- El módulo Go debe usar `github.com/janpereira-dev/quantum_log`.
- El ejecutable principal y comando de terminal debe llamarse `qlog`.
- El daemon, cuando exista, debe llamarse `qlogd`, pero el MVP debe priorizar un único binario `qlog` con subcomandos.
- La carpeta local de datos debe respetar XDG y las convenciones de cada sistema operativo; no fijar rutas Unix en Windows.
- No recrear el isotipo mediante caracteres ASCII o Unicode. En terminales sin soporte gráfico, mostrar el wordmark `QUANTUM_LOG`, no una imitación textual del logo.
- El producto no está relacionado con computación cuántica. Definir “quantum” como la unidad mínima observable de trabajo de IA: una llamada, evento, transición o evidencia registrada.

Tagline principal:

```text
Trace every agent. Trust every event.
```

Descripción corta:

```text
Local-first observability and FinOps for AI coding agents.
```

Descripción funcional:

```text
QUANTUM_LOG registra, normaliza, atribuye y audita el uso de modelos de IA, agentes, herramientas y tareas de desarrollo, sin depender de un único proveedor y conservando qué proyecto recibió cada unidad de consumo.
```

---

# 2. Misión del producto

Construir una herramienta open source, local-first y agnóstica de proveedor que permita a un desarrollador o equipo conocer:

- Qué proyecto consumió recursos de IA.
- Cuándo se produjo ese consumo dentro del proyecto.
- La ubicación absoluta local, checkout o worktree desde el que se trabajó.
- Qué agente originó la actividad.
- Qué proveedor y modelo se utilizó en cada llamada.
- Cuánto tiempo duró la tarea, sesión, contexto de trabajo, turno y llamada.
- Tamaño del prompt de entrada y respuesta de salida.
- Tokens de entrada, salida, reasoning, caché y escritura de caché.
- Porcentaje real de contexto utilizado cuando pueda calcularse con datos fiables.
- Tipo de tarea: plan, research, design, build, refactor, debug, test, review, documentation, migration, upgrade, security, deploy u other.
- Número de llamadas de modelo, herramientas, MCP, subagentes, reintentos y errores.
- Coste estimado en USD y EUR.
- Coste real facturado cuando pueda importarse de una fuente oficial.
- Coste atribuido o repartido entre proyectos cuando una llamada beneficie a más de uno.
- Calidad y procedencia de cada dato: exacto, estimado, inferido, manual o no disponible.
- Resultado de la tarea y relación entre coste y trabajo útil.
- Qué consumo continúa sin atribución de proyecto.
- Cómo se agrega el consumo por proyecto, tag, proveedor, modelo, agente, tipo de tarea, rama, host y periodo.

El producto debe soportar que un mismo agente, sesión o terminal trabaje sucesivamente en varios proyectos. La atribución debe seguir al proyecto activo en el momento exacto de cada llamada, no al proyecto con el que comenzó la sesión.

El producto debe poder responder preguntas como:

```text
¿Cuánto gasté hoy usando Codex, Claude Code, GitHub Copilot y OpenCode?
¿Cuánto consumió QUANTUM_LOG hoy, esta semana y este mes?
¿Cuándo se generó el consumo dentro de este proyecto?
¿Qué proyecto consumió más tokens este mes?
¿Qué proveedor y modelo consumió cada proyecto?
¿Qué modelo genera más reintentos dentro de cada proyecto?
¿Qué tipo de tarea tiene mayor coste medio?
¿Cuánto consumo fue atendido desde caché?
¿Qué porcentaje del coste provino de tokens de salida?
¿Qué sesiones tuvieron presión de contexto elevada?
¿Qué proyectos fueron utilizados durante una misma sesión?
¿Cuánto consumo corresponde al tag environment=work o cost-center=research?
¿Qué llamadas siguen en el bucket unattributed?
¿Qué registros son exactos y cuáles son estimaciones?
¿El ledger local ha sido alterado?
```

Las dimensiones deben almacenarse por separado. No persistir identificadores compuestos como `provider/model/project`. Los informes podrán ordenar libremente:

```text
project,provider,model
provider,model,project
project,agent,model
project-tag,project,provider,model
date,project,task-type,model
```

---

# 3. Diferenciadores obligatorios

QUANTUM_LOG no debe ser un contador superficial de tokens. Debe diferenciarse mediante estos pilares:

## 3.1 Ledger verificable

Mantener un almacén append-only de eventos normalizados y una cadena hash por fuente o sesión:

```text
event_hash = SHA-256(canonical_event + previous_event_hash)
```

Crear:

```bash
qlog verify
qlog verify --session <id>
qlog verify --from 2026-07-01
```

La cadena hash es una prueba local de integridad, no una blockchain. No introducir criptomonedas, consenso distribuido ni complejidad innecesaria.

## 3.2 Contabilidad consciente de incertidumbre

Nunca presentar estimaciones como datos oficiales. Cada métrica debe almacenar:

```text
capture_source
capture_quality
confidence
```

Valores de `capture_source`:

```text
provider_reported
otel_reported
sdk_reported
hook_reported
locally_counted
estimated
inferred
manual
unavailable
```

Valores de `confidence`:

```text
exact
high
medium
low
unknown
```

## 3.3 Costes con vigencia temporal

Los precios deben estar versionados y tener fecha de entrada en vigor. Una modificación futura del precio de un modelo no puede alterar silenciosamente informes históricos.

## 3.4 Local-first y privacy-first

No enviar datos a servidores externos por defecto. No guardar contenido de prompts o respuestas por defecto. Registrar tamaños, hashes y métricas suficientes para observabilidad sin capturar código o información sensible.

## 3.5 Agnosticismo real

No diseñar el dominio alrededor de OpenAI, Anthropic, Google, GitHub o un único agente. Los proveedores comerciales, modelos locales, modelos open source y modelos corporativos deben entrar mediante el mismo contrato de adaptador.

Los usuarios deben poder registrar sus propios proveedores, endpoints, modelos y fórmulas de coste sin modificar el núcleo.

## 3.6 Atribución project-first

El proyecto es una dimensión de primer nivel e independiente de agente, proveedor y modelo.

Reglas:

- Cada `ModelCall`, `ToolCall` y `RawEvent` debe poder asociarse al proyecto activo en ese instante.
- Una `Session` puede contener múltiples `WorkContext` y múltiples proyectos.
- La ruta física, rama y commit no son propiedades permanentes de `Project`.
- Los tags complementan al proyecto; nunca sustituyen `project_id`.
- El bucket `unattributed` debe ser visible y auditable.
- La resolución de proyecto debe guardar método, evidencia y confianza.
- Cuando una llamada beneficie materialmente a varios proyectos, permitir una asignación explícita en basis points; por defecto, el 100 % pertenece al proyecto primario.
- Los informes deben poder agrupar por cualquier orden sin cambiar el modelo almacenado.

## 3.7 Separación entre consumo y asignación

Distinguir:

```text
usage observed
cost calculated
cost allocated
```

El consumo real pertenece a una llamada. La asignación financiera puede asociarlo a uno o varios proyectos sin alterar los datos de origen.

---

# 4. No objetivos

No construir en el MVP ni introducir posteriormente sin una decisión explícita de producto:

- Una bóveda de tarjetas bancarias.
- Un sistema para autorizar compras de agentes.
- Una pasarela de pagos.
- Una blockchain.
- Una plataforma SaaS obligatoria.
- Un proxy obligatorio para todas las llamadas de IA.
- Un sistema que capture el contenido completo de prompts por defecto.
- Un dashboard web antes de que CLI, TUI, almacenamiento, atribución y captura sean sólidos. El dashboard podrá llegar en una fase futura.
- Integraciones falsas que simulen conocer tokens, costes, modelos o proyectos cuando la fuente no los expone.
- Atribuciones silenciosas basadas únicamente en el nombre del agente, proveedor o modelo.
- Un identificador persistente combinado `provider/model/project`.
- Datos hipotéticos presentados como actividad real. Solo registrar lo realizado o datos claramente marcados como fixture o demo.

---

# 5. Stack técnico obligatorio

## 5.1 Núcleo

- Go como lenguaje principal.
- Un único binario multiplataforma llamado `qlog`.
- `CGO_ENABLED=0` siempre que las dependencias lo permitan.
- Cobra para la jerarquía de comandos CLI.
- Bubble Tea para la arquitectura TUI.
- Lip Gloss para layout y estilos.
- Bubbles para componentes reutilizables de TUI.
- SQLite mediante un driver CGo-free, preferentemente `modernc.org/sqlite`.
- `database/sql` como abstracción base.
- Migraciones SQL embebidas con `go:embed`.
- `log/slog` para logging estructurado interno.
- OpenTelemetry para trazas, métricas y recepción de eventos GenAI cuando sea posible.
- Configuración YAML o TOML, validada y con schema versionado.
- JSON y NDJSON para importación y exportación.

## 5.2 Restricciones

- El núcleo no puede requerir Node.js, Python, Docker ni una JVM.
- Node/Bun solo podrán utilizarse como canales opcionales de distribución o para plugins específicos.
- No usar una ORM pesada.
- No usar floats para dinero.
- No almacenar costes como `REAL` sin control de precisión.
- No acoplar la interfaz TUI con la lógica de negocio.
- No mezclar acceso SQLite directamente dentro de comandos Cobra o modelos Bubble Tea.
- No acoplar la identidad de proyecto al proveedor o modelo.
- No usar un JSON libre como único mecanismo para tags consultables.
- Permitir modelos propios, endpoints privados y LLM locales mediante configuración y adaptadores, sin forks del core.
- La resolución de proyecto debe ser un servicio de dominio testeable, no lógica dispersa en comandos o plugins.

## 5.3 Dinero y precisión

Almacenar importes monetarios como enteros en micros de moneda:

```text
1 USD = 1_000_000 micros
1 EUR = 1_000_000 micros
```

Usar `int64` para tokens, tamaños, duraciones e importes. Guardar tasas FX como decimal canónico o entero escalado, junto con fuente y fecha.

---

# 6. Arquitectura del repositorio

Crear una arquitectura inicial similar a:

```text
quantum_log/
├── cmd/
│   └── qlog/
│       └── main.go
├── internal/
│   ├── app/
│   ├── audit/
│   ├── attribution/
│   │   ├── resolver/
│   │   ├── allocation/
│   │   └── evidence/
│   ├── cli/
│   ├── config/
│   ├── domain/
│   ├── projects/
│   ├── storage/
│   │   ├── sqlite/
│   │   └── migrations/
│   ├── ingest/
│   │   ├── otlp/
│   │   ├── jsonl/
│   │   └── manual/
│   ├── adapters/
│   │   ├── registry/
│   │   ├── generic/
│   │   ├── codex/
│   │   ├── claude/
│   │   ├── copilot/
│   │   └── opencode/
│   ├── pricing/
│   ├── fx/
│   ├── privacy/
│   ├── reports/
│   ├── tui/
│   ├── update/
│   └── version/
├── pkg/
│   └── sdk/
├── schemas/
│   ├── config.schema.json
│   ├── event.schema.json
│   ├── project.schema.json
│   └── pricing.schema.json
├── pricing/
│   ├── providers/
│   └── examples/
├── adapters/
│   ├── opencode-plugin/
│   ├── claude-hooks/
│   └── vscode-extension/
├── skills/
│   └── quantum-log-usage-governor/
│       └── SKILL.md
├── agents/
│   ├── quantum-log-product-orchestrator.md
│   ├── project-attribution-architect.md
│   ├── go-sqlite-core-engineer.md
│   ├── observability-adapters-engineer.md
│   ├── finops-pricing-engineer.md
│   ├── cli-tui-product-engineer.md
│   └── security-release-guardian.md
├── installers/
│   ├── install.sh
│   ├── install.ps1
│   ├── install.cmd
│   ├── uninstall.sh
│   └── uninstall.ps1
├── packaging/
│   ├── npm/
│   ├── homebrew/
│   ├── scoop/
│   ├── winget/
│   └── aur/
├── docs/
│   ├── architecture/
│   ├── attribution/
│   ├── adapters/
│   ├── privacy/
│   ├── pricing/
│   └── releases/
├── .github/
│   └── workflows/
├── .goreleaser.yaml
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
├── README.md
├── SECURITY.md
├── CONTRIBUTING.md
└── CHANGELOG.md
```

No crear carpetas vacías sin sentido. Cada carpeta introducida debe tener una responsabilidad documentada y al menos un caso de uso real.

Las responsabilidades deben respetar estas dependencias:

```text
CLI / TUI / adapters
        ↓
application services
        ↓
domain + attribution contracts
        ↓
storage / pricing / external integrations
```

El dominio y el resolver de proyecto no pueden depender de Cobra, Bubble Tea, SQLite ni de un proveedor específico.

---

# 7. Modelo de dominio

Modelar explícitamente:

```text
Host
Project
ProjectLocation
ProjectTag
WorkContext
Task
Session
Turn
ModelCall
ToolCall
UsageAllocation
RawEvent
NormalizedEvent
Agent
Adapter
PricingRule
CostSnapshot
FxRate
Budget
ExportJob
```

Relaciones conceptuales:

```text
Host
 └── ProjectLocation ───────┐
                            │
Project ────────────────────┤
 ├── ProjectTag             │
 └── WorkContext ◄──────────┘
      ├── Task
      ├── Session
      ├── Turn
      ├── ModelCall
      └── ToolCall

ModelCall / ToolCall
 └── UsageAllocation
      └── Project
```

Una sesión puede relacionarse con varios `WorkContext`; por tanto, `Session` no debe tener un único proyecto autoritativo.

## 7.1 Principios de atribución

- `Project` representa una identidad lógica estable.
- `ProjectLocation` representa una copia física, checkout o worktree en una máquina.
- `WorkContext` representa el contexto temporal desde el que ocurrió el consumo.
- `UsageAllocation` representa cómo se imputa consumo o coste a proyectos.
- `provider`, `model`, `agent` y `project` son dimensiones separadas.
- No persistir `provider/model/project` como clave compuesta de negocio.
- La agrupación es una responsabilidad de consulta y presentación.
- Cada dato de atribución debe incluir método, evidencia y confianza.
- La ausencia de proyecto debe conservarse como `unattributed`, nunca ocultarse ni asignarse arbitrariamente.

## 7.2 Hosts

Campos mínimos:

```text
id
name
os
arch
machine_fingerprint_hash
first_seen_at
last_seen_at
created_at
updated_at
```

No guardar identificadores de hardware sensibles en claro. El fingerprint debe ser local, estable y revocable.

## 7.3 Projects

Campos mínimos:

```text
id
slug
name
canonical_key
description
project_type
repository_url_normalized
vcs_provider
default_branch
status
created_at
updated_at
archived_at
```

Reglas:

- `canonical_key` debe ser estable y único dentro de la instalación.
- `slug` es una etiqueta humana y puede cambiar sin romper referencias.
- `Project` no debe almacenar como propiedades permanentes `absolute_path`, `git_branch` ni `git_commit`.
- Proyectos sin repositorio remoto deben poder existir mediante identidad local explícita.

## 7.4 Project locations

Campos mínimos:

```text
id
project_id
host_id
absolute_path
path_hash
vcs_root
workspace_root
worktree_name
is_primary
first_seen_at
last_seen_at
created_at
updated_at
```

Una misma identidad lógica puede aparecer en:

```text
C:\Repositorios\quantum_log
/home/jan/repos/quantum_log
/Users/jan/Code/quantum_log
```

Las tres ubicaciones pueden pertenecer al mismo `project_id`.

La ruta absoluta puede almacenarse localmente. En exportaciones debe poder sustituirse por hash, alias o ruta relativa.

## 7.5 Project tags

Campos mínimos:

```text
id
project_id
tag_key
tag_value
created_at
```

Aplicar unicidad por:

```text
project_id + tag_key + tag_value
```

Ejemplos:

```text
domain=ai-engineering
technology=go
portfolio=personal
environment=work
cost-center=research
client=example
```

Los tags deben estar normalizados para consultas SQL. Puede existir una representación JSON derivada, pero no debe ser la única fuente de verdad.

## 7.6 Work contexts

Campos mínimos:

```text
id
primary_project_id
project_location_id
task_id
session_id
host_id
cwd
git_root
workspace_root
workspace_name
git_branch
git_commit
terminal_id
process_id
started_at
finished_at
resolution_method
resolution_confidence
resolution_evidence_json
created_at
updated_at
```

Abrir un nuevo `WorkContext` cuando cambie de forma relevante alguno de estos elementos:

```text
cwd
git_root
workspace_root
project_id
worktree
git_branch, cuando afecte la atribución solicitada
```

Cerrar el contexto anterior con timestamp antes de abrir el siguiente.

## 7.7 Raw events

Mantener una tabla `raw_events` append-only que permita re-normalizar datos cuando mejoren los adaptadores.

Campos mínimos:

```text
id
source_event_id
schema_version
adapter_id
source
source_version
event_type
occurred_at
received_at
trace_id
span_id
parent_span_id
project_id
project_location_id
work_context_id
task_id
session_id
project_resolution_method
project_resolution_confidence
project_resolution_evidence_json
payload_json_sanitized
capture_source
capture_quality
confidence
previous_event_hash
event_hash
created_at
```

Aplicar una restricción única para evitar duplicados por `adapter_id + source_event_id` cuando exista identificador de origen.

El evento original sanitizado no debe modificarse después de insertarse.

## 7.8 Tasks

Campos mínimos:

```text
id
primary_project_id
initial_work_context_id
title
task_type
status
started_at
finished_at
duration_ms
result
human_outcome
created_at
updated_at
```

Tipos permitidos:

```text
plan
research
design
build
refactor
debug
test
review
documentation
migration
upgrade
security
deploy
other
```

Los tags de tarea deben normalizarse en una tabla propia cuando deban consultarse. No usar `tags_json` como única fuente para analítica.

Una tarea puede tener un proyecto primario, pero las llamadas individuales continúan siendo la autoridad para la atribución final.

## 7.9 Sessions y turns

Guardar agente, proceso, terminal, host, versión, correlación y tiempos.

Reglas:

- No asumir que una sesión utiliza un único modelo.
- No asumir que una sesión pertenece a un único proyecto.
- Una sesión puede contener varios `WorkContext` consecutivos o intercalados.
- Un turn puede heredar el contexto activo, pero cada llamada debe conservar una copia de la atribución resuelta.

## 7.10 Model calls

Campos mínimos:

```text
id
primary_project_id
project_location_id
work_context_id
task_id
session_id
turn_id
trace_id
span_id
started_at
finished_at
duration_ms
agent_name
agent_version
provider
model_id
model_version
input_tokens
output_tokens
reasoning_tokens
cached_input_tokens
cache_write_tokens
total_tokens
input_chars
output_chars
input_bytes
output_bytes
context_window_tokens
context_used_tokens
context_used_percent_basis_points
tool_calls_count
mcp_calls_count
subagent_calls_count
retry_count
billing_mode
pricing_rule_id
estimated_cost_usd_micros
estimated_cost_eur_micros
actual_cost_usd_micros
actual_cost_eur_micros
fx_rate_id
project_resolution_method
project_resolution_confidence
project_resolution_evidence_json
capture_source
capture_quality
confidence
success
error_type
created_at
```

Cada fila representa una llamada real a un único modelo.

`primary_project_id` debe almacenar el proyecto resuelto en el momento exacto de la llamada. No depender únicamente de `task_id` o `session_id` para reconstruirlo posteriormente.

No calcular porcentaje de contexto como `tokens acumulados / context window`. Solo calcularlo cuando exista una estimación fiable de los tokens actualmente presentes en la ventana del modelo.

## 7.11 Tool calls

Campos mínimos:

```text
id
primary_project_id
project_location_id
work_context_id
model_call_id
task_id
session_id
tool_name
tool_type
mcp_server
started_at
finished_at
duration_ms
success
input_size_bytes
output_size_bytes
project_resolution_method
project_resolution_confidence
capture_quality
created_at
```

La herramienta debe heredar el contexto de la llamada o sesión, pero debe conservar la atribución resuelta para auditoría.

## 7.12 Usage allocations

Crear una entidad separada para asignar consumo o coste a proyectos:

```text
id
subject_type
subject_id
project_id
allocation_basis_points
allocation_method
confidence
evidence_json
created_at
```

Valores iniciales de `subject_type`:

```text
model_call
tool_call
cost_snapshot
```

Valores iniciales de `allocation_method`:

```text
direct
explicit
work_context
adapter
manual
tag_rule
split
unattributed
```

Reglas:

- En el caso común, crear una asignación de `10000` basis points al `primary_project_id`.
- Si una llamada beneficia materialmente a varios proyectos, permitir varias filas cuya suma sea exactamente `10000`.
- Validar la suma dentro de una transacción.
- No modificar tokens originales para simular el reparto.
- Los informes financieros deben usar `UsageAllocation` cuando exista.
- `unattributed` es un bucket de reporting, no un proyecto ficticio obligatorio en la tabla `projects`.

## 7.13 Cost snapshots

Separar el consumo del cálculo financiero. Guardar cada cálculo con:

```text
id
model_call_id
pricing_rule_id
pricing_catalog_version
calculation_formula_version
fx_rate
fx_source
fx_date
calculated_at
estimated_cost
actual_cost
allocated_cost
created_at
```

Esto permitirá recalcular sin destruir el valor histórico original.

Los importes asignados a proyectos deben derivarse de `UsageAllocation` y conservar el snapshot utilizado.

## 7.14 Resolución automática de proyecto

Aplicar este orden de precedencia:

```text
1. Proyecto indicado explícitamente en el comando, MCP o API.
2. Tarea activa asociada a un proyecto.
3. Proyecto informado de forma fiable por el adaptador.
4. Variable QLOG_PROJECT.
5. Current working directory del proceso.
6. Git root detectado.
7. Remote URL normalizada.
8. Workspace del IDE o agente.
9. Longest path match contra ProjectLocation registradas.
10. Resolución manual posterior.
11. unattributed.
```

Métodos normalizados:

```text
explicit
active_task
adapter
environment
cwd
git_root
remote_url
ide_workspace
registered_path
manual
unresolved
```

Reglas:

- Nunca inferir el proyecto a partir del proveedor, modelo o nombre del agente.
- Guardar evidencia mínima y sanitizada de la resolución.
- No cambiar retroactivamente una atribución exacta sin crear un evento de corrección auditable.
- Las resoluciones inferidas deben poder revisarse o corregirse manualmente.

## 7.15 Sesiones multi-proyecto

Ejemplo obligatorio a soportar:

```text
09:00–09:40  ngAutoPilot
09:41–10:20  quantum-log
10:21–10:45  ngAutoPilot
```

La sesión debe conservar tres `WorkContext` y atribuir cada llamada al contexto activo:

```text
ngAutoPilot   60,000 tokens
quantum-log   31,000 tokens
```

No atribuir toda la sesión al proyecto inicial ni al último proyecto observado.

## 7.16 Dimensiones de consulta

Los informes deben poder agrupar y ordenar por:

```text
project
project-tag
project-location
provider
model
agent
task-type
date
day
hour
branch
host
capture-quality
```

Ejemplos de orden:

```text
project,provider,model
provider,model,project
project,model,agent
project-tag,project,provider,model
date,project,task-type
```

La elección de orden no debe crear esquemas o tablas diferentes.

---

# 8. Pricing Registry

Crear un registro extensible de precios cargado desde archivos YAML o JSON.

Ejemplo:

```yaml
schemaVersion: 1
provider: example-provider
modelPattern: example-model-pro
validFrom: 2026-07-01T00:00:00Z
validUntil: null
billingMode: token
currency: USD
unitTokens: 1000000
prices:
  inputMicros: 3000000
  cachedInputMicros: 750000
  cacheWriteMicros: 0
  outputMicros: 15000000
  reasoningMicros: 0
source:
  type: manual
  reference: provider-pricing-page
  checkedAt: 2026-07-16T00:00:00Z
version: 2026.07.1
```

Modos de facturación soportados:

```text
token
request
premium_request
session
subscription
seat
local_compute
fixed
custom_formula
unknown
```

Para modelos locales, permitir coste cero de API y una fórmula opcional por duración, GPU, CPU o electricidad.

Comandos:

```bash
qlog pricing list
qlog pricing show <provider/model>
qlog pricing add <file>
qlog pricing validate <file>
qlog pricing update
qlog pricing recalculate --from <date> --to <date>
```

Nunca sobrescribir reglas históricas. Crear nuevas versiones efectivas.

El motor de pricing calcula coste por llamada. La imputación por proyecto se realiza después mediante `UsageAllocation`; no duplicar reglas de precio por proyecto salvo que exista una tarifa contractual realmente distinta.

---

# 9. Contrato de adaptadores

Definir una interfaz de adaptador estable. Cada adaptador debe declarar capacidades, no solo un nombre.

Ejemplo conceptual:

```go
type Capabilities struct {
    ModelIdentity       bool
    InputTokens         bool
    OutputTokens        bool
    ReasoningTokens     bool
    CacheTokens         bool
    ContextUsage        bool
    ToolCalls           bool
    MCPCalls            bool
    Costs               bool
    PromptSizes         bool
    ResponseSizes       bool
    SessionLifecycle    bool
    TaskMetadata        bool
    ProjectIdentity     bool
    WorkingDirectory    bool
    VCSContext          bool
    WorkspaceContext    bool
    ProjectSwitchEvents bool
}
```

Cada adaptador debe implementar:

```text
ID
Name
Version
Detect
Capabilities
Install
Uninstall
HealthCheck
Ingest
Normalize
ExtractProjectSignals
```

Reglas:

- `Detect` no puede modificar archivos.
- `Install` debe soportar `--dry-run`.
- Antes de modificar configuraciones de un agente, crear copia de seguridad.
- Las modificaciones deben ser idempotentes.
- Nunca afirmar compatibilidad total cuando solo se capturan tiempo y proceso.
- Mostrar una matriz de capacidades en `qlog adapter list`.
- El adaptador entrega señales; el resolver central toma la decisión de proyecto.
- No duplicar lógica de resolución de proyecto en cada adaptador.
- Registrar si una señal de proyecto es exacta, inferida o no disponible.

Fuentes de captura, por prioridad:

1. Telemetría oficial del proveedor o agente.
2. OpenTelemetry GenAI.
3. SDK oficial instrumentado.
4. Hooks oficiales.
5. Plugin específico.
6. Importación de logs estructurados.
7. Wrapper genérico de proceso.
8. Estimación local claramente etiquetada.

Señales de proyecto, por prioridad:

1. Project ID explícito.
2. Task activa.
3. Workspace o repository ID oficial del agente.
4. CWD del proceso.
5. Git root y remote URL.
6. Workspace del editor.
7. Ruta registrada.
8. Sin atribución.

Adaptadores iniciales:

```text
Codex
Claude Code
GitHub Copilot / VS Code
OpenCode
OpenAI-compatible
Anthropic-compatible
Generic CLI wrapper
Manual JSON/NDJSON importer
```

No intentar implementar todos como falsos stubs. Comenzar con el contrato, el resolver central y dos adaptadores verificables.

---

# 10. CLI

Ejecutar `qlog` sin argumentos debe abrir la TUI cuando exista un TTY. En entornos no interactivos debe mostrar ayuda breve y salir sin bloquear.

Jerarquía recomendada:

```text
qlog
├── init
├── tui
├── status
├── doctor
├── verify
├── version
├── config
│   ├── show
│   ├── path
│   ├── set
│   └── validate
├── collector
│   ├── start
│   ├── stop
│   ├── status
│   └── serve
├── project
│   ├── register
│   ├── detect
│   ├── current
│   ├── list
│   ├── show
│   ├── rename
│   ├── link-location
│   ├── unlink-location
│   ├── locations
│   ├── tag
│   │   ├── add
│   │   ├── remove
│   │   └── list
│   └── merge
├── context
│   ├── current
│   ├── list
│   ├── show
│   └── switch
├── task
│   ├── start
│   ├── finish
│   ├── annotate
│   └── list
├── session
│   ├── list
│   ├── show
│   ├── current
│   └── tail
├── usage
│   ├── today
│   ├── week
│   ├── month
│   ├── project
│   ├── model
│   ├── agent
│   ├── tag
│   └── unattributed
├── report
│   ├── summary
│   ├── project
│   ├── compare-projects
│   ├── model
│   ├── agent
│   ├── task-type
│   └── unattributed
├── pricing
│   ├── list
│   ├── show
│   ├── add
│   ├── validate
│   ├── update
│   └── recalculate
├── allocation
│   ├── show
│   ├── split
│   ├── assign
│   └── repair
├── adapter
│   ├── list
│   ├── detect
│   ├── install
│   ├── uninstall
│   ├── status
│   └── test
├── ingest
│   ├── file
│   ├── stdin
│   └── otlp
├── run
├── export
├── completion
├── self-update
└── mcp
    └── serve
```

Ejemplos:

```bash
qlog init
qlog doctor

qlog project register --name QUANTUM_LOG --slug quantum-log --path .
qlog project current
qlog project tag add quantum-log domain=ai-engineering
qlog project tag add quantum-log technology=go

qlog task start --project quantum-log --type build --title "Implement project attribution"
qlog run --project quantum-log --agent custom-agent -- my-agent --flag
QLOG_PROJECT=quantum-log qlog run --agent codex -- codex

qlog adapter detect
qlog adapter install claude --dry-run
qlog adapter install opencode
qlog collector serve --listen 127.0.0.1:4318

qlog usage today --project quantum-log
qlog usage month --group-by project
qlog usage month --group-by project,provider,model
qlog usage month --group-by provider,model,project
qlog usage month --tag environment=work
qlog usage unattributed

qlog report project ngAutoPilot --from 2026-07-01
qlog report compare-projects ngAutoPilot quantum-log
qlog report unattributed --from 2026-07-01

qlog allocation split <model-call-id> quantum-log=6000 ngAutoPilot=4000
qlog session tail --follow
qlog export --format csv --redact-paths
qlog verify
```

Reglas de selección de proyecto:

- `--project` tiene prioridad sobre cualquier inferencia.
- `QLOG_PROJECT` actúa como señal explícita controlada por el usuario.
- `qlog project current` debe mostrar método, evidencia y confianza.
- `qlog context switch` debe cerrar el contexto anterior antes de abrir el siguiente.
- `qlog allocation split` debe validar que la suma sea `10000` basis points.

Todos los comandos deben:

- Tener `--help` útil.
- Soportar salida humana y `--json` cuando corresponda.
- Soportar `--group-by` con dimensiones validadas.
- Tener códigos de salida documentados.
- No escribir decoración ANSI cuando stdout no sea TTY.
- Respetar `NO_COLOR`.
- No mezclar datos y logs de diagnóstico en stdout; usar stderr para diagnóstico.
- No ocultar consumo sin proyecto: mostrarlo como `unattributed`.
- No cambiar atribuciones históricas sin registrar una corrección auditable.

---

# 11. TUI

Construir una TUI profesional, no una demo de neón.

Pantallas:

```text
Overview
Live Trace
Tasks
Sessions
Work Contexts
Projects
Project Detail
Project Tags
Unattributed
Agents
Models
Costs
Pricing
Adapters
Integrity
Settings / Doctor
```

## 11.1 Layout

Header:

```text
QUANTUM_LOG / TRACE CONSOLE                              v0.x.y
```

Resumen:

```text
PROJECTS  06 ACTIVE   AGENTS  04 OK     EVENTS  18,420
TOKENS    2.1M IN / 1.8M OUT             UNATTRIBUTED  0.7%
COST      $12.43 / €11.39   LEDGER: PENDING EVIDENCE   CAPTURE: EXAMPLE ONLY
```

Trace line canónica:

```text
2026-07-16T13:42:18.421Z | quantum-log | CODEX | model.call | gpt-x | in:43262 out:7580 | $0.214 | OK
```

Cada línea debe conservar significado completo al copiarla como texto plano.

## 11.2 Vista Projects

Tabla mínima:

```text
PROJECT          TOKENS      COST EUR    TASKS    LAST ACTIVE
quantum-log      182,420        4.28        12    17 Jul 10:42
ngAutoPilot      421,870        9.84        24    17 Jul 09:58
MultiCopy         64,230        1.12         7    16 Jul 21:14
unattributed       8,441        0.18         2    16 Jul 18:02
```

Detalle de proyecto:

```text
QUANTUM_LOG / PROJECT

IDENTITY
slug        quantum-log
canonical   janpereira-dev/quantum_log

LOCATIONS
C:\Repositorios\quantum_log
/home/jan/repos/quantum_log

USAGE
Today            31,822 tokens
This week       182,420 tokens
This month      611,205 tokens

COST
USD                  $4.65
EUR                   €4.28

MODELS
gpt-x              61.4%
claude-example     27.2%
local-llama        11.4%

CAPTURE (EJEMPLO, NO EVIDENCIA)
exact              84.0%
estimated          13.2%
unattributed        2.8%
```

## 11.3 Vista Work Contexts

Debe permitir observar cambios dentro de una misma sesión:

```text
09:00:00–09:40:14 | ngAutoPilot | C:\repos\ngAutoPilot | exact
09:41:02–10:20:08 | quantum-log | C:\repos\quantum_log | high
10:21:11–10:45:44 | ngAutoPilot | C:\repos\ngAutoPilot | exact
```

## 11.4 Color

Aplicar la semántica de marca:

```text
Terminal / recursos / OK: #00FF66
Agentes / procesos:       #A200FF
Texto violeta accesible:  #B45CFF
Fondo conceptual:         #0B0C10
Texto principal:          #F4F7FA
Texto secundario:         #9AA3AE
Warning:                  #FFB020
Error:                    #FF4D6D
```

Consideraciones:

- No asumir que el terminal permite fijar el fondo.
- Crear paleta TrueColor, paleta ANSI-256 y fallback ANSI-16.
- Usar `#B45CFF`, no el violeta core, para texto pequeño sobre fondo oscuro.
- El color refuerza significado; nunca sustituye `[OK]`, `[WARN]`, `[ERROR]`, iconos o labels.
- Respetar `NO_COLOR` y terminales `TERM=dumb`.
- No utilizar glow persistente en la TUI.
- No intentar imponer JetBrains Mono: documentarla como fuente recomendada, usando la monoespaciada configurada por el terminal.

## 11.5 Accesibilidad y UX

- Navegación completa por teclado.
- Atajos visibles mediante `?`.
- Focus visible.
- Layout responsive desde 80 columnas.
- Vista degradada legible en 60 columnas.
- No bloquear por terminal pequeño; mostrar aviso y modo compacto.
- Animaciones mínimas y desactivables.
- Tablas con alineación tabular.
- Unidades siempre visibles.
- Confirmación para operaciones destructivas.
- Confirmación explícita antes de fusionar proyectos o reasignar consumo histórico.
- `Esc` vuelve; `q` sale; `/` busca; `f` filtra; `r` refresca; `e` exporta.

---

# 12. Persistencia SQLite

Requisitos:

- Base de datos local con WAL.
- Foreign keys activadas.
- Busy timeout configurado.
- Migraciones transaccionales.
- Backups antes de migraciones destructivas.
- No abrir una conexión nueva por cada evento.
- Serializar escrituras o usar una estrategia clara de concurrencia.
- Tests de migración desde cada versión soportada.
- Comando `qlog doctor` que valide integridad con `PRAGMA integrity_check`.

Índices mínimos:

```text
occurred_at
received_at
project_id
project_location_id
work_context_id
host_id
agent_name
provider
model_id
task_type
trace_id
session_id
git_branch
capture_quality
```

Crear consultas o vistas eficientes para:

```text
uso diario por proyecto
uso mensual por proyecto/proveedor/modelo
uso por tag
coste asignado por proyecto
consumo unattributed
cambios de proyecto dentro de una sesión
porcentaje exacto frente a estimado
```

Reglas de consistencia:

- Un `ProjectLocation` debe pertenecer a un único `Project` por instalación, salvo una operación explícita de merge o relink.
- Un `WorkContext` no puede finalizar antes de comenzar.
- Una asignación simple debe ser `10000` basis points.
- Varias asignaciones del mismo subject deben sumar `10000` dentro de la transacción.
- No borrar eventos para corregir atribución; insertar una corrección o actualizar únicamente proyecciones derivadas con auditoría.
- Las migraciones deben preservar el historial de precios y atribuciones.

Rutas:

- Linux: respetar `$XDG_DATA_HOME`, `$XDG_CONFIG_HOME` y `$XDG_STATE_HOME`.
- macOS: respetar convenciones de Application Support cuando corresponda.
- Windows: usar `%LOCALAPPDATA%` o `%APPDATA%` de forma coherente.
- Permitir override con `QLOG_HOME`.

Archivos conceptuales:

```text
config.yaml
qlog.db
logs/
exports/
backups/
pricing/
```

---

# 13. Privacidad y seguridad

Valores por defecto:

```yaml
privacy:
  capturePromptContent: false
  captureResponseContent: false
  captureToolArguments: false
  captureToolResults: false
  captureAbsolutePathLocally: true
  hashPathsOnExport: true
  redactSecrets: true
  redactPII: true
```

Requisitos:

- No capturar secretos, tokens, cabeceras de autorización o variables sensibles.
- Crear un redactor extensible con patrones conocidos y reglas configurables.
- Registrar evidencia de que ocurrió una redacción sin conservar el secreto original.
- Permisos restrictivos del fichero SQLite y configuración.
- Exportaciones sanitizadas por defecto.
- La ruta absoluta puede conservarse localmente para resolver proyectos, pero debe poder ocultarse, hashearse o reemplazarse por alias al exportar.
- `resolution_evidence_json` debe almacenar solo evidencia mínima y sanitizada.
- No exportar nombres de usuario del sistema, home directories o remotes privados en claro por defecto.
- Telemetría del propio QUANTUM_LOG desactivada por defecto y totalmente separada de la telemetría ingerida.
- Ninguna conexión saliente salvo actualización explícita de precios, FX, versión o adaptadores.
- Mostrar claramente cada conexión de red en documentación y `qlog doctor`.
- Generar SBOM de releases.
- Publicar checksums y firmas.
- Ejecutar análisis de vulnerabilidades y dependencias en CI.
- Registrar cambios manuales de atribución como operaciones auditables.
- No usar un fingerprint de host reversible o basado en identificadores sensibles.

---

# 14. Instalación multiplataforma

El producto debe instalarse de forma sencilla en macOS, Linux, WSL y Windows nativo.

## 14.1 Comandos públicos deseados

El repositorio fuente y módulo Go ya están definidos. `<INSTALL_HOST>`, el scope npm y algunos identificadores de package managers continúan como placeholders hasta reservarlos y publicar sus repositorios auxiliares:

```bash
# macOS, Linux y WSL
curl -fsSL https://<INSTALL_HOST>/install.sh | sh

# Windows PowerShell
irm https://<INSTALL_HOST>/install.ps1 | iex

# Windows CMD
curl -fsSL https://<INSTALL_HOST>/install.cmd -o install.cmd && install.cmd && del install.cmd

# Go
GOFLAGS=-buildvcs=true go install github.com/janpereira-dev/quantum_log/cmd/qlog@latest

# npm
npm install -g @quantum-log/cli

# Bun
bun install -g @quantum-log/cli

# Homebrew
brew install janpereira-dev/tap/quantum-log

# Arch / AUR
paru -S quantum-log-bin

# Scoop
scoop bucket add quantum-log https://github.com/janpereira-dev/scoop-bucket
scoop install quantum-log

# WinGet
winget install <PUBLISHER>.QuantumLog
```

## 14.2 Requisitos del instalador

Los scripts deben:

- Detectar OS, arquitectura y libc cuando corresponda.
- Soportar `amd64` y `arm64` para Linux, macOS y Windows.
- Soportar canales `stable`, `latest` y versión fija.
- Descargar desde GitHub Releases o un host oficial configurable.
- Descargar manifest y checksum.
- Verificar SHA-256 antes de instalar.
- Verificar firma cuando esté disponible.
- Instalar sin privilegios de administrador por defecto.
- Ser idempotentes.
- Soportar `--dry-run`.
- Soportar `--version`, `--channel`, `--install-dir` y `--no-modify-path`.
- Crear backup antes de editar archivos de shell.
- Informar exactamente qué archivos modificó.
- Ejecutar `qlog --version` al finalizar.
- Sugerir `qlog doctor`, no ejecutarlo silenciosamente si requiere cambios.
- Ofrecer desinstalador simétrico.

## 14.3 Instalación segura documentada

Además del one-liner, documentar una ruta verificable:

```bash
curl -fsSLO https://<INSTALL_HOST>/install.sh
curl -fsSLO https://<INSTALL_HOST>/install.sh.sha256
sha256sum -c install.sh.sha256
sh install.sh
```

No ocultar que `curl | sh` ejecuta código remoto. La experiencia sencilla no debe eliminar controles de supply chain.

## 14.4 npm y Bun

El paquete npm debe ser un distribuidor fino del binario Go, no una reimplementación del core en JavaScript.

Opciones aceptables:

1. Paquetes opcionales por plataforma y arquitectura.
2. Postinstall que descarga un binario firmado y verifica checksum.

Después de la instalación, `qlog` no debe requerir Node para ejecutarse.

No publicar hasta reservar y verificar el scope/package name.

## 14.5 Package managers

Preparar GoReleaser para generar o publicar:

- GitHub Releases.
- Archives `.tar.gz` y `.zip`.
- Checksums.
- SBOM.
- Homebrew Tap.
- Scoop bucket.
- WinGet manifest.
- AUR package `quantum-log-bin`.
- Paquete npm fino.

Chocolatey, apt, dnf y apk pueden añadirse después del MVP.

## 14.6 Self-update

`qlog self-update` debe conocer el canal de instalación:

- Instalación nativa: puede actualizarse.
- Homebrew: mostrar `brew upgrade`.
- WinGet: mostrar `winget upgrade`.
- Scoop: mostrar `scoop update`.
- npm/Bun: mostrar el comando correspondiente.
- `go install`: explicar que debe volver a ejecutar `go install ...@latest`.

No sobrescribir binarios gestionados por package managers.

---

# 15. GoReleaser y CI/CD

Crear `.goreleaser.yaml` con:

- Binario `qlog`.
- Builds para `darwin`, `linux` y `windows`.
- Arquitecturas `amd64` y `arm64`.
- `CGO_ENABLED=0`.
- `-trimpath`.
- Variables de versión, commit y fecha mediante `-ldflags`.
- Archives correctos: `.zip` para Windows, `.tar.gz` para Unix.
- Checksums SHA-256.
- SBOM.
- Release notes desde changelog.
- Artefactos reproducibles cuando sea viable.
- Firma o attestations de provenance.

GitHub Actions:

```text
ci.yml
release.yml
security.yml
installers.yml
nightly.yml opcional
```

Matriz mínima:

```text
ubuntu-latest
macos-latest
windows-latest
Go stable
```

Validaciones:

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run
govulncheck ./...
gofmt check
go mod tidy check
```

Añadir tests de instalación en contenedores y runners limpios.

---

# 16. Testing

Aplicar TDD en dominio, atribución, pricing, integridad y almacenamiento.

Cobertura requerida por criticidad:

- Project resolver: precedencia, evidencia, confianza, ambigüedad y fallback `unattributed`.
- Project identity: normalización de remote URL, rutas, worktrees y múltiples hosts.
- WorkContext: apertura, cierre y cambio de proyecto dentro de una sesión.
- UsageAllocation: suma de basis points, reparto, corrección y concurrencia.
- Pricing engine: cobertura exhaustiva de ramas y casos límite.
- Hash chain: tests deterministas, manipulación, orden y concurrencia.
- SQLite migrations: tests desde base vacía y upgrades.
- Redacción: tests de secretos, rutas, remotes privados y falsos positivos.
- CLI: tests de comandos, `--group-by`, proyecto explícito y códigos de salida.
- TUI: golden tests de vistas sin depender de color exacto cuando no sea necesario.
- Installers: pruebas de OS/arch, checksum incorrecto, versión inexistente e idempotencia.
- Adaptadores: fixtures versionados y contract tests.

Casos obligatorios:

```text
una sesión trabaja en proyecto A, luego B y luego A
una misma identidad tiene rutas Windows, WSL y macOS
un evento contiene CWD pero no Git
un evento contiene Git root y remote URL
QLOG_PROJECT prevalece sobre inferencias débiles
--project prevalece sobre QLOG_PROJECT
una llamada queda unattributed sin inventar proyecto
una llamada se divide 60/40 entre dos proyectos
las asignaciones 60/50 son rechazadas
una corrección histórica deja evidencia auditable
los group-by project,provider,model y provider,model,project conservan los mismos totales
```

Añadir fuzz tests para:

```text
canonicalización de eventos
normalización de rutas y remotes
resolución de proyectos
cálculo y reparto de costes
parsing de pricing
parsing de importaciones
redacción
```

No perseguir “100%” mediante tests vacíos. Priorizar mutaciones, invariantes, contratos y escenarios multi-proyecto reales.

---

# 17. MCP y skill

El MCP es una capa de integración, no la fuente principal de verdad.

Herramientas futuras:

```text
qlog.register_project
qlog.get_current_project
qlog.switch_project
qlog.add_project_tag
qlog.start_task
qlog.annotate_task
qlog.finish_task
qlog.get_current_session
qlog.get_current_work_context
qlog.get_task_summary
qlog.get_project_summary
qlog.get_unattributed_summary
qlog.assign_usage
qlog.split_usage
qlog.check_budget
qlog.register_custom_model
qlog.verify_ledger
```

Crear una skill `quantum-log-usage-governor` que indique a los agentes:

- Detectar o resolver el proyecto antes de iniciar trabajo relevante.
- Iniciar o asociar una tarea con un proyecto primario.
- Clasificar el tipo de trabajo.
- Mantener correlación durante la sesión.
- Observar cambios de directorio, Git root o workspace.
- Solicitar o ejecutar `switch_project` cuando el contexto cambie.
- Finalizar la tarea y cerrar el `WorkContext`.
- Mostrar un resumen agrupado por proyecto, proveedor y modelo.
- Mostrar explícitamente consumo `unattributed`.
- No inventar proyecto, tokens, costes o modelo.
- Etiquetar estimaciones e inferencias.
- No almacenar contenido sensible.
- No crear tags inconsistentes o duplicados por diferencias de mayúsculas.

Reglas:

- La captura técnica debe seguir funcionando aunque la skill no se cargue.
- La skill mejora la semántica y la atribución explícita; no reemplaza telemetría, hooks ni el resolver central.
- El agente no debe asumir que toda la sesión corresponde al proyecto inicial.
- Una corrección manual debe quedar registrada y no sobrescribir silenciosamente el evento original.

---

# 18. Resultado final de una tarea

El resumen humano debe agrupar por proyecto y mostrar una línea por proveedor/modelo dentro de cada proyecto.

Ejemplo:

```text
AI USAGE · 2026-07-17 · task:qlog_01J...

PROJECT: quantum-log · BUILD
CODEX       | openai/gpt-x      | in 42,180 | out 8,420 | cache 12,300 | tools 18 | 12m41s | $0.84 | EXACT
CLAUDE CODE | anthropic/sonnet  | in  9,110 | out 1,824 | cache      0 | tools  3 |  1m22s | $0.06 | ESTIMATED
PROJECT TOTAL                    | 73,834 tokens | 14m03s | $0.90 / €0.82

PROJECT: ngAutoPilot · REVIEW
CODEX       | openai/gpt-x      | in 18,120 | out 3,110 | cache  4,220 | tools  8 |  5m08s | $0.31 | EXACT
PROJECT TOTAL                    | 25,450 tokens |  5m08s | $0.31 / €0.28

UNATTRIBUTED
OPENCODE    | local/qwen        | 8,441 tokens | €0.00 | INFERRED

GRAND TOTAL | 107,725 tokens | 19m11s | $1.21 / €1.10 | ledger: pending evidence
```

Debe soportar otras vistas sin cambiar los datos almacenados:

```text
--group-by project,provider,model
--group-by provider,model,project
--group-by project,agent,model
--group-by project-tag,project,provider,model
```

La salida JSON debe conservar todos los campos sin redondear e incluir:

```text
group_by
project_id
project_slug
project_location_id
work_context_id
provider
model
usage
cost
allocation
capture_quality
resolution_method
resolution_confidence
unattributed_usage
```

Cuando exista reparto entre proyectos, mostrar tanto consumo observado como coste asignado y evitar duplicar los totales.

---

# 19. README inicial

El README debe incluir:

1. Qué problema resuelve.
2. Qué no hace.
3. Por qué `Project`, `ProjectLocation` y `WorkContext` son conceptos distintos.
4. Cómo atribuye consumo cuando una sesión cambia de proyecto.
5. Captura exacta frente a estimada.
6. Bucket `unattributed` y cómo repararlo.
7. Tags y centros de coste.
8. Captura local-first y privacidad.
9. Instalación por plataforma.
10. Inicio rápido.
11. Registro y detección de proyectos.
12. Captura mediante adaptadores.
13. Captura genérica con `qlog run`.
14. Ejemplos de `--group-by`.
15. Ejemplos de informes por proyecto.
16. Captura de pantalla de la TUI.
17. Arquitectura resumida.
18. Roadmap.
19. Seguridad.
20. Licencia.
21. Estado honesto de cada integración.

No utilizar métricas ficticias sin marcarlas como `DEMO DATA`.

El quick start debe demostrar al menos:

```bash
qlog init
qlog project register --path . --name QUANTUM_LOG
qlog run --project quantum-log -- codex
qlog usage today --group-by project,provider,model
```

---

# 20. Fases de ejecución

No intentes implementar todo en una sola entrega.

## Milestone 0 — Foundation

Entregar:

- ADRs principales.
- Scaffold Go.
- Cobra.
- Configuración.
- Version metadata.
- CI base.
- GoReleaser base.
- README inicial.
- Licencia y documentos de seguridad.
- Contrato de trabajo del orquestador y subagentes.

Subagentes activos:

```text
quantum-log-product-orchestrator
project-attribution-architect
go-sqlite-core-engineer
security-release-guardian
```

## Milestone 1 — Ledger and Project Core

Entregar:

- Dominio base.
- `Host`, `Project`, `ProjectLocation`, `ProjectTag`, `WorkContext` y `UsageAllocation`.
- Resolver de proyecto con precedencia determinista.
- Bucket `unattributed`.
- SQLite CGo-free.
- Migraciones.
- Raw events append-only con referencias de proyecto y contexto.
- Normalización base.
- Cadena hash.
- `qlog init`.
- `qlog doctor`.
- `qlog verify`.
- `qlog project register`.
- `qlog project detect`.
- `qlog project current`.
- Importador NDJSON.
- Tests y fixtures multi-proyecto.

Subagentes activos:

```text
quantum-log-product-orchestrator
project-attribution-architect
go-sqlite-core-engineer
security-release-guardian
```

## Milestone 2 — CLI Reporting and FinOps

Entregar:

- Tareas, sesiones, turns, model calls y tool calls.
- Usage allocations.
- Tags normalizados.
- Queries y agregaciones.
- `--group-by` multidimensional.
- Pricing registry.
- Cost engine.
- Coste asignado por proyecto.
- `usage`, `report`, `project`, `allocation`, `pricing`, `export`.
- JSON output estable.
- Comparación de proyectos y reporte `unattributed`.

Subagentes activos:

```text
quantum-log-product-orchestrator
project-attribution-architect
go-sqlite-core-engineer
finops-pricing-engineer
cli-tui-product-engineer
security-release-guardian
```

## Milestone 3 — TUI

Entregar:

- Overview.
- Live Trace.
- Sessions.
- Work Contexts.
- Projects.
- Project Detail.
- Unattributed.
- Models.
- Costs.
- Integrity.
- Paletas accesibles y fallbacks.

## Milestone 4 — Capture

Entregar:

- OTLP HTTP receiver.
- Generic wrapper.
- Dos adaptadores oficiales verificables.
- Matriz de capacidades.
- Captura de CWD, VCS, workspace y cambios de proyecto cuando la fuente lo permita.
- Instalación idempotente de hooks/plugins.

## Milestone 5 — Distribution

Entregar:

- Releases multiplataforma.
- Install scripts.
- Checksums y SBOM.
- Homebrew.
- Scoop.
- WinGet.
- AUR.
- npm/Bun thin package.
- Tests de instalación.

## Milestone 6 — Agent Integration

Entregar:

- MCP server.
- Skill de gobierno.
- Registro y cambio de proyecto desde agentes.
- Resumen automático por tarea y proyecto.
- Reparación guiada de consumo `unattributed`.
- Budgets y alertas por proyecto y tag.

## Milestone 7 — Future Dashboard

No iniciar hasta que CLI, TUI, atribución, almacenamiento y captura sean estables.

Entregar en el futuro:

- Dashboard web opcional.
- Comparativas históricas.
- Cost allocation por portfolio, cliente o centro de coste.
- Importación multi-host o sincronización opcional.
- Nunca convertir el SaaS en requisito para usar el producto local.

---

# 21. Primera ejecución que debes realizar ahora

En esta primera ejecución implementa únicamente `Milestone 0` y el núcleo compilable de `Milestone 1`.

Activa únicamente:

```text
quantum-log-product-orchestrator
project-attribution-architect
go-sqlite-core-engineer
security-release-guardian
```

Debes:

1. Inspeccionar el repositorio actual antes de modificarlo.
2. Informar qué existe y qué falta.
3. Proponer ADRs breves y concretos.
4. Crear el scaffold con arquitectura limpia.
5. Configurar `qlog --version`, `qlog init`, `qlog doctor` y `qlog verify` como comandos reales.
6. Crear las primeras migraciones SQLite.
7. Implementar `Host`, `Project`, `ProjectLocation`, `WorkContext` y `RawEvent`.
8. Introducir el contrato de `UsageAllocation`, aunque el reparto avanzado pueda completarse en Milestone 2.
9. Implementar `qlog project register`, `qlog project detect` y `qlog project current` con salida humana y JSON.
10. Implementar un resolver mínimo con prioridad: explícito, `QLOG_PROJECT`, CWD, Git root, registered path y `unattributed`.
11. Implementar almacenamiento append-only de `raw_events` y cadena hash mínima.
12. Guardar método, evidencia sanitizada y confianza de la resolución.
13. Crear tests de integridad, migraciones, proyecto y cambio de contexto.
14. Crear un fixture donde una sesión pasa de proyecto A a B y vuelve a A.
15. Configurar CI y GoReleaser.
16. Ejecutar todas las validaciones posibles.
17. No simular resultados de comandos que no se hayan ejecutado.
18. No avanzar a TUI, adaptadores comerciales, pricing completo o dashboard todavía.

Antes de escribir código, entrega:

```text
A. Resumen de decisiones
B. Riesgos
C. ADRs propuestos
D. Árbol de archivos previsto
E. Plan de implementación en pasos pequeños
F. Modelo de datos inicial
G. Estrategia de resolución de proyecto
H. Criterios de aceptación
I. Reparto de responsabilidades entre subagentes
```

Después implementa, prueba y finaliza con:

```text
A. Archivos creados y modificados
B. Decisiones tomadas
C. Contratos de dominio introducidos
D. Comandos ejecutados
E. Resultado real de tests, lint y build
F. Escenarios multi-proyecto validados
G. Deuda técnica consciente
H. Próximo milestone recomendado
I. Riesgos o datos que continúan unattributed
```

---

# 22. Criterios de aceptación del primer milestone

- `go build ./...` finaliza correctamente.
- `go test ./...` finaliza correctamente.
- El binario se llama `qlog`.
- `qlog --version` muestra versión, commit y build date.
- `qlog init` crea configuración y SQLite en rutas correctas para el sistema operativo.
- `qlog init` es idempotente.
- `qlog doctor` no modifica el sistema.
- `qlog doctor --json` produce JSON válido.
- `qlog verify` detecta una cadena íntegra.
- Un test demuestra que la modificación manual de un evento rompe la verificación.
- SQLite funciona con `CGO_ENABLED=0`.
- Las migraciones están embebidas.
- Existen tablas o contratos iniciales para `hosts`, `projects`, `project_locations`, `work_contexts`, `raw_events` y `usage_allocations`.
- `qlog project register --path .` crea o reutiliza un proyecto de forma idempotente.
- `qlog project current` muestra proyecto, ubicación, método y confianza.
- `qlog project current --json` produce JSON estable.
- La ruta absoluta no vive como propiedad permanente de `Project`.
- `git_branch` y `git_commit` pertenecen a `WorkContext`, no a `Project`.
- Un test demuestra que una sesión puede contener proyecto A, luego B y luego A sin mezclar consumos.
- Un test demuestra que la misma identidad puede tener varias `ProjectLocation`.
- `--project` prevalece sobre `QLOG_PROJECT`.
- `QLOG_PROJECT` prevalece sobre inferencias de CWD o Git.
- Una resolución imposible queda como `unattributed` y no inventa un proyecto.
- Cada evento guarda `project_resolution_method`, `project_resolution_confidence` y evidencia sanitizada.
- La suma inválida de asignaciones es rechazada por el contrato o servicio de dominio.
- No se guarda contenido de prompt o respuesta.
- El proyecto tiene README, LICENSE, SECURITY, CONTRIBUTING y CHANGELOG.
- CI cubre Linux, macOS y Windows.
- GoReleaser puede producir snapshot local.
- No existen secretos, binarios ni bases de datos committeados.
- Los ejemplos de datos están marcados como DEMO o fixture.
- Ningún resultado de build, test o lint se presenta como exitoso sin haberse ejecutado.

---

# 23. Reglas de ingeniería

- Preferir diseño simple y contratos estables.
- Mantener dominio independiente de CLI, TUI y SQLite.
- Mantener atribución independiente de proveedores y adaptadores.
- No crear abstracciones sin un segundo caso real.
- No implementar interfaces gigantes.
- No ocultar errores.
- Usar errores tipados cuando aporten decisiones de control.
- Añadir contexto a errores sin duplicar mensajes.
- Usar `context.Context` en I/O y operaciones cancelables.
- Evitar goroutines huérfanas.
- Cerrar recursos correctamente.
- Documentar decisiones irreversibles mediante ADR.
- No introducir dependencias por comodidad sin revisar mantenimiento, licencia y superficie de ataque.
- Mantener una política de dependencias conservadora.
- No utilizar el logo como sustituto de arquitectura o calidad.
- La experiencia debe ser agradable, pero la evidencia y la fiabilidad tienen prioridad.
- Nunca usar `provider/model/project` como identidad persistente; son dimensiones independientes.
- Nunca atribuir un proyecto únicamente por el agente, proveedor o modelo.
- Nunca esconder consumo no atribuido para mejorar artificialmente los porcentajes.
- No modificar tokens originales al repartir coste entre proyectos.
- Toda corrección histórica debe ser auditable.
- Un `Project` es una identidad lógica; una ruta es una `ProjectLocation`; el estado temporal es un `WorkContext`.
- El orquestador debe impedir que varios subagentes editen el mismo contrato simultáneamente sin coordinación.
- Los subagentes deben entregar evidencia de sus resultados, no narrativas de intención.

---

# 24. Definición de éxito

QUANTUM_LOG será exitoso cuando un desarrollador pueda instalarlo en cualquier sistema soportado, activar una integración, trabajar con uno o varios agentes y obtener un registro verificable que explique:

```text
qué ocurrió
cuándo ocurrió
quién lo originó
qué agente participó
qué proveedor y modelo participaron
qué recursos consumió
cuándo se consumieron dentro de cada proyecto
cuánto costó
qué coste fue asignado a cada proyecto
qué parte es exacta
qué parte es estimada
qué parte continúa unattributed
qué herramientas intervinieron
qué proyecto recibió el beneficio
qué ubicación y contexto de trabajo estaban activos
si una sesión cambió entre varios proyectos
qué tags, portfolio o centro de coste corresponden
si el registro sigue íntegro
```

Debe poder responder con los mismos datos, sin duplicarlos ni alterar los totales, tanto en:

```text
project → provider → model
provider → model → project
project-tag → project → provider → model
```

La regla central del producto es:

> Cada llamada de modelo se atribuye al proyecto activo en el momento exacto del consumo, con evidencia y confianza explícitas.

Construye infraestructura de confianza. No construyas solamente una pantalla bonita.

---
## Contexto histórico no verificable

# Snapshot histórico del Master Prompt

El siguiente snapshot conserva requisitos y propuestas históricas; no demuestra
implementación ni verificación de M0-M6:

* **1 orquestador + 6 subagentes especializados**, con activación progresiva por milestone.
* Atribución **project-first** como parte central del producto.
* Nuevas entidades:

  * `Host`
  * `Project`
  * `ProjectLocation`
  * `ProjectTag`
  * `WorkContext`
  * `UsageAllocation`
* Separación correcta entre:

  * Proyecto lógico.
  * Ruta física o worktree.
  * Contexto temporal de trabajo.
  * Consumo observado.
  * Coste calculado.
  * Coste asignado.
* Soporte para una sesión que cambia entre varios proyectos.
* Resolución determinista mediante:

  * `--project`
  * tarea activa
  * adaptador
  * `QLOG_PROJECT`
  * CWD
  * Git root
  * remote URL
  * workspace
  * rutas registradas
  * `unattributed`
* Agrupaciones dinámicas:

  * `project,provider,model`
  * `provider,model,project`
  * `project,agent,model`
  * `project-tag,project,provider,model`
* Reparto excepcional de una llamada entre proyectos mediante **basis points**.
* Nuevos comandos `project`, `context`, `allocation` y reportes `unattributed`.
* TUI con vistas de proyectos, ubicaciones y cambios de contexto.
* Milestones, testing y criterios de aceptación revisados.
* Ruta Go actualizada a:

```text
github.com/janpereira-dev/quantum_log
```

## Archivos

[Descargar QUANTUM_LOG_MASTER_PROMPT.md actualizado](sandbox:/mnt/data/QUANTUM_LOG_MASTER_PROMPT.md)

[Descargar copia versionada QUANTUM_LOG_MASTER_PROMPT_v1.2.md](sandbox:/mnt/data/QUANTUM_LOG_MASTER_PROMPT_v1.2.md)

La regla arquitectónica central quedó fijada así:

> **Cada llamada de modelo se atribuye al proyecto activo en el momento exacto del consumo, con evidencia y confianza explícitas.**

---
Contexto Final:
Sí. Ese cambio no es menor: **la atribución por proyecto debe ser una capacidad central del dominio**, no un filtro añadido al final.

El prompt ya declara que QUANTUM_LOG debe identificar qué proyecto consumió recursos, su ruta y el modelo utilizado.  También define correctamente la jerarquía `Project → Task → Session → Turn → ModelCall`. 

Pero todavía falta resolver bien una cuestión crítica:

> Una sesión, un agente o incluso una misma terminal pueden trabajar alternativamente sobre varios proyectos.

Por tanto, **no basta con asociar el proyecto únicamente a la tarea o a la sesión**.

---

# 1. No crear `proveedor/modelo/proyecto` como identificador

No guardaría algo así:

```text
openai/gpt-5/ngautopilot
anthropic/claude/quantum-log
```

Eso mezcla dimensiones independientes y complica:

* Consultas.
* Cambios de nombre.
* Reutilización de modelos.
* Comparación entre proyectos.
* Etiquetado.
* Costes históricos.
* Migraciones.
* Índices de SQLite.

El almacenamiento correcto es normalizado:

```text
provider_id = openai
model_id = gpt-5
project_id = quantum-log
```

Después, el usuario decide cómo agruparlo:

```bash
qlog usage month --group-by project,provider,model

qlog usage month --group-by provider,model,project

qlog usage month --group-by project,model

qlog usage month --group-by tag,project,provider
```

La estructura del informe puede cambiar; **el dato base no debe cambiar**.

---

# 2. El proyecto debe ser una dimensión de primer nivel

El evento mínimo debería responder:

```text
QUÉ ocurrió
CUÁNDO ocurrió
EN QUÉ proyecto ocurrió
DESDE QUÉ ubicación ocurrió
QUÉ agente lo originó
QUÉ modelo participó
CUÁNTO consumió
CUÁNTO costó
```

Ejemplo:

```text
2026-07-17T09:18:42Z
project: quantum-log
location: C:\Repositorios\quantum_log
agent: codex
provider: openai
model: gpt-5
task: build
input: 42,180
output: 8,420
cost: €0.82
```

Así puedes contestar correctamente:

```text
¿Cuánto consumí hoy en QUANTUM_LOG?

¿Cuánto gastó ngAutoPilot durante julio?

¿Qué modelo consumió más dentro de QUANTUM_LOG?

¿Cuánto costaron las tareas de testing en cada proyecto?

¿Cuándo comenzó a crecer el consumo de este repositorio?

¿Qué proyecto utiliza más tokens de salida?

¿Cuánto consumió el proyecto A mientras también trabajaba en el proyecto B?
```

---

# 3. Corrección importante del esquema actual

Actualmente el prompt coloca estos campos dentro de `Project`:

```text
absolute_path
git_branch
git_commit
```



Eso debe corregirse.

## El problema

Un proyecto puede tener:

* Varias ubicaciones locales.
* Varios worktrees.
* Varias ramas.
* Varios commits.
* Copias en diferentes máquinas.
* Monorepos con varios subproyectos.
* Una ruta distinta en Windows, Linux o WSL.

Por tanto:

```text
Project ≠ ubicación física
Project ≠ checkout
Project ≠ rama actual
```

## Modelo correcto

```text
Project
 ├── ProjectLocation
 │    └── WorkContext
 │         ├── Task
 │         ├── Session
 │         ├── ModelCall
 │         └── ToolCall
 └── ProjectTag
```

---

# 4. Nuevas entidades

## `Project`

Es la identidad lógica y estable.

```text
id
slug
name
canonical_key
description
project_type
repository_url_normalized
default_branch
status
created_at
updated_at
archived_at
```

Ejemplo:

```json
{
  "id": "prj_01JQLOG",
  "slug": "quantum-log",
  "name": "QUANTUM_LOG",
  "canonicalKey": "janpereira-dev/quantum_log",
  "projectType": "go-cli"
}
```

---

## `ProjectLocation`

Representa dónde existe ese proyecto en una máquina concreta.

```text
id
project_id
host_id
absolute_path
path_hash
vcs_root
workspace_root
worktree_name
is_primary
first_seen_at
last_seen_at
created_at
updated_at
```

Ejemplos:

```text
Windows
C:\Repositorios\quantum_log

WSL
/home/jan/repos/quantum_log

macOS
/Users/jan/Code/quantum_log
```

Las tres ubicaciones pueden pertenecer al mismo `project_id`.

---

## `WorkContext`

Representa el contexto temporal desde el que se produjo el consumo.

```text
id
project_id
project_location_id
task_id
session_id
cwd
git_root
git_branch
git_commit
workspace_name
terminal_id
process_id
started_at
finished_at
resolution_method
resolution_confidence
resolution_evidence_json
created_at
```

Aquí sí deben vivir:

```text
git_branch
git_commit
cwd
workspace
```

Porque son datos del momento de ejecución, no propiedades permanentes del proyecto.

---

## `ProjectTag`

No usaría `tags_json` como único mecanismo. Para consultar bien en SQLite, los tags deberían estar normalizados:

```text
id
project_id
tag_key
tag_value
created_at
```

Ejemplos:

```text
owner       = jan
portfolio   = personal
domain      = ai-engineering
technology  = go
product     = quantum-log
cost-center = personal-rnd
environment = work
client      = mapfre
```

Esto permite:

```bash
qlog usage month --tag environment=work
qlog usage month --tag domain=ai-engineering
qlog usage month --tag client=mapfre
qlog report tag cost-center
```

Los tags sirven para clasificar y agregar. **No deben sustituir el `project_id`**, porque los nombres libres acaban degenerando en:

```text
ngautopilot
ngAutoPilot
ng-auto-pilot
NgAutopilot
```

Y después la contabilidad parece hecha por tres departamentos distintos un viernes por la tarde.

---

# 5. Atribución por llamada, no únicamente por sesión

Ahora mismo el prompt indica que cada fila de `model_calls` representa una llamada real y que los informes agregan por proveedor y modelo. 

Debe ampliarse:

```text
Cada ModelCall debe estar asociada directamente a:
- project_id
- project_location_id
- work_context_id
```

Aunque ya pueda inferirse mediante `task_id`, conviene guardar la atribución directa porque:

* Puede haber llamadas sin una tarea iniciada.
* Una sesión puede cambiar de proyecto.
* Un agente puede operar sobre varios repositorios.
* Una tarea podría sobrevivir a un cambio de workspace.
* Facilita índices y consultas.
* Conserva la atribución histórica aunque cambien otras relaciones.

## Nuevos campos de `model_calls`

```text
primary_project_id
project_location_id
work_context_id
project_resolution_method
project_resolution_confidence
project_resolution_evidence_json
```

También añadiría `project_id` y `work_context_id` en:

```text
raw_events
normalized_events
tool_calls
sessions
turns
cost_snapshots
```

No porque todos sean obligatorios, sino para que la correlación pueda sobrevivir incluso cuando falte alguna entidad intermedia.

---

# 6. Cómo detectar automáticamente el proyecto

La resolución debe tener una precedencia determinista.

## Orden recomendado

```text
1. Proyecto indicado explícitamente
2. Tarea activa asociada a un proyecto
3. Proyecto informado oficialmente por el adaptador
4. Current working directory del proceso
5. Git root detectado
6. Remote URL normalizada
7. Workspace del IDE
8. Longest path match contra ProjectLocation
9. Tag o variable QLOG_PROJECT
10. Proyecto desconocido / unattributed
```

Ejemplo:

```bash
qlog task start \
  --project quantum-log \
  --type build \
  --title "Implement SQLite migrations"
```

También:

```bash
QLOG_PROJECT=quantum-log codex
```

O mediante wrapper:

```bash
qlog run \
  --project quantum-log \
  --agent codex \
  -- codex
```

## Regla obligatoria

```text
Nunca atribuir un proyecto a partir del proveedor o del modelo.
```

Que se use Claude, Codex o GPT no dice nada sobre el proyecto trabajado.

---

# 7. Cambio de proyecto dentro de una misma sesión

Ejemplo real:

```text
09:00–09:40  ngAutoPilot
09:41–10:20  quantum-log
10:21–10:45  ngAutoPilot
```

No debe registrarse como una sola sesión asignada completamente a `ngAutoPilot`.

Debe quedar así:

```text
Session: codex_abc123

WorkContext 1
project: ngAutoPilot
09:00–09:40
tokens: 42,000

WorkContext 2
project: quantum-log
09:41–10:20
tokens: 31,000

WorkContext 3
project: ngAutoPilot
10:21–10:45
tokens: 18,000
```

Resultado:

```text
ngAutoPilot   60,000 tokens
quantum-log   31,000 tokens
```

Eso resuelve exactamente el problema que señalas.

---

# 8. Comandos nuevos

Añadiría al CLI:

```text
qlog project
├── register
├── detect
├── current
├── list
├── show
├── rename
├── link-location
├── unlink-location
├── add-tag
├── remove-tag
├── locations
└── merge
```

Ejemplos:

```bash
qlog project register \
  --name QUANTUM_LOG \
  --slug quantum-log \
  --path .

qlog project current

qlog project add-tag quantum-log domain=ai-engineering

qlog project add-tag quantum-log technology=go

qlog usage today --project quantum-log

qlog usage month --group-by project

qlog usage month --group-by project,provider,model

qlog report compare-projects ngautopilot quantum-log

qlog report project quantum-log \
  --from 2026-07-01 \
  --to 2026-07-31

qlog report project quantum-log \
  --group-by day,agent,model
```

---

# 9. TUI: vista de proyectos

La pantalla `Projects` ya está prevista.  Debe mostrar algo más útil que una lista de nombres.

```text
PROJECT          TOKENS      COST EUR    TASKS    LAST ACTIVE
quantum-log      182,420        4.28        12    17 Jul 10:42
ngAutoPilot      421,870        9.84        24    17 Jul 09:58
MultiCopy         64,230        1.12         7    16 Jul 21:14
unattributed       8,441        0.18         2    16 Jul 18:02
```

Detalle:

```text
QUANTUM_LOG / PROJECT

Location
C:\Repositorios\quantum_log

Usage
Today            31,822 tokens
This week       182,420 tokens
This month       611,205 tokens

Cost
USD                  $4.65
EUR                   €4.28

Models
gpt-x              61.4%
claude-example     27.2%
local-llama        11.4%

Tasks
build                  8
test                   4
review                 3
documentation          2
```

---

# 10. Cuántos subagentes crear

No crearía quince subagentes desde el primer día. Eso aumentaría coordinación, duplicidad y consumo justo dentro de una herramienta destinada a medir consumo. La ironía sería impecable, pero la arquitectura no 😄.

## Estructura recomendada: **1 orquestador + 6 subagentes**

| Agente                                   | Responsabilidad                                                                      |
| ---------------------------------------- | ------------------------------------------------------------------------------------ |
| **1. Product Orchestrator**              | Mantiene roadmap, dependencias, ADR, integración y gates                             |
| **2. Project Attribution Architect**     | Proyecto, ubicación, contexto, tags y resolución multi-proyecto                      |
| **3. Go & SQLite Core Engineer**         | Dominio, persistencia, migraciones, concurrencia y hash chain                        |
| **4. Observability & Adapters Engineer** | OTLP, hooks, plugins, eventos y matriz de capacidades                                |
| **5. FinOps & Pricing Engineer**         | Tokens, costes, FX, pricing histórico, presupuestos y agregaciones                   |
| **6. CLI/TUI Product Engineer**          | Cobra, Bubble Tea, accesibilidad, reportes y experiencia de uso                      |
| **7. Security & Release Guardian**       | Privacidad, redacción, integridad, supply chain, GoReleaser y revisión independiente |

Son **seis subagentes especializados más el orquestador**.

## Para Milestone 0 y 1

Activaría solo estos cuatro:

```text
Product Orchestrator
Project Attribution Architect
Go & SQLite Core Engineer
Security & Release Guardian
```

No tiene sentido activar todavía:

```text
Observability & Adapters
FinOps & Pricing
CLI/TUI
```

hasta que el modelo `Project → WorkContext → Event` sea estable.

---

# 11. Bloque exacto para añadir al master prompt

Añadiría este bloque antes de `7.1 Raw events`:

```md
## 7.0 Identidad, contexto y atribución de proyecto

El proyecto es una dimensión de primer nivel, independiente del agente,
proveedor y modelo.

No concatenar `provider/model/project` como identificador persistente.
Guardar estas dimensiones por separado y permitir que los informes decidan
el orden de agrupación:

- project, provider, model
- provider, model, project
- project, agent, model
- tag, project, provider, model

QUANTUM_LOG debe atribuir cada evento, llamada de modelo y llamada de
herramienta al proyecto que recibió el trabajo.

Una sesión de agente puede cambiar de proyecto. Por ello, `session_id`
no puede ser la única fuente de atribución. La atribución debe realizarse
como mínimo a nivel de `ModelCall` y, cuando sea posible, a nivel de
`RawEvent`.

Modelar explícitamente:

Project
ProjectLocation
ProjectTag
WorkContext

Relación:

Project
 ├── ProjectLocation
 │    └── WorkContext
 │         ├── Task
 │         ├── Session
 │         ├── Turn
 │         ├── ModelCall
 │         └── ToolCall
 └── ProjectTag

`Project` representa la identidad lógica estable.

`ProjectLocation` representa una copia, checkout, worktree o ubicación
física del proyecto en una máquina.

`WorkContext` representa el contexto temporal de ejecución e incluye:

- cwd
- git root
- workspace root
- branch
- commit
- host
- terminal
- proceso
- started_at
- finished_at

No guardar `git_branch` y `git_commit` como propiedades permanentes de
`Project`, porque cambian durante la vida del proyecto.

Cada `ModelCall` debe almacenar:

- primary_project_id
- project_location_id
- work_context_id
- project_resolution_method
- project_resolution_confidence
- project_resolution_evidence_json

Orden de resolución del proyecto:

1. Proyecto indicado explícitamente.
2. Tarea activa asociada a un proyecto.
3. Proyecto informado por el adaptador.
4. Current working directory del proceso.
5. Git root.
6. Remote URL normalizada.
7. Workspace del IDE.
8. Longest path match contra ubicaciones registradas.
9. Variable `QLOG_PROJECT` o tag explícito.
10. `unattributed`.

Nunca inferir el proyecto a partir del proveedor, agente o modelo.

Cuando una sesión cambie de directorio, repositorio o workspace, cerrar el
`WorkContext` anterior y abrir uno nuevo. El consumo posterior debe quedar
atribuido al nuevo proyecto.

Los tags complementan al proyecto, pero no sustituyen `project_id`.

Los tags deben usar pares normalizados `key=value`, por ejemplo:

- domain=ai-engineering
- technology=go
- portfolio=personal
- environment=work
- cost-center=research
- client=example

QUANTUM_LOG debe poder responder:

- Cuánto consumió cada proyecto.
- Cuándo se consumieron recursos dentro de un proyecto.
- Qué proveedor y modelo consumió cada proyecto.
- Qué proyectos se trabajaron durante una misma sesión.
- Qué consumo continúa sin atribución.
- Cuánto consumo corresponde a cada tag o centro de coste.
```

---

# 12. Cambio en el resumen final

El prompt ya indica una salida agregada por proveedor, modelo y proyecto.  La ampliaría así:

```text
AI USAGE · 2026-07-17

PROJECT: quantum-log
CODEX  | openai/gpt-x  | in 42,180 | out 8,420 | €0.82 | EXACT
TOTAL  | 50,600 tokens | €0.82

PROJECT: ngAutoPilot
CLAUDE | anthropic/x   | in 31,110 | out 4,824 | €0.63 | EXACT
CODEX  | openai/gpt-x  | in 18,120 | out 3,110 | €0.31 | ESTIMATED
TOTAL  | 57,164 tokens | €0.94

UNATTRIBUTED
OPENCode | local/qwen | 8,441 tokens | €0.00 | INFERRED
```

Y en JSON:

```json
{
  "groupBy": ["project", "provider", "model"],
  "projects": [
    {
      "projectId": "prj_quantum_log",
      "projectName": "QUANTUM_LOG",
      "usage": []
    }
  ],
  "unattributedUsage": []
}
```

---

# Decisión final

El dominio central debe evolucionar de:

```text
Project → Task → Session → ModelCall
```

a:

```text
Project
→ ProjectLocation
→ WorkContext
→ Task / Session / Turn
→ ModelCall / ToolCall
```

Y la regla esencial debe ser:

> **Cada llamada de modelo se atribuye al proyecto activo en el momento exacto del consumo.**

No al último proyecto abierto.
No al proyecto asociado inicialmente a la sesión.
No al proveedor.
No al modelo.
No a un tag ambiguo.

Eso es lo que permitirá que QUANTUM_LOG responda de forma fiable cuánto gastaste en cada proyecto, incluso cuando trabajaste simultáneamente en varios. 🚀

---
contexto Vjieo:
# **QUANTUM_LOG encaja muy bien**

Después de revisar la imagen y el manual, la marca ya no parece un experimento visual: tiene una base bastante sólida para convertirse en producto.

El manual define correctamente el norte: **infraestructura crítica, trazabilidad, precisión, continuidad y control**, no una interfaz decorativa de IA.  También establece una identidad **dark-first orientada a TUI y GUI**, exactamente alineada con el producto que estás planteando. 

## La única advertencia de marca

“Quantum” puede hacer pensar en computación cuántica. No cambiaría el nombre, pero sí fijaría esta definición oficial:

> **Un quantum es la unidad mínima observable de trabajo de IA: una llamada, una transición, un evento o una evidencia registrada.**

Así conviertes una posible confusión en parte del concepto.

---

# Decisiones que fijaría desde ahora

| Elemento                | Nombre                       |
| ----------------------- | ---------------------------- |
| Marca                   | `QUANTUM_LOG`                |
| Repositorio             | `quantum-log`                |
| Binario y CLI           | `qlog`                       |
| Daemon futuro           | `qlogd`                      |
| Base de datos           | `qlog.db`                    |
| Variable raíz           | `QLOG_HOME`                  |
| MCP                     | `qlog mcp serve`             |
| Skill                   | `quantum-log-usage-governor` |
| Paquete npm provisional | `@quantum-log/cli`           |
| AUR                     | `quantum-log-bin`            |

No usaría `quantum_log` como comando principal. Es más largo, menos natural y poco cómodo para trabajar diariamente.

```bash
qlog init
qlog doctor
qlog usage today
qlog session tail --follow
qlog report project ngAutoPilot
qlog verify
```

La marca conserva el underscore; la experiencia de terminal utiliza `qlog`.

---

# Qué añadiría para que sea realmente nuevo

La parte diferencial no debe ser únicamente el contador de tokens.

## 1. Ledger verificable

Cada evento podría encadenarse criptográficamente:

```text
event_hash = SHA-256(canonical_event + previous_event_hash)
```

Y verificarlo con:

```bash
qlog verify
qlog verify --session qlog_01J...
```

Esto aporta una función potente:

> No solo sabemos cuánto se consumió; también podemos comprobar que el histórico no fue alterado.

No es blockchain. Nada de montar una secta Web3 porque tenemos un hash 😄. Es una cadena de integridad local, sencilla y útil.

## 2. Contabilidad consciente de incertidumbre

Cada valor debe indicar su procedencia:

```text
provider_reported
otel_reported
hook_reported
locally_counted
estimated
inferred
manual
unavailable
```

Y su confianza:

```text
exact
high
medium
low
unknown
```

Así QUANTUM_LOG no cae en el error habitual de presentar una estimación local como si fuera la factura oficial del proveedor.

## 3. Matriz de capacidades por adaptador

```text
                MODEL  TOKENS  CACHE  CONTEXT  TOOLS  MCP  COST
Codex            YES     YES    YES      YES     YES  YES   YES
Claude Code      YES     YES    YES      PARTIAL YES  YES   YES
OpenCode         YES     YES    PARTIAL  PARTIAL YES  YES   YES
Generic wrapper  MAYBE   NO     NO       NO      NO   NO    NO
```

Esto genera confianza y evita integraciones de cartón piedra.

## 4. Pricing versionado

Los precios deben tener vigencia:

```yaml
provider: example-provider
modelPattern: example-model-pro
validFrom: 2026-07-01T00:00:00Z
billingMode: token
version: 2026.07.1
```

Los informes históricos conservarán el precio que utilizaron originalmente.

---

# Stack técnico recomendado

## Core

```text
Go
Cobra
Bubble Tea
Lip Gloss
Bubbles
SQLite CGo-free
OpenTelemetry
GoReleaser
GitHub Actions
```

Cobra aporta subcomandos, flags POSIX, ayuda automática y completado para Bash, Zsh, Fish y PowerShell. ([GitHub][1]) Bubble Tea proporciona una arquitectura basada en Elm para aplicaciones terminales simples o complejas, con soporte de teclado, ratón, renderizado y reducción de color. ([GitHub][2])

Para SQLite usaría `modernc.org/sqlite`: es un driver basado en una adaptación **sin CGo**, con soporte documentado para Darwin, Linux y Windows en arquitecturas relevantes. Esto permite generar binarios multiplataforma sin exigir compiladores nativos al usuario. ([pkg.go.dev][3])

---

# TUI: una corrección importante

El manual dice que no se debe reconstruir el símbolo usando texto, Unicode o caracteres de terminal. 

Por tanto, en la TUI mostraría:

```text
QUANTUM_LOG / TRACE CONSOLE                              v0.1.0
```

No intentaría dibujar una Z aproximada con caracteres.

La interfaz puede conservar el sistema cromático:

```text
#00FF66  recursos, tokens, valores y OK
#B45CFF  agentes y procesos en texto pequeño
#0B0C10  fondo conceptual
#F4F7FA  información primaria
#9AA3AE  información secundaria
#FFB020  warning
#FF4D6D  error
```

El manual ya advierte que el violeta principal no tiene contraste suficiente para texto pequeño y propone una variante accesible. Además, exige que éxito y error nunca dependan únicamente del color. 

Ejemplo:

```text
2026-07-16T13:42:18.421Z | CODEX  | model.call | gpt-x  | in:43262 out:7580 | $0.214 | OK
2026-07-16T13:41:02.140Z | CLAUDE | tool.call  | github | duration:842ms    |         | OK
2026-07-16T13:40:17.882Z | ROUTER | retry      | gpt-x  | attempt:2/3      |         | WARN
```

La línea conserva su significado aunque se copie sin colores, como exige el sistema TUI del manual. 

---

# Instalación multiplataforma

Tus ejemplos son correctos como experiencia objetivo. Claude Code y OpenCode emplean actualmente instaladores equivalentes para Unix, PowerShell y CMD, además de Homebrew, npm, Bun, WinGet, Scoop y AUR según el producto. ([Claude Platform Docs][4])

Los comandos de QUANTUM_LOG quedarían así —con dominios y registros provisionales—:

```bash
# macOS, Linux y WSL
curl -fsSL https://<INSTALL_HOST>/install.sh | sh

# Windows PowerShell
irm https://<INSTALL_HOST>/install.ps1 | iex

# Windows CMD
curl -fsSL https://<INSTALL_HOST>/install.cmd -o install.cmd && install.cmd && del install.cmd

# Go
go install github.com/<OWNER>/quantum-log/cmd/qlog@latest

# npm
npm install -g @quantum-log/cli

# Bun
bun install -g @quantum-log/cli

# Homebrew
brew install <OWNER>/tap/quantum-log

# Arch Linux / AUR
paru -S quantum-log-bin

# Scoop
scoop bucket add quantum-log https://github.com/<OWNER>/scoop-bucket
scoop install quantum-log

# WinGet
winget install <PUBLISHER>.QuantumLog
```

GoReleaser puede centralizar builds, archivos comprimidos, checksums y publicación para Homebrew, npm, WinGet, AUR y Scoop, reduciendo muchísimo la ingeniería manual de releases. ([goreleaser.com][5])

## Seguridad del instalador

Tendría el one-liner bonito, pero también una instalación verificable:

```bash
curl -fsSLO https://<INSTALL_HOST>/install.sh
curl -fsSLO https://<INSTALL_HOST>/install.sh.sha256
sha256sum -c install.sh.sha256
sh install.sh
```

Claude Code, por ejemplo, publica manifiestos con checksums y firmas para verificar sus binarios. Ese debe ser el estándar de QUANTUM_LOG, no simplemente “confía en este `curl | bash` y que Dios reparta suerte”. ([Claude Platform Docs][4])

---

# Prompt maestro preparado

He creado un prompt completo que incluye:

* Arquitectura Go.
* CLI y árbol de comandos.
* TUI.
* SQLite y migraciones.
* Ledger append-only.
* Cadena hash verificable.
* Pricing Registry.
* Modelos comerciales, locales y corporativos.
* Adaptadores.
* OpenTelemetry.
* Privacidad.
* Instaladores Unix, PowerShell y CMD.
* npm, Bun, Homebrew, WinGet, Scoop y AUR.
* GoReleaser.
* CI/CD.
* Testing.
* MCP.
* Skill de gobierno.
* Roadmap por milestones.
* Criterios de aceptación.
* Instrucción para que el agente no intente fabricar todo de golpe.

## 📄 Descarga

[Descargar el Master Build Prompt de QUANTUM_LOG](sandbox:/mnt/data/QUANTUM_LOG_MASTER_PROMPT.md)

La instrucción final del prompt obliga al agente a comenzar únicamente por:

```text
Milestone 0 — Foundation
Milestone 1 — Ledger Core
```

Eso evita el error clásico de pedirle en una sola ejecución la TUI, seis adaptadores, veinte instaladores y un dashboard, para recibir a cambio un castillo precioso construido con espaguetis. La base correcta es:

```text
Go compilable
→ dominio
→ SQLite
→ eventos append-only
→ hash chain
→ CLI
→ tests
→ CI y releases
→ TUI
→ adaptadores
→ distribución
```

[1]: https://github.com/spf13/cobra "GitHub - spf13/cobra: A Commander for modern Go CLI interactions · GitHub"
[2]: https://github.com/charmbracelet/bubbletea "GitHub - charmbracelet/bubbletea: A powerful little TUI framework  · GitHub"
[3]: https://pkg.go.dev/modernc.org/sqlite "sqlite package - modernc.org/sqlite - Go Packages"
[4]: https://docs.anthropic.com/en/docs/claude-code/setup "Advanced setup - Claude Code Docs"
[5]: https://goreleaser.com/customization/homebrew/ "Homebrew Casks – GoReleaser"
---
Antiguo Contexto:# MASTER BUILD PROMPT — QUANTUM_LOG

## Rol

Actúa como un equipo senior compuesto por:

- Principal Go Architect.
- Staff Engineer especializado en CLI y TUI multiplataforma.
- Ingeniero de observabilidad y OpenTelemetry para sistemas de IA generativa.
- Arquitecto de datos SQLite local-first.
- Ingeniero de seguridad, privacidad y supply chain.
- Release Engineer especializado en binarios Go, GoReleaser, GitHub Releases e instaladores multiplataforma.
- Product Engineer con criterio fuerte de usabilidad, accesibilidad y experiencia de desarrollador.

No te limites a generar un prototipo visual. Construye una base de producto mantenible, auditable, extensible y preparada para distribución pública. La idea es que nosotros vamos a generar todo esto para que otros devs, en sus entornos pueda usar todo esto

---

# 1. Nombre y contrato de marca

La marca del producto es:

```text
QUANTUM_LOG
```

Reglas obligatorias:

- En comunicación, documentación, encabezados y marca, escribir siempre `QUANTUM_LOG`, en mayúsculas y con underscore.
- El repositorio debe llamarse `quantum-log`. ya esta el repo creado
- El ejecutable principal y comando de terminal debe llamarse `qlog`.
- El daemon, cuando exista, debe llamarse `qlogd`, pero el MVP debe priorizar un único binario `qlog` con subcomandos.
- El módulo Go debe usar una ruta configurable como `https://github.com/janpereira-dev/quantum_log`.
- La carpeta local de datos debe respetar XDG y las convenciones de cada sistema operativo; no fijar rutas Unix en Windows.
- No recrear el isotipo mediante caracteres ASCII o Unicode. En terminales sin soporte gráfico, mostrar el wordmark `QUANTUM_LOG`, no una imitación textual del logo.
- El producto no está relacionado con computación cuántica. Definir “quantum” como la unidad mínima observable de trabajo de IA: una llamada, evento, transición o evidencia registrada.

Tagline principal:

```text
Trace every agent. Trust every event.
```

Descripción corta:

```text
Local-first observability and FinOps for AI coding agents.
```

Descripción funcional:

```text
QUANTUM_LOG registra, normaliza, atribuye y audita el uso de modelos de IA, agentes, herramientas y tareas de desarrollo, sin depender de un único proveedor.
```

---

# 2. Misión del producto

Construir una herramienta open source, local-first y agnóstica de proveedor que permita a un desarrollador o equipo conocer:

- Qué proyecto consumió recursos de IA.
- La ubicación absoluta local del proyecto.
- Qué agente originó la actividad.
- Qué proveedor y modelo se utilizó en cada llamada.
- Cuánto tiempo duró la tarea, sesión, turno y llamada.
- Tamaño del prompt de entrada y respuesta de salida.
- Tokens de entrada, salida, reasoning, caché y escritura de caché.
- Porcentaje real de contexto utilizado cuando pueda calcularse con datos fiables.
- Tipo de tarea: plan, research, design, build, refactor, debug, test, review, documentation, migration, upgrade, security, deploy u other.
- Número de llamadas de modelo, herramientas, MCP, subagentes, reintentos y errores.
- Coste estimado en USD y EUR.
- Coste real facturado cuando pueda importarse de una fuente oficial.
- Calidad y procedencia de cada dato: exacto, estimado, inferido, manual o no disponible.
- Resultado de la tarea y relación entre coste y trabajo útil.

El producto debe poder responder preguntas como:

```text
¿Cuánto gasté hoy usando Codex, Claude Code, GitHub Copilot y OpenCode?
¿Qué proyecto consumió más tokens este mes?
¿Qué modelo genera más reintentos?
¿Qué tipo de tarea tiene mayor coste medio?
¿Cuánto consumo fue atendido desde caché?
¿Qué porcentaje del coste provino de tokens de salida?
¿Qué sesiones tuvieron presión de contexto elevada?
¿Qué registros son exactos y cuáles son estimaciones?
¿El ledger local ha sido alterado?
```

---

# 3. Diferenciadores obligatorios

QUANTUM_LOG no debe ser un contador superficial de tokens. Debe diferenciarse mediante estos pilares:

## 3.1 Ledger verificable

Mantener un almacén append-only de eventos normalizados y una cadena hash por fuente o sesión:

```text
event_hash = SHA-256(canonical_event + previous_event_hash)
```

Crear:

```bash
qlog verify
qlog verify --session <id>
qlog verify --from 2026-07-01
```

La cadena hash es una prueba local de integridad, no una blockchain. No introducir criptomonedas, consenso distribuido ni complejidad innecesaria.

## 3.2 Contabilidad consciente de incertidumbre

Nunca presentar estimaciones como datos oficiales. Cada métrica debe almacenar:

```text
capture_source
capture_quality
confidence
```

Valores de `capture_source`:

```text
provider_reported
otel_reported
sdk_reported
hook_reported
locally_counted
estimated
inferred
manual
unavailable
```

Valores de `confidence`:

```text
exact
high
medium
low
unknown
```

## 3.3 Costes con vigencia temporal

Los precios deben estar versionados y tener fecha de entrada en vigor. Una modificación futura del precio de un modelo no puede alterar silenciosamente informes históricos.

## 3.4 Local-first y privacy-first

No enviar datos a servidores externos por defecto. No guardar contenido de prompts o respuestas por defecto. Registrar tamaños, hashes y métricas suficientes para observabilidad sin capturar código o información sensible.

## 3.5 Agnosticismo real

No diseñar el dominio alrededor de OpenAI, Anthropic, Google, GitHub o un único agente. Los proveedores comerciales, modelos locales y modelos corporativos deben entrar mediante el mismo contrato de adaptador.

---

# 4. No objetivos

No construir en el MVP:

- Una bóveda de tarjetas bancarias. nunca se va a hacer esto.
- Un sistema para autorizar compras de agentes. no es esto
- Una pasarela de pagos. tampoco nada de esto
- Un proxy obligatorio para todas las llamadas de IA. 
- Una plataforma SaaS obligatoria.
- Una blockchain.
- Un sistema que capture el contenido completo de prompts por defecto.
- Un dashboard web antes de que CLI, TUI, almacenamiento y captura sean sólidos. no vamos a llegar ahorita, a un futuro si
- Integraciones falsas que simulen conocer tokens cuando el proveedor no los expone. nada a futuro, solo lo realizado

---

# 5. Stack técnico obligatorio

## 5.1 Núcleo

- Go como lenguaje principal.
- Un único binario multiplataforma llamado `qlog`.
- `CGO_ENABLED=0` siempre que las dependencias lo permitan.
- Cobra para la jerarquía de comandos CLI.
- Bubble Tea para la arquitectura TUI.
- Lip Gloss para layout y estilos.
- Bubbles para componentes reutilizables de TUI.
- SQLite mediante un driver CGo-free, preferentemente `modernc.org/sqlite`.
- `database/sql` como abstracción base.
- Migraciones SQL embebidas con `go:embed`.
- `log/slog` para logging estructurado interno.
- OpenTelemetry para trazas, métricas y recepción de eventos GenAI cuando sea posible.
- Configuración YAML o TOML, validada y con schema versionado.
- JSON y NDJSON para importación y exportación.

## 5.2 Restricciones

- El núcleo no puede requerir Node.js, Python, Docker ni una JVM.
- Node/Bun solo podrán utilizarse como canales opcionales de distribución o para plugins específicos.
- No usar una ORM pesada.
- No usar floats para dinero.
- No almacenar costes como `REAL` sin control de precisión.
- No acoplar la interfaz TUI con la lógica de negocio.
- No mezclar acceso SQLite directamente dentro de comandos Cobra o modelos Bubble Tea.
- Los modelos son todos los actuales, pero vamos a jugar con algun usuario quiere meter su propias LLM o llamadas a sus LLM propios, lo puede hacer

## 5.3 Dinero y precisión

Almacenar importes monetarios como enteros en micros de moneda:

```text
1 USD = 1_000_000 micros
1 EUR = 1_000_000 micros
```

Usar `int64` para tokens, tamaños, duraciones e importes. Guardar tasas FX como decimal canónico o entero escalado, junto con fuente y fecha.

---

# 6. Arquitectura del repositorio

Crear una arquitectura inicial similar a:

```text
quantum-log/
├── cmd/
│   └── qlog/
│       └── main.go
├── internal/
│   ├── app/
│   ├── audit/
│   ├── cli/
│   ├── config/
│   ├── domain/
│   ├── storage/
│   │   ├── sqlite/
│   │   └── migrations/
│   ├── ingest/
│   │   ├── otlp/
│   │   ├── jsonl/
│   │   └── manual/
│   ├── adapters/
│   │   ├── registry/
│   │   ├── generic/
│   │   ├── codex/
│   │   ├── claude/
│   │   ├── copilot/
│   │   └── opencode/
│   ├── pricing/
│   ├── fx/
│   ├── privacy/
│   ├── reports/
│   ├── tui/
│   ├── update/
│   └── version/
├── pkg/
│   └── sdk/
├── schemas/
│   ├── config.schema.json
│   ├── event.schema.json
│   └── pricing.schema.json
├── pricing/
│   ├── providers/
│   └── examples/
├── adapters/
│   ├── opencode-plugin/
│   ├── claude-hooks/
│   └── vscode-extension/
├── skills/
│   └── quantum-log-usage-governor/
│       └── SKILL.md
├── installers/
│   ├── install.sh
│   ├── install.ps1
│   ├── install.cmd
│   ├── uninstall.sh
│   └── uninstall.ps1
├── packaging/
│   ├── npm/
│   ├── homebrew/
│   ├── scoop/
│   ├── winget/
│   └── aur/
├── docs/
│   ├── architecture/
│   ├── adapters/
│   ├── privacy/
│   ├── pricing/
│   └── releases/
├── .github/
│   └── workflows/
├── .goreleaser.yaml
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
├── README.md
├── SECURITY.md
├── CONTRIBUTING.md
└── CHANGELOG.md
```

No crear carpetas vacías sin sentido. Cada carpeta introducida debe tener una responsabilidad documentada.

---

# 7. Modelo de dominio

Modelar explícitamente:

```text
Project
Task
Session
Turn
ModelCall
ToolCall
RawEvent
NormalizedEvent
Agent
Adapter
PricingRule
CostSnapshot
FxRate
Budget
ExportJob
```

Relación principal:

```text
Project
 └── Task
      └── Session
           └── Turn
                ├── ModelCall
                └── ToolCall
```

## 7.1 Raw events

Mantener una tabla `raw_events` append-only que permita re-normalizar datos cuando mejoren los adaptadores.

Campos mínimos:

```text
id
source_event_id
schema_version
adapter_id
source
source_version
event_type
occurred_at
received_at
trace_id
span_id
parent_span_id
project_id
task_id
session_id
payload_json_sanitized
capture_source
capture_quality
confidence
previous_event_hash
event_hash
created_at
```

Aplicar una restricción única para evitar duplicados por `adapter_id + source_event_id` cuando exista identificador de origen.

## 7.2 Projects

Campos mínimos:

```text
id
name
absolute_path
path_hash
repository_url
git_branch
git_commit
vcs_provider
created_at
updated_at
```

La ruta absoluta puede almacenarse localmente. En exportaciones, debe poder sustituirse por hash, alias o ruta relativa.

## 7.3 Tasks

Campos mínimos:

```text
id
project_id
title
task_type
status
started_at
finished_at
duration_ms
result
human_outcome
tags_json
created_at
updated_at
```

Tipos permitidos:

```text
plan
research
design
build
refactor
debug
test
review
documentation
migration
upgrade
security
deploy
other
```

## 7.4 Sessions y turns

Guardar agente, proceso, terminal, host, versión, correlación y tiempos. No asumir que una sesión utiliza un único modelo.

## 7.5 Model calls

Campos mínimos:

```text
id
task_id
session_id
turn_id
trace_id
span_id
started_at
finished_at
duration_ms
agent_name
agent_version
provider
model_id
model_version
input_tokens
output_tokens
reasoning_tokens
cached_input_tokens
cache_write_tokens
total_tokens
input_chars
output_chars
input_bytes
output_bytes
context_window_tokens
context_used_tokens
context_used_percent_basis_points
tool_calls_count
mcp_calls_count
subagent_calls_count
retry_count
billing_mode
pricing_rule_id
estimated_cost_usd_micros
estimated_cost_eur_micros
actual_cost_usd_micros
actual_cost_eur_micros
fx_rate_id
capture_source
capture_quality
confidence
success
error_type
created_at
```

Cada fila representa una llamada real a un único modelo. Los informes pueden agregar por proveedor y modelo.

No calcular porcentaje de contexto como `tokens acumulados / context window`. Solo calcularlo cuando exista una estimación fiable de los tokens actualmente presentes en la ventana del modelo.

## 7.6 Tool calls

Campos mínimos:

```text
id
model_call_id
task_id
session_id
tool_name
tool_type
mcp_server
started_at
finished_at
duration_ms
success
input_size_bytes
output_size_bytes
error_type
capture_quality
created_at
```

## 7.7 Cost snapshots

Separar el consumo del cálculo financiero. Guardar cada cálculo con:

```text
pricing_rule_id
pricing_catalog_version
calculation_formula_version
fx_rate
fx_source
fx_date
calculated_at
estimated_cost
actual_cost
allocated_cost
```

Esto permitirá recalcular sin destruir el valor histórico original.

---

# 8. Pricing Registry

Crear un registro extensible de precios cargado desde archivos YAML o JSON.

Ejemplo:

```yaml
schemaVersion: 1
provider: example-provider
modelPattern: example-model-pro
validFrom: 2026-07-01T00:00:00Z
validUntil: null
billingMode: token
currency: USD
unitTokens: 1000000
prices:
  inputMicros: 3000000
  cachedInputMicros: 750000
  cacheWriteMicros: 0
  outputMicros: 15000000
  reasoningMicros: 0
source:
  type: manual
  reference: provider-pricing-page
  checkedAt: 2026-07-16T00:00:00Z
version: 2026.07.1
```

Modos de facturación soportados:

```text
token
request
premium_request
session
subscription
seat
local_compute
fixed
custom_formula
unknown
```

Para modelos locales, permitir coste cero de API y una fórmula opcional por duración, GPU, CPU o electricidad.

Comandos:

```bash
qlog pricing list
qlog pricing show <provider/model>
qlog pricing add <file>
qlog pricing validate <file>
qlog pricing update
qlog pricing recalculate --from <date> --to <date>
```

Nunca sobrescribir reglas históricas. Crear nuevas versiones efectivas.

---

# 9. Contrato de adaptadores

Definir una interfaz de adaptador estable. Cada adaptador debe declarar capacidades, no solo un nombre.

Ejemplo conceptual:

```go
type Capabilities struct {
    ModelIdentity     bool
    InputTokens       bool
    OutputTokens      bool
    ReasoningTokens   bool
    CacheTokens       bool
    ContextUsage      bool
    ToolCalls         bool
    MCPCalls          bool
    Costs             bool
    PromptSizes       bool
    ResponseSizes     bool
    SessionLifecycle  bool
    TaskMetadata      bool
}
```

Cada adaptador debe implementar:

```text
ID
Name
Version
Detect
Capabilities
Install
Uninstall
HealthCheck
Ingest
Normalize
```

Reglas:

- `Detect` no puede modificar archivos.
- `Install` debe soportar `--dry-run`.
- Antes de modificar configuraciones de un agente, crear copia de seguridad.
- Las modificaciones deben ser idempotentes.
- Nunca afirmar compatibilidad total cuando solo se capturan tiempo y proceso.
- Mostrar una matriz de capacidades en `qlog adapter list`.

Fuentes de captura, por prioridad:

1. Telemetría oficial del proveedor o agente.
2. OpenTelemetry GenAI.
3. SDK oficial instrumentado.
4. Hooks oficiales.
5. Plugin específico.
6. Importación de logs estructurados.
7. Wrapper genérico de proceso.
8. Estimación local claramente etiquetada.

Adaptadores iniciales:

```text
Codex
Claude Code
GitHub Copilot / VS Code
OpenCode
OpenAI-compatible
Anthropic-compatible
Generic CLI wrapper
Manual JSON/NDJSON importer
```

No intentar implementar todos como falsos stubs. Comenzar con el contrato y dos adaptadores verificables.

---

# 10. CLI

Ejecutar `qlog` sin argumentos debe abrir la TUI cuando exista un TTY. En entornos no interactivos debe mostrar ayuda breve y salir sin bloquear.

Jerarquía recomendada:

```text
qlog
├── init
├── tui
├── status
├── doctor
├── verify
├── version
├── config
│   ├── show
│   ├── path
│   ├── set
│   └── validate
├── collector
│   ├── start
│   ├── stop
│   ├── status
│   └── serve
├── task
│   ├── start
│   ├── finish
│   ├── annotate
│   └── list
├── session
│   ├── list
│   ├── show
│   ├── current
│   └── tail
├── usage
│   ├── today
│   ├── week
│   ├── month
│   ├── project
│   ├── model
│   └── agent
├── report
│   ├── summary
│   ├── project
│   ├── model
│   ├── agent
│   └── task-type
├── pricing
│   ├── list
│   ├── show
│   ├── add
│   ├── validate
│   ├── update
│   └── recalculate
├── adapter
│   ├── list
│   ├── detect
│   ├── install
│   ├── uninstall
│   ├── status
│   └── test
├── ingest
│   ├── file
│   ├── stdin
│   └── otlp
├── run
├── export
├── completion
├── self-update
└── mcp
    └── serve
```

Ejemplos:

```bash
qlog init
qlog doctor
qlog adapter detect
qlog adapter install claude --dry-run
qlog adapter install opencode
qlog collector serve --listen 127.0.0.1:4318
qlog run --agent custom-agent -- my-agent --flag
qlog usage today
qlog report project ngAutoPilot --from 2026-07-01
qlog session tail --follow
qlog export --format csv --redact-paths
qlog verify
```

Todos los comandos deben:

- Tener `--help` útil.
- Soportar salida humana y `--json` cuando corresponda.
- Tener códigos de salida documentados.
- No escribir decoración ANSI cuando stdout no sea TTY.
- Respetar `NO_COLOR`.
- No mezclar datos y logs de diagnóstico en stdout; usar stderr para diagnóstico.

---

# 11. TUI

Construir una TUI profesional, no una demo de neón.

Pantallas:

```text
Overview
Live Trace
Tasks
Sessions
Projects
Agents
Models
Costs
Pricing
Adapters
Integrity
Settings / Doctor
```

## 11.1 Layout

Header:

```text
QUANTUM_LOG / TRACE CONSOLE                              v0.x.y
```

Resumen:

```text
AGENTS  04 OK     EVENTS  18,420     TOKENS  2.1M IN / 1.8M OUT
COST    $12.43 / €11.39   LEDGER: PENDING EVIDENCE   CAPTURE: EXAMPLE ONLY
```

Trace line canónica:

```text
2026-07-16T13:42:18.421Z | CODEX | model.call | gpt-x | in:43262 out:7580 | $0.214 | OK
```

Cada línea debe conservar significado completo al copiarla como texto plano.

## 11.2 Color

Aplicar la semántica de marca:

```text
Terminal / recursos / OK: #00FF66
Agentes / procesos:       #A200FF
Texto violeta accesible:  #B45CFF
Fondo conceptual:         #0B0C10
Texto principal:          #F4F7FA
Texto secundario:         #9AA3AE
Warning:                  #FFB020
Error:                    #FF4D6D
```

Consideraciones:

- No asumir que el terminal permite fijar el fondo.
- Crear paleta TrueColor, paleta ANSI-256 y fallback ANSI-16.
- Usar `#B45CFF`, no el violeta core, para texto pequeño sobre fondo oscuro.
- El color refuerza significado; nunca sustituye `[OK]`, `[WARN]`, `[ERROR]`, iconos o labels.
- Respetar `NO_COLOR` y terminales `TERM=dumb`.
- No utilizar glow persistente en la TUI.
- No intentar imponer JetBrains Mono: documentarla como fuente recomendada, usando la monoespaciada configurada por el terminal.

## 11.3 Accesibilidad y UX

- Navegación completa por teclado.
- Atajos visibles mediante `?`.
- Focus visible.
- Layout responsive desde 80 columnas.
- Vista degradada legible en 60 columnas.
- No bloquear por terminal pequeño; mostrar aviso y modo compacto.
- Animaciones mínimas y desactivables.
- Tablas con alineación tabular.
- Unidades siempre visibles.
- Confirmación para operaciones destructivas.
- `Esc` vuelve; `q` sale; `/` busca; `f` filtra; `r` refresca; `e` exporta.

---

# 12. Persistencia SQLite

Requisitos:

- Base de datos local con WAL.
- Foreign keys activadas.
- Busy timeout configurado.
- Migraciones transaccionales.
- Backups antes de migraciones destructivas.
- Índices para fechas, proyecto, agente, modelo, task type, trace ID y session ID.
- Consultas agregadas eficientes para dashboard diario y mensual.
- No abrir una conexión nueva por cada evento.
- Serializar escrituras o usar una estrategia clara de concurrencia.
- Tests de migración desde cada versión soportada.
- Comando `qlog doctor` que valide integridad con `PRAGMA integrity_check`.

Rutas:

- Linux: respetar `$XDG_DATA_HOME`, `$XDG_CONFIG_HOME` y `$XDG_STATE_HOME`.
- macOS: respetar convenciones de Application Support cuando corresponda.
- Windows: usar `%LOCALAPPDATA%` o `%APPDATA%` de forma coherente.
- Permitir override con `QLOG_HOME`.

Archivos conceptuales:

```text
config.yaml
qlog.db
logs/
exports/
backups/
pricing/
```

---

# 13. Privacidad y seguridad

Valores por defecto:

```yaml
privacy:
  capturePromptContent: false
  captureResponseContent: false
  captureToolArguments: false
  captureToolResults: false
  captureAbsolutePathLocally: true
  hashPathsOnExport: true
  redactSecrets: true
  redactPII: true
```

Requisitos:

- No capturar secretos, tokens, cabeceras de autorización o variables sensibles.
- Crear un redactor extensible con patrones conocidos y reglas configurables.
- Registrar evidencia de que ocurrió una redacción sin conservar el secreto original.
- Permisos restrictivos del fichero SQLite y configuración.
- Exportaciones sanitizadas por defecto.
- Telemetría del propio QUANTUM_LOG desactivada por defecto y totalmente separada de la telemetría ingerida.
- Ninguna conexión saliente salvo actualización explícita de precios, FX, versión o adaptadores.
- Mostrar claramente cada conexión de red en documentación y `qlog doctor`.
- Generar SBOM de releases.
- Publicar checksums y firmas.
- Ejecutar análisis de vulnerabilidades y dependencias en CI.

---

# 14. Instalación multiplataforma

El producto debe instalarse de forma sencilla en macOS, Linux, WSL y Windows nativo.

## 14.1 Comandos públicos deseados

Los siguientes dominios y nombres son placeholders hasta confirmar dominio, organización y registros:

```bash
# macOS, Linux y WSL
curl -fsSL https://<INSTALL_HOST>/install.sh | sh

# Windows PowerShell
irm https://<INSTALL_HOST>/install.ps1 | iex

# Windows CMD
curl -fsSL https://<INSTALL_HOST>/install.cmd -o install.cmd && install.cmd && del install.cmd

# Go
GOFLAGS=-buildvcs=true go install github.com/janpereira-dev/quantum_log/cmd/qlog@latest

# npm
npm install -g @quantum-log/cli

# Bun
bun install -g @quantum-log/cli

# Homebrew
brew install <OWNER>/tap/quantum-log

# Arch / AUR
paru -S quantum-log-bin

# Scoop
scoop bucket add quantum-log https://github.com/janpereira-dev/quantum_log/scoop-bucket
scoop install quantum-log

# WinGet
winget install <PUBLISHER>.QuantumLog
```

## 14.2 Requisitos del instalador

Los scripts deben:

- Detectar OS, arquitectura y libc cuando corresponda.
- Soportar `amd64` y `arm64` para Linux, macOS y Windows.
- Soportar canales `stable`, `latest` y versión fija.
- Descargar desde GitHub Releases o un host oficial configurable.
- Descargar manifest y checksum.
- Verificar SHA-256 antes de instalar.
- Verificar firma cuando esté disponible.
- Instalar sin privilegios de administrador por defecto.
- Ser idempotentes.
- Soportar `--dry-run`.
- Soportar `--version`, `--channel`, `--install-dir` y `--no-modify-path`.
- Crear backup antes de editar archivos de shell.
- Informar exactamente qué archivos modificó.
- Ejecutar `qlog --version` al finalizar.
- Sugerir `qlog doctor`, no ejecutarlo silenciosamente si requiere cambios.
- Ofrecer desinstalador simétrico.

## 14.3 Instalación segura documentada

Además del one-liner, documentar una ruta verificable:

```bash
curl -fsSLO https://<INSTALL_HOST>/install.sh
curl -fsSLO https://<INSTALL_HOST>/install.sh.sha256
sha256sum -c install.sh.sha256
sh install.sh
```

No ocultar que `curl | sh` ejecuta código remoto. La experiencia sencilla no debe eliminar controles de supply chain.

## 14.4 npm y Bun

El paquete npm debe ser un distribuidor fino del binario Go, no una reimplementación del core en JavaScript.

Opciones aceptables:

1. Paquetes opcionales por plataforma y arquitectura.
2. Postinstall que descarga un binario firmado y verifica checksum.

Después de la instalación, `qlog` no debe requerir Node para ejecutarse.

No publicar hasta reservar y verificar el scope/package name.

## 14.5 Package managers

Preparar GoReleaser para generar o publicar:

- GitHub Releases.
- Archives `.tar.gz` y `.zip`.
- Checksums.
- SBOM.
- Homebrew Tap.
- Scoop bucket.
- WinGet manifest.
- AUR package `quantum-log-bin`.
- Paquete npm fino.

Chocolatey, apt, dnf y apk pueden añadirse después del MVP.

## 14.6 Self-update

`qlog self-update` debe conocer el canal de instalación:

- Instalación nativa: puede actualizarse.
- Homebrew: mostrar `brew upgrade`.
- WinGet: mostrar `winget upgrade`.
- Scoop: mostrar `scoop update`.
- npm/Bun: mostrar el comando correspondiente.
- `go install`: explicar que debe volver a ejecutar `go install ...@latest`.

No sobrescribir binarios gestionados por package managers.

---

# 15. GoReleaser y CI/CD

Crear `.goreleaser.yaml` con:

- Binario `qlog`.
- Builds para `darwin`, `linux` y `windows`.
- Arquitecturas `amd64` y `arm64`.
- `CGO_ENABLED=0`.
- `-trimpath`.
- Variables de versión, commit y fecha mediante `-ldflags`.
- Archives correctos: `.zip` para Windows, `.tar.gz` para Unix.
- Checksums SHA-256.
- SBOM.
- Release notes desde changelog.
- Artefactos reproducibles cuando sea viable.
- Firma o attestations de provenance.

GitHub Actions:

```text
ci.yml
release.yml
security.yml
installers.yml
nightly.yml opcional
```

Matriz mínima:

```text
ubuntu-latest
macos-latest
windows-latest
Go stable
```

Validaciones:

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run
govulncheck ./...
gofmt check
go mod tidy check
```

Añadir tests de instalación en contenedores y runners limpios.

---

# 16. Testing

Aplicar TDD en dominio, pricing, integridad y almacenamiento.

Cobertura requerida por criticidad:

- Pricing engine: cobertura exhaustiva de ramas y casos límite.
- Hash chain: tests deterministas, manipulación, orden y concurrencia.
- SQLite migrations: tests desde base vacía y upgrades.
- Redacción: tests de secretos y falsos positivos.
- CLI: tests de comandos y códigos de salida.
- TUI: golden tests de vistas sin depender de color exacto cuando no sea necesario.
- Installers: pruebas de OS/arch, checksum incorrecto, versión inexistente e idempotencia.
- Adaptadores: fixtures versionados y contract tests.

Añadir fuzz tests para:

```text
canonicalización de eventos
cálculo de costes
parsing de pricing
parsing de importaciones
redacción
```

No perseguir “100%” mediante tests vacíos. Priorizar mutaciones, invariantes y contratos reales.

---

# 17. MCP y skill

El MCP es una capa de integración, no la fuente principal de verdad.

Herramientas futuras:

```text
qlog.start_task
qlog.annotate_task
qlog.finish_task
qlog.get_current_session
qlog.get_task_summary
qlog.get_project_summary
qlog.check_budget
qlog.register_custom_model
qlog.verify_ledger
```

Crear una skill `quantum-log-usage-governor` que indique a los agentes:

- Iniciar o asociar una tarea.
- Clasificar el tipo de trabajo.
- Mantener correlación durante la sesión.
- Finalizar la tarea.
- Mostrar un resumen por modelo utilizado.
- No inventar tokens, costes o modelo.
- Etiquetar estimaciones.
- No almacenar contenido sensible.

La captura técnica debe seguir funcionando aunque la skill no se cargue.

---

# 18. Resultado final de una tarea

El resumen humano debe mostrar una línea por proveedor/modelo/proyecto agregado:

```text
AI USAGE · ngAutoPilot · BUILD · task:qlog_01J...

CODEX       | gpt-x          | in 42,180 | out 8,420 | cache 12,300 | tools 18 | 12m41s | $0.84 | EXACT
CLAUDE CODE | claude-example | in  9,110 | out 1,824 | cache      0 | tools  3 |  1m22s | $0.06 | ESTIMATED

TOTAL       | 73,834 tokens | 14m03s | $0.90 / €0.82 | ledger: pending evidence
```

La salida JSON debe conservar todos los campos sin redondear.

---

# 19. README inicial

El README debe incluir:

1. Qué problema resuelve.
2. Qué no hace.
3. Captura exacta frente a estimada.
4. Captura local-first y privacidad.
5. Instalación por plataforma.
6. Inicio rápido.
7. Captura mediante adaptadores.
8. Captura genérica con `qlog run`.
9. Ejemplos de informes.
10. Captura de pantalla de la TUI.
11. Arquitectura resumida.
12. Roadmap.
13. Seguridad.
14. Licencia.
15. Estado honesto de cada integración.

No utilizar métricas ficticias sin marcarlas como `DEMO DATA`.

---

# 20. Fases de ejecución

No intentes implementar todo en una sola entrega.

## Milestone 0 — Foundation

Entregar:

- ADRs principales.
- Scaffold Go.
- Cobra.
- Configuración.
- Version metadata.
- CI base.
- GoReleaser base.
- README inicial.
- Licencia y documentos de seguridad.

## Milestone 1 — Ledger Core

Entregar:

- Dominio.
- SQLite CGo-free.
- Migraciones.
- Raw events append-only.
- Normalización base.
- Cadena hash.
- `qlog init`.
- `qlog doctor`.
- `qlog verify`.
- Importador NDJSON.
- Tests y fixtures.

## Milestone 2 — CLI Reporting

Entregar:

- Proyectos, tareas, sesiones, model calls y tool calls.
- Queries y agregaciones.
- Pricing registry.
- Cost engine.
- `usage`, `report`, `pricing`, `export`.
- JSON output estable.

## Milestone 3 — TUI

Entregar:

- Overview.
- Live Trace.
- Sessions.
- Projects.
- Models.
- Costs.
- Integrity.
- Paletas accesibles y fallbacks.

## Milestone 4 — Capture

Entregar:

- OTLP HTTP receiver.
- Generic wrapper.
- Dos adaptadores oficiales verificables.
- Matriz de capacidades.
- Instalación idempotente de hooks/plugins.

## Milestone 5 — Distribution

Entregar:

- Releases multiplataforma.
- Install scripts.
- Checksums y SBOM.
- Homebrew.
- Scoop.
- WinGet.
- AUR.
- npm/Bun thin package.
- Tests de instalación.

## Milestone 6 — Agent Integration

Entregar:

- MCP server.
- Skill de gobierno.
- Resumen automático por tarea.
- Budgets y alertas.

---

# 21. Primera ejecución que debes realizar ahora

En esta primera ejecución implementa únicamente `Milestone 0` y el esqueleto compilable de `Milestone 1`.

Debes:

1. Inspeccionar el repositorio actual antes de modificarlo.
2. Informar qué existe y qué falta.
3. Proponer ADRs breves y concretos.
4. Crear el scaffold con arquitectura limpia.
5. Configurar `qlog --version`, `qlog init`, `qlog doctor` y `qlog verify` como comandos reales, aunque algunas capacidades de Milestone 1 estén inicialmente limitadas.
6. Crear la primera migración SQLite.
7. Implementar el almacenamiento de `raw_events` y la cadena hash mínima.
8. Crear tests de integridad y migraciones.
9. Configurar CI y GoReleaser.
10. Ejecutar todas las validaciones posibles.
11. No simular resultados de comandos que no se hayan ejecutado.
12. No avanzar a TUI, adaptadores comerciales o dashboard todavía.

Antes de escribir código, entrega:

```text
A. Resumen de decisiones
B. Riesgos
C. Árbol de archivos previsto
D. Plan de implementación en pasos pequeños
E. Criterios de aceptación
```

Después implementa, prueba y finaliza con:

```text
A. Archivos creados y modificados
B. Decisiones tomadas
C. Comandos ejecutados
D. Resultado real de tests, lint y build
E. Deuda técnica consciente
F. Próximo milestone recomendado
```

---

# 22. Criterios de aceptación del primer milestone

- `go build ./...` finaliza correctamente.
- `go test ./...` finaliza correctamente.
- El binario se llama `qlog`.
- `qlog --version` muestra versión, commit y build date.
- `qlog init` crea configuración y SQLite en rutas correctas para el sistema operativo.
- `qlog init` es idempotente.
- `qlog doctor` no modifica el sistema.
- `qlog doctor --json` produce JSON válido.
- `qlog verify` detecta una cadena íntegra.
- Un test demuestra que la modificación manual de un evento rompe la verificación.
- SQLite funciona con `CGO_ENABLED=0`.
- Las migraciones están embebidas.
- No se guarda contenido de prompt o respuesta.
- El proyecto tiene README, LICENSE, SECURITY, CONTRIBUTING y CHANGELOG.
- CI cubre Linux, macOS y Windows.
- GoReleaser puede producir snapshot local.
- No existen secretos, binarios ni bases de datos committeados.
- Los ejemplos de datos están marcados como DEMO o fixture.

---

# 23. Reglas de ingeniería

- Preferir diseño simple y contratos estables.
- Mantener dominio independiente de CLI, TUI y SQLite.
- No crear abstracciones sin un segundo caso real.
- No implementar interfaces gigantes.
- No ocultar errores.
- Usar errores tipados cuando aporten decisiones de control.
- Añadir contexto a errores sin duplicar mensajes.
- Usar `context.Context` en I/O y operaciones cancelables.
- Evitar goroutines huérfanas.
- Cerrar recursos correctamente.
- Documentar decisiones irreversibles mediante ADR.
- No introducir dependencias por comodidad sin revisar mantenimiento, licencia y superficie de ataque.
- Mantener una política de dependencias conservadora.
- No utilizar el logo como sustituto de arquitectura o calidad.
- La experiencia debe ser agradable, pero la evidencia y la fiabilidad tienen prioridad.

---

# 24. Definición de éxito

QUANTUM_LOG será exitoso cuando un desarrollador pueda instalarlo en cualquier sistema soportado, activar una integración, trabajar con uno o varios agentes y obtener un registro verificable que explique:

```text
qué ocurrió
quién lo originó
qué modelo participó
qué recursos consumió
cuando se consumio en este proyecto
cuánto costó
qué parte es exacta
qué parte es estimada
qué herramientas intervinieron
qué proyecto recibió el beneficio
si el registro sigue íntegro
```

Construye infraestructura de confianza. No construyas solamente una pantalla bonita.
